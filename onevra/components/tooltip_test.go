package components

import (
	"strings"
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestTooltipKeyboard(t *testing.T) {
	for _, dark := range []bool{false, true} {
		visible, disabled := false, false
		text := "Saves the document"
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.Palette(dark))
			ui.Column(c).Padding(60).Gap(20).Children(func() {
				anchor := Button(c, "Save", ButtonSecondary).Disabled(disabled)
				visible = Tooltip(c, anchor, text).Valid()
				other := Button(c, "Open", ButtonSecondary)
				Tooltip(c, other, "Opens a document")
			})
		}, 400, 300)
		if visible {
			t.Fatal("tooltip initially visible")
		}
		tt.Key(0, ui.KeyTab)
		if !visible || !tt.HasText(text) {
			t.Fatal("keyboard focus did not show tooltip")
		}
		tt.Key(0, ui.KeyEscape)
		if visible || tt.HasText(text) {
			t.Fatal("Escape did not dismiss tooltip")
		}
		tt.Key(0, ui.KeyTab)
		if visible || !tt.HasText("Opens a document") {
			t.Fatal("focus did not transfer tooltip")
		}
		tt.Key(ui.Shift, ui.KeyTab)
		if !visible {
			t.Fatal("returning focus did not restore tooltip")
		}
		text = ""
		tt.Frame()
		if visible || tt.HasText("Saves the document") {
			t.Fatal("empty tooltip remained visible")
		}
		text = "Saves the document"
		disabled = true
		tt.Frame()
		if visible {
			t.Fatal("disabled anchor showed tooltip")
		}
	}
}

func TestTooltipHover(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Padding(60).Children(func() {
			Tooltip(c, Button(c, "Save", ButtonSecondary), "Save changes")
		})
	}, 300, 200)
	r, _ := tt.Find("Save")
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	if tt.HasText("Save changes") {
		t.Fatal("hover tooltip appeared without delay")
	}
	tt.Advance(650 * time.Millisecond)
	tt.Frame()
	if !tt.HasText("Save changes") {
		t.Fatal("hover tooltip did not appear")
	}
	if err := tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	if tt.HasText("Save changes") {
		t.Fatal("click did not dismiss tooltip")
	}
	tt.Move(290, 190)
	tt.Frame()
	if tt.HasText("Save changes") {
		t.Fatal("tooltip remained after leaving anchor")
	}
}

func TestTooltipPlacement(t *testing.T) {
	for _, top := range []float32{2, 180} {
		var tipBounds ui.Rect
		tt := ui.NewTester(func(c *ui.Context) {
			anchor := Button(c, "Anchor", ButtonSecondary).Absolute().Left(40).Top(top)
			Tooltip(c, anchor, strings.Repeat("Helpful text ", 8)).DrawOver(func(_ *ui.Painter, r ui.Rect) { tipBounds = r })
		}, 360, 260)
		tt.Key(0, ui.KeyTab)
		tt.Frame()
		anchor, _ := tt.Find("Anchor")
		if tipBounds.W <= 0 || tipBounds.X < 0 || tipBounds.X+tipBounds.W > 360 || tipBounds.Y < 0 || tipBounds.Y+tipBounds.H > 260 {
			t.Fatalf("tooltip outside viewport: %+v", tipBounds)
		}
		if top == 2 && tipBounds.Y < anchor.Y+anchor.H || top == 180 && tipBounds.Y+tipBounds.H > anchor.Y {
			t.Fatalf("tooltip did not flip around anchor: tip=%+v anchor=%+v", tipBounds, anchor)
		}
	}
}

func TestTooltipAbsentAnchor(t *testing.T) {
	ui.NewTester(func(c *ui.Context) {
		if Tooltip(c, ui.Element{}, "Hint").Valid() {
			t.Fatal("absent anchor produced tooltip")
		}
	}, 200, 100)
}
