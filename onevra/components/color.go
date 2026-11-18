package components

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/auron-labs/mygo-framework/onevra/icons/lucide"
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// ColorControl exposes layout and an input-driven Changed result. Keep this
// handle when applying Element layout methods, which return ui.Element.
type ColorControl struct {
	ui.Element
	changed bool
}

// Changed reports a color value changed by input during this build.
func (c *ColorControl) Changed() bool { return c.changed }

type colorHSV struct{ h, s, v float64 }

func colorToHSV(c ui.Color) colorHSV {
	r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	hi, lo := max(r, g, b), min(r, g, b)
	d := hi - lo
	x := colorHSV{v: hi}
	if hi > 0 {
		x.s = d / hi
	}
	if d == 0 {
		return x
	}
	switch hi {
	case r:
		x.h = math.Mod((g-b)/d+6, 6) * 60
	case g:
		x.h = ((b-r)/d + 2) * 60
	default:
		x.h = ((r-g)/d + 4) * 60
	}
	return x
}

func (x colorHSV) color(alpha uint8) ui.Color {
	channel := func(n float64) uint8 {
		k := math.Mod(n+x.h/60, 6)
		v := x.v * (1 - x.s*max(0, min(k, 4-k, 1)))
		return uint8(math.Round(max(0, min(1, v)) * 255))
	}
	return ui.Color{R: channel(5), G: channel(3), B: channel(1), A: alpha}
}

func colorHex(c ui.Color) string { return fmt.Sprintf("%02X%02X%02X", c.R, c.G, c.B) }

func parseColorHex(s string, alpha uint8) (ui.Color, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 && len(s) != 8 {
		return ui.Color{}, false
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return ui.Color{}, false
	}
	if len(s) == 8 {
		alpha = uint8(v & 0xff)
		v >>= 8
	}
	return ui.Color{R: uint8((v >> 16) & 0xff), G: uint8((v >> 8) & 0xff), B: uint8(v & 0xff), A: alpha}, true
}

type colorViewState struct {
	hsv    colorHSV
	hsl    colorHSL
	last   ui.Color
	fields [5]string
	format colorFormat
}

func (s *colorViewState) sync(c ui.Color) {
	s.fields = [5]string{strconv.Itoa(int(c.R)), strconv.Itoa(int(c.G)), strconv.Itoa(int(c.B)), colorHex(c), colorDecimal(float64(c.A)*100/255) + "%"}
	if s.format != colorRGBA {
		for i := 0; i < 3; i++ {
			v, _ := s.channel(i, c)
			s.fields[i] = colorDecimal(v)
			if i > 0 {
				s.fields[i] += "%"
			}
		}
	}
	s.last = c
}

