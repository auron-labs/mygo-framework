package chat

import (
	"fmt"
	"strings"
	"testing"

	"github.com/auron-labs/mygo-framework/v2/onevra/components"
	"github.com/egoist/mygo/ui"
)

func TestFloatingConversationScrollbarClearsComposer(t *testing.T) {
	for _, width := range []int{900, 360} {
		for _, dark := range []bool{false, true} {
			t.Run(fmt.Sprintf("width=%d/dark=%v", width, dark), func(t *testing.T) {
				var layout FloatingConversationState
				var state ConversationState
				draft, showError := "", false
				theme := ui.LightTheme()
				if dark {
					theme = ui.DarkTheme()
				}
				// Isolate the scrollbar pixels from the content and glass.
				theme.Scrollbar = ui.RGB(240, 30, 70)
				tt := ui.NewTester(func(c *ui.Context) {
					c.SetTheme(theme)
					FloatingConversation(c, &layout, &state, "Latest", func(c *ui.Context) {
						ui.Box(c).Height(1800).Shrink(0)
						components.Text(c, "Last message").Height(20)
					}, func(c *ui.Context) {
						ui.Column(c).Label("Composer").FillWidth().Gap(6).Children(func() {
							if showError {
								GlassPanel(c, func(c *ui.Context) { components.Text(c, "Retrying") }).Height(44)
							}
							GlassComposer(c, &layout, &draft, ComposerOptions{
								Label:  "Reply",
								Footer: func(c *ui.Context) { components.Text(c, "Project / branch") },
							})
						})
					}).Label("Viewport")
				}, width, 600)
				for _, tc := range []struct {
					draft string
					err   bool
				}{{"", false}, {strings.Repeat("More lines\n", 6), false}, {"", true}, {"", false}} {
					draft, showError = tc.draft, tc.err
					state.JumpToLatest()
					tt.Frame()
					tt.Frame()
					x := float32(width - 6)
					tt.Move(x, 40)
					composer, ok := tt.Find("Composer")
					if !ok {
						t.Fatal("missing composer")
					}
					bottom := -1
					img := tt.Image()
					for y := 0; y < 600; y++ {
						pixel := img.RGBAAt(int(x), y)
						if pixel.R == 240 && pixel.G == 30 && pixel.B == 70 {
							bottom = y
						}
					}
					trackBottom := composer.Y - 8
					if bottom < int(trackBottom)-4 || bottom >= int(trackBottom) {
						t.Fatalf("scrollbar bottom=%d does not leave an 8-pixel gap above composer=%+v", bottom, composer)
					}
					viewport, _ := tt.Find("Viewport")
					last, _ := tt.Find("Last message")
					if viewport.H != 600 || !state.AtLatest() || last.Y+last.H > composer.Y {
						t.Fatalf("scrollbar inset changed viewport or latest visibility: viewport=%+v last=%+v composer=%+v", viewport, last, composer)
					}
					// The cleared region is not an invisible paging target.
					state.Scroll.Y = 0
					tt.Frame()
					for _, y := range []float32{composer.Y - 4, 590} {
						tt.ClickAt(x, y)
						if state.Scroll.Y != 0 {
							t.Fatalf("click below the scrollbar at %v paged to %v", y, state.Scroll.Y)
						}
					}
					// The shortened track still reaches the end of the transcript.
					// A press takes only a shown bar, and the click below hid it.
					tt.Move(x, 8)
					tt.Press(x, 8)
					tt.Move(x, trackBottom-2)
					tt.Release(x, trackBottom-2)
					if !state.AtLatest() {
						t.Fatalf("dragging to the track end did not reach latest: %+v", state.Scroll)
					}
				}
			})
		}
	}
}

func TestFloatingConversationTopInsetClearsScrollbar(t *testing.T) {
	const inset = 48
	var layout FloatingConversationState
	var state ConversationState
	draft := ""
	theme := ui.LightTheme()
	// Isolate the scrollbar pixels from the content and glass.
	theme.Scrollbar = ui.RGB(240, 30, 70)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme)
		FloatingConversationWith(c, &layout, &state, FloatingConversationOptions{
			JumpLabel: "Latest",
			TopInset:  inset,
			Content: func(c *ui.Context) {
				components.Text(c, "First message").Height(40).Shrink(0)
				ui.Box(c).Height(1800).Shrink(0)
			},
			Composer: func(c *ui.Context) {
				GlassComposer(c, &layout, &draft, ComposerOptions{Label: "Reply"})
			},
		}).Label("Viewport")
	}, 900, 600)
	tt.Frame()
	tt.Frame()
	// Read from the top rather than following the latest turn.
	state.Scroll.Y = 0
	tt.Frame()
	x := float32(900 - 6)
	tt.Move(x, 300)
	viewport, _ := tt.Find("Viewport")
	if viewport.Y != 0 || viewport.H != 600 {
		t.Fatalf("top inset shortened the viewport: %+v", viewport)
	}
	first, ok := tt.Find("First message")
	if !ok || first.Y != inset+22 {
		t.Fatalf("first message=%+v, want it %v below the top", first, inset+22)
	}
	img := tt.Image()
	top := -1
	for y := 0; y < 600; y++ {
		pixel := img.RGBAAt(int(x), y)
		if pixel.R == 240 && pixel.G == 30 && pixel.B == 70 {
			top = y
			break
		}
	}
	if top < inset || top > inset+4 {
		t.Fatalf("scrollbar top=%d does not start at the inset %d", top, inset)
	}
	// Scrolled content passes behind the inset rather than being clipped.
	state.Scroll.Y = 100
	tt.Frame()
	if first, _ = tt.Find("First message"); first.Y >= inset {
		t.Fatalf("scrolled first message %+v stayed below the inset", first)
	}
}
