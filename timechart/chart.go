// Package timechart draws continuous time-series charts in native UI: a
// time or numeric x-axis with clock-aligned ticks, zoom at the cursor, drag
// pan, double-click reset, line, step and min/max band series, event markers
// and a hover crosshair with a readout.
//
// Charts that share a Range zoom, pan and show the crosshair together, so
// stacked rows stay aligned. Series draw whatever a Provider returns for the
// visible window and plot width, fetched off the UI thread while the last
// result stays on screen. Decimate is a provider for in-memory samples.
//
//	rng := timechart.NewRange(start, end)
//	chart := &timechart.Chart{Range: rng, Label: "Heart rate", Series: []timechart.Series{{
//		ID: "hr", Label: "Heart rate", Provider: func(_ context.Context, from, to float64, w int) (timechart.Points, error) {
//			return timechart.Decimate(xs, ys, from, to, w), nil
//		},
//	}}}
//	chart.Build(c) // every frame
package timechart

import (
	"math"
	"sort"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"
)

// Kind is how a series is drawn.
type Kind int

const (
	// Line joins the points with straight lines.
	Line Kind = iota
	// Step holds each value until the next point.
	Step
	// Band fills between the lowest and highest point of each pixel column.
	Band
)

// Series is one drawn channel. ID must be unique in its chart and stable.
// A zero Color takes a theme color by series order.
type Series struct {
	ID, Label string
	Kind      Kind
	Color     ui.Color
	Provider  Provider
}

// Event marks a moment, or the interval Start to End when End is greater.
// Intervals are shaded; both get a flag with Label. A zero Color uses the
// theme's danger color.
type Event struct {
	Start, End float64
	Label      string
	Color      ui.Color
}

// Chart is one plot row. Keep it between frames: it holds the fetched
// points. Fill the exported fields before the first Build.
type Chart struct {
	// Range is the x-interval shared with the charts it is linked to.
	Range *Range
	// Label names the chart for assistive technology.
	Label  string
	Series []Series
	Events []Event
	// Height of the chart in DIPs including its axis; 0 is 160.
	Height float32
	// Numeric shows plain numbers on the x-axis instead of clock times for
	// x in Unix seconds.
	Numeric bool
	// Location is where clock times are read; nil is time.Local.
	Location *time.Location
	// HideXAxis leaves out the x labels, for all but the last of a stack.
	HideXAxis bool
	// YMin and YMax fix the y-axis; the axis fits the visible data when
	// YMax is not above YMin.
	YMin, YMax float64
	// FormatY labels y values on the axis and in the readout.
	FormatY func(float64) string

	fetchers map[string]*fetcher
	scratch  map[string]*drawing
	width    float32
	lastY    chart.Linear
	hasLastY bool
	drag     struct {
		active bool
		x      float32
		lo, hi float64
	}
}

// drawing holds the buffers one series reuses every frame.
type drawing struct {
	xs, ys, lo, hi []float32
	pts            [][2]float32
}

type frame struct {
	series Series
	color  ui.Color
	points Points
}

const (
	defaultHeight = 160
	defaultWidth  = 640
	axisWidth     = 52 // DIPs, equal in every row so stacked plots line up
	wheelZoom     = .004
	keyPan        = .1
	keyZoom       = .8
)

// Build lays out the chart: call it while building the view, once per frame.
func (ch *Chart) Build(c *ui.Context) ui.Element {
	t := c.Theme()
	lo, hi := ch.Range.Visible()
	width := ch.width
	if width <= 0 {
		width = defaultWidth
	}
	frames := ch.frames(c, t, lo, hi, int(math.Ceil(float64(width))))
	y := ch.yScale(frames, lo, hi)
	y.Format = ch.FormatY
	var xticks []chart.Tick
	if ch.Numeric {
		xticks = numericTicks(lo, hi, maxTicks(width))
	} else {
		xticks = timeTicks(lo, hi, ch.location(), maxTicks(width))
	}
	yticks := y.Ticks(4)
	axes := chart.Axes{Left: yticks, Label: func(side chart.Side, tick chart.Tick) {
		label := ui.Text(c, tick.Label).NoWrap().FontSize(t.FontSize - 2).TextColor(t.TextMuted)
		if side == chart.Left {
			label.Width(axisWidth).TextAlign(ui.End)
		}
	}}
	if !ch.HideXAxis {
		axes.Bottom = xticks
	}
	height := ch.Height
	if height <= 0 {
		height = defaultHeight
	}
	return chart.Plot(c, axes, func(area ui.Element) {
		area.Label(ch.Label).Value(ch.describeRange(lo, hi)).Focusable().Clip().Cursor(ui.CursorCrosshair)
		area.HandleInput(ch.handleInput)
		if _, _, over := area.PointerPosition(); !over && !ch.drag.active && ch.Range.crossOwner == ch {
			ch.Range.ClearCrosshair()
		}
		area.Draw(func(p *ui.Painter, r ui.Rect) {
			ch.width = r.W
			for _, tick := range yticks {
				p.Line(r.X, chart.Y(r, tick.At), r.X+r.W, chart.Y(r, tick.At), 1, t.Border.Alpha(.45))
			}
			for _, tick := range xticks {
				p.Line(chart.X(r, tick.At), r.Y, chart.X(r, tick.At), r.Y+r.H, 1, t.Border.Alpha(.25))
			}
			for _, f := range frames {
				ch.drawSeries(p, r, f, y, lo, hi)
			}
		})
		ch.events(c, t, lo, hi, width)
		ch.crosshair(c, t, frames, lo, hi, width)
	}).Height(height).Gap(6).Padding(4, 16, 0, 0).FillWidth()
}

