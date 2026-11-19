package graph

import (
	"math"
	"runtime"
	"sync"
)

// The layout uses a velocity-damped force model with an exponentially cooling
// alpha, as d3-force does: centering, link springs, Barnes–Hut repulsion and
// collision run in that order so collision has the last word in each tick.
const (
	defaultCenter            = 0.1
	defaultRepel             = -1000
	defaultCollision         = 60
	defaultCollisionStrength = 0.5
	defaultLinkDistance      = 250

	repelDistanceMin2  = 30 * 30
	barnesHutTheta2    = 0.9 * 0.9
	velocityRetention  = 0.6
	alphaMin           = 0.001
	settleTicks        = 300
	initialTicks       = 80
	updateAlpha        = 0.35
	pinAlpha           = 0.4
	pinAlphaTarget     = 0.25
	initialAreaPerNode = 60 * 60
	newNodeOffset      = 24
	parallelChunk      = 256
)

var alphaDecay = 1 - math.Pow(alphaMin, 1.0/settleTicks)

// Forces tunes the layout. Zero fields use the defaults: Center 0.1,
// Repel -1000 (negative values push nodes apart), Collision radius 60 with
// strength 0.5, and LinkDistance 250 for edges without their own distance.
type Forces struct {
	Center, Repel                float64
	Collision, CollisionStrength float64
	LinkDistance                 float64
}

func (f Forces) resolved() Forces {
	or := func(v, fallback float64) float64 {
		if v == 0 || math.IsNaN(v) || math.IsInf(v, 0) {
			return fallback
		}
		return v
	}
	return Forces{
		Center: or(f.Center, defaultCenter), Repel: or(f.Repel, defaultRepel),
		Collision: math.Abs(or(f.Collision, defaultCollision)), CollisionStrength: or(f.CollisionStrength, defaultCollisionStrength),
		LinkDistance: math.Abs(or(f.LinkDistance, defaultLinkDistance)),
	}
}

type body struct {
	x, y, vx, vy float64
	pinned       bool
	px, py       float64
}

type spring struct {
	source, target                 int
	distance, strength, sourceBias float64
}

// springInput is an edge between body indices; zero distance and strength
// fall back to the forces' distance and a degree-derived strength.
type springInput struct {
	source, target     int
	distance, strength float64
}

type simulation struct {
	bodies             []body
	springs            []spring
	forces             Forces
	alpha, alphaTarget float64
	running            bool
	seed               uint32
	tree               quadtree
	pushes             [][2]float64
}

func newSimulation(bodies []body, springs []springInput, forces Forces) *simulation {
	s := &simulation{bodies: bodies, forces: forces.resolved(), alpha: 1, running: true, seed: 1}
	s.setSprings(springs)
	return s
}

// setSprings resolves strengths: edges without an explicit strength weaken
// with the smaller degree of their ends, counting only such edges, so hubs do
// not collapse; bias moves the lighter-connected end further.
func (s *simulation) setSprings(inputs []springInput) {
	semantic := make([]int, len(s.bodies))
	count := make([]int, len(s.bodies))
	for _, in := range inputs {
		count[in.source]++
		count[in.target]++
		if in.strength == 0 {
			semantic[in.source]++
			semantic[in.target]++
		}
	}
	s.springs = s.springs[:0]
	for _, in := range inputs {
		sp := spring{source: in.source, target: in.target, distance: in.distance, strength: in.strength}
		if sp.distance <= 0 {
			sp.distance = s.forces.LinkDistance
		}
		if sp.strength == 0 {
			sp.strength = 1 / float64(max(1, min(semantic[in.source], semantic[in.target])))
		}
		sp.sourceBias = float64(count[in.source]) / float64(count[in.source]+count[in.target])
		s.springs = append(s.springs, sp)
	}
}

// jiggle separates exactly coincident points with a deterministic sequence.
func (s *simulation) jiggle() float64 { return nextJiggle(&s.seed) }

func nextJiggle(seed *uint32) float64 {
	*seed = *seed*1664525 + 1013904223
	return (float64(*seed)/4294967296 - 0.5) * 1e-6
}

