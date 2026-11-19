package chart

import (
	"math"

	"github.com/auron-labs/mygo-framework/onevra/components"
	"github.com/egoist/mygo/ui"
	base "github.com/egoist/mygo/ui/chart"
)

type kind uint8

const (
	bars kind = iota
	lines
	areas
	stacks
	donuts
)

type prepared struct {
	series []int
	values [][]float64
	stack  [][]base.Segment
	edges  []base.Segment
	y      base.Linear
	valid  bool
}

func prepare(data Data, state *State, kind kind) prepared {
	p := prepared{}
	lo, hi := 0.0, 0.0
	for j, s := range data.Series {
		if state.Hidden[s.ID] {
			continue
		}
		values := make([]float64, len(data.Labels))
		for i := range values {
			v, ok := valueAt(s, i)
			if !ok {
				values[i] = math.NaN()
				continue
			}
			values[i], p.valid = v, true
			lo, hi = min(lo, v), max(hi, v)
		}
		p.series, p.values = append(p.series, j), append(p.values, values)
	}
	if kind == stacks {
		p.restack()
		for _, edge := range p.edges {
			lo, hi = min(lo, edge.Lo), max(hi, edge.Hi)
		}
	}
	if lo == hi {
		hi = lo + 1
	}
	p.y = base.NewLinear(lo, hi).Nice(4)
	// Extremely large finite domains can overflow the upstream tick arithmetic.
	if !finite(p.y.Lo) || !finite(p.y.Hi) || !finite(p.y.Hi-p.y.Lo) {
		p.valid = false
	}
	return p
}

func (p *prepared) restack() {
	p.stack = base.Stack(p.values, base.StackStandard)
	if len(p.values) == 0 {
		p.edges = nil
		return
	}
	p.edges = make([]base.Segment, len(p.values[0]))
	for _, series := range p.stack {
		for i, segment := range series {
			if !finite(segment.Lo) || !finite(segment.Hi) {
				p.valid = false // Overflow cannot produce a meaningful scale.
				return
			}
			p.edges[i].Lo = min(p.edges[i].Lo, segment.Lo)
			p.edges[i].Hi = max(p.edges[i].Hi, segment.Hi)
		}
	}
}

func cartesian(c *ui.Context, state *State, data Data, opts Options, kind kind) ui.Element {
	e, state := frame(c, state)
	e.Children(func() {
		p := animatePrepared(c, e, data, opts, kind, prepare(data, state, kind))
		if state.Index < -1 || state.Index >= len(data.Labels) || !p.valid {
			state.Index = -1
		}
		selection := func(i int) Selection {
			s := Selection{Index: i}
			if i < 0 || i >= len(data.Labels) {
				return s
			}
			s.Label = data.Labels[i]
			for _, j := range p.series {
				series := data.Series[j]
				if v, ok := valueAt(series, i); ok {
					s.Values = append(s.Values, Value{series.ID, series.Label, v, color(c, j, series.Color)})
				}
			}
			return s
		}
		if p.valid {
			plot(c, state, data, opts, kind, p, selection)
		} else {
			empty(c, opts)
		}
		if !opts.HideLegend {
			seriesLegend(c, state, data)
		}
	})
	return e
}

