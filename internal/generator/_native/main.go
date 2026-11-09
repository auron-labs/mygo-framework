// Native demonstrates a Onevra window with retained state and framework settings.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"mygo-framework/database/migrations"
	"mygo-framework/internal/database"
	"mygo-framework/jobs"
	"mygo-framework/settings"
	sqlitesettings "mygo-framework/settings/sqlite"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	mygomcp "github.com/auron-labs/mygo-mcp"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/shells/plain"
	"github.com/oneo-me/onevra/source/theme"
)

// shutdownTimeout bounds how long quitting waits for background jobs to stop.
const shutdownTimeout = 5 * time.Second

const (
	appName     = "__PROJECT_TITLE__"
	profileName = "__PROJECT_SLUG__"
)

type screen struct {
	prefs     preferences
	saveError string
	// jobs runs the sample import; work builds the job for a folder.
	jobs *jobs.Manager
	work func(folder string) jobs.Func
	// win is set once the window exists; the folder picker reports through it.
	win           *mygo.Window
	importMessage string
	// themeChanged applies the selected appearance to the running application.
	themeChanged func()
}

func (s *screen) view(c *ui.Context) {
	gap := float32(24)
	if s.prefs.Compact {
		gap = 8
	}
	ui.Column(c).Fill().Padding(24).Gap(gap).Children(func() {
		components.Text(c, appName).FontSize(22).Bold()
		components.Checkbox(c, &s.prefs.Compact, "Compact spacing").Label("Toggle compact spacing")
		ui.RadioGroup(c, func() {
			components.Radio(c, &s.prefs.Units, unitsMetric, "Metric")
			components.Radio(c, &s.prefs.Units, unitsImperial, "Imperial")
		}).Label("Units")
		ui.RadioGroup(c, func() {
			for _, option := range []struct{ value, label string }{{themeSystem, "System"}, {themeLight, "Light"}, {themeDark, "Dark"}} {
				radio := components.Radio(c, &s.prefs.Theme, option.value, option.label)
				if s.themeChanged != nil {
					radio.OnChange(s.themeChanged)
				}
			}
		}).Label("Theme")
		components.Text(c, "Your preferences are saved when you close the window.").Wrap()
		if s.saveError != "" {
			components.Text(c, s.saveError).Wrap()
		}
		s.importView(c)
	})
}

func (s *screen) save(values *settings.Values) error {
	if err := s.prefs.save(context.Background(), values); err != nil {
		s.saveError = "Could not save your preferences. Please try closing the window again."
		return fmt.Errorf("save preferences: %w", err)
	}
	s.saveError = ""
	return nil
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
	mcpAddress := flag.String("mcp", "", "Serve an isolated headless UI over MCP at a loopback IP:port instead of opening a window")
	dataDir := flag.String("data-dir", "", "Directory for the database; defaults to a profile under the user configuration directory")
	flag.Parse()
	if *mcpAddress != "" {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		err := mygomcp.Run(ctx, *mcpAddress, []mygomcp.Fixture{nativeFixture()})
		stop()
		if err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := run(*dataDir); err != nil {
		log.Fatal(err)
	}
}

func run(dataDir string) (err error) {
	if dataDir == "" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			return fmt.Errorf("locate data directory: %w", err)
		}
		dataDir = filepath.Join(configDir, profileName)
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
	jobManager := jobs.New(ctx)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		err = errors.Join(err, jobManager.Shutdown(shutdownCtx))
	}()
	s := &screen{prefs: prefs, jobs: jobManager, work: sampleImport}
	s.themeChanged = func() { mygo.Theme.SetSource(mygo.ThemeSource(s.prefs.Theme)) }
	mygo.App.SetName(appName)
	s.themeChanged()
	mygo.App.WhenReady(func() {
		shell := plain.NewShell(plain.ShellOptions{
			Title: appName, Width: 640, Height: 640, MinWidth: 360, MinHeight: 560,
			StateKey: "main", Theme: theme.Onevra, Content: s.view,
			Hidden: true,
		})
		win := shell.Window()
		s.win = win
		// Invalidate is safe from the job goroutine; the view reads the job status itself.
		jobManager.Subscribe(func(jobs.Status) { win.Invalidate() })
		// MyGo also calls OnClose during application quit, before destroying the window.
		win.OnClose(func(event *mygo.CloseEvent) {
			if err := s.save(values); err != nil {
				event.PreventDefault()
				log.Print(err)
				win.Invalidate()
			}
		})
		win.OnClosed(func() { mygo.App.Quit() })
		win.OnReadyToShow(win.Show)
	})
	if err := mygo.App.Run(); err != nil {
		return fmt.Errorf("run native app: %w", err)
	}
	return nil
}
