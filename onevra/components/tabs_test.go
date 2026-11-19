package components

import (
	"bytes"
	"slices"
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/v2/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestTabsClick(t *testing.T) {
	for _, variant := range []TabsVariant{TabsUnderline, TabsSegmented, TabsHighlight} {
		for _, dark := range []bool{false, true} {
			selected := 0
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(theme.Palette(dark))
				Tabs(c, &selected, variant, "General", "Appearance", "Advanced")
			}, 400, 100)
			if err := tt.Click("Appearance"); err != nil {
				t.Fatal(err)
			}
			if selected != 1 {
				t.Errorf("variant=%v dark=%v: selected=%d, want 1", variant, dark, selected)
			}
			if err := tt.Click("Advanced"); err != nil {
				t.Fatal(err)
			}
			if selected != 2 {
				t.Errorf("variant=%v dark=%v: selected=%d, want 2", variant, dark, selected)
			}
		}
	}
}

// Each tab's leading slot precedes its label and leaves the tab clickable.
func TestTabsWithLeading(t *testing.T) {
	for _, variant := range []TabsVariant{TabsUnderline, TabsSegmented, TabsHighlight} {
		selected := 0
		marks := []string{"[g]", "[a]"}
		tt := ui.NewTester(func(c *ui.Context) {
			TabsWithLeading(c, &selected, variant, func(c *ui.Context, i int, _ float32) { Text(c, marks[i]) }, "General", "Appearance")
		}, 400, 100)
		for i, label := range []string{"General", "Appearance"} {
			mark, ok := tt.Find(marks[i])
			text, _ := tt.Find(label)
			if !ok || mark.X+mark.W > text.X {
				t.Fatalf("variant=%v: %s at %v, label at %v", variant, marks[i], mark, text)
			}
		}
		if err := tt.Click("[a]"); err != nil {
			t.Fatal(err)
		}
		if selected != 1 {
			t.Errorf("variant=%v: selected=%d, want 1", variant, selected)
		}
	}
}

// Each tab's trailing slot follows its label and leaves the tab clickable.
func TestTabsTrailing(t *testing.T) {
	for _, variant := range []TabsVariant{TabsUnderline, TabsSegmented, TabsHighlight} {
		selected := 0
		counts := []string{"[3]", "[5]"}
		tt := ui.NewTester(func(c *ui.Context) {
			TabsWith(c, &selected, TabsOptions{Variant: variant, Trailing: func(c *ui.Context, i int, _ float32) { Text(c, counts[i]) }}, "General", "Appearance")
		}, 400, 100)
		for i, label := range []string{"General", "Appearance"} {
			count, ok := tt.Find(counts[i])
			text, _ := tt.Find(label)
			if !ok || count.X < text.X+text.W {
				t.Fatalf("variant=%v: %s at %v, label at %v", variant, counts[i], count, text)
			}
		}
		if err := tt.Click("[5]"); err != nil {
			t.Fatal(err)
		}
		if selected != 1 {
			t.Errorf("variant=%v: selected=%d, want 1", variant, selected)
		}
	}
}

// A tab's context menu opens on a right-click without choosing it, and a tab
// whose builder is nil has none.
func TestTabsMenu(t *testing.T) {
	for _, variant := range []TabsVariant{TabsUnderline, TabsSegmented, TabsHighlight} {
		selected := 0
		opened := -1
		tt := ui.NewTester(func(c *ui.Context) {
			TabsWith(c, &selected, TabsOptions{Variant: variant, Menu: func(i int) func(m *ui.Menu) {
				if i == 0 {
					return nil
				}
				return func(m *ui.Menu) {
					if m.Item("Open").Chosen() {
						opened = i
					}
				}
			}}, "General", "Appearance", "Advanced")
		}, 400, 100)
		if err := tt.RightClick("Advanced"); err != nil {
			t.Fatal(err)
		}
		if menu := tt.Menu(); !slices.Equal(menu, []string{"Open"}) {
			t.Fatalf("variant=%v: menu %v", variant, menu)
		}
		if err := tt.ChooseMenuItem("Open"); err != nil {
			t.Fatal(err)
		}
		if opened != 2 || selected != 0 {
			t.Fatalf("variant=%v: opened %d, selected %d", variant, opened, selected)
		}
		if err := tt.RightClick("General"); err != nil {
			t.Fatal(err)
		}
		if menu := tt.Menu(); len(menu) != 0 {
			t.Fatalf("variant=%v: General's menu %v", variant, menu)
		}
	}
}

