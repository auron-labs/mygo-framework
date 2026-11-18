package sidebar

import (
	"github.com/auron-labs/mygo-framework/onevra/components"
	"github.com/egoist/mygo/ui"
)

// IconButton creates an icon-only button with a tooltip and accessible
// label. It uses MyGo's pointer, keyboard and focus behavior and can be placed
// in ui.Toolbar. Applications own the label and handle Clicked on the result.
// The face is transparent at rest; hover and press use translucent text color.
func IconButton(c *ui.Context, icon *ui.SVG, label string) ui.Element {
	return components.IconButton(c, icon, label, components.IconButtonPlain)
}

// iconButton varies the glyph size without shrinking the hit target.
func iconButton(c *ui.Context, icon *ui.SVG, label string, iconSize float32) ui.Element {
	t := c.Theme()
	size := max(32, t.Rem(2))
	b := ui.ButtonBase(c).Size(size, size).Radius(t.Radius).
		TextColor(t.TextMuted).Label(label)
	// Paint after properties such as Disabled have been applied by the caller.
	b.Draw(func(p *ui.Painter, r ui.Rect) {
		if b.IsDisabled() {
			return
		}
		if b.Pressed() {
			p.Fill(r, t.Text.Alpha(0.14), t.Radius)
		} else if b.Hovered() {
			p.Fill(r, t.Text.Alpha(0.08), t.Radius)
		}
	}).Children(func() {
		components.Icon(c, icon).Size(iconSize, iconSize)
	})
	components.Tooltip(c, b, label)
	return b
}
