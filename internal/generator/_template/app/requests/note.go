// Package requests defines validated application-service inputs.
package requests

// CreateNoteInput is the shared input for creating a note.
type CreateNoteInput struct {
	Body string `json:"body" validate:"required"`
}

// ListNotesInput is the shared input for listing notes.
type ListNotesInput struct{}
