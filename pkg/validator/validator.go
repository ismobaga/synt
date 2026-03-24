// Package validator provides input validation utilities.
package validator

import (
	"fmt"
	"strings"
)

// ValidationError holds multiple field errors.
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string {
	var msgs []string
	for field, msg := range e.Fields {
		msgs = append(msgs, fmt.Sprintf("%s: %s", field, msg))
	}
	return strings.Join(msgs, "; ")
}

// Validator accumulates validation errors.
type Validator struct {
	errors map[string]string
}

// New creates a new Validator.
func New() *Validator {
	return &Validator{errors: make(map[string]string)}
}

// Required checks that a string field is non-empty.
func (v *Validator) Required(field, value string) {
	if strings.TrimSpace(value) == "" {
		v.errors[field] = "is required"
	}
}

// MaxLen checks that a string does not exceed maxLen characters.
func (v *Validator) MaxLen(field, value string, maxLen int) {
	if len(value) > maxLen {
		v.errors[field] = fmt.Sprintf("must be %d characters or fewer", maxLen)
	}
}

// Min checks that an integer meets a minimum value.
func (v *Validator) Min(field string, value, min int) {
	if value < min {
		v.errors[field] = fmt.Sprintf("must be at least %d", min)
	}
}

// Max checks that an integer does not exceed a maximum.
func (v *Validator) Max(field string, value, max int) {
	if value > max {
		v.errors[field] = fmt.Sprintf("must be at most %d", max)
	}
}

// OneOf checks that a value is one of the allowed values.
func (v *Validator) OneOf(field, value string, allowed ...string) {
	for _, a := range allowed {
		if value == a {
			return
		}
	}
	v.errors[field] = fmt.Sprintf("must be one of: %s", strings.Join(allowed, ", "))
}

// Valid returns true if no validation errors have been recorded.
func (v *Validator) Valid() bool {
	return len(v.errors) == 0
}

// Err returns a ValidationError if any errors were recorded, or nil.
func (v *Validator) Err() error {
	if len(v.errors) == 0 {
		return nil
	}
	return &ValidationError{Fields: v.errors}
}
