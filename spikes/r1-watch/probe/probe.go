package main

// Minimal fsnotify recursive-watch probe.
// go run ./probe -vault <dir>

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

func main() {
	vault := flag.String("vault", "", "vault root")
	flag.Parse()
	root, _ := filepath.Abs(*vault)

	w, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}
	t0 := time.Now()
	err = w.Add(filepath.Join(root, "..."))
	fmt.Println("Add recursive:", time.Since(t0), "err:", err)
	if err != nil {
		return
	}
	go func() {
		for {
			select {
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				fmt.Println("EV", ev.Op, ev.Name)
			case e, ok := <-w.Errors:
				if !ok {
					return
				}
				fmt.Println("ERR", e)
			}
		}
	}()
	time.Sleep(500 * time.Millisecond)
	target := filepath.Join(root, "notes")
	// write one file deep in the tree
	var f string
	_ = filepath.Walk(target, func(p string, i os.FileInfo, e error) error {
		if e == nil && !i.IsDir() && f == "" {
			f = p
		}
		return nil
	})
	fmt.Println("touching", f)
	tmp := f + ".tmp"
	_ = os.WriteFile(tmp, []byte("x"), 0o644)
	_ = os.Rename(tmp, f)
	time.Sleep(3 * time.Second)
	fmt.Println("done")
}
