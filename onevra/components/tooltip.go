package components

import (
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// Tooltip attaches a non-interactive text hint to anchor. Call it each frame
// after building the anchor; both handles belong to the current build pass.
// MyGo owns hover delay, keyboard-focus activation, dismissal and positioning
// above the anchor (or below when space is limited).
//
// The text also becomes anchor's accessible description. To provide a different
// description, call anchor.Description after Tooltip. Empty text does nothing.
// The returned tip is invalid while hidden. Use Popover for interactive content.
func Tooltip(c *ui.Context, anchor ui.Element, text string) ui.Element {
	if text == "" || !anchor.Valid() {
		return ui.Element{}
	}
	anchor.Description(text)
	return ui.TooltipBase(c, anchor, func(tip ui.Element) {
		styleTip(c, tip).Children(func() { Text(c, text) })
	})
}

// styleTip gives a tip from ui.TooltipBase Tooltip's face.
func styleTip(c *ui.Context, tip ui.Element) ui.Element {
	t := c.Theme()
	return tip.MaxWidth(t.Space(80)).Padding(t.Space(1.5), t.Space(2)).
		Radius(t.Radius).Material(glass.Glass{Style: glass.Clear, Tint: t.Text}).TextColor(t.Background).
		FontSize(t.FontSize-1).
		Shadow(0, t.Space(.5), t.Space(2), 0, ui.RGBA(0, 0, 0, .14))
}
