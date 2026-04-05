package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOllamaClientComplete(t *testing.T) {
	var got ollamaGenerateRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/generate" {
			t.Fatalf("expected /api/generate, got %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":"{\"title\":\"Test\"}"}`))
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, "qwen3.5:0.8b")
	result, err := client.Complete(context.Background(), "write a short JSON script")
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}

	if result != `{"title":"Test"}` {
		t.Fatalf("unexpected result: %s", result)
	}
	if got.Model != "qwen3.5:0.8b" {
		t.Fatalf("expected model qwen3.5:0.8b, got %s", got.Model)
	}
	if got.Format != "json" {
		t.Fatalf("expected json format, got %q", got.Format)
	}
	if got.Stream {
		t.Fatal("expected streaming to be disabled")
	}
	if !strings.Contains(got.Prompt, "short JSON script") {
		t.Fatalf("prompt was not forwarded correctly: %q", got.Prompt)
	}
}

func TestNewFromEnvUsesOllama(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "ollama")
	t.Setenv("OLLAMA_BASE_URL", "http://example.local:11434")
	t.Setenv("OLLAMA_MODEL", "qwen3.5:0.8b")

	client, err := NewFromEnv()
	if err != nil {
		t.Fatalf("NewFromEnv returned error: %v", err)
	}

	ollamaClient, ok := client.(*OllamaClient)
	if !ok {
		t.Fatalf("expected *OllamaClient, got %T", client)
	}
	if ollamaClient.baseURL != "http://example.local:11434" {
		t.Fatalf("unexpected base URL: %s", ollamaClient.baseURL)
	}
	if ollamaClient.model != "qwen3.5:0.8b" {
		t.Fatalf("unexpected model: %s", ollamaClient.model)
	}
}
