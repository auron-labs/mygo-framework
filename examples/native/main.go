// Native demonstrates a Onevra window with retained state and framework settings.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"time"

	"github.com/auron-labs/mygo-framework/v2/database"
	"github.com/auron-labs/mygo-framework/v2/database/migrations"
	"github.com/auron-labs/mygo-framework/v2/datadir"
	"github.com/auron-labs/mygo-framework/v2/errorreport"
	"github.com/auron-labs/mygo-framework/v2/jobs"
	"github.com/auron-labs/mygo-framework/v2/locales"
	"github.com/auron-labs/mygo-framework/v2/logging"
	"github.com/auron-labs/mygo-framework/v2/settings"
	sqlitesettings "github.com/auron-labs/mygo-framework/v2/settings/sqlite"
	"github.com/auron-labs/mygo-framework/v2/state"

	mygomcp "github.com/auron-labs/mygo-framework/v2/mygomcp"
	"github.com/auron-labs/mygo-framework/v2/onevra/components"
	"github.com/auron-labs/mygo-framework/v2/onevra/shells/plain"
	"github.com/auron-labs/mygo-framework/v2/onevra/theme"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	// shutdownTimeout bounds how long quitting waits for background jobs to stop.
	shutdownTimeout = 5 * time.Second
	// saveDelay is how long preferences must stay unchanged before they are saved.
	saveDelay = 300 * time.Millisecond
	// saveTimeout bounds how long closing waits for unsaved preferences.
	saveTimeout = 2 * time.Second
	// suspendTimeout bounds how long a suspend waits for pending state.
	suspendTimeout = 2 * time.Second
	// logDirectory is where the rotating logs live, inside an explicit data directory.
	logDirectory = "logs"
)

const (
	appName     = "Framework Native"
	profileName = "mygo-framework-native"
)

type screen struct {
	prefs *state.Cell[preferences]
	saver *state.Saver[preferences]
	// jobs runs the sample import; work builds the job for a folder.
	jobs *jobs.Manager
	work func(folder string) jobs.Func
	// win is set once the window exists; the folder picker reports through it.
	win           *mygo.Window
	importMessage string
	// dataDir holds the database; logDir holds the logs. The support bundle reads both.
	dataDir string
	logDir  string
	// update is the newer version found at launch, if the build checks for updates.
	update *mygo.Update
	// db is checkpointed when the system suspends the app.
	db *database.Database
	// saveCopy is Dialog.SaveCopy; tests replace it to stand in for a phone's
	// save flow.
	saveCopy func(path, name string) (string, error)
	// openFiles is Dialog.Open; tests replace it to stand in for a phone's
	// document picker.
	openFiles func(options mygo.OpenDialogOptions) ([]string, error)
	// onPhone cancels a running import at suspend, where the system may end the
	// app without a quit, and saves the support bundle through Dialog.SaveCopy.
	onPhone bool
	// kept is what MyGo restores at the next launch, after a phone ends the app.
	kept keptState
}

// keptState outlives a phone's end of the app.
type keptState struct {
	// importRunning records an import that was running when MyGo kept state, so
	// a launch that restores it says the import stopped.
	importRunning bool
}

func (s *screen) view(c *ui.Context) {
	p := s.prefs.Get()
	gap := float32(24)
	if p.Compact {
		gap = 8
	}
	// The page scrolls: a phone's window is short, and larger text makes it
	// shorter still.
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).FillWidth().Padding(24).Gap(gap).Children(func() { s.controls(c, p) })
	})
}

// controls lists the preferences, the import and the support bundle.
func (s *screen) controls(c *ui.Context, p preferences) {
	components.Text(c, appName).FontSize(22).Bold()
	s.updateView(c)
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
	s.importView(c)
	if components.Button(c, locales.SaveSupportBundle(), components.ButtonSecondary).Clicked() {
		s.saveSupportBundle()
	}
}

