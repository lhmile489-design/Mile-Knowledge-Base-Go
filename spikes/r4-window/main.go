// R4/R5/R6/R8 spike: frameless window, tray, close-ask dialog, and a
// streaming channel driving an incremental-render benchmark in the page.
//
// Run (dev):  bun run dev:web  +  MYGO_DEV_URL=http://localhost:5274 \
//             MYGO_AUTOMATION=<sock> go run -tags e2e .
// Run (dist): bun run build:web && MYGO_FRONTEND_DIST=dist \
//             MYGO_AUTOMATION=<sock> go run -tags e2e .
package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
)

//go:embed resources/icon.png
var iconPNG []byte

var (
	mainWin  *mygo.Window
	tray     *mygo.Tray
	trayOK   bool
	quitting atomic.Bool
	startAt  = time.Now()
	logMu    sync.Mutex
)

// mark logs one line to stderr and appends it to results/spike-log.jsonl
// for the report.
func mark(stage string, fields map[string]any) {
	fields["stage"] = stage
	fields["t_ms"] = time.Since(startAt).Milliseconds()
	b, _ := json.Marshal(fields)
	logMu.Lock()
	defer logMu.Unlock()
	fmt.Fprintln(os.Stderr, "[spike]", string(b))
	dir := filepath.Join(os.Getenv("TEMP"), "r4-spike")
	_ = os.MkdirAll(dir, 0o755)
	f, err := os.OpenFile(filepath.Join(dir, "spike-log.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err == nil {
		defer f.Close()
		f.Write(b)
		f.Write([]byte("\n"))
	}
}

// ---------- services bound to the page ----------

type SpikeService struct{}

type resultMsg struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

// Ready is the first binding call the page makes after its listeners are
// attached; its arrival time approximates cold-start to first interactive
// paint.
func (SpikeService) Ready(ctx context.Context) map[string]any {
	mark("page-ready", map[string]any{})
	return map[string]any{
		"bootMs": time.Since(startAt).Milliseconds(),
		"trayOk": trayOK,
	}
}

// WinOp runs a window control from the title bar buttons.
func (SpikeService) WinOp(ctx context.Context, op string) bool {
	w := mygo.CallerWindow(ctx)
	if w == nil {
		return false
	}
	switch op {
	case "minimize":
		w.Minimize()
	case "toggleMaximize":
		w.ToggleMaximize()
	case "close":
		w.Close()
	case "hideToTray":
		hideToTray(w)
	case "show":
		showMain()
	case "setSize":
		// probe: request below-min size; OS should clamp to MinWidth/MinHeight
		w.SetSize(400, 300)
	}
	return true
}

// WinState reports window facts the page/assertions need.
func (SpikeService) WinState(ctx context.Context) map[string]any {
	w := mygo.CallerWindow(ctx)
	if w == nil {
		return map[string]any{}
	}
	b := w.Bounds()
	cw, ch := w.ContentSize()
	return map[string]any{
		"boundsX": b.X, "boundsY": b.Y, "boundsW": b.Width, "boundsH": b.Height,
		"contentW": cw, "contentH": ch,
		"maximized": w.IsMaximized(), "minimized": w.IsMinimized(),
		"visible": w.IsVisible(), "focused": true,
		"trayOk": trayOK,
	}
}

// StreamFrame is one coalesced push to the page.
type StreamFrame struct {
	Seq   int    `json:"seq"`
	Text  string `json:"text"`           // HTML fragment appended to the live message
	Done  bool   `json:"done,omitempty"`
	Chars int    `json:"chars,omitempty"`
}

// StreamReq picks a push strategy.
type StreamReq struct {
	Mode     string `json:"mode"`     // "frame16" (16ms coalesce) | "frame50" | "token"
	Interval int    `json:"interval"` // ms override
	Chunk    int    `json:"chunk"`    // chars per source token
	Total    int    `json:"total"`    // total chars to stream
}

var streamStats struct {
	sent   atomic.Int64
	lastNs atomic.Int64
}

// Stream simulates a 10k-char answer: it emits HTML fragments over a
// channel, either per "token" (worst case, P-5) or coalesced per frame.
func (SpikeService) Stream(ctx context.Context, req StreamReq, ch *mygo.Channel[StreamFrame]) error {
	total := req.Total
	if total <= 0 {
		total = 10000
	}
	chunk := req.Chunk
	if chunk <= 0 {
		chunk = 24 // ~a token's worth of text
	}
	interval := time.Duration(req.Interval) * time.Millisecond
	if interval <= 0 {
		switch req.Mode {
		case "token":
			interval = 2 * time.Millisecond // model-speed tokens
		case "frame50":
			interval = 50 * time.Millisecond
		default:
			interval = 16 * time.Millisecond
		}
	}
	streamStats.sent.Store(0)
	mark("stream-begin", map[string]any{"mode": req.Mode, "intervalMs": interval.Milliseconds()})

	var buf strings.Builder
	seq := 0
	emit := func(force bool) error {
		if buf.Len() == 0 && !force {
			return nil
		}
		seq++
		f := StreamFrame{Seq: seq, Text: buf.String()}
		buf.Reset()
		if err := ch.Send(f); err != nil {
			return err
		}
		streamStats.sent.Add(1)
		return nil
	}

	written := 0
	para := 0
	tick := time.NewTicker(interval)
	defer tick.Stop()
	src := time.NewTicker(2 * time.Millisecond) // token source: 500 tok/s
	defer src.Stop()
	pending := 0 // tokens produced but not yet coalesced
	for written < total {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-src.C:
			// produce one token into the coalescing buffer
			n := chunk
			if written+n > total {
				n = total - written
			}
			buf.WriteString(makeFragment(written, n, &para))
			written += n
			pending++
		case <-tick.C:
			if req.Mode == "token" {
				// worst case: flush whatever arrived, one Send per tick too
				// (a true per-token push is bounded by src rate anyway)
			}
			if err := emit(false); err != nil {
				return err
			}
			pending = 0
		}
	}
	// final frame
	if err := ch.Send(StreamFrame{Seq: seq + 1, Text: buf.String(), Done: true, Chars: written}); err != nil {
		return err
	}
	streamStats.sent.Add(1)
	mark("stream-end", map[string]any{"frames": seq + 1, "chars": written})
	return nil
}

