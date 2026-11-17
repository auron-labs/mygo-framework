package main

import (
	"github.com/egoist/mygo/ui"

	"github.com/auron-labs/mygo-framework/jobs"
)

// importPage is the sample import: folder picker, drop zone and job progress.
func importPage(c *ui.Context, s *screen) { s.importView(c) }

// activeImports is the navigation badge: the number of running imports.
func activeImports(s *screen) int {
	if status, started := s.jobs.Status(importJobName); started && status.State == jobs.Running {
		return 1
	}
	return 0
}
