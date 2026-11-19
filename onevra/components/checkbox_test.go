package components

import (
	"testing"

	"github.com/auron-labs/mygo-framework/v2/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestCheckboxToggle(t *testing.T) {
	for _, dark := range []bool{false, true} {
		mode := false
		if dark {
			mode = true
		}
		checked := false
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.Palette(mode))
			Checkbox(c, &checked, "Agree")
		}, 300, 100)
		if err := tt.Click("Agree"); err != nil {
			t.Fatal(err)
		}
		if !checked {
			t.Errorf("dark=%v: click did not check", dark)
		}
		if err := tt.Click("Agree"); err != nil {
			t.Fatal(err)
		}
		if checked {
			t.Errorf("dark=%v: click did not uncheck", dark)
		}
	}
}

func TestCheckboxKeyboard(t *testing.T) {
	checked := false
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightPalette())
		Checkbox(c, &checked, "Agree")
	}, 300, 100)
	tt.Key(0, ui.KeyTab)
	tt.Key(0, ui.KeySpace)
	if !checked {
		t.Error("Space did not toggle the focused check box")
	}
}
