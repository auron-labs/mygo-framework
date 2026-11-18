package components

import (
	"testing"

	"github.com/auron-labs/mygo-framework/onevra/icons/lucide"
	"github.com/auron-labs/mygo-framework/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestNumberInputOptionalControls(t *testing.T) {
	for _, dark := range []bool{false, true} {
		value := .2
		disabled := false
		options := NumberInputOptions{HideStepper: true}
		unit := float32(1)
		changes := 0
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.Palette(dark))
			unit = c.Theme().Space(.25)
			ui.Column(c).Disabled(disabled).Children(func() {
				n := NumberInput(c, &value, -1, 1, .1, options).Label("Amount")
				if n.Changed() {
					changes++
				}
			})
		}, 300, 100)
		for _, label := range []string{"Adjust Amount", "Amount Increase", "Amount Decrease"} {
			if _, ok := tt.Find(label); ok {
				t.Fatalf("default shows %s", label)
			}
		}
		options.LeadingText = "X"
		options.LeadingIcon = lucide.MoveHorizontal()
		tt.Frame()
		drag := func(delta float32) {
			t.Helper()
			r, ok := tt.Find("Adjust Amount")
			if !ok {
				t.Fatal("missing drag handle")
			}
			x, y := r.X+r.W/2, r.Y+r.H/2
			tt.Press(x, y)
			tt.Move(x+delta*unit, y)
			tt.Release(x+delta*unit, y)
		}
		drag(1)
		if value != .3 || changes != 1 {
			t.Fatalf("decimal scrub: %v (%d changes)", value, changes)
		}
		tt.Key(0, ui.KeyRight)
		if value != .4 {
			t.Fatal("keyboard step", value)
		}
		tt.Key(0, ui.KeyHome)
		if value != -1 {
			t.Fatal("Home", value)
		}
		tt.Key(0, ui.KeyEnd)
		if value != 1 {
			t.Fatal("End", value)
		}
		before := changes
		drag(100)
		if value != 1 || changes != before {
			t.Fatal("upper bound reported a change")
		}
		drag(-100)
		if value != -1 {
			t.Fatal("lower clamp", value)
		}
		// An external update starts the next drag at the new value.
		value = .5
		tt.Frame()
		drag(1)
		if value != .6 {
			t.Fatal("external value", value)
		}
		disabled = true
		tt.Frame()
		before = changes
		drag(2)
		tt.Key(0, ui.KeyHome)
		if value != .6 || changes != before {
			t.Fatal("disabled scrub")
		}
		disabled = false
		options = NumberInputOptions{}
		tt.Frame()
		if _, ok := tt.Find("Adjust Amount"); ok {
			t.Fatal("removed prefix still interactive")
		}
		if err := tt.Click("Amount Increase"); err != nil {
			t.Fatal(err)
		}
		if value != .7 {
			t.Fatal("optional stepper", value)
		}
	}
}