func TestTabsIndicatorAnimation(t *testing.T) {
	for _, mode := range []bool{false, true} {
		selected := 0
		var list ui.Rect
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.Palette(mode))
			parts := Tabs(c, &selected, TabsUnderline, "General", "Appearance", "Advanced")
			parts.List.Draw(func(_ *ui.Painter, r ui.Rect) { list = r })
		}, 400, 100)
		underline := func() (int, int) {
			t.Helper()
			img := tt.Image()
			y := int(list.Y + list.H - 1)
			bg := img.RGBAAt(399, y)
			left, right := -1, -1
			for x := 0; x < 400; x++ {
				if img.RGBAAt(x, y) != bg {
					if left < 0 {
						left = x
					}
					right = x
				}
			}
			if left < 0 {
				t.Fatal("missing underline")
			}
			return left, right
		}
		start, _ := underline()
		before, _ := tt.Find("Appearance")
		selected = 1
		tt.Frame()
		if x, _ := underline(); x < start || x >= int(before.X) {
			t.Fatal("underline jumped on selection")
		}
		tt.Advance(60 * time.Millisecond)
		tt.Frame()
		middle, _ := underline()
		if middle <= start || middle >= int(before.X) {
			t.Fatalf("not moving between tabs: %d", middle)
		}
		tt.Advance(tabsIndicatorDuration)
		tt.Frame()
		end, _ := underline()
		if end <= middle {
			t.Fatal("underline did not reach selected tab")
		}
		after, _ := tt.Find("Appearance")
		if before != after {
			t.Fatalf("selection changed text geometry: %v -> %v", before, after)
		}
		selected = 0
		tt.Frame()
		tt.Advance(40 * time.Millisecond)
		tt.Frame()
		selected = 1
		tt.Frame()
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		if x, _ := underline(); x != end {
			t.Fatal("reduced motion did not finish transition")
		}
		selected = 0
		tt.Frame()
		if x, _ := underline(); x != start {
			t.Fatal("reduced motion animated selection")
		}
	}
}

// TestTabsSegmentedThumb checks that the chosen segment is drawn on the theme's
// raised surface and that its label still paints over the thumb.
func TestTabsSegmentedThumb(t *testing.T) {
	for _, dark := range []bool{false, true} {
		selected := 0
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.Palette(dark))
			Tabs(c, &selected, TabsSegmented, "General", "Appearance", "Advanced")
		}, 400, 100)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		pal := theme.Palette(dark)
		want := pal.Surface
		if dark {
			want = pal.SurfaceHover
		}
		above := func(label string) ui.Color {
			t.Helper()
			r, ok := tt.Find(label)
			if !ok {
				t.Fatalf("missing %s", label)
			}
			p := tt.Image().RGBAAt(int(r.X+r.W/2), int(r.Y-2))
			return ui.Color{R: p.R, G: p.G, B: p.B, A: p.A}
		}
		thumb := above("General")
		if thumb.R != want.R || thumb.G != want.G || thumb.B != want.B {
			t.Fatalf("dark=%v: chosen segment %v, want %v", dark, thumb, want)
		}
		// The label paints over the thumb, so the text box is not one flat color.
		r, _ := tt.Find("General")
		img := tt.Image()
		flat := img.RGBAAt(int(r.X+r.W/2), int(r.Y-2))
		text := false
		for y := int(r.Y); y < int(r.Y+r.H) && !text; y++ {
			for x := int(r.X); x < int(r.X+r.W); x++ {
				if c := img.RGBAAt(x, y); c != flat {
					text = true
					break
				}
			}
		}
		if !text {
			t.Fatal("chosen label is hidden behind the thumb")
		}
		// The thumb follows the choice, and the other segments show the track.
		selected = 2
		tt.Frame()
		if moved := above("Advanced"); moved.R != want.R || moved.G != want.G || moved.B != want.B {
			t.Fatalf("dark=%v: thumb did not follow selection: %v", dark, moved)
		}
		if track := above("General"); track.R == want.R && track.G == want.G && track.B == want.B {
			t.Fatal("old segment kept the thumb")
		}
	}
}

