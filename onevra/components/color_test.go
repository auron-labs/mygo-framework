package components

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestColorConversion(t *testing.T) {
	for r := 0; r < 256; r += 17 {
		for g := 0; g < 256; g += 17 {
			for b := 0; b < 256; b += 17 {
				c := ui.Color{R: uint8(r), G: uint8(g), B: uint8(b), A: 123}
				if got := colorToHSV(c).color(c.A); got != c {
					t.Fatalf("round trip %v -> %v", c, got)
				}
			}
		}
	}
	for _, s := range []string{"", "fff", "#GG0000", "1234567", "123456789"} {
		if _, ok := parseColorHex(s, 20); ok {
			t.Fatalf("accepted %q", s)
		}
	}
	c, ok := parseColorHex(" #12aB34 ", 20)
	if !ok || c != (ui.Color{R: 18, G: 171, B: 52, A: 20}) {
		t.Fatal(c, ok)
	}
	c, ok = parseColorHex("12AB3480", 20)
	if !ok || c.A != 128 {
		t.Fatal(c, ok)
	}
}

func TestColorViewEditing(t *testing.T) {
	for _, dark := range []bool{false, true} {
		value := ui.RGB(255, 0, 0)
		changes := 0
		disabled := false
		var state *colorViewState
		tt := ui.NewTester(func(c *ui.Context) {
			ui.Column(c).Disabled(disabled).Children(func() {
				v := ColorView(c, &value)
				state = ui.Local(v.Element, "color-view", func() colorViewState { return colorViewState{} })
				if v.Changed() {
					changes++
				}
				Button(c, "Outside", ButtonOutline)
			})
		}, 400, 600)
		tt.SetDark(dark)
		click := func(label string) {
			t.Helper()
			if err := tt.Click(label); err != nil {
				t.Fatal(err)
			}
		}
		typeField := func(label, text string) {
			t.Helper()
			click(label)
			tt.Key(selectionModifier(), ui.KeyA)
			tt.Type(text)
		}
		typeField("Hex color", "11223380")
		if value != (ui.Color{R: 17, G: 34, B: 51, A: 128}) {
			t.Fatalf("hex: %v", value)
		}
		typeField("Red", "200")
		if value.R != 200 || value.A != 128 {
			t.Fatalf("RGB: %v", value)
		}
		typeField("Opacity percent", "25")
		if value.A != 64 {
			t.Fatalf("alpha: %v", value)
		}
		before := value
		typeField("Hex color", "not hex")
		if value != before || state.fields[3] != colorHex(before) {
			t.Fatal("unsupported hex text was not rejected", state.fields)
		}
		click("Outside")
		if state.fields[3] != colorHex(value) {
			t.Fatal("blur did not restore hex")
		}
		typeField("Green", "999")
		if value != before {
			t.Fatal("out of range applied")
		}
		tt.Key(0, ui.KeyEnter)
		if state.fields[1] != "34" {
			t.Fatal("submit did not reset", state.fields)
		}
		count := changes
		value = ui.RGB(0, 0, 255)
		tt.Frame()
		if changes != count || state.fields[3] != "0000FF" {
			t.Fatal("external change", changes, state.fields)
		}
		click("Hue")
		tt.Key(0, ui.KeyHome)
		if value != ui.RGB(255, 0, 0) {
			t.Fatal("hue keyboard", value)
		}
		tt.Key(0, ui.KeyEnd)
		beforeHSV := state.hsv
		click("Opacity")
		tt.Key(0, ui.KeyHome)
		if state.hsv != beforeHSV {
			t.Fatal("opacity changed latent HSV")
		}
		if value.A != 0 {
			t.Fatal("opacity keyboard", value)
		}
		value = ui.RGB(255, 0, 0)
		tt.Frame()
		r, _ := tt.Find("Saturation and brightness")
		tt.Press(r.X+r.W/2, r.Y+r.H/2)
		tt.Move(r.X+r.W+50, r.Y+r.H+50)
		tt.Release(r.X+r.W+50, r.Y+r.H+50)
		if value != ui.RGB(0, 0, 0) {
			t.Fatal("captured plane", value)
		}
		tt.Key(0, ui.KeyUp)
		if value.R == 0 || value.G != 0 || value.B != 0 {
			t.Fatal("black lost saturation", value)
		}
		disabled = true
		tt.Frame()
		before = value
		count = changes
		click("Hue")
		tt.Key(0, ui.KeyEnd)
		click("Saturation and brightness")
		tt.Key(0, ui.KeyRight)
		typeField("Hex color", "FFFFFF")
		if value != before || changes != count {
			t.Fatal("disabled edited color")
		}
	}
}

func TestColorViewFormats(t *testing.T) {
	value := ui.RGB(255, 0, 0)
	changes := 0
	var state *colorViewState
	tt := ui.NewTester(func(c *ui.Context) {
		v := ColorView(c, &value)
		state = ui.Local(v.Element, "color-view", func() colorViewState { return colorViewState{} })
		if v.Changed() {
			changes++
		}
	}, 360, 550)
	choose := func(label string) {
		t.Helper()
		if err := tt.Click("Color format"); err != nil {
			t.Fatal(err)
		}
		if err := tt.ChooseMenuItem(label); err != nil {
			t.Fatal(err)
		}
	}
	typeField := func(label, text string) {
		t.Helper()
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		tt.Key(selectionModifier(), ui.KeyA)
		tt.Type(text)
	}
	choose("HSBA")
	if state.format != colorHSBA || changes != 0 || value != ui.RGB(255, 0, 0) {
		t.Fatal("format changed color")
	}
	typeField("Hue degrees", "120")
	typeField("Saturation percent", "50%")
	typeField("Brightness percent", "50%")
	if value != ui.RGB(64, 128, 64) {
		t.Fatal("HSB editing", value)
	}
	before := value
	typeField("Hue degrees", "361")
	tt.Key(0, ui.KeyEnter)
	if value != before || state.fields[0] != "120" {
		t.Fatal("invalid hue", state.fields)
	}
	typeField("Brightness percent", "0")
	typeField("Hue degrees", "240")
	if value != ui.RGB(0, 0, 0) {
		t.Fatal("black hue changed RGB")
	}
	typeField("Brightness percent", "100")
	if value != ui.RGB(128, 128, 255) {
		t.Fatal("black lost hue or saturation", value)
	}
	count := changes
	choose("RGBA")
	if state.format != colorRGBA || changes != count || state.fields[0] != "128" {
		t.Fatal("RGBA mode sync")
	}
	choose("HSBA")
	value = ui.RGB(0, 255, 255)
	tt.Frame()
	if state.fields[0] != "180" || state.fields[1] != "100%" || changes != count {
		t.Fatal("HSB external sync", state.fields)
	}
}

