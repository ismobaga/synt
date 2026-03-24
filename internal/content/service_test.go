package content_test

import (
	"context"
	"testing"

	"github.com/ismobaga/synt/internal/content"
	"github.com/ismobaga/synt/pkg/llm"
)

func TestGenerateScript(t *testing.T) {
	svc := content.New(llm.NewStubClient())
	req := content.GenerateRequest{
		Topic:       "5 AI tools for small businesses",
		Platform:    "youtube_shorts",
		DurationSec: 30,
		Tone:        "educational",
		Language:    "en",
	}
	script, err := svc.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if script.Title == "" {
		t.Error("expected non-empty title")
	}
	if script.Hook == "" {
		t.Error("expected non-empty hook")
	}
	if len(script.Scenes) == 0 {
		t.Error("expected at least one scene")
	}
	if script.Language != "en" {
		t.Errorf("expected language 'en', got %s", script.Language)
	}
	for i, sc := range script.Scenes {
		if sc.Narration == "" {
			t.Errorf("scene %d has empty narration", i)
		}
		if sc.VisualQuery == "" {
			t.Errorf("scene %d has empty visual query", i)
		}
	}
}

func TestGenerateSetsLanguage(t *testing.T) {
	svc := content.New(llm.NewStubClient())
	req := content.GenerateRequest{
		Topic:       "tech tips",
		Platform:    "tiktok",
		DurationSec: 15,
		Language:    "es",
	}
	script, err := svc.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if script.Language != "es" {
		t.Errorf("expected language 'es', got %s", script.Language)
	}
}
