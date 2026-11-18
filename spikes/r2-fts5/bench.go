// bench.go — 各子命令实现。
package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

const bigN = 10_000

// ---------- bench-tokenizers ----------

func cmdBenchTokenizers() error {
	// 用 3000 篇做对比（建库时间 ×3 也能看出量级）
	n := 3000
	for _, tc := range []struct {
		name        string
		tokenizer   string
		pretokenize bool
	}{
		{"pre-tokenized+unicode61", "unicode61", true},
		{"raw+unicode61", "unicode61", false},
		{"raw+trigram", "trigram", false},
	} {
		dbPath := dbFileFor("bench-" + strings.ReplaceAll(tc.name, "+", "-"))
		elapsed, cnt, err := buildDB(dbPath, tc.tokenizer, tc.pretokenize, n, 1)
		if err != nil {
			return fmt.Errorf("%s: %w", tc.name, err)
		}
		db, _ := openDB(dbPath)
		// 中文二字词命中验证 + 延迟
		probe := []string{"知识", "检索", "deepseek", "v4.1"}
		fmt.Printf("\n[%s] build %d docs in %v, db %.1f MB\n", tc.name, cnt, elapsed, fileSizeMB(dbPath))
		for _, p := range probe {
			var q string
			if tc.pretokenize {
				q = ftsQuery(p, true)
			} else {
				q = `"` + p + `"` // 直接喂原文分词器
			}
			t0 := time.Now()
			var cnt int
			err := db.QueryRow(`SELECT count(*) FROM doc_fts WHERE doc_fts MATCH ?`, q).Scan(&cnt)
			fmt.Printf("  probe %-10q hits=%-5d took=%v (query=%s)\n", p, cnt, time.Since(t0), q)
			if err != nil {
				fmt.Printf("  probe %-10q err=%v\n", p, err)
			}
		}
		db.Close()
	}
	return nil
}

// ---------- build ----------

func cmdBuild() error {
	dbPath := dbFileFor("big")
	runtime.GC()
	a0, s0 := memStats()
	elapsed, cnt, err := buildDBWithCache(dbPath, "unicode61", true, bigN, 1)
	if err != nil {
		return err
	}
	a1, s1 := memStats()
	fmt.Printf("build: %d docs in %v (%.1f docs/s)\n", cnt, elapsed, float64(cnt)/elapsed.Seconds())
	fmt.Printf("db size: %.1f MB; wal %s; mem alloc %d->%d MB, sys %d->%d MB\n",
		fileSizeMB(dbPath), walSize(dbPath), a0, a1, s0, s1)
	return nil
}

func walSize(p string) string {
	fi, err := os.Stat(p + "-wal")
	if err != nil {
		return "n/a"
	}
	return fmt.Sprintf("%.1f MB", float64(fi.Size())/(1<<20))
}

// ---------- query ----------

// 30+ 条查询：覆盖中文二字/三字+/中英混合/英文带符号/标签/路径/无结果
var querySet = []string{
	// 中文二字（应命中：二元组）
	"知识", "检索", "索引", "分词", "增量", "事务", "链接", "高亮",
	// 中文三字以上（应命中：多个二元组 OR）
	"知识库", "检索系统", "增量更新", "权限控制", "知识库管理",
	// 中文长句（混合命中）
	"二元组分词对中日韩文字",
	// 中英混合
	"sqlite 索引", "bm25 权重", "webhook 事务", "provider 密钥",
	// 英文整词
	"sqlite", "tokenize", "markdown", "goroutine", "snapshot",
	// 英文带符号（词内 . - + #）
	"v4.1", "getusername", "c#", "x-ray",
	// 标签（应与 tags 列命中）
	"search", "历史", "检索",
	// 路径片段
	"doc-00042", "guide-v1", "skills", "raws",
	// 无结果（应返回空）
	"zzzznonexistent", "不存在的词组",
	// 单字中文
	"中", "库",
}

