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
	"github.com/ismobaga/synt/internal/publisher"
)

// ProjectHandler handles project-related HTTP endpoints.
type ProjectHandler struct {
	db           *db.DB
	orchestrator *orchestrator.Orchestrator
	publisher    *publisher.Service
}

type pipelineStepDescriptor struct {
	Stage   string
	JobType string
	Label   string
}

var pipelineStepDescriptors = []pipelineStepDescriptor{
	{Stage: db.StageSourceFetch, JobType: db.JobTypeSourceFetch, Label: "Fetching sources"},
	{Stage: db.StageScriptGeneration, JobType: db.JobTypeScriptGenerate, Label: "Generating script"},
	{Stage: db.StageScriptValidation, JobType: db.JobTypeScriptValidate, Label: "Validating script"},
	{Stage: db.StageMediaSearch, JobType: db.JobTypeMediaSearch, Label: "Searching media"},
	{Stage: db.StageMediaPrepare, JobType: db.JobTypeMediaPrepare, Label: "Preparing media"},
	{Stage: db.StageVoiceGeneration, JobType: db.JobTypeVoiceGenerate, Label: "Generating voice"},
	{Stage: db.StageSubtitleGeneration, JobType: db.JobTypeSubtitleGenerate, Label: "Generating subtitles"},
	{Stage: db.StageMusicSelection, JobType: db.JobTypeMusicSelect, Label: "Selecting music"},
	{Stage: db.StageTimelineBuild, JobType: db.JobTypeTimelineBuild, Label: "Building timeline"},
	{Stage: db.StageRenderPreview, JobType: db.JobTypeRenderPreview, Label: "Rendering preview"},
	{Stage: db.StageRenderFinal, JobType: db.JobTypeRenderFinal, Label: "Rendering final video"},
	{Stage: db.StageRenderThumbnail, JobType: db.JobTypeRenderThumbnail, Label: "Extracting thumbnail"},
	{Stage: db.StageFinalize, JobType: db.JobTypeProjectFinalize, Label: "Finalizing project"},
	{Stage: db.StageYouTubePublish, JobType: db.JobTypeYouTubePublish, Label: "Publishing to YouTube"},
}

func buildProjectStatusSteps(recentJobs []*db.Job) []PipelineStepStatusResponse {
	latestByType := make(map[string]*db.Job, len(recentJobs))
	for _, job := range recentJobs {
		if job == nil {
			continue
		}
		existing := latestByType[job.JobType]
		if existing == nil || job.CreatedAt.After(existing.CreatedAt) {
			latestByType[job.JobType] = job
		}
	}

	steps := make([]PipelineStepStatusResponse, 0, len(pipelineStepDescriptors))
	for _, descriptor := range pipelineStepDescriptors {
		step := PipelineStepStatusResponse{
			JobType: descriptor.JobType,
			Stage:   descriptor.Stage,
			Label:   descriptor.Label,
			Status:  "not_started",
		}
		if job := latestByType[descriptor.JobType]; job != nil {
			step.Status = job.Status
			step.Attempts = job.Attempts
			step.MaxAttempts = job.MaxAttempts
			step.LastError = job.LastError
			step.ScheduledAt = job.ScheduledAt
			step.StartedAt = job.StartedAt
			step.FinishedAt = job.FinishedAt
			if job.StartedAt != nil && job.FinishedAt != nil {
				duration := job.FinishedAt.Sub(*job.StartedAt)
				if duration > 0 {
					step.DurationMs = duration.Milliseconds()
				}
			}
		}
		steps = append(steps, step)
	}
	return steps
}

