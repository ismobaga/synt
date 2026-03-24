// Package voice generates TTS narration audio.
package voice

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ismobaga/synt/internal/db"
	"github.com/ismobaga/synt/pkg/tts"
)

// Service generates voiceover audio.
type Service struct {
	tts tts.Client
}

// New creates a new voice Service.
func New(client tts.Client) *Service {
	return &Service{tts: client}
}

// Generate synthesizes narration for all scenes in the script.
func (s *Service) Generate(ctx context.Context, projectID uuid.UUID, scriptJSON []byte, language string) (*db.AudioTrack, error) {
	narration, err := extractNarration(scriptJSON)
	if err != nil {
		return nil, fmt.Errorf("extract narration: %w", err)
	}

	result, err := s.tts.Synthesize(ctx, tts.SynthesizeRequest{
		Text:     narration,
		Language: language,
		Voice:    defaultVoice(language),
	})
	if err != nil {
		return nil, fmt.Errorf("tts synthesize: %w", err)
	}

	return &db.AudioTrack{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Kind:        "voiceover",
		Language:    language,
		VoiceName:   result.VoiceName,
		StoragePath: result.StoragePath,
		DurationSec: result.DurationSec,
		CreatedAt:   time.Now().UTC(),
	}, nil
}

func defaultVoice(language string) string {
	voices := map[string]string{
		"en": "en-US-Neural2-J",
		"es": "es-ES-Neural2-A",
		"fr": "fr-FR-Neural2-A",
		"de": "de-DE-Neural2-B",
		"pt": "pt-BR-Neural2-A",
	}
	if v, ok := voices[language]; ok {
		return v
	}
	return "en-US-Neural2-J"
}

func extractNarration(scriptJSON []byte) (string, error) {
	// Parse just the narration fields from scenes.
	type scene struct {
		Narration string `json:"narration"`
	}
	type script struct {
		Scenes []scene `json:"scenes"`
	}
	var s script
	if err := json.Unmarshal(scriptJSON, &s); err != nil {
		return "", fmt.Errorf("unmarshal script: %w", err)
	}
	var out string
	for i, sc := range s.Scenes {
		if i > 0 {
			out += " "
		}
		out += sc.Narration
	}
	return out, nil
}
