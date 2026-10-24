// Native demonstrates using framework settings without a web frontend or SQLite.
package main

import (
	"context"
	"log"
	"mygo-framework/settings"
	boltsettings "mygo-framework/settings/bbolt"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

var compactKey = settings.NewKey("native.compact", false, nil)

type screen struct{ compact bool }

func (s *screen) view(c *ui.Context) {
	gap := float32(24)
	if s.compact {
		gap = 8
	}
	ui.Column(c).Fill().Padding(24).Gap(gap).Children(func() {
		ui.Text(c, "Native, with framework settings").FontSize(22).Bold()
		ui.Checkbox(c, &s.compact, "Compact spacing")
		ui.Text(c, "This preference is saved when the app closes. No browser, Bun, IPC bindings, or application database is required.").Wrap()
	})
}

func main() {
	dir, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	dir = filepath.Join(dir, "mygo-framework-native")
	if err := os.MkdirAll(dir, 0700); err != nil {
		log.Fatal(err)
	}
	store, err := boltsettings.Open(filepath.Join(dir, "preferences.db"))
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	values, err := settings.New(store, compactKey)
	if err != nil {
		log.Print(err)
		return
	}
	compact, err := settings.Get(context.Background(), values, compactKey)
	if err != nil {
		log.Print(err)
		return
	}
	s := &screen{compact: compact}
	mygo.App.SetName("Framework Native")
	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(mygo.WindowOptions{Title: "Framework Native", Width: 640, Height: 360, Content: ui.View(s.view)})
		win.OnClosed(func() { mygo.App.Quit() })
	})
	if err := mygo.App.Run(); err != nil {
		log.Print(err)
		return
	}
	// Builders only mutate UI-owned state; persist after the event loop stops.
	if err := settings.Set(context.Background(), values, compactKey, s.compact); err != nil {
		log.Print(err)
	}
}
