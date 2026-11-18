package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
)

// mygo_modes.go: modes a (normal scan watch, MaxEntries raised) and
// b (HintsOnly + Go-side snapshot diff).
//
// mygo.WatchFiles needs the app running: we run the scenarios inside
// App.OnReady on a worker goroutine and quit when done.

type result struct {
	mode       string
	startErr   error
	watchSetup time.Duration
	heapAfter  uint64
	rssAfter   uint64
	idleCPUPer float64
	s1         []time.Duration
	s1Miss     int
	s2Total    time.Duration
	s2Seen     int
	s2Lost     int
	s3         map[string]bool
	s4Events   int // events that pointed inside ignored subtrees
	eventsSeen int
	notes      []string
}

func runMygo(root string, hintsOnly bool, rnd *rand.Rand) {
	done := make(chan *result, 1)
	mygo.App.OnReady(func() {
		go func() {
			done <- scenariosMygo(root, hintsOnly, rnd)
			// give stderr a moment to flush, then quit
			time.Sleep(200 * time.Millisecond)
			mygo.App.Quit()
		}()
	})
	if err := mygo.App.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "App.Run:", err)
		os.Exit(2)
	}
	r := <-done
	printResult(r)
}

func ignoreFunc(path string, isDir bool) bool {
	base := path
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	return isDir && (base == "node_modules" || base == ".git")
}

// ---- mode a --------------------------------------------------------------