func TestColorPlaneCorners(t *testing.T) {
	value := ui.RGB(255, 0, 0)
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Padding(20).Children(func() { ColorView(c, &value) })
	}, 360, 600)
	r, ok := tt.Find("Saturation and brightness")
	if !ok {
		t.Fatal("plane missing")
	}
	for _, corner := range []struct {
		x, y float32
		want ui.Color
	}{
		{r.X, r.Y, ui.RGB(255, 255, 255)},
		{r.X + r.W, r.Y, ui.RGB(255, 0, 0)},
		{r.X + r.W, r.Y + r.H, ui.RGB(0, 0, 0)},
		{r.X, r.Y + r.H, ui.RGB(0, 0, 0)},
	} {
		tt.Press(r.X+r.W/2, r.Y+r.H/2)
		tt.Move(corner.x, corner.y)
		tt.Release(corner.x, corner.y)
		if value != corner.want {
			t.Fatalf("corner (%v,%v): %v", corner.x, corner.y, value)
		}
	}
	// The last marker is centered on the bottom-left vertex, not inset.
	// Its outer dark rim must render to the left of the plane.
	img := tt.Image()
	visible := false
	for x := int(r.X) - 9; x < int(r.X)-2; x++ {
		for y := int(r.Y+r.H) - 3; y < int(r.Y+r.H)+3; y++ {
			if img.RGBAAt(x, y).A > 0 {
				visible = true
			}
		}
	}
	if !visible {
		t.Fatal("corner ring was clipped or inset")
	}
}

func TestColorChannelEndpoints(t *testing.T) {
	value := 0.0
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Padding(20).Children(func() {
			colorChannel(c, &value, 1, "Channel", func(p *ui.Painter, r ui.Rect) { p.Fill(r, ui.RGB(100, 100, 100), 0) })
		})
	}, 300, 100)
	r, ok := tt.Find("Channel")
	if !ok {
		t.Fatal("channel missing")
	}
	// The hit bounds now include the overhang; the visible track stays aligned
	// with its parent, inset inside the generous hit bounds.
	for _, end := range []float64{0, 1} {
		value = end
		tt.Frame()
		x := r.X + 8
		if end == 1 {
			x = r.X + r.W - 8
		}
		pixel := tt.Image().RGBAAt(int(x), int(r.Y+r.H/2))
		if pixel.A == 0 {
			t.Fatalf("thumb does not overhang end %v: %v", end, pixel)
		}
		// Start on the outer half, not the track, then drag towards the middle.
		tt.Press(x, r.Y+r.H/2)
		tt.Move(r.X+r.W/2, r.Y+r.H/2)
		tt.Release(r.X+r.W/2, r.Y+r.H/2)
		if value < .49 || value > .51 {
			t.Fatalf("outer half did not capture at end %v: %v", end, value)
		}
	}
	tt.Press(r.X+r.W/2, r.Y+r.H/2)
	tt.Move(r.X-30, r.Y+r.H/2)
	tt.Release(r.X-30, r.Y+r.H/2)
	if value != 0 {
		t.Fatal("captured left endpoint", value)
	}
	tt.Press(r.X+r.W/2, r.Y+r.H/2)
	tt.Move(r.X+r.W+30, r.Y+r.H/2)
	tt.Release(r.X+r.W+30, r.Y+r.H/2)
	if value != 1 {
		t.Fatal("captured right endpoint", value)
	}
}

func TestColorPicker(t *testing.T) {
	value := ui.RGB(100, 50, 200)
	changes := 0
	disabled := false
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Disabled(disabled).Children(func() {
			p := ColorPicker(c, &value)
			p.Label("Tint").Size(80, 40).Border(3, ui.RGB(0, 0, 0)).Radius(15)
			if p.Changed() {
				changes++
			}
		})
	}, 400, 600)
	if _, ok := tt.Find("Hue"); ok {
		t.Fatal("initially open")
	}
	if err := tt.Click("Tint"); err != nil {
		t.Fatal(err)
	}
	if _, ok := tt.Find("Hue"); !ok {
		t.Fatal("not open")
	}
	if changes != 0 {
		t.Fatal("opening reported change")
	}
	if err := tt.Click("Opacity"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyHome)
	if value.A != 0 || changes == 0 {
		t.Fatal("popup change not propagated")
	}
	tt.Key(0, ui.KeyEscape)
	if _, ok := tt.Find("Hue"); ok {
		t.Fatal("escape did not dismiss")
	}
	tt.Key(0, ui.KeySpace)
	if _, ok := tt.Find("Hue"); !ok {
		t.Fatal("keyboard did not reopen")
	}
	disabled = true
	tt.Frame()
	if _, ok := tt.Find("Hue"); ok {
		t.Fatal("disabling did not close")
	}
}
