// Package subtitle generates SRT/VTT subtitle files.
package subtitle

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ismobaga/synt/internal/db"
)

// Service generates subtitle files from audio tracks.
type Service struct{}

// New creates a new subtitle Service.
func New() *Service {
	return &Service{}
}

// CaptionEntry is a single timed caption.
type CaptionEntry struct {
	StartSec float64 `json:"start_sec"`
	EndSec   float64 `json:"end_sec"`
	Text     string  `json:"text"`
}

// Generate creates subtitle files for a project from its voiceover track.
func (s *Service) Generate(ctx context.Context, projectID uuid.UUID, track *db.AudioTrack) (*db.Subtitle, error) {
	captions, err := s.alignCaptions(ctx, track)
	if err != nil {
		return nil, fmt.Errorf("align captions: %w", err)
	}

	srtContent := buildSRT(captions)
	contentJSON, _ := json.Marshal(captions)

	// In production: upload to storage. Here we store content inline.
	sub := &db.Subtitle{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Format:      "srt",
		StoragePath: fmt.Sprintf("projects/%s/subtitles/captions.srt", projectID),
		Content:     contentJSON,
		CreatedAt:   time.Now().UTC(),
	}
	if err := validateSRT(srtContent); err != nil {
		return nil, err
	}
	return sub, nil
}

// alignCaptions produces timed phrase-based captions from the audio track metadata.
func (s *Service) alignCaptions(_ context.Context, track *db.AudioTrack) ([]CaptionEntry, error) {
	if track.DurationSec == 0 {
		return nil, fmt.Errorf("audio track has no duration")
	}

	// Parse metadata for word timing if available
	var meta struct {
		Words []struct {
			Word     string  `json:"word"`
			StartSec float64 `json:"start_sec"`
			EndSec   float64 `json:"end_sec"`
		} `json:"words"`
	}
	if len(track.Metadata) > 0 {
		if err := json.Unmarshal(track.Metadata, &meta); err == nil && len(meta.Words) > 0 {
			return phraseGroupWords(meta.Words), nil
		}
	}

	// Fallback: estimate timing based on duration only
	return estimateCaptions(track.DurationSec), nil
}

func phraseGroupWords(words []struct {
	Word     string  `json:"word"`
	StartSec float64 `json:"start_sec"`
	EndSec   float64 `json:"end_sec"`
}) []CaptionEntry {
	const maxPhraseWords = 5
	var entries []CaptionEntry
	i := 0
	for i < len(words) {
		end := i + maxPhraseWords
		if end > len(words) {
			end = len(words)
		}
		group := words[i:end]
		var texts []string
		for _, w := range group {
			texts = append(texts, w.Word)
		}
		entries = append(entries, CaptionEntry{
			StartSec: group[0].StartSec,
			EndSec:   group[len(group)-1].EndSec,
			Text:     strings.Join(texts, " "),
		})
		i = end
	}
	return entries
}

func estimateCaptions(totalSec float64) []CaptionEntry {
	// Generate placeholder captions evenly spaced
	phraseDuration := 3.0
	var entries []CaptionEntry
	t := 0.0
	idx := 1
	for t < totalSec {
		end := t + phraseDuration
		if end > totalSec {
			end = totalSec
		}
		entries = append(entries, CaptionEntry{
			StartSec: t,
			EndSec:   end,
			Text:     fmt.Sprintf("Caption %d", idx),
		})
		t = end
		idx++
	}
	return entries
}

// buildSRT converts caption entries to SRT format.
func buildSRT(entries []CaptionEntry) string {
	var sb strings.Builder
	for i, e := range entries {
		sb.WriteString(fmt.Sprintf("%d\n%s --> %s\n%s\n\n",
			i+1,
			formatSRTTime(e.StartSec),
			formatSRTTime(e.EndSec),
			e.Text,
		))
	}
	return sb.String()
}

func formatSRTTime(sec float64) string {
	h := int(sec) / 3600
	m := (int(sec) % 3600) / 60
	s := int(sec) % 60
	ms := int((sec - float64(int(sec))) * 1000)
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}

func validateSRT(srt string) error {
	if strings.TrimSpace(srt) == "" {
		return fmt.Errorf("generated SRT is empty")
	}
	return nil
}