// Loaded reports whether every series has fetched the points for the
// visible window, as after a pan or zoom settles.
func (ch *Chart) Loaded() bool {
	lo, hi := ch.Range.Visible()
	for _, s := range ch.Series {
		if f := ch.fetchers[s.ID]; f == nil || !f.ready(lo, hi) {
			return false
		}
	}
	return true
}

// Close stops fetching; call it when the chart is discarded.
func (ch *Chart) Close() {
	for _, f := range ch.fetchers {
		f.close()
	}
}

func (ch *Chart) location() *time.Location {
	if ch.Location != nil {
		return ch.Location
	}
	return time.Local
}

func (ch *Chart) frames(c *ui.Context, t *ui.Theme, lo, hi float64, pixels int) []frame {
	if ch.fetchers == nil {
		ch.fetchers = map[string]*fetcher{}
		ch.scratch = map[string]*drawing{}
	}
	palette := [...]ui.Color{t.Accent, t.Success, t.Warning, t.Danger}
	frames := make([]frame, len(ch.Series))
	for i, s := range ch.Series {
		f := ch.fetchers[s.ID]
		if f == nil {
			f = &fetcher{}
			ch.fetchers[s.ID] = f
		}
		color := s.Color
		if color == (ui.Color{}) {
			color = palette[i%len(palette)]
		}
		frames[i] = frame{s, color, f.points(s.Provider, c.Invalidate, lo, hi, pixels)}
	}
	return frames
}

// yScale fits the axis to the points in view, keeping the last fit while
// none have arrived.
func (ch *Chart) yScale(frames []frame, lo, hi float64) chart.Linear {
	if ch.YMax > ch.YMin {
		return chart.NewLinear(ch.YMin, ch.YMax)
	}
	vlo, vhi := math.Inf(1), math.Inf(-1)
	for _, f := range frames {
		from, to := visible(f.points.X, lo, hi)
		for _, v := range f.points.Y[from:to] {
			vlo, vhi = min(vlo, v), max(vhi, v)
		}
	}
	if vlo > vhi {
		if !ch.hasLastY {
			return chart.NewLinear(0, 1)
		}
		return ch.lastY
	}
	if vlo == vhi {
		vlo, vhi = vlo-1, vhi+1
	}
	pad := (vhi - vlo) * .08
	ch.lastY, ch.hasLastY = chart.NewLinear(vlo-pad, vhi+pad).Nice(4), true
	return ch.lastY
}

// visible returns the index range of xs in lo to hi.
func visible(xs []float64, lo, hi float64) (from, to int) {
	from, to = sort.SearchFloat64s(xs, lo), sort.SearchFloat64s(xs, math.Nextafter(hi, math.Inf(1)))
	return from, max(from, to)
}

func (ch *Chart) describeRange(lo, hi float64) string {
	if ch.Numeric {
		return formatNumber(lo) + " to " + formatNumber(hi)
	}
	return formatTime(lo, ch.location()) + " to " + formatTime(hi, ch.location())
}

func formatNumber(v float64) string { return strconv.FormatFloat(v, 'g', 6, 64) }

