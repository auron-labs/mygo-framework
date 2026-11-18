package components

import "github.com/egoist/mygo/ui"

// TextOptions selects editing behavior without adding input decoration.
type TextOptions struct {
	// Editable binds single-line editing to application-owned text. Nil keeps
	// display text; otherwise *Editable supplies the text instead of value.
	// Edits update the binding immediately, not on Enter or blur.
	Editable *string
}

// TextControl exposes the layout element. Keep this handle to query Changed or
// Submitted after applying Element methods, which return ui.Element.
type TextControl struct {
	ui.Element
	input    ui.Element
	editable bool
}

// Changed reports a user edit to the bound value.
func (t *TextControl) Changed() bool {
	return t.editable && t.input.Changed()
}

// Submitted reports Enter in editable text.
func (t *TextControl) Submitted() bool {
	return t.editable && t.input.Submitted()
}

// Label supplies the accessible name of the text or its editor.
func (t *TextControl) Label(label string) *TextControl {
	t.Element.Label(label)
	if t.editable {
		t.input.Label(label)
	}
	return t
}

// AutoFocus focuses the editor when editing is enabled.
func (t *TextControl) AutoFocus() *TextControl {
	if t.editable {
		t.input.AutoFocus()
	} else {
		t.Element.AutoFocus()
	}
	return t
}

// Text creates text inheriting the frame theme and parent font, size and color.
// With no Editable binding it is MyGo's text primitive, including wrapping and
// RichText composition. Editable text is single-line, like an inline name, and
// adds only the native editor's caret and selection, not a background, border,
// padding or focus ring. MyGo owns input, shaping and rendering.
//
//	text := Text(c, name, TextOptions{Editable: &name})
//	text.Label("Name").Bold()
//	if text.Changed() { /* use name */ }
//
// Apply layout and typography to the returned Element as usual. When editing,
// call Label and AutoFocus on TextControl before Element styling methods. Key a
// parent container for stable identity across reordering, not this element.
func Text(c *ui.Context, value string, options ...TextOptions) *TextControl {
	if len(options) > 1 {
		panic("components.Text: at most one options value")
	}
	var binding *string
	if len(options) == 1 {
		binding = options[0].Editable
	}
	if binding == nil {
		return &TextControl{Element: ui.Text(c, value)}
	}
	t := &TextControl{Element: ui.Box(c).MinWidth(0), editable: true}
	t.Element.Children(func() {
		// The text primitive owns intrinsic sizing, including inherited styles.
		// The unstyled editor follows that rectangle instead of using its fixed
		// intrinsic width. One extra DIP holds the caret without shifting text;
		// it takes no layout space. Empty text still has a hit area.
		ui.Text(c, *binding).SingleLine().MinWidth(1).Opacity(0).Unselectable().Role(ui.RoleNone)
		t.input = ui.TextInputBase(c, binding).Absolute().Top(0).Bottom(0).Left(0).Right(-1).MinWidth(0).FocusRing(false)
	})
	return t
}
