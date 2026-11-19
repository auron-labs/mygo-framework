package chat

import (
	"github.com/auron-labs/mygo-framework/onevra/components"
	"github.com/auron-labs/mygo-framework/onevra/icons/lucide"
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// FloatingConversationState retains composer measurements. Keep one alongside
// each ConversationState and share it with the floating GlassComposer.
type FloatingConversationState struct {
	height       float32
	cardHeight   float32
	footerHeight float32
}

// The composer card's height with one line of draft and no attachments,
// until a GlassComposer measures it, and how far ScrollEdge's progressive
// blur fades out beyond its extent.
const (
	restingCardHeight = 96
	edgeBlurFade      = 24
)

// FloatingConversation lets the transcript scroll behind the composer, but
// reserves enough trailing content space to leave the latest turn unobscured.
// The transcript blurs progressively from the bottom edge up to the middle of
// the composer card at rest, without following the card as it grows. Give the
// parent a bounded height and supply a localized jumpLabel; composer typically
// builds a GlassComposer. Applications own both states and all content;
// builders run synchronously.
func FloatingConversation(c *ui.Context, layout *FloatingConversationState, state *ConversationState, jumpLabel string, content, composer View) ui.Element {
	return FloatingConversationWith(c, layout, state, FloatingConversationOptions{JumpLabel: jumpLabel, Content: content, Composer: composer})
}

// FloatingConversationOptions supplies FloatingConversation's views. The
// conversation does not manage a window title bar or floating header: hosts
// that overlay one position it themselves and set TopInset to clear it.
type FloatingConversationOptions struct {
	JumpLabel         string
	Content, Composer View
	// TopInset reserves space above the transcript and its scroll bar, such
	// as for a title bar the conversation scrolls under; the content, and so
	// the first turn, starts below it. Negative values are treated as zero.
	TopInset float32
}

// FloatingConversationWith is FloatingConversation with its views as options.
func FloatingConversationWith(c *ui.Context, layout *FloatingConversationState, state *ConversationState, options FloatingConversationOptions) ui.Element {
	jumpLabel, content, composer := options.JumpLabel, options.Content, options.Composer
	inset := max(0, options.TopInset)
	return ui.Column(c).FillWidth().Grow(1).MinHeight(0).MinWidth(0).Clip().Children(func() {
		height := layout.height
		if height <= 0 {
			height = 140
		}
		if state.paused && state.Scroll.Y != state.pauseY && state.AtLatest() {
			state.paused = false
		}
		follow := state.AtLatest() && !state.paused
		previousY := state.Scroll.Y
		if follow {
			state.JumpToLatest()
		}
		// Leave a small gap above the floating composer without shortening
		// the transcript viewport that scrolls behind it.
		viewport := components.ScrollView(c, components.ScrollViewOptions{}).Key("transcript").FillWidth().Grow(1).MinHeight(0).
			ScrollbarInsets(inset, 0, height+8, 0).TrackScroll(&state.Scroll)
		viewport.Children(func() {
			ui.Column(c).FillWidth().MinWidth(0).Shrink(0).AlignItems(ui.Center).Padding(22+inset, 20, 0, 20).Children(func() {
				ui.Column(c).FillWidth().MaxWidth(ContentWidth).MinWidth(0).Gap(48).Children(func() {
					slot := conversationSlot(c)
					saved := *slot
					*slot = conversationScope{state: state, previousY: previousY, reading: !follow}
					defer func() { *slot = saved }()
					if content != nil {
						content(c)
					}
				})
				ui.Box(c).Key("composer-clearance").Height(height + 22).Shrink(0)
			})
		})
		// Disclosures may cancel following while building the transcript.
		viewport.TrackScroll(&state.Scroll)
		// Messages blur behind the footer and the card's lower half, across
		// the whole width so no unblurred strip shows at the right edge.
		card := layout.cardHeight
		if card <= 0 {
			card = restingCardHeight
		}
		edge := 8 + layout.footerHeight + card/2 - edgeBlurFade
		// Bare blur: the default background tint would wash out the transcript.
		components.ScrollEdge(c, components.ScrollEdgeOptions{Style: components.ScrollEdgeBlur, Bottom: true, Extent: edge, Uncolored: true}).Key("bottom-edge")
		if !follow {
			hit := max(float32(32), components.TouchMin(c))
			b := components.Button(c, "", components.ButtonGhost).Label(jumpLabel).Absolute().Attach(ui.AnchorBottom, ui.AnchorBottom).
				Bottom(height+12).Size(hit, hit).Padding(0).Radius(hit / 2)
			t := c.Theme()
			tint := t.Background.Alpha(.75)
			if b.Hovered() {
				tint = t.Background.Alpha(.85)
			}
			// A soft lift separates the floating control from the transcript it
			// covers without reading as a hard edge.
			shadow := ui.RGBA(0, 0, 0, 0.08)
			if t.Dark {
				shadow = ui.RGBA(0, 0, 0, 0.22)
			}
			b.Border(0, ui.Transparent).TextColor(t.Text).
				Material(glass.Glass{Style: glass.Regular, Tint: tint, Interactive: true}).
				Shadow(0, 1, 2, 0, shadow)
			b.Children(func() { components.Icon(c, lucide.ArrowDown).Size(16, 16) })
			components.Tooltip(c, b, jumpLabel)
			if b.Clicked() {
				state.JumpToLatest()
				c.Invalidate()
			}
		}
		ui.Row(c).Key("floating-composer").Absolute().Attach(ui.AnchorBottom, ui.AnchorBottom).
			FillWidth().Justify(ui.Center).Padding(0, 20, 8, 20).Draw(func(_ *ui.Painter, r ui.Rect) {
			if layout.height != r.H {
				layout.height = r.H
				c.Invalidate()
			}
		}).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(ContentWidth).MinWidth(0).Children(func() {
				if composer != nil {
					composer(c)
				}
			})
		})
	})
}

// GlassPanel is a compact card with GlassComposer's surface, for floating
// status the host overlays on a conversation, such as command output or
// errors above the composer. Its height follows content, and its tighter
// radius suits a panel docked near the window edge.
func GlassPanel(c *ui.Context, content View) ui.Element {
	t := c.Theme()
	return ui.Column(c).FillWidth().MinWidth(0).Padding(8, 12).Radius(8).Border(1, t.Border).
		Material(glass.Glass{Style: glass.Regular, Tint: t.Surface.Alpha(.6)}).Children(func() {
		if content != nil {
			content(c)
		}
	})
}

// GlassComposer keeps Onevra's editor and actions, replacing only its surface.
// The footer stays outside the glass so the card remains visually lightweight.
// layout may be nil outside a floating conversation; with it, the
// conversation's blur reaches the middle of the card at rest.
func GlassComposer(c *ui.Context, layout *FloatingConversationState, draft *string, options ComposerOptions) ComposerResult {
	theme := c.Theme()
	result := Composer(c, draft, options)
	tint := theme.Surface.Alpha(.6)
	if result.Dragging {
		tint = theme.Surface.Mix(theme.Text, .09).Alpha(.6)
	}
	result.Card.Material(glass.Glass{Style: glass.Regular, Tint: tint})
	if layout != nil {
		layout.footerHeight = 0
		if options.Footer != nil {
			layout.footerHeight = 28
		}
		// Only a single empty line with no attachments shows the card at rest.
		if *draft == "" && options.Attachments == nil {
			result.Card.Draw(func(_ *ui.Painter, r ui.Rect) {
				if layout.cardHeight != r.H {
					layout.cardHeight = r.H
					c.Invalidate()
				}
			})
		}
	}
	return result
}
