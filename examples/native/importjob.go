package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"mygo-framework/jobs"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
)

const (
	importJobName  = "sample-import"
	importStepWait = 250 * time.Millisecond
)

// sampleImport is the example background job: it reports progress after each
// entry of the folder and stops as soon as it is canceled. The wait stands in
// for real work on each entry.
func sampleImport(folder string) jobs.Func {
	return func(ctx context.Context, report jobs.Report) error {
		entries, err := os.ReadDir(folder)
		if err != nil {
			return fmt.Errorf("read import folder: %w", err)
		}
		for i, entry := range entries {
			select {
			case <-ctx.Done():
				return fmt.Errorf("import canceled: %w", ctx.Err())
			case <-time.After(importStepWait):
			}
			report(i+1, len(entries), fmt.Sprintf("Imported %s (%d of %d)", entry.Name(), i+1, len(entries)))
		}
		return nil
	}
}

// importView renders the import folder picker, drop zone and the job's latest
// status. It reads the status from the manager, which is safe to query from the
// UI thread while the job runs.
func (s *screen) importView(c *ui.Context) {
	status, started := s.jobs.Status(importJobName)
	running := started && status.State == jobs.Running
	components.Text(c, "Import").Bold()
	s.dropZone(c, !running)
	if running {
		components.Progress(c, status.Fraction()).Label("Import progress")
		components.Text(c, status.Message).Wrap()
		if components.Button(c, "Cancel import", components.ButtonSecondary).Clicked() {
			s.jobs.Cancel(importJobName)
		}
		return
	}
	switch {
	case started && status.State == jobs.Done:
		components.Text(c, "Import finished.").Wrap()
	case started && status.State == jobs.Canceled:
		components.Text(c, "Import canceled.").Wrap()
	case started && status.State == jobs.Failed:
		components.Text(c, "Import failed: "+status.Err).Wrap()
	}
	if s.importMessage != "" {
		components.Text(c, s.importMessage).Wrap()
	}
	if components.Button(c, "Choose folder...", components.ButtonSecondary).Clicked() {
		s.chooseFolder()
	}
	if components.Button(c, "Start import", components.ButtonPrimary).Clicked() {
		if s.prefs.ImportFolder == "" {
			s.chooseFolder()
		} else {
			s.startImport(s.prefs.ImportFolder)
		}
	}
}

// dropZone takes a folder dragged from Finder or Explorer. Files only reach an
// element that asks for them with DroppedFiles.
func (s *screen) dropZone(c *ui.Context, enabled bool) {
	t := c.Theme()
	zone := ui.Column(c).FillWidth().Padding(16).Radius(t.Radius).Border(1, t.Border)
	label := "Drop a folder here to import it."
	if s.prefs.ImportFolder != "" {
		label = "Last import folder: " + s.prefs.ImportFolder + "\nDrop a folder here to import another."
	}
	if enabled && zone.FileDragOver() {
		zone.Border(2, t.Accent)
		label = "Release to import this folder."
	}
	zone.Children(func() { components.Text(c, label).Wrap() })
	if files := zone.DroppedFiles(); enabled && len(files) > 0 {
		s.importDropped(files)
	}
}

// importDropped imports the first dropped folder.
func (s *screen) importDropped(paths []string) {
	for _, path := range paths {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			s.startImport(path)
			return
		}
	}
	s.importMessage = "Drop a folder, not a file."
}

// chooseFolder opens the system folder picker. Dialog.Open blocks until the
// dialog closes, so it runs off the UI thread and returns its result through
// Window.Update.
func (s *screen) chooseFolder() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: s.win, Title: "Choose a folder to import", ButtonLabel: "Import",
			DefaultPath: s.prefs.ImportFolder, Directory: true,
		})
		s.win.Update(func() {
			switch {
			case err != nil:
				log.Printf("choose import folder: %v", err)
				s.importMessage = "Could not open the folder picker."
			case len(paths) > 0:
				s.startImport(paths[0])
			}
		})
	}()
}

// startImport remembers folder as the last import folder and imports it.
func (s *screen) startImport(folder string) {
	folder, err := filepath.Abs(folder)
	if err != nil {
		s.importMessage = "That folder could not be used."
		log.Printf("resolve import folder: %v", err)
		return
	}
	err = s.jobs.Start(importJobName, s.work(folder))
	if errors.Is(err, jobs.ErrRunning) {
		return
	}
	if err != nil {
		s.importMessage = "The import could not be started."
		log.Print(err)
		return
	}
	s.importMessage = ""
	s.prefs.ImportFolder = folder
	if s.win != nil {
		s.win.Invalidate()
	}
}
