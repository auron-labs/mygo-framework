package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mygo-framework/internal/database"
	"mygo-framework/jobs"
	"mygo-framework/locales"
	"mygo-framework/state"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/theme"
)

// newScreen returns a screen whose preferences are never persisted.
func newScreen(t *testing.T, initial preferences) *screen {
	t.Helper()
	cell := state.NewCell(initial)
	saver := state.Autosave(cell, time.Hour, func(context.Context, preferences) error { return nil })
	t.Cleanup(func() { _ = saver.Close(context.Background()) })
	return &screen{prefs: cell, saver: saver, jobs: jobs.New(t.Context())}
}

func newTester(s *screen) *ui.Tester {
	return ui.NewTester(func(c *ui.Context) { theme.Apply(c); s.view(c) }, 640, 640)
}

func TestImportJobProgressAndCancel(t *testing.T) {
	reported := make(chan struct{})
	s := newScreen(t, preferences{ImportFolder: t.TempDir()})
	s.work = func(string) jobs.Func {
		return func(ctx context.Context, report jobs.Report) error {
			report(1, 4, "Imported 1 of 4 items")
			reported <- struct{}{}
			<-ctx.Done()
			return ctx.Err()
		}
	}
	finished := make(chan jobs.Status, 8)
	s.jobs.Subscribe(func(st jobs.Status) {
		if st.State != jobs.Running {
			finished <- st
		}
	})
	tt := newTester(s)
	if err := tt.Click("Start import"); err != nil {
		t.Fatal(err)
	}
	<-reported
	tt.Frame()
	if !tt.HasText("Imported 1 of 4 items") || !tt.HasText("Cancel import") {
		t.Fatalf("running view = %q", tt.Texts())
	}
	if err := tt.Click("Cancel import"); err != nil {
		t.Fatal(err)
	}
	if st := <-finished; st.State != jobs.Canceled {
		t.Fatalf("final = %+v", st)
	}
	tt.Frame()
	if !tt.HasText("Import canceled.") || !tt.HasText("Start import") {
		t.Fatalf("canceled view = %q", tt.Texts())
	}
}

func TestImportJobFailureIsShown(t *testing.T) {
	s := newScreen(t, preferences{ImportFolder: t.TempDir()})
	s.work = func(string) jobs.Func {
		return func(context.Context, jobs.Report) error { panic("unexpected input") }
	}
	finished := make(chan jobs.Status, 8)
	s.jobs.Subscribe(func(st jobs.Status) {
		if st.State != jobs.Running {
			finished <- st
		}
	})
	tt := newTester(s)
	if err := tt.Click("Start import"); err != nil {
		t.Fatal(err)
	}
	<-finished
	tt.Frame()
	if !tt.HasText("Import failed: job panicked: unexpected input") {
		t.Fatalf("failed view = %q", tt.Texts())
	}
}

