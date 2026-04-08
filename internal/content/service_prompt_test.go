package content

import (
	"strings"
	"testing"
)

func TestBuildPromptIncludesGroundingFactBankAndCitationRequirements(t *testing.T) {
	prompt := buildPrompt(GenerateRequest{
		Topic:          "Why the first YouTube video matters",
		Platform:       "youtube_shorts",
		DurationSec:    30,
		Tone:           "educational",
		Language:       "en",
		SourceURLs:     []string{"https://www.youtube.com/watch?v=jNQXAC9IVRw"},
		SourceNotes:    "Use grounded source facts only.",
		GroundingFacts: []string{"The clip shows elephants at the zoo.", "The speaker comments on their long trunks."},
	})

	for _, want := range []string{"Grounding fact bank", "used_source_facts", "source_fact_ids", "Do not invent names, numbers, quotes, or claims"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected prompt to contain %q, got:\n%s", want, prompt)
		}
	}
}
