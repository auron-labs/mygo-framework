package main

import (
	"mygo-framework/locales"

	"github.com/auron-labs/mygo-framework/errorreport"

	"github.com/auron-labs/mygo-framework/onevra/components"
	"github.com/egoist/mygo/ui"
)

// settingsPage edits the typed preferences. Controls bind to local copies and
// commit through the cell, which the autosaver persists.
func settingsPage(c *ui.Context, s *screen) {
	p := s.prefs.Get()
	compact := p.Compact
	components.Checkbox(c, &compact, locales.CompactSpacing()).Label(locales.CompactSpacingLabel()).OnChange(func() {
		s.prefs.Update(func(p *preferences) { p.Compact = compact })
	})
	units := p.Units
	ui.RadioGroup(c, func() {
		for _, option := range []struct{ value, label string }{{unitsMetric, locales.UnitsMetric()}, {unitsImperial, locales.UnitsImperial()}} {
			components.Radio(c, &units, option.value, option.label).OnChange(func() {
				s.prefs.Update(func(p *preferences) { p.Units = units })
			})
		}
	}).Label(locales.UnitsLabel())
	appearance := p.Theme
	ui.RadioGroup(c, func() {
		for _, option := range []struct{ value, label string }{{themeSystem, locales.ThemeSystem()}, {themeLight, locales.ThemeLight()}, {themeDark, locales.ThemeDark()}} {
			components.Radio(c, &appearance, option.value, option.label).OnChange(func() {
				s.prefs.Update(func(p *preferences) { p.Theme = appearance })
			})
		}
	}).Label(locales.ThemeLabel())
	if errorreport.Configured() {
		reports := p.ErrorReports
		components.Checkbox(c, &reports, locales.ErrorReports()).Label(locales.ErrorReportsLabel()).OnChange(func() {
			s.prefs.Update(func(p *preferences) { p.ErrorReports = reports })
		})
	}
	components.Text(c, locales.PreferencesSavedAutomatically()).Wrap()
	if s.saver.Err().Get() != nil {
		components.Text(c, locales.PreferencesSaveFailed()).Wrap()
	}
	if components.Button(c, locales.SaveSupportBundle(), components.ButtonSecondary).Clicked() {
		s.saveSupportBundle()
	}
}
