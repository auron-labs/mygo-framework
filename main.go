// Package main runs the MyGo framework reference application.
package main

import (
	"errors"
	"log"
	"strings"
	"sync"

	"github.com/egoist/mygo"
)

// Note is the foundation example's in-memory note representation.
type Note struct {
	ID   int    `json:"id"`
	Body string `json:"body"`
}

// Notes is the side-effect-free service exposed to the initial web frontend.
type Notes struct {
	mu     sync.Mutex
	nextID int
	notes  []Note
}

// Create adds an in-memory note.
func (s *Notes) Create(body string) (Note, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return Note{}, errors.New("note body is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextID++
	note := Note{ID: s.nextID, Body: body}
	s.notes = append(s.notes, note)

	return note, nil
}

// List returns a snapshot of the in-memory notes.
func (s *Notes) List() []Note {
	s.mu.Lock()
	defer s.mu.Unlock()

	notes := make([]Note, len(s.notes))
	copy(notes, s.notes)

	return notes
}

func main() {
	mygo.Bind(&Notes{})

	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title:           "MyGo Framework Notes",
			Width:           960,
			Height:          640,
			MinWidth:        480,
			MinHeight:       360,
			BackgroundColor: "light-dark(#f7f7f5, #171816)",
			StateKey:        "main",
			URL:             "/",
		})
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
