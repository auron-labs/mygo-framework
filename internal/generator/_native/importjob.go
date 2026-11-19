package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"mygo-framework/locales"
	"os"
	"path/filepath"
	"time"

	"github.com/auron-labs/mygo-framework/v2/jobs"

	"github.com/auron-labs/mygo-framework/v2/onevra/components"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	importJobName  = "sample-import"
	importStepWait = 250 * time.Millisecond
)

// sampleImport is the example desktop job: it reads the folder inside the job,
// so a missing folder fails the job rather than the click.
func sampleImport(folder string) jobs.Func {
	return func(ctx context.Context, report jobs.Report) error {
		entries, err := os.ReadDir(folder)
		if err != nil {
			return fmt.Errorf("read import folder: %w", err)
		}
		names := make([]string, len(entries))
		for i, entry := range entries {
			names[i] = entry.Name()
		}
		return importEntries(ctx, report, names)
	}
}

// sampleImportFiles is the phone job: MyGo's document picker copies the chosen
// files into a temporary directory, so the job imports those copies.
func sampleImportFiles(paths []string) jobs.Func {
	names := make([]string, len(paths))
	for i, path := range paths {
		names[i] = filepath.Base(path)
	}
	return func(ctx context.Context, report jobs.Report) error {
		return importEntries(ctx, report, names)
	}
}

// importEntries is the loop both imports share. It reports progress after each
// entry and stops as soon as the job is canceled. The wait stands in for real
// work on each entry.
func importEntries(ctx context.Context, report jobs.Report, names []string) error {
	for i, name := range names {
		select {
		case <-ctx.Done():
			return fmt.Errorf("import canceled: %w", ctx.Err())
		case <-time.After(importStepWait):
		}
		report(i+1, len(names), locales.ImportProgressMessage(name, i+1, len(names)))
	}
	return nil
}

// importView renders the import picker, drop zone (desktop only) and the job's
// latest status. It reads the status from the manager, which is safe to query
// from the UI thread while the job runs.
func (s *screen) importView(c *ui.Context) {
	status, started := s.jobs.Status(importJobName)
	running := started && status.State == jobs.Running
	components.Text(c, locales.ImportTitle()).Bold()
	if !s.onPhone {
		s.dropZone(c, !running)
	}
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
	if s.onPhone {
		if components.Button(c, locales.ChooseFiles(), components.ButtonPrimary).Clicked() {
			s.chooseFiles()
		}
		return
	}
	if components.Button(c, locales.ChooseFolder(), components.ButtonSecondary).Clicked() {
		s.chooseFolder()
	}
	if components.Button(c, locales.StartImport(), components.ButtonPrimary).Clicked() {
		s.beginImport()
	}
}

// beginImport imports the remembered folder, asking for one first when there
// is none.
func (s *screen) beginImport() {
	if folder := s.prefs.Get().ImportFolder; folder == "" {
		s.chooseFolder()
	} else {
		s.startImport(folder)
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

// chooseFiles opens the document picker for a phone import. It takes files, not
// a folder, because MyGo's folder picker does not work on Android. The picker
// copies the chosen files into a temporary directory, so the import works on
// the copies, and Dialog.Open blocks until it closes, so it runs off the UI
// thread and reports through Window.Update.
func (s *screen) chooseFiles() {
	open := s.openFiles
	if open == nil {
		open = mygo.Dialog.Open
	}
	options := mygo.OpenDialogOptions{
		Parent: s.win, Title: locales.ChooseFilesTitle(), ButtonLabel: locales.ChooseFolderButton(),
		Multiple: true,
	}
	// Tests build the view without a window, so the picker and its result stay
	// on this goroutine there instead of being posted to a window.
	if s.win == nil {
		s.applyChosenFiles(open(options))
		return
	}
	go func() {
		paths, err := open(options)
		s.win.Update(func() { s.applyChosenFiles(paths, err) })
	}()
}

// applyChosenFiles imports the files the document picker copied, or explains
// why the picker did not open.
func (s *screen) applyChosenFiles(paths []string, err error) {
	switch {
	case err != nil:
		log.Printf("choose import files: %v", err)
		s.importMessage = locales.FilePickerFailed()
	case len(paths) > 0:
		s.startFileImport(paths)
	}
}

// trackImport keeps the kept state in step with the job. It runs on the main
// thread, where KeepState reads.
func (s *screen) trackImport(status jobs.Status) {
	if status.Name != importJobName {
		return
	}
	s.kept.importRunning = status.State == jobs.Running
}

// restoreImportNotice explains an import that was running when MyGo last kept
// state, once, and reports whether it did.
func (s *screen) restoreImportNotice() bool {
	if !s.kept.importRunning {
		return false
	}
	s.importMessage = locales.ImportInterrupted()
	s.kept.importRunning = false
	return true
}

// startImport remembers folder as the last import folder and imports it.
func (s *screen) startImport(folder string) {
	folder, err := filepath.Abs(folder)
	if err != nil {
		s.importMessage = locales.FolderUnusable()
		log.Printf("resolve import folder: %v", err)
		return
	}
	if s.startJob(s.work(folder)) {
		s.prefs.Update(func(p *preferences) { p.ImportFolder = folder })
	}
}

// startFileImport imports the files a phone user picked. The paths are MyGo's
// temporary copies, so they are not remembered as the import folder and are
// removed when the job ends, however it ends.
func (s *screen) startFileImport(paths []string) {
	work := sampleImportFiles(paths)
	started := s.startJob(func(ctx context.Context, report jobs.Report) error {
		defer removeImportFiles(paths)
		return work(ctx, report)
	})
	if !started {
		removeImportFiles(paths)
	}
}

// startJob runs work as the import job and records it for the kept state. It
// reports whether the job started.
func (s *screen) startJob(work jobs.Func) bool {
	err := s.jobs.Start(importJobName, work)
	if errors.Is(err, jobs.ErrRunning) {
		return false
	}
	if err != nil {
		s.importMessage = locales.ImportStartFailed()
		log.Print(err)
		return false
	}
	s.importMessage = ""
	// MyGo writes kept state before suspend handlers run, so mark the import now:
	// a phone that ends the app mid-import keeps it and explains it next time.
	s.kept.importRunning = true
	if s.win != nil {
		s.win.Invalidate()
	}
	return true
}

// removeImportFiles removes the document picker's temporary copies once the
// job that imported them has ended.
func removeImportFiles(paths []string) {
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("remove imported file: %v", err)
		}
	}
}
