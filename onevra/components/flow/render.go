package flow

import (
	"math"
	"time"

	"github.com/auron-labs/mygo-framework/onevra/components/gesture"
	"github.com/egoist/mygo/ui"
)

const (
	defaultNodeGap   = 40
	defaultRankGap   = 72
	edgeGap          = 24
	fallbackWidth    = 200
	fallbackHeight   = 72
	planeRoom        = 1 << 14
	mutedNodeOpacity = .45
	mutedEdgeOpacity = .35
	minZoom          = .1
	maxZoom          = 2.5
	zoomStep         = 1.25
	fitPadding       = 32
	cameraResponse   = 90 * time.Millisecond
	// relayoutMotion is how long after a change of topology or spacing
	// new node sizes still move nodes rather than placing them.
	relayoutMotion   = 600 * time.Millisecond
	maxFrameStep     = 48 * time.Millisecond
	positionEpsilon  = .05
	zoomEpsilon      = .0005
	clickTolerance   = 4
	revealMargin     = 16
	defaultEdgeWidth = 1.5
	cornerRadius     = 12
	cornerSteps      = 6
	arrowLength      = 6
	arrowHalfWidth   = 3.5
	dashLength       = 4
	dashGap          = 5
	ringGap          = 3
	ringWidth        = 2
	labelSize        = 11
	labelPadX        = 6
	labelPadY        = 2
)

type camera struct{ x, y, zoom float64 }

type view struct {
	initialized bool
	nodes       []renderNode
	edges       []renderEdge
	index       map[string]int
	direction   Direction
	gaps        spacing
	layoutKey   uint64
	result      layoutResult
	// structureKey hashes the topology and spacing, which last changed at
	// structureAt; snap repositions this frame's layout at once.
	structureKey uint64
	structureAt  time.Time
	snap         bool

	reduceMotion bool
	selected     string
	selectFn     func(string)
	hovered      string
	focused      string

	ready, userMoved bool
	// camera is where the view is going and shown where it is this frame.
	camera, shown camera
	width, height float64
	lastFrame     time.Time

	panning, moved bool
	pressClicks    int
	startX, startY float64
	lastX, lastY   float64
	navigation     gesture.Navigation
}

func (v *view) init() {
	if v.initialized {
		return
	}
	v.initialized = true
	v.camera, v.shown = camera{zoom: 1}, camera{zoom: 1}
}

// resize keeps a diagram the user has not moved fitted to the view, and
// otherwise keeps the same point at the view's center.
func (v *view) resize(width, height float64) {
	if width <= 0 || height <= 0 || (width == v.width && height == v.height) {
		return
	}
	if v.width == 0 || !v.ready {
		v.width, v.height = width, height
		return
	}
	dx, dy := (width-v.width)/2, (height-v.height)/2
	v.width, v.height = width, height
	if !v.userMoved {
		v.fit(true)
		return
	}
	v.camera.x, v.camera.y = v.camera.x+dx, v.camera.y+dy
	v.shown.x, v.shown.y = v.shown.x+dx, v.shown.y+dy
}

// advanceCamera eases the shown camera toward its target and reports
// whether it is still moving.
func (v *view) advanceCamera(now time.Time) bool {
	elapsed := 16 * time.Millisecond
	if !v.lastFrame.IsZero() {
		elapsed = min(max(0, now.Sub(v.lastFrame)), maxFrameStep)
	}
	v.lastFrame = now
	dx, dy, dz := v.camera.x-v.shown.x, v.camera.y-v.shown.y, v.camera.zoom-v.shown.zoom
	if v.reduceMotion || (math.Abs(dx) < positionEpsilon && math.Abs(dy) < positionEpsilon && math.Abs(dz) < zoomEpsilon) {
		v.shown = v.camera
		return false
	}
	amount := 1 - math.Exp(-float64(elapsed)/float64(cameraResponse))
	v.shown = camera{v.shown.x + dx*amount, v.shown.y + dy*amount, v.shown.zoom + dz*amount}
	return true
}

func (v *view) fit(immediate bool) {
	if len(v.nodes) == 0 || v.width == 0 {
		return
	}
	w, h := max(1, v.result.width), max(1, v.result.height)
	zoom := clamp(min(max(1, v.width-2*fitPadding)/w, max(1, v.height-2*fitPadding)/h), minZoom, 1)
	v.camera = camera{v.width/2 - w/2*zoom, v.height/2 - h/2*zoom, zoom}
	v.userMoved = false
	if immediate || v.reduceMotion {
		v.shown = v.camera
	}
}

