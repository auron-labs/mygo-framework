package components

import (
	"testing"

	"github.com/auron-labs/mygo-framework/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestNumberInput(t *testing.T) {
	for _, dark := range []bool{false, true} {
		value := 0.2
		disabled := false
		changes := 0
		selectAll := false
		var draft string
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.Palette(dark))
			ui.Column(c).Disabled(disabled).Children(func() {
				control := NumberInput(c, &value, -1, 1, 0.1).Label("Amount")
				if selectAll {
					control.input.SetTextSelection(0, 100)
					selectAll = false
				}
				draft = ui.Local(control.Element, "number-input", func() numberInputState { return numberInputState{} }).text
				if control.Changed() {
					changes++
				}
				Button(c, "Outside", ButtonOutline)
			})
		}, 300, 120)
		click := func(label string) {
			t.Helper()
			if err := tt.Click(label); err != nil {
				t.Fatal(err)
			}
		}
		click("Amount Increase")
		if value != 0.3 || changes != 1 {
			t.Fatalf("increase: %v, changes %d", value, changes)
		}
		click("Amount Decrease")
		if value != 0.2 {
			up, _ := tt.Find("Amount Increase")
			down, _ := tt.Find("Amount Decrease")
			t.Fatalf("value %v; up=%v down=%v", value, up, down)
		}
		click("Amount")
		tt.Key(0, ui.KeyUp)
		if value != 0.3 {
			t.Fatal(value)
		}
		tt.Key(0, ui.KeyDown)
		if value != 0.2 {
			t.Fatal(value)
		}
		selectAll = true
		tt.Frame()
		tt.Type("-")
		if value != 0.2 {
			t.Fatal("partial input changed value")
		}
		tt.Type("0.7")
		if value != -0.7 {
			t.Fatalf("typed %v", value)
		}
		selectAll = true
		tt.Frame()
		tt.Type("99")
		if value != -0.7 {
			t.Fatal("out-of-range input changed value")
		}
		tt.Key(0, ui.KeyEnter)
		if draft != "-0.7" {
			t.Fatalf("Enter did not restore value: %q", draft)
		}
		selectAll = true
		tt.Frame()
		tt.Type("invalid")
		click("Outside")
		if draft != "-0.7" {
			t.Fatal("blur did not restore value")
		}
		click("Amount")
		value = 0.95
		tt.Frame()
		if draft != "0.95" {
			t.Fatal("external update not displayed")
		}
		click("Amount Increase")
		if value != 1 {
			t.Fatal("upper clamp", value)
		}
		before := changes
		click("Amount Increase")
		if changes != before {
			t.Fatal("bound reported change")
		}
		value = -1
		tt.Frame()
		click("Amount Decrease")
		if value != -1 {
			t.Fatal("lower clamp")
		}
		disabled = true
		tt.Frame()
		click("Amount Increase")
		click("Amount")
		tt.Key(0, ui.KeyUp)
		tt.Type("0")
		if value != -1 {
			t.Fatal("disabled changed")
		}
		input, _ := tt.Find("Amount")
		up, _ := tt.Find("Amount Increase")
		down, _ := tt.Find("Amount Decrease")
		if up.Y-1 != input.Y || down.Y+down.H+1 != input.Y+input.H || up.H != down.H {
			t.Fatalf("stepper must match text field height: %v %v %v", input, up, down)
		}
		if up.X+up.W != input.X+input.W-1 || down.X+down.W != input.X+input.W-1 {
			t.Fatalf("stepper must sit inside the editor's right border: %v %v %v", input, up, down)
		}
		if up.X <= input.X || up.Y >= down.Y || up.X != down.X || up.H <= 0 || down.H <= 0 {
			t.Fatalf("layout: %v %v %v", input, up, down)
		}
	}
}

func TestNumberInputClampAndIsolation(t *testing.T) {
	a, b := 20.0, 0.0
	var changed bool
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Children(func() {
			n := NumberInput(c, &a, 0, 10, 1).Label("A")
			changed = n.Changed()
			NumberInput(c, &b, 0, 10, 1).Label("B")
		})
	}, 300, 120)
	if a != 10 || changed {
		t.Fatal("initial clamp")
	}
	if err := tt.Click("B Increase"); err != nil {
		t.Fatal(err)
	}
	if b != 1 || a != 10 {
		t.Fatal("state isolation")
	}
}
