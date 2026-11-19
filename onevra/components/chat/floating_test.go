package chat

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/auron-labs/mygo-framework/v2/onevra/components"
	"github.com/auron-labs/mygo-framework/v2/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestFloatingConversationClearanceAndJump(t *testing.T) {
	var layout FloatingConversationState
	var state ConversationState
	var latest, composer, viewport ui.Rect
	draft := "Reply"
	tt := ui.NewTester(func(c *ui.Context) {
		view := FloatingConversation(c, &layout, &state, "Latest", func(c *ui.Context) {
			for i := range 20 {
				row := components.Text(c, fmt.Sprintf("Message %d", i)).Height(40).Shrink(0)
				if i == 19 {
					latest = row.Bounds()
				}
			}
		}, func(c *ui.Context) {
			result := GlassComposer(c, &layout, &draft, ComposerOptions{Label: "Reply", SendLabel: "Send"})
			composer = result.Root.Bounds()
		})
		viewport = view.Bounds()
	}, 900, 600)
	tt.Frame()
	if !state.AtLatest() || latest.Y+latest.H > composer.Y {
		t.Fatalf("latest obscured: scroll=%+v latest=%+v composer=%+v", state.Scroll, latest, composer)
	}
	// The viewport extends behind the floating card, not just to its top.
	if viewport.H < composer.Y+composer.H-2 {
		t.Fatalf("viewport clipped above composer: %+v, %+v", state.Scroll, composer)
	}
	state.Scroll.Y = 0
	tt.Frame()
	jump, ok := tt.Find("Latest")
	if !ok || jump.Y+jump.H > composer.Y {
		t.Fatalf("jump hidden by composer: %+v, %+v", jump, composer)
	}
	if err := tt.Click("Latest"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !state.AtLatest() {
		t.Fatal("jump did not resume following")
	}
	oldHeight := layout.height
	draft = strings.Repeat("More lines\n", 6)
	tt.Frame()
	tt.Frame()
	if layout.height <= oldHeight || latest.Y+latest.H > composer.Y {
		t.Fatalf("multiline clearance did not update: height=%v old=%v latest=%+v composer=%+v", layout.height, oldHeight, latest, composer)
	}
}

func TestFloatingConversationThemedGlassJump(t *testing.T) {
	for _, palette := range []*ui.Theme{theme.LightPalette(), theme.DarkPalette()} {
		t.Run(fmt.Sprintf("dark=%v", palette.Dark), func(t *testing.T) {
			var layout FloatingConversationState
			var state ConversationState
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(palette)
				FloatingConversation(c, &layout, &state, "Latest", func(c *ui.Context) {
					ui.Box(c).Height(2000).FillWidth().Shrink(0).Background(ui.RGB(0, 0, 0))
				}, nil)
			}, 900, 600)
			tt.Frame()
			state.Scroll.Y = 0
			tt.Frame()
			jump, ok := tt.Find("Latest")
			if !ok {
				t.Fatal("missing jump control")
			}
			for _, hovered := range []bool{false, true} {
				if hovered {
					tt.Move(jump.X+jump.W/2, jump.Y+jump.H/2)
				}
				// Sample clear of the rim and arrow. The translucent glass
				// should stay close to the theme background over the transcript.
				pixel := tt.Image().RGBAAt(int(jump.X+7), int(jump.Y+jump.H/2))
				bg := palette.Background
				if math.Abs(float64(pixel.R)-float64(bg.R)) > 60 ||
					math.Abs(float64(pixel.G)-float64(bg.G)) > 60 ||
					math.Abs(float64(pixel.B)-float64(bg.B)) > 60 {
					t.Fatalf("hovered=%v: jump surface %v does not follow background %v", hovered, pixel, bg)
				}
			}
			if err := tt.Click("Latest"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if !state.AtLatest() {
				t.Fatal("glass jump did not resume following")
			}
		})
	}
}

func TestFloatingConversationBlursBottomEdge(t *testing.T) {
	var layout FloatingConversationState
	var state ConversationState
	var card ui.Rect
	draft := "Reply"
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(ui.LightTheme())
		FloatingConversation(c, &layout, &state, "Latest", func(c *ui.Context) {
			// Fine stripes beside the card, which a blur mixes to gray.
			ui.Row(c).Width(900).Height(2000).Margin(0, -90, 0, -90).Shrink(0).Children(func() {
				for i := range 450 {
					color := ui.RGB(0, 0, 0)
					if i%2 == 1 {
						color = ui.RGB(255, 255, 255)
					}
					ui.Box(c).Width(2).FillHeight().Shrink(0).Background(color)
				}
			})
		}, func(c *ui.Context) {
			result := GlassComposer(c, &layout, &draft, ComposerOptions{
				Label: "Reply", Footer: func(c *ui.Context) { components.Text(c, "Project / branch") },
			})
			card = result.Card.Bounds()
		})
	}, 900, 600)
	// The blur reaches the middle of the card at rest and stays there as a
	// longer draft grows the card upward.
	var middle float32
	for _, text := range []string{"", strings.Repeat("More lines\n", 6)} {
		draft = text
		state.Scroll.Y = 200
		tt.Frame()
		tt.Frame()
		if text == "" {
			middle = card.Y + card.H/2
		}
		img := tt.Image()
		s := float32(img.Bounds().Dx()) / 900
		// The widest spread of a row's channel beside the card: crisp stripes
		// span black to white, blurred ones stay near gray.
		spread := func(y float32, from, to int) int {
			lo, hi := 255, 0
			for x := from; x < to; x++ {
				v := int(img.RGBAAt(int(float32(x)*s), int(y*s)).R)
				lo, hi = min(lo, v), max(hi, v)
			}
			return hi - lo
		}
		top, above, footer := spread(2, 20, 40), spread(middle-2, 20, 40), spread(card.Y+card.H+14, 20, 40)
		if top < 200 || above < 200 || footer > 128 {
			t.Fatalf("incorrect blur for card %+v: top=%d above middle=%d footer=%d", card, top, above, footer)
		}
		// The blur spans the whole width, leaving no crisp strip at the
		// right edge.
		topRight, aboveRight, footerRight := spread(2, 865, 895), spread(middle-2, 865, 895), spread(card.Y+card.H+14, 865, 895)
		if topRight < 200 || aboveRight < 200 || footerRight > 128 {
			t.Fatalf("blur gap at the right edge for card %+v: top=%d above middle=%d footer=%d", card, topRight, aboveRight, footerRight)
		}
	}
}

func TestGlassComposerPreservesThemeAndRootBackground(t *testing.T) {
	for _, theme := range []*ui.Theme{ui.LightTheme(), ui.DarkTheme()} {
		draft := ""
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme)
			c.Root().Background(ui.Transparent)
			GlassComposer(c, nil, &draft, ComposerOptions{
				Attachments: func(c *ui.Context) {
					if c.Theme() != theme {
						t.Error("attachments theme changed")
					}
				},
				Accessory: func(c *ui.Context) {
					if c.Theme().Surface != theme.Surface {
						t.Error("accessory theme changed")
					}
				},
				Footer: func(c *ui.Context) {
					if c.Theme().Surface != theme.Surface {
						t.Error("footer theme changed")
					}
				},
			})
			if c.Theme() != theme {
				t.Error("composer changed frame theme")
			}
		}, 900, 400)
		tt.Frame()
		// Below the composer, the host's transparent root must remain exposed.
		if got := tt.Image().RGBAAt(450, 390); got.A != 0 {
			t.Fatalf("composer made the window root opaque: %v", got)
		}
	}
}
