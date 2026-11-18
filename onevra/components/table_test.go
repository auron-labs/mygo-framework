package components

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func tableTestColumns() []ui.TableColumn {
	return []ui.TableColumn{
		{Title: "Name", Sortable: true},
		{Title: "Size", Width: 90, Align: ui.End, Sortable: true},
		{Title: "Kind", Width: 80},
	}
}

func TestTableHeaderInteractionAndAppearance(t *testing.T) {
	for _, virtual := range []bool{false, true} {
		for _, dark := range []bool{false, true} {
			for _, disabled := range []bool{false, true} {
				t.Run(fmt.Sprintf("virtual=%v/dark=%v/disabled=%v", virtual, dark, disabled), func(t *testing.T) {
					columns := tableTestColumns()
					order, sorts, chosen := ui.SortOrder{}, 0, -1
					var state TableVirtualizedState
					state.Sort, state.OnSort = &order, func() { sorts++ }
					state.List.Selected = &chosen
					tt := ui.NewTester(func(c *ui.Context) {
						c.SetTheme(theme.Palette(dark))
						original := c.Theme()
						cell := func(row, col int) {
							if c.Theme() != original {
								t.Error("cell theme leaked")
							}
							Text(c, fmt.Sprintf("r%d-%s", row, columns[col].Title)).SingleLine()
						}
						ui.Column(c).Disabled(disabled).Children(func() {
							if virtual {
								TableVirtualized(c, &state, columns, 3, cell).Size(400, 200)
							} else {
								Table(c, &state.TableState, columns, func() {
									for row := range 3 {
										if TableRow(c, row, fmt.Sprint("Row ", row), chosen == row, func(col int) { cell(row, col) }).Clicked() {
											chosen = row
										}
									}
								}).Width(400)
							}
						})
						if c.Theme() != original {
							t.Error("table theme leaked")
						}
					}, 420, 240)
					find := func(label string) ui.Rect {
						t.Helper()
						r, ok := tt.Find(label)
						if !ok {
							t.Fatalf("missing %q", label)
						}
						return r
					}
					// Header and cells share column geometry, including End alignment.
					if name, cell := find("Name"), find("r0-Name"); name.X != cell.X {
						t.Fatalf("Name header at %v, cell at %v", name.X, cell.X)
					}
					if size, cell := find("Size"), find("r0-Size"); size.X+size.W != cell.X+cell.W {
						t.Fatalf("Size header ends at %v, cell at %v", size.X+size.W, cell.X+cell.W)
					}
					tt.Move(419, 239)
					tt.Advance(200 * time.Millisecond)
					tt.Frame()
					before := bytes.Clone(tt.Image().Pix)
					row := find("r1-Kind")
					tt.Move(row.X, row.Y+row.H/2)
					if !bytes.Equal(before, tt.Image().Pix) {
						t.Fatal("hover changed table appearance")
					}

					palette := theme.LightPalette()
					if dark {
						palette = theme.DarkPalette()
					}
					delta := func(a, b uint8) int { return max(int(a)-int(b), int(b)-int(a)) }
					checkPixel := func(label string, want ui.Color) {
						t.Helper()
						r := find(label)
						pixel := tt.Image().RGBAAt(int(r.X)-4, int(r.Y+r.H/2))
						if delta(pixel.R, want.R) > 1 || delta(pixel.G, want.G) > 1 || delta(pixel.B, want.B) > 1 {
							t.Fatalf("%s background = %v, want %v", label, pixel, want)
						}
					}
					// Odd rows carry the stripe; even rows show the host background.
					// Disabled ancestors fade the whole table.
					if !disabled {
						checkPixel("r0-Name", palette.Background)
						checkPixel("r1-Name", palette.Text.Alpha(.025).Over(palette.Background))
						checkPixel("r2-Name", palette.Background)
					}

					name := find("Name")
					tt.ClickAt(name.X+4, name.Y+name.H/2)
					if (sorts == 1) == disabled || !disabled && order != (ui.SortOrder{Column: "Name"}) {
						t.Fatalf("sort = %+v after %d changes", order, sorts)
					}
					tt.ClickAt(name.X+4, name.Y+name.H/2)
					if !disabled && !order.Descending {
						t.Fatal("second click did not reverse the sort")
					}
					// Name shares the room left: 400 - 90 - 80. Its edge sits at the right.
					y := name.Y + name.H/2
					tt.Press(226, y)
					tt.Move(250, y)
					tt.Move(276, y)
					tt.Release(276, y)
					if w, ok := state.Columns.Widths["Name"]; ok == disabled || ok && w != 280 {
						t.Fatalf("resized widths = %v", state.Columns.Widths)
					}
					if sorts > 2 {
						t.Fatal("resizing sorted the table")
					}
					kind := find("Kind")
					tt.Press(kind.X+4, y)
					for _, dx := range []float32{-20, -60, -80} {
						tt.Move(kind.X+dx, y)
					}
					tt.Release(kind.X-80, y)
					if moved := len(state.Columns.Order) == 3 && state.Columns.Order[1] == "Kind"; moved == disabled {
						t.Fatalf("order = %v", state.Columns.Order)
					}
					if !disabled && find("Kind").X != find("r0-Kind").X {
						t.Fatal("moved header and cells disagree")
					}

					click := find("r1-Name")
					tt.ClickAt(click.X, click.Y+click.H/2)
					if disabled {
						if chosen != -1 || sorts != 0 {
							t.Fatal("disabled table changed state")
						}
						return
					}
					if chosen != 1 {
						t.Fatal("row click did not select")
					}
					// Selection replaces the stripe on the odd row.
					checkPixel("r1-Name", palette.Text.Alpha(.1).Over(palette.Background))
				})
			}
		}
	}
}

