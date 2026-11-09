package timechart

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui/chart"
)

// timeSteps are the tick spacings, in seconds, that line up with the clock.
var timeSteps = []float64{
	.001, .002, .005, .01, .02, .05, .1, .2, .5,
	1, 2, 5, 10, 15, 30,
	60, 120, 300, 600, 900, 1800,
	3600, 7200, 10800, 21600, 43200, 86400,
}

// maxTicks is how many tick labels fit along width DIPs.
func maxTicks(width float32) int { return max(2, min(14, int(width/90))) }

// timeTicks returns clock-aligned ticks in lo to hi (Unix seconds), no more
// than count, along 0 to 1 of the interval. The spacing is the finest of
// timeSteps that fits, so labels stay readable from days down to
// milliseconds. Midnight is labelled with the date.
func timeTicks(lo, hi float64, loc *time.Location, count int) []chart.Tick {
	span := hi - lo
	if !(span > 0) {
		return nil
	}
	step := timeSteps[len(timeSteps)-1]
	for _, s := range timeSteps {
		if span/s < float64(count) {
			step = s
			break
		}
	}
	offset := func(unix float64) float64 {
		_, seconds := time.Unix(int64(math.Floor(unix)), 0).In(loc).Zone()
		return float64(seconds)
	}
	var ticks []chart.Tick
	first := math.Ceil((lo + offset(lo)) / step)
	for k := first; ; k++ {
		wall := k * step
		at := wall - offset(wall-offset(lo))
		if at > hi {
			break
		}
		ticks = append(ticks, chart.Tick{
			At: float32((at - lo) / span), Value: at, Label: formatTick(at, step, loc),
		})
	}
	return ticks
}

// formatTick labels a tick with only the precision its spacing needs.
func formatTick(unix, step float64, loc *time.Location) string {
	t := time.Unix(0, int64(math.Round(unix*1e9))).In(loc)
	switch {
	case step >= 86400 || (step >= 3600 && t.Hour() == 0 && t.Minute() == 0):
		return t.Format("Jan 2")
	case step >= 60:
		return t.Format("15:04")
	case step >= 1:
		return t.Format("15:04:05")
	case step >= .1:
		return t.Format("15:04:05.0")
	case step >= .01:
		return t.Format("15:04:05.00")
	}
	return t.Format("15:04:05.000")
}

// formatTime gives a moment in full, for the crosshair readout.
func formatTime(unix float64, loc *time.Location) string {
	return time.Unix(0, int64(math.Round(unix*1e9))).In(loc).Format("Jan 2 15:04:05.000")
}

// numericTicks returns round ticks of a plain numeric axis along 0 to 1 of lo to hi.
func numericTicks(lo, hi float64, count int) []chart.Tick {
	ticks := chart.NewLinear(lo, hi).Ticks(count)
	for i := range ticks {
		ticks[i].At = float32((ticks[i].Value - lo) / (hi - lo))
	}
	return ticks
}
