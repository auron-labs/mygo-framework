package sidebar

import (
	"strconv"

	"github.com/auron-labs/mygo-framework/v2/onevra/internal/l10n"
	"github.com/auron-labs/mygo-framework/v2/onevra/internal/phone"

	"github.com/auron-labs/mygo-framework/v2/onevra/components"
	"github.com/auron-labs/mygo-framework/v2/onevra/icons/lucide"
	"github.com/egoist/mygo/ui"
)

// Tab is one destination of the phone's bottom tab bar, as the sidebar's
// navigation items are on a desktop. ShellOptions.Tabs builds them per frame,
// so selection and badges follow the application's route state.
type Tab struct {
	// Icon draws above the label, sized by the bar, as a Lucide glyph.
	Icon *ui.SVG
	// Label names the destination, shown under the icon and to tools.
	Label string
	// Selected marks the destination the application is on; the bar
	// highlights it. While no tab is selected, the shell shows Content as a
	// pushed screen with back.
	Selected bool
	// Badge counts what is new at the destination; zero shows none.
	Badge int
	// OnSelect runs when the user taps the tab.
	OnSelect func()
}

// keyboardCoversBar is the window's bottom inset that tells a keyboard rather
// than the system's bar, as MyGo's ui/mobile.BottomBar tells it: the bar hides
// while one covers it.
const keyboardCoversBar = 120

// phoneView builds the phone's window: the system owns its chrome, so there is
// no header to drag, no caption buttons and no sidebars to resize. The routes
// the sidebar lists become tabs in a bottom bar, and the rest shows as pushed
// screens with back. MyGo's root already keeps content inside the safe area and
// fills the bars with the background, so only what draws over them applies
// Context.SafeArea itself: the top bar reaches into the status bar's room, the
// tab bar under the home indicator, and the toasts at their edge.
func (s *Shell) phoneView(c *ui.Context) {
	background := c.Theme().Background.Alpha(1)
	tabs := s.phoneTabs(c)
	pushed := len(tabs) > 0 && !anyTabSelected(tabs)
	// No Clip: the tab bar's background extends past the column's box, under
	// the home indicator.
	root := ui.Column(c).Fill().Background(background)
	root.Children(func() {
		if s.page != nil {
			ui.Column(c).Key(true).Fill().Clip().Background(background).Children(func() { s.page(c) })
			return
		}
		ui.Column(c).Key("content").Grow(1).MinHeight(0).FillWidth().Clip().Children(func() {
			if s.options.Content != nil {
				s.options.Content(c)
			}
		})
		if !pushed && len(tabs) > 0 {
			s.phoneTabBar(c, tabs)
		}
	})
	if s.page == nil {
		s.phoneTopBar(c, pushed)
	}
	components.Toasts(c, phone.ToastOptions(c, s.options.Toasts))
}

// PhoneView returns the phone layout of a shell with options, without a
// window, for headless tests of an application's phone screens. It draws the
// phone layout on every system.
func PhoneView(options ShellOptions) func(*ui.Context) {
	s := newShell(options)
	return func(c *ui.Context) {
		if s.options.Theme != nil {
			c.SetTheme(s.options.Theme(c))
		}
		s.phoneView(c)
	}
}

// phoneTabs reads the destinations of the frame, nil without the option.
func (s *Shell) phoneTabs(c *ui.Context) []Tab {
	if s.options.Tabs == nil {
		return nil
	}
	return s.options.Tabs(c)
}

// anyTabSelected reports whether one of the tabs is the route shown, whose
// absence tells a pushed screen instead.
func anyTabSelected(tabs []Tab) bool {
	for _, tab := range tabs {
		if tab.Selected {
			return true
		}
	}
	return false
}

// phoneTopBar floats over the content, as the desktop's header does, so
// content keeps one clearance, HeaderHeight, on every platform. It paints from
// the top of the window through the status bar's room, and its controls sit
// below the safe area. A pushed screen shows the back button instead of the
// top actions.
func (s *Shell) phoneTopBar(c *ui.Context, pushed bool) {
	back := pushed && s.options.OnBack != nil
	actions := !pushed && s.options.TopActions != nil
	if !back && !actions && s.options.ContentTitle == nil {
		return
	}
	t := c.Theme()
	top, right, _, left := c.SafeArea()
	height := HeaderHeight(c)
	ui.Row(c).Key("top-bar").Absolute().Top(0).Left(0).Right(0).Height(top+height).
		AlignItems(ui.Center).Background(t.Background.Alpha(1)).
		Padding(top, max(4, right), 4, max(4, left)).Children(func() {
		if back {
			b := iconButton(c, lucide.ChevronLeft(), l10n.In(c.Locale()).Text("goBack"), max(20, t.Rem(1.25)))
			// The system's back, as Android's button and the Back key, takes
			// the pushed screen away too: asking for the key this frame tells
			// MyGo the window takes it.
			key := c.Shortcut(0, ui.KeyBack)
			if b.Clicked() || key {
				s.options.OnBack()
			}
		}
		if s.options.ContentTitle != nil {
			ui.Row(c).Key("top-title").Grow(1).MinWidth(0).FillHeight().AlignItems(ui.Center).Clip().
				Padding(0, 8, 0, 8).Children(func() { s.options.ContentTitle(c) })
		} else {
			ui.Row(c).Grow(1)
		}
		if actions {
			ui.Row(c).Key("top-actions").Shrink(0).Gap(4).Children(func() { s.options.TopActions(c) })
		}
	})
}

// phoneTabBar draws the bottom tab bar in Onevra's style: MyGo's
// ui/mobile.BottomBar layout with Lucide icons and the theme's colors, in the
// one window, so what the app draws — headless tests and tools included — sees
// it. The bar's background reaches the screen's edge under the home indicator,
// and it hides while the keyboard covers it, as the system's bar does.
func (s *Shell) phoneTabBar(c *ui.Context, tabs []Tab) {
	t := c.Theme()
	_, _, bottom, _ := c.SafeArea()
	bar := ui.Row(c).Key("tab-bar").FillWidth().Shrink(0).Background(t.Surface).
		Padding(0, 0, bottom, 0).Margin(0, 0, -bottom, 0).Role(ui.RoleTabList)
	if bottom > keyboardCoversBar {
		bar.Hidden(true)
		return
	}
	height := max(52, t.Rem(3.5))
	bar.Children(func() {
		for i, tab := range tabs {
			color := t.TextMuted
			if tab.Selected {
				color = t.Accent
			}
			item := ui.Column(c).Key(i).Grow(1).Basis(0).Height(height).
				AlignItems(ui.Center).Justify(ui.Center).Gap(2).
				Role(ui.RoleTab).Label(tab.Label).Checked(tab.Selected)
			item.Children(func() {
				components.Icon(c, tab.Icon).Size(t.Rem(1.5), t.Rem(1.5)).TextColor(color)
				if tab.Badge > 0 {
					components.Badge(c, strconv.Itoa(tab.Badge), components.BadgeOptions{Tone: components.BadgeAccent}).
						Absolute().Top(-6).Right(-14)
				}
				components.Text(c, tab.Label).FontSize(t.Rem(.75)).TextColor(color).NoWrap()
			})
			if item.Clicked() && !tab.Selected && tab.OnSelect != nil {
				tab.OnSelect()
			}
		}
	})
}
