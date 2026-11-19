package components

import (
	"bytes"
	"slices"
	"testing"

	"github.com/auron-labs/mygo-framework/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestListAppearanceAndSelection(t *testing.T) {
	for _, dark := range []bool{false, true} {
		selected := 0
		state := ListState{Selected: &selected}
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.Palette(dark))
			original := c.Theme()
			List(c, &state, 3, func(i int) {
				if c.Theme() != original {
					t.Error("row theme leaked")
				}
				ui.Textf(c, "Item %d", i+1).Padding(8)
			}).Size(200, 150)
			if c.Theme() != original {
				t.Error("list theme leaked")
			}
		}, 220, 200)
		for _, label := range []string{"Item 1", "Item 2"} {
			r, ok := tt.Find(label)
			if !ok {
				t.Fatal("missing row")
			}
			tt.Move(219, 199)
			before := bytes.Clone(tt.Image().Pix)
			tt.Move(r.X+20, r.Y+r.H/2)
			if !bytes.Equal(before, tt.Image().Pix) {
				t.Fatal("hover changed list appearance")
			}
		}
		if err := tt.Click("Item 2"); err != nil {
			t.Fatal(err)
		}
		if selected != 1 {
			t.Fatal("click did not select row")
		}
		r, _ := tt.Find("Item 2")
		p := tt.Image().RGBAAt(180, int(r.Y+r.H/2))
		palette := theme.LightPalette()
		if dark {
			palette = theme.DarkPalette()
		}
		want := palette.Text.Alpha(.1).Over(palette.Background)
		delta := func(a, b uint8) int { return max(int(a)-int(b), int(b)-int(a)) }
		if delta(p.R, want.R) > 1 || delta(p.G, want.G) > 1 || delta(p.B, want.B) > 1 {
			t.Fatalf("selection = %v, want %v", p, want)
		}
		tt.Key(0, ui.KeyDown)
		if selected != 2 {
			t.Fatal("keyboard did not advance selection")
		}
	}
}

func TestListVirtualizationAndDisabled(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		selected, built := 0, 0
		state := ListState{Selected: &selected}
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.LightPalette())
			ui.Column(c).Disabled(disabled).Children(func() {
				List(c, &state, 10000, func(i int) {
					built++
					ui.Textf(c, "Item %d", i+1).Padding(8)
				}).Size(200, 150)
			})
		}, 220, 200)
		if built >= 1000 {
			t.Fatalf("built too many rows: %d", built)
		}
		if err := tt.Click("Item 2"); err != nil {
			t.Fatal(err)
		}
		tt.Key(0, ui.KeyDown)
		if disabled && selected != 0 {
			t.Fatal("disabled list accepted input")
		}
		if !disabled {
			tt.Key(0, ui.KeyEnd)
			if selected != 9999 || !tt.HasText("Item 10000") {
				t.Fatal("End did not reveal final row")
			}
		}
	}
}

func TestListKeepOffset(t *testing.T) {
	for _, keep := range []bool{false, true} {
		rows := make([]int, 100)
		for i := range rows {
			rows[i] = i
		}
		selected := 20
		state := ListState{Selected: &selected, KeepOffset: keep, Key: func(i int) any { return rows[i] }}
		tt := ui.NewTester(func(c *ui.Context) {
			List(c, &state, len(rows), func(i int) {
				ui.Textf(c, "Item %d", rows[i]).Padding(8)
			}).Size(200, 150)
		}, 220, 200)
		state.ScrollTo(20, ui.Start)
		tt.Frame()
		before, _ := state.Visible()
		slices.Reverse(rows)
		tt.Frame()
		first, _ := state.Visible()
		if keep && first != before {
			t.Fatalf("KeepOffset moved the first row from %d to %d", before, first)
		}
		if !keep && first == before {
			t.Fatal("keyed list did not follow its top row")
		}
		if rows[selected] != 20 {
			t.Fatalf("selection followed row %d, want 20", rows[selected])
		}
	}
}
