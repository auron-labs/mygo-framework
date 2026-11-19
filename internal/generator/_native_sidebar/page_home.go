package main

import (
	"mygo-framework/locales"

	"github.com/auron-labs/mygo-framework/v2/onevra/components"
	"github.com/egoist/mygo/ui"
)

// homePage greets the user. It reads the shared state and nothing else.
func homePage(c *ui.Context, s *screen) {
	p := s.prefs.Get()
	components.Text(c, locales.HomeWelcome(appName)).FontSize(22).Bold().Wrap()
	components.Text(c, locales.HomeHint()).Wrap()
	components.Text(c, locales.HomeProfile(p.Profile)).Wrap()
}