// bodySeed gives each body its own jiggle sequence in passes that run in
// parallel, so the result does not depend on scheduling.
func (s *simulation) bodySeed(i int) uint32 { return s.seed ^ uint32(i+1)*2654435761 } // #nosec G115 -- The seed deliberately wraps to 32 bits.

// parallel runs fn over [0, n) in contiguous chunks, across processors when
// the graph is large enough to repay it. Chunks must write only their own
// bodies' state.
func parallel(n int, fn func(lo, hi int)) {
	workers := min(runtime.GOMAXPROCS(0), n/parallelChunk)
	if workers <= 1 {
		fn(0, n)
		return
	}
	size := (n + workers - 1) / workers
	var wg sync.WaitGroup
	for lo := 0; lo < n; lo += size {
		wg.Add(1)
		go func(lo, hi int) {
			defer wg.Done()
			fn(lo, hi)
		}(lo, min(n, lo+size))
	}
	wg.Wait()
}

func (s *simulation) tick() {
	s.alpha += (s.alphaTarget - s.alpha) * alphaDecay
	s.center()
	s.link()
	s.repel()
	s.collide()
	for i := range s.bodies {
		b := &s.bodies[i]
		if b.pinned {
			b.x, b.y, b.vx, b.vy = b.px, b.py, 0, 0
			continue
		}
		b.vx *= velocityRetention
		b.vy *= velocityRetention
		b.x += b.vx
		b.y += b.vy
	}
	if s.alpha < alphaMin {
		s.running = false
	}
}

// step ticks while the layout runs and reports whether it still does.
func (s *simulation) step(ticks int) bool {
	for i := 0; i < ticks && s.running; i++ {
		s.tick()
	}
	return s.running
}

func (s *simulation) center() {
	k := s.forces.Center * s.alpha
	for i := range s.bodies {
		b := &s.bodies[i]
		b.vx -= b.x * k
		b.vy -= b.y * k
	}
}

func (s *simulation) link() {
	for _, sp := range s.springs {
		a, b := &s.bodies[sp.source], &s.bodies[sp.target]
		x := b.x + b.vx - a.x - a.vx
		y := b.y + b.vy - a.y - a.vy
		if x == 0 {
			x = s.jiggle()
		}
		if y == 0 {
			y = s.jiggle()
		}
		l := math.Sqrt(x*x + y*y)
		l = (l - sp.distance) / l * s.alpha * sp.strength
		x, y = x*l, y*l
		b.vx -= x * sp.sourceBias
		b.vy -= y * sp.sourceBias
		a.vx += x * (1 - sp.sourceBias)
		a.vy += y * (1 - sp.sourceBias)
	}
}

func (s *simulation) repel() {
	if len(s.bodies) < 2 || s.forces.Repel == 0 {
		return
	}
	t := &s.tree
	t.build(s.bodies, false)
	s.jiggle()
	// Each body gathers its own push from the read-only tree.
	parallel(len(s.bodies), func(lo, hi int) {
		for i := lo; i < hi; i++ {
			seed := s.bodySeed(i)
			s.repelFrom(0, i, t.size, &seed)
		}
	})
}

func (s *simulation) repelFrom(q int32, i int, size float64, seed *uint32) {
	t := &s.tree
	cell := &t.cells[q]
	if cell.count == 0 {
		return
	}
	b := &s.bodies[i]
	dx, dy := cell.cx-b.x, cell.cy-b.y
	l := dx*dx + dy*dy
	// A distant cell acts as one body at its centre of mass.
	if size*size/barnesHutTheta2 < l {
		if dx == 0 {
			dx = nextJiggle(seed)
			l += dx * dx
		}
		if dy == 0 {
			dy = nextJiggle(seed)
			l += dy * dy
		}
		if l < repelDistanceMin2 {
			l = math.Sqrt(repelDistanceMin2 * l)
		}
		w := s.forces.Repel * float64(cell.count) * s.alpha / l
		b.vx += dx * w
		b.vy += dy * w
		return
	}
	if cell.first < 0 {
		for _, child := range cell.children {
			if child >= 0 {
				s.repelFrom(child, i, size/2, seed)
			}
		}
		return
	}
	for j := cell.first; j >= 0; j = t.next[j] {
		if int(j) == i {
			continue
		}
		o := &s.bodies[j]
		dx, dy := o.x-b.x, o.y-b.y
		if dx == 0 {
			dx = nextJiggle(seed)
		}
		if dy == 0 {
			dy = nextJiggle(seed)
		}
		l := dx*dx + dy*dy
		if l < repelDistanceMin2 {
			l = math.Sqrt(repelDistanceMin2 * l)
		}
		w := s.forces.Repel * s.alpha / l
		b.vx += dx * w
		b.vy += dy * w
	}
}

