package main

import (
	"context"
	"errors"
	"log"

	"github.com/auron-labs/mygo-framework/jobs"
	"github.com/auron-labs/mygo-framework/locales"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
)

const (
	updateCheckJob   = "update-check"
	updateInstallJob = "update-install"
)

// checkForUpdates looks for a newer signed version in the background. It
// contacts no server unless the build was made with an update feed and public
// key (the updates section of mygo.json); otherwise Updater.Enabled is false.
func (s *screen) checkForUpdates() {
	if !mygo.Updater.Enabled() {
		return
	}
	err := s.jobs.Start(updateCheckJob, func(ctx context.Context, _ jobs.Report) error {
		found, err := mygo.Updater.Check(ctx)
		if err != nil {
			// An unreachable feed is routine for a desktop app; the next launch asks again.
			log.Printf("check for updates: %v", err)
			return nil
		}
		if found != nil {
			s.win.Update(func() { s.update = found })
		}
		return nil
	})
	if err != nil {
		log.Print(err)
	}
}

func (s *screen) installUpdate() {
	found := s.update
	err := s.jobs.Start(updateInstallJob, func(ctx context.Context, report jobs.Report) error {
		return found.Install(ctx, func(downloaded, total int64) { report(int(downloaded>>10), int(total>>10), "") })
	})
	if err != nil && !errors.Is(err, jobs.ErrRunning) {
		log.Print(err)
	}
}

// updateView renders nothing until an update has been found.
func (s *screen) updateView(c *ui.Context) {
	if s.update == nil {
		return
	}
	status, started := s.jobs.Status(updateInstallJob)
	switch {
	case started && status.State == jobs.Running:
		components.Text(c, locales.UpdateInstalling(s.update.Version)).Wrap()
		components.Progress(c, status.Fraction()).Label(locales.UpdateProgressLabel())
	case started && status.State == jobs.Done:
		components.Text(c, locales.UpdateInstalled(s.update.Version)).Wrap()
		if components.Button(c, locales.UpdateRestart(), components.ButtonPrimary).Clicked() {
			mygo.App.Relaunch()
		}
	default:
		if started && status.State == jobs.Failed {
			components.Text(c, locales.UpdateFailed()).Wrap()
		}
		components.Text(c, locales.UpdateAvailable(s.update.Version)).Wrap()
		if components.Button(c, locales.UpdateInstall(), components.ButtonPrimary).Clicked() {
			s.installUpdate()
		}
	}
}
