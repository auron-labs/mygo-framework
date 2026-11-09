package timechart

import (
	"math"
	"sort"
)

// Points is a series window: X ascending, Y the value at each X.
type Points struct{ X, Y []float64 }

// Decimate reduces the samples of xs (ascending) and ys within from to to
// to at most 2*pixelWidth+2 points: the lowest and highest sample of each
// pixel column, in time order, so that narrow spikes survive, plus the
// nearest sample beyond each edge so lines run off the plot. It finds the
// window by binary search and visits only the samples in it. Samples with
// a non-finite y are skipped. The result does not share memory with the
// inputs, so a provider may hand it to the UI thread.
func Decimate(xs, ys []float64, from, to float64, pixelWidth int) Points {
	n := min(len(xs), len(ys))
	if n == 0 || pixelWidth <= 0 || !(to > from) {
		return Points{}
	}
	first := sort.Search(n, func(i int) bool { return xs[i] >= from })
	end := sort.Search(n, func(i int) bool { return xs[i] > to })
	if first > 0 {
		first--
	}
	if end < n {
		end++
	}
	out := Points{
		X: make([]float64, 0, min(end-first, 2*pixelWidth+2)),
		Y: make([]float64, 0, min(end-first, 2*pixelWidth+2)),
	}
	add := func(i int) {
		out.X = append(out.X, xs[i])
		out.Y = append(out.Y, ys[i])
	}
	finite := func(i int) bool { return !math.IsNaN(ys[i]) && !math.IsInf(ys[i], 0) }
	width := (to - from) / float64(pixelWidth)
	for i := first; i < end; {
		if !finite(i) {
			i++
			continue
		}
		if xs[i] < from || xs[i] > to {
			add(i)
			i++
			continue
		}
		column := int((xs[i] - from) / width)
		lowest, highest := i, i
		i++
		for ; i < end && xs[i] >= from && xs[i] <= to && int((xs[i]-from)/width) == column; i++ {
			if !finite(i) {
				continue
			}
			if ys[i] < ys[lowest] {
				lowest = i
			}
			if ys[i] > ys[highest] {
				highest = i
			}
		}
		add(min(lowest, highest))
		if lowest != highest {
			add(max(lowest, highest))
		}
	}
	return out
}
