package components

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestColorHSLConversion(t *testing.T) {
	for r := 0; r < 256; r += 17 {
		for g := 0; g < 256; g += 17 {
			for b := 0; b < 256; b += 17 {
				c := ui.Color{R: uint8(r), G: uint8(g), B: uint8(b), A: 128}
				if got := colorToHSV(c).hsl().hsv().color(c.A); got != c {
					t.Fatalf("HSL round trip %v -> %v", c, got)
				}
			}
		}
	}
}

func TestColorHSLFields(t *testing.T) {
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
	click := func(label string) {
		t.Helper()
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	choose := func(mode string) {
		t.Helper()
		click("Color format")
		if err := tt.ChooseMenuItem(mode); err != nil {
			t.Fatal(err)
		}
	}
	typeField := func(label, text string) {
		t.Helper()
		click(label)
		tt.Key(selectionModifier(), ui.KeyA)
		tt.Type(text)
	}
	choose("HSLA")
	if changes != 0 || state.format != colorHSLA || state.fields[2] != "50%" {
		t.Fatal("HSL mode", state.fields)
	}
	typeField("Hue degrees", "120")
	typeField("Saturation percent", "50%")
	typeField("Lightness percent", "50%")
	if value != ui.RGB(64, 191, 64) {
		t.Fatal("HSL editing", value)
	}
	for _, end := range []string{"0", "100"} {
		typeField("Lightness percent", end)
		typeField("Hue degrees", "240")
		typeField("Saturation percent", "75%")
		typeField("Opacity percent", "50%")
		typeField("Lightness percent", "50%")
		if value != (ui.Color{R: 32, G: 32, B: 223, A: 128}) {
			t.Fatal("HSL endpoint lost channels", value)
		}
	}
	count := changes
	choose("RGBA")
	choose("HSBA")
	choose("HSLA")
	if changes != count || state.fields[0] != "240" || state.fields[1] != "75%" || state.fields[2] != "50%" {
		t.Fatal("format change rounded channels", state.fields)
	}
	value = ui.RGB(0, 255, 255)
	tt.Frame()
	if state.fields[0] != "180" || state.fields[1] != "100%" || state.fields[2] != "50%" {
		t.Fatal("external HSL sync", state.fields)
	}
}

func TestColorCopyMenu(t *testing.T) {
	value := ui.Color{R: 255, A: 128}
	disabled := false
	changes := 0
	var clipboard string
	var state *colorViewState
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Disabled(disabled).Children(func() {
			v := ColorView(c, &value)
			state = ui.Local(v.Element, "color-view", func() colorViewState { return colorViewState{} })
			if v.Changed() {
				changes++
			}
		})
		clipboard = c.ReadClipboard()
	}, 360, 550)
	wants := map[string]string{
		"RGBA": "rgba(255, 0, 0, 0.502)",
		"HSBA": "hsba(0, 100%, 100%, 0.502)",
		"HSLA": "hsla(0, 100%, 50%, 0.502)",
		"HEX":  "#FF000080",
	}
	for _, format := range []string{"RGBA", "HSBA", "HSLA", "HEX"} {
		if err := tt.Click("Copy color"); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(tt.Menu(), []string{"RGBA", "HSBA", "HSLA", "HEX"}) {
			t.Fatal(tt.Menu())
		}
		if err := tt.ChooseMenuItem(format); err != nil {
			t.Fatal(err)
		}
		if clipboard != wants[format] {
			t.Fatalf("copy %s: %q", format, clipboard)
		}
		if changes != 0 || state.format != colorRGBA || value != (ui.Color{R: 255, A: 128}) {
			t.Fatal("copy mutated editor")
		}
	}
	value.A = 255
	tt.Frame()
	if err := tt.Click("Copy color"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("HEX"); err != nil {
		t.Fatal(err)
	}
	if clipboard != "#FF0000" {
		t.Fatal("opaque HEX", clipboard)
	}
	// The icon stays at the right edge of the view, independently of label width.
	icon, _ := tt.Find("Copy color")
	hex, _ := tt.Find("Hex color")
	alpha, _ := tt.Find("Opacity percent")
	if icon.X <= hex.X || icon.X+icon.W != alpha.X+alpha.W {
		t.Fatal("copy alignment", icon, hex, alpha)
	}
	disabled = true
	tt.Frame()
	if err := tt.Click("Copy color"); err != nil {
		t.Fatal(err)
	}
	if len(tt.Menu()) != 0 {
		t.Fatal("disabled copy menu opened")
	}
}

