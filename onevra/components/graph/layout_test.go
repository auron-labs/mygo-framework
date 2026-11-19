package graph

import (
	"fmt"
	"math"
	"testing"
)

func settle(s *simulation) {
	for i := 0; i < 2000 && s.running; i++ {
		s.tick()
	}
}

func TestLinkedPairSettlesNearItsDistance(t *testing.T) {
	s := newSimulation([]body{{x: -5}, {x: 5}}, []springInput{{source: 0, target: 1, distance: 160}}, Forces{Repel: -1, Collision: 1})
	settle(s)
	if s.running {
		t.Fatal("layout did not settle")
	}
	d := math.Hypot(s.bodies[1].x-s.bodies[0].x, s.bodies[1].y-s.bodies[0].y)
	if d < 100 || d > 180 {
		t.Fatalf("distance = %g, want near 160", d)
	}
	if cx := (s.bodies[0].x + s.bodies[1].x) / 2; math.Abs(cx) > 1 {
		t.Fatalf("centre drifted to %g", cx)
	}
}

func TestCollisionSeparatesCoincidentNodes(t *testing.T) {
	bodies := make([]body, 6)
	s := newSimulation(bodies, nil, Forces{Repel: -1e-9})
	settle(s)
	for i := range s.bodies {
		for j := i + 1; j < len(s.bodies); j++ {
			if d := math.Hypot(s.bodies[i].x-s.bodies[j].x, s.bodies[i].y-s.bodies[j].y); d < 60 {
				t.Fatalf("nodes %d and %d overlap at %g", i, j, d)
			}
		}
	}
}

// Barnes–Hut repulsion approximates the exact pairwise sum.
func TestRepulsionApproximatesPairwiseSum(t *testing.T) {
	bodies := make([]body, 120)
	for i := range bodies {
		bodies[i].x, bodies[i].y = initialPosition(string(rune('a'+i%26))+string(rune('A'+i/26)), len(bodies), -1, 0)
	}
	s := newSimulation(append([]body(nil), bodies...), nil, Forces{})
	s.alpha = 1
	s.repel()
	for i, b := range bodies {
		var ex, ey float64
		for j, o := range bodies {
			if i == j {
				continue
			}
			dx, dy := o.x-b.x, o.y-b.y
			l := dx*dx + dy*dy
			if l < repelDistanceMin2 {
				l = math.Sqrt(repelDistanceMin2 * l)
			}
			ex += dx * defaultRepel / l
			ey += dy * defaultRepel / l
		}
		got := s.bodies[i]
		if err := math.Hypot(got.vx-ex, got.vy-ey); err > .2*math.Hypot(ex, ey)+.05 {
			t.Fatalf("body %d force (%g,%g), want near (%g,%g)", i, got.vx, got.vy, ex, ey)
		}
	}
}

func TestPinHoldsUntilRelease(t *testing.T) {
	s := newSimulation([]body{{x: -40}, {x: 40}}, []springInput{{source: 0, target: 1}}, Forces{})
	settle(s)
	s.pin(0, 500, 300)
	for range 400 {
		s.tick()
	}
	if b := s.bodies[0]; b.x != 500 || b.y != 300 || !s.running {
		t.Fatalf("pinned body = %+v, running %t", b, s.running)
	}
	s.release(0)
	settle(s)
	if s.running || s.bodies[0].pinned {
		t.Fatal("released layout did not settle")
	}
}

func TestSpringStrengthFollowsDegree(t *testing.T) {
	s := newSimulation(make([]body, 4), []springInput{
		{source: 0, target: 1}, {source: 0, target: 2}, {source: 0, target: 3}, {source: 1, target: 2, strength: .7},
	}, Forces{})
	if got := s.springs[0].strength; got != 1 {
		t.Fatalf("leaf spring strength = %g, want 1", got)
	}
	if got := s.springs[3].strength; got != .7 {
		t.Fatalf("explicit strength = %g", got)
	}
	if got := s.springs[2].sourceBias; got != .75 {
		t.Fatalf("hub bias = %g, want 0.75", got)
	}
	if got := s.springs[0].distance; got != defaultLinkDistance {
		t.Fatalf("default distance = %g", got)
	}
}

func TestInitialPositionsAreStableAndClustered(t *testing.T) {
	x0, y0 := initialPosition("notes/a.md", 40, 0, 4)
	x1, y1 := initialPosition("notes/a.md", 40, 0, 4)
	if x0 != x1 || y0 != y1 {
		t.Fatal("seeded position is not deterministic")
	}
	// Cluster 1 of 4 sits a quarter turn round the ring: below the origin.
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		_, y := initialPosition(id, 400, 1, 4)
		if y <= 0 {
			t.Fatalf("%s left its cluster sector: y = %g", id, y)
		}
	}
}

func TestNodeDiameterBounds(t *testing.T) {
	if nodeDiameter(0) != 12 || nodeDiameter(1e6) != 44 {
		t.Fatalf("diameter bounds = %g..%g", nodeDiameter(0), nodeDiameter(1e6))
	}
	if !(nodeDiameter(10) > nodeDiameter(5)) {
		t.Fatal("diameter does not grow with weight")
	}
}

// Large graphs gather forces in parallel; the result must not depend on
// scheduling.
func TestParallelLayoutIsDeterministic(t *testing.T) {
	var d Data
	for i := range 1500 {
		d.Nodes = append(d.Nodes, Node{ID: fmt.Sprint(i)})
		if i > 0 {
			d.Edges = append(d.Edges, Edge{Source: fmt.Sprint(i), Target: fmt.Sprint(i / 3)})
		}
	}
	var a, b graphView
	a.update(d, Options{}, DefaultPalette(false), false, false)
	b.update(d, Options{}, DefaultPalette(false), false, false)
	a.sim.step(5)
	b.sim.step(5)
	for i := range a.sim.bodies {
		if a.sim.bodies[i] != b.sim.bodies[i] {
			t.Fatal("parallel layout is not deterministic")
		}
	}
}
