package timechart

import (
	"math"
	"testing"
)

func signal(n int) (xs, ys []float64) {
	xs, ys = make([]float64, n), make([]float64, n)
	for i := range xs {
		xs[i] = float64(i) / 25
		ys[i] = math.Sin(xs[i] / 3)
	}
	return xs, ys
}

func TestDecimateKeepsNarrowSpikes(t *testing.T) {
	xs, ys := signal(250_000)
	ys[123_457], ys[200_001] = 50, -50
	p := Decimate(xs, ys, 0, xs[len(xs)-1], 800)
	if len(p.X) > 2*800+2 || len(p.X) != len(p.Y) {
		t.Fatalf("%d points for 800 pixels", len(p.X))
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	for i, x := range p.X {
		if i > 0 && x <= p.X[i-1] {
			t.Fatalf("x not ascending at %d", i)
		}
		lo, hi = min(lo, p.Y[i]), max(hi, p.Y[i])
	}
	if lo != -50 || hi != 50 {
		t.Fatalf("extremes %v and %v, want -50 and 50", lo, hi)
	}
}

func TestDecimateLimitsToTheWindowPlusOneNeighbour(t *testing.T) {
	xs, ys := signal(10_000)
	p := Decimate(xs, ys, 100, 110, 4000)
	if p.X[0] >= 100 || p.X[len(p.X)-1] <= 110 {
		t.Fatalf("no neighbour past the edges: %v to %v", p.X[0], p.X[len(p.X)-1])
	}
	if p.X[1] < 100 || p.X[len(p.X)-2] > 110 {
		t.Fatal("points beyond the edge neighbours")
	}
	if len(p.X) != 253 {
		t.Fatalf("got %d points, want the 251 samples in the window and a neighbour either side", len(p.X))
	}
}

func TestDecimateEmptyAndOutOfRange(t *testing.T) {
	xs, ys := signal(100)
	for name, p := range map[string]Points{
		"no samples":    Decimate(nil, nil, 0, 10, 100),
		"no pixels":     Decimate(xs, ys, 0, 4, 0),
		"inverted":      Decimate(xs, ys, 4, 0, 100),
		"before":        Decimate(xs, ys, -20, -10, 100),
		"after":         Decimate(xs, ys, 100, 200, 100),
		"mismatched ys": Decimate(xs, nil, 0, 4, 100),
	} {
		if name == "before" || name == "after" {
			if len(p.X) > 1 {
				t.Errorf("%s: %d points", name, len(p.X))
			}
			continue
		}
		if len(p.X) != 0 {
			t.Errorf("%s: %d points", name, len(p.X))
		}
	}
}

func TestDecimateSkipsNonFiniteValues(t *testing.T) {
	xs, ys := signal(1000)
	ys[500] = math.NaN()
	ys[501] = math.Inf(1)
	p := Decimate(xs, ys, 0, xs[999], 100)
	for _, v := range p.Y {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			t.Fatal("non-finite value in the result")
		}
	}
}

func TestDecimateDoesNotShareMemory(t *testing.T) {
	xs, ys := signal(10)
	p := Decimate(xs, ys, 0, 1, 100)
	ys[0] = 99
	if p.Y[0] == 99 {
		t.Fatal("result aliases the samples")
	}
}
