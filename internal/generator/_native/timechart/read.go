package timechart

import "sort"

// readAt returns what a series reads at x: its value, interpolated for
// lines and held for steps, or for a band the lowest and highest point
// within tolerance of x. It reports false where the points do not reach x.
func readAt(kind Kind, p Points, x, tolerance float64) (lo, hi float64, ok bool) {
	n := len(p.X)
	if n == 0 || x < p.X[0] || x > p.X[n-1] {
		return 0, 0, false
	}
	i := sort.SearchFloat64s(p.X, x) // first point at or after x
	switch kind {                    //nolint:exhaustive // Line is the fall-through
	case Band:
		from, to := sort.SearchFloat64s(p.X, x-tolerance), sort.SearchFloat64s(p.X, x+tolerance)
		if to == from {
			to = min(n, from+1)
		}
		lo, hi = p.Y[from], p.Y[from]
		for _, v := range p.Y[from:to] {
			lo, hi = min(lo, v), max(hi, v)
		}
		return lo, hi, true
	case Step:
		if i < n && p.X[i] == x {
			return p.Y[i], p.Y[i], true
		}
		return p.Y[i-1], p.Y[i-1], true
	}
	if p.X[i] == x || i == 0 {
		return p.Y[i], p.Y[i], true
	}
	k := (x - p.X[i-1]) / (p.X[i] - p.X[i-1])
	v := p.Y[i-1] + k*(p.Y[i]-p.Y[i-1])
	return v, v, true
}
