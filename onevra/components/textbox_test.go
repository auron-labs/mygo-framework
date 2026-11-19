package components

import (
	"bytes"
	"image/color"
	"testing"

	"github.com/auron-labs/mygo-framework/onevra/icons/lucide"
	"github.com/auron-labs/mygo-framework/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestTextBoxLeadingIcon(t *testing.T) {
	for _, mode := range []bool{false, true} {
		for _, multiline := range []bool{false, true} {
			value := ""
			options := TextBoxOptions{Multiline: multiline}
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(theme.Palette(mode))
				TextBox(c, &value, options).Label("Search").Width(240).Height(100)
			}, 300, 120)
			plain := append([]byte(nil), tt.Image().Pix...)
			options.LeadingIcon = lucide.Search()
			tt.Frame()
			img := tt.Image()
			changed := 0
			for y := 0; y < 120; y++ {
				for x := 0; x < 300; x++ {
					i := img.PixOffset(x, y)
					if !bytes.Equal(plain[i:i+4], img.Pix[i:i+4]) {
						changed++
						if x < 10 || x >= 26 || (multiline && y >= 24) || (!multiline && (y < 42 || y >= 58)) {
							t.Fatalf("icon outside leading inset: %d,%d", x, y)
						}
					}
				}
			}
			if changed == 0 {
				t.Fatal("leading icon not rendered")
			}
			options.LeadingIcon = nil
			tt.Frame()
			if !bytes.Equal(plain, tt.Image().Pix) {
				t.Fatal("nil icon did not restore plain input")
			}
			options.LeadingIcon = lucide.Search()
			tt.Frame()
			tt.ClickAt(18, 15)
			tt.Type("query")
			if value != "query" {
				t.Fatalf("leading inset did not focus input: %q", value)
			}
		}
	}
}

func TestTextBoxLeadingText(t *testing.T) {
	for _, mode := range []bool{false, true} {
		for _, multiline := range []bool{false, true} {
			for _, buffered := range []bool{false, true} {
				value := ""
				buffer := ui.NewTextBuffer("")
				options := TextBoxOptions{Multiline: multiline, LeadingIcon: lucide.Search()}
				var input ui.Element
				tt := ui.NewTester(func(c *ui.Context) {
					c.SetTheme(theme.Palette(mode))
					if buffered {
						input = TextBoxBuffer(c, buffer, options)
					} else {
						input = TextBox(c, &value, options)
					}
					input.Label("Amount").Width(240).Height(100)
				}, 300, 120)
				plain := append([]byte(nil), tt.Image().Pix...)
				options.LeadingText = "USD"
				tt.Frame()
				img := tt.Image()
				changed := 0
				for y := 0; y < 120; y++ {
					for x := 0; x < 300; x++ {
						i := img.PixOffset(x, y)
						if !bytes.Equal(plain[i:i+4], img.Pix[i:i+4]) {
							changed++
							if x < 32 || x > 70 {
								t.Fatalf("prefix displaced icon or escaped inset: %d,%d", x, y)
							}
						}
					}
				}
				if changed == 0 {
					t.Fatal("prefix missing")
				}
				// The prefix remains in the input's hit region but outside its buffer.
				tt.ClickAt(40, 15)
				tt.Type("123")
				got := value
				if buffered {
					got = buffer.String()
				}
				if got != "123" {
					t.Fatalf("prefix became editable: %q", got)
				}
				options.ReadOnly = true
				tt.Frame()
				tt.Type("4")
				got = value
				if buffered {
					got = buffer.String()
				}
				if got != "123" {
					t.Fatal("read-only prefix input changed")
				}
				options.LeadingText = ""
				tt.Frame()
				options.ReadOnly = false
				tt.Frame()
				tt.Key(selectionModifier(), ui.KeyA)
				tt.Type("9")
				got = value
				if buffered {
					got = buffer.String()
				}
				if got != "9" {
					t.Fatal("removing prefix broke editing")
				}
			}
		}
	}
}