// ColorView edits an application-owned sRGB color in place. It includes a
// saturation/brightness plane, hue and opacity sliders, RGBA/HSBA/HSLA
// selection, RGB byte or HSB/HSL degree/percent fields, a six-digit Hex field (also
// accepts eight-digit RRGGBBAA), and opacity percent. Mode changes do not change
// the color or report Changed. The Copy color native menu copies RGBA, HSBA,
// HSLA or HEX text, including alpha. Numeric prefixes scrub horizontally by one
// unit per theme-scaled DIP and support arrow keys and Home/End. Hex uses a
// plain text field with a decorative prefix, without numeric stepping or scrubbing.
// Unsupported text is rejected; valid text applies immediately. Incomplete drafts survive while focused and
// reset on submission or blur. External changes replace drafts. Hue is retained
// through gray/black edits. RGB/HSV edits map wide-gamut inputs to sRGB;
// opacity-only edits preserve their gamut. Disable or key a parent before construction.
// The plane supports arrow keys; sliders retain MyGo keyboard/accessibility input.
func ColorView(c *ui.Context, value *ui.Color) *ColorControl {
	words := localized(c)
	if value == nil {
		panic("components.ColorView: nil color")
	}
	t := c.Theme()
	out := &ColorControl{Element: ui.Column(c).Width(t.Space(64)).MaxWidthPercent(100).MinWidth(0).Gap(t.Space(2)).Role(ui.RoleGroup)}
	st := ui.Local(out.Element, "color-view", func() colorViewState {
		s := colorViewState{hsv: colorToHSV(value.SRGB())}
		s.hsl = s.hsv.hsl()
		s.sync(*value)
		return s
	})
	if st.last != *value {
		st.hsv = colorToHSV(value.SRGB())
		st.hsl = st.hsv.hsl()
		st.sync(*value)
	}
	set := func(next ui.Color, source colorFormat) {
		if next != *value {
			*value = next
			out.changed = true
		}
		st.refreshModels(next, source)
		st.sync(*value)
	}
	out.Children(func() {
		plane := ui.Box(c).FillWidth().Height(t.Space(40)).Radius(t.Radius).Focusable().FocusRing(false).Role(ui.RoleSlider).Label(words.SaturationBrightness).Cursor(ui.CursorCrosshair)
		_, _, held := plane.Dragged()
		var knob ui.Element
		plane.Children(func() {
			// A real child hit target includes the part painted outside the plane.
			// The plane still owns keyboard focus and color coordinates.
			knob = ui.Box(c).Key("thumb").Absolute()
			_, _, knobHeld := knob.Dragged()
			held = held || knobHeld
		})
		target := float32(0)
		if held && !plane.IsDisabled() {
			target = 1
		}
		press := plane.Animate(0, target, 150*time.Millisecond)
		diameter := t.Space(4.5) * (1 + .15*press)
		x := st.hsv
		if !plane.IsDisabled() {
			if held {
				px, py, _ := plane.PointerPosition()
				r := plane.LayoutBounds()
				if r.W > 0 && r.H > 0 {
					x.s = float64(px / r.W)
					x.v = 1 - float64(py/r.H)
					plane.Focus()
				}
			}
			switch {
			case plane.Shortcut(0, ui.KeyRight):
				x.s += .01
			case plane.Shortcut(0, ui.KeyLeft):
				x.s -= .01
			case plane.Shortcut(0, ui.KeyUp):
				x.v += .01
			case plane.Shortcut(0, ui.KeyDown):
				x.v -= .01
			}
			x.s = max(0, min(1, x.s))
			x.v = max(0, min(1, x.v))
			if x != st.hsv {
				st.hsv = x
				set(x.color(value.A), colorHSBA)
			}
		}
		knob.Size(diameter, diameter).LeftPercent(float32(x.s)*100).TopPercent(float32(1-x.v)*100).
			Margin(-diameter/2, 0, 0, -diameter/2)
		plane.Range(0, 100, x.s*100).Step(1).Value(fmt.Sprintf("%.0f%% saturation, %.0f%% brightness", x.s*100, x.v*100))
		focus := plane.FocusVisible()
		plane.Draw(func(p *ui.Painter, r ui.Rect) {
			hue := colorHSV{h: st.hsv.h, s: 1, v: 1}.color(255)
			p.FillGradient(r, ui.LinearGradient{From: ui.RGB(255, 255, 255), To: hue, Angle: 90}, t.Radius)
			p.FillGradient(r, ui.LinearGradient{From: ui.RGBA(0, 0, 0, 0), To: ui.RGB(0, 0, 0), Angle: 180}, t.Radius)
			cx := r.X + r.W*float32(st.hsv.s)
			cy := r.Y + r.H*float32(1-st.hsv.v)
			paintColorPlaneKnob(p, ui.Rect{X: cx - diameter/2, Y: cy - diameter/2, W: diameter, H: diameter}, press)
			if focus {
				p.FocusRing(r, [4]float32{t.Radius, t.Radius, t.Radius, t.Radius})
			}
		})
		h := st.hsv.h
		hs := colorChannel(c, &h, 360, words.Hue, func(p *ui.Painter, r ui.Rect) {
			for i := 0; i < 6; i++ {
				a := colorHSV{h: float64(i) * 60, s: 1, v: 1}.color(255)
				b := colorHSV{h: float64(i+1) * 60, s: 1, v: 1}.color(255)
				p.FillGradient(ui.Rect{X: r.X + r.W*float32(i)/6, Y: r.Y, W: r.W/6 + .5, H: r.H}, ui.LinearGradient{From: a, To: b, Angle: 90}, 0)
			}
		})
		if hs.Changed() {
			st.hsv.h = h
			set(st.hsv.color(value.A), colorHSBA)
		}
		a := float64(value.A) / 255
		as := colorChannel(c, &a, 1, words.Opacity, func(p *ui.Painter, r ui.Rect) {
			paintColorChecks(p, r, t.Space(1.5))
			p.FillGradient(r, ui.LinearGradient{From: value.Alpha(0), To: value.Alpha(1), Angle: 90}, 0)
		})
		if as.Changed() {
			next := *value
			next.A = uint8(math.Round(a * 255))
			set(next, colorAlpha)
		}
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			labels := [...]string{"RGBA", "HSBA", "HSLA"}
			format := Button(c, "", ButtonGhost).Label(words.ColorFormat).Padding(0)
			format.Menu(func(m *ui.Menu) {
				for i, label := range labels {
					if m.Item(label).Checked(st.format == colorFormat(i)).Chosen() {
						st.format = colorFormat(i)
						st.sync(*value)
					}
				}
			})
			format.Children(func() {
				Text(c, labels[st.format]).SingleLine()
				Icon(c, lucide.ChevronDown()).Size(t.Space(3), t.Space(3)).TextColor(t.TextMuted)
			})
			ui.Box(c).Grow(1)
			copy := Button(c, "", ButtonGhost).Label(words.CopyColor).Size(t.Space(6), t.Space(6)).Padding(t.Space(1.5))
			copy.Children(func() { Icon(c, lucide.Copy()).Size(t.Space(3), t.Space(3)).TextColor(t.TextMuted) })
			copy.Menu(func(m *ui.Menu) {
				for _, format := range []string{"RGBA", "HSBA", "HSLA", "HEX"} {
					if m.Item(format).Chosen() {
						c.WriteClipboard(colorCopyText(*value, format))
					}
				}
			})
		})
		field := func(i int, label string) { colorField(c, st, value, i, label, set) }
		ui.Row(c).FillWidth().Gap(t.Space(2)).Children(func() {
			labels := [3]string{"R", "G", "B"}
			if st.format == colorHSBA {
				labels = [3]string{"H", "S", "B"}
			}
			if st.format == colorHSLA {
				labels = [3]string{"H", "S", "L"}
			}
			for i, label := range labels {
				field(i, label)
			}
		})
		ui.Row(c).FillWidth().Gap(t.Space(2)).Children(func() { field(3, "#"); field(4, "A") })
	})
	dimmed(out.Element)
	return out
}

