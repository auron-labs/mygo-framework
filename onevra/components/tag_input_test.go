package components

import (
	"slices"
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/v2/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestTagInput(t *testing.T) {
	for _, dark := range []bool{false, true} {
		tags := []string{"design", "motion"}
		changes := 0
		var control *TagInputControl
		var st *tagInputState
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.Palette(dark))
			ui.Column(c).Gap(8).Padding(8).Children(func() {
				control = TagInput(c, &tags).Label("Topics").Placeholder("Add a topic")
				control.Width(300)
				st = *ui.Local(control.Element, "tag-input", func() *tagInputState { return nil })
				if control.Changed() {
					changes++
				}
				Button(c, "Outside", ButtonOutline)
			})
		}, 400, 200)
		state := func() *tagInputState { return st }
		want := func(expected ...string) {
			t.Helper()
			tt.Frame()
			if !slices.Equal(tags, expected) {
				t.Fatalf("tags %q, want %q", tags, expected)
			}
		}
		if err := tt.Click("Topics"); err != nil {
			t.Fatal(err)
		}
		tt.Type("tokens")
		tt.Key(0, ui.KeyEnter)
		want("design", "motion", "tokens")
		if changes != 1 || state().draft != "" {
			t.Fatalf("Enter: changes %d, draft %q", changes, state().draft)
		}

		// A duplicate keeps the draft for correction and pulses the original.
		tt.Type("Design")
		tt.Key(0, ui.KeyEnter)
		want("design", "motion", "tokens")
		if state().draft != "Design" || state().pulses["design"] != 1 {
			t.Fatalf("duplicate: draft %q, pulses %v", state().draft, state().pulses)
		}
		tt.Key(selectionModifier(), ui.KeyA)
		tt.Key(0, ui.KeyBackspace)

		// Commas add each part, as typed or pasted.
		tt.Type("a, b,c")
		want("design", "motion", "tokens", "a", "b")
		if state().draft != "c" {
			t.Fatalf("comma draft %q", state().draft)
		}

		// Backspace edits the draft, then picks the last tag, then removes it.
		tt.Key(0, ui.KeyBackspace)
		tt.Key(0, ui.KeyBackspace)
		want("design", "motion", "tokens", "a", "b")
		if state().picked != "b" {
			t.Fatalf("picked %q", state().picked)
		}
		tt.Key(0, ui.KeyLeft)
		tt.Key(0, ui.KeyLeft)
		tt.Key(0, ui.KeyRight)
		tt.Frame()
		if state().picked != "a" {
			t.Fatalf("arrows picked %q", state().picked)
		}
		// The ring settles on the picked tag.
		tt.Advance(time.Second)
		label, labelFound := tt.Find("a")
		chip, buttonFound := tt.Find("Remove a")
		if !labelFound || !buttonFound {
			t.Fatal("picked tag is missing")
		}
		// Limit the scan to the chip so a text caret on the same row cannot
		// be mistaken for the ring.
		left, right := accentSpan(tt, theme.Palette(dark).Accent, chip.Y+chip.H/2, int(label.X)-12, int(chip.X+chip.W)+8)
		if left < chip.X-40 || left > chip.X-10 || right < chip.X+chip.W || right > chip.X+chip.W+8 {
			t.Fatalf("ring spans %v-%v around the picked tag's button %v", left, right, chip)
		}
		tt.Key(0, ui.KeyDelete)
		want("design", "motion", "tokens", "b")
		tt.Key(0, ui.KeyBackspace)
		tt.Key(0, ui.KeyEscape)
		tt.Key(0, ui.KeyBackspace)
		want("design", "motion", "tokens", "b")
		tt.Key(0, ui.KeyRight)
		tt.Frame()
		if state().picked != "" {
			t.Fatalf("Right past the last tag kept %q", state().picked)
		}

		// Typing clears a pick; leaving the field commits the draft.
		tt.Key(0, ui.KeyBackspace)
		tt.Type("x")
		tt.Frame()
		if state().picked != "" {
			t.Fatal("typing kept the pick")
		}
		if err := tt.Click("Outside"); err != nil {
			t.Fatal(err)
		}
		want("design", "motion", "tokens", "b", "x")

		// Clicking a tag picks it in the field, and clicking again clears it.
		if err := tt.Click("tokens"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if state().picked != "tokens" || !tt.Focused("Topics") {
			t.Fatalf("click picked %q", state().picked)
		}
		if err := tt.Click("tokens"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if state().picked != "" {
			t.Fatalf("second click kept %q", state().picked)
		}

		// The remove button removes its tag and returns focus to the field.
		if err := tt.Click("Remove motion"); err != nil {
			t.Fatal(err)
		}
		want("design", "tokens", "b", "x")
		if !tt.Focused("Topics") {
			t.Fatal("remove did not focus the field")
		}
		announced := tt.Announcements()
		for _, text := range []string{"Added tokens", "design is already added", "b selected. Press Backspace to remove it.", "Removed motion"} {
			if !slices.Contains(announced, text) {
				t.Fatalf("missing announcement %q in %q", text, announced)
			}
		}
	}
}

func TestTagInputHeight(t *testing.T) {
	tags := []string{"one"}
	var box, field ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.Palette(false))
		ui.Column(c).Gap(8).Padding(8).Children(func() {
			draft := ""
			box = TextBox(c, &draft, TextBoxOptions{}).Width(240).LayoutBounds()
			field = TagInput(c, &tags).Label("Tags").Width(240).LayoutBounds()
		})
	}, 300, 300)
	tt.Frame()
	if field.H != box.H {
		t.Fatalf("one row is %v high, TextBox %v", field.H, box.H)
	}
	single := field.H
	tags = []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta"}
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	if field.H <= single {
		t.Fatalf("wrapped field kept height %v", field.H)
	}
}

// accentSpan returns the outermost columns on row y drawn close to accent,
// where the pick ring's border crosses it.
func accentSpan(tt *ui.Tester, accent ui.Color, y float32, start, end int) (left, right float32) {
	img := tt.Image()
	left, right = -1, -1
	r, g, b := accent.R, accent.G, accent.B
	for x := start; x < end; x++ {
		p := img.RGBAAt(x, int(y))
		d := func(a, b uint8) int { return max(int(a)-int(b), int(b)-int(a)) }
		if d(p.R, r)+d(p.G, g)+d(p.B, b) < 60 {
			if left < 0 {
				left = float32(x)
			}
			right = float32(x)
		}
	}
	return left, right
}
