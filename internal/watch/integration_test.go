package watch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestMygoWatchIntegration exercises the real mygo.WatchFiles backend
// against a temp directory. It only runs when MKB_WATCH_IT=1 is set,
// because WatchFiles needs the mygo application runtime (needsApp): the
// backend libraries are initialized by App.Run. A plain `go test` cannot
// bring the app up, so the real-window behaviour is verified in the e2e
// stage instead (docs/PLAN-M0.md T10, V-WATCH-1). When the env var is set
// inside a harness that did start the app, this test performs S1/S3-style
// smoke checks end to end.
func TestMygoWatchIntegration(t *testing.T) {
	if os.Getenv("MKB_WATCH_IT") != "1" {
		t.Skip("mygo.WatchFiles needs the app runtime (needsApp); " +
			"set MKB_WATCH_IT=1 inside a harness that ran App.Run, " +
			"otherwise verify in e2e (V-WATCH-1). See spikes/r1-watch.")
	}

	root := t.TempDir()
	for _, d := range []string{"notes", "code-repo/node_modules/pkg", "code-repo/.git/objects"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	got := make(chan []string, 8)
	w, err := New(MygoBackend{}, root, func(dirs []string) { got <- dirs })
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	// Give the backend a moment to enrol the tree.
	time.Sleep(300 * time.Millisecond)

	write := func(rel string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("notes/a.md")
	write("code-repo/src/x.ts")              // watched (code-repo itself is not ignored)
	write("code-repo/node_modules/pkg/i.js") // ignored subtree
	write("code-repo/.git/objects/aa")       // ignored subtree

	deadline := time.After(3 * time.Second)
	seen := map[string]bool{}
	for {
		select {
		case dirs := <-got:
			for _, d := range dirs {
				seen[d] = true
			}
			if seen["notes"] && seen["code-repo/src"] {
				for bad := range seen {
					if bad == "code-repo/node_modules" || bad == "code-repo/node_modules/pkg" ||
						bad == "code-repo/.git" || bad == "code-repo/.git/objects" {
						t.Fatalf("ignored subtree leaked into dirs: %v", seen)
					}
				}
				cancel()
				if err := <-done; err != nil && err != context.Canceled {
					t.Fatalf("Run: %v", err)
				}
				return
			}
		case <-deadline:
			t.Fatalf("timed out waiting for notes + code-repo/src; got %v", seen)
		}
	}
}
