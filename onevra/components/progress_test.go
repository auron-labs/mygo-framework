package components

import (
	"fmt"
	"math"
	"testing"

	"github.com/auron-labs/mygo-framework/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestProgressValue(t *testing.T) {
	for _, tc := range []struct{ in, want float64 }{
		{0, 0},
		{0.4, 0.4},
		{1, 1},
		{2, 1},
		{-0.1, -1},
		{math.Inf(1), 1},
		{math.Inf(-1), -1},
		{math.NaN(), 0},
	} {
		if got := progressValue(tc.in); got != tc.want {
			t.Errorf("%v: got %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestProgressRendering(t *testing.T) {
	for _, ring := range []bool{false, true} {
		for _, dark := range []bool{false, true} {
			for _, value := range []float64{0, 0.01, 0.5, 1, -1, math.NaN(), math.Inf(1)} {
				t.Run(fmt.Sprint(ring, "/", dark, "/", value), func(t *testing.T) {
					tt := ui.NewTester(func(c *ui.Context) {
						c.SetTheme(theme.Palette(dark))
						ui.Column(c).Fill().Children(func() {
							if ring {
								ProgressRing(c, value).Size(80, 40).Label("progress")
							} else {
								Progress(c, value).Width(180).Label("progress")
							}
						})
					}, 200, 100)
					for _, scale := range []float32{1, 1.5} {
						tt.SetPreferences(ui.Preferences{TextScale: scale, ReduceMotion: true})
						r, ok := tt.Find("progress")
						if !ok || r.W <= 0 || r.H <= 0 {
							t.Fatal("missing indicator bounds")
						}
						if tt.Image() == nil {
							t.Fatal("missing rendering")
						}
					}
				})
			}
		}
	}
}
