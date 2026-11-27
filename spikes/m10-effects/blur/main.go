// M10 T00 blur spike: open the approved global-material prototype
// (design/r2/global-material.html) inside a real mygo + WebView2 window so the
// four blur levels can be captured and their text contrast sampled from real
// pixels. The run script generates web/ from the prototype and drives this
// window through the automation socket.
//
//	MYGO_FRONTEND_DIST=web MYGO_AUTOMATION=tcp:127.0.0.1:18431 \
//	go run -tags e2e .
package main

import (
	"log"

	"github.com/egoist/mygo"
)

func main() {
	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:           "M10 Blur Spike",
			Width:           1440,
			Height:          900,
			BackgroundColor: "light-dark(#eceef1, #0e0f11)",
			URL:             "/?state=work-doc&theme=light&blur=std&shot=1&motion=off",
		})
		win.OnReadyToShow(func() { win.Show() })
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
