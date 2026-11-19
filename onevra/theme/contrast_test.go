package theme

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

func luminance(c ui.Color) float64 {
	linear := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= 0.04045 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(c.R) + 0.7152*linear(c.G) + 0.0722*linear(c.B)
}

func contrast(a, b ui.Color) float64 {
	x, y := luminance(a), luminance(b)
	return (max(x, y) + 0.05) / (min(x, y) + 0.05)
}

func TestDarkPaletteContrast(t *testing.T) {
	p := DarkPalette()
	for name, bg := range map[string]ui.Color{
		"background": p.Background, "surface": p.Surface,
		"hover": p.SurfaceHover, "pressed": p.SurfacePressed,
		"selection": p.Selection.Over(p.Surface),
	} {
		for role, fg := range map[string]ui.Color{"text": p.Text, "muted": p.TextMuted} {
			if ratio := contrast(fg, bg); ratio < 4.5 {
				t.Errorf("%s on %s contrast = %.2f, want >= 4.5", role, name, ratio)
			}
		}
	}
	for name, bg := range map[string]ui.Color{
		"accent": p.Accent, "hover": p.AccentHover, "pressed": p.AccentPressed,
	} {
		if ratio := contrast(p.AccentText, bg); ratio < 4.5 {
			t.Errorf("accent text on %s contrast = %.2f, want >= 4.5", name, ratio)
		}
	}
	for name, fg := range map[string]ui.Color{
		"focus":  p.Focus.Over(p.SurfacePressed),
		"danger": p.Danger, "warning": p.Warning, "success": p.Success,
	} {
		if ratio := contrast(fg, p.SurfacePressed); ratio < 3 {
			t.Errorf("%s contrast = %.2f, want >= 3", name, ratio)
		}
	}
}

func TestDarkPaletteWarmHierarchy(t *testing.T) {
	p := DarkPalette()
	layers := []ui.Color{p.Background, p.Surface, p.SurfaceHover, p.SurfacePressed, p.Border}
	for i, c := range layers {
		if c.R < c.G || c.G < c.B {
			t.Errorf("layer %d lost its warm neutral hue: %v", i, c)
		}
		if i > 0 && luminance(c) <= luminance(layers[i-1]) { // #nosec G602 -- i > 0 guards the previous layer index.
			t.Errorf("layer %d must be lighter than the preceding layer", i)
		}
	}
	for _, c := range []ui.Color{LightPalette().Accent, p.Accent, p.AccentHover, p.AccentPressed} {
		if c.R <= c.G || c.G <= c.B {
			t.Errorf("accent must stay in the terracotta family: %v", c)
		}
	}
	if p.Selection != p.Accent.Alpha(0.24) || p.Focus != p.Accent.Alpha(0.8) {
		t.Error("selection and focus must share the dark accent hue")
	}
}
