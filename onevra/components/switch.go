package components

import (
	"math"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// SwitchOptions controls the visible label independently of the accessible name.
type SwitchOptions struct {
	// HideLabel hides the adjacent text while retaining the switch's accessible name.
	HideLabel bool
	// Compact draws a smaller switch for dense rows, such as table cells and
	// toolbars. Its knob grows into the same glass lens, scaled with it.
	Compact bool
}

// switchDrag follows one press of a switch: when and where it began,
// whether on the knob (grab), the knob's position then and, once the
// pointer travels past switchDragSlop, the position it drags the knob to.
// carry keeps the lens up after a drag until the knob comes to rest.
type switchDrag struct {
	pressed, grab, moved, carry bool
	x, from, pos                float32
	at                          time.Time
}

const (
	// switchDragSlop keeps a click with a little jitter a click.
	switchDragSlop = 3
	// switchTap is how long a press lasts before the knob lifts into a
	// lens. A shorter press is a tap, which toggles; letting go of a lifted
	// knob without dragging it leaves the switch as it was.
	switchTap = 120 * time.Millisecond
	// switchLift is how long the knob takes to grow into a lens and back.
	switchLift = 150 * time.Millisecond
)

// Switch creates a switch toggling *on, with a label. The label is also
// the accessible name.
//
// A tap, Space or Enter toggles it, sliding the knob across, as does a
// click beside the knob. Holding or dragging the knob lifts it as a glass
// lens; dragging moves it along the track, and releasing settles on the
// nearer side. Letting go of a held knob without dragging it sets the lens
// down and leaves the switch as it was. After a drag, the lens sets down
// once the knob comes to rest.
func Switch(c *ui.Context, on *bool, label string, options ...SwitchOptions) ui.Element {
	var o SwitchOptions
	if len(options) > 0 {
		o = options[0]
	}
	t := c.Theme()
	row := ui.Row(c).Gap(t.Space(2))
	row.Children(func() {
		w, h := t.Space(10), t.Space(5)
		if o.Compact {
			w, h = t.Space(7.5), t.Space(4)
		}
		// The resting knob is a capsule a little wider than it is high.
		in := t.Space(0.5)
		kh := h - 2*in
		kw := 1.3 * kh
		travel := w - 2*in - kw
		// SwitchBase toggles a copy, so a click ending a drag can be undone.
		value := *on
		sw := ui.SwitchBase(c, &value).Size(w, h).Radius(h / 2)
		if label != "" {
			sw.Label(label)
		}
		drag := ui.Local(sw, "switch-drag", func() switchDrag { return switchDrag{} })
		_, _, pressed := sw.Dragged()
		pressed = pressed && !sw.IsDisabled()
		x, _, _ := sw.PointerPosition()
		switch {
		case pressed:
			if !drag.pressed {
				from := boolToFloat(*on)
				left := in + from*travel
				*drag = switchDrag{pressed: true, x: x, from: from, at: c.Now(), grab: x >= left-in && x <= left+kw+in}
				if drag.grab {
					c.After(switchTap)
				}
			}
			if drag.grab && !drag.moved && max(x-drag.x, drag.x-x) >= switchDragSlop {
				drag.moved = true
			}
			if drag.moved {
				drag.pos = max(0, min(1, drag.from+(x-drag.x)/travel))
			}
		case drag.pressed:
			switch {
			case drag.moved:
				value = drag.pos >= 0.5
			case drag.grab && c.Now().Sub(drag.at) >= switchTap:
				// A held knob let go in place is no click.
				value = *on
			}
			*drag = switchDrag{carry: drag.moved}
		}
		if value != *on {
			*on = value
			sw.SetChanged()
		}
		sw.Checked(*on)
		var pos float32
		if drag.moved {
			pos = sw.Jump("knob", drag.pos)
		} else {
			pos = sw.AnimateTo("knob", boolToFloat(*on), ui.Animation{Duration: 200 * time.Millisecond, Ease: ui.EaseInOut})
		}
		if !pressed && sw.Settled("knob") {
			drag.carry = false
		}
		lifted := pressed && drag.grab && (drag.moved || c.Now().Sub(drag.at) >= switchTap)
		held := sw.Animate("held", boolToFloat(lifted || drag.carry), switchLift)
		off := t.Border.Mix(t.Text, 0.15)
		sw.Background(off.Mix(t.Accent, pos))
		sw.Draw(func(p *ui.Painter, r ui.Rect) {
			// Held, the knob grows into a lens well past the track, as
			// AppKit's does, so that the glass bends the track's edges.
			lw, lh := kw+(1.75*r.H-kw)*held, kh+(1.5*r.H-kh)*held
			cx, cy := r.X+in+kw/2+pos*travel, r.Y+r.H/2
			k := ui.Rect{X: cx - lw/2, Y: cy - lh/2, W: lw, H: lh}
			paintKnob(p, k, held, false, func(rad float32) { paintClearLens(p, k, rad, 0.3*min(lw, lh)) })
		})
		if label != "" && !o.HideLabel {
			Text(c, label)
		}
	})
	return dimmed(row)
}

// paintClearLens paints the Glass plugin's Clear material over k, curved
// only within bezel DIPs of its edge. glass.Paint curves up to half a small
// pane, which magnifies the track until it fills a knob-sized lens; AppKit's
// switch shows the track through a flat middle, as larger panes do. The
// parameters are glass.Paint's for Clear, which TestClearLensMatchesGlass
// checks.
func paintClearLens(p *ui.Painter, k ui.Rect, rad, bezel float32) {
	s := p.Scale()
	p.Effect(glass.Effect, k, [4]float32{rad, rad, rad, rad}, s, [5][4]float32{
		{bezel * s, 1.6 * bezel * s, 0.35, 1},
		{},
		{0.125, 1.082, 1, 1},
		{float32(math.Cos(math.Pi / 2)), float32(math.Sin(math.Pi / 2))},
		{},
	})
}

func boolToFloat(b bool) float32 {
	if b {
		return 1
	}
	return 0
}
