// Package tablebench measures Onevra's virtualized table and list at
// thousands of rows. It holds only tests and benchmarks.
package tablebench

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/theme"
)

var sizes = []int{1_000, 10_000, 100_000}

var columns = []ui.TableColumn{
	{Title: "Night", Width: 160},
	{Title: "Start", Width: 120},
	{Title: "Events", Width: 90},
	{Title: "Lowest SpO2", Width: 120},
}

func tableTester(rows int, built *int) (*ui.Tester, *components.TableVirtualizedState) {
	state := &components.TableVirtualizedState{}
	sel := 0
	state.List.Selected = &sel
	tt := ui.NewTester(func(c *ui.Context) {
		theme.Apply(c)
		ui.Column(c).Fill().Children(func() {
			components.TableVirtualized(c, state, columns, rows, func(row, col int) {
				if built != nil && col == 0 {
					*built++
				}
				components.Text(c, fmt.Sprintf("r%d c%d", row, col))
			}).Grow(1)
		})
	}, 640, 480)
	tt.Frame()
	return tt, state
}

func listTester(rows int, built *int) (*ui.Tester, *components.ListState) {
	state := &components.ListState{}
	sel := 0
	state.Selected = &sel
	tt := ui.NewTester(func(c *ui.Context) {
		theme.Apply(c)
		ui.Column(c).Fill().Children(func() {
			components.List(c, state, rows, func(row int) {
				if built != nil {
					*built++
				}
				components.Text(c, fmt.Sprintf("event %d", row))
			}).Grow(1)
		})
	}, 640, 480)
	tt.Frame()
	return tt, state
}

// TestOnlyVisibleRowsAreBuilt guards the property that makes large tables and
// lists responsive: the work per frame follows the rows on screen, not the
// row count.
func TestOnlyVisibleRowsAreBuilt(t *testing.T) {
	// Layout builds a row more than once per frame, so the budget is a few times the rows that fit; it must stay far below the smallest row count.
	const visibleRowBudget = 200
	for _, n := range sizes {
		for name, build := range map[string]func(*int) (*ui.Tester, func(int)){
			"table": func(built *int) (*ui.Tester, func(int)) {
				tt, state := tableTester(n, built)
				return tt, func(row int) { state.List.ScrollTo(row, ui.Start) }
			},
			"list": func(built *int) (*ui.Tester, func(int)) {
				tt, state := listTester(n, built)
				return tt, func(row int) { state.ScrollTo(row, ui.Start) }
			},
		} {
			t.Run(fmt.Sprintf("%s/%d rows", name, n), func(t *testing.T) {
				var built int
				tt, scrollTo := build(&built)
				tt.Frame()
				if built == 0 || built > visibleRowBudget {
					t.Fatalf("built %d rows at the top of %d", built, n)
				}
				built = 0
				scrollTo(n / 2)
				tt.Frame()
				if built == 0 || built > visibleRowBudget {
					t.Fatalf("built %d rows after jumping to the middle of %d", built, n)
				}
				if !tt.HasText(strconv.Itoa(n / 2)) {
					t.Errorf("row %d is not on screen after scrolling to it: %q", n/2, tt.Texts())
				}
			})
		}
	}
}

// BenchmarkTable measures one frame (build, layout, CPU paint) of Onevra's
// virtualized table at rest, while the wheel scrolls it, and after jumping
// to a far row.
func BenchmarkTable(b *testing.B) {
	for _, n := range sizes {
		b.Run(fmt.Sprintf("%d rows/first frame", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				tableTester(n, nil)
			}
		})
		b.Run(fmt.Sprintf("%d rows/steady frame", n), func(b *testing.B) {
			tt, _ := tableTester(n, nil)
			b.ReportAllocs()
			for b.Loop() {
				tt.Frame()
			}
		})
		b.Run(fmt.Sprintf("%d rows/wheel scroll", n), func(b *testing.B) {
			tt, _ := tableTester(n, nil)
			b.ReportAllocs()
			for i := 0; b.Loop(); i++ {
				tt.Wheel(300, 240, 0, 40+float32(i%7))
				tt.Frame()
			}
		})
		b.Run(fmt.Sprintf("%d rows/jump to middle", n), func(b *testing.B) {
			tt, state := tableTester(n, nil)
			b.ReportAllocs()
			for i := 0; b.Loop(); i++ {
				state.List.ScrollTo(n/2+i%50, ui.Start)
				tt.Frame()
			}
		})
	}
}

// BenchmarkList is BenchmarkTable for Onevra's list.
func BenchmarkList(b *testing.B) {
	for _, n := range sizes {
		b.Run(fmt.Sprintf("%d rows/first frame", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				listTester(n, nil)
			}
		})
		b.Run(fmt.Sprintf("%d rows/steady frame", n), func(b *testing.B) {
			tt, _ := listTester(n, nil)
			b.ReportAllocs()
			for b.Loop() {
				tt.Frame()
			}
		})
		b.Run(fmt.Sprintf("%d rows/wheel scroll", n), func(b *testing.B) {
			tt, _ := listTester(n, nil)
			b.ReportAllocs()
			for i := 0; b.Loop(); i++ {
				tt.Wheel(300, 240, 0, 40+float32(i%7))
				tt.Frame()
			}
		})
		b.Run(fmt.Sprintf("%d rows/jump to middle", n), func(b *testing.B) {
			tt, state := listTester(n, nil)
			b.ReportAllocs()
			for i := 0; b.Loop(); i++ {
				state.ScrollTo(n/2+i%50, ui.Start)
				tt.Frame()
			}
		})
	}
}
