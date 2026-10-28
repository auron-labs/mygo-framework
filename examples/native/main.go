// Native demonstrates a Onevra window with retained state and framework settings.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"mygo-framework/settings"
	boltsettings "mygo-framework/settings/bbolt"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/shells/plain"
	"github.com/oneo-me/onevra/source/theme"
)

const (
	appName     = "Framework Native"
	profileName = "mygo-framework-native"
)

var compactKey = settings.NewKey("native.compact", false, nil)

type screen struct {
	compact   bool
	saveError string
}

func (s *screen) view(c *ui.Context) {
	gap := float32(24)
	if s.compact {
		gap = 8
	}
	ui.Column(c).Fill().Padding(24).Gap(gap).Children(func() {
		components.Text(c, appName).FontSize(22).Bold()
		components.Checkbox(c, &s.compact, "Compact spacing")
		components.Text(c, "Your preference is saved when you close the window.").Wrap()
		if s.saveError != "" {
			components.Text(c, s.saveError).Wrap()
		}
	})
}

func (s *screen) save(values *settings.Values) error {
	if err := settings.Set(context.Background(), values, compactKey, s.compact); err != nil {
		s.saveError = "Could not save your preference. Please try closing the window again."
		return fmt.Errorf("save preferences: %w", err)
	}
	s.saveError = ""
	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() (err error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("locate preferences: %w", err)
	}
	dir = filepath.Join(dir, profileName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create preferences directory: %w", err)
	}
	store, err := boltsettings.Open(filepath.Join(dir, "preferences.db"))
	if err != nil {
		return fmt.Errorf("open preferences: %w", err)
	}
	defer func() { err = errors.Join(err, store.Close()) }()
	values, err := settings.New(store, compactKey)
	if err != nil {
		return fmt.Errorf("register preferences: %w", err)
	}
	compact, err := settings.Get(context.Background(), values, compactKey)
	if err != nil {
		return fmt.Errorf("load preferences: %w", err)
	}
	s := &screen{compact: compact}
	mygo.App.SetName(appName)
	mygo.App.WhenReady(func() {
		shell := plain.NewShell(plain.ShellOptions{
			Title: appName, Width: 640, Height: 360, MinWidth: 360, MinHeight: 300,
			StateKey: "main", Theme: theme.Onevra, Content: s.view,
		})
		win := shell.Window()
		// MyGo also calls OnClose during application quit, before destroying the window.
		win.OnClose(func(event *mygo.CloseEvent) {
			if err := s.save(values); err != nil {
				event.PreventDefault()
				log.Print(err)
				win.Invalidate()
			}
		})
		win.OnClosed(func() { mygo.App.Quit() })
	})
	if err := mygo.App.Run(); err != nil {
		return fmt.Errorf("run native app: %w", err)
	}
	return nil
}