// makeFragment produces a plausible HTML fragment: words, occasionally a
// paragraph break or inline code, deterministic on offset.
func makeFragment(off, n int, para *int) string {
	words := []string{"the", "incremental", "render", "pipeline", "keeps", "frames",
		"under", "sixteen", "milliseconds", "while", "streaming", "markdown",
		"chunks", "into", "the", "live", "message", "node", "only"}
	var b strings.Builder
	for b.Len() < n {
		w := words[(off+b.Len())%len(words)]
		b.WriteString(w)
		b.WriteByte(' ')
		if (off+b.Len())%997 < 3 {
			b.WriteString("<code>x=1</code> ")
		}
		if (off+b.Len())%1200 < 6 {
			b.WriteString("</p><p>")
			*para++
		}
	}
	s := b.String()
	if len(s) > n {
		s = s[:n]
	}
	return s
}

// Result lets the page ship measured numbers back to Go.
func (SpikeService) Result(ctx context.Context, payload resultMsg) error {
	mark("page-result", map[string]any{"key": payload.Key, "value": payload.Value})
	return nil
}

// DialogProbe shows the close-ask dialog on demand (for automation), so
// the button index / checkbox mapping can be verified without a real
// click on the X button.
func (SpikeService) DialogProbe(ctx context.Context) (map[string]any, error) {
	w := mygo.CallerWindow(ctx)
	res, err := askCloseDialog(w)
	out := map[string]any{"button": res.Button, "checkbox": res.CheckboxChecked}
	if err != nil {
		out["error"] = err.Error()
	}
	mark("dialog-probe", out)
	return out, err
}

