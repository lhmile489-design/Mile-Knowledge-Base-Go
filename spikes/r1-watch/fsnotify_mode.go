package main

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// fsnotify_mode.go: mode c — fsnotify recursive watch.
//
// On Windows, fsnotify supports `path\...` recursion: one
// ReadDirectoryChangesW handle on the root watches the whole tree,
// matching what mygo does with Subtree. We verify it works, and fall
// back to per-directory Add if it doesn't.
//
// fsnotify reports raw OS events without debounce or snapshot diffing;
// for S1 latency we record the first event touching the target path, and
// for S2 we record every path named by an event (create/write/remove or
// rename-from/to). That is the honest comparison point: what arrives is
// raw, the application must debounce/dedupe itself.

func runFSNotify(root string, rnd *rand.Rand) {
	r := &result{mode: "c/fsnotify-recursive", s3: map[string]bool{}}

	runtime_Gosched_freeOSMemory()

	t0 := time.Now()
	w, err := fsnotify.NewWatcher()
	if err != nil {
		r.startErr = err
		printResult(r)
		return
	}
	// fsnotify v1.10.1 public API has no recursion: the path\... syntax is
	// gated behind a test-only flag (enableRecurse, fsnotify.go:503), and
	// verified in this spike's probe: Add(root\...) silently watched only
	// the root's immediate children. So we add every non-ignored
	// directory ourselves. New directories created later also need an Add
	// — we handle that on create events below.
	var dircount, addFailed int
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		base := d.Name()
		if base == "node_modules" || base == ".git" {
			return filepath.SkipDir
		}
		if err := w.Add(p); err != nil {
			addFailed++
			return nil
		}
		dircount++
		return nil
	})
	r.watchSetup = time.Since(t0)
	r.notes = append(r.notes, fmt.Sprintf("per-dir watches=%d failed=%d (no recursive API)", dircount, addFailed))
	defer w.Close()

	snk := newMapSink()
	var mu sync.Mutex
	ignoredEvents := 0

	go func() {
		for {
			select {
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				rel := relTo(root, ev.Name)
				now := time.Now()
				mu.Lock()
				// rename produces two events (old, new); record both
				snk.seen(rel, now)
				if isIgnored(rel) {
					ignoredEvents++
				}
				r.eventsSeen++
				mu.Unlock()
				// New directories need their own Add (no recursive watch).
				// Do it outside mu; Add is serialized on fsnotify's I/O thread.
				if ev.Has(fsnotify.Create) && !isIgnored(rel) {
					if st, err := os.Stat(ev.Name); err == nil && st.IsDir() {
						_ = w.Add(ev.Name)
					}
				}
			case _, ok := <-w.Errors:
				if !ok {
					return
				}
			}
		}
	}()

	r.heapAfter = goHeap()
	r.rssAfter = procRSS()

	runScenarios(root, snk, &mu, r, rnd)

	mu.Lock()
	r.s4Events = ignoredEvents
	mu.Unlock()

	printResult(r)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

var _ = strings.HasPrefix // keep import
