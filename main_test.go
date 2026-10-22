package main

import (
	"mygo-framework/problem"
	"testing"
)

func TestNotesCreateAndList(t *testing.T) {
	t.Parallel()

	notes := &Notes{}

	response := notes.Create("  Buy milk  ")
	if response.Data == nil || response.Problem != nil {
		t.Fatalf("create response = %#v", response)
	}
	created := *response.Data
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
	response := notes.Create(" \n\t ")
	if response.Data != nil || response.Problem == nil {
		t.Fatalf("empty-note response = %#v", response)
	}
	if response.Problem.Code != problem.CodeValidation {
		t.Fatalf("problem code = %q, want %q", response.Problem.Code, problem.CodeValidation)
	}
	if listed := notes.List(); len(listed) != 0 {
		t.Fatalf("rejected note was stored: %#v", listed)
	}
}
