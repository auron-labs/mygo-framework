// Package ui adapts committed application changes to MyGo pages.
package ui

import (
	"context"
	"log/slog"
	"mygo-framework/app/models"
	"mygo-framework/app/services"
	"mygo-framework/logging"

	"github.com/egoist/mygo"
)

// NoteChanged tells open pages that the shared note store changed.
var NoteChanged = mygo.NewEvent[models.Note]("notes:changed")

// NewNoteChangedCallback adapts committed service changes to the MyGo UI.
func NewNoteChangedCallback(logger *slog.Logger) services.NoteChanged {
	return func(ctx context.Context, note models.Note) {
		if err := NoteChanged.Broadcast(note); err != nil {
			logging.ReportUnexpected(ctx, logger, "notes.changed", "ui", err)
		}
	}
}
