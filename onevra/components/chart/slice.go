package chart

import (
	"slices"

	"github.com/auron-labs/mygo-framework/v2/onevra/components"
	"github.com/egoist/mygo/ui"
)

// positive returns the input indices of the slices that take space, their
// values divided by the largest, and their total.
func positive(data []Slice) (indices []int, values []float64, total float64) {
	largest := 0.0
	for i, s := range data {
		if finite(s.Value) && s.Value > 0 {
			indices, values = append(indices, i), append(values, s.Value)
			total, largest = total+s.Value, max(largest, s.Value)
		}
	}
	for i := range values {
		values[i] /= largest // Keep sums finite even for very large values.
	}
	return indices, values, total
}

func sliceSelection(c *ui.Context, data []Slice, i int) Selection {
	if i < 0 {
		return Selection{Index: -1}
	}
	s := data[i]
	return Selection{Index: i, Label: s.Label, Values: []Value{{s.ID, s.Label, s.Value, color(c, i, s.Color)}}}
}

// sliceLegend selects a slice from its legend button.
func sliceLegend(c *ui.Context, state *State, data []Slice, indices []int, opts Options) {
	if opts.HideLegend {
		return
	}
	ui.Row(c).FillWidth().MinWidth(0).Wrap().Justify(ui.Center).Gap(c.Theme().Space(1)).Children(func() {
		for _, i := range indices {
			s := data[i]
			ui.Box(c).Key(s.ID).MaxWidthPercent(100).Children(func() {
				b := legendButton(c, s.Label, state.Index == i)
				b.Children(func() {
					swatch(c, color(c, i, s.Color))
					components.Text(c, s.Label).FontSize(c.Theme().FontSize - 1)
				})
				if b.Clicked() {
					state.Index = i
					c.Invalidate()
					c.Announce(describe(opts, sliceSelection(c, data, i)))
				}
			})
		}
	})
}

type shapeHover struct {
	x, y    float32
	over    bool
	pointer bool
	shape   []float64
	indices []int
}

// shapeSelection selects the slice under the pointer, as hit reports its
// position in indices (or -1) for a point local to the area. Arrow keys and
// Home/End step through indices; Escape clears the selection. shape
// describes the current picture, so a pointer resting over a changing one
// follows what moves beneath it.
func shapeSelection(c *ui.Context, area ui.Element, state *State, indices []int, shape []float64, hit func(x, y float32) int, describe func(int) string) {
	area.Focusable()
	position := slices.Index(indices, state.Index)
	keyboard := true
	switch {
	case area.Shortcut(0, ui.KeyRight):
		state.Index = indices[min(position+1, len(indices)-1)]
	case area.Shortcut(0, ui.KeyLeft):
		if position < 0 {
			position = len(indices)
		}
		state.Index = indices[max(0, position-1)]
	case area.Shortcut(0, ui.KeyHome):
		state.Index = indices[0]
	case area.Shortcut(0, ui.KeyEnd):
		state.Index = indices[len(indices)-1]
	case area.Shortcut(0, ui.KeyEscape):
		state.Index = -1
	default:
		keyboard = false
	}
	hover := ui.Local(area, "onevra.chart.shape", func() shapeHover { return shapeHover{} })
	changed := !slices.Equal(hover.shape, shape) || !slices.Equal(hover.indices, indices)
	hover.shape, hover.indices = shape, indices
	x, y, over := area.PointerPosition()
	if keyboard || !over {
		hover.pointer = false
	} else if !hover.over || x != hover.x || y != hover.y {
		hover.pointer = true
	}
	if over && !keyboard && (!hover.over || x != hover.x || y != hover.y || (changed && hover.pointer)) {
		state.Index = -1
		if j := hit(x, y); j >= 0 {
			state.Index = indices[j]
		}
	} else if !over && hover.over && !keyboard {
		state.Index = -1
	}
	hover.x, hover.y, hover.over = x, y, over
	if state.Index >= 0 {
		value := describe(state.Index)
		area.Value(value)
		if keyboard {
			c.Announce(value)
		}
	}
}
