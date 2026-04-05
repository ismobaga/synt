package tts

import (
	"encoding/base64"
	"testing"
)

func TestNewFromEnvKitten(t *testing.T) {
	t.Setenv("TTS_PROVIDER", "kitten")
	t.Setenv("KITTEN_TTS_BASE_URL", "http://localhost:8000")

	client, err := NewFromEnv()
	if err != nil {
		t.Fatalf("NewFromEnv returned error: %v", err)
	}
	if _, ok := client.(*KittenClient); !ok {
		t.Fatalf("expected *KittenClient, got %T", client)
	}
}

func TestNewFromEnvSpeechT5(t *testing.T) {
	t.Setenv("TTS_PROVIDER", "speecht5")
	t.Setenv("MICROSOFT_TTS_BASE_URL", "http://localhost:8010/models")

	client, err := NewFromEnv()
	if err != nil {
		t.Fatalf("NewFromEnv returned error: %v", err)
	}
	if _, ok := client.(*SpeechT5Client); !ok {
		t.Fatalf("expected *SpeechT5Client, got %T", client)
	}
}

func TestNewFromEnvChatterboxAlias(t *testing.T) {
	t.Setenv("TTS_PROVIDER", "chatterbox")
	t.Setenv("CHATTERBOX_TTS_BASE_URL", "http://localhost:8010/models")

	client, err := NewFromEnv()
	if err != nil {
		t.Fatalf("NewFromEnv returned error: %v", err)
	}
	if _, ok := client.(*SpeechT5Client); !ok {
		t.Fatalf("expected *SpeechT5Client, got %T", client)
	}
}

func TestNewFromEnvVibeVoiceAlias(t *testing.T) {
	t.Setenv("TTS_PROVIDER", "vibevoice")
	t.Setenv("VIBEVOICE_TTS_BASE_URL", "http://localhost:8010/models")

	client, err := NewFromEnv()
	if err != nil {
		t.Fatalf("NewFromEnv returned error: %v", err)
	}
	if _, ok := client.(*SpeechT5Client); !ok {
		t.Fatalf("expected *SpeechT5Client, got %T", client)
	}
}

func TestNewFromEnvEdgeTTS(t *testing.T) {
	t.Setenv("TTS_PROVIDER", "edge-tts")
	t.Setenv("EDGE_TTS_BASE_URL", "http://localhost:8010")

	client, err := NewFromEnv()
	if err != nil {
		t.Fatalf("NewFromEnv returned error: %v", err)
	}
	if _, ok := client.(*EdgeClient); !ok {
		t.Fatalf("expected *EdgeClient, got %T", client)
	}
}

func TestDecodeAudioPayloadFromBase64JSON(t *testing.T) {
	expected := []byte("RIFF-demo-wav")
	body := []byte(`{"audio_base64":"` + base64.StdEncoding.EncodeToString(expected) + `"}`)

	actual, err := decodeAudioPayload(body)
	if err != nil {
		t.Fatalf("decodeAudioPayload returned error: %v", err)
	}
	if string(actual) != string(expected) {
		t.Fatalf("unexpected audio payload: %q", string(actual))
	}
}
