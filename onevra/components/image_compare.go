package components

import (
	"math"
	"time"

	"github.com/auron-labs/mygo-framework/onevra/icons/lucide"
	"github.com/egoist/mygo/ui"
)

// ImageCompareOptions configures a before-and-after comparison.
type ImageCompareOptions struct {
	// Orientation defaults to Horizontal, with before on the left; Vertical
	// puts before on top. Unrecognized values are Horizontal.
	Orientation Orientation
	// BeforeLabel and AfterLabel caption the two sides; an empty string
	// omits its caption. Captions are decorative and hidden from assistive
	// technology; name the comparison with ImageCompareControl.Label.
	BeforeLabel, AfterLabel string
	// AspectRatio is the frame's width divided by its height. Zero uses 3/2;
	// a negative value leaves the height to the host. Nonfinite is invalid.
	AspectRatio float32
	// Default, from 0 to 100, is the position double-clicking the handle
	// restores. Zero uses 50; values outside the range are clamped.
	Default float32
}

// ImageCompareControl exposes the frame element. Keep this handle to query
// Changed after Element layout methods, which return ui.Element.
type ImageCompareControl struct {
	ui.Element
	handle  ui.Element
	changed bool
}

// Changed reports an input-driven position change.
func (s *ImageCompareControl) Changed() bool { return s.changed }

// Label sets the accessible name of the focusable divider handle.
func (s *ImageCompareControl) Label(label string) *ImageCompareControl {
	s.handle.Label(label)
	return s
}

// AutoFocus focuses the divider handle.
func (s *ImageCompareControl) AutoFocus() *ImageCompareControl {
	s.handle.AutoFocus()
	return s
}