// QuitNow quits the app for real (bypasses the ask flow).
func (SpikeService) QuitNow(ctx context.Context) {
	quitting.Store(true)
	mygo.App.Quit()
}

// ---------- close behavior ----------

func askCloseDialog(w *mygo.Window) (mygo.MessageResult, error) {
	return mygo.Dialog.Message(mygo.MessageOptions{
		Parent:         w,
		Type:           mygo.MessageQuestion,
		Title:          "关闭 Mile Knowledge Base",
		Message:        "要退出应用，还是挂到托盘继续运行？",
		Detail:         "挂到托盘后窗口会隐藏，单击托盘图标可重新打开。",
		Buttons:        []string{"挂到托盘", "退出应用"},
		DefaultButton:  0,
		CancelButton:   -1, // Esc/dismiss reports Button -1 on Windows task dialogs
		CheckboxLabel:  "记住我的选择",
	})
}

func hideToTray(w *mygo.Window) {
	if !trayOK {
		quitting.Store(true)
		mygo.App.Quit()
		return
	}
	w.SetSkipTaskbar(true)
	w.Hide()
	mark("hidden-to-tray", map[string]any{})
}

func showMain() {
	w := mainWin
	if w == nil {
		return
	}
	w.SetSkipTaskbar(false)
	if w.IsMinimized() {
		w.Restore()
	}
	w.Show()
	w.Focus()
	mark("shown-from-tray", map[string]any{})
}

func handleCloseRequest(w *mygo.Window) {
	res, err := askCloseDialog(w)
	mark("close-dialog", map[string]any{"button": res.Button, "checkbox": res.CheckboxChecked, "err": err != nil})
	switch res.Button {
	case 1: // 退出应用
		quitting.Store(true)
		mygo.App.Quit()
	default: // 0 = 挂到托盘; -1 (dismissed) also parks to tray per spec
		hideToTray(w)
	}
}

func main() {
	mygo.Bind(SpikeService{})
	mygo.Theme.SetSource(mygo.ThemeSystem)
	mark("pre-run", map[string]any{})

	mygo.App.WhenReady(func() {
		mark("app-ready", map[string]any{})

		win := mygo.NewWindow(mygo.WindowOptions{
			Title:           "R4 Spike",
			Width:           1440,
			Height:          900,
			MinWidth:        744,
			MinHeight:       640,
			Frameless:       true,
			Hidden:          true,
			BackgroundColor: "light-dark(#eceef1, #0e0f11)",
			URL:             "/",
		})
		mainWin = win
		win.OnReadyToShow(func() {
			mark("ready-to-show", map[string]any{})
			win.Show()
		})
		win.OnClose(func(e *mygo.CloseEvent) {
			if quitting.Load() {
				return
			}
			e.PreventDefault()
			go handleCloseRequest(win)
		})

		t, err := mygo.NewTray(mygo.TrayOptions{Icon: iconPNG, ToolTip: "R4 Spike"})
		if err != nil {
			mark("tray-error", map[string]any{"err": err.Error()})
			trayOK = false
		} else {
			tray = t
			trayOK = true
			tray.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
				{Label: "显示主窗口", Click: func(*mygo.MenuItem, *mygo.Window) { showMain() }},
				mygo.Separator(),
				{Label: "退出", Click: func(*mygo.MenuItem, *mygo.Window) {
					quitting.Store(true)
					mygo.App.Quit()
				}},
			}))
			tray.OnClick(func() {
				mark("tray-clicked", map[string]any{})
				showMain()
			})
			tray.OnRightClick(func() {
				mark("tray-right-clicked", map[string]any{})
			})
			mark("tray-created", map[string]any{})
		}
	})
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { quitting.Store(true) })

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
	mark("run-exit", map[string]any{})
}
