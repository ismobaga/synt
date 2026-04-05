// Package voice generates TTS narration audio.
package voice

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ismobaga/synt/internal/db"
	"github.com/ismobaga/synt/pkg/s3util"
	"github.com/ismobaga/synt/pkg/tts"
)

// Service generates voiceover audio.
type Service struct {
	tts     tts.Client
	storage s3util.Client
}

// New creates a new voice Service.
func New(client tts.Client, storage ...s3util.Client) *Service {
	var store s3util.Client
	if len(storage) > 0 {
		store = storage[0]
	}
	return &Service{tts: client, storage: store}
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

	metadata, err := s.publishVoiceover(ctx, projectID, result)
	if err != nil {
		return nil, fmt.Errorf("publish voiceover: %w", err)
	}

	return &db.AudioTrack{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Kind:        "voiceover",
		Language:    language,
		VoiceName:   result.VoiceName,
		StoragePath: result.StoragePath,
		DurationSec: result.DurationSec,
		Metadata:    metadata,
		CreatedAt:   time.Now().UTC(),
	}, nil
}

func (s *Service) publishVoiceover(ctx context.Context, projectID uuid.UUID, result *tts.SynthesizeResult) ([]byte, error) {
	meta := map[string]any{}
	if len(result.Metadata) > 0 {
		if err := json.Unmarshal(result.Metadata, &meta); err != nil {
			meta["raw_metadata"] = string(result.Metadata)
		}
	}

	meta["voice_name"] = result.VoiceName
	meta["duration_sec"] = result.DurationSec
	if result.StoragePath != "" {
		meta["local_path"] = result.StoragePath
	}

	if s.storage != nil && shouldUpload(result.StoragePath) {
		file, err := os.Open(result.StoragePath)
		if err != nil {
			return nil, fmt.Errorf("open generated audio: %w", err)
		}
		defer file.Close()

		objectPath := fmt.Sprintf("projects/%s/audio/%s", projectID, filepath.Base(result.StoragePath))
		publicURL, err := s.storage.Upload(ctx, objectPath, file, audioContentType(result.StoragePath))
		if err != nil {
			return nil, fmt.Errorf("upload generated audio: %w", err)
		}
		meta["public_url"] = publicURL
		meta["object_path"] = objectPath
	}

	payload, err := json.Marshal(meta)
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}
	return payload, nil
}

func shouldUpload(path string) bool {
	trimmed := strings.TrimSpace(path)
	return trimmed != "" && !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://")
}

func audioContentType(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3":
		return "audio/mpeg"
	case ".ogg":
		return "audio/ogg"
	default:
		return "audio/wav"
	}
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
