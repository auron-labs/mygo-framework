package components

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/auron-labs/mygo-framework/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestTextMatchesPrimitive(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, width := range []int{140, 480} {
			t.Run(fmt.Sprintf("%v/%d", dark, width), func(t *testing.T) {
				build := func(text func(*ui.Context, string) ui.Element) *ui.Tester {
					return ui.NewTester(func(c *ui.Context) {
						c.SetTheme(theme.Palette(dark))
						ui.Column(c).Fill().TextColor(c.Theme().TextMuted).FontSize(c.Theme().Rem(1.2)).Children(func() {
							text(c, "Hello, world. 你好，世界。こんにちは 🌍").Label("wrapped")
							text(c, "A long single-line label with inherited styles").SingleLine().Label("single")
							text(c, "Heading").Bold().FontSize(c.Theme().Rem(1.5)).TextColor(c.Theme().Text)
							text(c, "")
							RichText(c).Children(func() {
								text(c, "Inline ")
								text(c, "bold").Bold()
							})
						})
					}, width, 400)
				}
				got, want := build(func(c *ui.Context, value string) ui.Element { return Text(c, value).Element }), build(ui.Text)
				if !bytes.Equal(got.Image().Pix, want.Image().Pix) {
					t.Fatal("Text changed primitive rendering or style inheritance")
				}
				for _, label := range []string{"wrapped", "single"} {
					g, gok := got.Find(label)
					w, wok := want.Find(label)
					if !gok || !wok || g != w {
						t.Fatalf("%s: bounds %v, want %v", label, g, w)
					}
				}
				if !got.HasText("Inline bold") {
					t.Fatal("Text does not compose in RichText")
				}
			})
		}
	}
}

func TestEditableTextAppearance(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, size := range []float32{14, 24} {
			t.Run(fmt.Sprintf("%v/%v", dark, size), func(t *testing.T) {
				value := "Hello 世界"
				build := func(editable bool) *ui.Tester {
					return ui.NewTester(func(c *ui.Context) {
						c.SetTheme(theme.Palette(dark))
						ui.Column(c).Padding(12).AlignItems(ui.Start).Children(func() {
							o := TextOptions{}
							if editable {
								o.Editable = &value
							}
							Text(c, value, o).Label("text").FontSize(size).Bold().TextColor(c.Theme().TextMuted).SingleLine()
						})
					}, 300, 100)
				}
				plain, editable := build(false), build(true)
				if !bytes.Equal(plain.Image().Pix, editable.Image().Pix) {
					t.Fatal("editable text changes resting appearance")
				}
				p, _ := plain.Find("text")
				e, _ := editable.Find("text")
				if p != e {
					t.Fatalf("bounds changed: %v vs %v", p, e)
				}
				editable.Key(0, ui.KeyTab)
				editable.Key(0, ui.KeyRight) // Collapse MyGo's select-all-on-Tab selection.
				img, want := editable.Image(), plain.Image()
				changes := 0
				for y := 0; y < img.Bounds().Dy(); y++ {
					for x := 0; x < img.Bounds().Dx(); x++ {
						i := img.PixOffset(x, y)
						if !bytes.Equal(img.Pix[i:i+4], want.Pix[i:i+4]) {
							changes++
							if float32(x) < p.X+p.W-1 || float32(x) > p.X+p.W+1 {
								t.Fatalf("focus changed pixels outside caret: %d,%d", x, y)
							}
						}
					}
				}
				if changes == 0 {
					t.Fatal("focused text has no caret")
				}
			})
		}
	}
}

func TestTextEditing(t *testing.T) {
	value := "Hello"
	editable := true
	changed, submitted := false, false
	tt := ui.NewTester(func(c *ui.Context) {
		o := TextOptions{}
		if editable {
			o.Editable = &value
		}
		text := Text(c, value, o).Label("Name")
		changed = changed || text.Changed()
		submitted = submitted || text.Submitted()
	}, 300, 100)
	tt.Key(0, ui.KeyTab)
	tt.Key(0, ui.KeyRight)
	tt.Type(" 世界")
	if value != "Hello 世界" || !changed {
		t.Fatalf("edit: %q, changed=%v", value, changed)
	}
	tt.Key(0, ui.KeyEnter)
	if !submitted {
		t.Fatal("Enter did not submit")
	}
	editable = false
	tt.Frame()
	tt.Type("ignored")
	if value != "Hello 世界" {
		t.Fatalf("display dark edited: %q", value)
	}
	value = "External"
	editable = true
	tt.Frame()
	tt.Key(0, ui.KeyTab)
	tt.Key(0, ui.KeyRight)
	tt.Type("!")
	if value != "External!" {
		t.Fatalf("re-enabled binding: %q", value)
	}
}

func TestTextEmptyAndDisabled(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		value := ""
		tt := ui.NewTester(func(c *ui.Context) {
			ui.Column(c).Disabled(disabled).Children(func() {
				Text(c, value, TextOptions{Editable: &value}).Label("Empty").AutoFocus()
			})
		}, 300, 100)
		tt.Type("你好")
		want := "你好"
		if disabled {
			want = ""
		}
		if value != want {
			t.Fatalf("disabled=%v: got %q, want %q", disabled, value, want)
		}
	}
}
