package watch

import (
	"context"
	"errors"

	"github.com/egoist/mygo"
)

// MygoBackend implements Backend with mygo.WatchFiles. WatchFiles needs the
// mygo application running (needsApp), so this backend only works once the
// app is up — call it from a goroutine started in/after App.WhenReady, as
// the service layer does. Tests inject a fake Backend instead.
type MygoBackend struct{}

// Watch maps Options to mygo.FileWatchOptions and starts the watch.
func (MygoBackend) Watch(ctx context.Context, root string, opts Options) (Stream, error) {
	fw, err := mygo.WatchFiles(ctx, root, mygo.FileWatchOptions{
		Recursive:  opts.Recursive,
		Ignore:     opts.Ignore,
		Debounce:   opts.Debounce,
		MaxDelay:   opts.MaxDelay,
		MaxEntries: opts.MaxEntries,
		HintsOnly:  opts.HintsOnly,
	})
	if err != nil {
		// Fold the library sentinel into this package's so callers can
		// errors.Is(err, watch.ErrWatchLimit) without importing mygo.
		if errors.Is(err, mygo.ErrWatchLimit) {
			return nil, errors.Join(ErrWatchLimit, err)
		}
		return nil, err
	}
	return mygoStream{fw: fw}, nil
}

// mygoStream adapts *mygo.FileWatcher to Stream.
type mygoStream struct {
	fw *mygo.FileWatcher
}

func (s mygoStream) Next(ctx context.Context) ([]Event, error) {
	batch, err := s.fw.Next(ctx)
	if err != nil {
		if errors.Is(err, mygo.ErrWatchLimit) {
			err = errors.Join(ErrWatchLimit, err)
		}
		return nil, err
	}
	out := make([]Event, len(batch))
	for i, ev := range batch {
		out[i] = Event{
			Op:      string(ev.Op),
			Path:    ev.Path,
			OldPath: ev.OldPath,
			IsDir:   ev.IsDir,
		}
	}
	return out, nil
}

func (s mygoStream) Close() error { return s.fw.Close() }
