package components

import (
	"fmt"
	"image/color"
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestPickerGlassSurfaces(t *testing.T) {
	for _, mode := range []bool{false, true} {
		for _, kind := range []string{"Color", "Calendar", "Time"} {
			t.Run(fmt.Sprintf("%v/%s", mode, kind), func(t *testing.T) {
				backdrop := ui.RGB(0, 0, 0)
				value := ui.RGB(110, 90, 230)
				day := time.Date(2026, 6, 15, 12, 30, 0, 0, time.UTC)
				var tint ui.Color
				tt := ui.NewTester(func(c *ui.Context) {
					c.SetTheme(theme.Palette(mode))
					tint = c.Theme().Background
					ui.Column(c).Fill().Padding(40).Background(backdrop).Children(func() {
						switch kind {
						case "Color":
							ColorPicker(c, &value).Label("Picker")
						case "Calendar":
							CalendarPicker(c, &day).Label("Picker")
						case "Time":
							TimePicker(c, &day).Label("Picker")
						}
					})
				}, 600, 700)
				click := func(label string) {
					t.Helper()
					if err := tt.Click(label); err != nil {
						t.Fatal(err)
					}
				}
				click("Picker")
				sample := func() color.RGBA {
					t.Helper()
					tt.Frame()
					label := map[string]string{"Color": "Saturation and brightness", "Calendar": "June 2026", "Time": "Choose time"}[kind]
					r, ok := tt.Find(label)
					if !ok {
						t.Fatalf("missing popup content %s", label)
					}
					// Sample blank panel padding, away from text, controls and the rim.
					x, y := r.X-6, r.Y+40
					if kind == "Time" {
						x, y = r.X+r.W/2, r.Y+3
					}
					return tt.Image().RGBAAt(int(x), int(y))
				}
				dark := sample()
				backdrop = ui.RGB(255, 255, 255)
				light := sample()
				if int(light.R)-int(dark.R) < 15 {
					t.Fatalf("picker glass obscures its backdrop: dark=%v light=%v", dark, light)
				}
				for _, p := range []color.RGBA{dark, light} {
					for i, channel := range []uint8{p.R, p.G, p.B} {
						want := []uint8{tint.R, tint.G, tint.B}[i]
						if delta := int(channel) - int(want); delta < -50 || delta > 50 {
							t.Fatalf("picker lost theme tint: %v, want near %v", p, tint)
						}
					}
				}
				if kind == "Color" {
					click("Color format")
					if err := tt.ChooseMenuItem("HSLA"); err != nil {
						t.Fatal(err)
					}
					if !tt.HasText("Lightness percent") || !tt.HasText("Saturation and brightness") {
						t.Fatal("native format selection closed the picker or failed to apply")
					}
					if value != ui.RGB(110, 90, 230) {
						t.Fatal("format selection changed color")
					}
				}
				tt.Key(0, ui.KeyEscape)
				if tt.HasText("Saturation and brightness") || tt.HasText("Choose time") || tt.HasText("Choose month") {
					t.Fatal("Escape did not dismiss picker")
				}
			})
		}
	}
}
