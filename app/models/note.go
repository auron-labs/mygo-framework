// Package models contains the reference application's persistence models.
package models

import "time"

// Note is a persisted note.
type Note struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Body      string    `gorm:"not null" json:"body"`
	CreatedAt time.Time `gorm:"not null" json:"createdAt"`
}