func (s *simulation) collide() {
	if len(s.bodies) < 2 || s.forces.Collision <= 0 || s.forces.CollisionStrength == 0 {
		return
	}
	t := &s.tree
	t.build(s.bodies, true)
	reach := 2 * s.forces.Collision
	s.jiggle()
	if cap(s.pushes) < len(s.bodies) {
		s.pushes = make([][2]float64, len(s.bodies))
	}
	pushes := s.pushes[:len(s.bodies)]
	// Velocities stay unchanged until every body has gathered its push.
	parallel(len(s.bodies), func(lo, hi int) {
		for i := lo; i < hi; i++ {
			b := s.bodies[i]
			seed := s.bodySeed(i)
			pushes[i] = [2]float64{}
			s.collideWith(0, i, b.x+b.vx, b.y+b.vy, reach, t.x0, t.y0, t.size, &pushes[i], &seed)
		}
	})
	for i, push := range pushes {
		s.bodies[i].vx += push[0]
		s.bodies[i].vy += push[1]
	}
}

// collideWith pushes body i out of every node it overlaps by half the
// overlap; the other node takes the other half when its turn comes, as
// every node has the same radius.
func (s *simulation) collideWith(q int32, i int, xi, yi, reach, x0, y0, size float64, push *[2]float64, seed *uint32) {
	t := &s.tree
	cell := &t.cells[q]
	if cell.count == 0 || x0 > xi+reach || x0+size < xi-reach || y0 > yi+reach || y0+size < yi-reach {
		return
	}
	if cell.first < 0 {
		half := size / 2
		for k, child := range cell.children {
			if child >= 0 {
				s.collideWith(child, i, xi, yi, reach, x0+float64(k&1)*half, y0+float64(k>>1)*half, half, push, seed)
			}
		}
		return
	}
	for j := cell.first; j >= 0; j = t.next[j] {
		if int(j) == i {
			continue
		}
		o := &s.bodies[j]
		x := xi - o.x - o.vx
		y := yi - o.y - o.vy
		l := x*x + y*y
		if l >= reach*reach {
			continue
		}
		if x == 0 {
			// Coincident pairs separate in opposite directions.
			x = math.Abs(nextJiggle(seed))
			if int(j) < i {
				x = -x
			}
			l += x * x
		}
		if y == 0 {
			y = math.Abs(nextJiggle(seed))
			if int(j) < i {
				y = -y
			}
			l += y * y
		}
		l = math.Sqrt(l)
		l = (reach - l) / l * s.forces.CollisionStrength
		push[0] += x * l * .5
		push[1] += y * l * .5
	}
}

// settle runs a whole cooling schedule at once, for Reduced Motion.
func (s *simulation) settle() {
	s.step(settleTicks)
	s.running = false
}

// pin holds a body where the pointer drags it and keeps the layout warm
// around it until release.
func (s *simulation) pin(i int, x, y float64) {
	b := &s.bodies[i]
	b.pinned, b.px, b.py, b.x, b.y = true, x, y, x, y
	s.alpha = max(pinAlpha, s.alpha)
	s.alphaTarget = pinAlphaTarget
	s.running = true
}

func (s *simulation) release(i int) {
	s.bodies[i].pinned = false
	s.alphaTarget = 0
}

