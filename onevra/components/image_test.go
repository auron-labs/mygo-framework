package components_test

import (
	"bytes"
	"image"
	"image/color"
	"testing"

	"github.com/auron-labs/mygo-framework/v2/onevra/components"
	"github.com/auron-labs/mygo-framework/v2/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestImageSources(t *testing.T) {
	pixels := image.NewNRGBA(image.Rect(0, 0, 48, 24))
	for y := 0; y < 24; y++ {
		for x := 0; x < 48; x++ {
			pixels.SetNRGBA(x, y, color.NRGBA{uint8(x * 5), uint8(y * 10), 120, 255})
		}
	}
	var emptyBitmap *ui.Bitmap
	var emptySVG *ui.SVG
	sources := map[string]ui.ImageSource{
		"bitmap": ui.NewBitmap(pixels),
		"svg":    ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="48" height="24"><rect width="48" height="24" fill="#2563eb"/></svg>`)),
		"nil":    nil, "nil bitmap": emptyBitmap, "nil SVG": emptySVG,
	}
	for name, source := range sources {
		t.Run(name, func(t *testing.T) {
			for _, dark := range []bool{false, true} {
				for _, scale := range []float32{1, 2} {
					build := func(create func(*ui.Context, ui.ImageSource) ui.Element) *ui.Tester {
						tt := ui.NewTester(func(c *ui.Context) {
							c.SetTheme(theme.Palette(dark))
							ui.Row(c).Gap(4).Children(func() {
								create(c, source).Label("intrinsic")
								create(c, source).Width(60).Label("aspect")
								create(c, source).Size(60, 60).Fit(ui.Contain).Label("contain")
								create(c, source).Size(60, 60).Fit(ui.Cover).Radius(12).Label("cover")
								create(c, source).Size(60, 60).Fit(ui.FillBox).Grayscale().Label("stretch")
							})
						}, 400, 90)
						tt.SetScale(scale)
						return tt
					}
					got, want := build(components.Image), build(ui.Image)
					if !bytes.Equal(got.Image().Pix, want.Image().Pix) {
						t.Fatal("Image changed upstream rendering")
					}
					for _, label := range []string{"intrinsic", "aspect", "contain", "cover", "stretch"} {
						g, gok := got.Find(label)
						w, wok := want.Find(label)
						if !gok || !wok || g != w {
							t.Fatalf("%s bounds %v, want %v", label, g, w)
						}
					}
				}
			}
		})
	}
}
