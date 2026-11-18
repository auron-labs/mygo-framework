package components

import (
	"math"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// SliderOptions selects value behavior, not a visual variant.
type SliderOptions struct {
	// Step > 0 snaps input to multiples of Step from lo and displays ticks.
	// Zero keeps continuous pointer input and a keyboard step of (hi-lo)/100.
	Step float64
	// Range enables two thumbs: value is the minimum and Range is the maximum.
	// The pointers must be distinct. Nil keeps the single-thumb slider.
	Range *float64
	// Accessible names for range thumbs; defaults are "minimum" and "maximum".
	MinimumLabel, MaximumLabel string
}

// SliderControl exposes the underlying layout element. Keep this handle to
// query Changed after applying Element layout methods, which return ui.Element.
type SliderControl struct {
	ui.Element
	thumbs  []ui.Element
	labels  [2]string
	changed bool
	held    [2]bool
}

// Changed reports an input-driven change to either value, not initial clamping.
func (s *SliderControl) Changed() bool { return s.changed }

// Label names the slider and, in range mode, both independently focusable thumbs.
func (s *SliderControl) Label(label string) *SliderControl {
	s.Element.Label(label)
	for i, thumb := range s.thumbs {
		name := s.labels[i]
		if label != "" {
			name = label + " " + name
		}
		thumb.Label(name)
	}
	return s
}

// AutoFocus focuses the single thumb or the minimum thumb of a range.
func (s *SliderControl) AutoFocus() *SliderControl {
	if len(s.thumbs) > 0 {
		s.thumbs[0].AutoFocus()
	} else {
		s.Element.AutoFocus()
	}
	return s
}

// Slider creates a continuous, stepped or two-thumb slider. Existing calls can
// omit options. Range and Step may be combined. Applications own both values;
// range values are clamped and ordered on construction. Bounds must be finite
// with hi >= lo; values and Step must be finite. Nonpositive Step is continuous.
//
//	s := Slider(c, &low, 0, 100, SliderOptions{Range: &high, Step: 5})
//	s.Label("Price").FillWidth()
//	if s.Changed() { /* use low and high */ }
//
// Pointer input uses MyGo's drag capture. Tab moves between range thumbs;
// arrows, PageUp/PageDown, Home and End adjust the focused thumb without crossing.
func Slider(c *ui.Context, value *float64, lo, hi float64, options ...SliderOptions) *SliderControl {
	if len(options) > 1 {
		panic("components.Slider: at most one options value")
	}
	o := SliderOptions{}
	if len(options) > 0 {
		o = options[0]
	}
	if o.Range == value {
		panic("components.Slider: range values must use distinct pointers")
	}
	t := c.Theme()
	thumb := t.Space(5)
	var e ui.Element
	s := &SliderControl{labels: [2]string{o.MinimumLabel, o.MaximumLabel}}
	if s.labels[0] == "" {
		s.labels[0] = localized(c).Minimum
	}
	if s.labels[1] == "" {
		s.labels[1] = localized(c).Maximum
	}
	if o.Range == nil {
		e = ui.SliderBase(c, value, lo, hi).Step(o.Step)
		s.changed = e.Changed()
	} else {
		e = ui.Box(c).Role(ui.RoleGroup)
	}
	s.Element = e
	e.Height(t.Space(5)).MinWidth(t.Space(20)).PaddingX(thumb / 2).FocusRing(false)
	if o.Step > 0 {
		e.Height(t.Space(7))
	}
	if o.Range != nil {
		sliderRange(c, s, value, o.Range, lo, hi, o.Step, thumb)
		s.Label("")
	} else {
		_, _, held := e.Dragged()
		s.held[0] = held && !e.IsDisabled()
	}
	var glass [2]float32
	for i, held := range s.held {
		target := float32(0)
		if held {
			target = 1
		}
		glass[i] = e.Animate(i, target, 150*time.Millisecond)
	}
	// Paint runs after the build pass that owns the elements, so their focus
	// is read now.
	var focus [2]bool
	for i := range focus {
		focus[i] = e.FocusVisible()
		if i < len(s.thumbs) {
			focus[i] = s.thumbs[i].FocusVisible()
		}
	}
	e.Draw(func(p *ui.Painter, r ui.Rect) {
		h, cy := t.Space(1), r.Y+t.Space(2.5)
		tx, tw := r.X+thumb/2, max(float32(0), r.W-thumb)
		x0, x1 := tx, tx+tw*sliderFraction(*value, lo, hi)
		positions := []float32{x1}
		if o.Range != nil {
			x0, x1 = x1, tx+tw*sliderFraction(*o.Range, lo, hi)
			positions = []float32{x0, x1}
		}
		p.Fill(ui.Rect{X: tx, Y: cy - h/2, W: tw, H: h}, t.Border, h/2)
		p.Fill(ui.Rect{X: x0, Y: cy - h/2, W: max(float32(0), x1-x0), H: h}, t.Accent, h/2)
		// Bound tick density and work for tiny steps; snapping remains unchanged.
		if o.Step > 0 && hi > lo {
			count := math.Floor((hi - lo) / o.Step)
			if count < 61 {
				for i := 0; i <= int(count); i++ {
					x := tx + tw*sliderFraction(lo+float64(i)*o.Step, lo, hi)
					p.Fill(ui.Rect{X: x - t.Space(0.25), Y: cy + t.Space(3), W: t.Space(0.5), H: t.Space(0.5)}, t.Border, t.Space(0.25))
				}
			}
		}
		for i, x := range positions {
			paintSliderKnob(p, t, sliderKnobRect(t, x, cy, glass[i]), glass[i], focus[i])
		}
	})
	dimmed(e)
	return s
}

func sliderKnobRect(t *ui.Theme, x, y, held float32) ui.Rect {
	grow := 1 + 0.35*held
	w, h := t.Space(5)*grow, t.Space(4)*grow
	return ui.Rect{X: x - w/2, Y: y - h/2, W: w, H: h}
}

// MyGo owns the glass rendering and backdrop sampling.
func paintSliderKnob(p *ui.Painter, t *ui.Theme, k ui.Rect, held float32, focus bool) {
	paintKnob(p, k, held, focus, func(rad float32) { glass.Paint(p, k, rad, glass.Glass{Style: glass.Clear}) })
}

// paintKnob paints an opaque knob that turns into the glass lens paints
// as held goes to 1.
func paintKnob(p *ui.Painter, k ui.Rect, held float32, focus bool, lens func(rad float32)) {
	face, drop := ui.RGB(255, 255, 255), ui.RGBA(0, 0, 0, .14)
	rad := min(k.W, k.H) / 2
	if held > 0 {
		p.Shadow(k, rad, 0, 8, 14, -3, drop.Alpha(held))
	}
	if held < 1 {
		p.Shadow(k, rad, 0, .5, 1, 0, ui.RGBA(0, 0, 0, .08*(1-held)))
		p.Shadow(k, rad, 0, 1.5, 7, 0, ui.RGBA(0, 0, 0, .1*(1-held)))
	}
	if held > 0 {
		lens(rad)
	}
	// Keep the original opaque face at rest, revealing Clear glass on press.
	if held < 1 {
		p.Fill(k, face.Alpha(1-held), rad)
	}
	if focus {
		p.FocusRing(k, [4]float32{rad, rad, rad, rad})
	}
}

func sliderFraction(v, lo, hi float64) float32 {
	if hi <= lo {
		return 0
	}
	return float32(max(0, min(1, (v-lo)/(hi-lo))))
}

func sliderSnap(v, lo, hi, step float64) float64 {
	v = max(lo, min(hi, v))
	if step > 0 {
		n := math.Round((v - lo) / step)
		n = min(n, math.Floor((hi-lo)/step+1e-10))
		v = max(lo, min(hi, lo+n*step))
	}
	return v
}

func sliderRange(c *ui.Context, s *SliderControl, low, high *float64, lo, hi, step float64, thumb float32) {
	e := s.Element
	*low, *high = max(lo, min(hi, min(*low, *high))), max(lo, min(hi, max(*low, *high)))
	values := []*float64{low, high}
	keyStep := step
	if keyStep <= 0 {
		keyStep = (hi - lo) / 100
	}
	set := func(k int, v float64) {
		v = sliderSnap(v, lo, hi, step)
		if k == 0 {
			v = min(v, *high)
		} else {
			v = max(v, *low)
		}
		if *values[k] != v {
			*values[k] = v
			s.changed = true
		}
	}
	dragging := ui.Local(e, "slider-thumb", func() int { return -1 })
	_, _, trackHeld := e.Dragged()
	held := trackHeld
	e.Children(func() {
		for k, v := range values {
			knob := ui.Box(c).Key(k).Absolute().Top(0).Width(thumb).Height(c.Theme().Space(5)).Focusable().FocusRing(false).Role(ui.RoleSlider)
			_, _, pressed := knob.Dragged()
			if pressed {
				*dragging = k
				held = true
			}
			s.thumbs = append(s.thumbs, knob)
			switch {
			case knob.Shortcut(0, ui.KeyLeft), knob.Shortcut(0, ui.KeyDown):
				set(k, *v-keyStep)
			case knob.Shortcut(0, ui.KeyRight), knob.Shortcut(0, ui.KeyUp):
				set(k, *v+keyStep)
			case knob.Shortcut(0, ui.KeyPageDown):
				set(k, *v-max(keyStep, (hi-lo)/10))
			case knob.Shortcut(0, ui.KeyPageUp):
				set(k, *v+max(keyStep, (hi-lo)/10))
			case knob.Shortcut(0, ui.KeyHome):
				if k == 0 {
					set(k, lo)
				} else {
					set(k, *low)
				}
			case knob.Shortcut(0, ui.KeyEnd):
				if k == 0 {
					set(k, *high)
				} else {
					set(k, hi)
				}
			}
		}
	})
	if held && !e.IsDisabled() {
		x, _, _ := e.PointerPosition()
		width := e.LayoutBounds().W - thumb
		if width > 0 {
			v := lo + float64(max(float32(0), min(float32(1), (x-thumb/2)/width)))*(hi-lo)
			if *dragging < 0 {
				*dragging = 0
				if math.Abs(v-*high) < math.Abs(v-*low) || v > *high {
					*dragging = 1
				}
			}
			set(*dragging, v)
			s.thumbs[*dragging].Focus()
		}
	}
	if !held {
		*dragging = -1
	} else if *dragging >= 0 && !e.IsDisabled() {
		s.held[*dragging] = true
	}
	for k, knob := range s.thumbs {
		frac := sliderFraction(*values[k], lo, hi)
		knob.LeftPercent(frac*100).Margin(0, 0, 0, -frac*thumb)
		from, to := lo, *high
		if k == 1 {
			from, to = *low, hi
		}
		knob.Range(from, to, *values[k]).Step(keyStep)
	}
}