// savePreferences returns the autosave function for values. It reports each
// failure once; callers that receive the error from Flush must not report it
// again.
func savePreferences(values *settings.Values) func(context.Context, preferences) error {
	return func(ctx context.Context, p preferences) error {
		if err := p.save(ctx, values); err != nil {
			err = fmt.Errorf("save preferences: %w", err)
			log.Print(err)
			errorreport.Capture(err, "preferences.save", "ui")
			return err
		}
		return nil
	}
}

// beforeClose saves pending preferences and keeps the window open, showing the
// error, when that fails.
func (s *screen) beforeClose(event *mygo.CloseEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), saveTimeout)
	defer cancel()
	if err := s.saver.Flush(ctx); err != nil {
		event.PreventDefault()
		if s.win != nil {
			s.win.Invalidate()
		}
	}
}

// suspend saves state the system may end the app before it can: pending
// preferences, the SQLite write-ahead log and queued error reports. On phones,
// which may stop an app in the background without a quit, it also cancels a
// running import, which the view then shows as canceled. Nothing is released,
// so the app keeps working after a resume.
func (s *screen) suspend() {
	flushCtx, cancelFlush := context.WithTimeout(context.Background(), saveTimeout)
	if err := s.saver.Flush(flushCtx); err != nil && s.win != nil {
		s.win.Invalidate()
	}
	cancelFlush()
	if s.onPhone {
		s.jobs.Cancel(importJobName)
	}
	checkpointCtx, cancelCheckpoint := context.WithTimeout(context.Background(), suspendTimeout)
	defer cancelCheckpoint()
	if err := s.db.Checkpoint(checkpointCtx); err != nil {
		log.Print(err)
		errorreport.Capture(err, "database.checkpoint", "runtime")
	}
	errorreport.Flush()
}

// watchTheme calls apply with the preferred appearance whenever it changes.
func watchTheme(prefs *state.Cell[preferences], apply func(name string)) (stop func()) {
	return state.Select(prefs, func(p preferences) string { return p.Theme }).Subscribe(apply)
}

// setTheme switches the running application's appearance. SetSource is not
// documented as safe from other goroutines, so it runs on the main thread.
func setTheme(name string) {
	mygo.RunOnMain(func() { mygo.Theme.SetSource(mygo.ThemeSource(name)) })
}

// openPreferences opens the SQLite database in dir, applies pending
// migrations, and binds the typed preferences to it.
func openPreferences(ctx context.Context, dir string) (*database.Database, *settings.Values, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, nil, fmt.Errorf("create data directory: %w", err)
	}
	db, err := database.Open(ctx, filepath.Join(dir, "app.db"), migrations.FS())
	if err != nil {
		return nil, nil, fmt.Errorf("open database: %w", err)
	}
	store, err := sqlitesettings.New(db.SQL)
	if err != nil {
		return nil, nil, errors.Join(fmt.Errorf("open preferences: %w", err), db.Close())
	}
	values, err := settings.New(store, compactKey, timezoneKey, unitsKey, themeKey, profileKey, importFolderKey, errorReportsKey)
	if err != nil {
		return nil, nil, errors.Join(fmt.Errorf("register preferences: %w", err), db.Close())
	}
	return db, values, nil
}

func main() {
	if err := errorreport.Init(mygo.App.Version()); err != nil {
		log.Print(err)
	}
	if err := start(); err != nil {
		log.Fatal(err)
	}
}

func start() error {
	defer errorreport.Flush()
	defer errorreport.RecoverAndRepanic()
	mcpAddress := flag.String("mcp", "", "Serve an isolated headless UI over MCP at a loopback IP:port instead of opening a window")
	dataDir := flag.String("data-dir", "", "Directory for the database; defaults to the platform application data directory")
	flag.Parse()
	if *mcpAddress != "" {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		err := mygomcp.Run(ctx, *mcpAddress, []mygomcp.Fixture{nativeFixture()})
		stop()
		if err != nil {
			return fmt.Errorf("serve MCP: %w", err)
		}
		return nil
	}
	err := run(*dataDir)
	if err != nil {
		errorreport.Capture(err, "run", "runtime")
	}
	return err
}

