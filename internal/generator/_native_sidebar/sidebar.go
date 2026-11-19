package main

import (
	"mygo-framework/locales"
	"strconv"

	"github.com/auron-labs/mygo-framework/v2/jobs"

	"github.com/auron-labs/mygo-framework/v2/onevra/components"
	"github.com/auron-labs/mygo-framework/v2/onevra/icons/lucide"
	"github.com/auron-labs/mygo-framework/v2/onevra/shells/sidebar"
	"github.com/auron-labs/mygo-framework/v2/onevra/theme"
	"github.com/egoist/mygo/ui"
)

// shellOptions describes the window. The window restores its size and position
// from StateKey; the defaults apply on the first launch. Tabs, TopActions and
// OnBack carry the phone's navigation; on desktops the sidebar carries it and
// the shell ignores them.
func (s *screen) shellOptions() sidebar.ShellOptions {
	return sidebar.ShellOptions{
		Title: appName, Hidden: true, StateKey: "main", Theme: theme.Onevra,
		Width: defaultWidth, Height: defaultHeight, MinWidth: minWidth, MinHeight: minHeight,
		LeftWidth: defaultLeftWidth, LeftMinWidth: railWidth,
		Left: s.left, LeftToolbar: s.footer, ContentTitle: s.contentTitle, Content: s.content,
		Tabs: s.tabs, TopActions: s.topActions, OnBack: s.back,
	}
}

// view lays out the same regions without a window, for the headless tests and
// the MCP fixture: the phone layout on a phone, otherwise the sidebar, whose
// width comes from s.host, as in the window.
func (s *screen) view(c *ui.Context) {
	if s.onPhone {
		if s.phoneUI == nil {
			s.phoneUI = sidebar.PhoneView(s.shellOptions())
		}
		s.phoneUI(c)
		return
	}
	if s.host == nil {
		s.host = &staticLeft{width: defaultLeftWidth}
	}
	ui.Row(c).Fill().Children(func() {
		ui.Column(c).Width(s.host.LeftWidth()).Shrink(0).FillHeight().Clip().Children(func() {
			ui.Column(c).FillWidth().Grow(1).MinHeight(0).Clip().Children(func() { s.left(c) })
			ui.Column(c).FillWidth().Shrink(0).Padding(8).Children(func() { s.footer(c) })
		})
		ui.Column(c).Grow(1).MinWidth(0).FillHeight().Children(func() {
			ui.Row(c).Absolute().Top(0).Left(16).Height(sidebar.HeaderHeight(c)).AlignItems(ui.Center).Children(func() { s.contentTitle(c) })
			s.content(c)
		})
	})
}

// left is the navigation. It runs on the shell's frame, so it also keeps the
// sidebar's width in step with the window (see syncRail).
func (s *screen) left(c *ui.Context) {
	rail := s.syncRail(c)
	ui.Column(c).FillWidth().Padding(sidebar.HeaderHeight(c), 0, 0, 0).Children(func() {
		components.Sidebar(c).Children(func() {
			items := func() {
				for _, r := range routes() {
					if r.inNav {
						s.navItem(c, r, rail)
					}
				}
			}
			if rail {
				items()
			} else {
				components.SidebarSection(c, locales.NavigationLabel(), items)
			}
		})
	})
}

// navItem is one navigation entry: icon, label and optional count badge, or
// only the icon, with the label as tooltip, in the rail.
func (s *screen) navItem(c *ui.Context, r route, rail bool) {
	label := r.title()
	selected := s.current().id == r.id
	count := 0
	if r.count != nil {
		count = r.count(s)
	}
	var item ui.Element
	if rail {
		item = railItem(c, label, r.icon(), selected, count)
	} else {
		item = components.SidebarItemWithSlots(c, label, selected,
			func(c *ui.Context, size float32) {
				components.Icon(c, r.icon()).Size(size, size).TextColor(c.Theme().Accent)
			},
			badge(count))
	}
	// The accessible label differs from the visible text so that tools can tell
	// the button from its text.
	item.Label(locales.NavGoTo(label))
	if item.Clicked() {
		s.navigate(r.id)
	}
}

// badge returns a trailing slot showing count, or nil for none.
func badge(count int) func(*ui.Context, float32) {
	if count <= 0 {
		return nil
	}
	return func(c *ui.Context, _ float32) {
		components.Badge(c, strconv.Itoa(count), components.BadgeOptions{Tone: components.BadgeAccent})
	}
}

