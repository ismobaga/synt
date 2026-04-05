// Package tts provides local and configurable text-to-speech clients.
package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultTTSProvider  = "auto"
	defaultTTSOutputDir = "/tmp/synt/tts"
)

// ESpeakClient generates speech locally using the espeak/espeak-ng CLI.
type ESpeakClient struct {
	command   string
	outputDir string
}

// NewFromEnv creates the configured TTS client from environment variables.
func NewFromEnv() (Client, error) {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("TTS_PROVIDER")))
	if provider == "" {
		provider = defaultTTSProvider
	}

	outputDir := envOrDefault("TTS_OUTPUT_DIR", defaultTTSOutputDir)
	kittenEndpoint := normalizeKittenEndpoint(os.Getenv("KITTEN_TTS_BASE_URL"))
	kittenClient := func() (Client, error) {
		if kittenEndpoint == "" {
			return nil, fmt.Errorf("KITTEN_TTS_BASE_URL is required when TTS_PROVIDER=kitten")
		}
		return NewKittenClient(
			kittenEndpoint,
			envOrDefault("KITTEN_TTS_MODEL", defaultKittenModel),
			os.Getenv("KITTEN_TTS_API_KEY"),
			os.Getenv("KITTEN_TTS_VOICE"),
			outputDir,
		), nil
	}
	microsoftClient := func() (Client, error) {
		return NewSpeechT5Client(
			envOrDefault("MICROSOFT_TTS_BASE_URL", "https://api-inference.huggingface.co/models"),
			envOrDefault("MICROSOFT_TTS_MODEL", defaultSpeechT5Model),
			firstNonEmpty(os.Getenv("MICROSOFT_TTS_API_KEY"), os.Getenv("HUGGINGFACE_API_TOKEN")),
			outputDir,
		), nil
	}
	chatterboxClient := func() (Client, error) {
		return NewSpeechT5Client(
			firstNonEmpty(os.Getenv("CHATTERBOX_TTS_BASE_URL"), os.Getenv("MICROSOFT_TTS_BASE_URL"), "https://api-inference.huggingface.co/models"),
			envOrDefault("CHATTERBOX_TTS_MODEL", defaultChatterboxModel),
			firstNonEmpty(os.Getenv("CHATTERBOX_TTS_API_KEY"), os.Getenv("MICROSOFT_TTS_API_KEY"), os.Getenv("HUGGINGFACE_API_TOKEN")),
			outputDir,
		), nil
	}
	vibeVoiceClient := func() (Client, error) {
		return NewSpeechT5Client(
			firstNonEmpty(os.Getenv("VIBEVOICE_TTS_BASE_URL"), os.Getenv("MICROSOFT_TTS_BASE_URL"), "https://api-inference.huggingface.co/models"),
			envOrDefault("VIBEVOICE_TTS_MODEL", defaultVibeVoiceModel),
			firstNonEmpty(os.Getenv("VIBEVOICE_TTS_API_KEY"), os.Getenv("MICROSOFT_TTS_API_KEY"), os.Getenv("HUGGINGFACE_API_TOKEN")),
			outputDir,
		), nil
	}

	switch provider {
	case "stub":
		return NewStubClient(), nil
	case "auto":
		if kittenEndpoint != "" {
			return kittenClient()
		}
		if strings.TrimSpace(os.Getenv("MICROSOFT_TTS_BASE_URL")) != "" || strings.TrimSpace(os.Getenv("HUGGINGFACE_API_TOKEN")) != "" || strings.TrimSpace(os.Getenv("MICROSOFT_TTS_API_KEY")) != "" {
			return microsoftClient()
		}
		cmd, err := resolveESpeakCommand()
		if err != nil {
			return NewStubClient(), nil
		}
		return NewESpeakClient(cmd, outputDir), nil
	case "espeak", "espeak-ng", "local":
		cmd, err := resolveESpeakCommand()
		if err != nil {
			return nil, err
		}
		return NewESpeakClient(cmd, outputDir), nil
	case "kitten", "kitten-tts", "kitten-mini", "kitten-tts-mini":
		return kittenClient()
	case "chatterbox", "resemble-ai/chatterbox":
		return chatterboxClient()
	case "vibevoice", "vibevoice-realtime", "vibevoice-realtime-0.5b", "fishaudio/vibevoice-realtime-0.5b":
		return vibeVoiceClient()
	case "microsoft", "speecht5", "speech-t5":
		return microsoftClient()
	default:
		return nil, fmt.Errorf("unsupported TTS provider %q", provider)
	}
}

