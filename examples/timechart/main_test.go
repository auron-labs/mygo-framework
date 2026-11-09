package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/theme"
)

func newTester(t *testing.T) (*ui.Tester, *screen) {
	t.Helper()
	s := newScreen(synthesize(time.Date(2026, 3, 1, 22, 30, 0, 0, time.UTC)))
	t.Cleanup(s.close)
	tt := ui.NewTester(func(c *ui.Context) { theme.Apply(c); s.view(c) }, 1000, 760)
	tt.Frame()
	return tt, s
}

func TestSyntheticNightHasEightHoursAt25Hz(t *testing.T) {
	r := synthesize(time.Date(2026, 3, 1, 22, 30, 0, 0, time.UTC))
	if len(r.times) != 720_000 || len(r.airflow) != 720_000 || len(r.spo2) != 720_000 || len(r.pulse) != 720_000 {
		t.Fatalf("got %d samples", len(r.times))
	}
	if len(r.apneas) == 0 || len(r.arousalEvents) == 0 {
		t.Fatal("no events")
	}
}

func TestScreenShowsLinkedChartsAndEvents(t *testing.T) {
	tt, _ := newTester(t)
	for deadline := time.Now().Add(5 * time.Second); (!tt.HasText("Apnea") || !tt.HasText("Arousal")) && time.Now().Before(deadline); time.Sleep(2 * time.Millisecond) {
		tt.Frame()
	}
	for _, text := range []string{"Overnight recording", "Apnea", "Arousal"} {
		if !tt.HasText(text) {
			t.Errorf("missing %q in %v", text, tt.Texts())
		}
	}
}

func TestZoomOnAnyChartMovesAllAxes(t *testing.T) {
	tt, s := newTester(t)
	rng := s.charts[0].Range
	lo, hi := rng.Visible()
	tt.Scroll(500, 80, 0, -400) // over the first chart
	tt.Frame()
	nlo, nhi := rng.Visible()
	if nhi-nlo >= hi-lo {
		t.Fatalf("scrolling did not zoom: %v to %v", nlo, nhi)
	}
	for _, chart := range s.charts {
		if clo, chi := chart.Range.Visible(); clo != nlo || chi != nhi {
			t.Fatal("charts do not share the range")
		}
	}
	tt.ClickAt(500, 80)
	tt.ClickAt(500, 80)
	if rlo, rhi := rng.Visible(); rlo != lo || rhi != hi {
		t.Fatal("double click did not reset")
	}
}
