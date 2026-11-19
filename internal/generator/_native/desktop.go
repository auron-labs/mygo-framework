package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"mygo-framework/database/migrations"
	"mygo-framework/locales"
	"net/url"
	"os"
	"path/filepath"

	"github.com/auron-labs/mygo-framework/v2/diagnostics"
	"github.com/auron-labs/mygo-framework/v2/jobs"

	"github.com/egoist/mygo"
)

// importLinkHost is the host of the deep link that selects an import folder:
// <scheme>://import?folder=/path. The scheme is listed in mygo.json.
const importLinkHost = "import"

// supportBundleName is the file name offered for the support bundle.
const supportBundleName = "support-bundle.zip"

// menu is the application menu bar. Its items call the same actions as the
// buttons, on the main thread like the view.
func (s *screen) menu() *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu},
		{Label: locales.FileMenu(), Submenu: []*mygo.MenuItem{
			{ID: "choose-folder", Label: locales.ChooseFolder(), Accelerator: "CmdOrCtrl+O", Click: func(*mygo.MenuItem, *mygo.Window) {
				s.chooseFolder()
			}},
			{ID: "start-import", Label: locales.StartImport(), Accelerator: "CmdOrCtrl+I", Click: func(*mygo.MenuItem, *mygo.Window) {
				s.beginImport()
			}},
			mygo.Separator(),
			{Role: mygo.RoleClose},
			{Role: mygo.RoleQuit},
		}},
		{Role: mygo.RoleEditMenu},
		{Role: mygo.RoleWindowMenu},
		{Label: locales.HelpMenu(), Submenu: []*mygo.MenuItem{
			{ID: "support-bundle", Label: locales.SaveSupportBundle(), Click: func(*mygo.MenuItem, *mygo.Window) {
				s.saveSupportBundle()
			}},
		}},
	})
}

// showWindow brings the window to the front, for a second launch, a deep link
// or a clicked notification.
func (s *screen) showWindow() {
	if s.win == nil {
		return
	}
	s.win.Restore()
	s.win.Focus()
}

// openURL handles a deep link. It only selects the folder; the user starts the
// import.
func (s *screen) openURL(raw string) {
	u, err := url.Parse(raw)
	if err != nil || u.Host != importLinkHost {
		log.Printf("ignore link %q", raw)
		return
	}
	folder := u.Query().Get("folder")
	if folder == "" {
		log.Printf("ignore link %q: no folder", raw)
		return
	}
	folder, err = filepath.Abs(folder)
	if info, statErr := os.Stat(folder); err != nil || statErr != nil || !info.IsDir() {
		log.Printf("ignore link %q: not a folder", raw)
		return
	}
	s.prefs.Update(func(p *preferences) { p.ImportFolder = folder })
	s.showWindow()
}

// notifyImportDone tells the user that a background import ended while the
// window was not in front.
func (s *screen) notifyImportDone(status jobs.Status) {
	if status.Name != importJobName || (status.State != jobs.Done && status.State != jobs.Failed) || !mygo.NotificationsSupported() {
		return
	}
	// Focus is read on the main thread and Show can wait for a permission
	// prompt, so neither runs on the job's goroutine.
	go func() {
		if s.win.IsFocused() {
			return
		}
		body := locales.ImportFinished()
		if status.State == jobs.Failed {
			body = locales.ImportFailed(status.Err)
		}
		n := mygo.NewNotification(mygo.NotificationOptions{ID: importJobName, Title: appName, Body: body})
		n.OnClick(s.showWindow)
		if err := n.Show(); err != nil && !errors.Is(err, mygo.ErrNotificationsDenied) {
			log.Printf("show import notification: %v", err)
		}
	}()
}

// saveSupportBundle asks where to save the support bundle and writes it. A
// phone app has no path of its own, so there the bundle goes to a temporary
// file, which Dialog.SaveCopy copies where the user chooses. The dialogs block
// until they close, so they run off the UI thread.
func (s *screen) saveSupportBundle() {
	go func() {
		var err error
		if s.onPhone {
			saveCopy := s.saveCopy
			if saveCopy == nil {
				saveCopy = mygo.Dialog.SaveCopy
			}
			err = saveSupportBundleCopy(s.dataDir, saveCopy)
		} else {
			path, saveErr := mygo.Dialog.Save(mygo.SaveDialogOptions{
				Parent: s.win, Title: locales.SaveSupportBundleTitle(), DefaultPath: supportBundleName,
				Filters: []mygo.FileFilter{{Name: "Zip", Extensions: []string{"zip"}}},
			})
			if saveErr != nil || path == "" {
				if saveErr != nil {
					log.Printf("choose support bundle file: %v", saveErr)
				}
				return
			}
			err = writeSupportBundle(path, s.dataDir)
		}
		if err != nil {
			log.Print(err)
			mygo.Dialog.Error(appName, locales.SupportBundleFailed())
		}
	}()
}

// saveSupportBundleCopy writes the support bundle to a temporary file and hands
// it to save, which copies it where the user chose. The temporary file is
// removed afterwards, whether the user saved, canceled or the copy failed.
func saveSupportBundleCopy(dataDir string, save func(path, name string) (string, error)) error {
	file, err := os.CreateTemp("", "support-bundle-*.zip")
	if err != nil {
		return fmt.Errorf("create support bundle: %w", err)
	}
	path := file.Name()
	defer removeSupportBundleCopy(path)
	if err := file.Close(); err != nil {
		return fmt.Errorf("close support bundle: %w", err)
	}
	if err := writeSupportBundle(path, dataDir); err != nil {
		return err
	}
	_, err = save(path, supportBundleName)
	return err
}

// removeSupportBundleCopy removes the temporary file handed to Dialog.SaveCopy.
// A failure to remove it does not change the outcome of the save.
func removeSupportBundleCopy(path string) {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("remove support bundle copy: %v", err)
	}
}

func writeSupportBundle(path, dataDir string) (err error) {
	file, err := os.Create(path) // #nosec G304 -- the user chose this file in the save dialog.
	if err != nil {
		return fmt.Errorf("create support bundle: %w", err)
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	if err := diagnostics.Write(context.Background(), file, diagnostics.Options{
		Version:      mygo.App.Version(),
		LogDirectory: filepath.Join(dataDir, logDirectory),
		DatabasePath: filepath.Join(dataDir, "app.db"),
		Migrations:   migrations.FS(),
	}); err != nil {
		return fmt.Errorf("write support bundle: %w", err)
	}
	return nil
}
