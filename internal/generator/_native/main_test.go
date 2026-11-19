package main

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"mygo-framework/locales"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/database"
	"github.com/auron-labs/mygo-framework/jobs"
	"github.com/auron-labs/mygo-framework/state"

	"github.com/auron-labs/mygo-framework/onevra/theme"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
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

func TestPhoneImportImportsPickedFilesAndRemovesTheCopies(t *testing.T) {
	for _, tc := range []struct {
		name   string
		cancel bool
	}{
		{name: "finished"},
		{name: "canceled", cancel: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			picked := make([]string, 0, 2)
			for _, name := range []string{"a.txt", "b.txt"} {
				path := filepath.Join(dir, name)
				if err := os.WriteFile(path, []byte(name), 0o600); err != nil {
					t.Fatal(err)
				}
				picked = append(picked, path)
			}
			s := newScreen(t, preferences{})
			s.onPhone = true
			var options mygo.OpenDialogOptions
			s.openFiles = func(o mygo.OpenDialogOptions) ([]string, error) {
				options = o
				return picked, nil
			}
			running := make(chan struct{}, 1)
			finished := make(chan jobs.Status, 4)
			s.jobs.Subscribe(func(st jobs.Status) {
				switch st.State {
				case jobs.Queued:
				case jobs.Running:
					select {
					case running <- struct{}{}:
					default:
					}
				case jobs.Done, jobs.Failed, jobs.Canceled:
					finished <- st
				}
			})
			tt := newTester(s)
			if err := tt.Click(locales.ChooseFiles()); err != nil {
				t.Fatal(err)
			}
			select {
			case <-running:
			case <-time.After(5 * time.Second):
				t.Fatal("the file import did not start")
			}
			if !options.Multiple || options.Directory {
				t.Fatalf("dialog options = %+v, want several files and no folder", options)
			}
			if tc.cancel && !s.jobs.Cancel(importJobName) {
				t.Fatal("no running import to cancel")
			}
			select {
			case st := <-finished:
				want := jobs.Done
				if tc.cancel {
					want = jobs.Canceled
				}
				if st.State != want {
					t.Fatalf("final = %+v, want %s", st, want)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("the file import did not end")
			}
			for _, path := range picked {
				if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("picked copy remains after the %s import: %v", tc.name, err)
				}
			}
			if got := s.prefs.Get().ImportFolder; got != "" {
				t.Fatalf("picked files were remembered as the import folder: %q", got)
			}
			wantText := locales.ImportFinished()
			if tc.cancel {
				wantText = locales.ImportCanceled()
			}
			tt.Frame()
			if !tt.HasText(wantText) {
				t.Fatalf("view after the %s import = %q", tc.name, tt.Texts())
			}
		})
	}
}

func TestPhoneImportViewOffersOnlyTheFilePicker(t *testing.T) {
	folder := filepath.Join(t.TempDir(), "remembered")
	s := newScreen(t, preferences{ImportFolder: folder})
	s.onPhone = true
	tt := newTester(s)
	tt.Frame()
	for _, text := range []string{locales.ChooseFolder(), locales.StartImport(), locales.DropFolder(), locales.DropFolderLast(folder)} {
		if tt.HasText(text) {
			t.Fatalf("phone import view shows %q: %q", text, tt.Texts())
		}
	}
	if _, ok := tt.Find(locales.ChooseFiles()); !ok {
		t.Fatalf("phone import view has no file picker: %q", tt.Texts())
	}
}

