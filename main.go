package main

import (
	"log"

	"github.com/egoist/mygo"
)

// AppService is the bootstrap binding surface. Real services are registered
// from internal/service as later tasks land.
type AppService struct{}

// Ping is used by the frontend boot handshake.
func (AppService) Ping() string { return "pong" }

func main() {
	mygo.Bind(AppService{})

	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:           "Mile Knowledge Base",
			Width:           1440,
			Height:          900,
			MinWidth:        744,
			MinHeight:       640,
			Frameless:       true,
			BackgroundColor: "light-dark(#eceef1, #0e0f11)",
			StateKey:        "main",
			URL:             "/",
		})
		_ = win
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
