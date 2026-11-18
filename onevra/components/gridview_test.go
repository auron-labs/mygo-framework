package components

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/auron-labs/mygo-framework/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestGridViewSelectionAndVirtualization(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, disabled := range []bool{false, true} {
			selected, built, changes, submitted := 0, 0, 0, -1
			state := GridState{Selected: &selected, OnChange: func() { changes++ }, OnSubmit: func(i int) { submitted = i }}
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(theme.Palette(dark))
				original := c.Theme()
				c.Root().Background(ui.Transparent)
				ui.Column(c).Disabled(disabled).Children(func() {
					GridView(c, &state, 10000, 80, 50, func(i int) {
						built++
						ui.Textf(c, "Item %d", i).Padding(8)
						if c.Theme() != original {
							t.Error("cell theme changed")
						}
					}).Size(300, 180)
				})
				if c.Theme() != original {
					t.Error("grid theme changed")
				}
			}, 340, 220)
			// Selected applies upstream foreground defaults; Onevra must restore
			// the normal text color over its neutral selection background.
			palette := theme.LightPalette()
			if dark {
				palette = theme.DarkPalette()
			}
			selectedRect, _ := tt.Find("Item 0")
			textPixels := 0
			img := tt.Image()
			for y := int(selectedRect.Y); y < int(selectedRect.Y+selectedRect.H); y++ {
				for x := int(selectedRect.X); x < int(selectedRect.X+selectedRect.W); x++ {
					p := img.RGBAAt(x, y)
					if p.R == palette.Text.R && p.G == palette.Text.G && p.B == palette.Text.B {
						textPixels++
					}
				}
			}
			if !disabled && textPixels == 0 {
				t.Fatal("selected text lost its theme foreground")
			}
			if built > 500 {
				t.Fatalf("not virtualized: %d cells", built)
			}
			if tt.Image().RGBAAt(330, 210).A != 0 {
				t.Fatal("root background changed")
			}
			r, ok := tt.Find("Item 1")
			if !ok {
				t.Fatal("missing cell")
			}
			tt.Move(290, 170)
			before := bytes.Clone(tt.Image().Pix)
			tt.Move(r.X+4, r.Y+4)
			if !bytes.Equal(before, tt.Image().Pix) {
				t.Fatal("hover changed appearance")
			}
			if err := tt.Click("Item 1"); err != nil {
				t.Fatal(err)
			}
			if disabled {
				tt.Key(0, ui.KeyRight)
				if selected != 0 || changes != 0 {
					t.Fatal("disabled grid accepted input")
				}
				continue
			}
			if selected != 1 || changes != 1 {
				t.Fatalf("click: selected=%d changes=%d", selected, changes)
			}
			tt.Key(0, ui.KeyDown)
			if selected != 4 {
				t.Fatalf("Down selected %d, want 4", selected)
			}
			tt.Key(0, ui.KeyUp)
			tt.Key(0, ui.KeyRight)
			if selected != 2 {
				t.Fatalf("Right selected %d", selected)
			}
			tt.Key(0, ui.KeyEnter)
			if submitted != 2 {
				t.Fatal("Enter did not submit")
			}
			tt.Key(0, ui.KeyEnd)
			if selected != 9999 || !tt.HasText("Item 9999") {
				t.Fatal("End did not reveal final cell")
			}
			if built > 2000 {
				t.Fatalf("scroll built too many cells: %d", built)
			}
			tt.Key(0, ui.KeyHome)
			if selected != 0 || !tt.HasText("Item 0") {
				t.Fatal("Home did not reveal first cell")
			}
		}
	}
}

func TestGridViewKeysSelectionAndResize(t *testing.T) {
	selected := -1
	var selection ui.Selection[any]
	names := []string{"Alpha", "Bravo", "Charlie", "Delta", "Echo", "Foxtrot", "Golf", "Hotel", "India"}
	state := GridState{Selected: &selected, Selection: &selection, Key: func(i int) any { return names[i] }, Label: func(i int) string { return names[i] }}
	width := float32(300)
	tt := ui.NewTester(func(c *ui.Context) {
		GridView(c, &state, len(names), 80, 40, func(i int) { Text(c, names[i]).Padding(8) }).Size(width, 200)
	}, 340, 240)
	if err := tt.Click("Bravo"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Shift, ui.KeyDown)
	if selected != 4 || selection.Len() != 4 {
		t.Fatalf("range: cursor=%d length=%d", selected, selection.Len())
	}
	tt.Key(selectionModifier(), ui.KeyA)
	if selection.Len() != len(names) {
		t.Fatal("select all failed")
	}
	tt.Type("g")
	if selected != 6 || selection.Len() != 1 || !selection.Has("Golf") {
		t.Fatal("typeahead failed")
	}
	names[5], names[6] = names[6], names[5]
	tt.Frame()
	if selected != 5 {
		t.Fatalf("keyed cursor lost: %d", selected)
	}
	width = 190
	tt.Frame()
	tt.Key(0, ui.KeyUp)
	if selected != 3 {
		t.Fatalf("resize navigation selected %d, want 3", selected)
	}
	width = 90
	tt.Frame()
	tt.Key(0, ui.KeyUp)
	if selected != 2 {
		t.Fatalf("single column navigation selected %d", selected)
	}
}

func TestGridViewContentSizeAndEmpty(t *testing.T) {
	for _, state := range []*GridState{nil, {}} {
		tt := ui.NewTester(func(c *ui.Context) {
			GridView(c, state, 4, 80, 50, func(i int) { ui.Box(c).Fill().Label(fmt.Sprintf("cell-%d", i)) }).Size(300, 160)
		}, 320, 180)
		a, ok := tt.Find("cell-0")
		if !ok || a.H != 50 || a.W < 80 {
			t.Fatalf("content area: %+v", a)
		}
		b, _ := tt.Find("cell-1")
		d, _ := tt.Find("cell-3")
		if a.Y != b.Y || d.Y <= a.Y {
			t.Fatal("not a responsive grid")
		}
	}
	selected := 5
	state := GridState{Selected: &selected}
	tt := ui.NewTester(func(c *ui.Context) { GridView(c, &state, -1, 0, 0, nil).Size(100, 100) }, 120, 120)
	if selected != -1 {
		t.Fatal("empty grid retained invalid selection")
	}
	tt.Key(0, ui.KeyTab)
	tt.Key(0, ui.KeyEnd)
	tt.Key(0, ui.KeyEnter)
	if selected != -1 {
		t.Fatal("empty grid accepted selection")
	}
}

func TestGridViewRotatedKeyboardRows(t *testing.T) {
	selected := 1
	state := GridState{Selected: &selected}
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Padding(40).Rotate(12).Scale(.9, .9).Children(func() {
			GridView(c, &state, 20, 80, 40, func(i int) {
				Text(c, fmt.Sprintf("Item %d", i))
			}).Size(300, 180).AutoFocus()
		})
	}, 450, 350)
	tt.Key(0, ui.KeyDown)
	if selected != 4 {
		t.Fatalf("Down in rotated grid selected %d, want 4", selected)
	}
	tt.Key(0, ui.KeyUp)
	if selected != 1 {
		t.Fatalf("Up in rotated grid selected %d, want 1", selected)
	}
}
