package components

import "github.com/egoist/mygo/ui"

// TextBoxOptions configures TextBox and TextBoxBuffer editing behavior and appearance.
type TextBoxOptions struct {
	// Multiline accepts line breaks; the default is a single-line input.
	Multiline bool
	// ReadOnly allows focus, selection and copying, but not editing. It uses
	// a muted background and neutral border even when focused.
	ReadOnly bool
	// LeadingIcon is an optional decorative SVG, for example lucide.Search().
	// It reserves space on the left and aligns with the first line in multiline
	// mode. Supply the accessible name with Label. Nil leaves no icon or gap.
	LeadingIcon *ui.SVG
	// LeadingText is a non-editable, theme-font prefix inside the field, such as
	// "R", "#" or "$". It follows LeadingIcon when both are set, reserves measured
	// space and does not become part of the value, selection or copied text.
	// Keep it short and single-line; supply the accessible name with Label.
	LeadingText string
}

// TextBox creates a text input editing *value. Configure mode and read-only
// state through options, not the returned Element's ReadOnly method, so styling
// matches behavior. Other MyGo input properties can be chained normally, such as
// Label, Placeholder, Password (single-line only) and Lines (multiline only).
// For stable identity, key a parent container rather than this element:
// interaction styling is resolved during construction.
func TextBox(c *ui.Context, value *string, options TextBoxOptions) ui.Element {
	var e ui.Element
	if options.Multiline {
		e = ui.TextAreaBase(c, value)
	} else {
		e = ui.TextInputBase(c, value)
	}
	return styleTextBox(c, e, options)
}

// TextBoxBuffer creates a text input editing an application-owned buffer.
// Keep the non-nil buffer across builds; ordinary editing does not materialize
// the whole document as a string. Options and chained input properties follow
// TextBox, including its parent-key requirement for stable identity.
func TextBoxBuffer(c *ui.Context, value *ui.TextBuffer, options TextBoxOptions) ui.Element {
	var e ui.Element
	if options.Multiline {
		e = ui.TextAreaBufferBase(c, value)
	} else {
		e = ui.TextInputBufferBase(c, value)
	}
	return styleTextBox(c, e, options)
}

func styleTextBox(c *ui.Context, e ui.Element, options TextBoxOptions) ui.Element {
	t := c.Theme()
	if options.Multiline {
		e.MinHeight(t.Space(20))
	}
	e.ReadOnly(options.ReadOnly)
	background, border := t.Surface, t.Border
	if options.ReadOnly {
		background = background.Mix(t.Text, 0.04)
	}
	// Let host surfaces and native sidebar materials show through the field.
	background = background.Alpha(0.5)
	if !e.IsDisabled() {
		switch {
		case e.Focused() && !options.ReadOnly:
			border = t.Accent
		case e.Focused() || e.Hovered():
			border = t.Border.Mix(t.Text, 0.25)
		}
	}
	e.Padding(t.Space(1.5), t.Space(2.5)).Radius(t.Radius).Background(background).Border(1, border)
	if options.LeadingIcon != nil || options.LeadingText != "" {
		span := ui.Span{Text: options.LeadingText, Color: t.TextMuted}
		width, height := c.MeasureText(0, span)
		left := t.Space(2.5)
		if options.LeadingIcon != nil {
			left = t.Space(8)
		}
		inset := left
		if options.LeadingText != "" {
			inset += width + t.Space(2)
		}
		e.Padding(t.Space(1.5), t.Space(2.5), t.Space(1.5), inset)
		e.DrawOver(func(p *ui.Painter, r ui.Rect) {
			if icon := options.LeadingIcon; icon != nil {
				size := t.Space(4)
				y := r.Y + (r.H-size)/2
				if options.Multiline {
					y = r.Y + 1 + t.Space(1.5)
				}
				p.Icon(icon, ui.Rect{X: r.X + t.Space(2.5), Y: y, W: size, H: size}, t.TextMuted)
			}
			if options.LeadingText != "" {
				y := r.Y + (r.H-height)/2
				if options.Multiline {
					y = r.Y + 1 + t.Space(1.5)
				}
				p.Clip(r, t.Radius, func() { p.RichText(r.X+1+left, y, 0, span) })
			}
		})
	}
	return dimmed(e)
}
