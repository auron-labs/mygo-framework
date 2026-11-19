package components

import "github.com/egoist/mygo/ui"

// ButtonVariant selects a button's visual weight.
type ButtonVariant int

const (
	// ButtonPrimary is the accent-filled main action.
	ButtonPrimary ButtonVariant = iota
	// ButtonSecondary is a bordered action with a subtle text-tinted fill.
	ButtonSecondary
	// ButtonOutline is the bordered quiet action.
	ButtonOutline
	// ButtonGhost is the borderless quiet action.
	ButtonGhost
	// ButtonLink is inline text with an underline on hover.
	ButtonLink
	// ButtonToolbar is a raised toolbar control whose capsule and shadow match
	// the chosen segment of a segmented tab list, with a hairline edge that
	// keeps it readable outside a track.
	ButtonToolbar
)

// Button creates a labeled button in the Onevra style with a single standard
// size. Ask Clicked whether it was clicked; give it other content with Children
// and an empty label. The label is also the accessible name. ButtonLink uses
// text height without padding or a background and supports RichText composition;
// its children must be text. For stable identity, key a parent container rather
// than this element: interaction styling is resolved during construction.
func Button(c *ui.Context, label string, variant ButtonVariant) ui.Element {
	if variant == ButtonLink {
		return dimmed(ui.Link(c, label, ""))
	}
	b := styleButton(c, ui.ButtonBase(c), variant)
	if label != "" {
		b.Label(label)
		b.Children(func() { Text(c, label).SingleLine() })
	}
	return dimmed(b)
}

// styleButton gives a button base, such as one a MyGo part provides, the face
// of a non-link variant.
func styleButton(c *ui.Context, b ui.Element, variant ButtonVariant) ui.Element {
	t := c.Theme()
	b.Padding(t.Space(1.5), t.Space(3.5))
	bg, hover, pressed, fg, border := t.Surface, t.SurfaceHover, t.SurfacePressed, t.Text, t.Border
	radius := t.Radius
	var shadow ui.Color
	switch variant {
	case ButtonPrimary:
		bg, hover, pressed, fg, border = t.Accent, t.AccentHover, t.AccentPressed, t.AccentText, ui.Color{}
	case ButtonSecondary:
		bg, fg = t.Background.Mix(t.Text, .08), t.Text
		hover, pressed = t.Background.Mix(t.Text, .12), t.Background.Mix(t.Text, .18)
	case ButtonOutline:
		bg, border = ui.Color{}, t.Border
	case ButtonGhost:
		bg, border = ui.Color{}, ui.Color{}
	case ButtonToolbar:
		// The capsule matches the chosen segment of a segmented tab list, with a
		// hairline edge that keeps it readable off a track.
		bg, hover, pressed, shadow = toolbarFace(t)
		fg, border, radius = t.Text, t.Border.Alpha(0.8), t.Space(20)
	}
	if !b.IsDisabled() {
		switch {
		case b.Pressed():
			bg = pressed
		case b.Hovered():
			bg = hover
		}
	}
	if variant == ButtonPrimary {
		border = bg
	}
	// A transparent Ghost border reserves the same space as the filled variants.
	b.Radius(radius).Background(bg).TextColor(fg).Border(1, border)
	if variant == ButtonToolbar {
		// A toolbar button needs less lift than a chosen tab, which separates
		// itself from the track behind it.
		b.Shadow(0, 1, 2, 0, shadow.Alpha(0.5))
	}
	return b
}
