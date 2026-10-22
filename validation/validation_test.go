package validation

import (
	"errors"
	"mygo-framework/problem"
	"testing"

	"github.com/go-playground/validator/v10"
)

type profileRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type createRequest struct {
	Title   string         `json:"title" validate:"required,min=3,max=20"`
	Profile profileRequest `json:"profile" validate:"required"`
}

func TestStructMapsJSONFieldPaths(t *testing.T) {
	t.Parallel()

	err := New().Struct(createRequest{})
	var got *problem.Problem
	if !errors.As(err, &got) {
		t.Fatalf("validation error type = %T, want *problem.Problem", err)
	}
	if got.Code != problem.CodeValidation {
		t.Fatalf("code = %q, want %q", got.Code, problem.CodeValidation)
	}

	want := []problem.FieldError{
		{Field: "title", Code: "required", Message: "is required"},
		{Field: "profile", Code: "required", Message: "is required"},
	}
	if len(got.FieldErrors) != len(want) {
		t.Fatalf("field errors = %#v, want %#v", got.FieldErrors, want)
	}
	for index := range want {
		if got.FieldErrors[index] != want[index] {
			t.Fatalf("field error %d = %#v, want %#v", index, got.FieldErrors[index], want[index])
		}
	}

	var cause validator.ValidationErrors
	if !errors.As(err, &cause) {
		t.Fatal("validator cause is not available through errors.As")
	}
}

func TestStructMapsNestedAndParameterizedRules(t *testing.T) {
	t.Parallel()

	err := New().Struct(createRequest{
		Title:   "ab",
		Profile: profileRequest{Email: "not-an-email"},
	})
	var got *problem.Problem
	if !errors.As(err, &got) {
		t.Fatalf("validation error type = %T, want *problem.Problem", err)
	}

	want := []problem.FieldError{
		{Field: "title", Code: "min", Message: "must be at least 3"},
		{Field: "profile.email", Code: "email", Message: "must be a valid email address"},
	}
	if len(got.FieldErrors) != len(want) {
		t.Fatalf("field errors = %#v, want %#v", got.FieldErrors, want)
	}
	for index := range want {
		if got.FieldErrors[index] != want[index] {
			t.Fatalf("field error %d = %#v, want %#v", index, got.FieldErrors[index], want[index])
		}
	}
}

func TestStructAcceptsValidRequest(t *testing.T) {
	t.Parallel()

	err := New().Struct(createRequest{
		Title:   "Remember milk",
		Profile: profileRequest{Email: "notes@example.com"},
	})
	if err != nil {
		t.Fatalf("validate request: %v", err)
	}
}

func TestStructRejectsInvalidTarget(t *testing.T) {
	t.Parallel()

	err := New().Struct(nil)
	if err == nil {
		t.Fatal("Struct accepted a nil target")
	}
	var got *problem.Problem
	if errors.As(err, &got) {
		t.Fatalf("programming error was exposed as an application problem: %#v", got)
	}
}
