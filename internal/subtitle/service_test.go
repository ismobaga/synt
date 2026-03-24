package subtitle_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ismobaga/synt/internal/db"
	"github.com/ismobaga/synt/internal/subtitle"
)

func TestGenerateSubtitlesFromDuration(t *testing.T) {
	svc := subtitle.New()
	projectID := uuid.New()
	track := &db.AudioTrack{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Kind:        "voiceover",
		DurationSec: 30,
		CreatedAt:   time.Now(),
	}
	sub, err := svc.Generate(context.Background(), projectID, track)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sub == nil {
		t.Fatal("expected subtitle, got nil")
	}
	if sub.Format != "srt" {
		t.Errorf("expected srt format, got %s", sub.Format)
	}
	if sub.ProjectID != projectID {
		t.Errorf("wrong project ID")
	}
	if len(sub.Content) == 0 {
		t.Error("expected non-empty content JSON")
	}
}

func TestGenerateSubtitlesNoDuration(t *testing.T) {
	svc := subtitle.New()
	projectID := uuid.New()
	track := &db.AudioTrack{
		ID:        uuid.New(),
		ProjectID: projectID,
		Kind:      "voiceover",
		CreatedAt: time.Now(),
	}
	_, err := svc.Generate(context.Background(), projectID, track)
	if err == nil {
		t.Error("expected error for zero duration track")
	}
}
