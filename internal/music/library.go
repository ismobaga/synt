package music

import (
	"context"
	"fmt"
)

// defaultTracks is a curated set of licensed tracks by mood.
var defaultTracks = map[string]*Track{
	"upbeat corporate": {
		Name:        "Corporate Upbeat",
		StoragePath: "music/corporate_upbeat.mp3",
		DurationSec: 120,
		Mood:        "upbeat corporate",
		Source:      "internal",
	},
	"upbeat": {
		Name:        "Upbeat Pop",
		StoragePath: "music/upbeat_pop.mp3",
		DurationSec: 90,
		Mood:        "upbeat",
		Source:      "internal",
	},
	"inspirational": {
		Name:        "Inspirational Rise",
		StoragePath: "music/inspirational_rise.mp3",
		DurationSec: 120,
		Mood:        "inspirational",
		Source:      "internal",
	},
	"calm": {
		Name:        "Calm Ambient",
		StoragePath: "music/calm_ambient.mp3",
		DurationSec: 180,
		Mood:        "calm",
		Source:      "internal",
	},
	"dramatic": {
		Name:        "Dramatic Impact",
		StoragePath: "music/dramatic_impact.mp3",
		DurationSec: 90,
		Mood:        "dramatic",
		Source:      "internal",
	},
	"fun": {
		Name:        "Fun & Playful",
		StoragePath: "music/fun_playful.mp3",
		DurationSec: 120,
		Mood:        "fun",
		Source:      "internal",
	},
}

// DefaultLibrary returns tracks from the internal curated library.
type DefaultLibrary struct{}

// NewDefaultLibrary creates a new DefaultLibrary.
func NewDefaultLibrary() *DefaultLibrary {
	return &DefaultLibrary{}
}

// FindByMood returns the best matching track for a mood.
func (l *DefaultLibrary) FindByMood(_ context.Context, mood string) (*Track, error) {
	if t, ok := defaultTracks[mood]; ok {
		return t, nil
	}
	// Fuzzy fallback: find a partial match
	for key, t := range defaultTracks {
		if containsFold(mood, key) || containsFold(key, mood) {
			return t, nil
		}
	}
	// Final fallback: return upbeat
	if t, ok := defaultTracks["upbeat"]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("no track found for mood: %s", mood)
}

func containsFold(s, substr string) bool {
	return len(s) >= len(substr) && foldContains(s, substr)
}

func foldContains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalFold(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