// TestTabsSegmentedThumbAnimation checks that the thumb moves to the chosen
// segment instead of jumping there, and finishes when the frame settles.
func TestTabsSegmentedThumbAnimation(t *testing.T) {
	selected := 0
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightPalette())
		Tabs(c, &selected, TabsSegmented, "General", "Appearance", "Advanced")
	}, 400, 100)
	tt.Frame()
	thumb := theme.LightPalette().Surface
	isThumb := func(label string) bool {
		t.Helper()
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("missing %s", label)
		}
		p := tt.Image().RGBAAt(int(r.X+r.W/2), int(r.Y-2))
		return p.R == thumb.R && p.G == thumb.G && p.B == thumb.B
	}
	if !isThumb("General") {
		t.Fatal("thumb not on the chosen tab")
	}
	selected = 2
	tt.Frame()
	if isThumb("Advanced") {
		t.Fatal("thumb jumped to the choice instead of animating")
	}
	tt.Advance(tabsIndicatorDuration + 20*time.Millisecond)
	tt.Frame()
	if !isThumb("Advanced") {
		t.Fatal("thumb did not reach the chosen tab")
	}
	if isThumb("General") {
		t.Fatal("thumb remained on the old tab")
	}
}

// TestTabsSegmentedHoverDoesNotChangeAppearance keeps the segmented look free of
// hover faces, as the underline variant is.
func TestTabsSegmentedHoverDoesNotChangeAppearance(t *testing.T) {
	for _, mode := range []bool{false, true} {
		selected := 0
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.Palette(mode))
			Tabs(c, &selected, TabsSegmented, "General", "Appearance")
		}, 400, 100)
		before := bytes.Clone(tt.Image().Pix)
		for _, label := range []string{"General", "Appearance"} {
			r, ok := tt.Find(label)
			if !ok {
				t.Fatalf("missing %s", label)
			}
			tt.Move(r.X+r.W/2, r.Y+r.H/2)
			if !bytes.Equal(before, tt.Image().Pix) {
				t.Fatalf("hover changed %s", label)
			}
		}
	}
}

func TestTabsEmpty(t *testing.T) {
	for _, variant := range []TabsVariant{TabsUnderline, TabsSegmented, TabsHighlight} {
		selected := 0
		tt := ui.NewTester(func(c *ui.Context) { Tabs(c, &selected, variant) }, 400, 100)
		tt.Frame()
	}
}

func TestTabsKeyboard(t *testing.T) {
	for _, variant := range []TabsVariant{TabsUnderline, TabsSegmented, TabsHighlight} {
		selected := 0
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.LightPalette())
			Tabs(c, &selected, variant, "General", "Appearance", "Advanced")
		}, 400, 100)
		tt.Key(0, ui.KeyTab)
		tt.Key(0, ui.KeyRight)
		if selected != 1 {
			t.Errorf("variant=%v: Right chose %d, want 1", variant, selected)
		}
		tt.Key(0, ui.KeyLeft)
		if selected != 0 {
			t.Errorf("variant=%v: Left chose %d, want 0", variant, selected)
		}
	}
}

// TestTabsHighlightBlock checks that the highlight block sits behind the chosen
// label and slides to a new choice instead of jumping there.
func TestTabsHighlightBlock(t *testing.T) {
	for _, dark := range []bool{false, true} {
		selected := 0
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.Palette(dark))
			Tabs(c, &selected, TabsHighlight, "General", "Appearance", "Advanced")
		}, 400, 100)
		bg := tt.Image().RGBAAt(399, 99)
		filled := func(label string) bool {
			t.Helper()
			r, ok := tt.Find(label)
			if !ok {
				t.Fatalf("missing %s", label)
			}
			return tt.Image().RGBAAt(int(r.X+r.W/2), int(r.Y-2)) != bg
		}
		if !filled("General") || filled("Advanced") {
			t.Fatalf("dark=%v: block not behind the chosen tab", dark)
		}
		selected = 2
		tt.Frame()
		if filled("Advanced") {
			t.Fatal("block jumped to the choice instead of sliding")
		}
		tt.Advance(tabsIndicatorDuration + 20*time.Millisecond)
		tt.Frame()
		if !filled("Advanced") || filled("General") {
			t.Fatalf("dark=%v: block did not reach the chosen tab", dark)
		}
	}
}
