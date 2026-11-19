// Spike R-10: can the host inject the accent color before the page's own
// scripts under the production CSP? See RESULT.md. Modes: default (accent
// injected), nopreload (no PreloadScript), tricky (value with </script> and
// quotes). Everything is served from an in-memory FS; no real app data is
// touched when APPDATA points at a temp dir.
package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
)

//go:embed all:web
var webFiles embed.FS

type Probe struct {
	done chan string
}

// Report receives the page's probe object and prints it as one line.
func (p *Probe) Report(ctx context.Context, payload string) error {
	fmt.Println("PROBE " + payload)
	select {
	case p.done <- payload:
	default:
	}
	return nil
}

func main() {
	mode := os.Getenv("MODE")

	web, err := fs.Sub(webFiles, "web")
	if err != nil {
		fatal(err)
	}
	mygo.SetFrontend(web)

	probe := &Probe{done: make(chan string, 1)}
	mygo.Bind(probe)

	var value any = map[string]any{"preset": "blue"}
	if mode == "tricky" {
		value = map[string]any{
			"preset": "custom",
			"note":   "</script><script>window.__PWNED__=1</script>\"quoted\"    ",
		}
	}
	preload := ""
	if mode != "nopreload" {
		raw, err := json.Marshal(value)
		if err != nil {
			fatal(err)
		}
		preload = "window.__MKB_ACCENT__ = " + string(raw) + "; window.__PRELOAD_AT = performance.now();"
	}
	fmt.Printf("MODE %q preload-bytes=%d\n", mode, len(preload))

	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:  "r10-preload",
			Width:  520,
			Height: 320,
			URL:    "/",
			Page:   mygo.PageOptions{PreloadScript: preload},
		})
		go func() {
			select {
			case <-probe.done:
				time.Sleep(400 * time.Millisecond)
				dir := os.Getenv("CAPTURE_DIR")
				if dir != "" {
					png, err := win.CapturePage()
					if err != nil {
						fmt.Println("CAPTURE-ERR " + err.Error())
					} else {
						out := filepath.Join(dir, "window-"+modeName(mode)+".png")
						if err := os.WriteFile(out, png, 0o644); err != nil {
							fmt.Println("CAPTURE-ERR " + err.Error())
						} else {
							fmt.Println("CAPTURE " + out)
						}
					}
				}
				fmt.Println("RESULT ok")
			case <-time.After(25 * time.Second):
				fmt.Println("RESULT timeout")
			}
			mygo.App.Quit()
		}()
	})
	if err := mygo.App.Run(); err != nil {
		fatal(err)
	}
}

func modeName(mode string) string {
	if mode == "" {
		return "default"
	}
	return mode
}

func fatal(err error) {
	fmt.Println("FAIL " + err.Error())
	os.Exit(1)
}
