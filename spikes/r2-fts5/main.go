// main.go — R-2 实测入口。
// 子命令：
//   bench-tokenizers   对比 unicode61 / trigram / 预切词+unicode61 三种 FTS5 配置
//   build              生成 10000 篇样本并建库（单事务批量），输出耗时/库大小/内存
//   query              查询基准：30+ 条查询 p50/p95/p99，含 bm25 权重、过滤、limit
//   excerpt            片段与高亮生成耗时
//   incr               增量：改 1000 篇后的重索引耗时 + 期间并发查询延迟
//   links              链接图：全量解析 1 万篇耗时 + 改一篇后「只重算受影响边」耗时
//   all                依次跑 build→query→excerpt→incr→links
package main

import (
	"database/sql"
	"fmt"
	"os"
	"runtime"
	"sort"
	"time"

	_ "modernc.org/sqlite"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: r2 <bench-tokenizers|build|query|excerpt|incr|links|all> [opts]")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "bench-tokenizers":
		err = cmdBenchTokenizers()
	case "build":
		err = cmdBuild()
	case "query":
		err = cmdQuery()
	case "excerpt":
		err = cmdExcerpt()
	case "incr":
		err = cmdIncr()
	case "links":
		err = cmdLinks()
	case "probe":
		err = cmdProbe()
	case "offsets":
		err = cmdOffsets()
	case "all":
		for _, c := range []string{"bench-tokenizers", "build", "query", "excerpt", "incr", "links"} {
			fmt.Printf("\n========== %s ==========\n", c)
			if err = dispatch(c); err != nil {
				break
			}
		}
	default:
		err = fmt.Errorf("unknown subcommand %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func dispatch(c string) error {
	switch c {
	case "bench-tokenizers":
		return cmdBenchTokenizers()
	case "build":
		return cmdBuild()
	case "query":
		return cmdQuery()
	case "excerpt":
		return cmdExcerpt()
	case "incr":
		return cmdIncr()
	case "links":
		return cmdLinks()
	}
	return nil
}

func dsn(path string) string {
	return "file:" + path +
		"?_pragma=busy_timeout(5000)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=synchronous(NORMAL)" +
		"&_pragma=foreign_keys(1)"
}

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	i := int(float64(len(sorted)-1) * p)
	return sorted[i]
}

func stats(ds []time.Duration) (p50, p95, p99, max time.Duration) {
	s := append([]time.Duration(nil), ds...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	return percentile(s, 0.50), percentile(s, 0.95), percentile(s, 0.99), s[len(s)-1]
}

func memStats() (allocMB, sysMB int64) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.Alloc) / (1 << 20), int64(m.Sys) / (1 << 20)
}
