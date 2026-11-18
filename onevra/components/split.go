package components

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// Orientation describes the arrangement of a control's parts, such as a
// Split's panes, not its divider line, or a Stepper's steps.
type Orientation uint8

const (
	Horizontal Orientation = iota // Left and right panes; steps in a row.
	Vertical                      // Top and bottom panes; steps in a column.
)

// SplitOptions configures a two-pane resizable layout.
type SplitOptions struct {
	// Orientation defaults to Horizontal, including for unrecognized values.
	Orientation Orientation
	// Minimum pane sizes in DIPs. Zero uses Theme.Space(10); negative or
	// nonfinite values are invalid. When both minima cannot fit, they shrink
	// proportionally. Layout constraints do not overwrite the preferred size.
	FirstMin, SecondMin float32
	// Default, when positive, is the size of the first pane that
	// double-clicking the divider restores, within the effective limits: DIPs
	// for Split and a share from 0 to 100 for SplitPercent. Negative or
	// nonfinite values are invalid.
	Default float32
}

// SplitControl exposes the layout element. Keep this handle to query Changed
// after Element layout methods, which return ui.Element.
type SplitControl struct {
	ui.Element
	divider ui.Element
	changed bool
}

// Changed reports an input-driven size change, not a layout constraint.
func (s *SplitControl) Changed() bool { return s.changed }

// Label sets the accessible name of the keyboard-focusable divider.
func (s *SplitControl) Label(label string) *SplitControl {
	s.divider.Label(label)
	return s
}

// AutoFocus focuses the divider.
func (s *SplitControl) AutoFocus() *SplitControl {
	s.divider.AutoFocus()
	return s
}

// Split arranges two clipped panes separated by a one-DIP line. size is the
// application-owned preferred width (Horizontal) or height (Vertical) of the
// first pane and must be nonnil and finite. Give the split a bounded size.
// Changing orientation reinterprets size in the new axis; applications that
// need independent sizes should keep separate bindings.
//
//	Split(c, &width, files, editor).Fill()
//	Split(c, &height, preview, console, SplitOptions{Orientation: Vertical}).Fill()
//
// A transparent, theme-scaled handle captures pointer drags. The line keeps
// Theme.Border on hover and while dragging; the cursor and keyboard focus ring
// provide feedback. Tab focuses the divider between the panes; axis arrow keys
// move it by Theme.Space(2.5), and Home/End move to the effective limits.
// Label defaults to "Divider". Nil builders leave their panes empty.
// Double-clicking the divider restores SplitOptions.Default when set.
func Split(c *ui.Context, size *float32, first, second func(), options ...SplitOptions) *SplitControl {
	return split(c, "components.Split", size, false, first, second, options)
}

// SplitPercent is Split with *percent the share of the room, from 0 to 100,
// that the first pane takes, so both panes grow and shrink with the split in
// proportion. Values outside that range are clamped when shown. The minima
// still apply and are not written back, so the preferred share returns when
// there is room again. The divider reports its position to assistive
// technology as a percentage, and SplitOptions.Default is a share.
//
//	SplitPercent(c, &percent, files, editor).Fill()
//	SplitPercent(c, &percent, preview, console, SplitOptions{Orientation: Vertical}).Fill()
func SplitPercent(c *ui.Context, percent *float32, first, second func(), options ...SplitOptions) *SplitControl {
	return split(c, "components.SplitPercent", percent, true, first, second, options)
}

