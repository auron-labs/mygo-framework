package main

import (
	"testing"
)

func TestNotesCreateAndList(t *testing.T) {
	t.Parallel()

	notes := &Notes{}

	created, err := notes.Create("  Buy milk  ")
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	if created.ID != 1 || created.Body != "Buy milk" {
		t.Fatalf("created note = %#v", created)
	}

	listed := notes.List()
	if len(listed) != 1 || listed[0] != created {
		t.Fatalf("listed notes = %#v", listed)
	}
	listed[0].Body = "changed by caller"
	if got := notes.List()[0].Body; got != created.Body {
		t.Fatalf("stored note changed through list result: %q", got)
	}
}

func TestNotesRejectEmptyBody(t *testing.T) {
	t.Parallel()

	notes := &Notes{}
	if _, err := notes.Create(" \n\t "); err == nil {
		t.Fatal("Create accepted an empty note")
	}
	if listed := notes.List(); len(listed) != 0 {
		t.Fatalf("rejected note was stored: %#v", listed)
	}
}
