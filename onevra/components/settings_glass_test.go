package components

import (
	"fmt"
	"image/color"
	"testing"

	"github.com/auron-labs/mygo-framework/v2/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestSettingsPreservesHostBackground(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, narrow := range []bool{false, true} {
			for _, dialog := range []bool{false, true} {
				t.Run(fmt.Sprintf("%v/narrow=%v/dialog=%v", dark, narrow, dialog), func(t *testing.T) {
					state := SettingsState{}
					open, showSettings := true, false
					backdrop := ui.RGB(0, 0, 0)
					var bounds ui.Rect
					tt := ui.NewTester(func(c *ui.Context) {
						c.SetTheme(theme.Palette(dark))
						ui.Box(c).Absolute().Left(0).Top(0).Size(800, 600).Background(backdrop)
						build := func(panel ui.Element) {
							panel.Size(700, 500).Padding(0).Gap(0).DrawOver(func(_ *ui.Painter, r ui.Rect) { bounds = r })
							if showSettings {
								Settings(c, &state, settingsFixture(), SettingsOptions{Narrow: narrow})
							}
						}
						if dialog {
							Dialog(c, &open, DialogOptions{}, build)
						} else {
							panel := ui.Column(c)
							panel.Children(func() { build(panel) })
						}
					}, 800, 600)
					sample := func(x float32) color.RGBA {
						t.Helper()
						tt.Frame()
						// Empty space below the settings rows, clear of borders and controls.
						return tt.Image().RGBAAt(int(bounds.X+x), int(bounds.Y+bounds.H-60))
					}
					for _, bg := range []ui.Color{ui.RGB(0, 0, 0), ui.RGB(255, 255, 255)} {
						backdrop = bg
						showSettings = false
						wantContent, wantNav := sample(500), sample(100)
						showSettings = true
						if got := sample(500); got != wantContent {
							t.Fatalf("content obscures host: got %v, want %v", got, wantContent)
						}
						if !narrow {
							got := sample(100)
							for i, channel := range []uint8{got.R, got.G, got.B} {
								want := []uint8{wantNav.R, wantNav.G, wantNav.B}[i]
								if delta := int(channel) - int(want); delta < -10 || delta > 10 {
									t.Fatalf("navigation obscures host: got %v, want near %v", got, wantNav)
								}
							}
						}
					}
				})
			}
		}
	}
}
