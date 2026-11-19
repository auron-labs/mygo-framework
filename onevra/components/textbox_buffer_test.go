package components

import (
	"bytes"
	"testing"

	"github.com/auron-labs/mygo-framework/v2/onevra/icons/lucide"
	"github.com/auron-labs/mygo-framework/v2/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestTextBoxBufferEditing(t *testing.T) {
	for _, dark := range []bool{false, true} {
		buffer := ui.NewTextBuffer("")
		options := TextBoxOptions{LeadingIcon: lucide.Search()}
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.Palette(dark))
			TextBoxBuffer(c, buffer, options).Label("Buffer").Width(240).Height(100)
		}, 300, 120)
		tt.ClickAt(18, 15)
		tt.Type("Hello 世界")
		tt.Key(0, ui.KeyEnter)
		if got := buffer.String(); got != "Hello 世界" {
			t.Fatalf("single-line editing: %q", got)
		}
		options.Multiline = true
		tt.Frame()
		tt.Key(0, ui.KeyEnter)
		tt.Type("Second line")
		if got := buffer.String(); got != "Hello 世界\nSecond line" {
			t.Fatalf("multiline editing: %q", got)
		}
		options.ReadOnly = true
		tt.Frame()
		tt.Type("ignored")
		tt.Key(0, ui.KeyBackspace)
		if got := buffer.String(); got != "Hello 世界\nSecond line" {
			t.Fatalf("read-only editing: %q", got)
		}
		options.ReadOnly = false
		options.Multiline = false
		tt.Frame()
		tt.Key(0, ui.KeyEnter)
		tt.Type("!")
		if got := buffer.String(); got != "Hello 世界\nSecond line!" {
			t.Fatalf("restored single-line editing: %q", got)
		}
	}
}

func TestTextBoxBufferAppearance(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, multiline := range []bool{false, true} {
			for _, readOnly := range []bool{false, true} {
				for _, icon := range []*ui.SVG{nil, lucide.Search()} {
					value := "Hello 世界"
					buffer := ui.NewTextBuffer(value)
					options := TextBoxOptions{Multiline: multiline, ReadOnly: readOnly, LeadingIcon: icon}
					makeTester := func(buffered bool) *ui.Tester {
						return ui.NewTester(func(c *ui.Context) {
							c.SetTheme(theme.Palette(dark))
							var e ui.Element
							if buffered {
								e = TextBoxBuffer(c, buffer, options)
							} else {
								e = TextBox(c, &value, options)
							}
							e.Label("Input").Width(240).Height(100)
						}, 300, 120)
					}
					plain, buffered := makeTester(false), makeTester(true)
					if !bytes.Equal(plain.Image().Pix, buffered.Image().Pix) {
						t.Fatal("buffer appearance differs from TextBox")
					}
					if err := plain.Click("Input"); err != nil {
						t.Fatal(err)
					}
					if err := buffered.Click("Input"); err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(plain.Image().Pix, buffered.Image().Pix) {
						t.Fatal("focused buffer appearance differs from TextBox")
					}
				}
			}
		}
	}
}

func TestTextBoxBufferExternalEditsAndSelection(t *testing.T) {
	for _, multiline := range []bool{false, true} {
		buffer := ui.NewTextBuffer("Initial")
		var focused bool
		selectText := false
		var start, end int
		tt := ui.NewTester(func(c *ui.Context) {
			input := TextBoxBuffer(c, buffer, TextBoxOptions{Multiline: multiline, ReadOnly: true}).Label("Buffer")
			focused = input.Focused()
			if selectText {
				input.SetTextSelection(0, 2)
				start, end = input.TextSelection()
			}
		}, 300, 120)
		before := append([]byte(nil), tt.Image().Pix...)
		buffer.Set("你好 world")
		tt.Frame()
		if bytes.Equal(before, tt.Image().Pix) {
			t.Fatal("external update not displayed")
		}
		if err := tt.Click("Buffer"); err != nil {
			t.Fatal(err)
		}
		selectText = true
		tt.Frame()
		if !focused || start != 0 || end != 2 {
			t.Fatalf("read-only focus/selection: %d:%d", start, end)
		}
		tt.Type("ignored")
		if got := buffer.String(); got != "你好 world" {
			t.Fatalf("read-only selection changed buffer: %q", got)
		}
	}
}