func plot(c *ui.Context, state *State, data Data, opts Options, kind kind, p prepared, selection func(int) Selection) {
	t := c.Theme()
	x := base.NewBand(data.Labels...).Padding(.3, .3)
	xs := make([]float32, len(data.Labels))
	for i := range xs {
		xs[i] = x.At(i)
	}
	p.y.Format = opts.FormatValue
	// Measure the previous plot for density; the first frame uses a conservative
	// default. Every category remains selectable when tick labels are thinned.
	holder := ui.Column(c).FillWidth().MinWidth(0).Height(plotHeight(opts)).Padding(8, 8, 0, 0)
	width := holder.LayoutBounds().W
	if width <= 0 {
		width = 240
	}
	axisSize := t.FontSize - 2
	tickWidth := t.Space(11)
	for _, label := range data.Labels {
		w, _ := c.MeasureText(0, ui.Span{Text: label, Size: axisSize})
		tickWidth = max(tickWidth, min(t.Space(21), w+t.Space(3)))
	}
	limit := max(1, int((width-48)/tickWidth))
	step := max(1, int(math.Ceil(float64(len(xs))/float64(limit))))
	var ticks []base.Tick
	for i := 0; i < len(xs); i += step {
		ticks = append(ticks, base.Tick{At: xs[i], Label: data.Labels[i]})
	}
	axes := base.Axes{Left: p.y.Ticks(4), Bottom: ticks, Label: func(side base.Side, tick base.Tick) {
		label := components.Text(c, tick.Label).NoWrap().FontSize(axisSize).TextColor(t.TextMuted)
		if side == base.Bottom {
			label.MaxWidth(max(24, tickWidth-8)).Ellipsis("…")
		}
	}}
	holder.Children(func() {
		base.Plot(c, axes, func(area ui.Element) {
			area.Label(opts.Label).Description(opts.Description)
			if !area.IsDisabled() {
				// MyGo 0.4.3 divides local pointer X by transformed Bounds.W.
				// Compensate in selection positions until it uses LayoutBounds.
				positions := xs
				if width, raw := area.Bounds().W, area.LayoutBounds().W; width > 0 && raw > 0 && width != raw {
					positions = make([]float32, len(xs))
					for i, x := range xs {
						positions[i] = x * raw / width
					}
				}
				base.Select(c, area, &state.Index, positions, func(i int) string { return describe(opts, selection(i)) })
			}
			selected := state.Index
			area.Draw(func(painter *ui.Painter, r ui.Rect) {
				for _, tick := range axes.Left {
					shade := t.Border.Alpha(.45)
					if tick.Value == 0 {
						shade = t.Border.Alpha(.8)
					}
					painter.Line(r.X, base.Y(r, tick.At), r.X+r.W, base.Y(r, tick.At), 1, shade)
				}
				if selected >= 0 {
					w := max(12, r.W/float32(len(xs))*.8)
					painter.Fill(ui.Rect{X: base.X(r, xs[selected]) - w/2, Y: r.Y, W: w, H: r.H}, t.Text.Alpha(.025), 6)
				}
			})
			for k, j := range p.series {
				series := data.Series[j]
				shade := color(c, j, series.Color)
				ui.Box(c).Key(series.ID).Absolute().Left(0).Right(0).Top(0).Bottom(0).Children(func() {
					if kind == lines || kind == areas {
						drawLine(c, xs, p.values[k], p.y, shade, kind, selected)
						return
					}
					for i, v := range p.values[k] {
						if !finite(v) {
							continue
						}
						band, span := x.Band(i), p.y.Span(0, v)
						top, bottom := float32(3), float32(3)
						if kind == stacks {
							segment := p.stack[k][i]
							span = p.y.Span(segment.Lo, segment.Hi)
							// Round only the silhouette, including negative stacks;
							// adjacent segments must meet along a straight edge.
							if segment.Hi != p.edges[i].Hi {
								top = 0
							}
							if segment.Lo != p.edges[i].Lo {
								bottom = 0
							}
						} else {
							w := (band.Hi - band.Lo) / float32(len(p.series))
							band = base.Span{Lo: band.Lo + float32(k)*w, Hi: band.Lo + float32(k+1)*w - w*.12}
						}
						base.Rect(c, band, span).Radius(top, top, bottom, bottom).LinearGradient(ui.LinearGradient{
							From: shade, To: shade.Mix(t.Background, .18), Angle: 180,
						})
					}
				})
			}
			if selected >= 0 {
				base.RuleX(c, xs[selected], 1).Background(t.TextMuted.Alpha(.3))
			}
			tip(c, area, opts, selection(selected), func() ui.Element { return base.RuleX(c, xs[selected], 0) })
		}).Fill().MinWidth(0).Gap(10)
	})
}

func drawLine(c *ui.Context, xs []float32, values []float64, y base.Linear, shade ui.Color, kind kind, selected int) {
	ys, baseline := make([]float32, len(values)), make([]float32, len(values))
	for i, v := range values {
		ys[i], baseline[i] = y.At(v), y.At(0)
	}
	ui.Box(c).Absolute().Left(0).Right(0).Top(0).Bottom(0).Draw(func(p *ui.Painter, r ui.Rect) {
		if kind == areas {
			p.FillPathGradient(base.Area(r, xs, baseline, ys, base.CurveMonotone), ui.LinearGradient{
				From: shade.Alpha(.24), To: shade.Alpha(.025), Angle: 180,
			})
		}
		p.StrokePath(base.Line(r, xs, ys, base.CurveMonotone), 2, shade)
	})
	for i, v := range values {
		if !finite(v) || (len(xs) > 32 && i != selected) {
			continue
		}
		if i == selected {
			base.Point(c, xs[i], ys[i], 16, 16).Radius(8).Background(shade.Alpha(.16))
		}
		base.Point(c, xs[i], ys[i], 6, 6).Radius(3).Background(shade)
	}
}