// NewESpeakClient creates a new local espeak-backed client.
func NewESpeakClient(command, outputDir string) *ESpeakClient {
	command = strings.TrimSpace(command)
	if command == "" {
		command = "espeak-ng"
	}
	outputDir = strings.TrimSpace(outputDir)
	if outputDir == "" {
		outputDir = defaultTTSOutputDir
	}
	return &ESpeakClient{command: command, outputDir: outputDir}
}

// Synthesize generates a WAV file and estimated word timing metadata.
func (c *ESpeakClient) Synthesize(ctx context.Context, req SynthesizeRequest) (*SynthesizeResult, error) {
	text := strings.TrimSpace(req.Text)
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}
	if req.SpeedX <= 0 {
		req.SpeedX = 1
	}
	if err := os.MkdirAll(c.outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	outputPath := filepath.Join(c.outputDir, fmt.Sprintf("voiceover_%d.wav", time.Now().UnixNano()))
	voiceName := selectESpeakVoice(req)
	rate := int(math.Round(175 * req.SpeedX))
	if rate < 120 {
		rate = 120
	}
	if rate > 260 {
		rate = 260
	}

	args := []string{"-w", outputPath, "-s", fmt.Sprintf("%d", rate), "-v", voiceName, text}
	cmd := exec.CommandContext(ctx, c.command, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("run %s: %w: %s", c.command, err, strings.TrimSpace(stderr.String()))
	}

	meta, duration := buildMetadataForProvider("espeak", text, req.SpeedX)
	return &SynthesizeResult{
		StoragePath: outputPath,
		DurationSec: duration,
		VoiceName:   voiceName,
		Metadata:    meta,
	}, nil
}

func resolveESpeakCommand() (string, error) {
	for _, candidate := range []string{"espeak-ng", "espeak"} {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("espeak executable not found; install espeak, use TTS_PROVIDER=kitten or speecht5, or fall back to TTS_PROVIDER=stub")
}

func selectESpeakVoice(req SynthesizeRequest) string {
	language := strings.ToLower(strings.TrimSpace(req.Language))
	switch {
	case strings.HasPrefix(language, "en"):
		return "en-us"
	case strings.HasPrefix(language, "es"):
		return "es"
	case strings.HasPrefix(language, "fr"):
		return "fr"
	case strings.HasPrefix(language, "de"):
		return "de"
	case strings.HasPrefix(language, "pt"):
		return "pt"
	default:
		if strings.TrimSpace(req.Voice) != "" {
			return req.Voice
		}
		return "en-us"
	}
}

func buildMetadata(text string, speedX float64) ([]byte, float64) {
	return buildMetadataForProvider("tts", text, speedX)
}

func buildMetadataForProvider(provider, text string, speedX float64) ([]byte, float64) {
	if speedX <= 0 {
		speedX = 1
	}
	provider = firstNonEmpty(provider, "tts")
	words := strings.Fields(strings.TrimSpace(text))
	if len(words) == 0 {
		payload, _ := json.Marshal(map[string]any{
			"provider":   provider,
			"transcript": text,
			"words":      []map[string]any{},
		})
		return payload, 0
	}

	secondsPerWord := 0.42 / speedX
	duration := math.Max(1.2, float64(len(words))*secondsPerWord)
	entries := make([]map[string]any, 0, len(words))
	for i, word := range words {
		start := float64(i) * secondsPerWord
		end := start + secondsPerWord
		entries = append(entries, map[string]any{
			"word":      word,
			"start_sec": roundSeconds(start),
			"end_sec":   roundSeconds(end),
		})
	}

	payload, _ := json.Marshal(map[string]any{
		"provider":     provider,
		"transcript":   text,
		"duration_sec": roundSeconds(duration),
		"words":        entries,
	})
	return payload, roundSeconds(duration)
}

func roundSeconds(value float64) float64 {
	return math.Round(value*100) / 100
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