func TestTableLayoutOrderAndWidths(t *testing.T) {
	columns := []ui.TableColumn{
		{Title: "Pin", Width: 40, Fixed: true},
		{Title: "Name", ID: "name", MinWidth: 120},
		{Title: "Size", Width: 90, MaxWidth: 100},
		{Title: "Kind", Width: 80},
	}
	state := TableState{Columns: ui.TableLayout{
		Order:  []string{"Kind", "missing", "Pin", "Kind", "name"},
		Widths: map[string]float32{"Size": 500, "Pin": 300, "Kind": 1},
	}}
	tt := ui.NewTester(func(c *ui.Context) {
		layout := newTableColumns(c, &state, columns)
		if got := fmt.Sprint(layout.order); got != "[0 3 1 2]" {
			t.Errorf("order = %s", got)
		}
		for j, want := range []float32{40, 0, 100, 16} {
			if w, ok := layout.width(&columns[j]); w != want || ok != (want != 0) {
				t.Errorf("%s width = %v, %v", columns[j].Title, w, ok)
			}
		}
		if layout.least != 40+120+100+16 {
			t.Errorf("least = %v", layout.least)
		}
	}, 200, 100)
	tt.Frame()
}

func TestTableKeyboardFocus(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		Table(c, nil, tableTestColumns(), func() {
			for row := range 4 {
				TableRow(c, row, fmt.Sprint("Row ", row), false, func(col int) {
					Text(c, fmt.Sprintf("%d-%d", row, col))
				})
			}
		})
	}, 420, 240)
	if err := tt.Click("0-0"); err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		key   ui.Key
		label string
	}{{ui.KeyDown, "Row 1"}, {ui.KeyEnd, "Row 3"}, {ui.KeyUp, "Row 2"}, {ui.KeyHome, "Row 0"}} {
		tt.Key(0, step.key)
		if !tt.Focused(step.label) {
			t.Fatalf("%v did not focus %s", step.key, step.label)
		}
	}
	defer func() {
		if recover() == nil {
			t.Fatal("TableRow outside Table did not panic")
		}
	}()
	ui.NewTester(func(c *ui.Context) { TableRow(c, 0, "Row", false, nil) }, 100, 100)
}

func TestTableVirtualizedBuildsOnlyVisibleRows(t *testing.T) {
	selected, built, submitted := -1, 0, -1
	var state TableVirtualizedState
	state.List.Selected = &selected
	state.List.OnSubmit = func(i int) { submitted = i }
	columns := tableTestColumns()
	tt := ui.NewTester(func(c *ui.Context) {
		TableVirtualized(c, &state, columns, 10000, func(row, col int) {
			built++
			Text(c, fmt.Sprintf("%d-%s", row, columns[col].Title))
		}).Size(400, 200)
	}, 420, 240)
	if built >= 1000 {
		t.Fatalf("built %d cells", built)
	}
	if err := tt.Click("0-Name"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyEnd)
	if selected != 9999 || !tt.HasText("9999-Name") {
		t.Fatal("End did not reveal last row")
	}
	tt.Key(0, ui.KeyEnter)
	if submitted != 9999 {
		t.Fatal("Enter did not submit")
	}
	// Columns wider than the table scroll sideways, with the header.
	state.Columns.Widths = map[string]float32{"Name": 600}
	tt.Frame()
	tt.Scroll(200, 100, 400, 0)
	tt.Frame()
	head, okHead := tt.Find("Kind")
	cell, okCell := tt.Find("9999-Kind")
	if !okHead || !okCell || head.W == 0 || head.X != cell.X || head.X >= 400 {
		t.Fatalf("header %v and cell %v did not scroll together", head, cell)
	}
}

func TestTableScaledHeaderGeometry(t *testing.T) {
	for _, virtual := range []bool{false, true} {
		for _, scale := range []float32{.75, 1.5} {
			t.Run(fmt.Sprintf("virtual=%v/scale=%v", virtual, scale), func(t *testing.T) {
				var state TableVirtualizedState
				tt := ui.NewTester(func(c *ui.Context) {
					ui.Column(c).Scale(scale, scale).TransformOrigin(0, 0).Children(func() {
						if virtual {
							TableVirtualized(c, &state, tableTestColumns(), 2, func(row, col int) { Text(c, fmt.Sprint(row, col)) }).Size(400, 200)
						} else {
							Table(c, &state.TableState, tableTestColumns(), func() {}).Width(400)
						}
					})
				}, 660, 350)
				y := float32(16) * scale
				tt.Press(226*scale, y)
				tt.Move(276*scale, y)
				tt.Release(276*scale, y)
				if w := state.Columns.Widths["Name"]; w < 279.9 || w > 280.1 {
					t.Fatalf("scaled resize width = %v, want 280", w)
				}
				kind, _ := tt.Find("Kind")
				x := kind.X + 4*scale
				tt.Press(x, y)
				// Cross Size's layout midpoint (45), but not 1.5x its visual midpoint.
				tt.Move(x-55*scale, y)
				tt.Release(x-55*scale, y)
				if len(state.Columns.Order) != 3 || state.Columns.Order[1] != "Kind" {
					t.Fatalf("scaled reorder = %v", state.Columns.Order)
				}
			})
		}
	}
}