func TestColorFieldFiltering(t *testing.T) {
	value := ui.RGB(12, 34, 56)
	var state *colorViewState
	var paste string
	tt := ui.NewTester(func(c *ui.Context) {
		if paste != "" {
			c.WriteClipboard(paste)
			paste = ""
		}
		v := ColorView(c, &value)
		state = ui.Local(v.Element, "color-view", func() colorViewState { return colorViewState{} })
	}, 360, 550)
	replace := func(label, text string, clipboard bool) {
		t.Helper()
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		tt.Key(selectionModifier(), ui.KeyA)
		if clipboard {
			paste = text
			tt.Frame()
			tt.Key(selectionModifier(), ui.KeyV)
		} else {
			tt.Type(text)
		}
		tt.Frame()
	}
	for _, text := range []string{"hello", "1e2", "-2", "12.5", "999", "1%", "１２", "1 2", "NaN", "+2"} {
		before := value
		replace("Red", text, false)
		if value != before || state.fields[0] != "12" {
			t.Fatalf("accepted RGB %q: %v %v", text, value, state.fields)
		}
	}
	replace("Red", "200", true)
	if value.R != 200 {
		t.Fatal("valid RGB paste", value)
	}
	replace("Red", "201oops", true)
	if value.R != 200 || state.fields[0] != "200" {
		t.Fatal("invalid paste accepted")
	}
	replace("Hex color", "#AABBCC80", true)
	if value != (ui.Color{R: 170, G: 187, B: 204, A: 128}) {
		t.Fatal("valid hex paste", value)
	}
	for _, text := range []string{"#GG0000", "123456789", "##112233", "rgb(1,2,3)"} {
		before, draft := value, state.fields[3]
		replace("Hex color", text, true)
		if value != before || state.fields[3] != draft {
			t.Fatal("invalid HEX paste", text, state.fields)
		}
	}
	replace("Hex color", "AB", false)
	if state.fields[3] != "AB" {
		t.Fatal("partial hex rejected")
	}
	tt.Key(0, ui.KeyEnter)
	if state.fields[3] != "AABBCC" {
		t.Fatal("partial hex not restored")
	}
	if err := tt.Click("Color format"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("HSLA"); err != nil {
		t.Fatal(err)
	}
	replace("Lightness percent", "12.5%", true)
	if state.fields[2] != "12.5%" {
		t.Fatal("decimal percent rejected", state.fields)
	}
	for _, text := range []string{"101%", "1.2.3", "-1", "50%%", "Inf"} {
		before := value
		replace("Lightness percent", text, false)
		if value != before || state.fields[2] != "12.5%" {
			t.Fatal("invalid HSL accepted", text, state.fields)
		}
	}
}

func TestColorHexIsPlainText(t *testing.T) {
	value := ui.Color{R: 12, G: 34, B: 56, A: 128}
	tt := ui.NewTester(func(c *ui.Context) { ColorView(c, &value) }, 360, 550)
	for _, label := range []string{"Adjust Hex color", "Hex color Increase", "Hex color Decrease", "Red Increase", "Opacity percent Increase"} {
		if _, ok := tt.Find(label); ok {
			t.Fatalf("unexpected numeric control: %s", label)
		}
	}
	before := value
	if err := tt.Click("Hex color"); err != nil {
		t.Fatal(err)
	}
	for _, key := range []ui.Key{ui.KeyUp, ui.KeyDown, ui.KeyHome, ui.KeyEnd} {
		tt.Key(0, key)
	}
	r, _ := tt.Find("Hex color")
	x, y := r.X+8, r.Y+r.H/2
	tt.Press(x, y)
	tt.Move(x+30, y)
	tt.Release(x+30, y)
	if value != before {
		t.Fatal("hex navigation or prefix dragging adjusted color")
	}
	tt.Key(selectionModifier(), ui.KeyA)
	tt.Type("#AABBCC80")
	if value != (ui.Color{R: 170, G: 187, B: 204, A: 128}) {
		t.Fatal("hex text editing", value)
	}
}

func TestColorFieldScrub(t *testing.T) {
	value := ui.Color{R: 100, G: 50, B: 150, A: 128}
	disabled := false
	changes := 0
	unit := float32(1)
	var state *colorViewState
	tt := ui.NewTester(func(c *ui.Context) {
		unit = c.Theme().Space(.25)
		ui.Column(c).Disabled(disabled).Children(func() {
			v := ColorView(c, &value)
			state = ui.Local(v.Element, "color-view", func() colorViewState { return colorViewState{} })
			if v.Changed() {
				changes++
			}
		})
	}, 360, 550)
	drag := func(label string, delta float32) {
		t.Helper()
		r, ok := tt.Find("Adjust " + label)
		if !ok {
			t.Fatal(label)
		}
		x, y := r.X+r.W/2, r.Y+r.H/2
		tt.Press(x, y)
		tt.Move(x+delta*unit, y)
		tt.Release(x+delta*unit, y)
	}
	drag("Red", 20)
	if value.R != 120 || value.G != 50 || value.B != 150 || value.A != 128 || changes == 0 {
		t.Fatal("RGB scrub", value, changes)
	}
	drag("Red", 1000)
	if value.R != 255 {
		t.Fatal("scrub upper clamp", value)
	}
	drag("Red", -1000)
	if value.R != 0 {
		t.Fatal("scrub lower clamp", value)
	}
	tt.Key(0, ui.KeyRight)
	if value.R != 1 {
		t.Fatal("prefix keyboard", value)
	}
	before := value
	if err := tt.Click("Red"); err != nil {
		t.Fatal(err)
	}
	r, _ := tt.Find("Red")
	tt.Press(r.X+r.W-12, r.Y+r.H/2)
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Release(r.X+r.W/2, r.Y+r.H/2)
	if value != before {
		t.Fatal("text selection scrubbed color")
	}
	drag("Opacity percent", 10)
	if value.A != 154 {
		t.Fatal("alpha scrub", value)
	}
	if err := tt.Click("Color format"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("HSLA"); err != nil {
		t.Fatal(err)
	}
	old := state.hsl
	drag("Hue degrees", 10)
	if state.hsl.h != float64(int((old.h+10)*10+.5))/10 {
		t.Fatal("HSL hue scrub", old, state.hsl)
	}
	drag("Lightness percent", 1000)
	if state.hsl.l != 1 {
		t.Fatal("HSL upper clamp")
	}
	drag("Lightness percent", -50)
	if state.hsl.l != .5 {
		t.Fatal("HSL scrub from endpoint")
	}
	disabled = true
	tt.Frame()
	before = value
	drag("Hue degrees", 20)
	tt.Key(0, ui.KeyEnd)
	if value != before {
		t.Fatal("disabled scrub changed color")
	}
}
