package chart

import (
	"math"
	"slices"
	"time"

	"github.com/egoist/mygo/ui"
)

const (
	// bubbleGap separates packed bubbles, in radii of the largest bubble.
	bubbleGap = .06
	// bubbleMargin keeps the outermost bubbles and their rings inside the plot.
	bubbleMargin = 4
	// bubbleLift enlarges the selected bubble, within the gap around it.
	bubbleLift = 1.05
	// bubbleBalance weighs extending the pack evenly around the largest
	// bubble against its total extent, keeping that bubble near the middle.
	bubbleBalance = .2
)

// Bubble packs one circle per slice, its area proportional to the value, so
// larger values take larger bubbles. The pack keeps to the smallest extent
// the plot's proportions allow, centered, so the bubbles are as large as
// possible. Bubbles large enough show their label and formatted value.
// Bubbles show packed at once, then spring to new sizes and places and pop
// in as they appear. Hover, arrow keys, Home/End and legend buttons select
// a bubble; Escape clears it.
// State.Hidden is unused. Invalid and non-positive slices are omitted while
// State.Index refers to input order.
func Bubble(c *ui.Context, state *State, data []Slice, opts Options) ui.Element {
	e, state := frame(c, state)
	t := c.Theme()
	indices, values, _ := positive(data)
	if !slices.Contains(indices, state.Index) {
		state.Index = -1
	}
	// Pack for the plot measured in the previous frame; Draw repacks at once
	// when the plot's proportions changed since. Bubbles wait for the first
	// measurement, so they first show packed for the plot's real proportions.
	width, height := e.LayoutBounds().W, plotHeight(opts)
	measured := width > 0
	if !measured {
		width = 240
		c.Invalidate()
	}
	aspect := bubbleAspect(width, height)
	e.Children(func() {
		motion := ui.Local(e, "onevra.chart.bubbles", func() bubbleMotion { return bubbleMotion{} })
		ids := make([]string, len(indices))
		for j, i := range indices {
			ids[j] = data[i].ID
		}
		circles := motion.layout(ids, values, aspect)
		if measured {
			circles = motion.spring(e, data, indices, circles, aspect, opts)
		} else {
			circles = make([]bubbleCircle, len(circles))
		}
		if len(indices) == 0 {
			empty(c, opts)
		} else {
			area := ui.Box(c).FillWidth().MinWidth(0).Height(height).Label(opts.Label).Description(opts.Description)
			if !area.IsDisabled() {
				shape := make([]float64, 0, 3*len(circles))
				for _, b := range circles {
					shape = append(shape, b.x, b.y, b.r)
				}
				shapeSelection(c, area, state, indices, shape, func(x, y float32) int {
					r := area.LayoutBounds()
					for j, b := range viewBubbles(circles, ui.Rect{W: r.W, H: r.H}, aspect) {
						if dx, dy := x-b.x, y-b.y; dx*dx+dy*dy <= b.r*b.r {
							return j
						}
					}
					return -1
				}, func(i int) string { return describe(opts, sliceSelection(c, data, i)) })
			}
			selected := state.Index
			lift := make([]float32, len(indices))
			for j, i := range indices {
				target := float32(1)
				if i == selected {
					target = bubbleLift
				}
				lift[j] = e.Spring(bubbleKey{data[i].ID, bubbleLifted}, target, ui.SpringOptions{Duration: 300 * time.Millisecond, Bounce: .4})
			}
			area.Draw(func(p *ui.Painter, r ui.Rect) {
				shown, a := circles, bubbleAspect(r.W, r.H)
				if a != aspect {
					shown = pack(values, a)
				}
				var path ui.Path
				for j, b := range viewBubbles(shown, r, a) {
					i := indices[j]
					b.r *= lift[j]
					if b.r <= 0 {
						continue
					}
					shade := color(c, i, data[i].Color)
					fill, ring, width := shade.Alpha(.26), shade.Alpha(.55), float32(1)
					if i == selected {
						fill, ring, width = shade.Alpha(.42), shade, 2
					}
					p.FillPathGradient(path.Reset().Circle(b.x, b.y, b.r), ui.LinearGradient{From: fill, To: fill.Alpha(.45), Angle: 180})
					p.StrokePath(path.Reset().Circle(b.x, b.y, max(0, b.r-width/2)), width, ring)
					bubbleText(p, t, b, data[i].Label, format(opts, data[i].Value))
				}
			})
			area.Children(func() {
				tip(c, area, opts, sliceSelection(c, data, selected), func() ui.Element {
					r := area.LayoutBounds()
					b := viewBubbles(circles, ui.Rect{W: r.W, H: r.H}, aspect)[slices.Index(indices, selected)]
					return ui.Box(c).Absolute().Left(b.x-b.r).Top(b.y-b.r).Size(2*b.r, 2*b.r)
				})
			})
		}
		sliceLegend(c, state, data, indices, opts)
	})
	return e
}

