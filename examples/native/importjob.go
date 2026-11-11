package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/auron-labs/mygo-framework/jobs"
	"github.com/auron-labs/mygo-framework/locales"

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
			report(i+1, len(entries), locales.ImportProgressMessage(entry.Name(), i+1, len(entries)))
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
	components.Text(c, locales.ImportTitle()).Bold()
	s.dropZone(c, !running)
	if running {
		components.Progress(c, status.Fraction()).Label(locales.ImportProgressLabel())
		components.Text(c, status.Message).Wrap()
		if components.Button(c, locales.ImportCancel(), components.ButtonSecondary).Clicked() {
			s.jobs.Cancel(importJobName)
		}
		return
	}
	switch {
	case started && status.State == jobs.Done:
		components.Text(c, locales.ImportFinished()).Wrap()
	case started && status.State == jobs.Canceled:
		components.Text(c, locales.ImportCanceled()).Wrap()
	case started && status.State == jobs.Failed:
		components.Text(c, locales.ImportFailed(status.Err)).Wrap()
	}
	if s.importMessage != "" {
		components.Text(c, s.importMessage).Wrap()
	}
	if components.Button(c, locales.ChooseFolder(), components.ButtonSecondary).Clicked() {
		s.chooseFolder()
	}
	if components.Button(c, locales.StartImport(), components.ButtonPrimary).Clicked() {
		if folder := s.prefs.Get().ImportFolder; folder == "" {
			s.chooseFolder()
		} else {
			s.startImport(folder)
		}
	}
}

// dropZone takes a folder dragged from Finder or Explorer. Files only reach an
// element that asks for them with DroppedFiles.
func (s *screen) dropZone(c *ui.Context, enabled bool) {
	t := c.Theme()
	zone := ui.Column(c).FillWidth().Padding(16).Radius(t.Radius).Border(1, t.Border)
	label := locales.DropFolder()
	if folder := s.prefs.Get().ImportFolder; folder != "" {
		label = locales.DropFolderLast(folder)
	}
	if enabled && zone.FileDragOver() {
		zone.Border(2, t.Accent)
		label = locales.DropFolderRelease()
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
	s.importMessage = locales.DropNotAFolder()
}

// chooseFolder opens the system folder picker. Dialog.Open blocks until the
// dialog closes, so it runs off the UI thread and returns its result through
// Window.Update.
func (s *screen) chooseFolder() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: s.win, Title: locales.ChooseFolderTitle(), ButtonLabel: locales.ChooseFolderButton(),
			DefaultPath: s.prefs.Get().ImportFolder, Directory: true,
		})
		s.win.Update(func() {
			switch {
			case err != nil:
				log.Printf("choose import folder: %v", err)
				s.importMessage = locales.FolderPickerFailed()
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
		s.importMessage = locales.FolderUnusable()
		log.Printf("resolve import folder: %v", err)
		return
	}
	err = s.jobs.Start(importJobName, s.work(folder))
	if errors.Is(err, jobs.ErrRunning) {
		return
	}
	if err != nil {
		s.importMessage = locales.ImportStartFailed()
		log.Print(err)
		return
	}
	s.importMessage = ""
	s.prefs.Update(func(p *preferences) { p.ImportFolder = folder })
	if s.win != nil {
		s.win.Invalidate()
	}
}
