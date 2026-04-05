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

	switch provider {
	case "stub":
		return NewStubClient(), nil
	case "auto", "espeak", "espeak-ng", "local":
		cmd, err := resolveESpeakCommand()
		if err != nil {
			if provider == "auto" {
				return NewStubClient(), nil
			}
			return nil, err
		}
		return NewESpeakClient(cmd, envOrDefault("TTS_OUTPUT_DIR", defaultTTSOutputDir)), nil
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

	meta, duration := buildMetadata(text, req.SpeedX)
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
	return "", fmt.Errorf("espeak executable not found; install espeak-ng or use TTS_PROVIDER=stub")
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
	if speedX <= 0 {
		speedX = 1
	}
	words := strings.Fields(strings.TrimSpace(text))
	if len(words) == 0 {
		payload, _ := json.Marshal(map[string]any{
			"provider":   "espeak",
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
		"provider":     "espeak",
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
