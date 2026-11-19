package components

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestPopoverInteraction(t *testing.T) {
	for _, dark := range []bool{false, true} {
		open := false
		enabled := false
		outside := 0
		builds := 0
		tt := ui.NewTester(func(c *ui.Context) {
			ui.Column(c).Fill().Padding(20).Gap(180).Children(func() {
				anchor := Button(c, "Open", ButtonOutline)
				if anchor.Clicked() {
					open = !open
				}
				Popover(c, anchor, &open, func(panel ui.Element) {
					builds++
					panel.Label("Settings")
					Checkbox(c, &enabled, "Enabled")
					if Button(c, "Close", ButtonSecondary).Clicked() {
						open = false
					}
				})
				if Button(c, "Outside", ButtonSecondary).Clicked() {
					outside++
				}
			})
		}, 400, 500)
		tt.SetDark(dark)
		click := func(label string) {
			t.Helper()
			if err := tt.Click(label); err != nil {
				t.Fatal(err)
			}
		}
		if builds != 0 {
			t.Fatal("closed popover built content")
		}
		click("Open")
		if !open || !tt.HasText("Enabled") {
			t.Fatal("popover did not open")
		}
		click("Enabled")
		if !enabled || !open {
			t.Fatal("content interaction dismissed panel")
		}
		click("Open")
		if open {
			t.Fatal("anchor toggle did not close")
		}
		click("Open")
		tt.Key(0, ui.KeyEscape)
		if open {
			t.Fatal("Escape did not close")
		}
		click("Open")
		click("Outside")
		if open || outside != 1 {
			t.Fatal("outside press did not dismiss and pass through")
		}
		click("Open")
		click("Close")
		tt.Frame()
		if open || tt.HasText("Enabled") {
			t.Fatal("content action did not close")
		}
	}
}

func TestPopoverPlacement(t *testing.T) {
	for _, top := range []float32{20, 240} {
		open := true
		var bounds, anchorBounds ui.Rect
		tt := ui.NewTester(func(c *ui.Context) {
			anchor := Button(c, "Anchor", ButtonOutline).Absolute().Left(20).Top(top)
			anchor.DrawOver(func(_ *ui.Painter, r ui.Rect) { anchorBounds = r })
			Popover(c, anchor, &open, func(panel ui.Element) {
				panel.Width(160).Height(100).DrawOver(func(_ *ui.Painter, r ui.Rect) { bounds = r })
				Text(c, "Content")
			})
		}, 280, 300)
		tt.Frame()
		if bounds.W != 160 || bounds.H != 100 || bounds.X < 0 || bounds.X+bounds.W > 280 || bounds.Y < 0 || bounds.Y+bounds.H > 300 {
			t.Fatalf("panel geometry: %+v", bounds)
		}
		if top == 20 && bounds.Y < anchorBounds.Y+anchorBounds.H || top == 240 && bounds.Y+bounds.H > anchorBounds.Y {
			t.Fatalf("panel did not follow/flip around anchor: panel=%+v anchor=%+v", bounds, anchorBounds)
		}
	}
}

func TestPopoverReportsDismissal(t *testing.T) {
	open := true
	var dismissed []bool
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Padding(20).Gap(180).Children(func() {
			anchor := Button(c, "Open", ButtonOutline)
			dismissed = append(dismissed, Popover(c, anchor, &open, func(panel ui.Element) {
				Text(c, "Panel")
			}).Dismissed())
			Button(c, "Outside", ButtonSecondary)
		})
	}, 400, 500)
	count := func() (n int) {
		for _, d := range dismissed {
			if d {
				n++
			}
		}
		return n
	}
	tt.Key(0, ui.KeyEscape)
	if open || count() != 1 {
		t.Fatalf("Escape: open=%v dismissals=%d", open, count())
	}
	open = true
	tt.Frame()
	if err := tt.Click("Outside"); err != nil {
		t.Fatal(err)
	}
	if open || count() != 2 {
		t.Fatalf("outside press: open=%v dismissals=%d", open, count())
	}
}
