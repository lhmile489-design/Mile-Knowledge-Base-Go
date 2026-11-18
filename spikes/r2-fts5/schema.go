// schema.go — 建表、写入、查询的实现，以及 tokenizer 对比。
package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const schemaDocsLinks = `
CREATE TABLE IF NOT EXISTS documents (
  path         TEXT PRIMARY KEY,
  title        TEXT NOT NULL,
  area         TEXT NOT NULL,
  tags         TEXT NOT NULL DEFAULT '',
  extension    TEXT NOT NULL,
  mtime_ms     INTEGER NOT NULL,
  size         INTEGER NOT NULL,
  ignored      INTEGER NOT NULL DEFAULT 0,
  body_indexed INTEGER NOT NULL DEFAULT 1,
  updated_at   TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS documents_area  ON documents(area);
CREATE INDEX IF NOT EXISTS documents_mtime ON documents(mtime_ms);

CREATE TABLE IF NOT EXISTS links (
  from_path   TEXT NOT NULL,
  target      TEXT NOT NULL,
  kind        TEXT NOT NULL,
  to_path     TEXT,
  candidates  INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (from_path, target)
);
CREATE INDEX IF NOT EXISTS links_to   ON links(to_path);
CREATE INDEX IF NOT EXISTS links_from ON links(from_path);

CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
`

// ftsSchema 三种 tokenizer 配置：
//
//	A = 预切词 + unicode61（推荐：Go 侧切词后空格拼接写入）
//	B = unicode61 直接喂原文（对照组：CJK 整句被当一个 token）
//	C = trigram（备选：SQLite 3.34+ 自带子串匹配，不用预切词）
func ftsSchema(tokenizer string) string {
	return fmt.Sprintf(`
CREATE VIRTUAL TABLE IF NOT EXISTS doc_fts USING fts5(
  title, tags, headings, path, body,
  tokenize='%s'
);`, tokenizer)
}

// dropFTS 让同一库文件可以换 tokenizer 重建。
const dropFTS = `DROP TABLE IF EXISTS doc_fts;`

// insertDoc 把一篇文档写入 documents + doc_fts + links（调用方负责事务）。
// pretokenize=true 时五列都写「空格拼接的 token 串」；false 时写原文（仅用于对照）。
func insertDoc(tx *sql.Tx, doc parsedDocument, path, area, ext string, size int, mtime int64, pretokenize bool) error {
	var titleV, tagsV, headingsV, pathV, bodyV any
	if pretokenize {
		titleV = strings.Join(tokenize(doc.title), " ")
		tagsV = strings.Join(tokenize(strings.Join(doc.tags, " ")), " ")
		var h []string
		for _, hh := range doc.headings {
			h = append(h, tokenize(hh)...)
		}
		headingsV = strings.Join(h, " ")
		pathV = strings.Join(doc.pathTokens, " ")
		bodyV = strings.Join(tokenize(doc.body), " ")
	} else {
		titleV, tagsV, headingsV, pathV, bodyV = doc.title, strings.Join(doc.tags, " "),
			strings.Join(doc.headings, "\n"), path, doc.body
	}
	ignored := 0
	if area == "system" {
		ignored = 1
	}
	tagsJoined := strings.Join(doc.tags, " ")

	if _, err := tx.Exec(
		`INSERT INTO documents(path,title,area,tags,extension,mtime_ms,size,ignored,body_indexed,updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?)
		 ON CONFLICT(path) DO UPDATE SET title=excluded.title, area=excluded.area,
		   tags=excluded.tags, extension=excluded.extension, mtime_ms=excluded.mtime_ms,
		   size=excluded.size, ignored=excluded.ignored, body_indexed=excluded.body_indexed,
		   updated_at=excluded.updated_at`,
		path, doc.title, area, tagsJoined, ext, mtime, size, ignored, 1,
		time.UnixMilli(mtime).UTC().Format(time.RFC3339)); err != nil {
		return err
	}
	var rowid int64
	if err := tx.QueryRow(`SELECT rowid FROM documents WHERE path=?`, path).Scan(&rowid); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`INSERT INTO doc_fts(rowid,title,tags,headings,path,body) VALUES (?,?,?,?,?,?)`,
		rowid, titleV, tagsV, headingsV, pathV, bodyV); err != nil {
		return err
	}
	for _, l := range doc.links {
		if _, err := tx.Exec(
			`INSERT OR REPLACE INTO links(from_path,target,kind,to_path,candidates) VALUES (?,?,?,NULL,0)`,
			path, l.target, l.kind); err != nil {
			return err
		}
	}
	return nil
}