func TestDroppedFolderIsImportedAndRemembered(t *testing.T) {
	folder := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(folder, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	s := newScreen(t, preferences{})
	s.work = func(folder string) jobs.Func {
		return func(_ context.Context, report jobs.Report) error {
			entries, err := os.ReadDir(folder)
			if err != nil {
				return fmt.Errorf("read folder: %w", err)
			}
			report(len(entries), len(entries), "Imported everything")
			return nil
		}
	}
	finished := make(chan jobs.Status, 8)
	s.jobs.Subscribe(func(st jobs.Status) {
		if st.State != jobs.Running {
			finished <- st
		}
	})
	tt := newTester(s)
	tt.Frame()
	zone, ok := tt.Find("Drop a folder here to import it.")
	if !ok {
		t.Fatalf("no drop zone in %q", tt.Texts())
	}
	dropX, dropY := zone.X+zone.W/2, zone.Y+zone.H/2
	if !tt.DropFiles(dropX, dropY, filepath.Join(folder, "a.txt")) {
		t.Fatal("the drop zone did not take the files")
	}
	tt.Frame()
	if !tt.HasText("Drop a folder, not a file.") || s.prefs.Get().ImportFolder != "" {
		t.Fatalf("file drop view = %q, folder = %q", tt.Texts(), s.prefs.Get().ImportFolder)
	}
	if !tt.DropFiles(dropX, dropY, filepath.Join(folder, "a.txt"), folder) {
		t.Fatal("the drop zone did not take the files")
	}
	if st := <-finished; st.State != jobs.Done {
		t.Fatalf("final = %+v", st)
	}
	tt.Frame()
	if s.prefs.Get().ImportFolder != folder || !tt.HasText("Import finished.") {
		t.Fatalf("folder = %q, view = %q", s.prefs.Get().ImportFolder, tt.Texts())
	}
}

func TestSampleImportReportsEveryEntryAndStopsWhenCanceled(t *testing.T) {
	folder := t.TempDir()
	for _, name := range []string{"a", "b"} {
		if err := os.WriteFile(filepath.Join(folder, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var messages []string
	if err := sampleImport(folder)(t.Context(), func(_, _ int, message string) { messages = append(messages, message) }); err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 || messages[1] != "Imported b (2 of 2)" {
		t.Fatalf("messages = %q", messages)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := sampleImport(folder)(ctx, func(int, int, string) {}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled import returned %v", err)
	}
	if err := sampleImport(filepath.Join(folder, "missing"))(t.Context(), nil); err == nil {
		t.Fatal("a missing folder was imported")
	}
}

func TestPreferenceControlsUpdateTheCell(t *testing.T) {
	s := newScreen(t, preferences{Units: unitsMetric, Theme: themeSystem})
	tt := newTester(s)
	if err := tt.Click("Compact spacing"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Imperial"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Dark"); err != nil {
		t.Fatal(err)
	}
	if got := s.prefs.Get(); !got.Compact || got.Units != unitsImperial || got.Theme != themeDark {
		t.Fatalf("preferences = %+v", got)
	}
}

// persistentScreen returns a screen backed by the database in dir, as run builds it.
func persistentScreen(t *testing.T, dir string) (*screen, *database.Database) {
	t.Helper()
	db, values, err := openPreferences(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	initial, err := loadPreferences(t.Context(), values)
	if err != nil {
		t.Fatal(err)
	}
	cell := state.NewCell(initial)
	saver := state.Autosave(cell, time.Hour, savePreferences(values))
	t.Cleanup(func() { _ = saver.Close(context.Background()) })
	return &screen{prefs: cell, saver: saver, jobs: jobs.New(t.Context())}, db
}

func TestPreferencesPersistWithoutClosingTheWindow(t *testing.T) {
	dir := t.TempDir()
	s, db := persistentScreen(t, dir)
	tt := newTester(s)
	if err := tt.Click("Compact spacing"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Imperial"); err != nil {
		t.Fatal(err)
	}
	if err := s.saver.Flush(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	restarted, db := persistentScreen(t, dir)
	t.Cleanup(func() { _ = db.Close() })
	if got := restarted.prefs.Get(); !got.Compact || got.Units != unitsImperial {
		t.Fatalf("restarted preferences = %+v", got)
	}
}

func TestChoosingAThemeAppliesIt(t *testing.T) {
	s := newScreen(t, preferences{Units: unitsMetric, Theme: themeSystem})
	var applied []string
	defer watchTheme(s.prefs, func(name string) { applied = append(applied, name) })()
	tt := newTester(s)
	if err := tt.Click("Compact spacing"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Dark"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(applied, []string{themeDark}) {
		t.Fatalf("applied themes = %q, want only dark", applied)
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

func TestSaveFailureIsVisibleAndBlocksClose(t *testing.T) {
	s, db := persistentScreen(t, t.TempDir())
	tt := newTester(s)
	tt.Frame()
	if tt.HasText(locales.PreferencesSaveFailed()) {
		t.Fatal("save error shown before any failure")
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Compact spacing"); err != nil {
		t.Fatal(err)
	}
	var event mygo.CloseEvent
	s.beforeClose(&event)
	if !event.DefaultPrevented() {
		t.Fatal("a failed save must keep the window open")
	}
	tt.Frame()
	if !tt.HasText(locales.PreferencesSaveFailed()) {
		t.Fatalf("save error is not visible: %q", tt.Texts())
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

// requestCounter stands in for an update server and counts what reaches it.
type requestCounter struct{ requests atomic.Int32 }

func (r *requestCounter) Manifest(context.Context, bool) (io.ReadCloser, error) {
	r.requests.Add(1)
	return nil, errors.New("unexpected update request")
}

func (r *requestCounter) Open(context.Context, string) (io.ReadCloser, error) {
	r.requests.Add(1)
	return nil, errors.New("unexpected update request")
}

func TestUpdatesAreOffUnlessTheBuildConfiguresThem(t *testing.T) {
	server := &requestCounter{}
	mygo.Updater.SetSource(server)
	t.Cleanup(func() { mygo.Updater.SetSource(nil) })
	s := newScreen(t, preferences{})

	s.checkForUpdates()

	if _, started := s.jobs.Status(updateCheckJob); started {
		t.Fatal("an update check started in a build without updates")
	}
	if server.requests.Load() != 0 {
		t.Fatal("the update source was contacted")
	}
	if _, err := mygo.Updater.Check(t.Context()); !errors.Is(err, mygo.ErrUpdatesDisabled) {
		t.Fatalf("Check = %v, want ErrUpdatesDisabled", err)
	}
	tt := newTester(s)
	tt.Frame()
	if tt.HasText("Install update") {
		t.Fatalf("update controls shown without an update: %q", tt.Texts())
	}
}

func TestFoundUpdateOffersInstallAndReportsFailure(t *testing.T) {
	s := newScreen(t, preferences{})
	s.update = &mygo.Update{Version: "2.0.0"}
	finished := make(chan jobs.Status, 8)
	s.jobs.Subscribe(func(st jobs.Status) {
		if st.State != jobs.Running {
			finished <- st
		}
	})
	tt := newTester(s)
	tt.Frame()
	if !tt.HasText("Version 2.0.0 is available.") {
		t.Fatalf("view = %q", tt.Texts())
	}
	// An update that was not made by Check has no signed manifest, so it cannot install.
	if err := tt.Click("Install update"); err != nil {
		t.Fatal(err)
	}
	if st := <-finished; st.State != jobs.Failed {
		t.Fatalf("final = %+v", st)
	}
	tt.Frame()
	if !tt.HasText("The update could not be installed.") || !tt.HasText("Install update") {
		t.Fatalf("failed view = %q", tt.Texts())
	}
}