func paintColorPlaneKnob(p *ui.Painter, k ui.Rect, held float32) {
	rad := k.H / 2
	if held > 0 {
		glass.Paint(p, k, rad, glass.Glass{Style: glass.Clear})
	}
	p.Stroke(k, ui.RGBA(0, 0, 0, .4), rad, 3-held)
	p.Stroke(k, ui.RGBA(255, 255, 255, 1-.15*held), rad, 2-held)
}

func colorChannel(c *ui.Context, value *float64, hi float64, label string, paint func(*ui.Painter, ui.Rect)) ui.Element {
	t := c.Theme()
	d := t.Space(5)
	var e ui.Element
	ui.Box(c).FillWidth().Height(d).Children(func() {
		// Keep a generous hit area around the narrow thumb while aligning the
		// visible track with the plane. SliderBase maps its padded content box.
		e = ui.SliderBase(c, value, 0, hi).Absolute().Left(-d / 2).Right(-d / 2).
			Height(d).PaddingX(d / 2).Label(label).FocusRing(false)
	})
	// Apply pending input before capturing the fraction for this frame's paint.
	e.Changed()
	fraction := float32(*value / hi)
	_, _, held := e.Dragged()
	target := float32(0)
	if held && !e.IsDisabled() {
		target = 1
	}
	glass := e.Animate(0, target, 150*time.Millisecond)
	focus := e.FocusVisible()
	e.Draw(func(p *ui.Painter, r ui.Rect) {
		bar := ui.Rect{X: r.X + d/2, Y: r.Y + d*.2, W: max(0, r.W-d), H: d * .6}
		p.Clip(bar, bar.H/2, func() { paint(p, bar) })
		x, y := bar.X+bar.W*fraction, r.Y+d/2
		grow := 1 + .35*glass
		w, h := t.Space(1.5)*grow, t.Space(4)*grow
		knob := ui.Rect{X: x - w/2, Y: y - h/2, W: w, H: h}
		paintSliderKnob(p, t, knob, glass, focus)
		if glass > 0 {
			rad := min(w, h) / 2
			p.Stroke(knob, ui.RGBA(0, 0, 0, .4*glass), rad, 2)
			p.Stroke(knob, ui.RGBA(255, 255, 255, .85*glass), rad, 1)
		}
	})
	return e
}

func paintColorChecks(p *ui.Painter, r ui.Rect, size float32) {
	p.Fill(r, ui.RGB(255, 255, 255), 0)
	for row, y := 0, r.Y; y < r.Y+r.H; row, y = row+1, y+size {
		for col, x := 0, r.X; x < r.X+r.W; col, x = col+1, x+size {
			if (row+col)%2 == 0 {
				p.Fill(ui.Rect{X: x, Y: y, W: min(size, r.X+r.W-x), H: min(size, r.Y+r.H-y)}, ui.RGB(205, 205, 205), 0)
			}
		}
	}
}

// ColorPicker creates a rectangular color swatch opening ColorView in a
// dismissible Popover. Size, Border (stroke width/color), Radius and Padding can
// be customized on the returned Element. Use Label for its accessible name.
// Enter/Space activate; Escape/outside press dismiss. Changed reports edits,
// not opening/closing. Key or disable a parent, not the returned element.
func ColorPicker(c *ui.Context, value *ui.Color) *ColorControl {
	if value == nil {
		panic("components.ColorPicker: nil color")
	}
	t := c.Theme()
	out := &ColorControl{Element: ui.ButtonBase(c).Size(t.Space(12), t.Space(8)).Padding(t.Space(1)).Radius(t.Radius/2).Border(1, t.Border).Clip().Role(ui.RoleColorWell)}
	open := ui.Local(out.Element, "color-open", func() bool { return false })
	if out.IsDisabled() {
		*open = false
	} else if out.Clicked() {
		*open = !*open
	}
	out.Children(func() {
		ui.Box(c).Fill().Radius(t.Radius / 2).Clip().Draw(func(p *ui.Painter, r ui.Rect) { paintColorChecks(p, r, t.Space(1.5)); p.Fill(r, *value, 0) })
	})
	Popover(c, out.Element, open, func(panel ui.Element) {
		panel.Padding(t.Space(3)).MaxWidthPercent(100)
		view := ColorView(c, value)
		out.changed = view.Changed()
	})
	out.Expanded(*open).Value("#" + colorHex(*value)).Description(fmt.Sprintf("#%s, %.0f%% opacity", colorHex(*value), float64(value.A)*100/255))
	dimmed(out.Element)
	return out
}
