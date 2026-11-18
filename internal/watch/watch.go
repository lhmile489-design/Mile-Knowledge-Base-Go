// Package watch watches a knowledge-base directory for external changes and
// reports the affected directories in debounced batches.
//
// The heavy lifting is delegated to a Backend (mygo.WatchFiles in
// production). This package adds the vault-facing policy on top: the
// node_modules/.git ignore rule, the event-to-directory merge, a bounded
// second-stage coalescing window, and the ErrWatchLimit fallback to
// HintsOnly mode. It is deliberately free of UI and index concerns: callers
// receive sorted, slash-separated directory paths relative to the vault
// root ("" is the root itself) and decide what to refresh.
//
// Design basis: docs/技术设计.md §4.2 and spikes/r1-watch/RESULT.md (R-1,
// mode a with mode b as the limit fallback).
package watch

import (
	"context"
	"errors"
	"io"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
)

// Backend abstracts the platform watcher so tests can inject a fake.
type Backend interface {
	// Watch establishes the watch and returns once everything is
	// enrolled, mirroring mygo.WatchFiles.
	Watch(ctx context.Context, root string, opts Options) (Stream, error)
}

// Options carries the watch configuration to a Backend. Field meanings
// match mygo.FileWatchOptions.
type Options struct {
	Recursive  bool
	Ignore     func(path string, isDir bool) bool
	MaxEntries int
	HintsOnly  bool
	Debounce   time.Duration
	MaxDelay   time.Duration
}

// Stream is a batch source of events, mirroring *mygo.FileWatcher.
// Next blocks until a debounced batch is ready; it returns io.EOF once
// the watch ended cleanly or the error that stopped it.
type Stream interface {
	Next(ctx context.Context) ([]Event, error)
	Close() error
}

// Event is one filesystem change. Op is create | write | remove | rename
// (mygo.FileOp values); a Backend may also report the mygo-only "hint" op,
// which is treated as a change to the directory at Path.
type Event struct {
	Op      string
	Path    string
	OldPath string
	IsDir   bool
}

const (
	opCreate = "create"
	opWrite  = "write"
	opRemove = "remove"
	opRename = "rename"
	opHint   = "hint"
)

// ErrWatchLimit is reported by Backend.Watch (and by Stream.Next) when the
// tree exceeds Options.MaxEntries. The package copies mygo's sentinel as a
// variable so callers may errors.Is against either; MygoBackend wires them
// together. Fakes return this sentinel.
var ErrWatchLimit = errors.New("watch: entry limit exceeded")

const (
	defaultMaxEntries = 65536 // R-1: 4x headroom over a 15k-entry vault
	defaultDebounce   = 50 * time.Millisecond
	defaultMaxDelay   = 500 * time.Millisecond

	// mergeWindow is the second-stage coalescing window. mygo already
	// debounces into batches; several batches can still arrive back to
	// back while a slow sink drains the queue, so consecutive batches are
	// merged for up to this long. Bounded to stay under the PRD's 1s
	// external-change budget, and to bound how long a burst can postpone
	// a flush.
	mergeWindow = 100 * time.Millisecond

	// sinkQueueSize bounds the batches waiting for the sink. The drain
	// goroutine always keeps up unless the sink itself blocks; a large
	// queue just delays the point where Run applies backpressure.
	sinkQueueSize = 64
)

// Watcher merges watch events into affected-directory batches and feeds
// them to a sink without letting a slow sink stall event consumption.
type Watcher struct {
	backend Backend
	root    string
	opts    Options
	sink    func(dirs []string)

	mu       sync.Mutex
	stream   Stream
	degraded bool
	closed   bool
	running  bool

	queue chan []Event // batches waiting for the drain goroutine
}

// New prepares a watcher on root. The watch itself is established lazily
// inside Run, where the real ErrWatchLimit → HintsOnly fallback happens;
// New exists so the caller can hold the watcher before choosing the
// lifecycle context.
func New(b Backend, root string, sink func(dirs []string)) (*Watcher, error) {
	if b == nil {
		return nil, errors.New("watch: nil backend")
	}
	if root == "" {
		return nil, errors.New("watch: empty root")
	}
	if sink == nil {
		return nil, errors.New("watch: nil sink")
	}
	w := &Watcher{
		backend: b,
		root:    root,
		sink:    sink,
		opts: Options{
			Recursive:  true,
			Ignore:     Ignore,
			MaxEntries: defaultMaxEntries,
			Debounce:   defaultDebounce,
			MaxDelay:   defaultMaxDelay,
		},
		queue: make(chan []Event, sinkQueueSize),
	}
	return w, nil
}

// Ignore is the vault-wide ignore rule (PRD FR-AREA, R-1): directories
// named node_modules or .git are never watched, at any depth. .mile-kb is
// deliberately NOT ignored — the tree shows it; the app's own writes into
// it are filtered upstairs by revision comparison (docs/技术设计.md §4.2).
// The path arrives slash-separated, relative to the root, as mygo passes
// it to FileWatchOptions.Ignore.
func Ignore(p string, isDir bool) bool {
	if !isDir {
		return false
	}
	base := p
	if i := strings.LastIndexByte(base, '/'); i >= 0 {
		base = base[i+1:]
	}
	return base == "node_modules" || base == ".git"
}

// Degraded reports whether the watch fell back to HintsOnly after the
// backend refused the normal watch with ErrWatchLimit (huge vault). In
// that mode only directory hints exist, so every batch is reported as the
// root ("") and the consumer must rescan what it cares about.
func (w *Watcher) Degraded() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.degraded
}

