package graph

import (
	"math"
	"time"

	"github.com/auron-labs/mygo-framework/onevra/components/gesture"
	"github.com/egoist/mygo/ui"
)

const (
	minZoom              = 1.0 / 128
	maxZoom              = 8
	labelFadeStart       = .35
	labelFadeEnd         = .65
	labelMaxWidth        = 132
	labelSize            = 11
	labelGap             = 5
	fitPadding           = 72
	cameraResponse       = 72 * time.Millisecond
	hoverTransition      = 180 * time.Millisecond
	revealDuration       = 260 * time.Millisecond
	tickInterval         = time.Second / 60
	maxTicksPerFrame     = 2
	initialBudget        = 12 * time.Millisecond
	maxFrameStep         = 48 * time.Millisecond
	positionEpsilon      = .08
	zoomEpsilon          = .00005
	mutedNodeOpacity     = .08
	mutedEdgeOpacity     = .04
	mutedLabelOpacity    = .1
	hoverEdgePeakOpacity = .92
	hitRadiusMin         = 5
	clickTolerance       = 4
	shrinkZoom           = .4
	minScreenRadius      = .75
)

type camera struct{ x, y, zoom float64 }

type pointerGesture uint8

const (
	gestureNone pointerGesture = iota
	gesturePan
	gestureNodePending
	gestureNode
)

type graphView struct {
	initialized bool
	nodes       []renderNode
	edges       []renderEdge
	adjacency   [][]int
	muted       ui.Color
	ids         []string
	sim         *simulation
	layoutKey   uint64
	lookKey     uint64

	reduceMotion bool
	hoverDepth   int

	ready, autoFit, userInteracted bool
	ticked                         int
	tickDebt                       float64
	camera, target                 camera
	width, height                  float64
	lastFrame, lastAdvance         time.Time
	animating                      bool

	hovered, previousHovered        int
	hoverDepths, previousHoverDepth map[int]int
	hoverProgress                   float64

	revealing   bool
	revealStart time.Time
	revealed    int

	gesture            pointerGesture
	dragIndex          int
	offsetX, offsetY   float64
	startX, startY     float64
	lastX, lastY       float64
	moved, lastDragged bool
	pressClicks        int
	navigation         gesture.Navigation
}

func (g *graphView) init() {
	if g.initialized {
		return
	}
	g.initialized = true
	g.hovered, g.previousHovered = -1, -1
	g.camera, g.target = camera{zoom: 1}, camera{zoom: 1}
	g.hoverProgress = 1
}

func (g *graphView) draw(p *ui.Painter, r ui.Rect) {
	now := p.Now()
	if now != g.lastAdvance {
		g.lastAdvance = now
		g.animating = g.advance(now, float64(r.W), float64(r.H))
	}
	g.paint(p, r)
	if g.animating {
		p.AnimationFrame()
	}
}

