// M8 glass-scroll spike (T00): a 200-card page that mirrors the skills-page
// look (glass cards over a drifting colour field) so the scroll frame pacing
// can be measured in a real mygo + WebView2 window.
//
//	MYGO_FRONTEND_DIST=web SPIKE_W=1440 SPIKE_H=900 \
//	MYGO_AUTOMATION=tcp:127.0.0.1:18421 go run -tags e2e .
//
// The page exposes window.__spike.start(ms) / window.__spike.result(); the
// driver talks to the automation socket (eval) and collects the numbers.
package main

import (
	"log"
	"os"
	"strconv"

	"github.com/egoist/mygo"
)

func envInt(key string, def int) int {
	if value, err := strconv.Atoi(os.Getenv(key)); err == nil && value > 0 {
		return value
	}
	return def
}

func main() {
	mygo.App.WhenReady(func() {
		tag := os.Getenv("SPIKE_TAG")
		if tag == "" {
			tag = "1440x900"
		}
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:           "M8 Glass Spike " + tag,
			Width:           envInt("SPIKE_W", 1440),
			Height:          envInt("SPIKE_H", 900),
			BackgroundColor: "light-dark(#eceef1, #0e0f11)",
			URL:             "/?size=" + tag,
		})
		win.OnReadyToShow(func() { win.Show() })
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
