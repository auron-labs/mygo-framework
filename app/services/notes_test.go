package services

import (
	"context"
	"errors"
	"mygo-framework/app/requests"
	"mygo-framework/database/migrations"
	"mygo-framework/internal/database"
	"mygo-framework/problem"
	"mygo-framework/validation"
	"path/filepath"
	"testing"

	"gorm.io/gorm"
)

func TestNotesCreateAndList(t *testing.T) {
	t.Parallel()

	service, _ := newTestNotes(t)
	created, err := service.Create(t.Context(), requests.CreateNoteInput{Body: "  Buy milk  "})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	if created.Note.ID == 0 || created.Note.Body != "Buy milk" || created.Note.CreatedAt.IsZero() {
		t.Fatalf("created note = %#v", created.Note)
	}

	listed, err := service.List(t.Context(), requests.ListNotesInput{})
	if err != nil {
		t.Fatalf("list notes: %v", err)
	}
	if len(listed.Notes) != 1 ||
		listed.Notes[0].ID != created.Note.ID ||
		listed.Notes[0].Body != created.Note.Body ||
		!listed.Notes[0].CreatedAt.Equal(created.Note.CreatedAt) {
		t.Fatalf("listed notes = %#v, want %#v", listed.Notes, created.Note)
	}
}

func TestNotesCreateRejectsEmptyBodyBeforeWrite(t *testing.T) {
	t.Parallel()

	service, openedDatabase := newTestNotes(t)
	_, err := service.Create(t.Context(), requests.CreateNoteInput{Body: " \n\t "})
	var validationProblem *problem.Problem
	if !errors.As(err, &validationProblem) || validationProblem.Code != problem.CodeValidation {
		t.Fatalf("create error = %v, want validation problem", err)
	}
	if len(validationProblem.FieldErrors) != 1 || validationProblem.FieldErrors[0].Field != "body" {
		t.Fatalf("field errors = %#v, want body error", validationProblem.FieldErrors)
	}

	var count int64
	if err := openedDatabase.GORM.Model(&struct{ ID uint }{}).Table("notes").Count(&count).Error; err != nil {
		t.Fatalf("count notes: %v", err)
	}
	if count != 0 {
		t.Fatalf("stored notes = %d, want 0", count)
	}
}

func TestNotesCreateRollsBackFailedWrite(t *testing.T) {
	t.Parallel()

	service, openedDatabase := newTestNotes(t)
	forcedErr := errors.New("forced post-write failure")
	if err := openedDatabase.GORM.Callback().Create().After("gorm:create").Register(
		"test:fail_after_create",
		func(transaction *gorm.DB) {
			_ = transaction.AddError(forcedErr)
		},
	); err != nil {
		t.Fatalf("register failing callback: %v", err)
	}

	_, err := service.Create(t.Context(), requests.CreateNoteInput{Body: "must roll back"})
	if !errors.Is(err, forcedErr) {
		t.Fatalf("create error = %v, want forced failure", err)
	}
	var count int64
	if err := openedDatabase.GORM.Model(&struct{ ID uint }{}).Table("notes").Count(&count).Error; err != nil {
		t.Fatalf("count notes: %v", err)
	}
	if count != 0 {
		t.Fatalf("stored notes after rollback = %d, want 0", count)
	}
}

func TestNotesListPropagatesCancellation(t *testing.T) {
	t.Parallel()

	service, _ := newTestNotes(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := service.List(ctx, requests.ListNotesInput{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("list error = %v, want context cancellation", err)
	}
}

func newTestNotes(t *testing.T) (*Notes, *database.Database) {
	t.Helper()

	openedDatabase, err := database.Open(
		t.Context(),
		filepath.Join(t.TempDir(), "notes.db"),
		migrations.FS(),
	)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() {
		if err := openedDatabase.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})

	return NewNotes(openedDatabase.GORM, validation.New()), openedDatabase
}
