package components

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// BadgeTone selects a badge's status color.
type BadgeTone int

const (
	// BadgeNeutral is the bordered surface face.
	BadgeNeutral BadgeTone = iota
	// BadgeAccent tints the badge with the theme accent.
	BadgeAccent
	// BadgeSuccess tints the badge with the theme success color.
	BadgeSuccess
	// BadgeWarning tints the badge with the theme warning color.
	BadgeWarning
	// BadgeDanger tints the badge with the theme danger color.
	BadgeDanger
)

// BadgeOptions selects a badge's tone and optional leading icon.
type BadgeOptions struct {
	Tone BadgeTone
	// Icon is decorative; the text must state the status on its own.
	Icon *ui.SVG
}

// badgeLabelKey and badgeIconKey identify the label and icon by their content,
// so a change replaces the element and plays its exit and entry.
type (
	badgeLabelKey struct{ text string }
	badgeIconKey  struct{ icon *ui.SVG }
)

// badgeSwap is how a changed label or icon replaces the previous one.
const badgeSwap = 220 * time.Millisecond

// Badge creates a compact, non-interactive status label. Applications own its
// text and may override the inherited foreground and background colors.
//
// Keep a badge built in the same place and change its text or icon to morph
// it: the new label rises in while the old one lifts away, icons crossfade and
// the pill's width springs to fit. Reduced Motion fades the swap in place and
// resizes at once. A badge appearing with its parent shows without animation.
func Badge(c *ui.Context, text string, options ...BadgeOptions) ui.Element {
	if len(options) > 1 {
		panic("components.Badge: at most one options value")
	}
	var o BadgeOptions
	if len(options) == 1 {
		o = options[0]
	}
	t := c.Theme()
	size := t.Rem(.85)
	bg, fg, border := badgeColors(t, o.Tone)
	rise := size * .7
	swap := ui.ElementTransition{Duration: badgeSwap, Position: true}
	label, icon := swap, swap
	label.Enter, label.Exit = &ui.Motion{Y: rise}, &ui.Motion{Y: -rise}
	icon.Enter, icon.Exit = &ui.Motion{Scale: .6}, &ui.Motion{Scale: .6}
	b := ui.Row(c).AlignItems(ui.Center).Gap(t.Space(1)).AlignSelf(ui.Start).Shrink(0).Clip().
		FontSize(size).FontWeight(500).
		Padding(t.Space(.5), t.Space(2)).Radius(t.Space(3)).
		Background(bg).TextColor(fg).Border(1, border).
		Transition(ui.ElementTransition{Animation: &ui.Snappy, Size: true, Colors: true})
	b.Children(func() {
		if o.Icon != nil {
			Icon(c.Key(badgeIconKey{o.Icon}), o.Icon).Size(size, size).Shrink(0).Transition(icon)
		}
		// The label keeps its natural width while the pill springs around it.
		Text(c.Key(badgeLabelKey{text}), text).SingleLine().Unselectable().Shrink(0).Transition(label)
	})
	return b
}

// badgeColors returns the background, text and border colors of a tone.
// Tinted tones darken or lighten the text toward the theme text for contrast.
func badgeColors(t *ui.Theme, tone BadgeTone) (bg, fg, border ui.Color) {
	var tint ui.Color
	switch tone {
	case BadgeAccent:
		tint = t.Accent
	case BadgeSuccess:
		tint = t.Success
	case BadgeWarning:
		tint = t.Warning
	case BadgeDanger:
		tint = t.Danger
	default:
		return t.Surface, t.Text, t.Border
	}
	return t.Background.Mix(tint, .12), tint.Mix(t.Text, .3), t.Background.Mix(tint, .3)
}
