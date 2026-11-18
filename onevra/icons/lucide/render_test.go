package lucide_test

import (
	"fmt"
	"testing"

	"github.com/auron-labs/mygo-framework/onevra/components"
	"github.com/auron-labs/mygo-framework/onevra/icons/lucide"
	"github.com/egoist/mygo/ui"
)

func TestMyGoIconIntegration(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, scale := range []float32{1, 2} {
			for _, size := range []float32{16, 24, 32} {
				t.Run(fmt.Sprintf("dark=%t/scale=%g/size=%g", dark, scale, size), func(t *testing.T) {
					tt := ui.NewTester(func(c *ui.Context) {
						ui.Row(c).TextColor(ui.RGB(240, 20, 80)).FontSize(size).Children(func() {
							components.Icon(c, lucide.Search).Label("Search")
						})
					}, 100, 100)
					tt.SetDark(dark)
					tt.SetScale(scale)
					rect, ok := tt.Find("Search")
					if !ok || rect.W != size || rect.H != size {
						t.Fatalf("icon bounds: %+v, found=%t", rect, ok)
					}
					img := tt.Image()
					colored := 0
					for y := int(rect.Y * scale); y < int((rect.Y+rect.H)*scale); y++ {
						for x := int(rect.X * scale); x < int((rect.X+rect.W)*scale); x++ {
							p := img.RGBAAt(x, y)
							if p.R > 200 && p.G < 50 && p.B > 50 && p.B < 110 {
								colored++
							}
						}
					}
					if colored == 0 {
						t.Fatal("no icon pixels in inherited text color")
					}
				})
			}
		}
	}
}