// advance moves the layout, camera, hover fade and reveal to now and
// reports whether any of them is still moving. Painting the same frame
// again does not advance it twice.
func (g *graphView) advance(now time.Time, width, height float64) bool {
	elapsed := 16 * time.Millisecond
	if !g.lastFrame.IsZero() {
		elapsed = min(max(0, now.Sub(g.lastFrame)), maxFrameStep)
	}
	g.lastFrame = now
	g.resize(width, height)
	if g.sim == nil {
		return false
	}
	if !g.ready {
		// Settle the first ticks before showing anything, a frame's budget
		// at a time, so a large graph does not stall the window.
		want := initialTicks
		if g.reduceMotion {
			want = settleTicks
		}
		deadline := time.Now().Add(initialBudget)
		for g.ticked < want && g.sim.running {
			g.sim.tick()
			g.ticked++
			if time.Now().After(deadline) {
				break
			}
		}
		if g.ticked < want && g.sim.running {
			return true
		}
		if g.reduceMotion {
			g.sim.running = false
		}
		g.ready = true
		g.fit(true)
		g.autoFit = g.sim.running
		g.tickDebt = 0
		if !g.reduceMotion {
			g.revealing, g.revealStart, g.revealed = true, now, 0
		} else {
			g.revealed = len(g.nodes)
		}
	}

	if g.reduceMotion && g.sim.running && g.sim.alphaTarget == 0 {
		// Without motion the layout jumps to rest, unless a drag holds it.
		g.sim.settle()
	}
	layoutMoving := false
	if g.sim.running {
		g.tickDebt += float64(elapsed) / float64(tickInterval)
		ticks := min(int(g.tickDebt), maxTicksPerFrame)
		g.tickDebt = math.Min(g.tickDebt-float64(ticks), 1)
		if ticks > 0 {
			// While the first layout settles, keep the fitted graph anchored
			// as its shape drifts around the origin.
			anchor := g.autoFit && !g.userInteracted
			bx, by, ok := g.boundsCenter()
			g.sim.step(ticks)
			if ax, ay, ok2 := g.boundsCenter(); anchor && ok && ok2 {
				dx, dy := (bx-ax)*g.camera.zoom, (by-ay)*g.camera.zoom
				g.camera.x, g.camera.y = g.camera.x+dx, g.camera.y+dy
				g.target.x, g.target.y = g.target.x+dx, g.target.y+dy
			}
		}
		layoutMoving = g.sim.running
	}
	if g.autoFit && !g.sim.running {
		g.autoFit = false
		if !g.userInteracted {
			g.fit(false)
		}
	}

	dx, dy, dz := g.target.x-g.camera.x, g.target.y-g.camera.y, g.target.zoom-g.camera.zoom
	cameraMoving := math.Abs(dx) > positionEpsilon || math.Abs(dy) > positionEpsilon || math.Abs(dz) > zoomEpsilon
	if cameraMoving && !g.reduceMotion {
		amount := 1 - math.Exp(-float64(elapsed)/float64(cameraResponse))
		g.camera = camera{g.camera.x + dx*amount, g.camera.y + dy*amount, g.camera.zoom + dz*amount}
	} else {
		g.camera, cameraMoving = g.target, false
	}

	hoverMoving := false
	if g.reduceMotion {
		g.hoverProgress = 1
	} else if g.hoverProgress < 1 {
		g.hoverProgress = math.Min(1, g.hoverProgress+float64(elapsed)/float64(hoverTransition))
		hoverMoving = g.hoverProgress < 1
	}

	if g.revealing && !g.reduceMotion {
		g.revealed = revealCount(len(g.nodes), now.Sub(g.revealStart))
		if g.revealed >= len(g.nodes) {
			g.revealing = false
		}
	} else {
		g.revealing, g.revealed = false, len(g.nodes)
	}
	return layoutMoving || cameraMoving || hoverMoving || g.revealing
}

// revealCount eases nodes in by their order over the reveal duration.
func revealCount(count int, elapsed time.Duration) int {
	if count <= 0 {
		return 0
	}
	if elapsed >= revealDuration {
		return count
	}
	if elapsed < 0 {
		return 0
	}
	progress := float64(elapsed) / float64(revealDuration)
	eased := 1 - (1-progress)*(1-progress)
	return max(1, min(count, int(math.Ceil(float64(count)*eased))))
}

// resize keeps the view centred on the same graph point as the canvas
// changes size.
func (g *graphView) resize(width, height float64) {
	width, height = max(1, width), max(1, height)
	if g.width == 0 || !g.ready {
		g.width, g.height = width, height
		return
	}
	dx, dy := (width-g.width)/2, (height-g.height)/2
	g.width, g.height = width, height
	g.camera.x, g.camera.y = g.camera.x+dx, g.camera.y+dy
	g.target.x, g.target.y = g.target.x+dx, g.target.y+dy
}

