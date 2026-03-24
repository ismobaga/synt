package moderation_test

import (
	"context"
	"testing"

	"github.com/ismobaga/synt/internal/moderation"
)

func TestValidateCleanScript(t *testing.T) {
	svc := moderation.New()
	script := []byte(`{
		"title": "AI Tools for Business",
		"hook": "Save hours every week",
		"cta": "Follow for more",
		"scenes": [
			{"narration": "AI saves time", "caption": "Save time now"},
			{"narration": "Automate tasks", "caption": "Automate"}
		]
	}`)
	if err := svc.ValidateScript(context.Background(), script); err != nil {
		t.Errorf("expected clean script to pass, got: %v", err)
	}
}

func TestValidateBlockedPattern(t *testing.T) {
	svc := moderation.New()
	script := []byte(`{
		"title": "Test <script>alert(1)</script>",
		"hook": "Normal hook",
		"cta": "Follow",
		"scenes": []
	}`)
	if err := svc.ValidateScript(context.Background(), script); err == nil {
		t.Error("expected blocked pattern to fail validation")
	}
}

func TestValidateInvalidJSON(t *testing.T) {
	svc := moderation.New()
	if err := svc.ValidateScript(context.Background(), []byte("not json")); err == nil {
		t.Error("expected error for invalid JSON")
	}
}
