package watch

import (
	"context"
	"errors"
	"io"
	"sort"
	"sync"
	"testing"
	"time"
)

// fakeBackend feeds scripted batches to the stream it returns.
type fakeBackend struct {
	mu       sync.Mutex
	calls    []Options     // Watch options per call, for fallback assertions
	failNext error         // returned by the next Watch call, then cleared
	streams  []*fakeStream // every stream handed out
}

func (f *fakeBackend) Watch(ctx context.Context, root string, opts Options) (Stream, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, opts)
	if f.failNext != nil {
		err := f.failNext
		f.failNext = nil
		return nil, err
	}
	s := &fakeStream{ctx: ctx}
	f.streams = append(f.streams, s)
	return s, nil
}

// fakeStream is a Stream whose Next blocks on a channel.
type fakeStream struct {
	ctx     context.Context
	mu      sync.Mutex
	pending [][]Event
	wake    chan struct{} // signalled on push
	closed  bool
	err     error // terminal error once pending drains
}

func (s *fakeStream) push(ev ...Event) {
	s.mu.Lock()
	s.pending = append(s.pending, ev)
	select {
	case s.wake <- struct{}{}:
	default:
	}
	s.mu.Unlock()
}

func (s *fakeStream) finish(err error) {
	s.mu.Lock()
	s.err = err
	select {
	case s.wake <- struct{}{}:
	default:
	}
	s.mu.Unlock()
}

func (s *fakeStream) Next(ctx context.Context) ([]Event, error) {
	s.mu.Lock()
	if s.wake == nil {
		s.wake = make(chan struct{}, 1)
	}
	for {
		if len(s.pending) > 0 {
			b := s.pending[0]
			s.pending = s.pending[1:]
			s.mu.Unlock()
			return b, nil
		}
		if s.err != nil {
			err := s.err
			s.mu.Unlock()
			return nil, err
		}
		if s.closed {
			s.mu.Unlock()
			return nil, io.EOF
		}
		wake := s.wake
		s.mu.Unlock()
		select {
		case <-wake:
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-s.ctx.Done():
			return nil, s.ctx.Err()
		}
		s.mu.Lock()
	}
}

func (s *fakeStream) Close() error {
	s.mu.Lock()
	s.closed = true
	if s.err == nil {
		s.err = io.EOF
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
	s.mu.Unlock()
	return nil
}

// recorder collects sink calls.
type recorder struct {
	mu    sync.Mutex
	batch [][]string
}

func (r *recorder) sink(dirs []string) {
	r.mu.Lock()
	r.batch = append(r.batch, dirs)
	r.mu.Unlock()
}

func (r *recorder) waitFor(t *testing.T, n int, d time.Duration) [][]string {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		if len(r.batch) >= n {
			out := append([][]string(nil), r.batch...)
			r.mu.Unlock()
			return out
		}
		r.mu.Unlock()
		time.Sleep(2 * time.Millisecond)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	t.Fatalf("sink called %d times, want >= %d; got %v", len(r.batch), n, r.batch)
	return nil
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.batch)
}

func start(t *testing.T, b *fakeBackend, rec *recorder) (context.Context, context.CancelFunc, *Watcher, chan error) {
	t.Helper()
	w, err := New(b, "/vault", rec.sink)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	streamOf(t, b) // ensure the watch is established before returning
	return ctx, cancel, w, done
}

func streamOf(t *testing.T, b *fakeBackend) *fakeStream {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		b.mu.Lock()
		if len(b.streams) > 0 {
			s := b.streams[len(b.streams)-1]
			b.mu.Unlock()
			return s
		}
		b.mu.Unlock()
		time.Sleep(time.Millisecond)
	}
	t.Fatal("backend never produced a stream")
	return nil
}

