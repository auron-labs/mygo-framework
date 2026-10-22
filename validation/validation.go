// Package validation maps validator failures to application problems.
package validation

import (
	"errors"
	"fmt"
	"mygo-framework/problem"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

// Validator validates application request structs.
type Validator struct {
	validator *validator.Validate
}

// New constructs a validator that reports JSON field names.
func New() *Validator {
	validate := validator.New(validator.WithRequiredStructEnabled())
	validate.RegisterTagNameFunc(jsonFieldName)

	return &Validator{validator: validate}
}

// Struct validates value and maps field failures to a validation problem.
func (v *Validator) Struct(value any) error {
	err := v.validator.Struct(value)
	if err == nil {
		return nil
	}

	var invalid *validator.InvalidValidationError
	if errors.As(err, &invalid) {
		return fmt.Errorf("validate %T: %w", value, err)
	}

	var validationErrors validator.ValidationErrors
	if !errors.As(err, &validationErrors) {
		return fmt.Errorf("validate %T: %w", value, err)
	}

	fieldErrors := make([]problem.FieldError, 0, len(validationErrors))
	for _, fieldError := range validationErrors {
		fieldErrors = append(fieldErrors, problem.FieldError{
			Field:   jsonPath(fieldError),
			Code:    fieldError.Tag(),
			Message: fieldMessage(fieldError),
		})
	}

	validationProblem := problem.Validation(fieldErrors, err)

	return fmt.Errorf("validate %T: %w", value, validationProblem)
}

func jsonFieldName(field reflect.StructField) string {
	name := strings.SplitN(field.Tag.Get("json"), ",", 2)[0]
	if name == "-" {
		return ""
	}

	return name
}

func jsonPath(fieldError validator.FieldError) string {
	namespace := fieldError.Namespace()
	if _, path, found := strings.Cut(namespace, "."); found {
		return path
	}

	return fieldError.Field()
}

func fieldMessage(fieldError validator.FieldError) string {
	switch fieldError.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email address"
	case "min":
		return "must be at least " + fieldError.Param()
	case "max":
		return "must be at most " + fieldError.Param()
	default:
		return "is invalid"
	}
}
