package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"github.com/auron-labs/mygo-framework/profiles"
	"github.com/auron-labs/mygo-framework/settings"

	// Validating time zones must not depend on the host's zoneinfo database.
	_ "time/tzdata"
)

var (
	compactKey      = settings.NewKey("native.compact", false, nil)
	timezoneKey     = settings.NewKey("native.timezone", "UTC", validateTimezone)
	unitsKey        = settings.NewKey("native.units", unitsMetric, oneOf(unitsMetric, unitsImperial))
	themeKey        = settings.NewKey("native.theme", themeSystem, oneOf(themeSystem, themeLight, themeDark))
	profileKey      = settings.NewKey("native.profile", "Default", profiles.Validate)
	importFolderKey = settings.NewKey("native.import_folder", "", validateImportFolder)
	// errorReportsKey only matters to builds with a Sentry DSN.
	errorReportsKey = settings.NewKey("native.error_reports", true, nil)
)

const (
	unitsMetric   = "metric"
	unitsImperial = "imperial"
	themeSystem   = "system"
	themeLight    = "light"
	themeDark     = "dark"
)

// preferences is the typed set of values persisted by the application.
type preferences struct {
	Compact      bool
	Timezone     string
	Units        string
	Theme        string
	Profile      string
	ImportFolder string
	ErrorReports bool
}

func loadPreferences(ctx context.Context, values *settings.Values) (preferences, error) {
	var p preferences
	var errs [7]error
	p.Compact, errs[0] = settings.Get(ctx, values, compactKey)
	p.Timezone, errs[1] = settings.Get(ctx, values, timezoneKey)
	p.Units, errs[2] = settings.Get(ctx, values, unitsKey)
	p.Theme, errs[3] = settings.Get(ctx, values, themeKey)
	p.Profile, errs[4] = settings.Get(ctx, values, profileKey)
	p.ImportFolder, errs[5] = settings.Get(ctx, values, importFolderKey)
	p.ErrorReports, errs[6] = settings.Get(ctx, values, errorReportsKey)
	return p, errors.Join(errs[:]...)
}

func (p preferences) save(ctx context.Context, values *settings.Values) error {
	return errors.Join(
		settings.Set(ctx, values, compactKey, p.Compact),
		settings.Set(ctx, values, timezoneKey, p.Timezone),
		settings.Set(ctx, values, unitsKey, p.Units),
		settings.Set(ctx, values, themeKey, p.Theme),
		settings.Set(ctx, values, profileKey, p.Profile),
		settings.Set(ctx, values, importFolderKey, p.ImportFolder),
		settings.Set(ctx, values, errorReportsKey, p.ErrorReports),
	)
}

func oneOf(allowed ...string) func(string) error {
	return func(value string) error {
		if !slices.Contains(allowed, value) {
			return fmt.Errorf("%q is not one of %v", value, allowed)
		}
		return nil
	}
}

func validateTimezone(name string) error {
	if name == "" {
		return errors.New("time zone is required")
	}
	if _, err := time.LoadLocation(name); err != nil {
		return fmt.Errorf("load time zone: %w", err)
	}
	return nil
}

// An empty folder means nothing has been imported yet.
func validateImportFolder(path string) error {
	if path != "" && !filepath.IsAbs(path) {
		return errors.New("import folder must be an absolute path")
	}
	return nil
}
