package components

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// Progress creates a horizontal progress bar. Values in [0, 1] show completion;
// negative values show indeterminate activity. Values above 1 clamp to 1 and
// NaN is treated as 0. Name the indicator with Label for accessibility.
// The default height is theme-scaled; callers control width with layout methods.
func Progress(c *ui.Context, value float64) ui.Element {
	t := c.Theme()
	value = progressValue(value)
	reduced := c.Preferences().ReduceMotion
	e := ui.Box(c).Height(t.Space(1.5)).FillWidth().Role(ui.RoleProgress).Range(0, 1, value)
	e.Draw(func(p *ui.Painter, r ui.Rect) {
		if r.W <= 0 || r.H <= 0 {
			return
		}
		rad := r.H / 2
		p.Fill(r, t.Border, rad)
		x, w := r.X, r.W*float32(value)
		if value < 0 {
			phase := progressPhase(p, reduced)
			w = r.W * 0.3
			x = r.X + (r.W-w)*float32((1-math.Cos(phase*2*math.Pi))/2)
		}
		if w > 0 {
			p.Clip(r, rad, func() { p.Fill(ui.Rect{X: x, Y: r.Y, W: w, H: r.H}, t.Accent, rad) })
		}
	})
	return e
}

// ProgressRing creates a circular progress indicator with the same value
// semantics as Progress. Completion starts at twelve o'clock and runs clockwise.
// Its default diameter is theme-scaled. A custom non-square layout keeps the
// ring circular and centered. Reduced Motion leaves indeterminate activity static.
func ProgressRing(c *ui.Context, value float64) ui.Element {
	t := c.Theme()
	value = progressValue(value)
	reduced := c.Preferences().ReduceMotion
	e := ui.Box(c).Size(t.Space(8), t.Space(8)).Role(ui.RoleProgress).Range(0, 1, value)
	e.Draw(func(p *ui.Painter, r ui.Rect) {
		d := min(r.W, r.H)
		if d <= 0 {
			return
		}
		width := min(t.Space(0.75), d/4)
		radius := (d - width) / 2
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		var track ui.Path
		track.Circle(cx, cy, radius)
		p.StrokePath(&track, width, t.Border)
		start, sweep := -math.Pi/2, value*2*math.Pi
		if value < 0 {
			start += progressPhase(p, reduced) * 2 * math.Pi
			sweep = math.Pi / 2
		}
		if sweep <= 0 {
			return
		}
		var fill ui.Path
		if value == 1 {
			fill.Circle(cx, cy, radius)
		} else {
			progressArc(&fill, cx, cy, radius, start, sweep)
		}
		p.StrokePath(&fill, width, t.Accent)
	})
	return e
}

func progressValue(value float64) float64 {
	if math.IsNaN(value) {
		return 0
	}
	if value < 0 {
		return -1
	}
	return min(value, 1)
}

func progressPhase(p *ui.Painter, reduced bool) float64 {
	if reduced {
		return 0.25
	}
	p.AnimationFrame()
	return float64(p.Now().UnixMilli()%1400) / 1400
}

// Approximate each quarter-turn with a cubic Bezier using MyGo's path primitive.
func progressArc(path *ui.Path, cx, cy, radius float32, start, sweep float64) {
	point := func(a float64) (float32, float32) {
		return cx + radius*float32(math.Cos(a)), cy + radius*float32(math.Sin(a))
	}
	x, y := point(start)
	path.MoveTo(x, y)
	steps := int(math.Ceil(sweep / (math.Pi / 2)))
	delta := sweep / float64(steps)
	k := float32(4.0/3*math.Tan(delta/4)) * radius
	for i := 0; i < steps; i++ {
		a, b := start+float64(i)*delta, start+float64(i+1)*delta
		x0, y0 := point(a)
		x1, y1 := point(b)
		path.CubeTo(x0-k*float32(math.Sin(a)), y0+k*float32(math.Cos(a)), x1+k*float32(math.Sin(b)), y1-k*float32(math.Cos(b)), x1, y1)
	}
}
