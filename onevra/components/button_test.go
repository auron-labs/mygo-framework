package components

import (
	"bytes"
	"slices"
	"testing"

	"github.com/auron-labs/mygo-framework/v2/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestButtonVariantsClick(t *testing.T) {
	variants := []ButtonVariant{ButtonPrimary, ButtonSecondary, ButtonOutline, ButtonGhost, ButtonLink, ButtonToolbar}
	for _, dark := range []bool{false, true} {
		for _, variant := range variants {
			clicked := false
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(theme.Palette(dark))
				if Button(c, "OK", variant).Clicked() {
					clicked = true
				}
			}, 300, 100)
			if err := tt.Click("OK"); err != nil {
				t.Fatal(err)
			}
			if !clicked {
				t.Errorf("dark=%v variant=%d: click not reported", dark, variant)
			}
		}
	}
}

func TestButtonToolbarFace(t *testing.T) {
	for _, dark := range []bool{false, true} {
		face, hover, _, _ := toolbarFace(theme.Palette(dark))
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.Palette(dark))
			Button(c, "Copy", ButtonToolbar)
		}, 300, 100)
		r, ok := tt.Find("Copy")
		if !ok {
			t.Fatal("button not found")
		}
		// Inside the horizontal padding, clear of the label glyphs and border.
		sample := func() ui.Color {
			p := tt.Image().RGBAAt(int(r.X+4), int(r.Y+r.H/2))
			return ui.Color{R: p.R, G: p.G, B: p.B, A: p.A}
		}
		if got := sample(); got.R != face.R || got.G != face.G || got.B != face.B {
			t.Fatalf("dark=%v: face %v, want %v", dark, got, face)
		}
		tt.Move(r.X+r.W/2, r.Y+r.H/2)
		if got := sample(); got.R != hover.R || got.G != hover.G || got.B != hover.B {
			t.Fatalf("dark=%v: hover face %v, want %v", dark, got, hover)
		}
	}
}

func TestButtonCursorOverridesParent(t *testing.T) {
	for _, variant := range []ButtonVariant{ButtonPrimary, ButtonSecondary, ButtonOutline, ButtonGhost, ButtonLink, ButtonToolbar} {
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.LightPalette())
			ui.Row(c).Fill().Cursor(ui.CursorText).Children(func() {
				Button(c, "Default", variant)
				Button(c, "Explicit", variant).Cursor(ui.CursorPointer)
			})
		}, 300, 100)
		defaultCursor := ui.CursorDefault
		if variant == ButtonLink {
			defaultCursor = ui.CursorPointer
		}
		for _, want := range []struct {
			label  string
			cursor ui.Cursor
		}{{"Default", defaultCursor}, {"Explicit", ui.CursorPointer}} {
			r, ok := tt.Find(want.label)
			if !ok {
				t.Fatalf("missing %s button", want.label)
			}
			tt.Move(r.X+r.W/2, r.Y+r.H/2)
			if got := tt.Cursor(); got != want.cursor {
				t.Errorf("variant=%v %s: cursor=%v, want %v", variant, want.label, got, want.cursor)
			}
		}
		tt.Move(290, 90)
		if tt.Cursor() != ui.CursorText {
			t.Fatal("parent cursor lost outside buttons")
		}
	}
}

func TestButtonKeyboard(t *testing.T) {
	clicked := false
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightPalette())
		if Button(c, "Save", ButtonPrimary).Clicked() {
			clicked = true
		}
	}, 300, 100)
	tt.Key(0, ui.KeyTab)
	tt.Key(0, ui.KeySpace)
	if !clicked {
		t.Error("Space did not click the focused button")
	}
}

func TestButtonDisabledDims(t *testing.T) {
	sample := func(disabled bool) (r, g, b uint8) {
		var rect ui.Rect
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.LightPalette())
			ui.Column(c).Disabled(disabled).Fill().Children(func() {
				Button(c, "OK", ButtonPrimary)
			})
		}, 300, 100)
		var ok bool
		rect, ok = tt.Find("OK")
		if !ok {
			t.Fatal("button not found")
		}
		// Inside the left padding, clear of the label glyphs.
		p := tt.Image().RGBAAt(int(rect.X+6), int(rect.Y+rect.H/2))
		return p.R, p.G, p.B
	}
	er, eg, eb := sample(false)
	dr, dg, db := sample(true)
	if er == dr && eg == dg && eb == db {
		t.Fatal("disabled button renders identically to enabled")
	}
	// Dimming blends the accent toward the light background.
	if dr <= er || dg <= eg || db <= eb {
		t.Errorf("disabled pixel (%d,%d,%d) not lighter than enabled (%d,%d,%d)", dr, dg, db, er, eg, eb)
	}
}