func cmdQuery() error {
	dbPath := dbFileFor("big")
	db, err := openDB(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	bodyOf := bodyReaderFromDB(db)

	fmt.Printf("%-30s %6s %9s %9s %9s %9s %6s\n", "query", "hits", "p50", "p95", "p99", "max", "matched")
	var allP95 []time.Duration
	for _, q := range querySet {
		var ds []time.Duration
		var hits []hitRow
		for r := 0; r < 20; r++ {
			h, _, d, err := runQuery(db, q, 50, "AND d.ignored=0", nil, bodyOf, true)
			if err != nil {
				return fmt.Errorf("query %q: %w", q, err)
			}
			ds = append(ds, d)
			hits = h
		}
		p50, p95, p99, mx := stats(ds)
		allP95 = append(allP95, p95)
		matched := "-"
		if len(hits) > 0 {
			matched = strings.Join(hits[0].matched, "+")
		}
		fmt.Printf("%-30q %6d %9v %9v %9v %9v %6s\n",
			truncate(q, 30), len(hits), p50, p95, p99, mx, matched)
	}
	s := append([]time.Duration(nil), allP95...)
	sortDur(s)
	fmt.Printf("\nsummary: max p95 across queries = %v; median p95 = %v\n",
		s[len(s)-1], s[len(s)/2])

	// 过滤与 limit 变体（在代表性的中文词上测）
	for _, tc := range []struct {
		name, filters string
		args          []any
		limit         int
	}{
		{"limit=200 area+ext", "AND d.ignored=0 AND d.area IN ('skills','notes') AND d.extension='md'", nil, 200},
		{"limit=50 tag", "AND d.ignored=0 AND d.tags LIKE '%search%'", nil, 50},
		{"limit=50 mtime>=30d", fmt.Sprintf("AND d.ignored=0 AND d.mtime_ms >= %d", time.Now().Add(-30*24*time.Hour).UnixMilli()), nil, 50},
		{"limit=50 area=system", "AND d.area='system'", nil, 50},
	} {
		var ds []time.Duration
		for r := 0; r < 20; r++ {
			_, _, d, err := runQuery(db, "知识库", tc.limit, tc.filters, tc.args, bodyOf, false)
			if err != nil {
				return err
			}
			ds = append(ds, d)
		}
		p50, p95, _, _ := stats(ds)
		fmt.Printf("filter %-22s p50=%v p95=%v\n", tc.name, p50, p95)
	}
	return nil
}

func sortDur(s []time.Duration) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// bodyReaderFromDB 模拟「正文不入库，按 path 从磁盘读」。
// 本 spike 中把所有文档 body 缓存在 map（相当于小 LRU 全命中）来测纯 CPU 端 excerpt 耗时；
// 同时也测一个「模拟磁盘延迟」版本作为对照。
func bodyReaderFromDB(db *sql.DB) func(string) string {
	// 简化：从 documents 表读不出正文（我们没有入库正文），
	// 所以单独保留一份内存 map——这与旧版 excerptFor 从 vault 读文件语义一致，
	// 但 spike 用内存来隔离「片段算法」与「磁盘 IO」两个变量。
	return func(path string) string {
		return bodyCache[path]
	}
}

var bodyCache = map[string]string{}

// ---------- excerpt ----------

func cmdExcerpt() error {
	dbPath := dbFileFor("big")
	db, err := openDB(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	// 先确认 bodyCache 已填充（build 时应填充；若没有则用一条典型 body 测算法本身）
	if len(bodyCache) == 0 {
		// build 时不在同一进程（分开跑的），用合成 body 测算法耗时
		rng := rand.New(rand.NewSource(7))
		for i := 0; i < 200; i++ {
			p, c := genDocument(rng, i)
			bodyCache[p] = parseDocument(p, c).body
		}
	}

	qtoks := tokenize("知识库")
	var ds []time.Duration
	for _, body := range bodyCache {
		t0 := time.Now()
		ex := buildExcerpt(body, qtoks, 60)
		_ = ex
		hl := highlightRanges(ex, qtoks)
		_ = hl
		ds = append(ds, time.Since(t0))
	}
	p50, p95, p99, mx := stats(ds)
	fmt.Printf("excerpt+highlight over %d bodies: p50=%v p95=%v p99=%v max=%v\n",
		len(ds), p50, p95, p99, mx)
	return nil
}

// ---------- incr ----------

func cmdIncr() error {
	dbPath := dbFileFor("big")
	db, err := openDB(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	// 改 1000 篇：从已有 documents 取 1000 个真实 path，生成新内容重索引
	var paths []string
	rows0, err := db.Query(`SELECT path FROM documents WHERE ignored=0 ORDER BY rowid LIMIT 1000`)
	if err != nil {
		return err
	}
	for rows0.Next() {
		var p string
		_ = rows0.Scan(&p)
		paths = append(paths, p)
	}
	rows0.Close()
	rng := rand.New(rand.NewSource(2))
	changed := make([]struct {
		path, content string
		doc           parsedDocument
	}, 0, len(paths))
	for _, p := range paths {
		_, c := genDocument(rng, 0) // 新内容
		changed = append(changed, struct {
			path, content string
			doc           parsedDocument
		}{p, c, parseDocument(p, c)})
	}

	t0 := time.Now()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	for _, d := range changed {
		area := strings.SplitN(d.path, "/", 2)[0]
		var rowid int64
		if err := tx.QueryRow(`SELECT rowid FROM documents WHERE path=?`, d.path).Scan(&rowid); err != nil {
			_ = tx.Rollback()
			return err
		}
		if _, err := tx.Exec(`DELETE FROM doc_fts WHERE rowid=?`, rowid); err != nil {
			_ = tx.Rollback()
			return err
		}
		if _, err := tx.Exec(`DELETE FROM links WHERE from_path=?`, d.path); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := insertDoc(tx, d.doc, d.path, area, "md", len(d.content),
			time.Now().UnixMilli(), true); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	commitDur := time.Since(t0)
	if err := tx.Commit(); err != nil {
		return err
	}
	fmt.Printf("incr: 1000 docs reindexed (single tx) in %v (%.1f docs/s)\n",
		commitDur, 1000/commitDur.Seconds())

	// 并发：一边写一边查，测查询延迟是否被写锁阻塞
	bodyOf := bodyReaderFromDB(db)
	var wg sync.WaitGroup
	stop := make(chan struct{})
	var queryLat []time.Duration
	var mu sync.Mutex
	wg.Add(1)
	go func() {
		defer wg.Done()
		// 第二个连接读（WAL 允许读不阻塞写）
		rdb, err := sql.Open("sqlite", dsn(dbPath))
		if err != nil {
			return
		}
		defer rdb.Close()
		rdb.SetMaxOpenConns(1)
		for {
			select {
			case <-stop:
				return
			default:
				_, _, d, err := runQuery(rdb, "知识库", 50, "AND d.ignored=0", nil, bodyOf, false)
				if err == nil {
					mu.Lock()
					queryLat = append(queryLat, d)
					mu.Unlock()
				}
			}
		}
	}()
	// 再跑一批更新（用真实 path）
	rng2 := rand.New(rand.NewSource(3))
	var paths2 []string
	rows3, _ := db.Query(`SELECT path FROM documents WHERE ignored=0 ORDER BY rowid DESC LIMIT 500`)
	for rows3.Next() {
		var p string
		_ = rows3.Scan(&p)
		paths2 = append(paths2, p)
	}
	rows3.Close()
	t0 = time.Now()
	tx2, _ := db.Begin()
	for _, p := range paths2 {
		_, c := genDocument(rng2, 0)
		pd := parseDocument(p, c)
		area := strings.SplitN(p, "/", 2)[0]
		var rowid int64
		if err := tx2.QueryRow(`SELECT rowid FROM documents WHERE path=?`, p).Scan(&rowid); err == nil {
			_, _ = tx2.Exec(`DELETE FROM doc_fts WHERE rowid=?`, rowid)
			_, _ = tx2.Exec(`DELETE FROM links WHERE from_path=?`, p)
			_ = insertDoc(tx2, pd, p, area, "md", len(c), time.Now().UnixMilli(), true)
		}
	}
	_ = tx2.Commit()
	writeDur := time.Since(t0)
	close(stop)
	wg.Wait()
	if len(queryLat) > 0 {
		p50, p95, p99, mx := stats(queryLat)
		fmt.Printf("during %v write: %d queries p50=%v p95=%v p99=%v max=%v\n",
			writeDur, len(queryLat), p50, p95, p99, mx)
	}
	return nil
}

// ---------- links ----------

func cmdLinks() error {
	dbPath := dbFileFor("big")
	db, err := openDB(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	// 全量候选：path + title
	type cand struct{ path, title string }
	var candidates []linkTarget
	rows, err := db.Query(`SELECT path, title FROM documents WHERE ignored=0`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var c cand
		if err := rows.Scan(&c.path, &c.title); err != nil {
			return err
		}
		candidates = append(candidates, linkTarget{path: c.path, title: c.title})
	}
	rows.Close()
	fmt.Printf("link candidates: %d\n", len(candidates))

	// 全量解析：对 links 表所有未解析边跑 resolveLink
	t0 := time.Now()
	var linkCount, resolvedCount, multiCount, noneCount int
	rows2, err := db.Query(`SELECT from_path, target FROM links`)
	if err != nil {
		return err
	}
	type upd struct{ from, target, to string; cands int }
	var ups []upd
	for rows2.Next() {
		var from, target string
		if err := rows2.Scan(&from, &target); err != nil {
			return err
		}
		linkCount++
		ok, to, n := resolveLink(target, candidates)
		if ok {
			resolvedCount++
			ups = append(ups, upd{from, target, to, 1})
		} else {
			if n > 1 {
				multiCount++
			} else {
				noneCount++
			}
			ups = append(ups, upd{from, target, "", n})
		}
	}
	rows2.Close()
	resolveDur := time.Since(t0)

	t0 = time.Now()
	tx, _ := db.Begin()
	for _, u := range ups {
		if u.to == "" {
			if _, err := tx.Exec(`UPDATE links SET to_path=NULL, candidates=? WHERE from_path=? AND target=?`,
				u.cands, u.from, u.target); err != nil {
				return err
			}
		} else {
			if _, err := tx.Exec(`UPDATE links SET to_path=?, candidates=1 WHERE from_path=? AND target=?`,
				u.to, u.from, u.target); err != nil {
				return err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	writeDur := time.Since(t0)
	fmt.Printf("resolve all %d links: resolve=%v write=%v (resolved=%d multi=%d none=%d)\n",
		linkCount, resolveDur, writeDur, resolvedCount, multiCount, noneCount)

	// 增量：改一篇文档，只重算「指向它的入边」
	// 选一篇真实存在的 path，改它的标题，找所有 target 匹配它的入边重算
	var targetPath string
	if err := db.QueryRow(`SELECT path FROM documents WHERE ignored=0 LIMIT 1 OFFSET 500`).Scan(&targetPath); err != nil {
		return err
	}
	var oldTitle string
	if err := db.QueryRow(`SELECT title FROM documents WHERE path=?`, targetPath).Scan(&oldTitle); err != nil {
		return err
	}
	newTitle := "renamed-title-" + time.Now().Format("150405")

	t0 = time.Now()
	// 受影响源文档：links.target ∈ {旧标题, 旧文件名, 新标题}
	affected := map[string]bool{}
	likeTargets := []string{oldTitle, baseName(stripExtension(targetPath)), newTitle}
	for _, t := range likeTargets {
		rows, err := db.Query(`SELECT from_path, target FROM links WHERE target=?`, t)
		if err != nil {
			return err
		}
		for rows.Next() {
			var f, tg string
			_ = rows.Scan(&f, &tg)
			affected[f+"\x00"+tg] = true
		}
		rows.Close()
	}
	tx, _ = db.Begin()
	_, _ = tx.Exec(`UPDATE documents SET title=? WHERE path=?`, newTitle, targetPath)
	recount := 0
	for k := range affected {
		parts := strings.SplitN(k, "\x00", 2)
		from, target := parts[0], parts[1]
		ok, to, n := resolveLink(target, candidates) // 用旧 candidates；实际应用应重读新 title 的候选
		_ = ok
		if n == 0 && to == "" {
			_, _ = tx.Exec(`UPDATE links SET to_path=NULL, candidates=0 WHERE from_path=? AND target=?`, from, target)
		} else {
			_, _ = tx.Exec(`UPDATE links SET to_path=?, candidates=? WHERE from_path=? AND target=?`, to, n, from, target)
		}
		recount++
	}
	_ = tx.Commit()
	incrDur := time.Since(t0)
	fmt.Printf("incr relink (1 doc title change): %d affected edges re-resolved in %v\n",
		recount, incrDur)
	return nil
}