func TestTextBoxType(t *testing.T) {
	for _, dark := range []bool{false, true} {
		mode := false
		if dark {
			mode = true
		}
		value := ""
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.Palette(mode))
			TextBox(c, &value, TextBoxOptions{}).Label("Name").Placeholder("Ada").AutoFocus()
		}, 300, 100)
		tt.Type("Ada")
		if value != "Ada" {
			t.Errorf("dark=%v: typed %q", dark, value)
		}
	}
}

func TestTextBoxReadOnly(t *testing.T) {
	for _, mode := range []bool{false, true} {
		for _, multiline := range []bool{false, true} {
			value := "Selectable text"
			readOnly := true
			var focused, selectText bool
			var start, end int
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(theme.Palette(mode))
				input := TextBox(c, &value, TextBoxOptions{Multiline: multiline, ReadOnly: readOnly}).
					Label("Input").Width(240).Height(100)
				focused = input.Focused()
				if selectText {
					input.SetTextSelection(0, 10)
					start, end = input.TextSelection()
				}
			}, 300, 120)
			if err := tt.Click("Input"); err != nil {
				t.Fatal(err)
			}
			if !focused {
				t.Fatal("read-only input cannot be focused")
			}
			tt.Type("Changed")
			tt.Key(0, ui.KeyEnter)
			tt.Key(0, ui.KeyBackspace)
			if value != "Selectable text" {
				t.Fatalf("read-only input changed: %q", value)
			}
			selectText = true
			tt.Frame()
			if start != 0 || end != 10 {
				t.Fatalf("read-only selection: %d:%d", start, end)
			}
			selectText = false
			readOnly = false
			tt.Frame() // Rebuild with editable options before the next event.
			tt.Type("Updated")
			if value == "Selectable text" {
				t.Fatal("input did not become editable")
			}
		}
	}
}

func TestTextBoxReadOnlyAppearance(t *testing.T) {
	for _, mode := range []bool{false, true} {
		for _, multiline := range []bool{false, true} {
			var backgrounds, borders [2]color.RGBA
			var neutral ui.Color
			for i, readOnly := range []bool{false, true} {
				value := ""
				tt := ui.NewTester(func(c *ui.Context) {
					c.SetTheme(theme.Palette(mode))
					neutral = c.Theme().Border.Mix(c.Theme().Text, 0.25)
					TextBox(c, &value, TextBoxOptions{Multiline: multiline, ReadOnly: readOnly}).
						Label("Input").Width(240).Height(100)
				}, 300, 120)
				if err := tt.Click("Input"); err != nil {
					t.Fatal(err)
				}
				img := tt.Image()
				backgrounds[i], borders[i] = img.RGBAAt(120, 50), img.RGBAAt(120, 0)
			}
			if backgrounds[0] == backgrounds[1] || borders[0] == borders[1] {
				t.Fatalf("mode=%v multiline=%v: read-only appearance is not distinct", mode, multiline)
			}
			b := borders[1]
			if b != (color.RGBA{R: neutral.R, G: neutral.G, B: neutral.B, A: neutral.A}) {
				t.Fatalf("read-only focus border is not neutral: %v", b)
			}
		}
	}
}

func TestTextBoxModeSwitch(t *testing.T) {
	value := ""
	multiline := false
	tt := ui.NewTester(func(c *ui.Context) {
		TextBox(c, &value, TextBoxOptions{Multiline: multiline}).AutoFocus()
	}, 300, 120)
	tt.Type("one")
	tt.Key(0, ui.KeyEnter)
	if value != "one" {
		t.Fatalf("single-line accepted newline: %q", value)
	}
	multiline = true
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Type("two")
	if value != "one\ntwo" {
		t.Fatalf("switch to multiline lost content or newline: %q", value)
	}
	multiline = false
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	if value != "one\ntwo" {
		t.Fatalf("switch to single-line accepted newline: %q", value)
	}
}

func TestTextBoxMultilineType(t *testing.T) {
	value := ""
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.DarkPalette())
		TextBox(c, &value, TextBoxOptions{Multiline: true}).Label("Notes").AutoFocus()
	}, 300, 120)
	tt.Type("line one")
	tt.Key(0, ui.KeyEnter)
	tt.Type("line two")
	if value != "line one\nline two" {
		t.Errorf("typed %q", value)
	}
}
