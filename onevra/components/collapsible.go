package components

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// ArrowPosition places a Collapsible's arrow immediately beside its label.
type ArrowPosition uint8

const (
	ArrowRight ArrowPosition = iota
	ArrowLeft
)

// CollapsibleOptions configures the disclosure arrow, whose default is right,
// optional leading content, such as an icon, built inside the trigger before
// its label, and optional trailing content, such as a count, built at the
// header's end outside the trigger and vertically centered on it.
type CollapsibleOptions struct {
	ArrowPosition ArrowPosition
	Leading       func()
	Trailing      func()
}

// DisclosureParts exposes the container and disclosure button for this frame.
// Changed reports activation of the trigger, not application-owned state edits.
type DisclosureParts struct {
	Root    ui.Element
	Trigger ui.Element
}

func (p DisclosureParts) Changed() bool { return p.Trigger.Changed() }

// Collapsible shows an animated disclosure with MyGo's spacing and appearance,
// without hover styling. The caller owns open; Enter and Space toggle it.
func Collapsible(c *ui.Context, label string, open *bool, options CollapsibleOptions, fn func()) DisclosureParts {
	t := c.Theme()
	result := DisclosureParts{Root: ui.Column(c).Shrink(0)}
	result.Root.Children(func() {
		var p ui.CollapsibleParts
		trigger := func() {
			p = ui.CollapsibleBase(c, open)
			result.Trigger = p.Trigger.AlignSelf(ui.Start).AlignItems(ui.Center).Gap(t.Space(1)).
				Padding(t.Space(1), t.Space(2), t.Space(1), t.Space(0.5)).Radius(t.Radius)
			result.Trigger.Children(func() {
				if options.ArrowPosition == ArrowLeft {
					disclosureArrow(c, 90*p.Progress())
				}
				if options.Leading != nil {
					options.Leading()
				}
				Text(c, label).SingleLine()
				if options.ArrowPosition != ArrowLeft {
					disclosureArrow(c, 90*p.Progress())
				}
			})
		}
		if options.Trailing == nil {
			trigger()
		} else {
			// The trailing content sits beside the trigger, so it never toggles.
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
				trigger()
				result.Trigger.AlignSelf(ui.Center).MinWidth(0)
				ui.Spacer(c)
				ui.Row(c).Shrink(0).AlignItems(ui.Center).Children(options.Trailing)
			})
		}
		indent := t.Space(0.5)
		if options.ArrowPosition == ArrowLeft {
			indent = t.Space(5.5)
		}
		p.Panel(fn).Padding(t.Space(1), 0, t.Space(1), indent).Gap(t.Space(2))
	})
	return result
}

func disclosureArrow(c *ui.Context, degrees float32) ui.Element {
	t := c.Theme()
	return ui.Box(c).Size(t.Space(4), t.Space(4)).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
		sin, cos := math.Sincos(float64(degrees) * math.Pi / 180)
		at := func(x, y float32) (float32, float32) {
			return r.X + r.W/2 + x*float32(cos) - y*float32(sin), r.Y + r.H/2 + x*float32(sin) + y*float32(cos)
		}
		d := r.W / 8
		var path ui.Path
		path.MoveTo(at(-d, -2*d)).LineTo(at(d, 0)).LineTo(at(-d, 2*d))
		p.StrokePath(&path, 1.5, t.TextMuted)
	})
}
