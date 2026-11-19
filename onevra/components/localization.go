package components

import (
	"math"
	"time"

	"github.com/auron-labs/mygo-framework/v2/onevra/internal/l10n"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

// localization holds the text of component internals from Onevra's catalog
// in the language the window shows (ui.Context.Locale); applications change
// it with "onevra.<key>" messages in their MyGo catalogs. Dates and the first
// weekday follow the user's regional formats, as MyGo's date controls do.
// Weekdays are Sunday-first.
type localization struct {
	Minimum, Maximum          string
	Today                     string
	ChooseMonth               string
	ChooseYear                string
	PreviousMonth             string
	NextMonth                 string
	PreviousYear              string
	NextYear                  string
	PreviousYears             string
	NextYears                 string
	MonthSelection            string
	YearSelection             string
	ChooseTime                string
	Hours                     string
	Minutes                   string
	Divider                   string
	SaturationBrightness      string
	Hue                       string
	Opacity                   string
	ColorFormat               string
	CopyColor                 string
	Red                       string
	Green                     string
	Blue                      string
	HexColor                  string
	OpacityPercent            string
	HueDegrees                string
	SaturationPercent         string
	BrightnessPercent         string
	LightnessPercent          string
	Settings                  string
	SearchSettings            string
	SearchSettingsPlaceholder string
	SettingsCategory          string
	SearchResults             string
	NoSettings                string
	CloseSettings             string
	CloseToast                string
	HoldDone                  string
	Weekdays                  [7]string

	messages     l10n.Messages
	region       i18n.Region
	formatLocale string
}

func localized(c *ui.Context) localization {
	var words localization
	m := l10n.In(c.Locale())
	for _, field := range []struct {
		value *string
		key   string
	}{
		{&words.Minimum, "minimum"},
		{&words.Maximum, "maximum"},
		{&words.Today, "today"},
		{&words.ChooseMonth, "chooseMonth"},
		{&words.ChooseYear, "chooseYear"},
		{&words.PreviousMonth, "previousMonth"},
		{&words.NextMonth, "nextMonth"},
		{&words.PreviousYear, "previousYear"},
		{&words.NextYear, "nextYear"},
		{&words.PreviousYears, "previousYears"},
		{&words.NextYears, "nextYears"},
		{&words.MonthSelection, "monthSelection"},
		{&words.YearSelection, "yearSelection"},
		{&words.ChooseTime, "chooseTime"},
		{&words.Hours, "hours"},
		{&words.Minutes, "minutes"},
		{&words.Divider, "divider"},
		{&words.SaturationBrightness, "saturationBrightness"},
		{&words.Hue, "hue"},
		{&words.Opacity, "opacity"},
		{&words.ColorFormat, "colorFormat"},
		{&words.CopyColor, "copyColor"},
		{&words.Red, "red"},
		{&words.Green, "green"},
		{&words.Blue, "blue"},
		{&words.HexColor, "hexColor"},
		{&words.OpacityPercent, "opacityPercent"},
		{&words.HueDegrees, "hueDegrees"},
		{&words.SaturationPercent, "saturationPercent"},
		{&words.BrightnessPercent, "brightnessPercent"},
		{&words.LightnessPercent, "lightnessPercent"},
		{&words.Settings, "settings"},
		{&words.SearchSettings, "searchSettings"},
		{&words.SearchSettingsPlaceholder, "searchSettingsPlaceholder"},
		{&words.SettingsCategory, "settingsCategory"},
		{&words.SearchResults, "searchResults"},
		{&words.NoSettings, "noSettings"},
		{&words.CloseSettings, "closeSettings"},
		{&words.CloseToast, "closeToast"},
		{&words.HoldDone, "holdDone"},
	} {
		*field.value = m.Text(field.key)
	}
	words.messages = m
	words.region, words.formatLocale = formats(c)
	words.Weekdays = i18n.Weekdays(words.formatLocale, i18n.Short, true)
	return words
}

// formats returns the user's regional formats and the locale that writes
// dates in them and the window's language, as MyGo's date controls do. Where
// the system reports none, as in a headless Tester, CLDR's for the language.
func formats(c *ui.Context) (i18n.Region, string) {
	lang := c.Locale()
	if lang == "" {
		lang = "en"
	}
	r := c.Region()
	if r.HourCycle != 12 && r.HourCycle != 24 {
		r = i18n.RegionFor(lang)
		r.Locale = ""
	}
	return r, r.FormatLocale(lang)
}

// Date presentations, named by Go layouts: "January 2006", "January 2, 2006",
// "Jan" and "2006-01-02".
var dateSkeletons = map[string]string{"January 2006": "yMMMM", "January 2, 2006": "yMMMMd", "2006-01-02": "yMd"}

func (l localization) date(value time.Time, layout string) string {
	if layout == "Jan" {
		return i18n.Months(l.formatLocale, i18n.Abbreviated, true)[value.Month()-1]
	}
	if skeleton, ok := dateSkeletons[layout]; ok {
		return i18n.FormatDate(l.formatLocale, value, i18n.DateOptions{Skeleton: skeleton})
	}
	return value.Format(layout)
}

// numberAction names a numeric editor's "adjust", "increase" or "decrease"
// control after the application-supplied field label.
func (l localization) numberAction(action, label string) string {
	key := map[string]string{"adjust": "numberAdjust", "increase": "numberIncrease"}[action]
	if key == "" {
		key = "numberDecrease"
	}
	return l.messages.Label(key, label)
}

// holdHint describes how to complete a hold of d, rounded to a tenth of a
// second.
func (l localization) holdHint(d time.Duration) string {
	return l.messages.Format("holdHint", map[string]any{"seconds": math.Round(d.Seconds()*10) / 10})
}

func (l localization) firstWeekday() time.Weekday { return l.region.FirstWeekday }
