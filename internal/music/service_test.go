package music_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/ismobaga/synt/internal/music"
)

func TestSelectByMood(t *testing.T) {
	svc := music.New(music.NewDefaultLibrary())
	projectID := uuid.New()
	script := []byte(`{"music_mood": "upbeat corporate", "scenes": []}`)

	track, err := svc.Select(context.Background(), projectID, script)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if track == nil {
		t.Fatal("expected track, got nil")
	}
	if track.Kind != "music" {
		t.Errorf("expected kind 'music', got %s", track.Kind)
	}
	if track.StoragePath == "" {
		t.Error("expected non-empty storage path")
	}
}

func TestSelectFallback(t *testing.T) {
	svc := music.New(music.NewDefaultLibrary())
	projectID := uuid.New()
	// No music_mood in script — should fall back
	script := []byte(`{"scenes": []}`)

	track, err := svc.Select(context.Background(), projectID, script)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if track == nil {
		t.Fatal("expected fallback track, got nil")
	}
}

func TestSelectUnknownMoodFallback(t *testing.T) {
	svc := music.New(music.NewDefaultLibrary())
	projectID := uuid.New()
	script := []byte(`{"music_mood": "zydeco polka fusion", "scenes": []}`)

	track, err := svc.Select(context.Background(), projectID, script)
	if err != nil {
		t.Fatalf("expected fallback track, got error: %v", err)
	}
	if track == nil {
		t.Fatal("expected fallback track, got nil")
	}
}
