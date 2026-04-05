package tts

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultKittenModel     = "KittenML/kitten-tts-mini-0.8"
	defaultSpeechT5Model   = "microsoft/speecht5_tts"
	defaultChatterboxModel = "resemble-ai/chatterbox"
	defaultVibeVoiceModel  = "fishaudio/VibeVoice-Realtime-0.5B"
	defaultEdgeTTSModel    = "microsoft/edge-tts"
)

// KittenClient synthesizes speech through a Kitten TTS compatible HTTP API.
type KittenClient struct {
	endpoint     string
	apiKey       string
	model        string
	providerName string
	defaultVoice string
	outputDir    string
	httpClient   *http.Client
}

// EdgeClient synthesizes speech through an Edge TTS compatible HTTP API.
type EdgeClient = KittenClient

// SpeechT5Client synthesizes speech through Microsoft's open-source SpeechT5 model.
type SpeechT5Client struct {
	endpoint   string
	apiKey     string
	model      string
	outputDir  string
	httpClient *http.Client
}

// NewKittenClient creates a new Kitten-compatible client.
func NewKittenClient(endpoint, model, apiKey, defaultVoice, outputDir string) *KittenClient {
	return &KittenClient{
		endpoint:     normalizeKittenEndpoint(endpoint),
		apiKey:       strings.TrimSpace(apiKey),
		model:        firstNonEmpty(model, defaultKittenModel),
		providerName: "kitten",
		defaultVoice: strings.TrimSpace(defaultVoice),
		outputDir:    firstNonEmpty(outputDir, defaultTTSOutputDir),
		httpClient: &http.Client{
			Timeout: 90 * time.Second,
		},
	}
}

// NewEdgeClient creates a new Edge TTS compatible client.
func NewEdgeClient(endpoint, apiKey, defaultVoice, outputDir string) *EdgeClient {
	return &KittenClient{
		endpoint:     normalizeKittenEndpoint(endpoint),
		apiKey:       strings.TrimSpace(apiKey),
		model:        defaultEdgeTTSModel,
		providerName: "edge-tts",
		defaultVoice: firstNonEmpty(defaultVoice, "en-US-JennyNeural"),
		outputDir:    firstNonEmpty(outputDir, defaultTTSOutputDir),
		httpClient: &http.Client{
			Timeout: 90 * time.Second,
		},
	}
}

// NewSpeechT5Client creates a new SpeechT5-compatible client.
func NewSpeechT5Client(baseURL, model, apiKey, outputDir string) *SpeechT5Client {
	model = firstNonEmpty(model, defaultSpeechT5Model)
	return &SpeechT5Client{
		endpoint:  normalizeSpeechT5Endpoint(baseURL, model),
		apiKey:    strings.TrimSpace(apiKey),
		model:     model,
		outputDir: firstNonEmpty(outputDir, defaultTTSOutputDir),
		httpClient: &http.Client{
			Timeout: 90 * time.Second,
		},
	}
}

// Synthesize generates a WAV file using a Kitten-compatible API.
func (c *KittenClient) Synthesize(ctx context.Context, req SynthesizeRequest) (*SynthesizeResult, error) {
	text := strings.TrimSpace(req.Text)
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}
	if c.endpoint == "" {
		return nil, fmt.Errorf("Kitten TTS endpoint is not configured")
	}
	if req.SpeedX <= 0 {
		req.SpeedX = 1
	}

	providerName := firstNonEmpty(c.providerName, "kitten")
	fallbackVoice := "expr-voice-2-f"
	if providerName == "edge-tts" {
		fallbackVoice = selectEdgeVoice(req.Language)
	}
	voiceName := firstNonEmpty(req.Voice, c.defaultVoice, fallbackVoice)
	outputPath, err := prepareOutputPath(c.outputDir, "wav")
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"model":           c.model,
		"input":           text,
		"voice":           voiceName,
		"response_format": "wav",
		"format":          "wav",
		"speed":           req.SpeedX,
	}
	if err := synthesizeRemote(ctx, c.httpClient, c.endpoint, c.apiKey, payload, outputPath); err != nil {
		return nil, fmt.Errorf("kitten tts synthesize: %w", err)
	}

	meta, duration := buildMetadataForProvider(providerName, text, req.SpeedX)
	meta = enrichMetadata(meta, map[string]any{
		"model":      c.model,
		"voice_name": voiceName,
	})
	return &SynthesizeResult{
		StoragePath: outputPath,
		DurationSec: duration,
		VoiceName:   voiceName,
		Metadata:    meta,
	}, nil
}

