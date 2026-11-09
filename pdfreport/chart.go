package pdfreport

import (
	"errors"
	"fmt"
	"math"
	"strconv"
)

// Point is one sample of a series.
type Point struct{ X, Y float64 }

// Series is one line of a chart.
type Series struct {
	Name   string
	Points []Point
}

// LineChart describes a vector line chart with axes, grid lines and a legend.
type LineChart struct {
	Title  string
	Series []Series
	// Height is the plot's height in points; zero means 180.
	Height float64
	// XLabel formats x-axis ticks; nil prints the number.
	XLabel func(float64) string
	// YLabel formats y-axis ticks; nil prints the number.
	YLabel func(float64) string
}

var seriesColors = [][3]uint8{{37, 99, 235}, {220, 38, 38}, {22, 163, 74}, {217, 119, 6}, {124, 58, 237}}

const (
	axisGutter   = 44.0
	tickCount    = 5
	legendSwatch = 14.0
)

// LineChart draws the chart at the current position.
func (r *Report) LineChart(chart LineChart) {
	height := chart.Height
	if height == 0 {
		height = 180
	}
	xmin, xmax, ymin, ymax, err := bounds(chart.Series)
	if err != nil {
		r.check(err)
		return
	}
	formatX := formatter(chart.XLabel)
	formatY := formatter(chart.YLabel)
	xticks := niceTicks(xmin, xmax, tickCount)
	yticks := niceTicks(ymin, ymax, tickCount)
	xmin, xmax = xticks[0], xticks[len(xticks)-1]
	ymin, ymax = yticks[0], yticks[len(yticks)-1]

	// Title, plot, axis labels and legend stay together on one page.
	r.ensure(height + 70)
	if chart.Title != "" {
		r.Heading(chart.Title, 12)
	}
	left, top := margin+axisGutter, r.pdf.GetY()+4
	width := r.contentWidth() - axisGutter
	px := func(x float64) float64 { return left + (x-xmin)/(xmax-xmin)*width }
	py := func(y float64) float64 { return top + height - (y-ymin)/(ymax-ymin)*height }

	r.setFont(regular, 8)
	r.pdf.SetTextColor(90, 90, 90)
	r.pdf.SetLineWidth(0.4)
	for _, y := range yticks {
		r.pdf.SetStrokeColor(225, 225, 225)
		r.pdf.Line(left, py(y), left+width, py(y))
		r.textRight(formatY(y), left-6, py(y)+3)
	}
	for _, x := range xticks {
		r.pdf.SetStrokeColor(235, 235, 235)
		r.pdf.Line(px(x), top, px(x), top+height)
		r.textCentered(formatX(x), px(x), top+height+14)
	}
	r.pdf.SetStrokeColor(120, 120, 120)
	r.pdf.SetLineWidth(0.8)
	r.pdf.Line(left, top, left, top+height)
	r.pdf.Line(left, top+height, left+width, top+height)

	r.pdf.SetLineWidth(1.5)
	for i, s := range chart.Series {
		color := seriesColors[i%len(seriesColors)]
		r.pdf.SetStrokeColor(color[0], color[1], color[2])
		for j := 1; j < len(s.Points); j++ {
			a, b := s.Points[j-1], s.Points[j]
			r.pdf.Line(px(a.X), py(a.Y), px(b.X), py(b.Y))
		}
	}

	legendY := top + height + 30
	x := left
	r.setFont(regular, 9)
	for i, s := range chart.Series {
		color := seriesColors[i%len(seriesColors)]
		r.pdf.SetStrokeColor(color[0], color[1], color[2])
		r.pdf.SetLineWidth(2)
		r.pdf.Line(x, legendY-3, x+legendSwatch, legendY-3)
		r.pdf.SetTextColor(40, 40, 40)
		r.pdf.SetXY(x+legendSwatch+4, legendY)
		r.check(r.pdf.Text(s.Name))
		w, err := r.pdf.MeasureTextWidth(s.Name)
		r.check(err)
		x += legendSwatch + 4 + w + 16
	}
	r.pdf.SetXY(margin, legendY+lineHeight)
}

func (r *Report) textRight(text string, right, y float64) {
	w, err := r.pdf.MeasureTextWidth(text)
	r.check(err)
	r.pdf.SetXY(right-w, y)
	r.check(r.pdf.Text(text))
}

func (r *Report) textCentered(text string, center, y float64) {
	w, err := r.pdf.MeasureTextWidth(text)
	r.check(err)
	x := min(max(center-w/2, margin), r.contentRight()-w)
	r.pdf.SetXY(x, y)
	r.check(r.pdf.Text(text))
}

func bounds(series []Series) (xmin, xmax, ymin, ymax float64, err error) {
	xmin, ymin = math.Inf(1), math.Inf(1)
	xmax, ymax = math.Inf(-1), math.Inf(-1)
	for _, s := range series {
		for _, p := range s.Points {
			if math.IsNaN(p.X) || math.IsNaN(p.Y) || math.IsInf(p.X, 0) || math.IsInf(p.Y, 0) {
				return 0, 0, 0, 0, fmt.Errorf("series %q has a point that is not a finite number", s.Name)
			}
			xmin, xmax = math.Min(xmin, p.X), math.Max(xmax, p.X)
			ymin, ymax = math.Min(ymin, p.Y), math.Max(ymax, p.Y)
		}
	}
	if math.IsInf(xmin, 1) {
		return 0, 0, 0, 0, errors.New("line chart needs at least one point")
	}
	return xmin, xmax, ymin, ymax, nil
}

func formatter(f func(float64) string) func(float64) string {
	if f != nil {
		return f
	}
	return func(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
}

// niceTicks returns about count evenly spaced round values covering [lo, hi].
// A flat range widens so the chart still has an extent.
func niceTicks(lo, hi float64, count int) []float64 {
	if lo == hi {
		lo, hi = lo-1, hi+1
	}
	step := niceStep((hi - lo) / float64(count-1))
	first := math.Floor(lo/step) * step
	last := math.Ceil(hi/step) * step
	var ticks []float64
	for v := first; v <= last+step/2; v += step {
		// Rounding keeps accumulated float error out of the labels.
		ticks = append(ticks, math.Round(v/step)*step)
	}
	return ticks
}

func niceStep(raw float64) float64 {
	magnitude := math.Pow(10, math.Floor(math.Log10(raw)))
	switch fraction := raw / magnitude; {
	case fraction <= 1:
		return magnitude
	case fraction <= 2:
		return 2 * magnitude
	case fraction <= 5:
		return 5 * magnitude
	default:
		return 10 * magnitude
	}
}
