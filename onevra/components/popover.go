package components

import (
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// Popover builds a non-modal panel anchored below anchor, flipping above it when
// needed. Applications own open and trigger activation and call Popover every
// frame after building the anchor. The returned Element is absent while closed.
// MyGo owns positioning, focus order and Escape/outside-press dismissal; outside
// presses continue to the underlying control. Pressing the anchor does not
// dismiss the panel automatically.
//
// build styles the panel and creates its children. It may override placement
// with AttachTo or supply scrolling for long content. Keep anchor and panel
// handles within the current build pass and use keyed parents for independent
// popovers. Label the panel in build when an accessible name is needed.
func Popover(c *ui.Context, anchor ui.Element, open *bool, build func(panel ui.Element)) ui.Element {
	t := c.Theme()
	return ui.PopoverBase(c, anchor, open, func(panel ui.Element) {
		panel.Column().Padding(t.Space(4)).Gap(t.Space(3)).
			Margin(t.Space(1), 0, 0, 0).Radius(t.Radius*1.5).
			Material(glass.Glass{Tint: t.Background.Alpha(.92)}).TextColor(t.Text).Border(1, t.Border).
			Shadow(0, t.Space(1), t.Space(4), 0, ui.RGBA(0, 0, 0, .14))
		if build != nil {
			build(panel)
		}
	})
}