// railItem is an icon-only button. It keeps its label for accessibility and
// shows it as a tooltip.
func railItem(c *ui.Context, label string, icon *ui.SVG, selected bool, count int) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Key(label).FillWidth().Justify(ui.Center).AlignItems(ui.Center).
		Padding(t.Space(2)).Gap(t.Space(1)).Radius(t.Radius).Selected(selected).
		Background(ui.Color{}).TextColor(t.Text)
	if selected {
		b.Background(t.Text.Alpha(.1))
	}
	b.Children(func() {
		size := t.Space(4.5)
		components.Icon(c, icon).Size(size, size).TextColor(t.Accent)
		if count > 0 {
			components.Badge(c, strconv.Itoa(count), components.BadgeOptions{Tone: components.BadgeAccent})
		}
	})
	components.Tooltip(c, b, label)
	return b
}

// footer is the sidebar's bottom area: the update prompt, account card,
// Settings and the version.
func (s *screen) footer(c *ui.Context) {
	rail := s.compactWindow(c)
	p := s.prefs.Get()
	t := c.Theme()
	ui.Column(c).FillWidth().Gap(t.Space(2)).Children(func() {
		if rail {
			s.railFooter(c, p.Profile)
			return
		}
		s.updateView(c)
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(2)).Padding(t.Space(1), t.Space(2)).Children(func() {
			components.Avatar(c, p.Profile, components.AvatarOptions{})
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				components.Text(c, locales.AccountName(p.Profile)).Bold().SingleLine().Ellipsis("…")
				components.Text(c, locales.AccountCaption()).TextColor(t.TextMuted).SingleLine().Ellipsis("…")
			})
		})
		s.settingsItem(c, false)
		components.Text(c, locales.VersionLabel(s.version)).TextColor(t.TextMuted).Padding(t.Space(1), t.Space(2))
	})
}

func (s *screen) railFooter(c *ui.Context, profile string) {
	if s.update != nil {
		b := sidebar.IconButton(c, lucide.Download(), locales.RailUpdateAvailable())
		if b.Clicked() {
			if status, started := s.jobs.Status(updateInstallJob); !started || status.State == jobs.Failed {
				s.installUpdate()
			}
		}
	}
	ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
		components.Tooltip(c, components.Avatar(c, profile, components.AvatarOptions{}), locales.AccountName(profile))
	})
	s.settingsItem(c, true)
}

// settingsItem selects the Settings page.
func (s *screen) settingsItem(c *ui.Context, rail bool) {
	for _, r := range routes() {
		if r.id == pageSettings {
			s.navItem(c, r, rail)
		}
	}
}

// tabs lists the phone's tab bar: the sidebar's navigation routes, whose
// selection follows the current page. A footer route leaves none chosen, which
// the shell shows as a pushed screen with back.
func (s *screen) tabs(_ *ui.Context) []sidebar.Tab {
	current := s.current().id
	var tabs []sidebar.Tab
	for _, r := range routes() {
		if !r.inNav {
			continue
		}
		count := 0
		if r.count != nil {
			count = r.count(s)
		}
		tabs = append(tabs, sidebar.Tab{
			Icon: r.icon(), Label: r.title(), Selected: r.id == current, Badge: count,
			OnSelect: func() { s.navigate(r.id) },
		})
	}
	return tabs
}

// topActions opens the footer routes from the phone's top bar, as the sidebar's
// footer does on a desktop.
func (s *screen) topActions(c *ui.Context) {
	for _, r := range routes() {
		if r.inNav {
			continue
		}
		if sidebar.IconButton(c, r.icon(), locales.NavGoTo(r.title())).Clicked() {
			s.navigate(r.id)
		}
	}
}

// back returns from a pushed footer route to the page before it. A launch that
// restores a footer route has no page before, and returns to Home.
func (s *screen) back() {
	s.syncPhoneRouter()
	if s.router.CanGoBack() {
		s.router.Back()
		return
	}
	s.navigate(routes()[0].id)
}

// compactWindow reports whether the window is narrower than the breakpoint.
func (s *screen) compactWindow(c *ui.Context) bool {
	width, _ := c.Size()
	return width < railBreakpoint
}

// contentTitle is the top bar slot: the selected page's title.
func (s *screen) contentTitle(c *ui.Context) {
	components.Text(c, s.current().title()).Bold().SingleLine()
}

// content builds the selected page below the top bar. On a phone the router
// moves between pages, so a slide shows two of them at once.
func (s *screen) content(c *ui.Context) {
	if !s.onPhone {
		s.page(c, s.current())
		return
	}
	s.syncPhoneRouter()
	s.router.View(c, func(r *ui.Route) {
		page := routes()[0]
		for _, candidate := range routes() {
			if r.Match("/" + candidate.id) {
				page = candidate
			}
		}
		r.Title(page.title())
		s.page(c, page)
	})
}

// page builds the page r in a scrolling column.
func (s *screen) page(c *ui.Context, r route) {
	gap := float32(24)
	if s.prefs.Get().Compact {
		gap = 8
	}
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).FillWidth().Padding(sidebar.HeaderHeight(c)+16, 24, 24, 24).Gap(gap).Children(func() { r.view(c, s) })
	})
}
