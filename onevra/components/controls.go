package components

import "github.com/egoist/mygo/ui"

// toolbarFace returns the raised face of a chosen segment or toolbar control,
// with its hover and pressed shades and its cast shadow: a lighter surface in
// either appearance.
func toolbarFace(t *ui.Theme) (face, hover, pressed, shadow ui.Color) {
	if t.Dark {
		return t.SurfaceHover, t.SurfaceHover.Mix(t.Text, 0.08), t.SurfacePressed, ui.RGBA(0, 0, 0, 0.4)
	}
	return t.Surface, t.SurfaceHover, t.SurfacePressed, ui.RGBA(0, 0, 0, 0.14)
}

// toolbarTrack returns the recessed track behind a segmented tab list: the
// foreground color at low opacity, so it recesses whatever surface it sits on.
func toolbarTrack(t *ui.Theme) ui.Color { return t.Text.Alpha(0.06) }

// dimmed applies the design style's disabled treatment. The bases own the
// disabled semantics; without their styleFn hook, opacity is the faithful
// visual equivalent.
func dimmed(e ui.Element) ui.Element {
	if e.IsDisabled() {
		e.Opacity(0.45)
	}
	return e
}
