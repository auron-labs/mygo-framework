package chat

import (
	"math"

	"github.com/auron-labs/mygo-framework/onevra/components"
	"github.com/egoist/mygo/ui"
)

// ContextMeterOptions describes context-window occupancy. Fraction is 0..1,
// or negative when nothing is measured. Label is the trigger's accessible
// name and Tooltip its hover text; Title and Value head the panel, such as
// "Context window" and "61k / 272k (22%)".
type ContextMeterOptions struct {
	Label, Tooltip, Title, Value string
	Fraction                     float64
}

// MeterState retains whether a meter's panel is open.
type MeterState struct{ Open bool }

// ContextMeter is a small ring gauge, suited to the composer's Trailing slot,
// that opens a panel with the value and a bar. It fills with the accent,
// warns from 80% and alerts from 95%. The application measures usage.
func ContextMeter(c *ui.Context, state *MeterState, options ContextMeterOptions) ui.Element {
	t := c.Theme()
	fill := meterColor(t, options.Fraction)
	b := ui.ButtonBase(c).Label(options.Label).Height(max(float32(20), components.TouchMin(c))).
		MinWidth(max(float32(23), components.TouchMin(c))).Padding(0, 5).Radius(5).Shrink(0).Center().Background(ui.Color{})
	if state.Open {
		b.Background(overlayStrong(t))
	} else if b.Hovered() || b.Focused() {
		b.Background(overlay(t))
	}
	b.Children(func() { gauge(c, options.Fraction, fill) })
	if options.Tooltip != "" {
		components.Tooltip(c, b, options.Tooltip)
	}
	if b.Clicked() {
		state.Open = !state.Open
	}
	components.Popover(c, b, &state.Open, func(panel ui.Element) {
		panel.Label(options.Title).Width(320).Padding(14).Gap(7).Children(func() {
			ui.Row(c).FillWidth().MinWidth(0).AlignItems(ui.Center).Gap(8).Children(func() {
				components.Text(c, options.Title).Grow(1).MinWidth(0).SingleLine().Ellipsis("…").FontSize(12.5)
				components.Text(c, options.Value).Shrink(0).FontSize(12.5).TextColor(tertiary(t))
			})
			bar(c, options.Fraction, fill).FillWidth().Height(6)
		})
	})
	return b
}

// bar is a track filled to fraction; negative draws only the track. Size it
// with the returned element.
func bar(c *ui.Context, fraction float64, fill ui.Color) ui.Element {
	value := max(fraction, 0)
	return ui.Box(c).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(r, p.Theme().Border, r.H/2)
		if w := r.W * float32(min(value, 1)); w > 0 {
			p.Fill(ui.Rect{X: r.X, Y: r.Y, W: max(w, r.H), H: r.H}, fill, r.H/2)
		}
	})
}

func meterColor(t *ui.Theme, fraction float64) ui.Color {
	switch {
	case fraction >= .95:
		return t.Danger
	case fraction >= .8:
		return t.Warning
	}
	return t.Accent
}

// gauge draws a ring on the theme border; a fraction at or below zero draws
// only the track.
func gauge(c *ui.Context, fraction float64, fill ui.Color) {
	ui.Box(c).Size(13, 13).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
		const stroke = 2.5
		radius := (r.W - stroke) / 2
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		var ring ui.Path
		ring.Circle(cx, cy, radius)
		p.StrokePath(&ring, stroke, p.Theme().Border)
		if fraction <= 0 {
			return
		}
		var arc ui.Path
		if fraction >= 1 {
			arc.Circle(cx, cy, radius)
		} else {
			arcPath(&arc, cx, cy, radius, fraction)
		}
		p.StrokePath(&arc, stroke, fill)
	})
}

// arcPath draws a clockwise arc from 12 o'clock with cubic segments.
func arcPath(path *ui.Path, cx, cy, radius float32, fraction float64) {
	start, sweep := -math.Pi/2, fraction*2*math.Pi
	point := func(a float64) (float32, float32) {
		return cx + radius*float32(math.Cos(a)), cy + radius*float32(math.Sin(a))
	}
	x, y := point(start)
	path.MoveTo(x, y)
	steps := int(math.Ceil(sweep / (math.Pi / 2)))
	delta := sweep / float64(steps)
	k := float32(4.0/3*math.Tan(delta/4)) * radius
	for i := range steps {
		a, b := start+float64(i)*delta, start+float64(i+1)*delta
		x0, y0 := point(a)
		x1, y1 := point(b)
		path.CubeTo(x0-k*float32(math.Sin(a)), y0+k*float32(math.Cos(a)), x1+k*float32(math.Sin(b)), y1-k*float32(math.Cos(b)), x1, y1)
	}
}