func (ch *Chart) drawSeries(p *ui.Painter, r ui.Rect, f frame, y chart.Linear, lo, hi float64) {
	d := ch.scratch[f.series.ID]
	if d == nil {
		d = &drawing{}
		ch.scratch[f.series.ID] = d
	}
	// Only the points in view, and the neighbour either side so lines run
	// off the plot, are worth stroking.
	from, to := visible(f.points.X, lo, hi)
	from, to = max(0, from-1), min(len(f.points.X), to+1)
	span := hi - lo
	d.xs, d.ys = d.xs[:0], d.ys[:0]
	for i := from; i < to; i++ {
		d.xs = append(d.xs, float32((f.points.X[i]-lo)/span))
		d.ys = append(d.ys, y.At(f.points.Y[i]))
	}
	switch f.series.Kind {
	case Line:
		p.StrokePolyline(d.polyline(r, d.xs, d.ys, false), 1.5, f.color)
	case Step:
		p.StrokePolyline(d.polyline(r, d.xs, d.ys, true), 1.5, f.color)
	case Band:
		d.envelope(r.W)
		p.FillPath(chart.Area(r, d.xs, d.lo, d.hi, chart.CurveLinear), f.color.Alpha(.28))
		p.StrokePolyline(d.polyline(r, d.xs, d.hi, false), 1, f.color.Alpha(.7))
		p.StrokePolyline(d.polyline(r, d.xs, d.lo, false), 1, f.color.Alpha(.7))
	}
}

// polyline places points at fractions xs and ys of r, joined by steps that
// hold each value until the next point when step is set.
func (d *drawing) polyline(r ui.Rect, xs, ys []float32, step bool) [][2]float32 {
	d.pts = d.pts[:0]
	for i := range xs {
		if step && i > 0 {
			d.pts = append(d.pts, [2]float32{chart.X(r, xs[i]), chart.Y(r, ys[i-1])})
		}
		d.pts = append(d.pts, [2]float32{chart.X(r, xs[i]), chart.Y(r, ys[i])})
	}
	return d.pts
}

// envelope reduces d.xs and d.ys, fractions of a plot width DIPs wide, to the
// lowest and highest of each pixel column, leaving them in d.xs, d.lo and d.hi.
func (d *drawing) envelope(width float32) {
	d.lo, d.hi = d.lo[:0], d.hi[:0]
	n := 0
	column := math.MinInt
	for i, x := range d.xs {
		c := int(math.Floor(float64(x * width)))
		if c != column || n == 0 {
			column = c
			d.xs[n] = x
			d.lo = append(d.lo, d.ys[i])
			d.hi = append(d.hi, d.ys[i])
			n++
			continue
		}
		d.lo[n-1] = min(d.lo[n-1], d.ys[i])
		d.hi[n-1] = max(d.hi[n-1], d.ys[i])
	}
	d.xs = d.xs[:n]
}

// events adds shaded intervals, marker lines and flags for the events in view.
func (ch *Chart) events(c *ui.Context, t *ui.Theme, lo, hi float64, width float32) {
	span := hi - lo
	flagEnd := float32(-1) // right edge of the last flag, as a fraction of the plot
	for _, e := range ch.Events {
		if max(e.End, e.Start) < lo || e.Start > hi {
			continue
		}
		color := e.Color
		if color == (ui.Color{}) {
			color = t.Danger
		}
		start := float32((e.Start - lo) / span)
		if e.End > e.Start {
			end := float32((e.End - lo) / span)
			chart.Rect(c, chart.Span{Lo: max(0, start), Hi: min(1, end)}, chart.Span{Lo: 0, Hi: 1}).
				Background(color.Alpha(.12)).PassThrough()
		}
		if start >= 0 {
			chart.RuleX(c, start, 1).Background(color.Alpha(.7)).PassThrough()
		}
		at := max(0, start)
		flagWidth := (float32(utf8.RuneCountInString(e.Label))*(t.FontSize-2)*.6 + 12) / width
		if e.Label == "" || at < flagEnd {
			continue
		}
		flagEnd = at + flagWidth
		chart.Anchor(c, at, 1, ui.Start, ui.Start, func() {
			ui.Row(c).Padding(1, 4).Radius(2).Background(t.Background.Alpha(.9)).Border(1, color.Alpha(.7)).PassThrough().
				Children(func() {
					ui.Text(c, e.Label).NoWrap().FontSize(t.FontSize - 2).TextColor(t.Text)
				})
		}).PassThrough()
	}
}

