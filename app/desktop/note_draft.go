package desktop

import "sync"

// NoteDraft keeps the note body a page has typed but not submitted, so a
// phone's system can end the app without losing it.
type NoteDraft struct {
	// mu guards body, which bindings write off the Go UI thread.
	mu   sync.Mutex
	body string
}

// NewNoteDraft constructs an empty draft.
func NewNoteDraft() *NoteDraft { return &NoteDraft{} }

// Set stores the unsaved note body.
func (d *NoteDraft) Set(body string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.body = body
}

// Get returns the unsaved note body, empty when none is kept.
func (d *NoteDraft) Get() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.body
}
