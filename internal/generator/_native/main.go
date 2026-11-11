// Native demonstrates a Onevra window with retained state and framework settings.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"mygo-framework/database/migrations"
	"mygo-framework/locales"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/auron-labs/mygo-framework/database"
	"github.com/auron-labs/mygo-framework/errorreport"
	"github.com/auron-labs/mygo-framework/jobs"
	"github.com/auron-labs/mygo-framework/settings"
	sqlitesettings "github.com/auron-labs/mygo-framework/settings/sqlite"
	"github.com/auron-labs/mygo-framework/state"

	mygomcp "github.com/auron-labs/mygo-mcp"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/shells/plain"
	"github.com/oneo-me/onevra/source/theme"
)

const (
	// shutdownTimeout bounds how long quitting waits for background jobs to stop.
	shutdownTimeout = 5 * time.Second
	// saveDelay is how long preferences must stay unchanged before they are saved.
	saveDelay = 300 * time.Millisecond
	// saveTimeout bounds how long closing waits for unsaved preferences.
	saveTimeout = 2 * time.Second
)

const (
	appName     = "__PROJECT_TITLE__"
	profileName = "__PROJECT_SLUG__"
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
	// update is the newer version found at launch, if the build checks for updates.
	update *mygo.Update
}

func (s *screen) view(c *ui.Context) {
	p := s.prefs.Get()
	gap := float32(24)
	if p.Compact {
		gap = 8
	}
	ui.Column(c).Fill().Padding(24).Gap(gap).Children(func() {
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
		components.Text(c, locales.PreferencesSavedAutomatically()).Wrap()
		if s.saver.Err().Get() != nil {
			components.Text(c, locales.PreferencesSaveFailed()).Wrap()
		}
		s.importView(c)
	})
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
	values, err := settings.New(store, compactKey, timezoneKey, unitsKey, themeKey, profileKey, importFolderKey)
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
	dataDir := flag.String("data-dir", "", "Directory for the database; defaults to a profile under the user configuration directory")
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
		configDir, err := os.UserConfigDir()
		if err != nil {
			return fmt.Errorf("locate data directory: %w", err)
		}
		dataDir = filepath.Join(configDir, profileName)
	}
	// A second launch with the same data directory would open the database
	// twice; -data-dir runs a separate instance on purpose.
	if useDefaultDir && !mygo.App.RequestSingleInstanceLock() {
		return nil
	}
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
	s := &screen{prefs: cell, saver: saver, jobs: jobManager, work: sampleImport}
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
		// Invalidate is safe from any goroutine; the view reads the job status and preferences itself.
		jobManager.Subscribe(func(status jobs.Status) {
			win.Invalidate()
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