func (g *graphView) boundsCenter() (float64, float64, bool) {
	if g.sim == nil || len(g.sim.bodies) == 0 {
		return 0, 0, false
	}
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, b := range g.sim.bodies {
		minX, minY, maxX, maxY = min(minX, b.x), min(minY, b.y), max(maxX, b.x), max(maxY, b.y)
	}
	return (minX + maxX) / 2, (minY + maxY) / 2, true
}

func (g *graphView) fit(immediate bool) {
	if g.sim == nil || len(g.sim.bodies) == 0 || g.width == 0 {
		return
	}
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, b := range g.sim.bodies {
		minX, minY, maxX, maxY = min(minX, b.x), min(minY, b.y), max(maxX, b.x), max(maxY, b.y)
	}
	zoom := clamp(math.Min(
		max(1, g.width-2*fitPadding)/max(1, maxX-minX),
		max(1, g.height-2*fitPadding)/max(1, maxY-minY),
	), minZoom, maxZoom)
	g.target = camera{g.width/2 - (minX+maxX)/2*zoom, g.height/2 - (minY+maxY)/2*zoom, zoom}
	if immediate || g.reduceMotion {
		g.camera = g.target
	}
}

// zoomAt scales the view around a canvas point, immediately or toward the
// camera's target.
func (g *graphView) zoomAt(x, y, factor float64, animated bool) {
	from := g.camera
	if animated {
		from = g.target
	}
	zoom := clamp(from.zoom*factor, minZoom, maxZoom)
	gx, gy := (x-from.x)/from.zoom, (y-from.y)/from.zoom
	g.target = camera{x - gx*zoom, y - gy*zoom, zoom}
	if !animated || g.reduceMotion {
		g.camera = g.target
	}
}

func screenRadius(diameter float32, zoom float64) float64 {
	return math.Max(minScreenRadius, float64(diameter)/2*math.Min(1, zoom/shrinkZoom))
}

func (g *graphView) visibleCount() int {
	if !g.ready || g.sim == nil {
		return 0
	}
	return min(len(g.nodes), g.revealed, len(g.sim.bodies))
}

func (g *graphView) screen(i int) (float64, float64) {
	b := g.sim.bodies[i]
	return b.x*g.camera.zoom + g.camera.x, b.y*g.camera.zoom + g.camera.y
}

// hit returns the node nearest a canvas point within its radius, or -1.
func (g *graphView) hit(x, y float64) int {
	best, bestDistance := -1, math.Inf(1)
	for i := range g.visibleCount() {
		sx, sy := g.screen(i)
		d := math.Hypot(x-sx, y-sy)
		if d <= math.Max(hitRadiusMin, screenRadius(g.nodes[i].diameter, g.camera.zoom)) && d < bestDistance {
			best, bestDistance = i, d
		}
	}
	return best
}

