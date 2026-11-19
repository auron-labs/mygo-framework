package components

import (
	"image/color"
	"testing"

	"github.com/auron-labs/mygo-framework/v2/onevra/icons/lucide"
	"github.com/auron-labs/mygo-framework/v2/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestSidebarLeadingSlot(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, disabled := range []bool{false, true} {
			clicks := 0
			var suggested float32
			var buildContext *ui.Context
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(theme.Palette(dark))
				Sidebar(c).Disabled(disabled).Children(func() {
					leading := func(slotContext *ui.Context, size float32) {
						buildContext, suggested = slotContext, size
						// A slot is not restricted to a square icon.
						ui.Box(slotContext).Label("Mark").Size(size*2, size).Shrink(0).
							Radius(4).Background(slotContext.Theme().Accent)
					}
					if SidebarItemWithLeading(c, "Custom", true, leading).Clicked() {
						clicks++
					}
					SidebarItemWithLeading(c, "Empty", false, nil)
					SidebarItem(c, "Plain", nil, false)
					if buildContext != c {
						t.Fatal("slot did not receive the current context")
					}
				})
			}, 240, 200)
			row, _ := tt.Find("Custom")
			mark, ok := tt.Find("Mark")
			empty, _ := tt.Find("Empty")
			plain, _ := tt.Find("Plain")
			if !ok || mark.W != suggested*2 || mark.H != suggested || mark.X < row.X || mark.X+mark.W > row.X+row.W ||
				empty.H != plain.H {
				t.Fatalf("slot or nil layout: row=%v mark=%v empty=%v plain=%v", row, mark, empty, plain)
			}
			if err := tt.Click("Mark"); err != nil {
				t.Fatal(err)
			}
			tt.Key(0, ui.KeyEnter)
			tt.Key(0, ui.KeySpace)
			want := 3
			if disabled {
				want = 0
			}
			if clicks != want {
				t.Fatalf("disabled=%v: got %d activations, want %d", disabled, clicks, want)
			}
		}
	}
}

func TestSettingsLeadingSlot(t *testing.T) {
	red := color.RGBA{R: 255, A: 255}
	for _, dark := range []bool{false, true} {
		state := SettingsState{}
		narrow, calls := false, 0
		catalog := settingsFixture()
		catalog[1].Icon = lucide.House()
		catalog[1].Image = solidBitmap(18, 18, red)
		catalog[1].Leading = func(c *ui.Context, size float32) {
			calls++
			ui.Box(c).Label("Custom mark").Size(size, size).Background(ui.RGB(0, 255, 0))
		}
		SearchSettings(catalog, "appearance")
		if calls != 0 {
			t.Fatal("metadata search built a slot")
		}
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.Palette(dark))
			Settings(c, &state, catalog, SettingsOptions{Narrow: narrow})
		}, 800, 600)
		mark, ok := tt.Find("Custom mark")
		if !ok || calls == 0 {
			t.Fatal("settings did not build the slot")
		}
		center := func(r ui.Rect) color.RGBA {
			return tt.Image().RGBAAt(int(r.X+r.W/2), int(r.Y+r.H/2))
		}
		if center(mark) != (color.RGBA{G: 255, A: 255}) {
			t.Fatal("slot did not replace the image and icon")
		}
		if err := tt.Click("Custom mark"); err != nil {
			t.Fatal(err)
		}
		if state.Category != "appearance" || !tt.HasText("Theme") {
			t.Fatal("slot intercepted category activation")
		}
		catalog[1].Leading = nil
		tt.Frame()
		if _, ok := tt.Find("Custom mark"); ok || center(mark) != red {
			t.Fatal("removing the slot did not restore the image")
		}
		catalog[1].Leading = func(*ui.Context, float32) { t.Fatal("narrow layout built a sidebar slot") }
		narrow = true
		tt.Frame()
	}
}