// generateDocs 生成 n 篇文档的内存表示（供 build/bench 共用）。
func generateDocs(n int, seed int64) []struct {
	path, content string
	doc           parsedDocument
} {
	rng := rand.New(rand.NewSource(seed))
	out := make([]struct {
		path, content string
		doc           parsedDocument
	}, 0, n)
	for i := 0; i < n; i++ {
		p, c := genDocument(rng, i)
		out = append(out, struct {
			path, content string
			doc           parsedDocument
		}{p, c, parseDocument(p, c)})
	}
	return out
}

func fileSizeMB(path string) float64 {
	fi, err := os.Stat(path)
	if err != nil {
		return -1
	}
	return float64(fi.Size()) / (1 << 20)
}

// buildDB 用给定 tokenizer 建库；pretokenize 决定五列是否写预切词。
// 返回首建耗时与文档数。
func buildDB(dbPath, tokenizer string, pretokenize bool, n int, seed int64) (time.Duration, int, error) {
	elapsed, cnt, err := buildDBWithCache(dbPath, tokenizer, pretokenize, n, seed)
	return elapsed, cnt, err
}

// buildDBWithCache 同 buildDB，但把 body 填充进 bodyCache（供 excerpt 测试用）。
func buildDBWithCache(dbPath, tokenizer string, pretokenize bool, n int, seed int64) (time.Duration, int, error) {
	_ = os.Remove(dbPath)
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")
	db, err := openDB(dbPath)
	if err != nil {
		return 0, 0, err
	}
	defer db.Close()
	if _, err := db.Exec(schemaDocsLinks); err != nil {
		return 0, 0, err
	}
	if _, err := db.Exec(dropFTS); err != nil {
		return 0, 0, err
	}
	if _, err := db.Exec(ftsSchema(tokenizer)); err != nil {
		return 0, 0, err
	}

	docs := generateDocs(n, seed)
	bodyCache = make(map[string]string, n)
	for _, d := range docs {
		bodyCache[d.path] = d.doc.body
	}
	t0 := time.Now()
	tx, err := db.Begin()
	if err != nil {
		return 0, 0, err
	}
	for i, d := range docs {
		area := strings.SplitN(d.path, "/", 2)[0]
		mtime := time.Now().Add(-time.Duration(i%60) * 24 * time.Hour).UnixMilli()
		if err := insertDoc(tx, d.doc, d.path, area, "md", len(d.content), mtime, pretokenize); err != nil {
			_ = tx.Rollback()
			return 0, 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	elapsed := time.Since(t0)
	return elapsed, len(docs), nil
}

// queryFTS 构造 FTS5 查询串：Go 侧切词后按 joiner（" OR " / " "）连接。
func ftsQuery(text string, orJoin bool) string {
	toks := tokenize(text)
	if len(toks) == 0 {
		return ""
	}
	esc := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
	parts := make([]string, len(toks))
	for i, t := range toks {
		parts[i] = esc(t)
	}
	if orJoin {
		return strings.Join(parts, " OR ")
	}
	return strings.Join(parts, " ")
}

// hitRow 一条检索结果。
type hitRow struct {
	path, title, area string
	matched           []string // 命中的列名
	excerpt           string
	rank              float64
}

// runQuery 执行一次检索，返回命中行与总耗时。
// matchedFields=true 时对返回页用一次 SQL 批量探测各列（而不是逐行 5 次）。
// 片段从磁盘按 path 读出（模拟「正文不入库」）——本 spike 中 body 存在内存 map 代替磁盘。
func runQuery(db *sql.DB, text string, limit int, filters string, args []any,
	bodyOf func(path string) string, matchedFields bool) ([]hitRow, int, time.Duration, error) {
	t0 := time.Now()
	q := ftsQuery(text, true) // OR 召回，对齐旧版「任一词命中即召回」
	if q == "" {
		return nil, 0, 0, nil
	}
	sqlStr := `SELECT d.rowid, d.path, d.title, d.area, bm25(doc_fts, 12.0, 8.0, 6.0, 4.0, 1.0)
		FROM doc_fts JOIN documents d ON d.rowid = doc_fts.rowid
		WHERE doc_fts MATCH ? ` + filters + ` ORDER BY 5 LIMIT ?`
	full := append([]any{q}, append(args, limit)...)
	rows, err := db.Query(sqlStr, full...)
	if err != nil {
		return nil, 0, 0, err
	}
	var hits []hitRow
	var rowids []int64
	for rows.Next() {
		var h hitRow
		var rid int64
		if err := rows.Scan(&rid, &h.path, &h.title, &h.area, &h.rank); err != nil {
			rows.Close()
			return nil, 0, 0, err
		}
		hits = append(hits, h)
		rowids = append(rowids, rid)
	}
	rows.Close()
	total := len(hits)

	qtoks := tokenize(text)

	// 匹配理由：一次 SELECT 拿出 5 列的预切词串，Go 侧对每列做 token contains。
	// 比 column-filter UNION（22ms）或逐行 5 次 count（25ms）快一个量级。
	if matchedFields && len(rowids) > 0 {
		rows, err := db.Query(
			`SELECT rowid, title, tags, headings, path, body FROM doc_fts WHERE rowid IN (`+
				placeholders(len(rowids))+`)`, func() []any {
				a := make([]any, len(rowids))
				for i, r := range rowids {
					a[i] = r
				}
				return a
			}()...)
		if err == nil {
			type rowCols struct{ t, tg, h, p, b string }
			colsByRid := map[int64]rowCols{}
			for rows.Next() {
				var rid int64
				var rc rowCols
				_ = rows.Scan(&rid, &rc.t, &rc.tg, &rc.h, &rc.p, &rc.b)
				colsByRid[rid] = rc
			}
			rows.Close()
			// 写入时是空格拼接的 token 串。FTS5 的 unicode61 会把含 - . + # 的 Go token
			// 再拆成子 token（如 doc-00042 → doc, 00042），查询侧同样拆。
			// 所以 matched 计算也要按 unicode61 口径把 Go token 拆一次。
			unicodeToks := func(goTok string) []string {
				var parts []string
				var cur strings.Builder
				for _, r := range goTok {
					isWord := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
						(r >= '0' && r <= '9') || r >= 0x80
					if isWord {
						cur.WriteRune(r)
					} else {
						if cur.Len() > 0 {
							parts = append(parts, cur.String())
							cur.Reset()
						}
					}
				}
				if cur.Len() > 0 {
					parts = append(parts, cur.String())
				}
				return parts
			}
			containsTok := func(col, tok string) bool {
				if col == "" {
					return false
				}
				padded := " " + col + " "
				// Go token 按 unicode61 口径再拆，所有子 token 都要命中才算命中
				for _, sub := range unicodeToks(tok) {
					if !strings.Contains(padded, " "+sub+" ") {
						return false
					}
				}
				return true
			}
			for i := range hits {
				rc, ok := colsByRid[rowids[i]]
				if !ok {
					continue
				}
				var fields []string
				for _, tok := range qtoks {
					if containsTok(rc.t, tok) {
						fields = append(fields, "title")
						break
					}
				}
				for _, tok := range qtoks {
					if containsTok(rc.tg, tok) {
						fields = append(fields, "tags")
						break
					}
				}
				for _, tok := range qtoks {
					if containsTok(rc.h, tok) {
						fields = append(fields, "headings")
						break
					}
				}
				for _, tok := range qtoks {
					if containsTok(rc.p, tok) {
						fields = append(fields, "path")
						break
					}
				}
				for _, tok := range qtoks {
					if containsTok(rc.b, tok) {
						fields = append(fields, "body")
						break
					}
				}
				hits[i].matched = fields
			}
		}
	}

	for i := range hits {
		if body := bodyOf(hits[i].path); body != "" {
			hits[i].excerpt = buildExcerpt(body, qtoks, 60)
		}
	}
	return hits, total, time.Since(t0), nil
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	s := "?"
	for i := 1; i < n; i++ {
		s += ",?"
	}
	return s
}

// joinColTokens 把 token 列表拼成 column-filter 下的 OR 组：("a" OR "b")
// FTS5 的 token 字面量用双引号，不用单引号（单引号会被当成语法错误）。
func joinColTokens(toks []string) string {
	if len(toks) == 0 {
		return `""`
	}
	esc := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
	parts := make([]string, len(toks))
	for i, t := range toks {
		parts[i] = esc(t)
	}
	return `(` + strings.Join(parts, " OR ") + `)`
}

// dbFileFor 返回临时目录下的库路径。
func dbFileFor(name string) string {
	dir := os.TempDir()
	return filepath.Join(dir, "r2fts5-"+name+".db")
}
