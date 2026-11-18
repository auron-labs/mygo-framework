// Package plain provides a single-content native window without the default
// title bar. The window title is shown in the draggable header instead, so it
// stays visible while the shell owns only presentation. Applications own
// content, localization, state and lifetime.
package plain

import (
	"runtime"

	"github.com/auron-labs/mygo-framework/onevra/internal/l10n"

	"github.com/auron-labs/mygo-framework/onevra/components"
	"github.com/auron-labs/mygo-framework/onevra/icons/lucide"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// ShellOptions configures a single-content window. Builders and Shell methods
// run synchronously on the UI thread, after MyGo is ready.
type ShellOptions struct {
	// Title names the window and is shown in the shell's header. Later
	// Window().SetTitle calls update both the native title and the header.
	Title string
	// Dimensions are outer DIPs. Defaults are 760x560 with a 360x300 minimum.
	Width, Height, MinWidth, MinHeight int
	// StateKey, when non-empty, has MyGo remember the window's bounds and
	// maximized or full-screen state under this key and restore them at
	// creation. MyGo stores them in its user-data directory, or in the store
	// set with mygo.App.SetWindowStateStore. Use a distinct key per window kind.
	StateKey string
	// Hidden allows the application to install lifecycle handlers before showing.
	Hidden bool
	// HeaderLeading builds optional leading header content, such as a segmented
	// tab list. It replaces the header title, which remains the native window
	// title; build a title inside it if the header should also show one. It runs
	// on the UI thread and must not start background work.
	HeaderLeading func(*ui.Context)
	// HeaderActions builds optional trailing controls at the right of the
	// header, before the Windows caption buttons, such as a Settings action. It
	// runs on the UI thread and must not start background work.
	HeaderActions func(*ui.Context)
	// Content is automatically inset below the draggable window-control area.
	// Dialogs inherit its drag strip and remain relative to the whole window.
	Content func(*ui.Context)
	// ContentBehindHeader extends Content to the top of the window, behind
	// the unpainted header, such as for content that scrolls under it. Use
	// HeaderHeight(c) as top padding to keep controls below the header.
	ContentBehindHeader bool
	// Theme, when set, returns the frame's theme; Shell installs it before
	// building anything, so its own surfaces and all content use it. It runs
	// on every frame, so it can follow application state; pass theme.Onevra
	// for Onevra's palettes. Nil keeps MyGo's frame theme. Light or dark
	// belongs to mygo.Theme.SetSource, which also covers native chrome.
	Theme func(*ui.Context) *ui.Theme
	// Toasts places the window's toasts; top placements also clear the
	// window-control area.
	Toasts components.ToastOptions
}

// Shell owns presentation only; Window exposes MyGo's original lifecycle API.
type Shell struct {
	window  *mygo.Window
	host    windowHost
	options ShellOptions
}

type windowHost interface {
	Invalidate()
	Minimize()
	ToggleMaximize()
	IsMaximized() bool
	Close()
	Title() string
}

// NewShell hides the standard title bar, retaining native macOS/Linux window
// controls and composing Onevra caption buttons on Windows. It does not show a
// title or add navigation, persistence, or application lifecycle policies;
// StateKey delegates window geometry to MyGo.
func NewShell(options ShellOptions) *Shell {
	s := newShell(options)
	s.window = mygo.NewWindow(s.nativeOptions())
	s.host = s.window
	if runtime.GOOS == "windows" {
		s.window.OnMaximize(s.refresh)
		s.window.OnUnmaximize(s.refresh)
	}
	return s
}

func newShell(o ShellOptions) *Shell {
	if o.Width <= 0 {
		o.Width = 760
	}
	if o.Height <= 0 {
		o.Height = 560
	}
	if o.MinWidth <= 0 {
		o.MinWidth = 360
	}
	if o.MinHeight <= 0 {
		o.MinHeight = 300
	}
	o.Width, o.Height = max(o.Width, o.MinWidth), max(o.Height, o.MinHeight)
	return &Shell{options: o}
}

func (s *Shell) nativeOptions() mygo.WindowOptions {
	o := s.options
	return mygo.WindowOptions{
		Title: o.Title, Width: o.Width, Height: o.Height, MinWidth: o.MinWidth, MinHeight: o.MinHeight, Hidden: o.Hidden, StateKey: o.StateKey,
		TitleBarStyle: mygo.TitleBarHidden, HideWindowButtons: runtime.GOOS == "windows",
		TrafficLightPosition: &mygo.Point{X: 16, Y: 16}, Content: ui.View(s.view),
	}
}

// Window exposes native events and operations. Do not replace its content.
func (s *Shell) Window() *mygo.Window { return s.window }

// SetToasts changes toast placement without recreating the window or toasts.
func (s *Shell) SetToasts(options components.ToastOptions) { s.options.Toasts = options; s.refresh() }

func (s *Shell) refresh() {
	if s.host != nil {
		s.host.Invalidate()
	}
}

// currentTitle returns the window's current title so the header follows
// SetTitle; before the host exists it falls back to the construction value.
func (s *Shell) currentTitle() string {
	if s.host != nil {
		return s.host.Title()
	}
	return s.options.Title
}

func (s *Shell) headerTitle(c *ui.Context) {
	title := s.currentTitle()
	if title == "" {
		ui.Spacer(c)
		return
	}
	ui.Row(c).Key("header-title").Grow(1).MinWidth(0).FillHeight().AlignItems(ui.Center).Clip().Children(func() {
		components.Text(c, title).Bold().SingleLine().Ellipsis("…")
	})
}

// HeaderHeight is the frame's native-control clearance and modal drag height.
// Shell insets content by this amount unless ContentBehindHeader is set.
func HeaderHeight(c *ui.Context) float32 { return max(48, c.TitleBar().Height, c.Theme().Rem(3)) }

func (s *Shell) view(c *ui.Context) {
	if s.options.Theme != nil {
		c.SetTheme(s.options.Theme(c))
	}
	height := HeaderHeight(c)
	inset := height
	if s.options.ContentBehindHeader {
		inset = 0
	}
	components.WithDialogWindowDrag(c, height, func() {
		ui.Column(c).Fill().Background(c.Theme().Background).Clip().Children(func() {
			ui.Column(c).Key("content").Fill().Padding(inset, 0, 0, 0).Children(func() {
				if s.options.Content != nil {
					s.options.Content(c)
				}
			})
			bar := c.TitleBar()
			// Leave a gap beyond the macOS window controls so the title does not
			// sit against the traffic lights.
			leftInset := max(8, bar.Left)
			if runtime.GOOS == "darwin" && bar.Left > 0 {
				leftInset += 16
			}
			ui.Row(c).Key("header").Absolute().Top(0).Left(0).Right(0).Height(height).
				Padding(6, max(8, bar.Right), 6, leftInset).AlignItems(ui.Center).DragWindow().Children(func() {
				if s.options.HeaderLeading != nil {
					ui.Row(c).Key("header-leading").Grow(1).MinWidth(0).FillHeight().AlignItems(ui.Center).Clip().Children(func() {
						s.options.HeaderLeading(c)
					})
				} else {
					s.headerTitle(c)
				}
				if s.options.HeaderActions != nil {
					ui.Row(c).Key("header-actions").Shrink(0).Gap(4).Children(func() { s.options.HeaderActions(c) })
				}
				if runtime.GOOS == "windows" {
					s.captionButtons(c)
				}
			})
		})
	})
	toasts := s.options.Toasts
	if toasts.Placement.Top() {
		toasts.Inset += height
	}
	components.Toasts(c, toasts)
}

func captionButton(c *ui.Context, icon *ui.SVG, name string) ui.Element {
	size := max(32, c.Theme().Rem(2))
	return components.Button(c, "", components.ButtonGhost).Label(name).Size(size, size).Padding(0).Children(func() {
		components.Icon(c, icon).Size(c.Theme().Rem(.875), c.Theme().Rem(.875))
	})
}

func (s *Shell) captionButtons(c *ui.Context) {
	w, m := s.host, l10n.In(c.Locale())
	ui.Row(c).Key("window-controls").Shrink(0).Gap(4).Children(func() {
		if captionButton(c, lucide.Minus(), m.Text("minimizeWindow")).Clicked() && w != nil {
			w.Minimize()
		}
		icon, name := lucide.Square(), m.Text("maximizeWindow")
		if w != nil && w.IsMaximized() {
			icon, name = lucide.Copy(), m.Text("restoreWindow")
		}
		if captionButton(c, icon, name).MaximizeButton().Clicked() && w != nil {
			w.ToggleMaximize()
		}
		if captionButton(c, lucide.X(), m.Text("closeWindow")).Clicked() && w != nil {
			w.Close()
		}
	})
}
