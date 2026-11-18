package components

import (
	"fmt"
	"image/color"
	"testing"

	"github.com/auron-labs/mygo-framework/onevra/theme"
	"github.com/egoist/mygo/ui"
)

// Glass must sample the backdrop while remaining close to the original surface
// color, including Tooltip's inverted palette. Existing interaction tests cover
// placement, dismissal, focus and caller overrides independently of rendering.
func TestOverlayGlassTint(t *testing.T) {
	for _, mode := range []bool{false, true} {
		for _, kind := range []string{"Dialog", "Popover", "Tooltip", "Select"} {
			t.Run(fmt.Sprintf("%v/%s", mode, kind), func(t *testing.T) {
				backdrop := ui.RGB(0, 0, 0)
				open := true
				selected := "A"
				var bounds ui.Rect
				var tint ui.Color
				tt := ui.NewTester(func(c *ui.Context) {
					c.SetTheme(theme.Palette(mode))
					tint = c.Theme().Background
					ui.Box(c).Absolute().Left(0).Top(0).Size(480, 361).Background(backdrop)
					style := func(panel ui.Element) {
						panel.Size(180, 120).DrawOver(func(_ *ui.Painter, r ui.Rect) { bounds = r })
					}
					switch kind {
					case "Dialog":
						Dialog(c, &open, DialogOptions{}, style)
					case "Popover":
						anchor := Button(c, "Anchor", ButtonSecondary).Absolute().Left(100).Top(160)
						style(Popover(c, anchor, &open, nil))
					case "Tooltip":
						tint = c.Theme().Text
						anchor := Button(c, "Anchor", ButtonSecondary).Absolute().Left(100).Top(160)
						style(Tooltip(c, anchor, "Hint"))
					case "Select":
						sel := Select(c, &selected)
						sel.Trigger.Label("Anchor").Absolute().Left(100).Top(160)
						sel.Popup(style)
					}
				}, 480, 361)
				if kind == "Tooltip" {
					tt.Key(0, ui.KeyTab)
				}
				if kind == "Select" {
					if err := tt.Click("Anchor"); err != nil {
						t.Fatal(err)
					}
				}
				sample := func() color.RGBA {
					t.Helper()
					tt.Frame()
					if bounds.W <= 0 || bounds.H <= 0 {
						t.Fatal("missing surface")
					}
					return tt.Image().RGBAAt(int(bounds.X+bounds.W/2), int(bounds.Y+bounds.H/2))
				}
				dark := sample()
				if kind == "Dialog" || kind == "Popover" {
					palette := theme.LightPalette()
					if mode {
						palette = theme.DarkPalette()
					}
					edge := tt.Image().RGBAAt(int(bounds.X+bounds.W/2), int(bounds.Y))
					want := palette.Border
					if kind == "Dialog" {
						want = palette.Background.Mix(palette.Text, .25)
					}
					if edge.R != want.R || edge.G != want.G || edge.B != want.B {
						t.Fatalf("panel border = %v, want %v", edge, want)
					}
				}
				backdrop = ui.RGB(255, 255, 255)
				light := sample()
				if dark == light {
					t.Fatal("surface is opaque: backdrop does not affect glass")
				}
				minBackdropDelta := 15
				if kind == "Dialog" {
					// Dialog uses a stronger tint than the other overlay panels.
					minBackdropDelta = 12
				}
				if kind != "Tooltip" && int(light.R)-int(dark.R) < minBackdropDelta {
					t.Fatalf("panel glass obscures its backdrop: dark=%v light=%v", dark, light)
				}
				for _, pixel := range []color.RGBA{dark, light} {
					for i, got := range []uint8{pixel.R, pixel.G, pixel.B} {
						want := []uint8{tint.R, tint.G, tint.B}[i]
						delta := int(got) - int(want)
						if delta < -40 || delta > 40 {
							t.Fatalf("surface lost theme tint: pixel=%v tint=%v", pixel, tint)
						}
					}
				}
			})
		}
	}
}
