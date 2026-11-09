package timechart

import (
	"slices"
	"testing"
	"time"
)

func TestTimeTicksAtEveryZoomLevel(t *testing.T) {
	start := float64(time.Date(2026, 3, 1, 22, 0, 0, 0, time.UTC).Unix())
	for _, span := range []float64{8 * 3600, 3600, 600, 60, 10, 1, .2, .05} {
		for _, width := range []float32{300, 900, 1600} {
			lo, hi := start+1234.5678, start+1234.5678+span
			count := maxTicks(width)
			ticks := timeTicks(lo, hi, time.UTC, count)
			if len(ticks) < 2 || len(ticks) > count {
				t.Errorf("span %v width %v: %d ticks, want 2 to %d", span, width, len(ticks), count)
			}
			prev := lo
			for _, tick := range ticks {
				if tick.Value < prev || tick.Value > hi || tick.At < 0 || tick.At > 1 {
					t.Errorf("span %v: tick %v outside %v to %v", span, tick.Value, lo, hi)
				}
				prev = tick.Value
			}
		}
	}
}

func TestTimeTicksLabelHoursOfNight(t *testing.T) {
	lo := float64(time.Date(2026, 3, 1, 21, 40, 0, 0, time.UTC).Unix())
	ticks := timeTicks(lo, lo+8*3600, time.UTC, 9)
	labels := make([]string, 0, len(ticks))
	for _, tick := range ticks {
		labels = append(labels, tick.Label)
	}
	want := []string{"22:00", "23:00", "Mar 2", "01:00", "02:00", "03:00", "04:00", "05:00"}
	if !slices.Equal(labels, want) {
		t.Fatalf("got %v want %v", labels, want)
	}
}

func TestTimeTicksFollowTheLocation(t *testing.T) {
	zone := time.FixedZone("plus-9:30", 9*3600+1800)
	lo := float64(time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC).Unix())
	ticks := timeTicks(lo, lo+3*3600, zone, 4)
	if len(ticks) == 0 || ticks[0].Label != "22:00" {
		t.Fatalf("got %+v", ticks)
	}
}

func TestSubSecondTicksShowFractions(t *testing.T) {
	lo := float64(time.Date(2026, 3, 1, 22, 0, 0, 0, time.UTC).Unix())
	ticks := timeTicks(lo+.01, lo+.31, time.UTC, 6)
	if len(ticks) < 2 || ticks[0].Label != "22:00:00.05" {
		t.Fatalf("got %+v", ticks)
	}
}

func TestNumericTicksMapOntoTheInterval(t *testing.T) {
	ticks := numericTicks(10, 20, 5)
	if len(ticks) == 0 || ticks[0].At != 0 || ticks[len(ticks)-1].At != 1 {
		t.Fatalf("got %+v", ticks)
	}
}
