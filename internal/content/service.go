// Package content generates structured video scripts using an LLM.
package content

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ismobaga/synt/pkg/llm"
)

// Service generates video script content.
type Service struct {
	llm llm.Client
}

// New creates a new content Service.
func New(client llm.Client) *Service {
	return &Service{llm: client}
}

// GenerateRequest holds parameters for script generation.
type GenerateRequest struct {
	Topic       string
	Platform    string
	DurationSec int
	Tone        string
	Language    string
	BrandConfig map[string]any
}

// SceneContent describes one scene in the script.
type SceneContent struct {
	Index        int    `json:"index"`
	DurationSec  int    `json:"duration_sec"`
	Narration    string `json:"narration"`
	Caption      string `json:"caption"`
	VisualQuery  string `json:"visual_query"`
	OverlayStyle string `json:"overlay_style"`
}

// ScriptContent is the full structured script.
type ScriptContent struct {
	Title       string         `json:"title"`
	Hook        string         `json:"hook"`
	DurationSec int            `json:"duration_sec"`
	Language    string         `json:"language"`
	CTA         string         `json:"cta"`
	MusicMood   string         `json:"music_mood"`
	Scenes      []SceneContent `json:"scenes"`
}

// Generate produces a structured video script for the given topic.
func (s *Service) Generate(ctx context.Context, req GenerateRequest) (*ScriptContent, error) {
	prompt := buildPrompt(req)
	raw, err := s.llm.Complete(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("llm complete: %w", err)
	}
	var script ScriptContent
	if err := json.Unmarshal([]byte(raw), &script); err != nil {
		return nil, fmt.Errorf("parse script json: %w", err)
	}
	script.Language = req.Language
	if script.DurationSec == 0 {
		script.DurationSec = req.DurationSec
	}
	return &script, nil
}

func buildPrompt(req GenerateRequest) string {
	return fmt.Sprintf(`You are a professional short-video scriptwriter.
Generate a structured JSON script for a %d-second %s video about: %q
Platform: %s
Tone: %s
Language: %s

Return ONLY valid JSON matching this schema:
{
  "title": "string",
  "hook": "string",
  "duration_sec": number,
  "language": "string",
  "cta": "string",
  "music_mood": "string",
  "scenes": [
    {
      "index": number,
      "duration_sec": number,
      "narration": "string",
      "caption": "string",
      "visual_query": "string",
      "overlay_style": "hook|main|cta"
    }
  ]
}

Rules:
- Total scene durations must sum to %d seconds
- Keep captions short (max 6 words)
- Visual queries should be specific and searchable
- First scene is the hook, last scene is the CTA
- Generate 3-6 scenes appropriate for %d seconds
`,
		req.DurationSec, req.Platform, req.Topic,
		req.Platform, req.Tone, req.Language,
		req.DurationSec, req.DurationSec,
	)
}
