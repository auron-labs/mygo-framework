package components

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// RatingControl retains value changes separately from Element styling.
type RatingControl struct {
	ui.Element
	changed bool
}

// Changed reports a user-driven value change, not initial clamping.
func (r *RatingControl) Changed() bool { return r.changed }

// Rating creates max stars bound to an application-owned integer. Values are
// clamped to [0, max]; a negative max means zero stars. A nil value is invalid.
// Clicking the selected star clears the rating. Arrows, Home and End operate
// on the focused control. Label supplies its accessible name. Keep the control
// to query Changed after applying Element methods. Key a parent for identity.
func Rating(c *ui.Context, value *int, maximum int) *RatingControl {
	if value == nil {
		panic("components.Rating: nil value")
	}
	maximum = max(0, maximum)
	*value = min(maximum, max(0, *value))
	t := c.Theme()
	r := &RatingControl{Element: ui.Row(c).Gap(t.Space(.5)).Shrink(0).Focusable().Role(ui.RoleSlider)}
	set := func(v int) {
		v = min(maximum, max(0, v))
		if v != *value {
			*value, r.changed = v, true
		}
	}
	if !r.IsDisabled() {
		switch {
		case r.Shortcut(0, ui.KeyRight), r.Shortcut(0, ui.KeyUp):
			set(*value + 1)
		case r.Shortcut(0, ui.KeyLeft), r.Shortcut(0, ui.KeyDown):
			set(*value - 1)
		case r.Shortcut(0, ui.KeyHome):
			set(0)
		case r.Shortcut(0, ui.KeyEnd):
			set(maximum)
		}
	}
	hover := -1
	r.Element.Children(func() {
		for i := range maximum {
			star := ui.Box(c).Size(t.Rem(1.5), t.Rem(1.5)).Shrink(0).Role(ui.RoleNone)
			if star.Clicked() && !r.IsDisabled() {
				if *value == i+1 {
					set(0)
				} else {
					set(i + 1)
				}
				r.Focus()
			}
			if star.Hovered() && !r.IsDisabled() {
				hover = i
			}
			star.Draw(func(p *ui.Painter, bounds ui.Rect) {
				filled := i < *value
				color := t.Warning
				if hover >= 0 {
					filled, color = i <= hover, color.Alpha(.6)
				}
				path := ratingStar(bounds)
				if filled {
					p.FillPath(path, color)
					p.StrokePath(path, 1.2, color)
				} else {
					p.StrokePath(path, 1.2, t.TextMuted)
				}
			})
		}
	})
	r.Range(0, float64(maximum), float64(*value))
	dimmed(r.Element)
	return r
}

func ratingStar(r ui.Rect) *ui.Path {
	var path ui.Path
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	outer, inner := min(r.W, r.H)*.44, min(r.W, r.H)*.19
	for i := range 10 {
		radius := outer
		if i%2 != 0 {
			radius = inner
		}
		angle := float64(i)*math.Pi/5 - math.Pi/2
		x, y := cx+radius*float32(math.Cos(angle)), cy+radius*float32(math.Sin(angle))
		if i == 0 {
			path.MoveTo(x, y)
		} else {
			path.LineTo(x, y)
		}
	}
	path.Close()
	return &path
}
