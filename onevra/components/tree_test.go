package components

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/v2/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestTreeDisclosureAndAppearance(t *testing.T) {
	for _, virtual := range []bool{false, true} {
		for _, dark := range []bool{false, true} {
			for _, disabled := range []bool{false, true} {
				t.Run(fmt.Sprintf("virtual=%v/dark=%v/disabled=%v", virtual, dark, disabled), func(t *testing.T) {
					open, selected := false, ""
					var state TreeVirtualizedState[string]
					state.cursor = -1
					tt := ui.NewTester(func(c *ui.Context) {
						c.SetTheme(theme.Palette(dark))
						original := c.Theme()
						ui.Column(c).Disabled(disabled).Children(func() {
							if virtual {
								TreeVirtualized(c, &state, []string{"Folder", "Leaf"}, func(name string) []string {
									if name == "Folder" {
										return []string{"Child"}
									}
									return nil
								}, func(name string) {
									if c.Theme() != original {
										t.Error("row theme leaked")
									}
									Text(c, name)
								}).Size(240, 160)
							} else {
								Tree(c, func() {
									if TreeItem(c, "Folder", &open, selected == "Folder", func() {
										if c.Theme() != original {
											t.Error("child theme leaked")
										}
										TreeItem(c, "Child", nil, selected == "Child", nil)
									}).Clicked() {
										selected = "Folder"
									}
									TreeItem(c, "Leaf", nil, false, nil)
								}).Width(240)
							}
						})
						if c.Theme() != original {
							t.Error("tree theme leaked")
						}
					}, 260, 200)
					pause := func() {
						tt.Advance(200 * time.Millisecond)
						tt.Frame()
					}
					click := func(label string) {
						t.Helper()
						if err := tt.Click(label); err != nil {
							t.Fatal(err)
						}
					}
					checkHover := func() {
						t.Helper()
						tt.Move(259, 199)
						pause()
						before := bytes.Clone(tt.Image().Pix)
						r, _ := tt.Find("Folder")
						tt.Move(r.X+r.W/2, r.Y+r.H/2)
						if !bytes.Equal(before, tt.Image().Pix) {
							t.Fatal("hover changed tree appearance")
						}
					}
					checkHover()
					click("Folder")
					if tt.HasText("Child") {
						t.Fatal("single row click expanded branch")
					}
					click("Folder")
					if tt.HasText("Child") == disabled {
						t.Fatal("double click expansion mismatch")
					}
					checkHover()
					if disabled {
						r, _ := tt.Find("Folder")
						tt.ClickAt(12, r.Y+r.H/2)
						tt.Key(0, ui.KeyRight)
						if tt.HasText("Child") {
							t.Fatal("disabled arrow or keyboard expanded branch")
						}
						if selected != "" || state.cursor != -1 {
							t.Fatal("disabled tree changed selection")
						}
						return
					}
					if virtual && *state.List.Selected != 0 || !virtual && selected != "Folder" {
						t.Fatal("row click did not select")
					}
					r, _ := tt.Find("Folder")
					palette := theme.LightPalette()
					if dark {
						palette = theme.DarkPalette()
					}
					pixel := tt.Image().RGBAAt(220, int(r.Y+r.H/2))
					want := palette.Text.Alpha(.1).Over(palette.Background)
					delta := func(a, b uint8) int { return max(int(a)-int(b), int(b)-int(a)) }
					if delta(pixel.R, want.R) > 1 || delta(pixel.G, want.G) > 1 || delta(pixel.B, want.B) > 1 {
						t.Fatalf("selection = %v, want %v", pixel, want)
					}
					// The arrow is independently clickable and must not require a double click.
					tt.ClickAt(12, r.Y+r.H/2)
					if tt.HasText("Child") {
						t.Fatal("arrow did not collapse")
					}
					pause()
					tt.ClickAt(12, r.Y+r.H/2)
					if !tt.HasText("Child") {
						t.Fatal("arrow did not expand")
					}
					pause()
					click("Folder")
					click("Folder")
					if tt.HasText("Child") {
						t.Fatal("double click did not collapse")
					}
					tt.Key(0, ui.KeyRight)
					if !tt.HasText("Child") {
						t.Fatal("Right did not expand")
					}
					tt.Key(0, ui.KeyLeft)
					if tt.HasText("Child") {
						t.Fatal("Left did not collapse")
					}
					click("Leaf")
					click("Leaf")
					if tt.HasText("Child") || state.Open.Has("Leaf") {
						t.Fatal("leaf double click changed expansion")
					}
				})
			}
		}
	}
}

func TestTreeItemWithLeading(t *testing.T) {
	open, arrow := true, float32(0)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightPalette())
		arrow = c.Theme().Space(4)
		Tree(c, func() {
			TreeItemWithLeading(c, "Folder", &open, false, func(c *ui.Context, size float32) {
				ui.Box(c).Size(size, size).Label("mark")
			}, func() { TreeItem(c, "Child", nil, false, nil) })
		}).Width(240)
	}, 260, 200)
	folder, _ := tt.Find("Folder")
	mark, ok := tt.Find("mark")
	if !ok || !tt.HasText("Child") {
		t.Fatal("missing leading mark or children")
	}
	// The mark follows the disclosure arrow inside its row.
	if mark.X < folder.X+arrow || mark.X+mark.W > folder.X+folder.W || mark.Y < folder.Y || mark.Y+mark.H > folder.Y+folder.H {
		t.Fatalf("mark %v outside row %v after its arrow", mark, folder)
	}
}

func TestTreeVirtualizedBuildsOnlyVisibleRows(t *testing.T) {
	var state TreeVirtualizedState[int]
	selected, built := 0, 0
	state.List.Selected = &selected
	state.Open.Add(-1)
	items := make([]int, 10000)
	for i := range items {
		items[i] = i
	}
	tt := ui.NewTester(func(c *ui.Context) {
		TreeVirtualized(c, &state, []int{-1}, func(item int) []int {
			if item == -1 {
				return items
			}
			return nil
		}, func(item int) {
			built++
			ui.Textf(c, "Item %d", item)
		}).Size(240, 160)
	}, 260, 200)
	if built >= 1000 {
		t.Fatalf("built %d rows", built)
	}
	if err := tt.Click("Item 0"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyEnd)
	if selected != 10000 || !tt.HasText("Item 9999") {
		t.Fatal("End did not reveal last row")
	}
	tt.Key(0, ui.KeyLeft)
	if selected != 0 || !tt.HasText("Item -1") {
		t.Fatal("Left did not select parent")
	}
	tt.Key(0, ui.KeyLeft)
	if state.Open.Has(-1) || state.Rows() != 1 {
		t.Fatal("Left did not collapse parent")
	}
}