// ImageCompare overlays before on after in a rounded, clipped frame and
// reveals before up to a draggable divider. *position is the
// application-owned share of the frame, from 0 to 100, that before covers;
// it must be nonnil and finite, and values outside the range are clamped
// when shown. Both builders lay out at the full frame size so their content
// stays aligned, and both remain built at every position. Nil builders
// leave their side empty. Content receives no pointer input.
//
//	ImageCompare(c, &position, func() { Image(c, raw).Fill().Fit(ui.Cover) },
//		func() { Image(c, edited).Fill().Fit(ui.Cover) },
//		ImageCompareOptions{BeforeLabel: "Before", AfterLabel: "After"}).
//		Label("Comparison").FillWidth()
//
// Pressing anywhere on the frame moves the divider there with a spring;
// dragging tracks the pointer and keeps its release velocity as it settles,
// while the handle stretches into a capsule. Double-clicking the handle
// restores Default. The focusable handle reports its position as a slider:
// axis arrows move it by 1, or 10 with Shift, PageUp/PageDown by 10, and
// Home/End to 0 and 100. Captions fade as the divider nears them. Reduced
// Motion moves the divider at once. Label defaults to "Divider".
func ImageCompare(c *ui.Context, position *float32, before, after func(), options ...ImageCompareOptions) *ImageCompareControl {
	if len(options) > 1 {
		panic("components.ImageCompare: at most one options value")
	}
	o := ImageCompareOptions{}
	if len(options) == 1 {
		o = options[0]
	}
	finite := func(v float32) bool { return !math.IsNaN(float64(v)) && !math.IsInf(float64(v), 0) }
	if position == nil || !finite(*position) {
		panic("components.ImageCompare: the position must be nonnil and finite")
	}
	if !finite(o.AspectRatio) || !finite(o.Default) {
		panic("components.ImageCompare: the aspect ratio and default must be finite")
	}
	if o.AspectRatio == 0 {
		o.AspectRatio = 3.0 / 2
	}
	if o.Default == 0 {
		o.Default = 50
	}
	clamp := func(v float32) float32 { return max(0, min(v, 100)) }
	t := c.Theme()
	vertical := o.Orientation == Vertical
	frame := ui.Box(c).Clip().Radius(t.Radius).Background(t.Surface).MinWidth(t.Space(20)).MinHeight(t.Space(20))
	if o.AspectRatio > 0 {
		frame.AspectRatio(o.AspectRatio)
	}
	s := &ImageCompareControl{Element: frame}
	frame.Children(func() {
		fill := func(e ui.Element) ui.Element { return e.Absolute().Left(0).Top(0).Right(0).Bottom(0) }
		layer := fill(ui.Box(c).Key("after"))
		if after != nil {
			layer.Children(after)
		}
		reveal := ui.Box(c).Key("before").Absolute().Left(0).Top(0).Clip()
		var content ui.Element
		reveal.Children(func() {
			content = ui.Box(c).Key("content").Absolute().Left(0).Top(0)
			if before != nil {
				content.Children(before)
			}
		})
		caption := func(key, text string) ui.Element {
			if text == "" {
				return ui.Element{}
			}
			return Badge(c, text).Key(key).Absolute().Role(ui.RoleNone).
				Background(ui.RGBA(0, 0, 0, .5)).TextColor(ui.RGB(255, 255, 255)).Border(0, ui.Color{})
		}
		inset := t.Space(3)
		beforeCaption := caption("before-caption", o.BeforeLabel).Top(inset).Left(inset)
		afterCaption := caption("after-caption", o.AfterLabel)
		if vertical {
			afterCaption.Bottom(inset).Left(inset)
		} else {
			afterCaption.Top(inset).Right(inset)
		}

		handle := fill(ui.Box(c).Key("handle")).Focusable().FocusRing(false).
			Role(ui.RoleSlider).Label(localized(c).Divider)
		s.handle = handle
		bounds := handle.LayoutBounds()
		extent := bounds.W
		if vertical {
			extent = bounds.H
		}
		enabled := !handle.IsDisabled()
		at := func(x, y float32) float32 {
			if extent <= 0 {
				return clamp(*position)
			}
			if vertical {
				x = y
			}
			return clamp(x / extent * 100)
		}
		set := func(v float32) {
			if v = clamp(v); v != *position {
				*position, s.changed = v, true
			}
		}

		// moved separates a drag, which tracks the pointer, from a press,
		// which springs to it; recentered ignores the press that double-clicked.
		// shown is the divider's last painted position.
		type gesture struct {
			moved, recentered bool
			shown             float32
		}
		g := ui.Local(handle, "image-compare-gesture", func() gesture { return gesture{} })
		x, y, _ := handle.PointerPosition()
		near := t.Space(5)
		dx, dy, held := handle.Dragged()
		held = held && enabled
		if held && (dx != 0 || dy != 0) {
			g.moved = true
		}
		if held && handle.DoubleClicked() {
			p := x
			if vertical {
				p = y
			}
			if math.Abs(float64(p-g.shown/100*extent)) <= float64(near) {
				g.recentered = true
				set(o.Default)
			}
		}
		if held && !g.recentered {
			handle.Focus()
			set(at(x, y))
			if g.moved {
				handle.Jump("position", clamp(*position))
			}
		}
		if !held {
			g.moved, g.recentered = false, false
		}

		back, forth := ui.KeyLeft, ui.KeyRight
		if vertical {
			back, forth = ui.KeyUp, ui.KeyDown
		}
		value := clamp(*position)
		switch {
		case handle.Shortcut(0, back):
			set(value - 1)
		case handle.Shortcut(ui.Shift, back):
			set(value - 10)
		case handle.Shortcut(0, forth):
			set(value + 1)
		case handle.Shortcut(ui.Shift, forth):
			set(value + 10)
		case handle.Shortcut(0, ui.KeyPageDown):
			set(value - 10)
		case handle.Shortcut(0, ui.KeyPageUp):
			set(value + 10)
		case handle.Shortcut(0, ui.KeyHome):
			set(0)
		case handle.Shortcut(0, ui.KeyEnd):
			set(100)
		}
		shown := clamp(handle.AnimateTo("position", clamp(*position), ui.Smooth))
		g.shown = shown
		handle.Range(0, 100, float64(clamp(*position))).Step(1)

		// The revealed region keeps before at the frame size; a tiny minimum
		// share keeps the scale finite while before is fully hidden.
		share := max(shown, 0.01)
		if vertical {
			reveal.Right(0).HeightPercent(share)
			content.Right(0).HeightPercent(100 * 100 / share)
		} else {
			reveal.Bottom(0).WidthPercent(share)
			content.Bottom(0).WidthPercent(100 * 100 / share)
		}
		line := shown / 100 * extent
		// Captions fade over Space(4) as the divider reaches them.
		fade := func(e ui.Element, beforeSide bool) {
			r := e.LayoutBounds()
			if r.W == 0 {
				return
			}
			lo, hi := r.X-bounds.X, r.X-bounds.X+r.W
			if vertical {
				lo, hi = r.Y-bounds.Y, r.Y-bounds.Y+r.H
			}
			d := line - hi - t.Space(2)
			if !beforeSide {
				d = lo - line - t.Space(2)
			}
			e.Opacity(max(0, min(1, d/t.Space(4))))
		}
		fade(beforeCaption, true)
		fade(afterCaption, false)

		target := float32(0)
		if held || g.moved {
			target = 1
		}
		stretch := handle.Animate("stretch", target, 150*time.Millisecond)
		focus := handle.FocusVisible()
		cursor := ui.CursorResizeEW
		icon := lucide.ChevronsLeftRight()
		if vertical {
			cursor, icon = ui.CursorResizeNS, lucide.ChevronsUpDown()
		}
		if enabled {
			handle.Cursor(cursor)
		}
		// LayoutBounds describes the previous layout. Request one follow-up
		// frame when the frame changes size, as Split does.
		handle.Draw(func(p *ui.Painter, r ui.Rect) {
			if r != bounds {
				c.Invalidate()
			}
			pos := shown / 100
			bar := ui.Rect{X: r.X + r.W*pos - 1, Y: r.Y, W: 2, H: r.H}
			cx, cy := r.X+r.W*pos, r.Y+r.H/2
			w, h := t.Space(8), t.Space(8)*(1+.5*stretch)
			if vertical {
				bar = ui.Rect{X: r.X, Y: r.Y + r.H*pos - 1, W: r.W, H: 2}
				cx, cy = r.X+r.W/2, r.Y+r.H*pos
				w, h = h, w
			}
			p.Shadow(bar, 0, 0, 0, 6, 0, ui.RGBA(0, 0, 0, .25))
			p.Fill(bar, ui.RGB(255, 255, 255), 0)
			knob := ui.Rect{X: cx - w/2, Y: cy - h/2, W: w, H: h}
			paintSliderKnob(p, t, knob, stretch, focus)
			size := t.Space(4)
			color := ui.RGB(38, 38, 38).Mix(ui.RGB(255, 255, 255), stretch)
			p.Icon(icon, ui.Rect{X: cx - size/2, Y: cy - size/2, W: size, H: size}, color)
		})
	})
	dimmed(frame)
	return s
}