// bubbleText centers the label and value inside a bubble when they fit,
// or the label alone, or nothing.
func bubbleText(p *ui.Painter, t *ui.Theme, b bubblePoint, label, value string) {
	inner := b.r * 1.6
	name := ui.Span{Text: label, Size: t.FontSize - 1, Color: t.Text}
	amount := ui.Span{Text: value, Size: t.FontSize, Weight: 600, Color: t.Text}
	nw, nh := p.MeasureText(0, name)
	vw, vh := p.MeasureText(0, amount)
	switch {
	case label != "" && max(nw, vw) <= inner && nh+vh <= inner:
		top := b.y - (nh+vh)/2
		p.RichText(b.x-nw/2, top, 0, name)
		p.RichText(b.x-vw/2, top+nh, 0, amount)
	case vw <= inner && vh <= inner:
		p.RichText(b.x-vw/2, b.y-vh/2, 0, amount)
	}
}

// bubbleAspect rounds the plot's width-to-height ratio, so resizing repacks
// only when its proportions change noticeably.
func bubbleAspect(w, h float32) float64 {
	if w <= 0 || h <= 0 {
		return 1
	}
	return math.Round(min(4, max(.5, float64(w/h)))*4) / 4
}

type bubbleCircle struct{ x, y, r float64 }

type bubbleKey struct {
	id   string
	part uint8
}

const (
	bubbleX uint8 = iota
	bubbleY
	bubbleR
	bubbleLifted
)

// bubbleMotion caches the pack and remembers the bubbles shown last frame.
type bubbleMotion struct {
	ids     []string
	values  []float64
	aspect  float64
	circles []bubbleCircle
	shown   map[string]bool
}

// layout reuses the previous pack while values and proportions stay. A new
// pack is mirrored to whichever side keeps bubbles nearest their previous
// places, so fewer of them cross on the way.
func (m *bubbleMotion) layout(ids []string, values []float64, aspect float64) []bubbleCircle {
	if m.aspect == aspect && slices.Equal(m.values, values) && slices.Equal(m.ids, ids) {
		return m.circles
	}
	previous := make(map[string]bubbleCircle, len(m.ids))
	for j, id := range m.ids {
		previous[id] = m.circles[j]
	}
	circles := pack(values, aspect)
	best, flip := math.Inf(1), [2]float64{1, 1}
	for _, f := range [...][2]float64{{1, 1}, {-1, 1}, {1, -1}, {-1, -1}} {
		cost := 0.0
		for j, id := range ids {
			if p, ok := previous[id]; ok {
				b := circles[j]
				cost += b.r * b.r * ((f[0]*b.x-p.x)*(f[0]*b.x-p.x) + (f[1]*b.y-p.y)*(f[1]*b.y-p.y))
			}
		}
		if cost < best {
			best, flip = cost, f
		}
	}
	for j := range circles {
		circles[j].x, circles[j].y = flip[0]*circles[j].x, flip[1]*circles[j].y
	}
	m.ids, m.values, m.circles = slices.Clone(ids), slices.Clone(values), circles
	return circles
}

// spring moves each bubble toward its place in the pack on MyGo springs,
// keyed by slice ID so a bubble keeps its velocity when targets change.
// The first pack and new proportions take effect at once; bubbles added
// later grow from their place.
func (m *bubbleMotion) spring(e ui.Element, data []Slice, indices []int, target []bubbleCircle, aspect float64, opts Options) []bubbleCircle {
	jump := opts.AnimationDuration < 0 || m.shown == nil || (m.aspect != 0 && m.aspect != aspect)
	spring := ui.SpringOptions{Duration: 550 * time.Millisecond, Bounce: .3}
	if opts.AnimationDuration > 0 {
		spring.Duration = opts.AnimationDuration
	}
	shown := make(map[string]bool, len(indices))
	circles := make([]bubbleCircle, len(target))
	for j, i := range indices {
		id, b := data[i].ID, target[j]
		shown[id] = true
		values := [3]float64{b.x, b.y, b.r}
		for part, v := range values {
			key := bubbleKey{id, uint8(part)}
			switch {
			case jump:
				e.Jump(key, float32(v))
			case !m.shown[id] && uint8(part) == bubbleR:
				e.Jump(key, 0) // A new bubble grows from nothing in its place.
			case !m.shown[id]:
				e.Jump(key, float32(v))
			}
			values[part] = float64(e.Spring(key, float32(v), spring))
		}
		circles[j] = bubbleCircle{values[0], values[1], values[2]}
	}
	m.aspect, m.shown = aspect, shown
	return circles
}

