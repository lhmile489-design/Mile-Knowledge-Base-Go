package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

// r1-watch spike: compares three ways to watch a large vault on Windows.
//
//	go run . -mode gen <dir>                    build the sample tree only
//	go run . -mode a -vault <dir>               mygo.WatchFiles, normal, MaxEntries raised
//	go run . -mode b -vault <dir>               mygo.WatchFiles, HintsOnly + Go-side dir diff
//	go run . -mode c -vault <dir>               fsnotify recursive (path\... on Windows)
//
// Modes a/b/c run the S1–S4 scenarios and print a result table as JSON-ish
// lines. A global deadline exits the process no matter what.

var (
	mode    = flag.String("mode", "gen", "gen|a|b|c")
	vault   = flag.String("vault", "", "vault root; gen creates it, others expect it to exist")
	maxEnt  = flag.Int("maxentries", 65536, "MaxEntries for mode a")
	deb     = flag.Duration("debounce", 50*time.Millisecond, "debounce for mode a/b")
	s1n     = flag.Int("s1n", 50, "S1 repetitions")
	timeout = flag.Duration("timeout", 10*time.Minute, "global watchdog")
	quiet   = flag.Bool("quiet", false, "less progress output")
)

func logf(format string, args ...any) {
	if !*quiet {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}

func main() {
	flag.Parse()
	// Watchdog: never hang forever.
	go func() {
		time.Sleep(*timeout)
		fmt.Fprintln(os.Stderr, "FATAL: global timeout hit")
		os.Exit(3)
	}()

	root := *vault
	if root == "" {
		var err error
		root, err = os.MkdirTemp("", "r1-vault-")
		if err != nil {
			panic(err)
		}
		if *mode != "gen" {
			defer os.RemoveAll(root)
		}
	}
	root, _ = filepath.Abs(root)

	rnd := rand.New(rand.NewSource(20261118))

	switch *mode {
	case "gen":
		st, err := buildSample(root, rnd)
		if err != nil {
			panic(err)
		}
		fmt.Printf("sample: root=%s md=%d dirs=%d repoFiles=%d ignoredFiles=%d totalFiles=%d totalDirs=%d\n",
			st.Root, st.MDFiles, st.Dirs, st.RepoFiles, st.IgnoredFiles, st.TotalFiles, st.TotalDirs)
	case "a", "b":
		runMygo(root, *mode == "b", rnd)
	case "c":
		runFSNotify(root, rnd)
	default:
		panic("bad -mode")
	}
}
