package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mygo-framework/jobs"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/theme"
)

func newTester(s *screen) *ui.Tester {
	if s.jobs == nil {
		s.jobs = jobs.New(context.Background())
	}
	return ui.NewTester(func(c *ui.Context) { theme.Apply(c); s.view(c) }, 640, 640)
}

func TestImportJobProgressAndCancel(t *testing.T) {
	reported := make(chan struct{})
	s := &screen{
		jobs:  jobs.New(t.Context()),
		prefs: preferences{ImportFolder: t.TempDir()},
		work: func(string) jobs.Func {
			return func(ctx context.Context, report jobs.Report) error {
				report(1, 4, "Imported 1 of 4 items")
				reported <- struct{}{}
				<-ctx.Done()
				return ctx.Err()
			}
		},
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
	s := &screen{
		jobs:  jobs.New(t.Context()),
		prefs: preferences{ImportFolder: t.TempDir()},
		work: func(string) jobs.Func {
			return func(context.Context, jobs.Report) error { panic("unexpected input") }
		},
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
	s := &screen{jobs: jobs.New(t.Context()), work: func(folder string) jobs.Func {
		return func(_ context.Context, report jobs.Report) error {
			entries, err := os.ReadDir(folder)
			if err != nil {
				return fmt.Errorf("read folder: %w", err)
			}
			report(len(entries), len(entries), "Imported everything")
			return nil
		}
	}}
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
	if !tt.HasText("Drop a folder, not a file.") || s.prefs.ImportFolder != "" {
		t.Fatalf("file drop view = %q, folder = %q", tt.Texts(), s.prefs.ImportFolder)
	}
	if !tt.DropFiles(dropX, dropY, filepath.Join(folder, "a.txt"), folder) {
		t.Fatal("the drop zone did not take the files")
	}
	if st := <-finished; st.State != jobs.Done {
		t.Fatalf("final = %+v", st)
	}
	tt.Frame()
	if s.prefs.ImportFolder != folder || !tt.HasText("Import finished.") {
		t.Fatalf("folder = %q, view = %q", s.prefs.ImportFolder, tt.Texts())
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
	s := &screen{jobs: jobs.New(t.Context())}

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
	s := &screen{jobs: jobs.New(t.Context()), update: &mygo.Update{Version: "2.0.0"}}
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