func TestPhoneImportShowsPickerFailure(t *testing.T) {
	s := newScreen(t, preferences{})
	s.onPhone = true
	s.openFiles = func(mygo.OpenDialogOptions) ([]string, error) {
		return nil, errors.New("denied")
	}
	tt := newTester(s)
	if err := tt.Click(locales.ChooseFiles()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(locales.FilePickerFailed()) {
		t.Fatalf("view after a failed picker = %q", tt.Texts())
	}
	if got := s.prefs.Get().ImportFolder; got != "" {
		t.Fatalf("failed picker remembered a folder: %q", got)
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

func TestSaveSupportBundleButtonUsesTheCopyFlowOnPhones(t *testing.T) {
	s := newScreen(t, preferences{Units: unitsMetric, Theme: themeSystem})
	s.onPhone = true
	s.dataDir = t.TempDir()
	offered := make(chan string, 1)
	s.saveCopy = func(path, name string) (string, error) {
		reader, err := zip.OpenReader(path)
		if err != nil {
			t.Errorf("open support bundle: %v", err)
			return "", fmt.Errorf("open support bundle: %w", err)
		}
		defer func() {
			if err := reader.Close(); err != nil {
				t.Errorf("close support bundle: %v", err)
			}
		}()
		if len(reader.File) == 0 {
			t.Error("support bundle is empty")
		}
		offered <- name
		return "/saved/" + name, nil
	}
	tt := newTester(s)
	if err := tt.Click(locales.SaveSupportBundle()); err != nil {
		t.Fatal(err)
	}
	select {
	case name := <-offered:
		if name != supportBundleName {
			t.Fatalf("offered %q, want %q", name, supportBundleName)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the button did not save a copy")
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
	return &screen{prefs: cell, saver: saver, jobs: jobs.New(t.Context()), db: db}, db
}

// walSize returns the size of the write-ahead log, or zero if it does not exist.
func walSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0
	}
	if err != nil {
		t.Fatalf("stat write-ahead log: %v", err)
	}
	return info.Size()
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
	if defaults != (preferences{Timezone: "UTC", Units: unitsMetric, Theme: themeSystem, Profile: "Default", ErrorReports: true}) {
		t.Fatalf("defaults = %+v", defaults)
	}
	want := preferences{Compact: true, Timezone: "Australia/Sydney", Units: unitsImperial, Theme: themeDark, Profile: "Work", ImportFolder: t.TempDir(), ErrorReports: false}
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
	valid := preferences{Timezone: "UTC", Units: unitsMetric, Theme: themeSystem, Profile: "Default", ErrorReports: true}
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

func TestSuspendSavesPreferencesAndCheckpoints(t *testing.T) {
	dir := t.TempDir()
	s, db := persistentScreen(t, dir)
	t.Cleanup(func() { _ = db.Close() })
	tt := newTester(s)
	if err := tt.Click("Compact spacing"); err != nil {
		t.Fatal(err)
	}

	s.suspend()

	if got := walSize(t, filepath.Join(dir, "app.db-wal")); got != 0 {
		t.Fatalf("write-ahead log size after suspend = %d, want 0", got)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, reopened := persistentScreen(t, dir)
	t.Cleanup(func() { _ = reopened.Close() })
	if got := restarted.prefs.Get(); !got.Compact {
		t.Fatalf("preferences after suspend = %+v, want the change persisted", got)
	}
}

func TestSuspendCancelsImportOnPhones(t *testing.T) {
	s, db := persistentScreen(t, t.TempDir())
	t.Cleanup(func() { _ = db.Close() })
	s.onPhone = true
	// Several files keep the job running after its first progress report, so
	// suspend finds it to cancel rather than already finished.
	dir := t.TempDir()
	picked := make([]string, 0, 6)
	for i := range 6 {
		path := filepath.Join(dir, fmt.Sprintf("%d.txt", i))
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		picked = append(picked, path)
	}
	s.openFiles = func(mygo.OpenDialogOptions) ([]string, error) { return picked, nil }
	reported := make(chan struct{}, 1)
	finished := make(chan jobs.Status, 8)
	s.jobs.Subscribe(func(st jobs.Status) {
		switch {
		case st.State == jobs.Running && st.Message != "":
			select {
			case reported <- struct{}{}:
			default:
			}
		case st.State != jobs.Running:
			finished <- st
		}
	})
	tt := newTester(s)
	if err := tt.Click(locales.ChooseFiles()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-reported:
	case <-time.After(5 * time.Second):
		t.Fatal("the import did not report progress")
	}
	if !s.kept.importRunning {
		t.Fatal("running import was not kept")
	}

	s.suspend()

	if st := <-finished; st.State != jobs.Canceled {
		t.Fatalf("final = %+v", st)
	}
	tt.Frame()
	if !tt.HasText(locales.ImportCanceled()) || tt.HasText(locales.ImportInterrupted()) {
		t.Fatalf("view after suspend = %q", tt.Texts())
	}
}

func TestRestoredImportNoticeExplainsInterruptedImport(t *testing.T) {
	s := newScreen(t, preferences{})
	s.kept.importRunning = true
	s.restoreImportNotice()
	if s.kept.importRunning {
		t.Fatal("restored notice left importRunning set")
	}
	tt := newTester(s)
	tt.Frame()
	if !tt.HasText(locales.ImportInterrupted()) {
		t.Fatalf("view after restore = %q", tt.Texts())
	}
}

func TestFinishedImportClearsRunningState(t *testing.T) {
	s := newScreen(t, preferences{})
	s.trackImport(jobs.Status{Name: importJobName, State: jobs.Running})
	if !s.kept.importRunning {
		t.Fatal("running import was not kept")
	}
	s.trackImport(jobs.Status{Name: importJobName, State: jobs.Done})
	s.restoreImportNotice()
	if s.kept.importRunning {
		t.Fatal("finished import stayed kept as running")
	}
	tt := newTester(s)
	tt.Frame()
	if tt.HasText(locales.ImportInterrupted()) {
		t.Fatalf("view after finished import = %q", tt.Texts())
	}
}

func TestPreferencesSaveAfterSuspend(t *testing.T) {
	dir := t.TempDir()
	s, db := persistentScreen(t, dir)
	t.Cleanup(func() { _ = db.Close() })
	s.suspend()
	s.prefs.Update(func(p *preferences) { p.Units = unitsImperial })
	if err := s.saver.Flush(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, reopened := persistentScreen(t, dir)
	t.Cleanup(func() { _ = reopened.Close() })
	if got := restarted.prefs.Get(); got.Units != unitsImperial {
		t.Fatalf("preferences after suspend and save = %+v, want imperial", got)
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

// phoneTester shows the starter in a phone-sized window with an iPhone's safe
// area.
func phoneTester(s *screen, width, height int) *ui.Tester {
	s.onPhone = true
	tt := ui.NewTester(func(c *ui.Context) { theme.Apply(c); s.view(c) }, width, height)
	tt.SetInsets(47, 12, 34, 9)
	tt.Frame()
	return tt
}

// A finger changes a preference, and a finger drag scrolls to the controls
// below the fold of a short window.
func TestPhoneFingerReachesEveryControl(t *testing.T) {
	s := newScreen(t, preferences{Units: unitsMetric, Theme: themeSystem})
	s.dataDir = t.TempDir()
	offered := make(chan struct{}, 1)
	s.saveCopy = func(string, string) (string, error) { offered <- struct{}{}; return "", nil }
	tt := phoneTester(s, 844, 390)
	r, ok := tt.Find(locales.CompactSpacing())
	if !ok {
		t.Fatalf("missing %q: %q", locales.CompactSpacing(), tt.Texts())
	}
	tt.FingerDown(r.X+r.W/2, r.Y+r.H/2)
	tt.FingerUp(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	if !s.prefs.Get().Compact {
		t.Fatal("a tap did not change the preference")
	}
	// The support bundle button starts below the window; a finger drag up
	// brings it into view.
	for range 6 {
		if b, ok := tt.Find(locales.SaveSupportBundle()); ok && b.Y+b.H <= 390-34 {
			break
		}
		tt.FingerDown(400, 300)
		tt.FingerMove(400, 250)
		tt.FingerMove(400, 150)
		tt.FingerUp(400, 150)
		tt.Frame()
		tt.Advance(time.Second)
	}
	b, ok := tt.Find(locales.SaveSupportBundle())
	if !ok || b.Y+b.H > 390-34 {
		t.Fatalf("the support bundle button is out of reach: %+v %v", b, ok)
	}
	tt.FingerDown(b.X+b.W/2, b.Y+b.H/2)
	tt.FingerUp(b.X+b.W/2, b.Y+b.H/2)
	select {
	case <-offered:
	case <-time.After(5 * time.Second):
		t.Fatal("tapping the button did not save a copy")
	}
}

// Every control on the main screen has an accessible name and a role.
func TestPhoneControlsHaveNamesAndRoles(t *testing.T) {
	named := []ui.Role{
		ui.RoleButton, ui.RoleLink, ui.RoleCheckBox, ui.RoleRadio, ui.RoleSwitch, ui.RoleTab,
		ui.RoleTextField, ui.RoleComboBox, ui.RolePopUpButton, ui.RoleToggleButton, ui.RoleSlider,
	}
	s := newScreen(t, preferences{Units: unitsMetric, Theme: themeSystem})
	s.update = &mygo.Update{Version: "2.0.0"}
	tt := phoneTester(s, 390, 1600)
	controls := 0
	for _, a := range tt.AccessAll() {
		if !slices.Contains(named, a.Role) {
			continue
		}
		controls++
		if a.Name == "" {
			t.Errorf("a %v at %+v has no accessible name", a.Role, a.Bounds)
		}
	}
	if controls < 5 {
		t.Fatalf("found %d controls: %q", controls, tt.Texts())
	}
}
