package theme

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestPalettes(t *testing.T) {
	light, dark := LightPalette(), DarkPalette()
	if light.Dark {
		t.Error("LightPalette must not set Dark")
	}
	if !dark.Dark {
		t.Error("DarkPalette must set Dark")
	}
	if light.Background == dark.Background {
		t.Error("light and dark backgrounds must differ")
	}
}

func TestScrollbarTextTint(t *testing.T) {
	for _, tc := range []struct {
		name    string
		palette *ui.Theme
		alpha   float32
	}{
		{"light", LightPalette(), 0.30},
		{"dark", DarkPalette(), 0.35},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Verify the chosen text tint, not an unrelated RGB-distance threshold.
			want := tc.palette.Text.Alpha(tc.alpha)
			if got := tc.palette.Scrollbar; got != want {
				t.Errorf("scrollbar = %v, want text tint at %.0f%% opacity: %v", got, tc.alpha*100, want)
			}
		})
	}
}

func TestApplyFollowsAppearance(t *testing.T) {
	var got *ui.Theme
	tt := ui.NewTester(func(c *ui.Context) {
		Apply(c)
		got = c.Theme()
	}, 200, 100)
	if got.Background != LightPalette().Background {
		t.Fatalf("initial background = %v, want light %v", got.Background, LightPalette().Background)
	}
	tt.SetDark(true)
	if got.Background != DarkPalette().Background {
		t.Fatalf("after dark, background = %v, want dark %v", got.Background, DarkPalette().Background)
	}
	tt.SetDark(false)
	if got.Background != LightPalette().Background {
		t.Fatalf("after light, background = %v, want light %v", got.Background, LightPalette().Background)
	}
}

func TestOnevraFollowsPreferences(t *testing.T) {
	var got *ui.Theme
	tt := ui.NewTester(func(c *ui.Context) { got = Onevra(c) }, 200, 100)
	base := LightPalette()
	if *got != *base {
		t.Fatalf("usual preferences changed the palette: %+v", got)
	}
	accent := ui.Hex("#2563eb")
	tt.SetPreferences(ui.Preferences{Accent: accent, HighContrast: true, TextScale: 1.5})
	if got.FontSize != base.FontSize*1.5 {
		t.Errorf("font size = %v, want %v", got.FontSize, base.FontSize*1.5)
	}
	if got.Border == base.Border || got.TextMuted == base.TextMuted || got.Focus.A != 255 {
		t.Errorf("higher contrast not applied: border %v, muted %v, focus %v", got.Border, got.TextMuted, got.Focus)
	}
	if got.Accent != base.Accent {
		t.Errorf("accent = %v, want Onevra's %v", got.Accent, base.Accent)
	}
}
