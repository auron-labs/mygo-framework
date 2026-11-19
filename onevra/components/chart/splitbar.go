package chart

import (
	"slices"
	"strings"
	"time"

	"github.com/auron-labs/mygo-framework/v2/onevra/components"
	"github.com/auron-labs/mygo-framework/v2/onevra/internal/l10n"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

// SplitBar draws one bar divided into the shares of the visible slices, for
// part-to-whole splits of about two to eight categories. Options.Label titles
// the bar and names it; Description is a line under the title, and a readout
// beside them shows the total or the highlighted slice's value and share.
// FormatValue writes values, unit included (nil writes grouped numbers);
// shares are percentages with one decimal. PlotHeight is the bar's thickness
// (default 32 DIPs); labels that do not fit inside their segment move to a
// lane under it on leader lines. Detail replaces the readout.
//
// State.Index is the highlighted slice in input order, set by hover, arrow
// keys, Home/End and legend hover; Escape clears it. State.Hidden records
// slices the legend, Enter or Space switched off, keyed by Slice.ID; the last
// visible slice cannot be switched off. Invalid and non-positive slices are
// omitted. Segment edges follow a spring, which AnimationDuration sets (zero
// uses MyGo's smooth spring, negative disables motion), and figures roll as
// they change. The total's name and refusal message come from the onevra
// catalog.
func SplitBar(c *ui.Context, state *State, data []Slice, opts Options) ui.Element {
	e, state := frame(c, state)
	words := l10n.In(c.Locale())
	locale := c.Locale()
	if locale == "" {
		locale = "en"
	}
	s := prepareSplit(data, state.Hidden)
	if j := slices.Index(s.indices, state.Index); j < 0 || !s.visible[j] {
		state.Index = -1
	}
	st := ui.Local(e, "onevra.chart.split", func() splitMotion { return splitMotion{} })
	formatValue := func(v float64) string {
		if !finite(v) {
			return "—"
		}
		if opts.FormatValue != nil {
			return opts.FormatValue(v)
		}
		return i18n.FormatNumber(locale, v, i18n.NumberOptions{})
	}
	percent := func(share float64) string {
		return i18n.FormatNumber(locale, share, i18n.NumberOptions{Percent: true, MinFractionDigits: 1, MaxFractionDigits: 1})
	}
	describeSlice := func(i int) string {
		j := slices.Index(s.indices, i)
		return data[i].Label + " " + percent(s.shares[j]) + ", " + formatValue(data[i].Value)
	}
	// toggle switches a slice off or back on, refusing to hide the last one.
	toggle := func(i int) {
		j := slices.Index(s.indices, i)
		id := data[i].ID
		if s.visible[j] && s.count == 1 {
			st.shake = id
			c.Announce(words.Text("chartKeepOneSegment"))
			c.Invalidate()
			return
		}
		if state.Hidden == nil {
			state.Hidden = make(map[string]bool)
		}
		state.Hidden[id] = s.visible[j]
		if s.visible[j] {
			if state.Index == i {
				state.Index = -1
			}
			c.Announce(data[i].Label + " " + words.Text("chartOff"))
		} else {
			c.Announce(data[i].Label)
		}
		c.Invalidate()
	}
	e.Children(func() {
		if len(s.indices) == 0 {
			splitHeader(c, opts, func() {})
			o := opts
			o.PlotHeight = splitThickness(opts) * 2
			empty(c, o)
			return
		}
		// The header shows the highlight the previous input left; the frame is
		// rebuilt below when the bar or legend changes it.
		shown := state.Index
		splitHeader(c, opts, func() {
			if opts.Detail != nil {
				selection := Selection{Index: -1}
				if i := shown; i >= 0 {
					sl := data[i]
					selection = Selection{Index: i, Label: sl.Label, Values: []Value{{sl.ID, sl.Label, sl.Value, color(c, i, sl.Color)}}}
				}
				opts.Detail(c, selection)
				return
			}
			label, value, share, shareValue := words.Text("chartTotal"), s.total, "", 0.0
			if i := shown; i >= 0 {
				shareValue = s.shares[slices.Index(s.indices, i)]
				label, value, share = data[i].Label, data[i].Value, percent(shareValue)
			}
			splitReadout(c, st, label, value, formatValue(value), share, shareValue)
		})
		plot := ui.Box(c).FillWidth().MinWidth(0).Label(opts.Label).Description(s.summary(data, percent))
		if !plot.IsDisabled() {
			splitSelection(c, plot, state, st, s, splitThickness(opts), func(i int) {
				if i >= 0 {
					toggle(i)
				}
			}, describeSlice)
		}
		splitPlot(c, plot, st, s, data, state.Index, opts, percent)
		if !opts.HideLegend {
			splitLegend(c, st, s, data, state, percent, words.Text("chartOff"), toggle)
		}
		if state.Index != shown {
			c.Invalidate()
		}
	})
	return e
}

// split holds the valid slices in input order with their visible shares.
type split struct {
	indices []int
	visible []bool
	shares  []float64 // of the visible total; zero when hidden
	total   float64
	count   int
}

func prepareSplit(data []Slice, hidden map[string]bool) split {
	var s split
	largest := 0.0
	for i, sl := range data {
		if finite(sl.Value) && sl.Value > 0 {
			s.indices = append(s.indices, i)
			s.visible = append(s.visible, !hidden[sl.ID])
			largest = max(largest, sl.Value)
		}
	}
	// Scale by the largest value so the sum of shares stays finite.
	sum := 0.0
	s.shares = make([]float64, len(s.indices))
	for j, i := range s.indices {
		if s.visible[j] {
			s.shares[j] = data[i].Value / largest
			sum += s.shares[j]
			s.total += data[i].Value
			s.count++
		}
	}
	for j := range s.shares {
		if sum > 0 {
			s.shares[j] /= sum
		}
	}
	return s
}

func (s split) summary(data []Slice, percent func(float64) string) string {
	var parts []string
	for j, i := range s.indices {
		if s.visible[j] {
			parts = append(parts, data[i].Label+" "+percent(s.shares[j]))
		}
	}
	return strings.Join(parts, ", ")
}

// splitMotion retains what the bar needs between frames: the edges hit
// testing uses, the measured width and the rolling direction of figures.
type splitMotion struct {
	edges   []float64
	width   float32
	hover   shapeHover
	legend  string
	shake   string
	rows    map[string]int // outside label rows by slice ID
	figures map[string]rolling
}

type rolling struct {
	text  string
	value float64
	down  bool
}

// roll keeps a figure's rolling direction until its text changes again.
func (st *splitMotion) roll(key, text string, value float64) bool {
	if st.figures == nil {
		st.figures = make(map[string]rolling)
	}
	r, ok := st.figures[key]
	if !ok || r.text != text {
		st.figures[key] = rolling{text, value, ok && value < r.value}
		return ok && value < r.value
	}
	return r.down
}

func splitThickness(opts Options) float32 {
	if opts.PlotHeight > 0 {
		return opts.PlotHeight
	}
	return 32
}

func splitAnimation(opts Options) (ui.Animation, bool) {
	switch {
	case opts.AnimationDuration < 0:
		return ui.Animation{}, false
	case opts.AnimationDuration > 0:
		return ui.Animation{Spring: ui.SpringOptions{Duration: opts.AnimationDuration}}, true
	}
	return ui.Smooth, true
}

func splitHeader(c *ui.Context, opts Options, readout func()) {
	t := c.Theme()
	ui.Row(c).FillWidth().MinWidth(0).Wrap().AlignItems(ui.End).Justify(ui.SpaceBetween).Gap(t.Space(3)).Children(func() {
		ui.Column(c).MinWidth(0).Shrink(1).Gap(t.Space(.5)).Children(func() {
			if opts.Label != "" {
				components.Text(c, opts.Label).Bold()
			}
			if opts.Description != "" {
				components.Text(c, opts.Description).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
			}
		})
		readout()
	})
}

// splitReadout crossfades between the total and the highlighted slice.
func splitReadout(c *ui.Context, st *splitMotion, label string, value float64, text, share string, shareValue float64) {
	t := c.Theme()
	ui.Column(c).MinWidth(0).AlignItems(ui.End).Gap(t.Space(.5)).Children(func() {
		components.Text(c, label).FontSize(t.FontSize - 1).TextColor(t.TextMuted).NoWrap().
			ContentTransition(ui.ContentTransition{})
		ui.Row(c).AlignItems(ui.End).Gap(t.Space(1.5)).Children(func() {
			components.Text(c, text).FontSize(t.FontSize * 1.5).Bold().NoWrap().FontFeatures("tnum").
				ContentTransition(ui.ContentTransition{Numeric: true, Down: st.roll("readout", text, value)})
			if share != "" {
				components.Text(c, share).TextColor(t.TextMuted).NoWrap().FontFeatures("tnum").
					ContentTransition(ui.ContentTransition{Numeric: true, Down: st.roll("readout.share", share, shareValue)})
			}
		})
	})
}

// splitSelection highlights segments by pointer and keyboard and switches the
// highlighted one off with Enter or Space.
func splitSelection(c *ui.Context, plot ui.Element, state *State, st *splitMotion, s split, thickness float32, toggle func(int), describe func(int) string) {
	plot.Focusable()
	var order []int
	for j, i := range s.indices {
		if s.visible[j] {
			order = append(order, i)
		}
	}
	position := slices.Index(order, state.Index)
	keyboard := true
	switch {
	case plot.Shortcut(0, ui.KeyRight), plot.Shortcut(0, ui.KeyDown):
		state.Index = order[min(position+1, len(order)-1)]
	case plot.Shortcut(0, ui.KeyLeft), plot.Shortcut(0, ui.KeyUp):
		if position < 0 {
			position = len(order)
		}
		state.Index = order[max(0, position-1)]
	case plot.Shortcut(0, ui.KeyHome):
		state.Index = order[0]
	case plot.Shortcut(0, ui.KeyEnd):
		state.Index = order[len(order)-1]
	case plot.Shortcut(0, ui.KeyEscape):
		state.Index = -1
	case plot.Shortcut(0, ui.KeyEnter), plot.Shortcut(0, ui.KeySpace):
		toggle(state.Index)
		return
	default:
		keyboard = false
	}
	hover := &st.hover
	x, y, over := plot.PointerPosition()
	over = over && y <= thickness
	if keyboard || !over {
		hover.pointer = false
	} else if !hover.over || x != hover.x || y != hover.y {
		hover.pointer = true
	}
	if over && !keyboard && (!hover.over || x != hover.x || y != hover.y) {
		if w := plot.LayoutBounds().W; w > 0 && len(st.edges) == len(s.indices) {
			state.Index = -1
			start := 0.0
			at := float64(x / w)
			for j, end := range st.edges {
				if s.visible[j] && at >= start && at < end {
					state.Index = s.indices[j]
				}
				start = end
			}
		}
	} else if !over && hover.over && !keyboard {
		state.Index = -1
	}
	hover.x, hover.y, hover.over = x, y, over
	if state.Index >= 0 {
		value := describe(state.Index)
		plot.Value(value)
		if keyboard {
			c.Announce(value)
		}
	}
}

// Label modes inside a segment.
const (
	labelOutside = iota
	labelShare
	labelFull
)

type splitKey struct{ kind, id string }

// splitTargets widens the active segment by 4.5% of the bar, or enough for
// its label, while the others give way proportionally.
func splitTargets(shares []float64, active int, need float64) []float64 {
	targets := slices.Clone(shares)
	if active < 0 || shares[active] >= 1 {
		return targets
	}
	w := shares[active]
	grown := max(w+.045, min(need, .5))
	grown = min(grown, max(w, .9))
	k := (1 - grown) / (1 - w)
	for j := range targets {
		targets[j] *= k
	}
	targets[active] = grown
	return targets
}

func splitPlot(c *ui.Context, plot ui.Element, st *splitMotion, s split, data []Slice, highlighted int, opts Options, percent func(float64) string) {
	t := c.Theme()
	thickness := splitThickness(opts)
	anim, animated := splitAnimation(opts)
	animate := func(key splitKey, target float32) float32 {
		if !animated {
			return plot.Jump(key, target)
		}
		return plot.AnimateTo(key, target, anim)
	}
	width := plot.LayoutBounds().W
	if width != st.width {
		known := st.width > 0
		st.width = width
		if width > 0 && known {
			c.Invalidate() // Refit labels at the new width.
		}
	}
	size := t.FontSize - 1
	pad := t.Space(2)
	n := len(s.indices)
	names, shares := make([]string, n), make([]string, n)
	nameW, shareW := make([]float32, n), make([]float32, n)
	gapW := t.Space(1.5)
	active := slices.Index(s.indices, highlighted)
	need := 0.0
	for j, i := range s.indices {
		names[j], shares[j] = data[i].Label, percent(s.shares[j])
		nameW[j], _ = c.MeasureText(0, ui.Span{Text: names[j], Size: size, Weight: 600})
		shareW[j], _ = c.MeasureText(0, ui.Span{Text: shares[j], Size: size, Features: "tnum"})
		if j == active && width > 0 {
			need = float64((nameW[j] + gapW + shareW[j] + 2*pad + 1) / width)
		}
	}
	targets := splitTargets(s.shares, active, need)
	modes := make([]int, n)
	outside := false
	for j := range s.indices {
		w := float32(targets[j]) * width
		switch {
		case !s.visible[j] || width <= 0:
		case w >= nameW[j]+gapW+shareW[j]+2*pad:
			modes[j] = labelFull
		case w >= shareW[j]+2*pad:
			modes[j] = labelShare
		default:
			modes[j] = labelOutside
			outside = true
		}
	}
	_, lineH := c.MeasureText(0, ui.Span{Text: "0", Size: size})
	leader, spacing, rowGap := t.Space(4), t.Space(3), t.Space(1)
	// Outside labels share as few rows as hold them, alternating between rows.
	outW := make([]float32, n)
	var out []int
	total := float32(0)
	for j := range s.indices {
		outW[j], _ = c.MeasureText(0, outsideSpans(t, names[j], shares[j], size, 1)...)
		if modes[j] == labelOutside && s.visible[j] && width > 0 {
			out = append(out, j)
			total += outW[j]
		}
	}
	rows := 1
	for rows < len(out) && total+spacing*float32(len(out)-rows) > float32(rows)*width {
		rows++
	}
	if st.rows == nil {
		st.rows = make(map[string]int)
	}
	for k, j := range out {
		st.rows[data[s.indices[j]].ID] = k % rows
	}
	laneTarget := float32(0)
	if outside {
		laneTarget = leader + float32(rows)*(lineH+rowGap)
	}
	if width <= 0 || !animated {
		plot.Jump(splitKey{"lane", ""}, laneTarget)
	}
	lane := animate(splitKey{"lane", ""}, laneTarget)
	plot.Height(thickness + max(0, lane))

	edges := make([]float64, n)
	nameK, dims, fades := make([]float32, n), make([]float32, n), make([]float32, n)
	end := 0.0
	for j, i := range s.indices {
		id := data[i].ID
		end += targets[j]
		edges[j] = float64(animate(splitKey{"edge", id}, float32(end)))
		if j > 0 {
			edges[j] = max(edges[j], edges[j-1])
		}
		edges[j] = min(max(edges[j], 0), 1)
		full := float32(0)
		if modes[j] == labelFull {
			full = 1
		}
		nameK[j] = animate(splitKey{"name", id}, full)
		dim := float32(1)
		if active >= 0 && j != active {
			dim = .4
		}
		dims[j] = animate(splitKey{"dim", id}, dim)
		fade := float32(0)
		if s.visible[j] && modes[j] == labelOutside && width > 0 {
			fade = 1
		}
		fades[j] = animate(splitKey{"outside", id}, fade)
	}
	if n > 0 {
		edges[n-1] = 1
	}
	st.edges = edges
	colors := make([]ui.Color, n)
	rowOf := make([]int, n)
	for j, i := range s.indices {
		colors[j] = color(c, i, data[i].Color)
		rowOf[j] = st.rows[data[i].ID]
	}
	focused := plot.FocusVisible()
	plot.Draw(func(p *ui.Painter, r ui.Rect) {
		if r.W <= 0 {
			return
		}
		bar := ui.Rect{X: r.X, Y: r.Y, W: r.W, H: thickness}
		radius := min(thickness/2, t.Radius*1.5)
		gap := float32(2)
		p.Clip(bar, radius, func() {
			start := 0.0
			for j := range s.indices {
				x0, x1 := bar.X+float32(start)*bar.W, bar.X+float32(edges[j])*bar.W
				start = edges[j]
				if x1-x0 < .5 {
					continue
				}
				if x0 > bar.X {
					x0 += gap / 2
				}
				if x1 < bar.X+bar.W-.5 {
					x1 -= gap / 2
				}
				seg := ui.Rect{X: x0, Y: bar.Y, W: max(0, x1-x0), H: bar.H}
				p.Fill(seg, colors[j].Alpha(dims[j]), min(radius, 4))
				if modes[j] == labelOutside && nameK[j] <= 0 {
					continue
				}
				fg := contrastText(colors[j]).Alpha(dims[j])
				p.Clip(seg, 0, func() {
					x, y := seg.X+pad, seg.Y+(seg.H-lineH)/2
					if k := nameK[j]; k > 0 {
						if col, ok := faded(fg, dims[j]*k); ok {
							p.RichText(x, y, 0, ui.Span{Text: names[j], Size: size, Weight: 600, Color: col})
						}
						x += (nameW[j] + gapW) * k
					}
					if col, ok := faded(fg, dims[j]*.85); ok {
						p.RichText(x, y, 0, ui.Span{Text: shares[j], Size: size, Features: "tnum", Color: col})
					}
				})
			}
		})
		if focused {
			p.FocusRing(bar, [4]float32{radius, radius, radius, radius})
		}
		splitOutside(p, t, bar, edges, outW, rowOf, fades, dims, names, shares, size, leader, spacing, lineH+rowGap)
	})
}

// splitOutside places labels that do not fit on a lane under the bar, pushed
// apart within their rows so they never overlap, with S-shaped leader lines
// to their segments.
func splitOutside(p *ui.Painter, t *ui.Theme, bar ui.Rect, edges []float64, widths []float32, rowOf []int, fades, dims []float32, names, shares []string, size, leader, spacing, rowH float32) {
	type placed struct {
		j        int
		cx, x, w float32
	}
	rows := map[int][]placed{}
	start := 0.0
	for j := range edges {
		if fades[j] > .01 {
			cx := bar.X + float32(start+edges[j])/2*bar.W
			rows[rowOf[j]] = append(rows[rowOf[j]], placed{j, cx, cx - widths[j]/2, widths[j]})
		}
		start = edges[j]
	}
	top := bar.Y + bar.H
	for row, labels := range rows {
		for k := range labels {
			if k > 0 {
				labels[k].x = max(labels[k].x, labels[k-1].x+labels[k-1].w+spacing)
			}
			labels[k].x = max(labels[k].x, bar.X)
		}
		for k := len(labels) - 1; k >= 0; k-- {
			limit := bar.X + bar.W - labels[k].w
			if k < len(labels)-1 {
				limit = min(limit, labels[k+1].x-spacing-labels[k].w)
			}
			labels[k].x = max(min(labels[k].x, limit), bar.X)
		}
		y := top + leader + float32(row)*rowH
		for _, l := range labels {
			a := fades[l.j] * dims[l.j]
			var path ui.Path
			end, mid := l.x+l.w/2, top+(y-top)/2
			path.MoveTo(l.cx, top+2).CubeTo(l.cx, mid, end, mid, end, y-2)
			p.StrokePath(&path, 1, t.TextMuted.Alpha(.5*a))
			p.RichText(l.x, y, 0, outsideSpans(t, names[l.j], shares[l.j], size, a)...)
		}
	}
}

func outsideSpans(t *ui.Theme, name, share string, size, alpha float32) []ui.Span {
	return []ui.Span{
		{Text: name + " ", Size: size, Color: t.TextMuted.Alpha(alpha)},
		{Text: share, Size: size, Weight: 600, Features: "tnum", Color: t.Text.Alpha(alpha)},
	}
}

// faded returns c with its alpha multiplied by k, and whether it paints
// anything: a span whose alpha rounds to zero takes the theme's text color
// instead, which would flash a fading label in that color.
func faded(c ui.Color, k float32) (ui.Color, bool) {
	c = c.Alpha(k)
	return c, c.A > 0
}

// contrastText picks dark or light text for a segment color.
func contrastText(c ui.Color) ui.Color {
	lum := (.2126*float64(c.R) + .7152*float64(c.G) + .0722*float64(c.B)) / 255
	if lum > .6 {
		return ui.Hex("#141414")
	}
	return ui.Hex("#ffffff")
}

func splitLegend(c *ui.Context, st *splitMotion, s split, data []Slice, state *State, percent func(float64) string, off string, toggle func(int)) {
	t := c.Theme()
	hovered := ""
	ui.Row(c).FillWidth().MinWidth(0).Wrap().Gap(t.Space(1)).Children(func() {
		for j, i := range s.indices {
			sl := data[i]
			visible := s.visible[j]
			share := percent(s.shares[j])
			label := sl.Label + " " + share
			if !visible {
				label = sl.Label + " " + off
			}
			item := ui.Box(c).Key(sl.ID).MaxWidthPercent(100)
			key := splitKey{"shake", sl.ID}
			if st.shake == sl.ID {
				item.Jump(key, t.Space(1.5))
				st.shake = ""
			}
			item.Translate(item.AnimateTo(key, 0, ui.Animation{Spring: ui.SpringOptions{Duration: 450 * time.Millisecond, Bounce: .7}}), 0)
			item.Children(func() {
				b := legendButton(c, label, visible)
				b.Children(func() {
					if visible {
						swatch(c, color(c, i, sl.Color))
					} else {
						ui.Box(c).Size(8, 8).Shrink(0).Radius(3).Border(1, t.TextMuted)
					}
					name := components.Text(c, sl.Label).FontSize(t.FontSize - 1)
					if !visible {
						name.Strikethrough()
					}
					figure := share
					if !visible {
						figure = off
					}
					components.Text(c, figure).FontSize(t.FontSize - 1).TextColor(t.TextMuted).NoWrap().FontFeatures("tnum").
						ContentTransition(ui.ContentTransition{Numeric: visible, Down: st.roll("legend."+sl.ID, figure, s.shares[j])})
				})
				if b.Hovered() && visible {
					hovered = sl.ID
					if st.legend != sl.ID {
						state.Index = i
					}
				}
				if b.Clicked() {
					toggle(i)
				}
			})
		}
	})
	if hovered == "" && st.legend != "" {
		if j := slices.IndexFunc(s.indices, func(i int) bool { return data[i].ID == st.legend }); j >= 0 && state.Index == s.indices[j] {
			state.Index = -1
		}
	}
	st.legend = hovered
}