func (g *graphView) paint(p *ui.Painter, r ui.Rect) {
	count := g.visibleCount()
	if count == 0 {
		return
	}
	transition := 1.0
	if !g.reduceMotion {
		transition = 1 - math.Pow(1-g.hoverProgress, 3)
	}
	amount := g.hoverAmount(transition)
	ox, oy := float64(r.X), float64(r.Y)
	w, h := float64(r.W), float64(r.H)
	p.Clip(r, 0, func() {
		edgeEnds := func(e renderEdge) (x0, y0, x1, y1 float64, ok bool) {
			if e.source >= count || e.target >= count {
				return 0, 0, 0, 0, false
			}
			x0, y0 = g.screen(e.source)
			x1, y1 = g.screen(e.target)
			if math.Max(x0, x1) < 0 || math.Min(x0, x1) > w || math.Max(y0, y1) < 0 || math.Min(y0, y1) > h {
				return 0, 0, 0, 0, false
			}
			return x0 + ox, y0 + oy, x1 + ox, y1 + oy, true
		}
		for _, e := range g.edges {
			x0, y0, x1, y1, ok := edgeEnds(e)
			if !ok {
				continue
			}
			color := e.color.Alpha(e.opacity)
			if amount > 0 {
				emphasis := g.edgeEmphasis(e, transition)
				peak := mutedEdgeOpacity + (float64(e.hover)-mutedEdgeOpacity)*emphasis
				alpha := float64(e.opacity) + (peak-float64(e.opacity))*amount
				mute := muteAmount(g.previousHoverDepth, g.hoverDepths, transition, e.source, e.target)
				color = e.color.Mix(g.muted, float32(mute)).Alpha(float32(alpha))
			}
			segment(p, x0, y0, x1, y1, e.width, color)
		}

		for i := range count {
			n := g.nodes[i]
			x, y := g.screen(i)
			radius := screenRadius(n.diameter, g.camera.zoom)
			if x+radius < 0 || x-radius > w || y+radius < 0 || y-radius > h {
				continue
			}
			emphasis := g.nodeEmphasis(i, transition)
			alpha := float32(1 - amount*(1-emphasis)*(1-mutedNodeOpacity))
			fill := n.fill.Mix(g.muted, float32(muteAmount(g.previousHoverDepth, g.hoverDepths, transition, i, i)))
			// Fully rounded boxes draw as circles without rasterizing paths.
			box := ui.Rect{X: float32(x + ox - radius), Y: float32(y + oy - radius), W: float32(2 * radius), H: float32(2 * radius)}
			if radius < 2 {
				p.Fill(box, fill.Alpha(alpha), float32(radius))
				continue
			}
			// A soft light falls from the top left onto a deeper rim.
			p.FillGradient(box, ui.LinearGradient{
				From:  fill.Mix(ui.RGB(255, 255, 255), .34).Alpha(alpha),
				To:    fill.Mix(ui.RGB(0, 0, 0), .2).Alpha(alpha),
				Angle: 135,
			}, float32(radius))
		}

		labelAlpha := clamp((g.camera.zoom-labelFadeStart)/(labelFadeEnd-labelFadeStart), 0, 1)
		if labelAlpha <= 0 {
			return
		}
		for i := range count {
			n := g.nodes[i]
			x, y := g.screen(i)
			if x < -140 || x > w+140 || y < -24 || y > h+24 || n.label == "" {
				continue
			}
			if n.measuredFor != n.label || n.fitted == "" {
				continue
			}
			emphasis := g.nodeEmphasis(i, transition)
			alpha := labelAlpha * (1 - amount*(1-emphasis)*(1-mutedLabelOpacity))
			radius := screenRadius(n.diameter, g.camera.zoom)
			p.RichText(float32(x+ox)-n.fittedWidth/2, float32(y+oy+radius+labelGap), 0, labelSpan(n.fitted, n.labelColor.Alpha(float32(alpha))))
		}
	})
}

// measureLabels fits labels that changed while building: measuring text
// while painting is not supported.
func (g *graphView) measureLabels(c *ui.Context) {
	measure := func(s string) float32 {
		width, _ := c.MeasureText(0, labelSpan(s, ui.Color{}))
		return width
	}
	for i := range g.nodes {
		n := &g.nodes[i]
		if n.measuredFor != n.label {
			n.fitted, n.fittedWidth = fitLabel(n.label, labelMaxWidth, measure)
			n.measuredFor = n.label
		}
	}
}

// segment draws a line with round caps as a rotated, fully rounded box,
// which the GPU draws directly where a path would need a rasterized mask.
func segment(p *ui.Painter, x0, y0, x1, y1 float64, width float32, c ui.Color) {
	length := math.Hypot(x1-x0, y1-y0)
	if length == 0 {
		return
	}
	mx, my := float32((x0+x1)/2), float32((y0+y1)/2)
	angle := float32(math.Atan2(y1-y0, x1-x0) * 180 / math.Pi)
	p.Transform(ui.Rotation(angle), mx, my, func() {
		half := float32(length)/2 + width/2
		p.Fill(ui.Rect{X: mx - half, Y: my - width/2, W: 2 * half, H: width}, c, width/2)
	})
}

