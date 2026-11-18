package theme

import "github.com/egoist/mygo/ui"

// Onevra pairs warm white and warm charcoal surfaces with a terracotta accent.
// Dark mode lifts the accent for legibility without changing its color family.
// Spacing and font size keep following MyGo.

// LightPalette returns Onevra's light palette.
func LightPalette() *ui.Theme {
	t := ui.LightTheme()
	accent := ui.Hex("#bd481b")
	black := ui.RGB(0, 0, 0)
	t.Background = ui.Hex("#fcfbf9")
	t.Surface = ui.Hex("#fefefd")
	t.SurfaceHover = ui.Hex("#eceae4")
	t.SurfacePressed = ui.Hex("#d9d7d2")
	t.Border = ui.Hex("#deded5")
	t.Text = ui.Hex("#33332f")
	t.TextMuted = ui.Hex("#72726a")
	t.Accent = accent
	t.AccentHover = accent.Mix(black, 0.12)
	t.AccentPressed = accent.Mix(black, 0.25)
	t.AccentText = ui.Hex("#ffffff")
	t.Danger = ui.Hex("#b4444c")
	// Amber complements the warm accent while remaining distinct from danger.
	t.Warning = ui.Hex("#b45309")
	t.Success = ui.Hex("#059669")
	t.Selection = ui.RGBA(230, 101, 43, 0.22)
	t.Focus = ui.RGBA(185, 71, 22, 0.55)
	t.Scrollbar = t.Text.Alpha(.30)
	t.Radius = 8
	return t
}

// DarkPalette returns Onevra's dark palette.
// MyGo widgets branch on the Dark flag, so it stays set.
func DarkPalette() *ui.Theme {
	t := ui.DarkTheme()
	t.Dark = true
	accent := ui.Hex("#ed986e")
	black := ui.RGB(0, 0, 0)
	white := ui.RGB(255, 255, 255)
	t.Background = ui.Hex("#211f1c")
	t.Surface = ui.Hex("#2b2925")
	t.SurfaceHover = ui.Hex("#36332e")
	t.SurfacePressed = ui.Hex("#403c36")
	t.Border = ui.Hex("#504b43")
	t.Text = ui.Hex("#eeeae2")
	t.TextMuted = ui.Hex("#bdb6aa")
	t.Accent = accent
	t.AccentHover = accent.Mix(white, 0.12)
	t.AccentPressed = accent.Mix(black, 0.12)
	t.AccentText = ui.Hex("#291b14")
	t.Danger = ui.Hex("#e58f95")
	// Muted semantic colors stay readable without overpowering the accent.
	t.Warning = ui.Hex("#e6b567")
	t.Success = ui.Hex("#79bea1")
	t.Selection = accent.Alpha(0.24)
	t.Focus = accent.Alpha(0.8)
	t.Scrollbar = t.Text.Alpha(.35)
	t.Radius = 8
	return t
}

// Palette returns Onevra's dark palette when dark is set, and its light
// palette otherwise. It ignores the desktop's preferences; see Onevra.
func Palette(dark bool) *ui.Theme {
	if dark {
		return DarkPalette()
	}
	return LightPalette()
}

// Onevra returns the palette matching the frame's light or dark appearance,
// which follows mygo.Theme's source. Like MyGo's default theme, it follows the
// desktop's text scale and higher contrast; it keeps Onevra's own accent
// rather than the desktop's.
//
// Its signature fits a Shell's Theme option. Windows a Shell does not create
// install it with Apply.
func Onevra(c *ui.Context) *ui.Theme {
	t := Palette(c.Theme().Dark)
	p := c.Preferences()
	// The same adjustments as MyGo's default theme.
	if p.HighContrast {
		t.Border = t.Border.Mix(t.Text, 0.45)
		t.TextMuted = t.TextMuted.Mix(t.Text, 0.4)
		t.Focus.A = 255
		t.Scrollbar = t.Scrollbar.Alpha(1.6)
	}
	if s := p.TextScale; s > 0 && s != 1 {
		t.FontSize *= s
	}
	return t
}

// Apply installs Onevra for the current frame. Call it at the start of the
// root content builder, before creating any widgets: SetTheme only affects
// the window root and the widgets created after the call. MyGo reruns the
// frame when the appearance or the desktop's preferences change.
func Apply(c *ui.Context) { c.SetTheme(Onevra(c)) }