// zoomAt scales the view around a point of the view, immediately or toward
// the camera's target.
func (v *view) zoomAt(x, y, factor float64, animated bool) {
	from := v.shown
	if animated {
		from = v.camera
	}
	zoom := clamp(from.zoom*factor, minZoom, maxZoom)
	gx, gy := (x-from.x)/from.zoom, (y-from.y)/from.zoom
	v.camera = camera{x - gx*zoom, y - gy*zoom, zoom}
	v.userMoved = true
	if !animated || v.reduceMotion {
		v.shown = v.camera
	}
}

func (v *view) reveal(id string) {
	i, ok := v.index[id]
	if !ok || i >= len(v.result.x) || v.width == 0 {
		return
	}
	w, h := v.nodes[i].size()
	cx, cy := v.result.x[i]+w/2, v.result.y[i]+h/2
	v.camera.x, v.camera.y = v.width/2-cx*v.camera.zoom, v.height/2-cy*v.camera.zoom
	v.userMoved = true
	if v.reduceMotion {
		v.shown = v.camera
	}
}

// revealIfHidden reveals a node that is not wholly in view.
func (v *view) revealIfHidden(id string) {
	i, ok := v.index[id]
	if !ok || i >= len(v.result.x) {
		return
	}
	w, h := v.nodes[i].size()
	z := v.camera.zoom
	x, y := v.result.x[i]*z+v.camera.x, v.result.y[i]*z+v.camera.y
	if x < revealMargin || y < revealMargin || x+w*z > v.width-revealMargin || y+h*z > v.height-revealMargin {
		v.reveal(id)
	}
}

// hit returns the node shown under a point of the view, or -1.
func (v *view) hit(x, y float64) int {
	z := v.shown.zoom
	if z <= 0 {
		return -1
	}
	gx, gy := (x-v.shown.x)/z, (y-v.shown.y)/z
	for i := len(v.nodes) - 1; i >= 0; i-- {
		n := &v.nodes[i]
		w, h := n.size()
		if gx >= n.x && gx <= n.x+w && gy >= n.y && gy <= n.y+h {
			return i
		}
	}
	return -1
}

func (v *view) cursor() ui.Cursor {
	if v.panning {
		return ui.CursorGrabbing
	}
	return ui.CursorGrab
}

// input pans when empty space is dragged, leaving presses on nodes to
// them. Clicking empty space clears the selection and double-clicking it
// zooms.
func (v *view) input(ev ui.InputEvent) bool {
	x, y := float64(ev.X), float64(ev.Y)
	switch ev.Kind {
	case ui.InputPointerDown:
		// Button 2 is the middle button.
		if !v.ready || (ev.Button != 0 && ev.Button != 2) || v.hit(x, y) >= 0 {
			return false
		}
		v.panning, v.moved, v.pressClicks = true, false, ev.Clicks
		v.startX, v.startY, v.lastX, v.lastY = x, y, x, y
		v.camera = v.shown
		return true
	case ui.InputPointerMove:
		if !v.panning {
			return false
		}
		if math.Hypot(x-v.startX, y-v.startY) > clickTolerance {
			v.moved = true
		}
		v.camera.x += x - v.lastX
		v.camera.y += y - v.lastY
		v.shown, v.lastX, v.lastY = v.camera, x, y
		v.userMoved = true
		return true
	case ui.InputPointerUp:
		if !v.panning {
			return false
		}
		v.panning = false
		if v.moved || ev.Button != 0 {
			return true
		}
		if v.pressClicks >= 2 {
			factor := 2.0
			if ev.Mods&ui.Shift != 0 {
				factor = .5
			}
			v.zoomAt(x, y, factor, true)
		} else if v.selectFn != nil && v.selected != "" {
			v.selectFn("")
		}
		return true
	}
	return false
}

// navigate pans and zooms: the wheel and pinches zoom around the pointer,
// and two fingers on a touchpad pan.
func (v *view) navigate(e gesture.Event) {
	if !v.ready {
		return
	}
	switch e.Kind {
	case gesture.Pan:
		v.camera = camera{v.shown.x + float64(e.DX), v.shown.y + float64(e.DY), v.shown.zoom}
		v.shown = v.camera
		v.userMoved = true
	case gesture.Zoom:
		v.zoomAt(float64(e.X), float64(e.Y), e.Scale, false)
	}
}

type colors struct {
	line, emphasis, ring          ui.Color
	label, labelFace, labelBorder ui.Color
}

func (v *view) palette(t *ui.Theme) colors {
	return colors{
		line: t.TextMuted.Mix(t.Background, .45), emphasis: t.Accent.Alpha(.85), ring: t.Accent.Alpha(.7),
		label: t.TextMuted, labelFace: t.Background, labelBorder: t.Border.Alpha(.7),
	}
}