// split shares the room between first and second: *value is first's size in
// DIPs, or with percent its share of the room.
func split(c *ui.Context, name string, value *float32, percent bool, first, second func(), options []SplitOptions) *SplitControl {
	words := localized(c)
	if len(options) > 1 {
		panic(name + ": at most one options value")
	}
	o := SplitOptions{}
	if len(options) == 1 {
		o = options[0]
	}
	finite := func(v float32) bool { return !math.IsNaN(float64(v)) && !math.IsInf(float64(v), 0) }
	if value == nil || !finite(*value) || !finite(o.FirstMin) || !finite(o.SecondMin) || o.FirstMin < 0 || o.SecondMin < 0 {
		panic(name + ": the value must be nonnil and finite; minima must be finite and nonnegative")
	}
	if !finite(o.Default) || o.Default < 0 {
		panic(name + ": the default must be finite and nonnegative")
	}
	t := c.Theme()
	if o.FirstMin == 0 {
		o.FirstMin = t.Space(10)
	}
	if o.SecondMin == 0 {
		o.SecondMin = t.Space(10)
	}
	vertical := o.Orientation == Vertical
	s := &SplitControl{Element: ui.Box(c).Column().MinWidth(0).MinHeight(0)}
	s.Element.Children(func() {
		in := ui.Box(c).Key("panes").Grow(1).MinWidth(0).MinHeight(0).AlignItems(ui.Stretch).Clip()
		total := in.LayoutBounds().W
		in.Row()
		if vertical {
			in.Column()
			total = in.LayoutBounds().H
		}
		// LayoutBounds describes the previous layout. Request one follow-up frame
		// when this container changes size, even without further user input.
		in.Draw(func(_ *ui.Painter, r ui.Rect) {
			actual := r.W
			if vertical {
				actual = r.H
			}
			if actual != total {
				c.Invalidate()
			}
		})
		lo, hi := splitLimits(total, o.FirstMin, o.SecondMin)
		// room is what the panes share, without the line between them.
		room := max(0, total-1)
		preferred := *value
		if percent {
			preferred = min(max(*value, 0), 100) / 100 * room
		}
		position := max(lo, min(preferred, hi))
		in.Children(func() {
			a := ui.Box(c).Key("first").Shrink(0).MinWidth(0).MinHeight(0).Clip()
			if first != nil {
				a.Children(first)
			}
			div := ui.Box(c).Key("divider").Shrink(0).Focusable().FocusRing(false).
				Role(ui.RoleSplitter).Label(words.Divider).Background(t.Border)
			s.divider = div
			b := ui.Box(c).Key("second").Grow(1).Shrink(1).MinWidth(0).MinHeight(0).Clip()
			if second != nil {
				b.Children(second)
			}
			handle := ui.Box(c).Key("handle").Absolute()
			back, forth := ui.KeyLeft, ui.KeyRight
			if vertical {
				back, forth = ui.KeyUp, ui.KeyDown
			}
			next := position
			if dx, dy, held := handle.Dragged(); held && !handle.IsDisabled() {
				div.Focus()
				if vertical {
					next += dy
				} else {
					next += dx
				}
			}
			step := t.Space(2.5)
			if div.Shortcut(0, back) {
				next -= step
			}
			if div.Shortcut(0, forth) {
				next += step
			}
			if div.Shortcut(0, ui.KeyHome) {
				next = lo
			}
			if div.Shortcut(0, ui.KeyEnd) {
				next = hi
			}
			if o.Default > 0 && handle.DoubleClicked() && !handle.IsDisabled() {
				div.Focus()
				next = o.Default
				if percent {
					next = min(o.Default, 100) / 100 * room
				}
			}
			next = max(lo, min(next, hi))
			if next != position && !div.IsDisabled() && (!percent || room > 0) {
				*value, position, s.changed = next, next, true
				if percent {
					*value = next / room * 100
				}
				c.Invalidate()
			}
			if percent && room > 0 {
				scale := float64(100 / room)
				div.Range(float64(lo)*scale, float64(hi)*scale, float64(position)*scale).Step(float64(step) * scale)
			} else {
				div.Range(float64(lo), float64(hi), float64(position)).Step(float64(step))
			}
			grip := t.Space(1.5)
			if vertical {
				a.Height(position)
				div.Height(1)
				handle.Left(0).Right(0).Top(position - (grip-1)/2).Height(grip).Cursor(ui.CursorResizeNS)
			} else {
				a.Width(position)
				div.Width(1)
				handle.Top(0).Bottom(0).Left(position - (grip-1)/2).Width(grip).Cursor(ui.CursorResizeEW)
			}
			focus := div.FocusVisible()
			handle.Draw(func(p *ui.Painter, r ui.Rect) {
				if focus {
					p.FocusRing(r, [4]float32{})
				}
			})
		})
	})
	return s
}

func splitLimits(total, first, second float32) (float32, float32) {
	room := max(float32(0), total-1)
	if room < first+second {
		// Float64 avoids overflowing the sum for large finite minima.
		lo := float32(float64(room) * float64(first) / (float64(first) + float64(second)))
		return lo, lo
	}
	return first, room - second
}
