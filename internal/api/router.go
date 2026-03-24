package api

import (
	"net/http"
	"strings"

	"github.com/ismobaga/synt/internal/db"
	"github.com/ismobaga/synt/internal/orchestrator"
)

// Router sets up and returns the HTTP mux for the API.
func NewRouter(database *db.DB, orch *orchestrator.Orchestrator) http.Handler {
	mux := http.NewServeMux()

	projects := NewProjectHandler(database, orch)
	templates := NewTemplateHandler(database)
	brandKits := NewBrandKitHandler(database)

	// Project CRUD
	mux.HandleFunc("/v1/projects", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			projects.Create(w, r)
		case http.MethodGet:
			projects.List(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	mux.HandleFunc("/v1/projects/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/v1/projects/")
		parts := strings.Split(strings.Trim(path, "/"), "/")

		if len(parts) == 1 {
			// /v1/projects/:id
			switch r.Method {
			case http.MethodGet:
				projects.Get(w, r)
			case http.MethodDelete:
				projects.Delete(w, r)
			default:
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			}
			return
		}

		if len(parts) >= 2 {
			switch parts[1] {
			case "generate":
				if r.Method == http.MethodPost {
					projects.Generate(w, r)
				} else {
					writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				}
			case "status":
				if r.Method == http.MethodGet {
					projects.Status(w, r)
				} else {
					writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				}
			case "retry":
				if r.Method == http.MethodPost {
					projects.Retry(w, r)
				} else {
					writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				}
			case "script":
				if len(parts) == 3 && parts[2] == "regenerate" {
					if r.Method == http.MethodPost {
						projects.RegenerateScript(w, r)
					} else {
						writeError(w, http.StatusMethodNotAllowed, "method not allowed")
					}
				} else {
					switch r.Method {
					case http.MethodGet:
						projects.GetScript(w, r)
					case http.MethodPut:
						projects.UpdateScript(w, r)
					default:
						writeError(w, http.StatusMethodNotAllowed, "method not allowed")
					}
				}
			case "assets":
				if r.Method == http.MethodGet {
					projects.GetAssets(w, r)
				} else {
					writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				}
			case "audio":
				if r.Method == http.MethodGet {
					projects.GetAudio(w, r)
				} else {
					writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				}
			case "subtitles":
				if len(parts) == 3 && parts[2] == "regenerate" {
					if r.Method == http.MethodPost {
						projects.RegenerateSubtitles(w, r)
					} else {
						writeError(w, http.StatusMethodNotAllowed, "method not allowed")
					}
				} else {
					if r.Method == http.MethodGet {
						projects.GetSubtitles(w, r)
					} else {
						writeError(w, http.StatusMethodNotAllowed, "method not allowed")
					}
				}
			case "render":
				if len(parts) == 3 {
					switch parts[2] {
					case "preview":
						if r.Method == http.MethodPost {
							projects.RenderPreview(w, r)
						} else {
							writeError(w, http.StatusMethodNotAllowed, "method not allowed")
						}
					case "final":
						if r.Method == http.MethodPost {
							projects.RenderFinal(w, r)
						} else {
							writeError(w, http.StatusMethodNotAllowed, "method not allowed")
						}
					default:
						writeError(w, http.StatusNotFound, "not found")
					}
				} else {
					if r.Method == http.MethodGet {
						projects.GetRender(w, r)
					} else {
						writeError(w, http.StatusMethodNotAllowed, "method not allowed")
					}
				}
			default:
				writeError(w, http.StatusNotFound, "not found")
			}
		}
	})

	// Templates
	mux.HandleFunc("/v1/templates", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			templates.List(w, r)
		} else {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	// Brand kits
	mux.HandleFunc("/v1/brand-kits", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			brandKits.List(w, r)
		case http.MethodPost:
			brandKits.Create(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	return mux
}