// emphasized reports whether an edge touches the hovered or selected node.
func (v *view) emphasized(e renderEdge) bool {
	for _, id := range [...]string{v.hovered, v.selected} {
		if id == "" {
			continue
		}
		if i, ok := v.index[id]; ok && (e.source == i || e.target == i) {
			return true
		}
	}
	return false
}

// shows reports whether both of an edge's ends show.
func (v *view) shows(e renderEdge) bool {
	return v.nodes[e.source].measured && v.nodes[e.target].measured
}

// route returns an edge's points where its ends show this frame: while
// nodes move to a new layout, each point follows the end it is nearer
// along the route.
func (v *view) route(i int, ox, oy float64) []point {
	if i >= len(v.result.routes) {
		return nil
	}
	laid := v.result.routes[i]
	if len(laid) < 2 {
		return nil
	}
	e := v.edges[i]
	shift := func(n int) point {
		return point{v.nodes[n].x - v.result.x[n], v.nodes[n].y - v.result.y[n]}
	}
	s, t := shift(e.source), shift(e.target)
	total := 0.0
	for k := 1; k < len(laid); k++ {
		total += math.Hypot(laid[k].x-laid[k-1].x, laid[k].y-laid[k-1].y)
	}
	out := make([]point, len(laid))
	along := 0.0
	for k, p := range laid {
		if k > 0 {
			along += math.Hypot(p.x-laid[k-1].x, p.y-laid[k-1].y)
		}
		w := 0.0
		if total > 0 {
			w = along / total
		}
		out[k] = point{p.x + s.x*(1-w) + t.x*w + ox, p.y + s.y*(1-w) + t.y*w + oy}
	}
	return out
}

func (v *view) paintEdges(p *ui.Painter, r ui.Rect, c colors) {
	ox, oy := float64(r.X), float64(r.Y)
	for pass := range 2 {
		for i, e := range v.edges {
			if v.emphasized(e) != (pass == 1) || !v.shows(e) {
				continue
			}
			points := v.route(i, ox, oy)
			if points == nil {
				continue
			}
			color, width := e.color, e.width
			if width <= 0 {
				width = defaultEdgeWidth
			}
			if pass == 1 {
				width += .5
				if color == (ui.Color{}) {
					color = c.emphasis
				}
			}
			if color == (ui.Color{}) {
				color = c.line
			}
			if v.nodes[e.source].muted || v.nodes[e.target].muted {
				color = color.Alpha(mutedEdgeOpacity)
			}
			drawRoute(p, points, width, color, e.dashed)
		}
	}
	for i, e := range v.edges {
		if e.label == "" || !v.shows(e) {
			continue
		}
		points := v.route(i, ox, oy)
		if points == nil {
			continue
		}
		alpha := float32(1)
		if v.nodes[e.source].muted || v.nodes[e.target].muted {
			alpha = mutedEdgeOpacity
		}
		x, y := labelAnchor(points)
		span := ui.Span{Text: e.label, Size: labelSize, Weight: 500, Color: c.label.Alpha(alpha)}
		w, h := p.MeasureText(0, span)
		box := ui.Rect{X: float32(x) - w/2 - labelPadX, Y: float32(y) - h/2 - labelPadY, W: w + 2*labelPadX, H: h + 2*labelPadY}
		p.Fill(box, c.labelFace.Alpha(alpha), box.H/2)
		p.Stroke(box, c.labelBorder.Alpha(alpha), box.H/2, 1)
		p.RichText(box.X+labelPadX, box.Y+labelPadY, 0, span)
	}
}

// paintSelection rings the selected node where it shows.
func (v *view) paintSelection(p *ui.Painter, r ui.Rect, c colors) {
	i, ok := v.index[v.selected]
	if !ok || !v.nodes[i].measured {
		return
	}
	n := &v.nodes[i]
	w, h := n.size()
	box := ui.Rect{X: r.X + float32(n.x) - ringGap, Y: r.Y + float32(n.y) - ringGap, W: float32(w) + 2*ringGap, H: float32(h) + 2*ringGap}
	p.Stroke(box, c.ring, CardRadius+ringGap, ringWidth)
}

// labelAnchor is the middle of a route's longest segment.
func labelAnchor(points []point) (float64, float64) {
	best, length := 1, -1.0
	for k := 1; k < len(points); k++ {
		if l := math.Hypot(points[k].x-points[k-1].x, points[k].y-points[k-1].y); l > length {
			best, length = k, l
		}
	}
	a, b := points[best-1], points[best]
	return (a.x + b.x) / 2, (a.y + b.y) / 2
}