func run(dataDir string) (err error) {
	useDefaultDir := dataDir == ""
	if useDefaultDir {
		defaultDir, err := datadir.Default(profileName)
		if err != nil {
			return fmt.Errorf("locate data directory: %w", err)
		}
		dataDir = defaultDir
	}
	// A second launch with the same data directory would open the database
	// twice; -data-dir runs a separate instance on purpose.
	if useDefaultDir && !mygo.App.RequestSingleInstanceLock() {
		return nil
	}
	logDir := filepath.Join(dataDir, logDirectory)
	if useDefaultDir {
		defaultLogDir, err := datadir.Logs(profileName)
		if err != nil {
			return fmt.Errorf("locate log directory: %w", err)
		}
		logDir = defaultLogDir
	}
	logger, closeLogs, err := logging.New(logging.DefaultOptions(mygo.IsDev(), logDir))
	if err != nil {
		return fmt.Errorf("open logs: %w", err)
	}
	defer func() { err = errors.Join(err, closeLogs.Close()) }()
	// The standard log package writes through the default logger, so the
	// existing log calls reach the rotating file.
	slog.SetDefault(logger)
	ctx := context.Background()
	db, values, err := openPreferences(ctx, dataDir)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	prefs, err := loadPreferences(ctx, values)
	if err != nil {
		return fmt.Errorf("load preferences: %w", err)
	}
	cell := state.NewCell(prefs)
	saver := state.Autosave(cell, saveDelay, savePreferences(values))
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), saveTimeout)
		defer cancel()
		err = errors.Join(err, saver.Close(closeCtx))
	}()
	jobManager := jobs.New(ctx)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		err = errors.Join(err, jobManager.Shutdown(shutdownCtx))
	}()
	s := &screen{
		prefs: cell, saver: saver, jobs: jobManager, work: sampleImport, dataDir: dataDir, logDir: logDir,
		db: db, onPhone: runtime.GOOS == "ios" || runtime.GOOS == "android",
	}
	mygo.KeepState(&s.kept)
	defer mygo.Power.OnSuspend(s.suspend)()
	errorreport.SetEnabled(prefs.ErrorReports)
	defer state.Select(cell, func(p preferences) bool { return p.ErrorReports }).Subscribe(errorreport.SetEnabled)()
	mygo.App.OnSecondInstance(func([]string, string) { s.showWindow() })
	mygo.App.SetName(appName)
	mygo.Theme.SetSource(mygo.ThemeSource(prefs.Theme))
	defer watchTheme(cell, setTheme)()
	mygo.App.WhenReady(func() {
		shell := plain.NewShell(plain.ShellOptions{
			Title: appName, Width: 640, Height: 640, MinWidth: 360, MinHeight: 560,
			StateKey: "main", Theme: theme.Onevra, Content: s.view,
			Hidden: true,
		})
		win := shell.Window()
		s.win = win
		if s.restoreImportNotice() {
			win.Invalidate()
		}
		// Update is safe from any goroutine; the view reads the job status and preferences itself.
		jobManager.Subscribe(func(status jobs.Status) {
			win.Update(func() { s.trackImport(status) })
			s.notifyImportDone(status)
		})
		mygo.App.SetMenu(s.menu())
		win.OnClosed(cell.Subscribe(func(preferences) { win.Invalidate() }))
		win.OnClosed(saver.Err().Subscribe(func(error) { win.Invalidate() }))
		// MyGo also calls OnClose during application quit, before destroying the window.
		win.OnClose(s.beforeClose)
		win.OnClosed(func() { mygo.App.Quit() })
		win.OnReadyToShow(win.Show)
		s.checkForUpdates()
	})
	// Registered after the window so a link that launched the app finds it.
	mygo.App.OnOpenURL(func(raw string) { mygo.App.WhenReady(func() { s.openURL(raw) }) })
	if err := mygo.App.Run(); err != nil {
		return fmt.Errorf("run native app: %w", err)
	}
	return nil
}
