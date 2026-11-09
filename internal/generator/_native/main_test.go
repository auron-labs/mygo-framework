package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/theme"
)

func newTester(s *screen) *ui.Tester {
	return ui.NewTester(func(c *ui.Context) { theme.Apply(c); s.view(c) }, 640, 420)
}

func TestPreferenceControls(t *testing.T) {
	themeChanges := 0
	s := &screen{prefs: preferences{Units: unitsMetric, Theme: themeSystem}, themeChanged: func() { themeChanges++ }}
	tt := newTester(s)
	if err := tt.Click("Compact spacing"); err != nil {
		t.Fatal(err)
	}
	if !s.prefs.Compact {
		t.Fatal("checkbox did not update the retained preference")
	}
	if err := tt.Click("Imperial"); err != nil {
		t.Fatal(err)
	}
	if s.prefs.Units != unitsImperial {
		t.Fatalf("units = %q", s.prefs.Units)
	}
	if err := tt.Click("Dark"); err != nil {
		t.Fatal(err)
	}
	if s.prefs.Theme != themeDark || themeChanges != 1 {
		t.Fatalf("theme = %q after %d change notifications", s.prefs.Theme, themeChanges)
	}
}

func TestOpenPreferencesMigratesAndPersists(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "data")

	db, values, err := openPreferences(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	var applied int
	if err := db.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM goose_db_version WHERE version_id = 1 AND is_applied").Scan(&applied); err != nil || applied != 1 {
		t.Fatalf("settings migration applied = %d, %v", applied, err)
	}
	defaults, err := loadPreferences(ctx, values)
	if err != nil {
		t.Fatal(err)
	}
	if defaults != (preferences{Timezone: "UTC", Units: unitsMetric, Theme: themeSystem, Profile: "Default"}) {
		t.Fatalf("defaults = %+v", defaults)
	}
	want := preferences{Compact: true, Timezone: "Australia/Sydney", Units: unitsImperial, Theme: themeDark, Profile: "Work", ImportFolder: t.TempDir()}
	if err := want.save(ctx, values); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, values, err = openPreferences(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	got, err := loadPreferences(ctx, values)
	if err != nil || got != want {
		t.Fatalf("reloaded = %+v, %v; want %+v", got, err, want)
	}
}

func TestPreferencesRejectInvalidValues(t *testing.T) {
	ctx := context.Background()
	db, values, err := openPreferences(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	valid := preferences{Timezone: "UTC", Units: unitsMetric, Theme: themeSystem, Profile: "Default"}
	for name, p := range map[string]preferences{
		"timezone": {Timezone: "Mars/Olympus", Units: valid.Units, Theme: valid.Theme, Profile: valid.Profile},
		"units":    {Timezone: valid.Timezone, Units: "furlongs", Theme: valid.Theme, Profile: valid.Profile},
		"theme":    {Timezone: valid.Timezone, Units: valid.Units, Theme: "neon", Profile: valid.Profile},
		"profile":  {Timezone: valid.Timezone, Units: valid.Units, Theme: valid.Theme},
		"folder":   {Timezone: valid.Timezone, Units: valid.Units, Theme: valid.Theme, Profile: valid.Profile, ImportFolder: "relative/path"},
	} {
		if err := p.save(ctx, values); err == nil {
			t.Errorf("%s: invalid value was saved", name)
		}
	}
}

func TestPreferenceSaveFailure(t *testing.T) {
	ctx := context.Background()
	db, values, err := openPreferences(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := &screen{prefs: preferences{Timezone: "UTC", Units: unitsMetric, Theme: themeSystem, Profile: "Default", Compact: true}}
	if err := s.save(values); err != nil || s.saveError != "" {
		t.Fatalf("save = %v, error text %q", err, s.saveError)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.save(values); err == nil || s.saveError == "" {
		t.Fatal("save failure must keep the window open with a visible error")
	}
	if !newTester(s).HasText(s.saveError) {
		t.Fatal("save error is not visible")
	}
}

func TestOpenPreferencesAcceptsRelativeDataDirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "launch"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Join(root, "launch"))

	db, _, err := openPreferences(t.Context(), filepath.Join("..", "..", filepath.Base(root), "data"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "data", "app.db")); err != nil {
		t.Fatalf("database is not in the directory relative to the working directory: %v", err)
	}
}