// Run establishes the watch and consumes events until ctx ends, the
// backend stops the stream, or an error ends the watch. It must be called
// at most once; the second call returns an error. A nil error (or ctx's
// own error) is returned when the watch ended via ctx cancel or io.EOF;
// any backend error is propagated.
func (w *Watcher) Run(ctx context.Context) error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return errors.New("watch: closed")
	}
	if w.running {
		w.mu.Unlock()
		return errors.New("watch: Run called twice")
	}
	w.running = true
	w.mu.Unlock()

	stream, err := w.establish(ctx)
	if err != nil {
		return err
	}
	w.mu.Lock()
	w.stream = stream
	if w.closed {
		w.mu.Unlock()
		stream.Close()
		return errors.New("watch: closed")
	}
	w.mu.Unlock()

	defer func() {
		stream.Close()
		w.mu.Lock()
		w.stream = nil
		w.mu.Unlock()
	}()

	// The drain goroutine owns sink calls: Run never blocks on the sink,
	// so a 50ms+ sink cannot stall Next. It also applies the second-stage
	// merge window. On Run's return the queue is closed, which flushes
	// pending events and lets the goroutine exit on its own.
	drainExited := make(chan struct{})
	go w.drain(drainExited)
	defer func() {
		close(w.queue)
		<-drainExited
	}()

	for {
		batch, err := stream.Next(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return ctx.Err() // nil when the stream ended on its own
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		if len(batch) == 0 {
			continue
		}
		select {
		case w.queue <- batch:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// establish tries the normal watch first; on ErrWatchLimit it retries
// with HintsOnly (docs/技术设计.md §4.2 degraded path).
func (w *Watcher) establish(ctx context.Context) (Stream, error) {
	stream, err := w.backend.Watch(ctx, w.root, w.opts)
	if err == nil {
		return stream, nil
	}
	if !errors.Is(err, ErrWatchLimit) {
		return nil, err
	}
	degraded := w.opts
	degraded.HintsOnly = true
	// HintsOnly counts enrolled directories, not files; the same
	// MaxEntries is generous headroom either way.
	stream, err = w.backend.Watch(ctx, w.root, degraded)
	if err != nil {
		return nil, err
	}
	w.mu.Lock()
	w.degraded = true
	w.mu.Unlock()
	return stream, nil
}

// drain merges queued batches and calls the sink, one batch at a time.
// The queue is closed by Run's exit path, which flushes what is pending
// before the goroutine returns. The merge window bounds how long
// consecutive batches coalesce: the first event of a cluster starts a
// timer; batches keep merging until it fires.
func (w *Watcher) drain(done chan<- struct{}) {
	defer close(done)
	dirty := map[string]bool{}
	have := false
	window := time.NewTimer(mergeWindow)
	if !window.Stop() {
		<-window.C
	}
	var windowC <-chan time.Time

	flush := func() {
		if !have {
			return
		}
		dirs := make([]string, 0, len(dirty))
		for d := range dirty {
			dirs = append(dirs, d)
		}
		sort.Strings(dirs)
		clear(dirty)
		have = false
		windowC = nil
		w.sink(dirs)
	}

	for {
		select {
		case batch, ok := <-w.queue:
			if !ok {
				flush()
				return
			}
			if !have {
				window.Reset(mergeWindow)
				windowC = window.C
			}
			for _, ev := range batch {
				for _, d := range dirsOf(ev) {
					dirty[d] = true
				}
			}
			have = len(dirty) > 0
		case <-windowC:
			windowC = nil
			flush()
		}
	}
}

// dirsOf maps an event to the directories it affects: a file's parent,
// and a directory's parent plus itself; a rename counts both endpoints.
// Paths arrive slash-separated relative to the root ("." is the root).
// The result uses "" for the root directory.
func dirsOf(ev Event) []string {
	seen := map[string]bool{}
	var out []string
	add := func(rel string) {
		rel = cleanRel(rel)
		if !seen[rel] {
			seen[rel] = true
			out = append(out, rel)
		}
	}

	// In HintsOnly a hint names the directory that changed; report the
	// root so the consumer rescans. A hint at "." is the overflow case
	// ("look at everything"), also the root.
	if ev.Op == opHint {
		add("")
		return out
	}

	p := cleanRel(ev.Path)
	if p == "" {
		// The watched root itself changed (e.g. removed or renamed):
		// everything is affected.
		add("")
	} else {
		add(parentOf(p))
		if ev.IsDir {
			add(p)
		}
	}
	if ev.Op == opRename && ev.OldPath != "" {
		o := cleanRel(ev.OldPath)
		if o == "" {
			add("")
		} else {
			add(parentOf(o))
			if ev.IsDir {
				add(o)
			}
		}
	}
	return out
}

// cleanRel normalises a slash-separated watch-relative path: "." and ""
// become "" (the root), slashes are cleaned, leading "./" is dropped.
func cleanRel(p string) string {
	p = strings.TrimSpace(p)
	if p == "" || p == "." {
		return ""
	}
	p = path.Clean("/" + p)[1:] // keep it relative, collapse ../ and //
	if p == "." {
		return ""
	}
	return p
}

// parentOf returns the parent directory of a non-root relative path,
// "" for top-level entries.
func parentOf(rel string) string {
	if i := strings.LastIndexByte(rel, '/'); i >= 0 {
		return rel[:i]
	}
	return ""
}

// Close stops the watcher. It is idempotent. If Run is blocked inside a
// backend Next it stays there until the stream observes the close — the
// caller should cancel the Run context for a prompt stop.
func (w *Watcher) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	stream := w.stream
	w.mu.Unlock()
	if stream != nil {
		return stream.Close()
	}
	return nil
}
