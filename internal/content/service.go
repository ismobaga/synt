// Package content generates structured video scripts using an LLM.
package content

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
	SourceURLs  []string
	SourceNotes string
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
	sourceContext := ""
	if len(req.SourceURLs) > 0 || strings.TrimSpace(req.SourceNotes) != "" {
		lines := []string{"", "Source material to ground the script:"}
		for index, rawURL := range req.SourceURLs {
			lines = append(lines, fmt.Sprintf("- Source %d: %s", index+1, rawURL))
		}
		if note := strings.TrimSpace(req.SourceNotes); note != "" {
			lines = append(lines, "- Important notes: "+note)
		}
		lines = append(lines,
			"Use these sources as guidance for framing and claims.",
			"Do not invent specific facts that are not supported by the provided references.",
		)
		sourceContext = strings.Join(lines, "\n") + "\n"
	}

	return fmt.Sprintf(`You are a professional short-video scriptwriter.
Generate a structured JSON script for a %d-second %s video about: %q
Platform: %s
Tone: %s
Language: %s%s
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
	  "visual_type": "string",
      "visual_query": "string",
      "overlay_style": "hook|main|cta"
    }
  ]
}


Style guidelines:
- Modern YouTube style. Simple, clear, confident language. Short punchy sentences. No academic or blog tone

Rules:
- Total scene durations must sum to %d seconds
- Keep captions short (max 6 words)
- Visual queries should be specific and searchable
- First scene is the hook, last scene is the CTA
- Create ONE scene for EVERY sentence, clause, or standalone idea.
- Total scenes last for %d seconds, but do NOT explicitly say durations in the narration.
- Do NOT group multiple sentences into one scene.
- Do NOT summarize or compress content.

VISUAL USAGE RULES:
- Use host for hooks, key explanations, emotional emphasis, and transitions.
- Use screen recording only when referencing studies, tools, data, or research.
- Use text overlays for statistics, key claims, or strong statements.
- Use graphics for processes, comparisons, biology, psychology, and mechanisms.
- Use stock footage for emotional context, real-world relatability, and behavior examples.
- Use AI-generated visuals for abstract concepts, internal states, evolution, chemistry, and metaphors.
- Avoid repeating the same visual type more than 2 times consecutively.
STORY FLOW RULES:
- Maintain logical continuity between scenes.
- Each scene should visually advance understanding.
- Avoid generic visuals; every scene must directly represent the script line.


`,
		req.DurationSec, req.Platform, req.Topic,
		req.Platform, req.Tone, req.Language, sourceContext,
		req.DurationSec, req.DurationSec,
	)
}
