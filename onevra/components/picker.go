package components

import "github.com/egoist/mygo/ui"

// pickerTrigger shares the TextBox-like face of date and time pickers.
func pickerTrigger(c *ui.Context, icon *ui.SVG) ui.Element {
	t := c.Theme()
	e := ui.ButtonBase(c).MinWidth(t.Space(36)).Justify(ui.Start).Role(ui.RolePopUpButton)
	border := t.Border
	if !e.IsDisabled() {
		switch {
		case e.Focused():
			border = t.Accent
		case e.Hovered():
			border = t.Border.Mix(t.Text, .25)
		}
	}
	e.Padding(t.Space(1.5), t.Space(2.5), t.Space(1.5), t.Space(8)).
		Radius(t.Radius).Background(t.Surface.Alpha(0.5)).Border(1, border).TextColor(t.Text)
	e.DrawOver(func(p *ui.Painter, r ui.Rect) {
		size := t.Space(4)
		p.Icon(icon, ui.Rect{X: r.X + t.Space(2.5), Y: r.Y + (r.H-size)/2, W: size, H: size}, t.TextMuted)
	})
	return dimmed(e)
}
