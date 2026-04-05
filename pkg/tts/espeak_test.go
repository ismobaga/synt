package tts

import (
	"encoding/json"
	"testing"
)

func TestBuildMetadataIncludesTranscriptAndWords(t *testing.T) {
	metaBytes, duration := buildMetadata("Hello world from Synt", 1)
	if duration <= 0 {
		t.Fatalf("expected positive duration, got %v", duration)
	}

	var meta struct {
		Provider   string `json:"provider"`
		Transcript string `json:"transcript"`
		Words      []struct {
			Word     string  `json:"word"`
			StartSec float64 `json:"start_sec"`
			EndSec   float64 `json:"end_sec"`
		} `json:"words"`
	}
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}

	if meta.Transcript != "Hello world from Synt" {
		t.Fatalf("unexpected transcript: %q", meta.Transcript)
	}
	if meta.Provider == "" {
		t.Fatal("expected provider to be populated")
	}
	if len(meta.Words) != 4 {
		t.Fatalf("expected 4 word timings, got %d", len(meta.Words))
	}
	if meta.Words[0].Word != "Hello" {
		t.Fatalf("unexpected first word: %q", meta.Words[0].Word)
	}
}

func TestNewFromEnvStub(t *testing.T) {
	t.Setenv("TTS_PROVIDER", "stub")
	client, err := NewFromEnv()
	if err != nil {
		t.Fatalf("NewFromEnv returned error: %v", err)
	}
	if _, ok := client.(*StubClient); !ok {
		t.Fatalf("expected *StubClient, got %T", client)
	}
}
