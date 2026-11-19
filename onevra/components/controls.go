package components

import "github.com/egoist/mygo/ui"

// toolbarFace returns the raised face of a chosen segment or toolbar control,
// with its hover and pressed shades and its cast shadow: a lighter surface in
// either appearance.
func toolbarFace(t *ui.Theme) (face, hover, pressed, shadow ui.Color) {
	if t.Dark {
		return t.SurfaceHover, t.SurfaceHover.Mix(t.Text, 0.08), t.SurfacePressed, ui.RGBA(0, 0, 0, 0.4)
	}
	return t.Surface, t.SurfaceHover, t.SurfacePressed, ui.RGBA(0, 0, 0, 0.14)
}

// toolbarTrack returns the recessed track behind a segmented tab list: the
// foreground color at low opacity, so it recesses whatever surface it sits on.
func toolbarTrack(t *ui.Theme) ui.Color { return t.Text.Alpha(0.06) }

// dimmed applies the design style's disabled treatment. The bases own the
// disabled semantics; without their styleFn hook, opacity is the faithful
// visual equivalent.
func dimmed(e ui.Element) ui.Element {
	if e.IsDisabled() {
		e.Opacity(0.45)
	}
	return e
}

// TouchMin is the smallest square hit target Onevra's interactive controls
// take under a coarse pointer: 44 DIPs, the minimum iOS asks and MyGo's
// ui/mobile rows and bars use (Android asks 48 dp; one minimum serves both,
// and MyGo's finger slop covers near-misses). Under a fine pointer it is 0,
// where controls keep the compact sizes their desktop layouts are built
// around. Interactive parts that cannot take the minimum without breaking
// the rows they sit in are compact use, documented in the README.
func TouchMin(c *ui.Context) float32 {
	if c.Preferences().Pointer == ui.PointerCoarse {
		return 44
	}
	return 0
}

// fontDIPs returns n DIPs of type following the desktop's text scale the way
// the theme's FontSize does: n at the usual scale, so a text-scale-1 layout
// keeps its sizes, and n times the scale otherwise, as phones report iOS
// Dynamic Type and Android's font scale.
func fontDIPs(c *ui.Context, n float32) float32 {
	if s := c.Preferences().TextScale; s > 0 && s != 1 {
		return n * s
	}
	return n
}

// fingerSlop is how far a finger moves, in DIPs, before it is known to drag a
// control rather than pan what holds it, as MyGo's touch slop.
const fingerSlop = 8

// fingerAxis is the axis a control follows a finger along.
type fingerAxis uint8

const (
	fingerHorizontal fingerAxis = iota + 1
	fingerVertical
	fingerBoth
)

// fingerTrack follows a finger on a control across a touchscreen. Linux and
// Windows report a finger as raw touches beside the scrolling they make of
// it, where phones raise pointer drags; a touchpad reports no touches, so its
// scrolling never drags a control.
type fingerTrack struct {
	id             uint64
	axis           fingerAxis
	startX, startY float32
	x, y           float32
	decided        bool
	dragging       bool
}

// follow takes the fingers on e, reporting how far the finger that drags the
// control moved since the last build. Call it each build. A finger that moves
// mostly across a single axis is left to pan what scrolls.
func (f *fingerTrack) follow(e ui.Element, axis fingerAxis, moved func(dx, dy float32)) {
	f.axis = axis
	touches := e.Touches()
	if len(touches) == 0 {
		*f = fingerTrack{axis: axis}
		return
	}
	for _, t := range touches {
		switch t.Phase {
		case ui.TouchBegan:
			if f.id == 0 {
				*f = fingerTrack{id: t.ID, axis: axis, startX: t.X, startY: t.Y, x: t.X, y: t.Y}
			}
		case ui.TouchMoved:
			if t.ID != f.id {
				continue
			}
			dx, dy := t.X-f.startX, t.Y-f.startY
			if !f.decided {
				if abs32(dx) < fingerSlop && abs32(dy) < fingerSlop {
					continue
				}
				f.decided = true
				switch axis {
				case fingerHorizontal:
					f.dragging = abs32(dx) > abs32(dy)
				case fingerVertical:
					f.dragging = abs32(dy) > abs32(dx)
				default:
					f.dragging = true
				}
			}
			if !f.dragging {
				continue
			}
			stepX, stepY := t.X-f.x, t.Y-f.y
			f.x, f.y = t.X, t.Y
			if stepX != 0 || stepY != 0 {
				moved(stepX, stepY)
			}
		case ui.TouchEnded, ui.TouchCanceled:
			if t.ID == f.id {
				*f = fingerTrack{axis: axis}
			}
		}
	}
}

// takesScroll reports whether ev is the scrolling a raw finger makes while it
// drags the control, which the control takes rather than letting the page pan.
// The first scroll while the finger is down decides: across a single axis it
// leaves the gesture to the page. A touchpad reports no touches, so its
// scrolling is left alone.
func (f *fingerTrack) takesScroll(ev ui.InputEvent) bool {
	if f.id == 0 || ev.Kind != ui.InputScroll {
		return false
	}
	if f.dragging {
		return true
	}
	if f.decided {
		return false
	}
	along := true
	switch f.axis {
	case fingerHorizontal:
		along = abs32(ev.DX) > abs32(ev.DY)
	case fingerVertical:
		along = abs32(ev.DY) > abs32(ev.DX)
	}
	f.decided, f.dragging = true, along
	return along
}
