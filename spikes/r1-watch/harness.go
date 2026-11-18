package main

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// harness.go: shared measurement helpers and the S1–S4 scenarios.
//
// An event sink is anything that can tell us, for a relative vault path,
// when it was reported changed (nanos, first report) and whether it was
// reported at all.

type sink interface {
	// seen records that rel was reported at time t.
	seen(rel string, t time.Time)
	// seenAt returns when rel was first reported, ok=false if never.
	seenAt(rel string) (time.Time, bool)
	// count of all events seen so far (for diagnostics).
	count() int
}

type mapSink struct {
	m     map[string]time.Time
	total int
}

func newMapSink() *mapSink { return &mapSink{m: map[string]time.Time{}} }

func (s *mapSink) seen(rel string, t time.Time) {
	rel = filepath.ToSlash(rel)
	s.total++
	if _, ok := s.m[rel]; !ok {
		s.m[rel] = t
	}
}
func (s *mapSink) seenAt(rel string) (time.Time, bool) {
	t, ok := s.m[filepath.ToSlash(rel)]
	return t, ok
}
func (s *mapSink) count() int { return s.total }

// rss returns the process working set in bytes via runtime/metrics? On
// Windows Go doesn't expose RSS through runtime.ReadMemStats. We read
// MemStats for the Go heap and use a tiny syscall-free approach for RSS:
// GetProcessMemoryInfo via golang.org/x/sys/windows would be ideal; to
// keep the spike dependency-light we shell nothing out and instead read
// /proc — unavailable on Windows. So RSS is measured with
// windows.GetProcessMemoryInfo in rss_windows.go.
func goHeap() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

// cpuSample reads process CPU time (kernel+user) in nanoseconds.
// Implemented in cpu_windows.go.
// func procCPU() int64

func percentile(vs []time.Duration, p float64) time.Duration {
	if len(vs) == 0 {
		return 0
	}
	s := append([]time.Duration(nil), vs...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	k := int(float64(len(s)-1) * p)
	return s[k]
}

// atomicWrite writes content to dir/.tmp-name then renames it to name,
// like an editor's atomic save.
func atomicWrite(path string, content []byte) error {
	dir := filepath.Dir(path)
	tmp := filepath.Join(dir, ".r1tmp-"+filepath.Base(path))
	if err := os.WriteFile(tmp, content, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// waitSeen polls sink until rel is reported or timeout; returns latency.
func waitSeen(s sink, rel string, timeout time.Duration) (time.Duration, bool) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if t, ok := s.seenAt(rel); ok {
			_ = t
			return 0, true
		}
		time.Sleep(2 * time.Millisecond)
	}
	return 0, false
}

// waitSeenFrom polls until rel reported with timestamp >= start; latency
// measured by the caller against start (we return the sink's stamp).
func waitSeenFrom(s sink, rel string, start time.Time, timeout time.Duration) (time.Time, bool) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if t, ok := s.seenAt(rel); ok && !t.Before(start) {
			return t, true
		}
		time.Sleep(2 * time.Millisecond)
	}
	return time.Time{}, false
}

func relTo(root, p string) string {
	r, err := filepath.Rel(root, p)
	if err != nil {
		return filepath.ToSlash(p)
	}
	return filepath.ToSlash(r)
}

// pickTargets chooses n md files spread across dirs for S1 latency, and
// n files for S2 burst.
func pickTargets(root string, rnd *rand.Rand, n int) []string {
	var files []string
	_ = filepath.Walk(filepath.Join(root, "notes"), func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(p, ".md") {
			files = append(files, p)
		}
		return nil
	})
	if len(files) == 0 {
		return nil
	}
	step := len(files) / n
	if step < 1 {
		step = 1
	}
	var out []string
	for i := 0; i < n && i*step < len(files); i++ {
		out = append(out, files[i*step])
	}
	return out
}

func isIgnored(rel string) bool {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for _, p := range parts {
		if p == "node_modules" || p == ".git" {
			return true
		}
	}
	return false
}

func fmtDur(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%.1fus", float64(d)/float64(time.Microsecond))
	}
	return fmt.Sprintf("%.2fms", float64(d)/float64(time.Millisecond))
}

func fmtMB(b uint64) string { return fmt.Sprintf("%.1fMiB", float64(b)/1048576) }
