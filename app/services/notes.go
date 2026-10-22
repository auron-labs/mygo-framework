// Package services contains the application's transport-independent features.
package services

import (
	"context"
	"fmt"
	"mygo-framework/app/models"
	"mygo-framework/app/requests"
	"mygo-framework/validation"
	"strings"

	"gorm.io/gorm"
)

// CreateNoteOutput is the result of creating a note.
type CreateNoteOutput struct {
	Note models.Note `json:"note"`
}

// ListNotesOutput is the result of listing notes.
type ListNotesOutput struct {
	Notes []models.Note `json:"notes"`
}

// Notes implements the reference application's note features.
type Notes struct {
	database  *gorm.DB
	validator *validation.Validator
}

// NewNotes constructs the note service from direct dependencies.
func NewNotes(database *gorm.DB, validator *validation.Validator) *Notes {
	return &Notes{database: database, validator: validator}
}

// Create validates and stores one note in a service-owned transaction.
func (s *Notes) Create(ctx context.Context, input requests.CreateNoteInput) (CreateNoteOutput, error) {
	input.Body = strings.TrimSpace(input.Body)
	if err := s.validator.Struct(input); err != nil {
		return CreateNoteOutput{}, fmt.Errorf("validate create note input: %w", err)
	}

	var output CreateNoteOutput
	if err := s.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		output.Note = models.Note{Body: input.Body}
		if err := transaction.Create(&output.Note).Error; err != nil {
			return fmt.Errorf("insert note: %w", err)
		}

		return nil
	}); err != nil {
		return CreateNoteOutput{}, fmt.Errorf("create note transaction: %w", err)
	}

	return output, nil
}

// List validates the request and returns notes in creation order.
func (s *Notes) List(ctx context.Context, input requests.ListNotesInput) (ListNotesOutput, error) {
	if err := s.validator.Struct(input); err != nil {
		return ListNotesOutput{}, fmt.Errorf("validate list notes input: %w", err)
	}

	output := ListNotesOutput{Notes: []models.Note{}}
	if err := s.database.WithContext(ctx).Order("id ASC").Find(&output.Notes).Error; err != nil {
		return ListNotesOutput{}, fmt.Errorf("list notes: %w", err)
	}

	return output, nil
}