// NewProjectHandler creates a new ProjectHandler.
func NewProjectHandler(database *db.DB, orch *orchestrator.Orchestrator, pub *publisher.Service) *ProjectHandler {
	return &ProjectHandler{db: database, orchestrator: orch, publisher: pub}
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
	req.AutoRender = true
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req = GenerateRequest{AutoRender: true}
	}

	if err := h.orchestrator.TriggerGeneration(r.Context(), uid, req.AutoRender, req.AutoPublishYouTube); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to trigger generation")
		return
	}

	writeJSON(w, http.StatusOK, GenerateResponse{
		Status:       db.ProjectStatusQueued,
		CurrentStage: db.StageSourceFetch,
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
	var steps []PipelineStepStatusResponse
	if recentJobs, jobsErr := h.db.ListJobsByProject(r.Context(), uid, 64); jobsErr != nil {
		log.Printf("failed to load job history for project %s: %v", uid, jobsErr)
	} else {
		steps = buildProjectStatusSteps(recentJobs)
	}
	writeJSON(w, http.StatusOK, ProjectStatusResponse{
		ID:           project.ID.String(),
		Status:       project.Status,
		CurrentStage: project.CurrentStage,
		ErrorMessage: project.ErrorMessage,
		UpdatedAt:    project.UpdatedAt,
		Steps:        steps,
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
	if err := h.db.UpdateScript(r.Context(), uid, req.Content.Title, req.Content.Hook, req.Content.CTA, req.Content.Language, contentJSON); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update script")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func mimeTypeForAssetType(assetType string) string {
	switch strings.ToLower(strings.TrimSpace(assetType)) {
	case "video":
		return "video/mp4"
	case "image":
		return "image/jpeg"
	default:
		return "application/octet-stream"
	}
}

// UpdateAsset handles PUT /v1/projects/:id/assets/:assetId.
func (h *ProjectHandler) UpdateAsset(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(pathParam(r, "projects"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	assetID, err := uuid.Parse(pathParam(r, "assets"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid asset id")
		return
	}

	var req UpdateAssetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	assets, err := h.db.GetAssets(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load assets")
		return
	}

	var current *db.Asset
	for _, asset := range assets {
		if asset != nil && asset.ID == assetID {
			current = asset
			break
		}
	}
	if current == nil {
		writeError(w, http.StatusNotFound, "asset not found")
		return
	}

	if value := strings.TrimSpace(req.Type); value != "" {
		current.Type = value
	}
	if value := strings.TrimSpace(req.Provider); value != "" {
		current.Provider = value
	}
	if value := strings.TrimSpace(req.URL); value != "" {
		current.URL = value
		if strings.TrimSpace(req.StoragePath) == "" {
			current.StoragePath = value
		}
	}
	if value := strings.TrimSpace(req.StoragePath); value != "" {
		current.StoragePath = value
	}
	if value := strings.TrimSpace(req.MimeType); value != "" {
		current.MimeType = value
	} else if strings.TrimSpace(current.MimeType) == "" {
		current.MimeType = mimeTypeForAssetType(current.Type)
	}
	if req.Metadata != nil {
		metadata, err := json.Marshal(req.Metadata)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid metadata")
			return
		}
		current.Metadata = metadata
	}

	if err := h.db.UpdateAsset(r.Context(), current); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update asset")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func rerunJobType(step string) (string, string, error) {
	switch strings.ToLower(strings.TrimSpace(step)) {
	case "source", "sources", "source_fetch":
		return db.JobTypeSourceFetch, db.StageSourceFetch, nil
	case "script", "script_generation":
		return db.JobTypeScriptGenerate, db.StageScriptGeneration, nil
	case "validate", "script_validation":
		return db.JobTypeScriptValidate, db.StageScriptValidation, nil
	case "media", "media_search":
		return db.JobTypeMediaSearch, db.StageMediaSearch, nil
	case "media_prepare":
		return db.JobTypeMediaPrepare, db.StageMediaPrepare, nil
	case "voice", "voice_generation":
		return db.JobTypeVoiceGenerate, db.StageVoiceGeneration, nil
	case "subtitles", "subtitle_generation":
		return db.JobTypeSubtitleGenerate, db.StageSubtitleGeneration, nil
	case "music", "music_selection":
		return db.JobTypeMusicSelect, db.StageMusicSelection, nil
	case "timeline", "timeline_build":
		return db.JobTypeTimelineBuild, db.StageTimelineBuild, nil
	case "preview", "render_preview":
		return db.JobTypeRenderPreview, db.StageRenderPreview, nil
	case "final", "render_final":
		return db.JobTypeRenderFinal, db.StageRenderFinal, nil
	default:
		return "", "", http.ErrNotSupported
	}
}

// RerunStep handles POST /v1/projects/:id/steps/:step/rerun.
func (h *ProjectHandler) RerunStep(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(pathParam(r, "projects"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	step := pathParam(r, "steps")
	jobType, stage, err := rerunJobType(step)
	if err != nil {
		writeError(w, http.StatusBadRequest, "unsupported step")
		return
	}

	if err := h.db.UpdateProjectStatus(r.Context(), projectID, db.ProjectStatusQueued, stage, ""); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update project status")
		return
	}
	payload, _ := json.Marshal(map[string]any{"auto_render": false, "source": "manual_rerun", "step": step})
	if err := h.orchestrator.EnqueueJob(r.Context(), projectID, jobType, payload); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to queue step rerun")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "queued", "step": step})
}

// RegenerateScript handles POST /v1/projects/:id/script/regenerate.
func (h *ProjectHandler) RegenerateScript(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r, "projects")
	uid, err := uuid.Parse(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	payload, _ := json.Marshal(map[string]any{"auto_render": false, "source": "manual_regenerate", "step": "script"})
	if err := h.db.UpdateProjectStatus(r.Context(), uid, db.ProjectStatusQueued, db.StageSourceFetch, ""); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update project status")
		return
	}
	if err := h.orchestrator.EnqueueJob(r.Context(), uid, db.JobTypeSourceFetch, payload); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enqueue source fetch")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "queued", "step": "source_fetch"})
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

func latestFinalRender(renders []*db.Render) *db.Render {
	for _, render := range renders {
		if render != nil && render.Kind == "final" && render.Status == "done" {
			return render
		}
	}
	return nil
}

func defaultYouTubeTitle(project *db.Project) string {
	if project == nil {
		return "Synt Generated Video"
	}
	if title := strings.TrimSpace(project.Topic); title != "" {
		return title
	}
	return "Synt Generated Video"
}

// PublishYouTube handles POST /v1/projects/:id/publish/youtube.
func (h *ProjectHandler) PublishYouTube(w http.ResponseWriter, r *http.Request) {
	if h.publisher == nil || !h.publisher.YouTubeConfigured() {
		writeError(w, http.StatusBadRequest, "youtube publishing is not configured")
		return
	}

	projectID, err := uuid.Parse(pathParam(r, "projects"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	project, err := h.db.GetProject(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	renders, err := h.db.GetRenders(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load renders")
		return
	}
	finalRender := latestFinalRender(renders)
	if finalRender == nil || strings.TrimSpace(finalRender.StoragePath) == "" {
		writeError(w, http.StatusBadRequest, "no completed final render available")
		return
	}

	var req PublishYouTubeRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	result, err := h.publisher.PublishToYouTube(r.Context(), publisher.YouTubePublishRequest{
		VideoPath:     strings.TrimSpace(finalRender.StoragePath),
		Title:         firstNonEmpty(strings.TrimSpace(req.Title), defaultYouTubeTitle(project)),
		Description:   strings.TrimSpace(req.Description),
		PrivacyStatus: strings.TrimSpace(req.PrivacyStatus),
		Tags:          req.Tags,
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, "youtube publish failed: "+err.Error())
		return
	}

	metadata, _ := json.Marshal(map[string]any{
		"video_id":       result.VideoID,
		"watch_url":      result.WatchURL,
		"published_at":   result.PublishedAt.Format(time.RFC3339),
		"publish_method": "manual",
	})
	asset := &db.Asset{
		ID:          uuid.New(),
		ProjectID:   &projectID,
		Type:        "distribution",
		Source:      "generated",
		Provider:    "youtube",
		URL:         result.WatchURL,
		StoragePath: result.WatchURL,
		MimeType:    "text/uri-list",
		Metadata:    metadata,
		CreatedAt:   time.Now().UTC(),
	}
	if err := h.db.CreateAsset(r.Context(), asset); err != nil {
		log.Printf("failed to persist youtube publish metadata for project %s: %v", projectID, err)
	}

	writeJSON(w, http.StatusOK, PublishYouTubeResponse{
		Status:      "published",
		ProjectID:   projectID.String(),
		VideoID:     result.VideoID,
		WatchURL:    result.WatchURL,
		PublishedAt: result.PublishedAt.Format(time.RFC3339),
	})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
