package main

import (
	"mygo-framework/locales"

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
	all := routes()
	for _, r := range all {
		if r.id == s.page {
			return r
		}
	}
	return all[0]
}

// navigate selects a page by navigation id.
func (s *screen) navigate(id string) { s.page = id }
