package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ismobaga/synt/internal/api"
)

// healthHandler is a simple handler for testing route setup
func TestHealthEndpoint(t *testing.T) {
	router := api.NewRouter(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", resp["status"])
	}
}

func TestCreateProjectBadRequest(t *testing.T) {
	router := api.NewRouter(nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/projects", bytes.NewBufferString("not json"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for bad JSON, got %d", w.Code)
	}
}

func TestCreateProjectMissingTopic(t *testing.T) {
	router := api.NewRouter(nil, nil)
	body, _ := json.Marshal(map[string]any{"language": "en"})
	req := httptest.NewRequest(http.MethodPost, "/v1/projects", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing topic, got %d", w.Code)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	router := api.NewRouter(nil, nil)
	req := httptest.NewRequest(http.MethodDelete, "/v1/templates", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}
