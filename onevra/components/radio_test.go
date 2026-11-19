package components

import (
	"testing"

	"github.com/auron-labs/mygo-framework/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestRadioSelect(t *testing.T) {
	for _, dark := range []bool{false, true} {
		mode := false
		if dark {
			mode = true
		}
		selected := "a"
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.Palette(mode))
			Radio(c, &selected, "a", "Alpha")
			Radio(c, &selected, "b", "Beta")
		}, 300, 100)
		if err := tt.Click("Beta"); err != nil {
			t.Fatal(err)
		}
		if selected != "b" {
			t.Errorf("dark=%v: selected=%q, want b", dark, selected)
		}
		// Clicking the chosen one keeps it.
		if err := tt.Click("Beta"); err != nil {
			t.Fatal(err)
		}
		if selected != "b" {
			t.Errorf("dark=%v: selected=%q, want b", dark, selected)
		}
		if err := tt.Click("Alpha"); err != nil {
			t.Fatal(err)
		}
		if selected != "a" {
			t.Errorf("dark=%v: selected=%q, want a", dark, selected)
		}
	}
}

func TestRadioKeyboard(t *testing.T) {
	selected := "a"
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightPalette())
		Radio(c, &selected, "a", "Alpha")
		Radio(c, &selected, "b", "Beta")
	}, 300, 100)
	tt.Key(0, ui.KeyTab)
	tt.Key(0, ui.KeyTab)
	tt.Key(0, ui.KeySpace)
	if selected != "b" {
		t.Errorf("Space did not choose the focused radio, selected=%q", selected)
	}
}
