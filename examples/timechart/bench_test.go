package main

import (
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/timechart"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/theme"
)

// BenchmarkFrame builds, lays out and paints (CPU renderer) the three
// linked charts over 8 h of 25 Hz samples with the points already fetched,
// as every frame of a zoom or pan within the fetched window does. The frame
// budget at 60 Hz is 16.6 ms.
func BenchmarkFrame(b *testing.B) {
	rec := synthesize(time.Date(2026, 3, 1, 22, 30, 0, 0, time.UTC))
	s := newScreen(rec)
	defer s.close()
	tt := ui.NewTester(func(c *ui.Context) { theme.Apply(c); s.view(c) }, 1000, 760)
	rng := s.charts[0].Range
	settle := func() {
		b.Helper()
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
			tt.Frame()
			if s.loaded() {
				return
			}
		}
		b.Fatal("chart data did not arrive")
	}
	night := 8.0 * 3600
	for _, mode := range []struct {
		name  string
		start func()
		move  func(i int)
	}{
		{"hover at full night", func() { rng.Reset() }, func(i int) { rng.SetCrosshair(rec.start + float64(i%1000)*28) }},
		{
			"zoom", func() { rng.Set(rec.start+2*3600, rec.start+4*3600) },
			func(i int) { rng.ZoomAt(rec.start+3*3600, []float64{.98, 1.02}[i%2]) },
		},
		{
			"pan", func() { rng.Set(rec.start+2*3600, rec.start+4*3600) },
			func(i int) { rng.Set(rec.start+2*3600+float64(i%200-100)*5, rec.start+4*3600+float64(i%200-100)*5) },
		},
		{
			"zoomed to seconds", func() { rng.Set(rec.start+night/2, rec.start+night/2+60) },
			func(i int) { rng.Pan(float64(i%2*2-1) * .05) },
		},
	} {
		b.Run(mode.name, func(b *testing.B) {
			mode.start()
			settle()
			b.ReportAllocs()
			for i := 0; b.Loop(); i++ {
				mode.move(i)
				tt.Frame()
			}
		})
	}
}

// BenchmarkProvider is the work a provider does off the UI thread for one
// window: the full night, and two hours.
func BenchmarkProvider(b *testing.B) {
	rec := synthesize(time.Date(2026, 3, 1, 22, 30, 0, 0, time.UTC))
	for name, window := range map[string][2]float64{
		"full night": {rec.start, rec.start + 8*3600},
		"two hours":  {rec.start + 3600, rec.start + 3*3600},
	} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				timechart.Decimate(rec.times, rec.airflow, window[0], window[1], 1500)
			}
		})
	}
}
