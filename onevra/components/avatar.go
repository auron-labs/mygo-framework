package components

import (
	"strings"
	"unicode"

	"github.com/auron-labs/mygo-framework/v2/onevra/internal/l10n"
	"github.com/egoist/mygo/ui"
)

// AvatarStatus is the presence an Avatar shows with a corner dot.
type AvatarStatus int

const (
	// AvatarNoStatus shows no presence dot.
	AvatarNoStatus AvatarStatus = iota
	// AvatarOnline shows a success-colored dot.
	AvatarOnline
	// AvatarOffline shows a muted dot.
	AvatarOffline
)

// AvatarOptions customize an Avatar.
type AvatarOptions struct {
	// Image is a decoded photo, cover-cropped to the circle. Nil shows the
	// name's initials instead, so an application shows them while loading
	// and after a failure by passing nil.
	Image ui.ImageSource
	// Status adds a presence dot and appends it to the accessible name.
	Status AvatarStatus
	// Size is the diameter in DIPs; zero uses the standard theme-scaled size.
	// Initials and the presence dot scale with it.
	Size float32
}

// Avatar creates a circular identity marker for a person or account named
// name: a photo, or up to two initials on a muted surface. It draws no
// outline, so it sits on any surface without a seam. Applications own
// loading, decoding and caching photos. Assistive technology sees an image
// named name and its status. Key a parent for identity.
func Avatar(c *ui.Context, name string, options AvatarOptions) ui.Element {
	return avatar(c, name, options, 0)
}

// avatar builds an Avatar with a separator of width w in the surface color,
// which the photo leaves uncovered, so overlapping avatars in a group cut
// into the ones beneath them.
func avatar(c *ui.Context, name string, options AvatarOptions, w float32) ui.Element {
	t := c.Theme()
	d := avatarDiameter(c, options.Size)
	label := name
	switch options.Status {
	case AvatarOnline:
		label = l10n.In(c.Locale()).Label("avatarOnline", name)
	case AvatarOffline:
		label = l10n.In(c.Locale()).Label("avatarOffline", name)
	}
	e := ui.Box(c).Size(d, d).Shrink(0).Center().Radius(d/2).
		Background(t.SurfaceHover).TextColor(t.Text).Border(w, t.Surface).
		Role(ui.RoleImage).Label(label)
	e.Children(func() {
		if !imageMissing(options.Image) {
			ui.Image(c, options.Image).Fit(ui.Cover).Size(d-2*w, d-2*w).Radius(d/2 - w)
		} else {
			ui.Text(c, avatarInitials(name)).SingleLine().Unselectable().
				FontSize(avatarFontSize(d)).FontWeight(500)
		}
		if options.Status != AvatarNoStatus {
			avatarStatusDot(c, d, options.Status)
		}
	})
	return e
}

func avatarStatusDot(c *ui.Context, d float32, status AvatarStatus) {
	t := c.Theme()
	// 8 DIPs at 28, 10 at 36, 12 at 48, at most 14; large avatars inset it
	// toward the curve.
	size := min(14, max(8, 4+d/6))
	inset := max(0, (d-48)/10)
	ring := float32(2)
	if d < 32 {
		ring = 1.5
	}
	color := t.Success
	if status == AvatarOffline {
		color = t.TextMuted
	}
	ui.Box(c).Key(status).Absolute().Right(inset).Bottom(inset).Size(size, size).Radius(size/2).
		Background(color).Border(ring, t.Surface).
		Transition(ui.ElementTransition{
			Animation: &ui.Snappy,
			Enter:     &ui.Motion{Scale: .6},
			Exit:      &ui.Motion{Scale: .6},
		})
}

// avatarDiameter returns size, or the standard diameter of 36 DIPs at a
// 13-DIP font size, scaled with the text.
func avatarDiameter(c *ui.Context, size float32) float32 {
	if size > 0 {
		return size
	}
	return c.Theme().Rem(2.75)
}

// avatarFontSize follows the reference sizes: 10 at 28 DIPs, about 12 at 36,
// 14 at 48 and 20 at 88.
func avatarFontSize(d float32) float32 { return max(8, 6+d*.16) }

// avatarInitials returns the uppercase first letters of the first two words.
func avatarInitials(name string) string {
	var b strings.Builder
	for i, word := range strings.Fields(name) {
		if i == 2 {
			break
		}
		for _, r := range word {
			b.WriteRune(unicode.ToUpper(r))
			break
		}
	}
	return b.String()
}

// imageMissing reports a nil source, including a typed nil bitmap or SVG.
func imageMissing(source ui.ImageSource) bool {
	switch v := source.(type) {
	case nil:
		return true
	case *ui.Bitmap:
		return v == nil
	case *ui.SVG:
		return v == nil
	}
	return false
}
