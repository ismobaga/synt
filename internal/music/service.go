// Package music selects and prepares background music.
package music

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ismobaga/synt/internal/db"
)

// Track describes a music track.
type Track struct {
	Name        string
	StoragePath string
	DurationSec float64
	Mood        string
	Source      string
}

// Library provides a curated licensed music library.
type Library interface {
	FindByMood(ctx context.Context, mood string) (*Track, error)
}

// Service manages background music selection.
type Service struct {
	library Library
}

// New creates a new music Service.
func New(lib Library) *Service {
	return &Service{library: lib}
}

// Select picks a music track matching the project's mood.
func (s *Service) Select(ctx context.Context, projectID uuid.UUID, scriptJSON []byte) (*db.AudioTrack, error) {
	mood := extractMood(scriptJSON)
	track, err := s.library.FindByMood(ctx, mood)
	if err != nil {
		return nil, fmt.Errorf("find music for mood %q: %w", mood, err)
	}

	meta, _ := json.Marshal(map[string]string{
		"name":   track.Name,
		"mood":   track.Mood,
		"source": track.Source,
	})

	return &db.AudioTrack{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Kind:        "music",
		StoragePath: track.StoragePath,
		DurationSec: track.DurationSec,
		Metadata:    meta,
		CreatedAt:   time.Now().UTC(),
	}, nil
}

func extractMood(scriptJSON []byte) string {
	var s struct {
		MusicMood string `json:"music_mood"`
	}
	if err := json.Unmarshal(scriptJSON, &s); err == nil && s.MusicMood != "" {
		return s.MusicMood
	}
	return "upbeat"
}