func TestButtonLinkInline(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, disabled := range []bool{false, true} {
			clicks := 0
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(theme.Palette(dark))
				ui.Column(c).Fill().AlignItems(ui.Start).Disabled(disabled).Children(func() {
					ui.Text(c, "Details").Label("plain")
					RichText(c).Children(func() {
						ui.Text(c, "Read ")
						if Button(c, "Details", ButtonLink).Clicked() {
							clicks++
						}
						ui.Text(c, " for more.")
					})
				})
			}, 300, 120)
			link, ok := tt.Find("Details")
			if !ok {
				t.Fatal("missing inline link")
			}
			plain, ok := tt.Find("plain")
			if !ok || link.H != plain.H || link.W != plain.W {
				t.Fatalf("link %+v must match plain text %+v", link, plain)
			}
			if !tt.HasText("Read Details for more.") || link.X <= 0 {
				t.Fatal("link is not composed inline")
			}
			tt.Move(290, 110)
			rest := slices.Clone(tt.Image().Pix)
			tt.Move(link.X+link.W/2, link.Y+link.H/2)
			hover := slices.Clone(tt.Image().Pix)
			if bytes.Equal(rest, hover) != disabled {
				t.Fatalf("disabled=%v: incorrect underline feedback", disabled)
			}
			tt.Move(290, 110)
			if !bytes.Equal(rest, tt.Image().Pix) {
				t.Fatal("underline remains after pointer exit")
			}
			if err := tt.Click("Details"); err != nil {
				t.Fatal(err)
			}
			want := 1
			if disabled {
				want = 0
			}
			if clicks != want || len(tt.OpenedURLs()) != 0 {
				t.Fatal("link must report clicks without opening a URL")
			}
		}
	}
}

func TestButtonSecondaryFill(t *testing.T) {
	for _, dark := range []bool{false, true} {
		var want ui.Color
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.Palette(dark))
			want = c.Theme().Background.Mix(c.Theme().Text, .08)
			Button(c, "Secondary", ButtonSecondary)
		}, 300, 100)
		r, _ := tt.Find("Secondary")
		tt.Move(290, 90)
		pixel := tt.Image().RGBAAt(int(r.X+6), int(r.Y+r.H/2))
		if pixel.R != want.R || pixel.G != want.G || pixel.B != want.B {
			t.Fatalf("dark=%v: default fill = %v, want %v", dark, pixel, want)
		}
		border := theme.LightPalette().Border
		if dark {
			border = theme.DarkPalette().Border
		}
		edge := tt.Image().RGBAAt(int(r.X+r.W/2), int(r.Y))
		if edge.R != border.R || edge.G != border.G || edge.B != border.B {
			t.Fatalf("dark=%v: secondary border = %v, want %v", dark, edge, border)
		}
		tt.Move(r.X+r.W/2, r.Y+r.H/2)
		if !dark {
			want = theme.LightPalette().Background.Mix(theme.LightPalette().Text, .12)
		} else {
			want = theme.DarkPalette().Background.Mix(theme.DarkPalette().Text, .12)
		}
		pixel = tt.Image().RGBAAt(int(r.X+6), int(r.Y+r.H/2))
		if pixel.R != want.R || pixel.G != want.G || pixel.B != want.B {
			t.Fatalf("dark=%v: hovered default fill = %v, want %v", dark, pixel, want)
		}
	}
}

func TestButtonPrimaryBorderAndHeight(t *testing.T) {
	for _, dark := range []bool{false, true} {
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.Palette(dark))
			ui.Row(c).Gap(12).Children(func() {
				Button(c, "Primary", ButtonPrimary)
				Button(c, "Secondary", ButtonSecondary)
				Button(c, "Outline", ButtonOutline)
				Button(c, "Ghost", ButtonGhost)
			})
		}, 400, 100)
		primary, ok := tt.Find("Primary")
		if !ok {
			t.Fatal("primary button not found")
		}
		for _, label := range []string{"Secondary", "Outline", "Ghost"} {
			r, ok := tt.Find(label)
			if !ok || r.H != primary.H {
				t.Fatalf("dark=%v: %s height = %v, primary = %v", dark, label, r.H, primary.H)
			}
		}
		palette := theme.LightPalette()
		if dark {
			palette = theme.DarkPalette()
		}
		x, y := primary.X+primary.W/2, primary.Y+primary.H/2
		check := func(want ui.Color) {
			t.Helper()
			edge := tt.Image().RGBAAt(int(x), int(primary.Y))
			if edge.R != want.R || edge.G != want.G || edge.B != want.B {
				t.Fatalf("dark=%v: primary border = %v, want %v", dark, edge, want)
			}
		}
		tt.Move(390, 90)
		check(palette.Accent)
		tt.Move(x, y)
		check(palette.AccentHover)
		tt.Press(x, y)
		check(palette.AccentPressed)
		tt.Release(x, y)
	}
}

func TestButtonGhostTransparentBorder(t *testing.T) {
	for _, dark := range []bool{false, true} {
		var palette *ui.Theme
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.Palette(dark))
			palette = c.Theme()
			Button(c, "Ghost", ButtonGhost)
		}, 200, 100)
		r, ok := tt.Find("Ghost")
		if !ok {
			t.Fatal("ghost button not found")
		}
		x, y := r.X+r.W/2, r.Y+r.H/2
		check := func(want ui.Color) {
			t.Helper()
			edge := tt.Image().RGBAAt(int(x), int(r.Y))
			if edge.R != want.R || edge.G != want.G || edge.B != want.B {
				t.Fatalf("dark=%v: ghost edge = %v, want %v", dark, edge, want)
			}
		}
		tt.Move(190, 90)
		check(palette.Background)
		tt.Move(x, y)
		check(palette.SurfaceHover)
		tt.Press(x, y)
		check(palette.SurfacePressed)
		tt.Release(x, y)
	}
}

func TestButtonLabel(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightPalette())
		Button(c, "Delete", ButtonSecondary)
	}, 300, 100)
	if !tt.HasText("Delete") {
		t.Error("button label not shown")
	}
}
