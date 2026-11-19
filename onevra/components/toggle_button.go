package components

import "github.com/egoist/mygo/ui"

// ToggleButton creates a button that toggles *on by click or focused Space.
// Changed reports user changes. The label is also the accessible name; for
// custom content, pass an empty label and use Label and Children.
// It uses one standard size, a quiet off face and an accent-filled on face.
// Key a parent container for stable identity, as styling is resolved here.
func ToggleButton(c *ui.Context, on *bool, label string) ui.Element {
	t := c.Theme()
	b := ui.ToggleBase(c, on).Padding(t.Space(1.5), t.Space(3.5))
	bg, hover, pressed, fg, border := ui.Color{}, t.SurfaceHover, t.SurfacePressed, t.Text, t.Border
	if *on {
		bg, hover, pressed, fg = t.Accent, t.AccentHover, t.AccentPressed, t.AccentText
	}
	if !b.IsDisabled() {
		switch {
		case b.Pressed():
			bg = pressed
		case b.Hovered():
			bg = hover
		}
	}
	if *on {
		border = bg
	}
	b.Radius(t.Radius).Background(bg).TextColor(fg).Border(1, border)
	if label != "" {
		b.Label(label)
		b.Children(func() { Text(c, label).SingleLine() })
	}
	return dimmed(b)
}
