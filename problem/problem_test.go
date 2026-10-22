package problem

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestProblemPreservesCauseWithoutSerializingIt(t *testing.T) {
	t.Parallel()

	cause := errors.New("private database detail")
	problem := Wrap(CodeConflict, "The note changed.", cause)
	if !errors.Is(problem, cause) {
		t.Fatal("wrapped cause is not available through errors.Is")
	}

	encoded, err := json.Marshal(problem)
	if err != nil {
		t.Fatalf("marshal problem: %v", err)
	}
	if strings.Contains(string(encoded), cause.Error()) {
		t.Fatalf("serialized problem exposed its cause: %s", encoded)
	}
	if got := string(encoded); got != `{"code":"conflict","message":"The note changed.","fieldErrors":[]}` {
		t.Fatalf("serialized problem = %s", got)
	}
}

func TestValidationCopiesFieldErrors(t *testing.T) {
	t.Parallel()

	fields := []FieldError{{Field: "profile.email", Code: "required", Message: "is required"}}
	problem := Validation(fields, nil)
	fields[0].Field = "changed"

	if problem.Code != CodeValidation {
		t.Fatalf("code = %q, want %q", problem.Code, CodeValidation)
	}
	if got := problem.FieldErrors[0].Field; got != "profile.email" {
		t.Fatalf("field = %q, want profile.email", got)
	}
}

func TestFromError(t *testing.T) {
	t.Parallel()

	existing := New(CodeNotFound, "The note was not found.")
	if got := FromError(existing); got != existing {
		t.Fatal("FromError replaced an existing problem")
	}

	canceled := FromError(context.Canceled)
	if canceled.Code != CodeCanceled || !errors.Is(canceled, context.Canceled) {
		t.Fatalf("canceled problem = %#v", canceled)
	}

	cause := errors.New("driver failure")
	internal := FromError(cause)
	if internal.Code != CodeInternal || !errors.Is(internal, cause) {
		t.Fatalf("internal problem = %#v", internal)
	}
	if got := FromError(nil); got != nil {
		t.Fatalf("nil error mapped to %#v", got)
	}
}
