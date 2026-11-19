package main

import (
	"mygo-framework/locales"
	"strings"

	"github.com/auron-labs/mygo-framework/onevra/icons/lucide"
	"github.com/egoist/mygo/ui"
)

// Navigation ids. Add a page by adding an id, a view file and a route below.
const (
	pageHome     = "home"
	pageImport   = "import"
	pageSettings = "settings"
)

// route connects a navigation id to its page view. Pages receive the shared
// state and never call each other; only this table knows them all.
type route struct {
	id    string
	title func() string
	icon  func() *ui.SVG
	// inNav puts the route in the navigation list; the others are reached
	// from the sidebar footer.
	inNav bool
	// count is the optional badge number; zero shows no badge.
	count func(*screen) int
	view  func(*ui.Context, *screen)
}

func routes() []route {
	return []route{
		{id: pageHome, title: locales.NavHome, icon: lucide.House, inNav: true, view: homePage},
		{id: pageImport, title: locales.NavImport, icon: lucide.FolderInput, inNav: true, count: activeImports, view: importPage},
		{id: pageSettings, title: locales.NavSettings, icon: lucide.Settings, view: settingsPage},
	}
}

// current returns the selected route; an unknown id selects Home.
func (s *screen) current() route {
	id := s.kept.page
	if s.onPhone {
		id = s.syncPhoneRouter()
	}
	all := routes()
	for _, r := range all {
		if r.id == id {
			return r
		}
	}
	return all[0]
}

// restoreNotice explains an import that was running when MyGo last kept state,
// opening Import so the notice is visible.
func (s *screen) restoreNotice() {
	if s.restoreImportNotice() {
		s.navigate(pageImport)
	}
}

// navigate selects a page by navigation id. On a phone the pages are a
// ui.Router's history: a navigation route replaces the page, as a tab does, and
// any other route is pushed above it, so the system's back and an iOS edge
// swipe return to the page before.
func (s *screen) navigate(id string) {
	s.kept.page = id
	if !s.onPhone {
		return
	}
	s.syncPhoneRouter()
	for _, r := range routes() {
		if r.id != id {
			continue
		}
		if r.inNav {
			s.router.Replace("/" + id)
		} else {
			s.router.Push("/" + id)
		}
	}
}

// syncPhoneRouter returns the id of the page the phone's router shows, and
// records it as the page MyGo keeps. A launch that restores a page starts the
// router there; the system's back moves the router without calling navigate.
func (s *screen) syncPhoneRouter() string {
	if s.router == nil {
		s.router = ui.NewRouter("/" + s.kept.page)
	}
	s.kept.page = strings.TrimPrefix(s.router.Path(), "/")
	return s.kept.page
}
