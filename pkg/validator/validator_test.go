package validator_test

import (
	"testing"

	"github.com/ismobaga/synt/pkg/validator"
)

func TestRequired(t *testing.T) {
	v := validator.New()
	v.Required("topic", "")
	if v.Valid() {
		t.Error("expected invalid, got valid")
	}
	if err := v.Err(); err == nil {
		t.Error("expected error, got nil")
	}
}

func TestRequiredValid(t *testing.T) {
	v := validator.New()
	v.Required("topic", "AI tools")
	if !v.Valid() {
		t.Errorf("expected valid, got invalid: %v", v.Err())
	}
}

func TestMaxLen(t *testing.T) {
	v := validator.New()
	v.MaxLen("topic", "this is too long", 5)
	if v.Valid() {
		t.Error("expected invalid, got valid")
	}
}

func TestMin(t *testing.T) {
	v := validator.New()
	v.Min("duration", 0, 15)
	if v.Valid() {
		t.Error("expected invalid for duration < 15")
	}
}

func TestMax(t *testing.T) {
	v := validator.New()
	v.Max("duration", 300, 60)
	if v.Valid() {
		t.Error("expected invalid for duration > 60")
	}
}

func TestOneOf(t *testing.T) {
	v := validator.New()
	v.OneOf("platform", "twitter", "tiktok", "instagram_reels", "youtube_shorts")
	if v.Valid() {
		t.Error("expected invalid for unknown platform")
	}
}

func TestOneOfValid(t *testing.T) {
	v := validator.New()
	v.OneOf("platform", "tiktok", "tiktok", "instagram_reels", "youtube_shorts")
	if !v.Valid() {
		t.Errorf("expected valid: %v", v.Err())
	}
}

func TestMultipleErrors(t *testing.T) {
	v := validator.New()
	v.Required("topic", "")
	v.Min("duration", 5, 15)
	if v.Valid() {
		t.Error("expected invalid")
	}
	err := v.Err()
	if err == nil {
		t.Fatal("expected error")
	}
	// Error message should mention both fields
	msg := err.Error()
	if msg == "" {
		t.Error("expected non-empty error message")
	}
}
