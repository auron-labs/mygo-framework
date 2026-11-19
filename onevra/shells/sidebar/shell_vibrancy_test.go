package sidebar

import (
	"testing"

	"github.com/auron-labs/mygo-framework/v2/onevra/theme"
	"github.com/egoist/mygo/ui"
)

// Exercise effective material changes, not host OS assumptions. This includes
// unsupported systems, compositor failure and Window.SetVibrancy(None).
func TestShellEffectiveVibrancy(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			s := newShell(ShellOptions{Theme: theme.Onevra, LeftToolbar: func(c *ui.Context) { ui.Spacer(c) }})
			tt := ui.NewTester(s.view, 820, 600)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			palette := theme.Palette(dark)
			for _, vibrant := range []bool{false, true, false, true} {
				s.SetPage(nil)
				s.SetLeftExpanded(true)
				tt.SetVibrancy(vibrant)
				wantAlpha := uint8(255)
				if vibrant {
					wantAlpha = 0
				}
				for _, y := range []int{2, 400, 599} {
					pixel := tt.Image().RGBAAt(100, y)
					if pixel.A != wantAlpha {
						t.Fatalf("material=%v y=%d: sidebar=%v, want alpha %d", vibrant, y, pixel, wantAlpha)
					}
					if !vibrant && (pixel.R != palette.Surface.R || pixel.G != palette.Surface.G || pixel.B != palette.Surface.B) {
						t.Fatalf("fallback does not use theme surface: %v", pixel)
					}
					if tt.Image().RGBAAt(400, y).A != 255 {
						t.Fatal("main content must stay opaque")
					}
				}
				s.SetLeftExpanded(false)
				tt.Frame()
				if tt.Image().RGBAAt(100, 400).A != 255 {
					t.Fatal("collapsed sidebar left transparency")
				}
				s.SetLeftExpanded(true)
				s.SetPage(func(*ui.Context) {})
				tt.Frame()
				if tt.Image().RGBAAt(100, 400).A != 255 {
					t.Fatal("replacement page must stay opaque")
				}
			}
		})
	}
}