func assertDirs(t *testing.T, got []string, want ...string) {
	t.Helper()
	sort.Strings(want)
	if !equalStrings(got, want) {
		t.Fatalf("dirs = %v, want %v", got, want)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ---- cases ----

func TestMergeWithinOneBatch(t *testing.T) {
	b := &fakeBackend{}
	rec := &recorder{}
	_, cancel, _, _ := start(t, b, rec)
	defer cancel()

	s := streamOf(t, b)
	s.push(
		Event{Op: opWrite, Path: "notes/a.md"},
		Event{Op: opWrite, Path: "notes/b.md"},
		Event{Op: opCreate, Path: "docs/x.md"},
	)
	got := rec.waitFor(t, 1, time.Second)
	assertDirs(t, got[0], "docs", "notes")
}

func TestMergeAcrossBatchesWithinWindow(t *testing.T) {
	b := &fakeBackend{}
	rec := &recorder{}
	_, cancel, _, _ := start(t, b, rec)
	defer cancel()

	s := streamOf(t, b)
	s.push(Event{Op: opWrite, Path: "a/1.md"})
	s.push(Event{Op: opWrite, Path: "b/2.md"})
	s.push(Event{Op: opWrite, Path: "a/3.md"})
	got := rec.waitFor(t, 1, time.Second)
	assertDirs(t, got[0], "a", "b")
	if n := rec.count(); n != 1 {
		t.Fatalf("sink calls = %d, want 1 (second-stage merge)", n)
	}
}

func TestIgnoreNodeModulesAndGit(t *testing.T) {
	b := &fakeBackend{}
	rec := &recorder{}
	_, cancel, _, _ := start(t, b, rec)
	defer cancel()

	// The backend would normally apply Ignore itself; the watcher must
	// pass the rule through. Verify the rule on the recorded options.
	b.mu.Lock()
	ignore := b.calls[0].Ignore
	b.mu.Unlock()
	if ignore == nil {
		t.Fatal("Ignore not forwarded to backend")
	}
	cases := []struct {
		path  string
		isDir bool
		want  bool
	}{
		{"node_modules", true, true},
		{"x/node_modules", true, true},
		{"x/y/node_modules", true, true},
		{".git", true, true},
		{"repo/.git", true, true},
		{"repo/sub/.git", true, true},
		{".mile-kb", true, false},      // watched: tree shows it (FR-AREA)
		{".claude", true, false},       // watched
		{"node_modules", false, false}, // a FILE named node_modules: not ignored
		{"notes/node_modules.md", false, false},
		{"git", true, false}, // "git" != ".git"
		{"my.git", true, false},
	}
	for _, c := range cases {
		if got := ignore(c.path, c.isDir); got != c.want {
			t.Fatalf("Ignore(%q, %v) = %v, want %v", c.path, c.isDir, got, c.want)
		}
	}
}

func TestDirectoryRenameCountsBoth(t *testing.T) {
	b := &fakeBackend{}
	rec := &recorder{}
	_, cancel, _, _ := start(t, b, rec)
	defer cancel()

	s := streamOf(t, b)
	s.push(Event{Op: opRename, Path: "docs/new", OldPath: "docs/old", IsDir: true})
	got := rec.waitFor(t, 1, time.Second)
	assertDirs(t, got[0], "docs", "docs/new", "docs/old")
}

func TestFileRenameCountsBothParents(t *testing.T) {
	b := &fakeBackend{}
	rec := &recorder{}
	_, cancel, _, _ := start(t, b, rec)
	defer cancel()

	s := streamOf(t, b)
	s.push(Event{Op: opRename, Path: "a/f.md", OldPath: "b/f.md"})
	got := rec.waitFor(t, 1, time.Second)
	assertDirs(t, got[0], "a", "b")
}

func TestDeepRelativePaths(t *testing.T) {
	b := &fakeBackend{}
	rec := &recorder{}
	_, cancel, _, _ := start(t, b, rec)
	defer cancel()

	s := streamOf(t, b)
	s.push(Event{Op: opWrite, Path: "x/y/z/deep.md"})
	got := rec.waitFor(t, 1, time.Second)
	assertDirs(t, got[0], "x/y/z")
}

func TestRootItselfAndRootLevelFiles(t *testing.T) {
	b := &fakeBackend{}
	rec := &recorder{}
	_, cancel, _, _ := start(t, b, rec)
	defer cancel()

	s := streamOf(t, b)
	// A root-level file's parent is "" (the root).
	s.push(Event{Op: opCreate, Path: "README.md"})
	// A directory at the top level contributes its parent ("") and itself.
	s.push(Event{Op: opCreate, Path: "newdir", IsDir: true})
	got := rec.waitFor(t, 1, time.Second)
	assertDirs(t, got[0], "", "newdir")
}

func TestDotPathMeansRoot(t *testing.T) {
	b := &fakeBackend{}
	rec := &recorder{}
	_, cancel, _, _ := start(t, b, rec)
	defer cancel()

	s := streamOf(t, b)
	s.push(Event{Op: opRemove, Path: "."})
	got := rec.waitFor(t, 1, time.Second)
	assertDirs(t, got[0], "")
}

func TestErrorPropagation(t *testing.T) {
	b := &fakeBackend{}
	rec := &recorder{}
	_, cancel, _, done := start(t, b, rec)
	defer cancel()

	s := streamOf(t, b)
	want := errors.New("backend exploded")
	s.finish(want)
	select {
	case err := <-done:
		if !errors.Is(err, want) {
			t.Fatalf("Run = %v, want %v", err, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after backend error")
	}
}

func TestCtxCancelExits(t *testing.T) {
	b := &fakeBackend{}
	rec := &recorder{}
	ctx, cancel, _, done := start(t, b, rec)

	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after ctx cancel")
	}
	_ = ctx
}

func TestEOFOnRootGone(t *testing.T) {
	b := &fakeBackend{}
	rec := &recorder{}
	_, cancel, _, done := start(t, b, rec)
	defer cancel()

	s := streamOf(t, b)
	s.finish(io.EOF) // mygo reports io.EOF when the root directory goes away
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run = %v, want nil after io.EOF", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after io.EOF")
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	b := &fakeBackend{}
	rec := &recorder{}
	w, err := New(b, "/vault", rec.sink)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	// Run after Close reports closed.
	if err := w.Run(context.Background()); err == nil {
		t.Fatal("Run after Close returned nil")
	}
}

func TestRunTwiceFails(t *testing.T) {
	b := &fakeBackend{}
	rec := &recorder{}
	w, err := New(b, "/vault", rec.sink)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	streamOf(t, b)
	if err := w.Run(ctx); err == nil {
		t.Fatal("second Run returned nil")
	}
	cancel()
	<-done
}

func TestErrWatchLimitFallsBackToHintsOnly(t *testing.T) {
	b := &fakeBackend{failNext: ErrWatchLimit}
	rec := &recorder{}
	w, err := New(b, "/vault", rec.sink)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()

	streamOf(t, b) // second stream = degraded watch established

	if !w.Degraded() {
		t.Fatal("Degraded() = false after ErrWatchLimit fallback")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.calls) != 2 {
		t.Fatalf("backend Watch calls = %d, want 2", len(b.calls))
	}
	if b.calls[0].HintsOnly {
		t.Fatal("first call unexpectedly HintsOnly")
	}
	if !b.calls[1].HintsOnly {
		t.Fatal("fallback call not HintsOnly")
	}
	if b.calls[1].MaxEntries != b.calls[0].MaxEntries {
		t.Fatal("fallback changed MaxEntries")
	}
}

func TestDegradedHintsReportRoot(t *testing.T) {
	b := &fakeBackend{failNext: ErrWatchLimit}
	rec := &recorder{}
	ctx, cancel, w, _ := start(t, b, rec)
	defer cancel()
	_ = ctx
	if !w.Degraded() {
		t.Fatal("not degraded")
	}

	s := streamOf(t, b)
	s.push(Event{Op: opHint, Path: "some/dir"})
	got := rec.waitFor(t, 1, time.Second)
	assertDirs(t, got[0], "") // hint → root → full rescan upstream
}

func TestErrWatchLimitOnFallbackIsPropagated(t *testing.T) {
	rec := &recorder{}
	b := &sequencedBackend{errs: []error{ErrWatchLimit, errors.New("still too big")}}
	w, err := New(b, "/vault", rec.sink)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	err = w.Run(context.Background())
	if err == nil || err.Error() != "still too big" {
		t.Fatalf("Run = %v, want 'still too big'", err)
	}
}

// sequencedBackend returns one error per Watch call, then a stream.
type sequencedBackend struct {
	mu   sync.Mutex
	errs []error
	i    int
}

func (s *sequencedBackend) Watch(ctx context.Context, root string, opts Options) (Stream, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.i < len(s.errs) {
		err := s.errs[s.i]
		s.i++
		return nil, err
	}
	return &fakeStream{ctx: ctx}, nil
}

// ---- stress: burst of 1000 file events (git checkout scale, R-1 S2) ----

func TestBurstThousandEvents(t *testing.T) {
	b := &fakeBackend{}
	rec := &recorder{}
	_, cancel, _, _ := start(t, b, rec)
	defer cancel()

	s := streamOf(t, b)
	const files = 1000
	// Simulate mygo's debounced batches arriving back to back while the
	// vault is rewritten (e.g. git checkout): several medium batches.
	for i := 0; i < files; {
		n := 50
		batch := make([]Event, 0, n)
		for j := 0; j < n && i < files; j++ {
			batch = append(batch, Event{
				Op:   opWrite,
				Path: "bulk/dir" + itoa(i%10) + "/f" + itoa(i) + ".md",
			})
			i++
		}
		s.push(batch...)
	}

	// The measured interval starts once the burst is fully queued: the
	// watcher must merge and report everything well under 200ms of work —
	// the merge window itself is the only deliberate wait.
	start := time.Now()
	deadline := time.Now().Add(2 * time.Second)
	var got [][]string
	for time.Now().Before(deadline) {
		got = rec.waitFor(t, 1, 200*time.Millisecond)
		seen := map[string]bool{}
		for _, batch := range got {
			for _, d := range batch {
				seen[d] = true
			}
		}
		if len(seen) == 10 {
			break
		}
	}
	elapsed := time.Since(start)

	// Sink call budget: the burst must coalesce well under 10 calls.
	if n := rec.count(); n > 10 {
		t.Fatalf("sink calls = %d, want <= 10", n)
	}
	// All ten directories are covered.
	seen := map[string]bool{}
	for _, batch := range got {
		for _, d := range batch {
			seen[d] = true
		}
	}
	for i := 0; i < 10; i++ {
		want := "bulk/dir" + itoa(i)
		if !seen[want] {
			t.Fatalf("missing dir %q; batches %v", want, got)
		}
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("burst settle took %v, want < 200ms", elapsed)
	}
}

// itoa avoids strconv import noise for tiny numbers.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [4]byte
	p := len(buf)
	for i > 0 {
		p--
		buf[p] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[p:])
}

// TestSinkDoesNotBlockEventLoop: a sink slower than 50ms must not stall
// consumption of subsequent batches.
func TestSlowSinkDoesNotStallNext(t *testing.T) {
	b := &fakeBackend{}
	var mu sync.Mutex
	var calls int
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	slow := func(dirs []string) {
		mu.Lock()
		calls++
		first := calls == 1
		mu.Unlock()
		if first {
			entered <- struct{}{}
			<-release // first sink call parks; second must still arrive
		}
	}
	w, err := New(b, "/vault", slow)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	s := streamOf(t, b)

	s.push(Event{Op: opWrite, Path: "a/1.md"})
	// Wait for the merge window so the first flush actually reaches the
	// sink, then it parks on release.
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("sink never entered")
	}
	// More batches must still be pulled off Next while the sink blocks.
	for i := 0; i < 8; i++ {
		s.push(Event{Op: opWrite, Path: "b/" + itoa(i) + ".md"})
	}
	// Let the Run loop drain the queued batches, then release the sink.
	time.Sleep(30 * time.Millisecond)
	close(release)
	time.Sleep(mergeWindow + 150*time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if calls < 2 {
		t.Fatalf("sink calls = %d, want >= 2 after release", calls)
	}
	cancel()
	<-done
}
