package main

import (
	"context"
	"mygo-framework/settings"
	boltsettings "mygo-framework/settings/bbolt"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/theme"
)

func TestNativePreferenceControl(t *testing.T) {
	s := &screen{}
	tt := ui.NewTester(func(c *ui.Context) { theme.Apply(c); s.view(c) }, 640, 360)
	if err := tt.Click("Compact spacing"); err != nil {
		t.Fatal(err)
	}
	if !s.compact {
		t.Fatal("checkbox did not update the retained preference")
	}
	if err := tt.Click("Compact spacing"); err != nil {
		t.Fatal(err)
	}
	if s.compact {
		t.Fatal("checkbox did not toggle back")
	}
}

func TestPreferenceSaveAndFailure(t *testing.T) {
	store, err := boltsettings.Open(filepath.Join(t.TempDir(), "preferences.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	values, err := settings.New(store, compactKey)
	if err != nil {
		t.Fatal(err)
	}
	s := &screen{compact: true}
	if err := s.save(values); err != nil {
		t.Fatal(err)
	}
	got, err := settings.Get(context.Background(), values, compactKey)
	if err != nil || !got {
		t.Fatalf("saved preference = %v, %v", got, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.save(values); err == nil || s.saveError == "" {
		t.Fatal("save failure must keep the window open with a visible error")
	}
	tt := ui.NewTester(func(c *ui.Context) { theme.Apply(c); s.view(c) }, 640, 360)
	if !tt.HasText(s.saveError) {
		t.Fatalf("error is not visible: %v", tt.Texts())
	}
}
