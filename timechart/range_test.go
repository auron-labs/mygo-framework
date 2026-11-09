package timechart

import (
	"math"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestZoomKeepsAnchorFixed(t *testing.T) {
	r := NewRange(0, 100)
	r.Set(20, 60)
	anchor := 30.0
	before := (anchor - 20) / 40
	r.ZoomAt(anchor, .5)
	lo, hi := r.Visible()
	if !near(hi-lo, 20) || !near((anchor-lo)/(hi-lo), before) {
		t.Fatalf("visible %v to %v, anchor fraction %v want %v", lo, hi, (anchor-lo)/(hi-lo), before)
	}
}

func TestRangeStaysInsideBounds(t *testing.T) {
	r := NewRange(10, 110)
	r.ZoomAt(50, 1000)
	if lo, hi := r.Visible(); lo != 10 || hi != 110 {
		t.Fatalf("zoom out went past the data: %v to %v", lo, hi)
	}
	r.ZoomAt(50, 1e-12)
	lo, hi := r.Visible()
	if !near(hi-lo, r.MinSpan) || lo < 10 || hi > 110 {
		t.Fatalf("zoom in passed the minimum span: %v to %v", lo, hi)
	}
	r.Set(20, 40)
	r.Pan(-1000)
	if lo, hi := r.Visible(); lo != 10 || !near(hi-lo, 20) {
		t.Fatalf("pan left: %v to %v", lo, hi)
	}
	r.Pan(1000)
	if lo, hi := r.Visible(); hi != 110 || !near(hi-lo, 20) {
		t.Fatalf("pan right: %v to %v", lo, hi)
	}
	r.Reset()
	if lo, hi := r.Visible(); lo != 10 || hi != 110 {
		t.Fatalf("reset: %v to %v", lo, hi)
	}
}

func TestZoomAtBoundKeepsSpan(t *testing.T) {
	r := NewRange(0, 100)
	r.Set(0, 50)
	r.ZoomAt(0, 2)
	if lo, hi := r.Visible(); lo != 0 || !near(hi, 100) {
		t.Fatalf("got %v to %v", lo, hi)
	}
	r.ZoomAt(100, 2)
	if lo, hi := r.Visible(); lo != 0 || hi != 100 {
		t.Fatalf("got %v to %v", lo, hi)
	}
}

func TestRangeIgnoresNonFiniteInput(t *testing.T) {
	r := NewRange(0, 100)
	r.Set(math.NaN(), 5)
	r.ZoomAt(50, math.NaN())
	r.ZoomAt(50, math.Inf(1))
	if lo, hi := r.Visible(); lo != 0 || hi != 100 {
		t.Fatalf("got %v to %v", lo, hi)
	}
}