// Synthesize generates a WAV file using the Microsoft SpeechT5 model.
func (c *SpeechT5Client) Synthesize(ctx context.Context, req SynthesizeRequest) (*SynthesizeResult, error) {
	text := strings.TrimSpace(req.Text)
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}
	if c.endpoint == "" {
		return nil, fmt.Errorf("SpeechT5 endpoint is not configured")
	}
	if req.SpeedX <= 0 {
		req.SpeedX = 1
	}

	voiceName := firstNonEmpty(req.Voice, selectRemoteVoice(req.Language), "en-US")
	outputPath, err := prepareOutputPath(c.outputDir, "wav")
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"inputs": text,
		"options": map[string]any{
			"wait_for_model": true,
		},
	}
	if voiceName != "" {
		payload["parameters"] = map[string]any{
			"voice": voiceName,
		}
	}
	if err := synthesizeRemote(ctx, c.httpClient, c.endpoint, c.apiKey, payload, outputPath); err != nil {
		return nil, fmt.Errorf("speecht5 synthesize: %w", err)
	}

	meta, duration := buildMetadataForProvider("speecht5", text, req.SpeedX)
	meta = enrichMetadata(meta, map[string]any{
		"model":      c.model,
		"voice_name": voiceName,
	})
	return &SynthesizeResult{
		StoragePath: outputPath,
		DurationSec: duration,
		VoiceName:   voiceName,
		Metadata:    meta,
	}, nil
}

func prepareOutputPath(outputDir, format string) (string, error) {
	if strings.TrimSpace(outputDir) == "" {
		outputDir = defaultTTSOutputDir
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}
	format = strings.TrimPrefix(strings.TrimSpace(format), ".")
	if format == "" {
		format = "wav"
	}
	return filepath.Join(outputDir, fmt.Sprintf("voiceover_%d.%s", time.Now().UnixNano(), format)), nil
}

func synthesizeRemote(ctx context.Context, client *http.Client, endpoint, apiKey string, payload any, outputPath string) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "audio/wav, audio/mpeg, application/json")
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(data))
		if message == "" {
			message = resp.Status
		}
		return fmt.Errorf("remote TTS returned %s: %s", resp.Status, message)
	}

	audioBytes := data
	if looksLikeJSON(resp.Header.Get("Content-Type"), data) {
		audioBytes, err = decodeAudioPayload(data)
		if err != nil {
			return err
		}
	}
	if len(audioBytes) == 0 {
		return fmt.Errorf("remote TTS returned empty audio payload")
	}

	if err := os.WriteFile(outputPath, audioBytes, 0o644); err != nil {
		return fmt.Errorf("write audio file: %w", err)
	}
	return nil
}

func decodeAudioPayload(body []byte) ([]byte, error) {
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode json tts response: %w", err)
	}

	value, ok := findAudioValue(payload)
	if !ok {
		if message := findErrorMessage(payload); message != "" {
			return nil, fmt.Errorf("%s", message)
		}
		return nil, fmt.Errorf("json response did not contain audio data")
	}
	return materializeAudioValue(value)
}

func findAudioValue(value any) (any, bool) {
	switch v := value.(type) {
	case map[string]any:
		for _, key := range []string{"audio_base64", "audio", "b64_json", "base64", "wav", "data", "output"} {
			if candidate, ok := v[key]; ok {
				return candidate, true
			}
		}
		for _, candidate := range v {
			if nested, ok := findAudioValue(candidate); ok {
				return nested, true
			}
		}
	case []any:
		for _, candidate := range v {
			if nested, ok := findAudioValue(candidate); ok {
				return nested, true
			}
		}
	}
	return nil, false
}