func scenariosMygo(root string, hintsOnly bool, rnd *rand.Rand) *result {
	r := &result{s3: map[string]bool{}}
	if hintsOnly {
		r.mode = "b/mygo-hintsonly"
	} else {
		r.mode = "a/mygo-normal"
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runtime_Gosched_freeOSMemory()

	t0 := time.Now()
	w, err := mygo.WatchFiles(ctx, root, mygo.FileWatchOptions{
		Recursive:  true,
		Ignore:     ignoreFunc,
		Debounce:   *deb,
		MaxEntries: *maxEnt,
		HintsOnly:  hintsOnly,
	})
	r.watchSetup = time.Since(t0)
	if err != nil {
		r.startErr = err
		return r
	}
	defer w.Close()

	snk := newMapSink()
	var mu sync.Mutex
	ignoredEvents := 0

	// For HintsOnly we keep a snapshot of the vault (path -> mtime,size)
	// and diff on every hint to derive per-file changes, which is what a
	// real indexer would do.
	var snap *vaultSnap
	if hintsOnly {
		snap = newVaultSnap(root)
		snap.scan()
	}

	go func() {
		for {
			evs, err := w.Next(ctx)
			if err != nil {
				if err != io.EOF && err != context.Canceled {
					logf("next err: %v", err)
				}
				return
			}
			now := time.Now()
			mu.Lock()
			if hintsOnly {
				// diff affected dirs (or everything for ".")
				for _, rel := range snap.diff(evs) {
					snk.seen(rel, now)
					if isIgnored(rel) {
						ignoredEvents++
					}
				}
			} else {
				for _, e := range evs {
					rel := e.Path
					snk.seen(rel, now)
					if isIgnored(rel) {
						ignoredEvents++
					}
				}
			}
			r.eventsSeen += len(evs)
			mu.Unlock()
		}
	}()

	r.heapAfter = goHeap()
	r.rssAfter = procRSS()

	runScenarios(root, snk, &mu, r, rnd)

	mu.Lock()
	r.s4Events = ignoredEvents
	mu.Unlock()
	return r
}

// runScenarios executes S1-S4 against any sink being fed in the
// background. mu serializes access to snk.
func runScenarios(root string, snk *mapSink, mu *sync.Mutex, r *result, rnd *rand.Rand) {
	// ---- idle CPU: 30s would dominate the run; use 10s and note it. The
	// requirement says 30s idle; we do 30s once here.
	cpu0 := procCPU()
	time.Sleep(30 * time.Second)
	r.idleCPUPer = float64(procCPU()-cpu0) / float64(30*time.Second) * 100

	// ---- S1: atomic save latency, s1n reps.
	targets := pickTargets(root, rnd, *s1n)
	for i, f := range targets {
		rel := relTo(root, f)
		content := []byte(fmt.Sprintf("# edit %d\n", i))
		start := time.Now()
		if err := atomicWrite(f, content); err != nil {
			r.s1Miss++
			continue
		}
		if t, ok := waitSeenFrom(snk, rel, start, 10*time.Second); ok {
			r.s1 = append(r.s1, t.Sub(start))
		} else {
			r.s1Miss++
		}
		// settle: let watcher catch up so next write is isolated
		time.Sleep(60 * time.Millisecond)
	}

	// ---- S2: burst-write 1000 files; time until all seen.
	var burst []string
	for _, f := range pickTargets(root, rnd, 1200)[:1000] {
		burst = append(burst, f)
	}
	// clear prior stamps so only this burst counts
	mu.Lock()
	snk.m = map[string]time.Time{}
	mu.Unlock()
	start := time.Now()
	for i, f := range burst {
		_ = atomicWrite(f, []byte(fmt.Sprintf("burst %d", i)))
	}
	writeDone := time.Now()
	deadline := time.Now().Add(120 * time.Second)
	seenSet := map[string]bool{}
	for time.Now().Before(deadline) && len(seenSet) < len(burst) {
		for _, f := range burst {
			rel := relTo(root, f)
			if !seenSet[rel] {
				if t, ok := snk.seenAt(rel); ok && !t.Before(start) {
					seenSet[rel] = true
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	r.s2Total = time.Since(start)
	r.s2Seen = len(seenSet)
	r.s2Lost = len(burst) - len(seenSet)
	r.notes = append(r.notes, fmt.Sprintf("S2 write phase took %s", fmtDur(writeDone.Sub(start))))

	// ---- S3: mkdir / rmdir / rename dir.
	s3root := filepath.Join(root, "notes")
	newDir := filepath.Join(s3root, "r1-newdir")
	renDir := filepath.Join(s3root, "r1-renamed")
	mu.Lock()
	snk.m = map[string]time.Time{}
	mu.Unlock()
	_ = os.MkdirAll(newDir, 0o755)
	_ = os.WriteFile(filepath.Join(newDir, "x.md"), []byte("x"), 0o644)
	ok1 := waitDirOrContent(snk, root, "notes/r1-newdir", 10*time.Second)
	_ = os.Rename(newDir, renDir)
	ok2 := waitDirOrContent(snk, root, "notes/r1-renamed", 10*time.Second)
	_ = os.RemoveAll(renDir)
	ok3 := waitDirOrContent(snk, root, "notes/r1-renamed", 10*time.Second) // remove event may mark the dir or its file
	r.s3["mkdir+file"] = ok1
	r.s3["dirRename"] = ok2
	r.s3["dirRemove"] = ok3

	// ---- S4: write inside node_modules must not surface (or be filtered).
	mu.Lock()
	snk.m = map[string]time.Time{}
	mu.Unlock()
	nmTarget := filepath.Join(root, "code-repo", "node_modules", "pkg-00", "lib", "s4-probe.js")
	before := snkCountFiltered(snk)
	_ = atomicWrite(nmTarget, []byte("probe"))
	time.Sleep(1500 * time.Millisecond)
	after := snkCountFiltered(snk)
	_ = before
	_ = after
	// count events under ignored subtrees accumulated by collector instead
}

// waitDirOrContent waits until the dir itself or any path under it was
// reported (for hints/scan the file inside may be what shows up).
func waitDirOrContent(snk *mapSink, root, rel string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		snkMuSafe := func() bool {
			for k := range snk.m {
				if k == rel || strings.HasPrefix(k, rel+"/") {
					return true
				}
			}
			return false
		}
		if snkMuSafe() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func snkCountFiltered(s *mapSink) int { return s.count() }

func runtime_Gosched_freeOSMemory() {}

// ---- HintsOnly snapshot diff --------------------------------------------

// vaultSnap keeps path -> (mtime,size,isDir) for everything not ignored.
type vaultSnap struct {
	root  string
	mu    sync.Mutex
	state map[string]snapEnt
}

type snapEnt struct {
	mtime time.Time
	size  int64
	dir   bool
	gone  bool
}

func newVaultSnap(root string) *vaultSnap {
	return &vaultSnap{root: root, state: map[string]snapEnt{}}
}

func (v *vaultSnap) scan() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.state = map[string]snapEnt{}
	_ = filepath.WalkDir(v.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel := relTo(v.root, p)
		if d.IsDir() && (filepath.Base(p) == "node_modules" || filepath.Base(p) == ".git") {
			return filepath.SkipDir
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		v.state[rel] = snapEnt{mtime: info.ModTime(), size: info.Size(), dir: d.IsDir()}
		return nil
	})
}

// diff rescan the vault (bounded: we rescan the hinted dirs only when
// they name a real directory; "." means rescan everything) and returns
// relative paths that changed vs the snapshot. This is O(entries in the
// hinted subtree); the 8192-entry limit doesn't apply because we keep no
// watch snapshot — this map is ours.
func (v *vaultSnap) diff(evs []mygo.FileEvent) []string {
	v.mu.Lock()
	defer v.mu.Unlock()
	roots := map[string]bool{}
	for _, e := range evs {
		if e.Op != mygo.FileHint {
			// FileRemove "." etc.
			if e.Path != "" {
				roots[e.Path] = true
			}
			continue
		}
		if e.Path == "." {
			roots["."] = true
		} else {
			roots[e.Path] = true
		}
	}
	if len(roots) == 0 {
		return nil
	}
	var changed []string
	for r := range roots {
		var scanRoot string
		if r == "." {
			scanRoot = v.root
		} else {
			scanRoot = filepath.Join(v.root, filepath.FromSlash(r))
			// if the hinted dir itself is gone, flag it
			if st, err := os.Stat(scanRoot); err != nil || !st.IsDir() {
				if _, ok := v.state[r]; ok {
					delete(v.state, r)
					changed = append(changed, r)
				}
				// also drop everything under it
				prefix := r + "/"
				for k := range v.state {
					if strings.HasPrefix(k, prefix) {
						delete(v.state, k)
						changed = append(changed, k)
					}
				}
				continue
			}
		}
		seenNow := map[string]bool{}
		_ = filepath.WalkDir(scanRoot, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			rel := relTo(v.root, p)
			if d.IsDir() && (filepath.Base(p) == "node_modules" || filepath.Base(p) == ".git") {
				return filepath.SkipDir
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			seenNow[rel] = true
			cur := snapEnt{mtime: info.ModTime(), size: info.Size(), dir: d.IsDir()}
			old, ok := v.state[rel]
			if !ok || old.mtime != cur.mtime || old.size != cur.size || old.dir != cur.dir {
				v.state[rel] = cur
				changed = append(changed, rel)
			}
			return nil
		})
		// anything under scanRoot previously present but missing now = removed
		prefix := relTo(v.root, scanRoot)
		if prefix != "." {
			prefix += "/"
		}
		for k := range v.state {
			if k == "." {
				continue
			}
			if prefix == "./" || k == strings.TrimSuffix(prefix, "/") || strings.HasPrefix(k, prefix) {
				if !seenNow[k] {
					delete(v.state, k)
					changed = append(changed, k)
				}
			}
		}
	}
	return changed
}

func printResult(r *result) {
	fmt.Printf("\n==== %s ====\n", r.mode)
	if r.startErr != nil {
		fmt.Printf("start_error: %v\n", r.startErr)
		return
	}
	fmt.Printf("watch_setup: %s\n", fmtDur(r.watchSetup))
	fmt.Printf("go_heap_after_setup: %s\n", fmtMB(r.heapAfter))
	fmt.Printf("process_rss_after_setup: %s\n", fmtMB(r.rssAfter))
	fmt.Printf("idle_cpu_30s: %.2f%%\n", r.idleCPUPer)
	if len(r.s1) > 0 {
		fmt.Printf("S1 atomic-write latency n=%d p50=%s p95=%s max=%s miss=%d\n",
			len(r.s1), fmtDur(percentile(r.s1, 0.5)), fmtDur(percentile(r.s1, 0.95)),
			fmtDur(percentile(r.s1, 1.0)), r.s1Miss)
	} else {
		fmt.Printf("S1 no successful samples (miss=%d)\n", r.s1Miss)
	}
	fmt.Printf("S2 burst-1000: total=%s seen=%d lost=%d\n", fmtDur(r.s2Total), r.s2Seen, r.s2Lost)
	keys := make([]string, 0, len(r.s3))
	for k := range r.s3 {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("S3 %s: %v\n", k, r.s3[k])
	}
	fmt.Printf("S4 events inside ignored subtrees: %d\n", r.s4Events)
	for _, n := range r.notes {
		fmt.Println("note:", n)
	}
}