// pack places circles whose areas follow values and returns them in input
// order, centered in a view aspect wide and 1 high. Largest first, each
// takes the free spot touching placed circles that least extends the pack
// within those proportions, partly measured around the largest circle, and
// of those, the one nearest the middle. Each placement checks every pair of
// placed circles, for bounded data.
func pack(values []float64, aspect float64) []bubbleCircle {
	circles := make([]bubbleCircle, len(values))
	order := make([]int, len(values))
	for i, v := range values {
		circles[i].r, order[i] = math.Sqrt(v), i
	}
	if len(values) == 0 {
		return circles
	}
	slices.SortStableFunc(order, func(a, b int) int {
		switch {
		case circles[a].r > circles[b].r:
			return -1
		case circles[a].r < circles[b].r:
			return 1
		}
		return 0
	})
	placed := make([]bubbleCircle, 0, len(values))
	free := func(x, y, r float64) bool {
		for _, o := range placed {
			if d := o.r + r + bubbleGap; (x-o.x)*(x-o.x)+(y-o.y)*(y-o.y) < d*d*(1-1e-9) {
				return false
			}
		}
		return true
	}
	first := circles[order[0]].r
	x0, y0, x1, y1 := -first, -first, first, first
	for n, i := range order {
		r := circles[i].r
		best, score := bubbleCircle{r: r}, math.Inf(1)
		mx, my := (x0+x1)/2, (y0+y1)/2
		try := func(x, y float64) {
			extent := max((max(x1, x+r)-min(x0, x-r))/aspect, max(y1, y+r)-min(y0, y-r))
			balanced := 2 * max(max(x1, -x0, math.Abs(x)+r)/aspect, max(y1, -y0, math.Abs(y)+r))
			dx, dy := (x-mx)/aspect, y-my
			if s := (1-bubbleBalance)*extent + bubbleBalance*balanced + 1e-3*math.Sqrt(dx*dx+dy*dy); s < score && free(x, y, r) {
				best, score = bubbleCircle{x, y, r}, s
			}
		}
		if n == 0 {
			try(0, 0)
		}
		for a, o := range placed {
			d := o.r + r + bubbleGap
			try(o.x+d, o.y)
			try(o.x-d, o.y)
			try(o.x, o.y+d)
			try(o.x, o.y-d)
			for _, q := range placed[a+1:] {
				if x, y, ok := tangent(o, q, r); ok {
					try(x[0], y[0])
					try(x[1], y[1])
				}
			}
		}
		if math.IsInf(score, 1) {
			// No spot touching a circle is free; go beyond the pack.
			best.x = x1 + r + bubbleGap
		}
		circles[i] = best
		placed = append(placed, best)
		x0, y0 = min(x0, best.x-r), min(y0, best.y-r)
		x1, y1 = max(x1, best.x+r), max(y1, best.y+r)
	}
	scale := 1 / max((x1-x0)/aspect, y1-y0)
	for i, b := range circles {
		circles[i] = bubbleCircle{(b.x - (x0+x1)/2) * scale, (b.y - (y0+y1)/2) * scale, b.r * scale}
	}
	return circles
}

// tangent returns the two centers where a circle of radius r touches both a
// and b, keeping bubbleGap from each.
func tangent(a, b bubbleCircle, r float64) (x, y [2]float64, ok bool) {
	ra, rb := a.r+r+bubbleGap, b.r+r+bubbleGap
	dx, dy := b.x-a.x, b.y-a.y
	d := math.Hypot(dx, dy)
	if d == 0 || d > ra+rb || d < math.Abs(ra-rb) {
		return x, y, false
	}
	l := (ra*ra - rb*rb + d*d) / (2 * d)
	h := math.Sqrt(max(0, ra*ra-l*l))
	mx, my := a.x+l*dx/d, a.y+l*dy/d
	return [2]float64{mx - h*dy/d, mx + h*dy/d}, [2]float64{my + h*dx/d, my - h*dx/d}, true
}

type bubblePoint struct{ x, y, r float32 }

// viewBubbles maps circles packed for a view of the given aspect into r,
// centered, so moving bubbles never rescale the others.
func viewBubbles(circles []bubbleCircle, r ui.Rect, aspect float64) []bubblePoint {
	w, h := max(0, float64(r.W)-2*bubbleMargin), max(0, float64(r.H)-2*bubbleMargin)
	scale := min(w/aspect, h)
	cx, cy := float64(r.X+r.W/2), float64(r.Y+r.H/2)
	points := make([]bubblePoint, len(circles))
	for i, b := range circles {
		points[i] = bubblePoint{float32(cx + b.x*scale), float32(cy + b.y*scale), float32(max(0, b.r) * scale)}
	}
	return points
}
