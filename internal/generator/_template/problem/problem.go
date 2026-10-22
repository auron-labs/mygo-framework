// Package problem defines transport-independent application failures.
package problem

import (
	"context"
	"errors"
)

// Code identifies a stable class of application failure.
type Code string

const (
	// CodeValidation reports invalid application input.
	CodeValidation Code = "validation"
	// CodeNotFound reports a missing requested resource.
	CodeNotFound Code = "not_found"
	// CodeConflict reports an operation that conflicts with current state.
	CodeConflict Code = "conflict"
	// CodeUnavailable reports a temporarily unavailable application capability.
	CodeUnavailable Code = "unavailable"
	// CodeCanceled reports canceled request work.
	CodeCanceled Code = "canceled"
	// CodeInternal reports an unexpected application failure.
	CodeInternal Code = "internal"
)

// FieldError describes one invalid input field.
type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Problem is a safe application failure that can cross transport boundaries.
//
//nolint:errname // Problem is the public wire-contract name specified by PLAN.md.
type Problem struct {
	Code        Code         `json:"code"`
	Message     string       `json:"message"`
	FieldErrors []FieldError `json:"fieldErrors"`

	cause error
}

// New constructs a problem without an underlying cause.
func New(code Code, message string) *Problem {
	return &Problem{
		Code:        code,
		Message:     message,
		FieldErrors: []FieldError{},
	}
}

// Wrap constructs a problem that preserves cause for errors.Is and errors.As.
func Wrap(code Code, message string, cause error) *Problem {
	problem := New(code, message)
	problem.cause = cause

	return problem
}

// Validation constructs a validation problem with stable field details.
func Validation(fieldErrors []FieldError, cause error) *Problem {
	problem := Wrap(CodeValidation, "Some fields are invalid.", cause)
	problem.FieldErrors = append([]FieldError(nil), fieldErrors...)

	return problem
}

// FromError converts an error into a safe application problem.
func FromError(err error) *Problem {
	if err == nil {
		return nil
	}

	var existing *Problem
	if errors.As(err, &existing) {
		return existing
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return Wrap(CodeCanceled, "The operation was canceled.", err)
	}

	return Wrap(CodeInternal, "The operation could not be completed.", err)
}

// Error returns the safe problem message.
func (p *Problem) Error() string {
	if p == nil {
		return ""
	}
	if p.Message != "" {
		return p.Message
	}
	if p.Code != "" {
		return string(p.Code)
	}

	return "application error"
}

// Unwrap returns the internal cause without exposing it in JSON.
func (p *Problem) Unwrap() error {
	if p == nil {
		return nil
	}

	return p.cause
}
