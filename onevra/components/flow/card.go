package flow

import (
	"github.com/auron-labs/mygo-framework/v2/onevra/components"
	"github.com/egoist/mygo/ui"
)

// CardRadius rounds Card's corners; the selection ring follows it.
const CardRadius = 14

// CardOptions describes a Card; every region is a slot. Width defaults to
// 240 DIPs. Header builds a small row at the top, such as a status;
// Leading builds content before the body, such as an icon tile; Body
// builds the main content, or else Title and Description show, each on one
// line; Trailing builds content after the body, such as a badge or menu;
// Footer builds a small muted row at the bottom, such as counts. A nonzero
// Border replaces the theme's soft border color, as for a status.
type CardOptions struct {
	Width                   float32
	Title, Description      string
	Header, Leading, Footer func(*ui.Context)
	Body, Trailing          func(*ui.Context)
	Border                  ui.Color
}

// Card builds the default node face for Node.Content: a softly shadowed,
// rounded surface with optional header, leading, body, trailing and footer
// content.
func Card(c *ui.Context, opts CardOptions) ui.Element {
	t := c.Theme()
	width := opts.Width
	if width <= 0 {
		width = 240
	}
	border := opts.Border
	if border == (ui.Color{}) {
		border = t.Border.Alpha(.7)
	}
	small := t.Rem(.85)
	body := opts.Body
	if body == nil && (opts.Title != "" || opts.Description != "") {
		body = func(c *ui.Context) {
			if opts.Title != "" {
				components.Text(c, opts.Title).SingleLine().FontWeight(600)
			}
			if opts.Description != "" {
				components.Text(c, opts.Description).SingleLine().FontSize(small).TextColor(t.TextMuted)
			}
		}
	}
	return ui.Column(c).Width(width).Padding(t.Space(3)).Gap(t.Space(2.5)).
		Radius(CardRadius).Background(t.Surface).Border(1, border).
		Shadow(0, t.Space(.5), t.Space(3), 0, ui.RGBA(0, 0, 0, .05)).Children(func() {
		if opts.Header != nil {
			ui.Row(c).Gap(t.Space(1.5)).FontSize(small).Children(func() { opts.Header(c) })
		}
		if opts.Leading != nil || body != nil || opts.Trailing != nil {
			ui.Row(c).Gap(t.Space(2.5)).Children(func() {
				if opts.Leading != nil {
					opts.Leading(c)
				}
				ui.Column(c).Grow(1).MinWidth(0).Gap(t.Space(.5)).Children(func() {
					if body != nil {
						body(c)
					}
				})
				if opts.Trailing != nil {
					opts.Trailing(c)
				}
			})
		}
		if opts.Footer != nil {
			ui.Row(c).Gap(t.Space(3)).FontSize(small).TextColor(t.TextMuted).Children(func() { opts.Footer(c) })
		}
	})
}
