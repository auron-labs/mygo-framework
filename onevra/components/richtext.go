package components

import "github.com/egoist/mygo/ui"

// RichText composes styled spans and inline Text or ButtonLink children into a
// single wrapping paragraph. Zero span fields inherit the surrounding style.
// MyGo's unstyled text primitive owns shaping, selection, inline interaction
// and accessibility; only text elements belong in Children.
func RichText(c *ui.Context, spans ...ui.Span) ui.Element {
	// RichText is MyGo's unstyled span primitive. Supplying spans directly keeps
	// one text node, avoiding a UI element per syntax token in large paragraphs.
	return ui.RichText(c, spans...)
}
