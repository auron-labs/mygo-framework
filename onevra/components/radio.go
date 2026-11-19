package components

import "github.com/egoist/mygo/ui"

// Radio creates a radio button that selects value into *selected, with a
// label. The label is also the accessible name.
func Radio[T comparable](c *ui.Context, selected *T, value T, label string) ui.Element {
	t := c.Theme()
	row := ui.RadioBase(c, selected, value).Gap(t.Space(2)).FocusRing(false)
	on, focus := *selected == value, row.FocusVisible()
	row.Children(func() {
		dot := ui.Box(c).Size(t.Space(4), t.Space(4)).Radius(t.Space(2)).Shrink(0)
		border := t.Border.Mix(t.Text, 0.25)
		if !on && row.Hovered() {
			border = t.Accent
		}
		if on {
			dot.Background(t.Accent)
		} else {
			dot.Background(t.Surface).Border(1, border)
		}
		dot.DrawOver(func(p *ui.Painter, r ui.Rect) {
			if on {
				d := t.Space(1.5)
				p.Fill(ui.Rect{X: r.X + (r.W-d)/2, Y: r.Y + (r.H-d)/2, W: d, H: d}, t.AccentText, d/2)
			}
			if focus {
				p.FocusRing(r, [4]float32{t.Radius, t.Radius, t.Radius, t.Radius})
			}
		})
		if label != "" {
			row.Label(label)
			Text(c, label)
		}
	})
	return dimmed(row)
}
