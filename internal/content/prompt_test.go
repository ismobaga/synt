package content

import (
	"strings"
	"testing"
)

func TestBuildPromptIncludesSourceMaterial(t *testing.T) {
	req := GenerateRequest{
		Topic:       "AI tools for startups",
		Platform:    "youtube_shorts",
		DurationSec: 30,
		Tone:        "educational",
		Language:    "en",
		SourceURLs: []string{
			"https://example.com/article-about-ai",
			"https://youtu.be/demo-video",
		},
		SourceNotes: "Focus on the practical benefits and mention one real-world example.",
	}

	prompt := buildPrompt(req)
	for _, want := range []string{
		"Source material to ground the script:",
		"https://example.com/article-about-ai",
		"https://youtu.be/demo-video",
		"Focus on the practical benefits",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected prompt to include %q, got: %s", want, prompt)
		}
	}
}
