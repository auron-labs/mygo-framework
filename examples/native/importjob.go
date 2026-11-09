package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"mygo-framework/jobs"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
)

const (
	importJobName  = "sample-import"
	importSteps    = 20
	importStepWait = 250 * time.Millisecond
)

// sampleImport is the example background job: it reports progress after each
// step and stops as soon as it is canceled. The wait stands in for real work.
func sampleImport(ctx context.Context, report jobs.Report) error {
	for i := 1; i <= importSteps; i++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("import canceled: %w", ctx.Err())
		case <-time.After(importStepWait):
		}
		report(i, importSteps, fmt.Sprintf("Imported %d of %d items", i, importSteps))
	}
	return nil
}

// importView renders the import button and the job's latest status. It reads
// the status from the manager, which is safe to query from the UI thread while
// the job runs.
func (s *screen) importView(c *ui.Context) {
	status, started := s.jobs.Status(importJobName)
	components.Text(c, "Import").Bold()
	if started && status.State == jobs.Running {
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
	if components.Button(c, "Start import", components.ButtonPrimary).Clicked() {
		if err := s.jobs.Start(importJobName, s.work); err != nil && !errors.Is(err, jobs.ErrRunning) {
			log.Print(err)
		}
	}
}