func materializeAudioValue(value any) ([]byte, error) {
	switch v := value.(type) {
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return nil, fmt.Errorf("audio payload was empty")
		}
		if strings.HasPrefix(trimmed, "data:") {
			if idx := strings.Index(trimmed, ","); idx >= 0 {
				trimmed = trimmed[idx+1:]
			}
		}
		if decoded, err := base64.StdEncoding.DecodeString(trimmed); err == nil {
			return decoded, nil
		}
		if decoded, err := base64.RawStdEncoding.DecodeString(trimmed); err == nil {
			return decoded, nil
		}
		return nil, fmt.Errorf("audio payload string was not valid base64")
	case map[string]any, []any:
		nested, ok := findAudioValue(v)
		if !ok {
			return nil, fmt.Errorf("nested audio payload was missing audio data")
		}
		return materializeAudioValue(nested)
	default:
		return nil, fmt.Errorf("unsupported audio payload type %T", value)
	}
}

func findErrorMessage(value any) string {
	switch v := value.(type) {
	case map[string]any:
		for _, key := range []string{"error", "message", "detail"} {
			if candidate, ok := v[key].(string); ok && strings.TrimSpace(candidate) != "" {
				return strings.TrimSpace(candidate)
			}
		}
		for _, candidate := range v {
			if message := findErrorMessage(candidate); message != "" {
				return message
			}
		}
	case []any:
		for _, candidate := range v {
			if message := findErrorMessage(candidate); message != "" {
				return message
			}
		}
	}
	return ""
}

func looksLikeJSON(contentType string, data []byte) bool {
	if strings.Contains(strings.ToLower(contentType), "json") {
		return true
	}
	trimmed := bytes.TrimSpace(data)
	return len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[')
}

func normalizeKittenEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return ""
	}
	trimmed := strings.TrimRight(endpoint, "/")
	if strings.Contains(trimmed, "/v1/audio/speech") || strings.Contains(trimmed, "/audio/speech") || strings.Contains(trimmed, "/synthesize") {
		return trimmed
	}
	return trimmed + "/v1/audio/speech"
}

func normalizeSpeechT5Endpoint(baseURL, model string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = "https://api-inference.huggingface.co/models"
	}
	if strings.Contains(baseURL, "/models/") || strings.Contains(baseURL, "/predict") || strings.Contains(baseURL, "/infer") {
		return baseURL
	}
	return baseURL + "/" + strings.TrimLeft(strings.TrimSpace(model), "/")
}

func selectRemoteVoice(language string) string {
	language = strings.ToLower(strings.TrimSpace(language))
	switch {
	case strings.HasPrefix(language, "en"):
		return "en-US"
	case strings.HasPrefix(language, "es"):
		return "es-ES"
	case strings.HasPrefix(language, "fr"):
		return "fr-FR"
	case strings.HasPrefix(language, "de"):
		return "de-DE"
	case strings.HasPrefix(language, "pt"):
		return "pt-BR"
	default:
		return ""
	}
}

func selectEdgeVoice(language string) string {
	language = strings.ToLower(strings.TrimSpace(language))
	switch {
	case strings.HasPrefix(language, "en"):
		return "en-US-JennyNeural"
	case strings.HasPrefix(language, "es"):
		return "es-ES-ElviraNeural"
	case strings.HasPrefix(language, "fr"):
		return "fr-FR-DeniseNeural"
	case strings.HasPrefix(language, "de"):
		return "de-DE-KatjaNeural"
	case strings.HasPrefix(language, "pt"):
		return "pt-BR-FranciscaNeural"
	default:
		return "en-US-JennyNeural"
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func enrichMetadata(base []byte, fields map[string]any) []byte {
	meta := map[string]any{}
	if len(base) > 0 {
		if err := json.Unmarshal(base, &meta); err != nil {
			meta["raw_metadata"] = string(base)
		}
	}
	for key, value := range fields {
		meta[key] = value
	}
	payload, err := json.Marshal(meta)
	if err != nil {
		return base
	}
	return payload
}