// reheat restarts a settled or running layout after its graph changed.
func (s *simulation) reheat() {
	s.alpha = max(updateAlpha, s.alpha)
	s.alphaTarget = 0
	s.running = true
}

// quadtree is a square region tree over body positions, rebuilt each pass;
// its buffers are reused between ticks.
type quadtree struct {
	cells        []quadCell
	next         []int32
	x0, y0, size float64
}

type quadCell struct {
	children [4]int32
	first    int32 // first body of a leaf, -1 for branches and empty cells
	count    int
	cx, cy   float64 // centre of mass
}

const maxQuadDepth = 48

func (t *quadtree) build(bodies []body, ahead bool) {
	t.cells = t.cells[:0]
	t.next = t.next[:0]
	pos := func(i int) (float64, float64) {
		b := bodies[i]
		if ahead {
			return b.x + b.vx, b.y + b.vy
		}
		return b.x, b.y
	}
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for i := range bodies {
		x, y := pos(i)
		minX, minY, maxX, maxY = min(minX, x), min(minY, y), max(maxX, x), max(maxY, y)
		t.next = append(t.next, -1)
	}
	t.x0, t.y0 = minX, minY
	t.size = max(maxX-minX, maxY-minY, 1)
	t.cells = append(t.cells, emptyCell())
	for i := range bodies {
		x, y := pos(i)
		t.insert(0, int32(i), x, y, t.x0, t.y0, t.size, 0, pos)
	}
}

func emptyCell() quadCell { return quadCell{children: [4]int32{-1, -1, -1, -1}, first: -1} }

func (t *quadtree) insert(q, i int32, x, y, x0, y0, size float64, depth int, pos func(int) (float64, float64)) {
	for {
		cell := &t.cells[q]
		n := float64(cell.count)
		cell.cx = (cell.cx*n + x) / (n + 1)
		cell.cy = (cell.cy*n + y) / (n + 1)
		cell.count++
		if cell.count == 1 {
			cell.first = i
			return
		}
		if cell.first >= 0 {
			// Coincident points, or a tree too deep to separate them,
			// share one leaf.
			fx, fy := pos(int(cell.first))
			if (fx == x && fy == y) || depth >= maxQuadDepth {
				t.next[i] = cell.first
				cell.first = i
				return
			}
			// Split the leaf, moving its bodies one level down.
			moved := cell.first
			cell.first = -1
			half := size / 2
			for j := moved; j >= 0; {
				following := t.next[j]
				t.next[j] = -1
				jx, jy := pos(int(j))
				k := quadrant(jx, jy, x0+half, y0+half)
				t.child(q, k)
				t.insertLeaf(t.cells[q].children[k], j, jx, jy)
				j = following
			}
		}
		half := size / 2
		k := quadrant(x, y, x0+half, y0+half)
		child := t.child(q, k)
		if t.cells[child].count == 0 {
			t.insertLeaf(child, i, x, y)
			return
		}
		q, x0, y0, size, depth = child, x0+float64(k&1)*half, y0+float64(k>>1)*half, half, depth+1
	}
}

// insertLeaf adds a body to a cell that has no branches yet. Every body
// already in it came from the same parent leaf, so a second body makes a
// leaf of coincident points only when they share a position; otherwise the
// caller's later inserts split it.
func (t *quadtree) insertLeaf(q, i int32, x, y float64) {
	cell := &t.cells[q]
	n := float64(cell.count)
	cell.cx = (cell.cx*n + x) / (n + 1)
	cell.cy = (cell.cy*n + y) / (n + 1)
	cell.count++
	t.next[i] = cell.first
	cell.first = i
}

func (t *quadtree) child(q int32, k int) int32 {
	if c := t.cells[q].children[k]; c >= 0 {
		return c
	}
	t.cells = append(t.cells, emptyCell())
	c := int32(len(t.cells) - 1) // #nosec G115 -- Cell indexes use int32 throughout the quadtree.
	t.cells[q].children[k] = c
	return c
}

func quadrant(x, y, mx, my float64) int {
	k := 0
	if x >= mx {
		k |= 1
	}
	if y >= my {
		k |= 2
	}
	return k
}
