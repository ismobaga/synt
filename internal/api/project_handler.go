package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
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

func normalizeRenderEngine(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case RenderEngineRemotion, "react", "react-composition":
		return RenderEngineRemotion
	default:
		return RenderEngineFFmpeg
	}
}

func resolveProjectTemplateID(templateID, renderEngine string) string {
	trimmed := strings.TrimSpace(templateID)
	if trimmed == "" {
		trimmed = "fast_caption_v1"
	}

	lowered := strings.ToLower(trimmed)
	wantsRemotion := normalizeRenderEngine(renderEngine) == RenderEngineRemotion
	if strings.HasPrefix(lowered, "remotion_") {
		if wantsRemotion || strings.TrimSpace(renderEngine) == "" {
			return trimmed
		}
		return strings.TrimPrefix(trimmed, "remotion_")
	}
	if wantsRemotion {
		return "remotion_" + trimmed
	}
	return trimmed
}

func normalizeSourceURLs(values []string) []string {
	seen := make(map[string]bool, len(values))
	normalized := make([]string, 0, len(values))
	for _, raw := range values {
		for _, candidate := range strings.FieldsFunc(raw, func(r rune) bool {
			return r == '\n' || r == '\r' || r == ','
		}) {
			trimmed := strings.TrimSpace(candidate)
			lowered := strings.ToLower(trimmed)
			if trimmed == "" || (!strings.HasPrefix(lowered, "http://") && !strings.HasPrefix(lowered, "https://")) {
				continue
			}
			if seen[trimmed] {
				continue
			}
			seen[trimmed] = true
			normalized = append(normalized, trimmed)
		}
	}
	return normalized
}

func sourceProviderForURL(raw string) string {
	lowered := strings.ToLower(strings.TrimSpace(raw))
	if strings.Contains(lowered, "youtube.com") || strings.Contains(lowered, "youtu.be") {
		return "youtube"
	}
	return "webpage"
}

func (h *ProjectHandler) storeSourceMaterial(ctx context.Context, projectID uuid.UUID, req CreateProjectRequest) error {
	for index, rawURL := range normalizeSourceURLs(req.SourceURLs) {
		metadata, err := json.Marshal(map[string]any{
			"kind":         "source_url",
			"input_order":  index + 1,
			"fetch_status": "pending",
		})
		if err != nil {
			return err
		}
		asset := &db.Asset{
			ID:          uuid.New(),
			ProjectID:   &projectID,
			Type:        "source_material",
			Source:      "user_input",
			Provider:    sourceProviderForURL(rawURL),
			URL:         rawURL,
			StoragePath: "",
			MimeType:    "text/uri-list",
			Metadata:    metadata,
			CreatedAt:   time.Now().UTC(),
		}
		if err := h.db.CreateAsset(ctx, asset); err != nil {
			return err
		}
	}

	if note := strings.TrimSpace(req.SourceNotes); note != "" {
		metadata, err := json.Marshal(map[string]any{
			"kind":  "source_note",
			"notes": note,
		})
		if err != nil {
			return err
		}
		asset := &db.Asset{
			ID:          uuid.New(),
			ProjectID:   &projectID,
			Type:        "source_note",
			Source:      "user_input",
			Provider:    "manual_note",
			URL:         "",
			StoragePath: "",
			MimeType:    "text/plain",
			Metadata:    metadata,
			CreatedAt:   time.Now().UTC(),
		}
		if err := h.db.CreateAsset(ctx, asset); err != nil {
			return err
		}
	}

	return nil
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
	req.TemplateID = resolveProjectTemplateID(req.TemplateID, req.RenderEngine)

	userID, err := ensureDefaultUser(r.Context(), h.db)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to initialize default user")
		log.Println("failed to initialize default user:", err)
		return
	}

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
		writeError(w, http.StatusInternalServerError, "failed to create project: "+err.Error())
		log.Println("failed to create project:", err)
		return
	}
	if err := h.storeSourceMaterial(r.Context(), project.ID, req); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store source material")
		log.Println("failed to store source material:", err)
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