// drawRoute draws a route with rounded bends, ending in an arrowhead.
func drawRoute(p *ui.Painter, points []point, width float32, c ui.Color, dashed bool) {
	n := len(points)
	end, before := points[n-1], points[n-2]
	dx, dy := end.x-before.x, end.y-before.y
	length := math.Hypot(dx, dy)
	if length == 0 {
		return
	}
	ux, uy := dx/length, dy/length
	head := math.Min(arrowLength, length)
	line := append([]point(nil), points...)
	line[n-1] = point{end.x - ux*head*.8, end.y - uy*head*.8}
	var path ui.Path
	if dashed {
		dashes(&path, rounded(line))
	} else {
		path.MoveTo(float32(line[0].x), float32(line[0].y))
		corners(line, func(a, control, b point) {
			path.LineTo(float32(a.x), float32(a.y)).QuadTo(float32(control.x), float32(control.y), float32(b.x), float32(b.y))
		})
		last := line[n-1]
		path.LineTo(float32(last.x), float32(last.y))
	}
	p.StrokePath(&path, width, c)
	var arrow ui.Path
	arrow.MoveTo(float32(end.x), float32(end.y)).
		LineTo(float32(end.x-ux*head-uy*arrowHalfWidth), float32(end.y-uy*head+ux*arrowHalfWidth)).
		LineTo(float32(end.x-ux*head+uy*arrowHalfWidth), float32(end.y-uy*head-ux*arrowHalfWidth)).
		Close()
	// The stroke's round joins soften the head's points.
	p.FillPath(&arrow, c)
	p.StrokePath(&arrow, 1, c)
}

// corners calls bend for each interior point of a route with where its
// curve leaves the incoming segment, the point itself as control and where
// it joins the outgoing one. Bends take at most half of either segment, so
// short jogs become smooth steps.
func corners(points []point, bend func(a, control, b point)) {
	for k := 1; k+1 < len(points); k++ {
		prev, at, next := points[k-1], points[k], points[k+1]
		in, out := math.Hypot(at.x-prev.x, at.y-prev.y), math.Hypot(next.x-at.x, next.y-at.y)
		if in == 0 || out == 0 {
			continue
		}
		r := math.Min(cornerRadius, math.Min(in, out)/2)
		bend(point{at.x - (at.x-prev.x)/in*r, at.y - (at.y-prev.y)/in*r}, at,
			point{at.x + (next.x-at.x)/out*r, at.y + (next.y-at.y)/out*r})
	}
}

// rounded returns a route as a polyline whose bends follow the same curves
// as a solid route's.
func rounded(points []point) []point {
	out := []point{points[0]}
	corners(points, func(a, control, b point) {
		out = append(out, a)
		for step := 1; step <= cornerSteps; step++ {
			t := float64(step) / cornerSteps
			u := 1 - t
			out = append(out, point{u*u*a.x + 2*u*t*control.x + t*t*b.x, u*u*a.y + 2*u*t*control.y + t*t*b.y})
		}
	})
	return append(out, points[len(points)-1])
}

// dashes adds the dashes of a polyline to path. Straight segments along an
// axis anchor the pattern to the plane, so edges sharing a track dash in
// step whichever way they run; curves continue it.
func dashes(path *ui.Path, points []point) {
	const period = dashLength + dashGap
	phase, drawing := 0.0, false
	for k := 1; k < len(points); k++ {
		a, b := points[k-1], points[k]
		length := math.Hypot(b.x-a.x, b.y-a.y)
		if length == 0 {
			continue
		}
		ux, uy := (b.x-a.x)/length, (b.y-a.y)/length
		if math.Abs(ux) > .999 || math.Abs(uy) > .999 {
			// Along the axis a dash covers [k·period, k·period+dashLength]
			// of the plane's coordinate, entered from either end.
			anchor := ux*a.x + uy*a.y
			if ux < 0 || uy < 0 {
				anchor += dashLength
			}
			if next := math.Mod(math.Mod(anchor, period)+period, period); math.Abs(next-phase) > .01 {
				drawing = false
				phase = next
			}
		}
		at := 0.0
		for at < length {
			offset := math.Mod(phase+at, period)
			run := period - offset
			if offset < dashLength {
				run = dashLength - offset
			}
			run = math.Min(run, length-at)
			if offset < dashLength {
				if !drawing {
					path.MoveTo(float32(a.x+ux*at), float32(a.y+uy*at))
					drawing = true
				}
				path.LineTo(float32(a.x+ux*(at+run)), float32(a.y+uy*(at+run)))
			} else {
				drawing = false
			}
			at += run
		}
		phase = math.Mod(phase+length, period)
	}
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(v, hi)) }