// crosshair shows the shared hover position and what each series reads there.
func (ch *Chart) crosshair(c *ui.Context, t *ui.Theme, frames []frame, lo, hi float64, width float32) {
	x, ok := ch.Range.Crosshair()
	if !ok || x < lo || x > hi {
		return
	}
	at := float32((x - lo) / (hi - lo))
	chart.RuleX(c, at, 1).Background(t.TextMuted.Alpha(.55)).PassThrough()
	tolerance := (hi - lo) / float64(width)
	horizontal := ui.Start
	if at > .5 {
		horizontal = ui.End
	}
	chart.Anchor(c, at, 1, horizontal, ui.Start, func() {
		ui.Column(c).Margin(8).Padding(t.Space(1.5), t.Space(2)).Gap(t.Space(.5)).Radius(t.Radius).
			Background(t.Background.Alpha(.94)).Border(1, t.Border).TextColor(t.Text).FontSize(t.FontSize - 1).
			PassThrough().Children(func() {
			ui.Text(c, ch.formatX(x)).Bold().NoWrap()
			for _, f := range frames {
				vlo, vhi, ok := readAt(f.series.Kind, f.points, x, tolerance)
				if !ok {
					continue
				}
				value := ch.formatY(vlo)
				if f.series.Kind == Band {
					value += " – " + ch.formatY(vhi)
				}
				ui.Row(c).AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
					ui.Box(c).Size(8, 8).Radius(4).Background(f.color)
					ui.Text(c, f.series.Label).NoWrap().TextColor(t.TextMuted)
					ui.Box(c).Grow(1).MinWidth(t.Space(2))
					ui.Text(c, value).Bold().NoWrap()
				})
			}
		})
	}).PassThrough()
}

func (ch *Chart) formatX(x float64) string {
	if ch.Numeric {
		return formatNumber(x)
	}
	return formatTime(x, ch.location())
}

func (ch *Chart) formatY(v float64) string {
	if ch.FormatY != nil {
		return ch.FormatY(v)
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// handleInput zooms with the wheel and pinch, pans by dragging, resets on
// double click, and moves the crosshair with the pointer.
func (ch *Chart) handleInput(ev ui.InputEvent) bool {
	width := float64(ch.width)
	if width <= 0 {
		return false
	}
	r := ch.Range
	lo, hi := r.Visible()
	span := hi - lo
	at := lo + float64(ev.X)/width*span
	switch ev.Kind { //nolint:exhaustive // other input is left to the plot's ancestors
	case ui.InputScroll:
		if math.Abs(float64(ev.DX)) > math.Abs(float64(ev.DY)) {
			r.Pan(float64(ev.DX) / width * span)
		} else {
			r.ZoomAt(at, math.Exp(float64(ev.DY)*wheelZoom))
		}
	case ui.InputZoom:
		if ev.Scale <= 0 {
			return false
		}
		r.ZoomAt(at, 1/float64(ev.Scale))
	case ui.InputPointerDown:
		if ev.Button != 0 {
			return false
		}
		if ev.Clicks >= 2 {
			r.Reset()
			ch.drag.active = false
			return true
		}
		ch.drag.active, ch.drag.x, ch.drag.lo, ch.drag.hi = true, ev.X, lo, hi
	case ui.InputPointerUp:
		ch.drag.active = false
	case ui.InputPointerMove:
		if ch.drag.active {
			shift := float64(ev.X-ch.drag.x) / width * (ch.drag.hi - ch.drag.lo)
			r.Set(ch.drag.lo-shift, ch.drag.hi-shift)
			lo, hi = r.Visible()
			at = lo + float64(ev.X)/width*(hi-lo)
		}
		if ev.X >= 0 && float64(ev.X) <= width {
			r.cross, r.hasCross, r.crossOwner = at, true, ch
		}
	case ui.InputKeyDown:
		return ch.handleKey(ev.Key, ev.Mods)
	default:
		return false
	}
	return true
}

func (ch *Chart) handleKey(key ui.Key, mods ui.Modifiers) bool {
	if mods != 0 {
		return false
	}
	lo, hi := ch.Range.Visible()
	switch key { //nolint:exhaustive // other keys are left alone
	case ui.KeyLeft:
		ch.Range.Pan(-(hi - lo) * keyPan)
	case ui.KeyRight:
		ch.Range.Pan((hi - lo) * keyPan)
	case ui.KeyEqual:
		ch.Range.ZoomAt((lo+hi)/2, keyZoom)
	case ui.KeyMinus:
		ch.Range.ZoomAt((lo+hi)/2, 1/keyZoom)
	case ui.KeyHome:
		ch.Range.Reset()
	default:
		return false
	}
	return true
}
