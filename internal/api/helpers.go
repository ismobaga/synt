package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

// writeJSON encodes v as JSON and writes it to the response.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, "encoding error", http.StatusInternalServerError)
	}
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, ErrorResponse{Error: msg})
}

// pathParam extracts a path parameter from the URL.
// For a path like /v1/projects/abc123/script, calling pathParam(r, "projects")
// returns "abc123".
func pathParam(r *http.Request, segment string) string {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	for i, p := range parts {
		if p == segment && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}
