package components

import "github.com/egoist/mygo/ui"

// AccordionGroup builds the items of one Accordion. It is valid only inside
// the Accordion callback; nested accordions have independent groups.
type AccordionGroup struct {
	c       *ui.Context
	headers []ui.Element
}

// Accordion builds independently expandable sections in MyGo's bordered box,
// without hover styling. Use group.Item inside fn. Up, Down, Home and End
// move focus between headers; Tab still visits controls inside open panels.
func Accordion(c *ui.Context, fn func(group *AccordionGroup)) ui.Element {
	t := c.Theme()
	root := ui.Column(c).Radius(t.Radius).Border(1, t.Border).Clip()
	group := &AccordionGroup{c: c}
	root.Children(func() { fn(group) })
	for i, header := range group.headers {
		to := -1
		switch {
		case header.Shortcut(0, ui.KeyDown):
			to = min(i+1, len(group.headers)-1)
		case header.Shortcut(0, ui.KeyUp):
			to = max(i-1, 0)
		case header.Shortcut(0, ui.KeyHome):
			to = 0
		case header.Shortcut(0, ui.KeyEnd):
			to = len(group.headers) - 1
		}
		if to >= 0 {
			group.headers[to].Focus()
			break
		}
	}
	return root
}

// Item builds a section, with application-owned expansion state and content.
// Its Changed method reports trigger activation. To allow just one open item,
// use that signal to close other application-owned expansion values.
func (group *AccordionGroup) Item(title string, open *bool, fn func()) DisclosureParts {
	c := group.c
	t := c.Theme()
	result := DisclosureParts{Root: ui.Column(c).Shrink(0)}
	if len(group.headers) > 0 {
		result.Root.BorderWidth(1, 0, 0, 0).BorderColor(t.Border)
	}
	result.Root.Children(func() {
		p := ui.CollapsibleBase(c, open)
		tr := p.Trigger.AlignItems(ui.Center).Gap(t.Space(2)).Padding(t.Space(2.5), t.Space(3)).FocusRing(false)
		result.Trigger = tr
		group.headers = append(group.headers, tr)
		focus := tr.FocusVisible()
		tr.DrawOver(func(p *ui.Painter, r ui.Rect) {
			if focus {
				p.FocusRing(ui.Rect{X: r.X + 4, Y: r.Y + 4, W: r.W - 8, H: r.H - 8}, [4]float32{t.Radius, t.Radius, t.Radius, t.Radius})
			}
		})
		tr.Children(func() {
			Text(c, title).Grow(1).FontWeight(500)
			disclosureArrow(c, 90+180*p.Progress())
		})
		p.Panel(fn).Padding(0, t.Space(3), t.Space(3)).Gap(t.Space(2))
	})
	return result
}