func labelSpan(s string, c ui.Color) ui.Span {
	return ui.Span{Text: s, Size: labelSize, Weight: 500, Color: c}
}

// fitLabel shortens a label with an ellipsis to fit width.
func fitLabel(label string, width float32, measure func(string) float32) (string, float32) {
	if w := measure(label); w <= width {
		return label, w
	}
	const ellipsis = "…"
	if measure(ellipsis) > width {
		return "", 0
	}
	runes := []rune(label)
	lo, hi := 0, len(runes)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if measure(string(runes[:mid])+ellipsis) <= width {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	fitted := string(runes[:lo]) + ellipsis
	return fitted, measure(fitted)
}

func (g *graphView) hoverAmount(transition float64) float64 {
	if g.previousHovered >= 0 {
		if g.hovered >= 0 {
			return 1
		}
		return 1 - transition
	}
	if g.hovered >= 0 {
		return transition
	}
	return 0
}

// emphasis is full for the hovered node and slightly less for its
// neighbourhood.
func emphasis(depth int, ok bool) float64 {
	switch {
	case !ok:
		return 0
	case depth == 0:
		return 1
	default:
		return .8
	}
}

func (g *graphView) nodeEmphasis(i int, transition float64) float64 {
	pd, pok := g.previousHoverDepth[i]
	cd, cok := g.hoverDepths[i]
	previous, current := emphasis(pd, pok), emphasis(cd, cok)
	return previous + (current-previous)*transition
}

func edgeEmphasis(depths map[int]int, e renderEdge) float64 {
	s, sok := depths[e.source]
	t, tok := depths[e.target]
	if !sok || !tok {
		return 0
	}
	return emphasis(max(s, t), true)
}

func (g *graphView) edgeEmphasis(e renderEdge, transition float64) float64 {
	previous, current := edgeEmphasis(g.previousHoverDepth, e), edgeEmphasis(g.hoverDepths, e)
	return previous + (current-previous)*transition
}

// muteAmount fades what lies outside the hovered neighbourhood toward the
// muted color; a node passes itself as both ends.
func muteAmount(previous, current map[int]int, transition float64, a, b int) float64 {
	muted := func(depths map[int]int) float64 {
		if depths == nil {
			return 0
		}
		_, aok := depths[a]
		_, bok := depths[b]
		if aok && bok {
			return 0
		}
		return 1
	}
	from, to := muted(previous), muted(current)
	return from + (to-from)*clamp(transition, 0, 1)
}

// hoverDepths walks the graph breadth-first from root up to limit edges.
func hoverDepths(adjacency [][]int, root, limit int) map[int]int {
	if root < 0 || root >= len(adjacency) {
		return nil
	}
	depths := map[int]int{root: 0}
	queue := []int{root}
	for head := 0; head < len(queue); head++ {
		node := queue[head]
		depth := depths[node]
		if depth >= limit {
			continue
		}
		for _, next := range adjacency[node] {
			if _, ok := depths[next]; !ok {
				depths[next] = depth + 1
				queue = append(queue, next)
			}
		}
	}
	return depths
}

func adjacency(count int, edges []renderEdge) [][]int {
	result := make([][]int, count)
	for _, e := range edges {
		result[e.source] = append(result[e.source], e.target)
		result[e.target] = append(result[e.target], e.source)
	}
	return result
}

func (g *graphView) setHovered(i int) {
	if i == g.hovered {
		return
	}
	g.previousHovered, g.previousHoverDepth = g.hovered, g.hoverDepths
	g.hovered = i
	g.hoverDepths = hoverDepths(g.adjacency, i, g.hoverDepth)
	g.hoverProgress = 0
	if g.reduceMotion {
		g.hoverProgress = 1
	}
}

func (g *graphView) refreshHover() {
	g.hoverDepths = hoverDepths(g.adjacency, g.hovered, g.hoverDepth)
	g.previousHoverDepth = hoverDepths(g.adjacency, g.previousHovered, g.hoverDepth)
}

func (g *graphView) cursor() ui.Cursor {
	switch {
	case g.gesture == gesturePan || g.gesture == gestureNode:
		return ui.CursorGrabbing
	case g.hovered >= 0 && g.hovered < len(g.nodes) && g.nodes[g.hovered].openable:
		return ui.CursorPointer
	default:
		return ui.CursorGrab
	}
}

// input handles the pointer as it comes: pressing a node drags it once the
// pointer moves past the click tolerance, and pressing empty space pans.
func (g *graphView) input(ev ui.InputEvent, open func(string)) bool {
	x, y := float64(ev.X), float64(ev.Y)
	switch ev.Kind {
	case ui.InputPointerDown:
		if !g.ready {
			return false
		}
		g.pressClicks = ev.Clicks
		i := g.hit(x, y)
		switch {
		case i >= 0 && ev.Button == 0:
			b := g.sim.bodies[i]
			g.gesture, g.dragIndex = gestureNodePending, i
			g.offsetX = (x-g.camera.x)/g.camera.zoom - b.x
			g.offsetY = (y-g.camera.y)/g.camera.zoom - b.y
		case i < 0 && (ev.Button == 0 || ev.Button == 2) && ev.Mods&ui.Ctrl == 0:
			// Button 2 is the middle button.
			g.gesture = gesturePan
			g.setHovered(-1)
		default:
			return false
		}
		g.startX, g.startY, g.lastX, g.lastY = x, y, x, y
		g.moved, g.lastDragged = false, false
		g.userInteracted = true
		g.target = g.camera
		return true
	case ui.InputPointerMove:
		if g.gesture != gestureNone && math.Hypot(x-g.startX, y-g.startY) > clickTolerance {
			g.moved = true
		}
		switch g.gesture {
		case gesturePan:
			g.camera.x += x - g.lastX
			g.camera.y += y - g.lastY
			g.target = g.camera
			g.lastX, g.lastY = x, y
			return true
		case gestureNodePending, gestureNode:
			if !g.moved {
				return true
			}
			g.gesture = gestureNode
			gx := (x-g.camera.x)/g.camera.zoom - g.offsetX
			gy := (y-g.camera.y)/g.camera.zoom - g.offsetY
			g.sim.pin(g.dragIndex, gx, gy)
			return true
		}
		if g.ready {
			g.setHovered(g.hit(x, y))
		}
		return false
	case ui.InputPointerUp:
		if g.gesture == gestureNone {
			return false
		}
		dragged := g.gesture == gestureNode
		if dragged {
			g.sim.release(g.dragIndex)
		}
		g.lastDragged = dragged || g.moved
		g.gesture = gestureNone
		if g.pressClicks >= 2 && ev.Button == 0 && !g.lastDragged {
			if i := g.hit(x, y); i >= 0 {
				if g.nodes[i].openable && open != nil {
					open(g.nodes[i].id)
				}
			} else {
				factor := 2.0
				if ev.Mods&ui.Shift != 0 {
					factor = .5
				}
				g.zoomAt(x, y, factor, true)
			}
		}
		g.setHovered(g.hit(x, y))
		return true
	}
	return false
}

// navigate pans and zooms the camera: the wheel and pinches zoom around
// the pointer, and two fingers on a touchpad pan.
func (g *graphView) navigate(e gesture.Event) {
	if !g.ready {
		return
	}
	g.userInteracted = true
	x, y := float64(e.X), float64(e.Y)
	switch e.Kind {
	case gesture.Pan:
		g.camera.x += float64(e.DX)
		g.camera.y += float64(e.DY)
		g.target = g.camera
		if g.gesture == gestureNone {
			g.setHovered(g.hit(x, y))
		}
	case gesture.Zoom:
		g.zoomAt(x, y, e.Scale, false)
	}
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(v, hi)) }
