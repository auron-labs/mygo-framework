package timechart

import "math"

// Range is the visible x-interval of one or more charts, kept inside the
// bounds of the data. Charts that share a Range zoom, pan and show the
// hover crosshair together. A Range belongs to the UI thread.
type Range struct {
	// MinSpan is the narrowest interval zooming reaches; NewRange sets a
	// millionth of the bounds.
	MinSpan float64

	min, max float64
	lo, hi   float64

	cross      float64
	hasCross   bool
	crossOwner *Chart
}

// NewRange returns a Range that shows all of from to to.
func NewRange(from, to float64) *Range {
	if to < from {
		from, to = to, from
	}
	return &Range{MinSpan: (to - from) * 1e-6, min: from, max: to, lo: from, hi: to}
}

// Bounds returns the extent of the data, which the visible interval never leaves.
func (r *Range) Bounds() (from, to float64) { return r.min, r.max }

// Visible returns the interval shown.
func (r *Range) Visible() (lo, hi float64) { return r.lo, r.hi }

// Set shows lo to hi, narrowed or widened to the allowed span and moved
// inside the bounds.
func (r *Range) Set(lo, hi float64) {
	if math.IsNaN(lo) || math.IsNaN(hi) || math.IsInf(lo, 0) || math.IsInf(hi, 0) {
		return
	}
	span := max(hi-lo, r.MinSpan)
	span = min(span, r.max-r.min)
	lo = max(r.min, min(lo, r.max-span))
	r.lo, r.hi = lo, lo+span
}

// ZoomAt scales the visible span by factor, below 1 to zoom in, keeping the
// value at anchor at the same place on screen.
func (r *Range) ZoomAt(anchor, factor float64) {
	span := r.hi - r.lo
	if span <= 0 || factor <= 0 || math.IsNaN(factor) || math.IsInf(factor, 0) {
		return
	}
	next := max(r.MinSpan, min(span*factor, r.max-r.min))
	at := (anchor - r.lo) / span
	r.Set(anchor-at*next, anchor-at*next+next)
}

// Pan moves the visible interval by delta, stopping at the bounds.
func (r *Range) Pan(delta float64) { r.Set(r.lo+delta, r.hi+delta) }

// Reset shows all of the data.
func (r *Range) Reset() { r.lo, r.hi = r.min, r.max }

// Crosshair returns the x the pointer hovers in any linked chart.
func (r *Range) Crosshair() (x float64, ok bool) { return r.cross, r.hasCross }

// SetCrosshair shows the crosshair at x in every linked chart; ClearCrosshair removes it.
func (r *Range) SetCrosshair(x float64) { r.cross, r.hasCross, r.crossOwner = x, true, nil }

// ClearCrosshair removes the crosshair.
func (r *Range) ClearCrosshair() { r.hasCross, r.crossOwner = false, nil }
