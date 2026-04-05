// Package llm provides Ollama-backed LLM clients.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultLLMProvider = "stub"
	defaultOllamaURL   = "http://localhost:11434"
	defaultOllamaModel = "qwen3.5:0.8b"
	defaultTemperature = 0.2
)

// OllamaClient sends prompts to an Ollama server.
type OllamaClient struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

type ollamaGenerateRequest struct {
	Model   string         `json:"model"`
	Prompt  string         `json:"prompt"`
	Stream  bool           `json:"stream"`
	Format  string         `json:"format,omitempty"`
	Options map[string]any `json:"options,omitempty"`
}

type ollamaGenerateResponse struct {
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

// NewFromEnv constructs the configured LLM client from environment variables.
func NewFromEnv() (Client, error) {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("LLM_PROVIDER")))
	if provider == "" {
		provider = defaultLLMProvider
	}

	switch provider {
	case "stub":
		return NewStubClient(), nil
	case "ollama", "qwen":
		return NewOllamaClient(
			getEnv("OLLAMA_BASE_URL", defaultOllamaURL),
			getEnv("OLLAMA_MODEL", defaultOllamaModel),
		), nil
	default:
		return nil, fmt.Errorf("unsupported LLM provider %q", provider)
	}
}

// NewOllamaClient creates an Ollama client for the given base URL and model.
func NewOllamaClient(baseURL, model string) *OllamaClient {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = defaultOllamaURL
	}
	model = strings.TrimSpace(model)
	if model == "" {
		model = defaultOllamaModel
	}

	return &OllamaClient{
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: 90 * time.Second,
		},
	}
}

// Complete sends the prompt to Ollama and returns the generated text.
func (c *OllamaClient) Complete(ctx context.Context, prompt string) (string, error) {
	payload, err := json.Marshal(ollamaGenerateRequest{
		Model:  c.model,
		Prompt: prompt,
		Stream: false,
		Format: "json",
		Options: map[string]any{
			"temperature": defaultTemperature,
		},
	})
	if err != nil {
		return "", fmt.Errorf("marshal ollama request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/generate", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create ollama request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read ollama response: %w", err)
	}

	if resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("ollama returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var out ollamaGenerateResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("decode ollama response: %w", err)
	}
	if out.Error != "" {
		return "", fmt.Errorf("ollama error: %s", out.Error)
	}
	if strings.TrimSpace(out.Response) == "" {
		return "", fmt.Errorf("ollama returned empty response")
	}

	return strings.TrimSpace(out.Response), nil
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
