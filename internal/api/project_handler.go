package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/ismobaga/synt/internal/db"
	"github.com/ismobaga/synt/internal/orchestrator"
)

// ProjectHandler handles project-related HTTP endpoints.
type ProjectHandler struct {
	db           *db.DB
	orchestrator *orchestrator.Orchestrator
}

// NewProjectHandler creates a new ProjectHandler.
func NewProjectHandler(database *db.DB, orch *orchestrator.Orchestrator) *ProjectHandler {
	return &ProjectHandler{db: database, orchestrator: orch}
}

// Create handles POST /v1/projects.
func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Topic == "" {
		writeError(w, http.StatusBadRequest, "topic is required")
		return
	}
	if req.Language == "" {
		req.Language = "en"
	}
	if req.Platform == "" {
		req.Platform = PlatformYouTubeShorts
	}
	if req.DurationSec == 0 {
		req.DurationSec = 30
	}
	if req.TemplateID == "" {
		req.TemplateID = "fast_caption_v1"
	}

	// Use a placeholder user ID until auth middleware is in place.
	userID := uuid.New()

	project := &db.Project{
		ID:           uuid.New(),
		UserID:       userID,
		Topic:        req.Topic,
		Language:     req.Language,
		Platform:     req.Platform,
		DurationSec:  req.DurationSec,
		Tone:         req.Tone,
		TemplateID:   req.TemplateID,
		Status:       db.ProjectStatusDraft,
		CurrentStage: db.StageCreated,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	if err := h.db.CreateProject(r.Context(), project); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create project")
		return
	}

	writeJSON(w, http.StatusCreated, CreateProjectResponse{
		ID:           project.ID.String(),
		Status:       project.Status,
		CurrentStage: project.CurrentStage,
	})
}

// Get handles GET /v1/projects/:id.
func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r, "projects")
	if id == "" {
		writeError(w, http.StatusBadRequest, "project id required")
		return
	}
	uid, err := uuid.Parse(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	project, err := h.db.GetProject(r.Context(), uid)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	writeJSON(w, http.StatusOK, project)
}

// List handles GET /v1/projects.
func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	projects, err := h.db.ListProjects(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list projects")
		return
	}
	writeJSON(w, http.StatusOK, projects)
}

// Delete handles DELETE /v1/projects/:id.
func (h *ProjectHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r, "projects")
	uid, err := uuid.Parse(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	if err := h.db.DeleteProject(r.Context(), uid); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete project")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Generate handles POST /v1/projects/:id/generate.
func (h *ProjectHandler) Generate(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r, "projects")
	uid, err := uuid.Parse(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.AutoRender = true
	}

	if err := h.orchestrator.TriggerGeneration(r.Context(), uid, req.AutoRender); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to trigger generation")
		return
	}

	writeJSON(w, http.StatusOK, GenerateResponse{
		Status:       db.ProjectStatusQueued,
		CurrentStage: db.StageScriptGeneration,
	})
}

// Status handles GET /v1/projects/:id/status.
func (h *ProjectHandler) Status(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r, "projects")
	uid, err := uuid.Parse(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	project, err := h.db.GetProject(r.Context(), uid)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	writeJSON(w, http.StatusOK, ProjectStatusResponse{
		ID:           project.ID.String(),
		Status:       project.Status,
		CurrentStage: project.CurrentStage,
		ErrorMessage: project.ErrorMessage,
		UpdatedAt:    project.UpdatedAt,
	})
}

// Retry handles POST /v1/projects/:id/retry.
func (h *ProjectHandler) Retry(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r, "projects")
	uid, err := uuid.Parse(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	if err := h.orchestrator.RetryProject(r.Context(), uid); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retry project")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "retrying"})
}

// GetScript handles GET /v1/projects/:id/script.
func (h *ProjectHandler) GetScript(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r, "projects")
	uid, err := uuid.Parse(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	script, err := h.db.GetScript(r.Context(), uid)
	if err != nil {
		writeError(w, http.StatusNotFound, "script not found")
		return
	}
	writeJSON(w, http.StatusOK, script)
}

// UpdateScript handles PUT /v1/projects/:id/script.
func (h *ProjectHandler) UpdateScript(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r, "projects")
	uid, err := uuid.Parse(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	var req UpdateScriptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	contentJSON, err := json.Marshal(req.Content)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid script content")
		return
	}
	if err := h.db.UpdateScript(r.Context(), uid, contentJSON); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update script")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RegenerateScript handles POST /v1/projects/:id/script/regenerate.
func (h *ProjectHandler) RegenerateScript(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r, "projects")
	uid, err := uuid.Parse(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	if err := h.orchestrator.EnqueueJob(r.Context(), uid, db.JobTypeScriptGenerate, nil); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enqueue script generation")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "queued"})
}

// GetAssets handles GET /v1/projects/:id/assets.
func (h *ProjectHandler) GetAssets(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r, "projects")
	uid, err := uuid.Parse(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	assets, err := h.db.GetAssets(r.Context(), uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get assets")
		return
	}
	writeJSON(w, http.StatusOK, assets)
}

// GetAudio handles GET /v1/projects/:id/audio.
func (h *ProjectHandler) GetAudio(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r, "projects")
	uid, err := uuid.Parse(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	tracks, err := h.db.GetAudioTracks(r.Context(), uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get audio")
		return
	}
	writeJSON(w, http.StatusOK, tracks)
}

// GetSubtitles handles GET /v1/projects/:id/subtitles.
func (h *ProjectHandler) GetSubtitles(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r, "projects")
	uid, err := uuid.Parse(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	subtitles, err := h.db.GetSubtitles(r.Context(), uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get subtitles")
		return
	}
	writeJSON(w, http.StatusOK, subtitles)
}

// RegenerateSubtitles handles POST /v1/projects/:id/subtitles/regenerate.
func (h *ProjectHandler) RegenerateSubtitles(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r, "projects")
	uid, err := uuid.Parse(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	if err := h.orchestrator.EnqueueJob(r.Context(), uid, db.JobTypeSubtitleGenerate, nil); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enqueue subtitle generation")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "queued"})
}

// RenderPreview handles POST /v1/projects/:id/render/preview.
func (h *ProjectHandler) RenderPreview(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r, "projects")
	uid, err := uuid.Parse(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	if err := h.orchestrator.EnqueueJob(r.Context(), uid, db.JobTypeRenderPreview, nil); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enqueue preview render")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "queued", "kind": "preview"})
}

// RenderFinal handles POST /v1/projects/:id/render/final.
func (h *ProjectHandler) RenderFinal(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r, "projects")
	uid, err := uuid.Parse(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	if err := h.orchestrator.EnqueueJob(r.Context(), uid, db.JobTypeRenderFinal, nil); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enqueue final render")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "queued", "kind": "final"})
}

// GetRender handles GET /v1/projects/:id/render.
func (h *ProjectHandler) GetRender(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r, "projects")
	uid, err := uuid.Parse(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	renders, err := h.db.GetRenders(r.Context(), uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get renders")
		return
	}
	writeJSON(w, http.StatusOK, renders)
}
