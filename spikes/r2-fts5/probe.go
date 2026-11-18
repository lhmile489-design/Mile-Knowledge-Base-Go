// probe.go — 诊断 raw unicode61 tokenizer 对 CJK 的切分行为。
package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// cmdProbe 在一个小表上观察 unicode61 对中文的真实切分。
// 运行：./r2fts5 probe
func cmdProbe() error {
	dbPath := filepath.Join(os.TempDir(), "r2fts5-probe.db")
	_ = os.Remove(dbPath)
	db, err := sql.Open("sqlite", dsn(dbPath))
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE VIRTUAL TABLE t USING fts5(x, tokenize='unicode61')`); err != nil {
		return err
	}
	if _, err := db.Exec(`INSERT INTO t(x) VALUES ('知识库管理'), ('检索系统'), ('hello world')`); err != nil {
		return err
	}
	// 列出 fts5vocab 不可用时，用 MATCH 探测单字 vs 词
	for _, tok := range []string{"知", "识", "知识", "识库", "检索", "索引", "中", "库", "hello", "hello world"} {
		var n int
		err := db.QueryRow(`SELECT count(*) FROM t WHERE t MATCH ?`, `"`+tok+`"`).Scan(&n)
		fmt.Printf("probe %-14q hits=%d err=%v\n", tok, n, err)
	}
	// 单字 CJK：写入单字 token 后能否被 MATCH
	if _, err := db.Exec(`DELETE FROM t`); err != nil {
		return err
	}
	if _, err := db.Exec(`INSERT INTO t(x) VALUES ('中'), ('库'), ('中文'), ('知识')`); err != nil {
		return err
	}
	fmt.Println("--- single-char table ---")
	for _, tok := range []string{"中", "库", "中文", "知识"} {
		var n int
		err := db.QueryRow(`SELECT count(*) FROM t WHERE t MATCH ?`, `"`+tok+`"`).Scan(&n)
		fmt.Printf("probe %-14q hits=%d err=%v\n", tok, n, err)
	}
	// FTS5 的 vocab 表能看实际 token
	if _, err := db.Exec(`CREATE VIRTUAL TABLE v USING fts5vocab(t, 'row')`); err == nil {
		rows, _ := db.Query(`SELECT term, doc FROM v LIMIT 50`)
		fmt.Println("\n--- vocab ---")
		for rows.Next() {
			var term string
			var doc int
			_ = rows.Scan(&term, &doc)
			fmt.Printf("  term=%q doc=%d\n", term, doc)
		}
		rows.Close()
	} else {
		fmt.Println("fts5vocab err:", err)
	}

	// 拆解 "知识" 查询耗时：FTS 本体 vs excerpt vs matched-fields
	fmt.Println("\n--- stage breakdown on big db ---")
	bigPath := dbFileFor("big")
	bdb, err := sql.Open("sqlite", dsn(bigPath))
	if err != nil {
		return err
	}
	defer bdb.Close()
	bdb.SetMaxOpenConns(1)
	bodyOf := func(p string) string { return bodyCache[p] }
	for _, q := range []string{"知识", "二元组分词对中日韩文字"} {
		qt := ftsQuery(q, true)
		// 阶段 A：纯 MATCH + rank + limit
		t0 := time.Now()
		var cnt int
		_ = bdb.QueryRow(`SELECT count(*) FROM (SELECT 1 FROM doc_fts WHERE doc_fts MATCH ? LIMIT 50)`, qt).Scan(&cnt)
		dA := time.Since(t0)
		// 阶段 B：FTS + join + rank
		t0 = time.Now()
		rows, _ := bdb.Query(`SELECT d.rowid, d.path, bm25(doc_fts,12.0,8.0,6.0,4.0,1.0)
			FROM doc_fts JOIN documents d ON d.rowid=doc_fts.rowid
			WHERE doc_fts MATCH ? ORDER BY 3 LIMIT 50`, qt)
		var rids []int64
		var paths []string
		for rows.Next() {
			var r int64
			var p string
			var rk float64
			_ = rows.Scan(&r, &p, &rk)
			rids = append(rids, r)
			paths = append(paths, p)
		}
		rows.Close()
		dB := time.Since(t0)
		// 阶段 C：excerpt on 50 hits
		t0 = time.Now()
		for _, p := range paths {
			_ = buildExcerpt(bodyOf(p), tokenize(q), 60)
		}
		dC := time.Since(t0)
		fmt.Printf("q=%q  fts-only=%v  join+rank=%v  excerpt50=%v\n", q, dA, dB, dC)
	}
	return nil
}

// cmdOffsets 测 matched-fields 几种实现的耗时对比。
func cmdOffsets() error {
	bigPath := dbFileFor("big")
	db, err := sql.Open("sqlite", dsn(bigPath))
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	q := "知识"
	qt := ftsQuery(q, true)
	qtoks := tokenize(q)

	// 先拿 50 个 rowid
	var rowids []int64
	rows, _ := db.Query(`SELECT d.rowid FROM doc_fts JOIN documents d ON d.rowid=doc_fts.rowid
		WHERE doc_fts MATCH ? ORDER BY bm25(doc_fts,12,8,6,4,1) LIMIT 50`, qt)
	for rows.Next() {
		var r int64
		_ = rows.Scan(&r)
		rowids = append(rowids, r)
	}
	rows.Close()
	fmt.Printf("50 rowids: %v...\n", rowids[:5])
	// 验证 path 列内容与 tokenize 输出对齐
	var pathCol string
	_ = db.QueryRow(`SELECT path FROM doc_fts WHERE rowid=?`, rowids[0]).Scan(&pathCol)
	fmt.Printf("rowid=%d path col=%q\n", rowids[0], pathCol)
	var docPath string
	_ = db.QueryRow(`SELECT path FROM documents WHERE rowid=?`, rowids[0]).Scan(&docPath)
	fmt.Printf("  documents.path=%q  tokenizePath=%v\n", docPath, tokenizePath(docPath))

	// 验证 unicode61 对 doc-00042 的实际切分（vocab 表看写入后的真实 token）
	if _, err := db.Exec(`DROP TABLE IF EXISTS vt`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE VIRTUAL TABLE vt USING fts5vocab(doc_fts, 'row')`); err == nil {
		rows, err := db.Query(`SELECT term FROM vt WHERE doc=? AND col='path'`, rowids[0])
		if err == nil {
			var terms []string
			for rows.Next() {
				var t string
				_ = rows.Scan(&t)
				terms = append(terms, t)
			}
			rows.Close()
			fmt.Printf("  fts5vocab path terms=%v\n", terms)
		} else {
			fmt.Println("vocab query err:", err)
		}
	} else {
		fmt.Println("fts5vocab err:", err)
	}
	// 查询侧 unicode61 对 "doc-00042" 的切分
	for _, tok := range []string{"doc-00042", "v4.1", "c#", "x-ray"} {
		var n int
		_ = db.QueryRow(`SELECT count(*) FROM doc_fts WHERE doc_fts MATCH ?`, `"`+tok+`"`).Scan(&n)
		fmt.Printf("  query %-12q hits=%d\n", tok, n)
	}

	// 方案 A：UNION 5 列 column-filter
	t0 := time.Now()
	var unions []string
	var args []any
	for _, c := range []string{"title", "tags", "headings", "path", "body"} {
		unions = append(unions, fmt.Sprintf(
			`SELECT rowid, '%s' col FROM doc_fts WHERE doc_fts MATCH ? AND rowid IN (%s)`,
			c, placeholders(len(rowids))))
		args = append(args, c+" : "+joinColTokens(qtoks))
		for _, r := range rowids {
			args = append(args, r)
		}
	}
	rows2, err := db.Query(strings.Join(unions, " UNION ALL "), args...)
	for rows2.Next() {
	}
	rows2.Close()
	dA := time.Since(t0)
	fmt.Printf("A union-col-filter: %v err=%v\n", dA, err)

	// 方案 B：每行 5 次 count (50×5=250 次单行查询)
	t0 = time.Now()
	hitCount := 0
	for _, rid := range rowids {
		for _, c := range []string{"title", "tags", "headings", "path", "body"} {
			var n int
			_ = db.QueryRow(`SELECT count(*) FROM doc_fts WHERE doc_fts MATCH ? AND rowid=?`,
				c+" : "+joinColTokens(qtoks), rid).Scan(&n)
			if n > 0 {
				hitCount++
			}
		}
	}
	dB := time.Since(t0)
	fmt.Printf("B per-row-5col: %v (hits=%d)\n", dB, hitCount)

	// 方案 C：用 FTS5 自带 column-filter 语法直接查每个列名（只查一次，返回 rowid 集合）
	t0 = time.Now()
	colSet := map[string][]int64{}
	for _, c := range []string{"title", "tags", "headings", "path", "body"} {
		rows, _ := db.Query(`SELECT rowid FROM doc_fts WHERE doc_fts MATCH ?`,
			fmt.Sprintf(`{%s} : %s`, c, joinColTokens(qtoks)))
		for rows.Next() {
			var r int64
			_ = rows.Scan(&r)
			colSet[c] = append(colSet[c], r)
		}
		rows.Close()
	}
	dC := time.Since(t0)
	for c, rs := range colSet {
		_ = c
		_ = rs
	}
	fmt.Printf("C colset-scan: %v\n", dC)

	// 方案 D：在 Go 侧做 matched fields —— 用 buildExcerpt 的变体直接对每列 token 串做 contains
	// 这要求我们把每列 token 串 SELECT 出来（一次），然后 Go 侧 contains
	t0 = time.Now()
	var unions2 []string
	var args2 []any
	_ = unions2
	_ = args2
	rows3, _ := db.Query(`SELECT d.rowid, doc_fts.title, doc_fts.tags, doc_fts.headings, doc_fts.path, doc_fts.body
		FROM doc_fts JOIN documents d ON d.rowid=doc_fts.rowid
		WHERE doc_fts.rowid IN (`+placeholders(len(rowids))+`)`, func() []any {
		a := make([]any, len(rowids))
		for i, r := range rowids {
			a[i] = r
		}
		return a
	}()...)
	for rows3.Next() {
		var rid int64
		var t, tg, h, p, b string
		_ = rows3.Scan(&rid, &t, &tg, &h, &p, &b)
		// Go 侧字面匹配每个 qtok 是否在该列
		_ = t
		_ = tg
		_ = h
		_ = p
		_ = b
	}
	rows3.Close()
	dD := time.Since(t0)
	fmt.Printf("D select-5col+go-match: %v\n", dD)
	return nil
}
