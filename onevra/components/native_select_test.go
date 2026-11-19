package components

import (
	"slices"
	"testing"

	"github.com/auron-labs/mygo-framework/v2/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestNativeSelect(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, disabled := range []bool{false, true} {
			choice, changes := 1, 0
			options := []NativeSelectOption[int]{{1, "Small", false}, {2, "Large", false}, {3, "Unavailable", true}}
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(theme.Palette(dark))
				ui.Column(c).Disabled(disabled).Children(func() {
					s := NativeSelect(c, &choice, options)
					s.Trigger.Label("Size")
					if s.Changed() {
						changes++
					}
				})
			}, 280, 200)
			if err := tt.Click("Size"); err != nil {
				t.Fatal(err)
			}
			if disabled {
				if len(tt.Menu()) != 0 {
					t.Fatal("disabled menu opened")
				}
				continue
			}
			if !slices.Equal(tt.Menu(), []string{"Small", "Large", "Unavailable"}) {
				t.Fatal(tt.Menu())
			}
			if err := tt.ChooseMenuItem("Small"); err != nil {
				t.Fatal(err)
			}
			if changes != 0 {
				t.Fatal("same value reported change")
			}
			tt.Key(0, ui.KeyDown)
			if err := tt.ChooseMenuItem("Large"); err != nil {
				t.Fatal(err)
			}
			if choice != 2 || changes != 1 || !tt.HasText("Large") {
				t.Fatalf("choice=%d changes=%d", choice, changes)
			}
			tt.Key(0, ui.KeyDown)
			_ = tt.ChooseMenuItem("Unavailable")
			if choice != 2 || changes != 1 {
				t.Fatal("disabled option changed selection")
			}
			tt.CloseMenu()
			_ = tt.Click("Size")
			tt.CloseMenu()
			if changes != 1 {
				t.Fatal("dismissal changed selection")
			}
		}
	}
}

func TestNativeSelectEmpty(t *testing.T) {
	choice := "unknown"
	tt := ui.NewTester(func(c *ui.Context) {
		NativeSelect(c, &choice, []NativeSelectOption[string]{}).Trigger.Label("Empty")
	}, 200, 100)
	if err := tt.Click("Empty"); err != nil {
		t.Fatal(err)
	}
	if len(tt.Menu()) != 0 || choice != "unknown" {
		t.Fatal("empty select changed")
	}
}
