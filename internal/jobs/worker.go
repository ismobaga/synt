// Package jobs provides the job queue worker.
package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ismobaga/synt/internal/content"
	"github.com/ismobaga/synt/internal/db"
	"github.com/ismobaga/synt/internal/media"
	"github.com/ismobaga/synt/internal/moderation"
	"github.com/ismobaga/synt/internal/music"
	"github.com/ismobaga/synt/internal/render"
	"github.com/ismobaga/synt/internal/source"
	"github.com/ismobaga/synt/internal/subtitle"
	"github.com/ismobaga/synt/internal/voice"
)

// Worker processes jobs from the queue.
type Worker struct {
	db           *db.DB
	source       *source.Service
	content      *content.Service
	media        *media.Service
	voice        *voice.Service
	subtitle     *subtitle.Service
	music        *music.Service
	render       *render.Service
	moderation   *moderation.Service
	pollInterval time.Duration
}

// Config holds worker configuration.
type Config struct {
	PollInterval time.Duration
}

type pipelineStepDef struct {
	Stage   string
	JobType string
	Label   string
}

var pipelineStepDefs = []pipelineStepDef{
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
}

// PipelineStepStatus exposes the latest observable state for a pipeline step.
type PipelineStepStatus struct {
	JobType     string     `json:"job_type"`
	Stage       string     `json:"stage"`
	Label       string     `json:"label"`
	Status      string     `json:"status"`
	Attempts    int        `json:"attempts"`
	MaxAttempts int        `json:"max_attempts"`
	LastError   string     `json:"last_error,omitempty"`
	ScheduledAt *time.Time `json:"scheduled_at,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
	DurationMs  int64      `json:"duration_ms,omitempty"`
}

// BuildPipelineStepStatuses returns a step-by-step view of the most recent job attempts.
func BuildPipelineStepStatuses(jobRecords []*db.Job) []PipelineStepStatus {
	return buildPipelineStepStatuses(jobRecords)
}

func buildPipelineStepStatuses(jobRecords []*db.Job) []PipelineStepStatus {
	latestByType := make(map[string]*db.Job, len(jobRecords))
	for _, job := range jobRecords {
		if job == nil {
			continue
		}
		existing := latestByType[job.JobType]
		if existing == nil || job.CreatedAt.After(existing.CreatedAt) {
			latestByType[job.JobType] = job
		}
	}

	steps := make([]PipelineStepStatus, 0, len(pipelineStepDefs))
	for _, def := range pipelineStepDefs {
		step := PipelineStepStatus{
			JobType: def.JobType,
			Stage:   def.Stage,
			Label:   def.Label,
			Status:  "not_started",
		}
		if job := latestByType[def.JobType]; job != nil {
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

// New creates a new Worker.
func New(
	database *db.DB,
	contentSvc *content.Service,
	mediaSvc *media.Service,
	voiceSvc *voice.Service,
	subtitleSvc *subtitle.Service,
	musicSvc *music.Service,
	renderSvc *render.Service,
	moderationSvc *moderation.Service,
	cfg Config,
) *Worker {
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 5 * time.Second
	}
	return &Worker{
		db:           database,
		source:       source.New(nil),
		content:      contentSvc,
		media:        mediaSvc,
		voice:        voiceSvc,
		subtitle:     subtitleSvc,
		music:        musicSvc,
		render:       renderSvc,
		moderation:   moderationSvc,
		pollInterval: cfg.PollInterval,
	}
}

// Run starts the worker loop. It blocks until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	log.Println("[worker] starting")
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Println("[worker] stopping")
			return
		case <-ticker.C:
			w.processBatch(ctx)
		}
	}
}

func (w *Worker) processBatch(ctx context.Context) {
	jobs, err := w.db.ListPendingJobs(ctx, 10)
	if err != nil {
		log.Printf("[worker] list jobs error: %v", err)
		return
	}
	for _, j := range jobs {
		if err := w.process(ctx, j); err != nil {
			log.Printf("[worker] bookkeeping error for job %s (%s): %v", j.ID, j.JobType, err)
		}
	}
}

func (w *Worker) process(ctx context.Context, j *db.Job) error {
	attempt := j.Attempts + 1
	stage := stageForJobType(j.JobType)
	log.Printf("[worker] start job=%s type=%s stage=%s project=%s attempt=%d/%d", j.ID, j.JobType, stage, j.ProjectID, attempt, j.MaxAttempts)
	if err := w.db.MarkJobRunning(ctx, j.ID); err != nil {
		return fmt.Errorf("mark job running: %w", err)
	}
	j.Attempts = attempt
	startedAt := time.Now().UTC()

	var err error
	switch j.JobType {
	case db.JobTypeProjectGenerate:
		err = w.handleProjectGenerate(ctx, j)
	case db.JobTypeSourceFetch:
		err = w.handleSourceFetch(ctx, j)
	case db.JobTypeScriptGenerate:
		err = w.handleScriptGenerate(ctx, j)
	case db.JobTypeScriptValidate:
		err = w.handleScriptValidate(ctx, j)
	case db.JobTypeMediaSearch:
		err = w.handleMediaSearch(ctx, j)
	case db.JobTypeMediaPrepare:
		err = w.handleMediaPrepare(ctx, j)
	case db.JobTypeVoiceGenerate:
		err = w.handleVoiceGenerate(ctx, j)
	case db.JobTypeSubtitleGenerate:
		err = w.handleSubtitleGenerate(ctx, j)
	case db.JobTypeMusicSelect:
		err = w.handleMusicSelect(ctx, j)
	case db.JobTypeTimelineBuild:
		err = w.handleTimelineBuild(ctx, j)
	case db.JobTypeRenderPreview:
		err = w.handleRenderPreview(ctx, j)
	case db.JobTypeRenderFinal:
		err = w.handleRenderFinal(ctx, j)
	case db.JobTypeRenderThumbnail:
		err = w.handleRenderThumbnail(ctx, j)
	case db.JobTypeProjectFinalize:
		err = w.handleProjectFinalize(ctx, j)
	default:
		err = fmt.Errorf("unknown job type: %s", j.JobType)
	}

	if err != nil {
		return w.handleJobFailure(ctx, j, err, startedAt)
	}

	duration := time.Since(startedAt).Round(10 * time.Millisecond)
	log.Printf("[worker] done job=%s type=%s stage=%s project=%s attempt=%d/%d duration=%s", j.ID, j.JobType, stage, j.ProjectID, j.Attempts, j.MaxAttempts, duration)
	return w.db.MarkJobDone(ctx, j.ID)
}

func (w *Worker) handleJobFailure(ctx context.Context, j *db.Job, err error, startedAt time.Time) error {
	stage := stageForJobType(j.JobType)
	duration := time.Since(startedAt).Round(10 * time.Millisecond)
	if shouldRetryJob(ctx, j, err) {
		delay := retryDelayForAttempt(j.Attempts)
		scheduledAt := time.Now().UTC().Add(delay)
		log.Printf("[worker] retry job=%s type=%s stage=%s project=%s next_in=%s attempt=%d/%d duration=%s error=%v", j.ID, j.JobType, stage, j.ProjectID, delay.Round(time.Second), j.Attempts+1, j.MaxAttempts, duration, err)
		if markErr := w.db.MarkJobRetrying(ctx, j.ID, err.Error(), scheduledAt); markErr != nil {
			return fmt.Errorf("mark job retrying: %w", markErr)
		}
		message := fmt.Sprintf("Retrying %s in %s (attempt %d/%d): %v", stage, delay.Round(time.Second), j.Attempts+1, j.MaxAttempts, err)
		return w.db.UpdateProjectStatus(ctx, j.ProjectID, db.ProjectStatusQueued, stage, message)
	}

	log.Printf("[worker] failed job=%s type=%s stage=%s project=%s attempt=%d/%d duration=%s error=%v", j.ID, j.JobType, stage, j.ProjectID, j.Attempts, j.MaxAttempts, duration, err)
	if markErr := w.db.MarkJobFailed(ctx, j.ID, err.Error()); markErr != nil {
		return fmt.Errorf("mark job failed: %w", markErr)
	}
	return w.db.UpdateProjectStatus(ctx, j.ProjectID, db.ProjectStatusFailed, stage, err.Error())
}

func (w *Worker) handleProjectGenerate(ctx context.Context, j *db.Job) error {
	_ = w.db.UpdateProjectStatus(ctx, j.ProjectID, db.ProjectStatusQueued, db.StageSourceFetch, "")
	return w.enqueueJob(ctx, j.ProjectID, db.JobTypeSourceFetch, j.Payload)
}

func (w *Worker) handleSourceFetch(ctx context.Context, j *db.Job) error {
	if err := w.hydrateSourceMaterials(ctx, j.ProjectID); err != nil {
		return err
	}
	return w.enqueueJob(ctx, j.ProjectID, db.JobTypeScriptGenerate, j.Payload)
}

func (w *Worker) hydrateSourceMaterials(ctx context.Context, projectID uuid.UUID) error {
	assets, err := w.db.GetAssets(ctx, projectID)
	if err != nil {
		return err
	}

	hasSources := false
	for _, asset := range assets {
		if asset != nil && asset.Type == "source_material" && strings.TrimSpace(asset.URL) != "" {
			hasSources = true
			break
		}
	}
	if !hasSources {
		log.Printf("[worker] source_fetch project=%s skipped (no source materials)", projectID)
		return nil
	}

	_ = w.db.UpdateProjectStatus(ctx, projectID, db.ProjectStatusProcessing, db.StageSourceFetch, "")
	for _, asset := range assets {
		if asset == nil || asset.Type != "source_material" || strings.TrimSpace(asset.URL) == "" {
			continue
		}
		if sourceMaterialAlreadyFetched(asset) {
			log.Printf("[worker] source_fetch project=%s asset=%s skipped (already fetched)", projectID, asset.ID)
			continue
		}

		fetchStarted := time.Now()
		result, fetchErr := w.source.Fetch(ctx, asset.URL)
		metadata := mergeSourceAssetMetadata(asset.Metadata, result, fetchErr)
		if err := w.db.UpdateAssetMetadata(ctx, asset.ID, metadata); err != nil {
			return err
		}
		if fetchErr != nil {
			log.Printf("[worker] source_fetch project=%s asset=%s duration=%s warning=%v", projectID, asset.ID, time.Since(fetchStarted).Round(10*time.Millisecond), fetchErr)
			continue
		}
		transcriptSource := ""
		if result != nil {
			transcriptSource = result.TranscriptSource
		}
		log.Printf("[worker] source_fetch project=%s asset=%s duration=%s transcript_source=%s", projectID, asset.ID, time.Since(fetchStarted).Round(10*time.Millisecond), transcriptSource)
	}
	return nil
}

func (w *Worker) enqueueJob(ctx context.Context, projectID uuid.UUID, jobType string, payload []byte) error {
	if payload == nil {
		payload = []byte("{}")
	}
	job := &db.Job{
		ID:          uuid.New(),
		ProjectID:   projectID,
		JobType:     jobType,
		Status:      db.JobStatusPending,
		Payload:     payload,
		Attempts:    0,
		MaxAttempts: 5,
		CreatedAt:   time.Now().UTC(),
	}
	if err := w.db.CreateJob(ctx, job); err != nil {
		return fmt.Errorf("create step job %s: %w", jobType, err)
	}
	return nil
}

func (w *Worker) handleScriptGenerate(ctx context.Context, j *db.Job) error {
	project, err := w.db.GetProject(ctx, j.ProjectID)
	if err != nil {
		return err
	}
	if err := w.hydrateSourceMaterials(ctx, j.ProjectID); err != nil {
		return fmt.Errorf("hydrate source material: %w", err)
	}
	_ = w.db.UpdateProjectStatus(ctx, j.ProjectID, db.ProjectStatusProcessing, db.StageScriptGeneration, "")

	assets, err := w.db.GetAssets(ctx, j.ProjectID)
	if err != nil {
		return err
	}
	sourceURLs, sourceNotes := sourceMaterialContextFromAssets(assets)

	req := content.GenerateRequest{
		Topic:       project.Topic,
		Platform:    project.Platform,
		DurationSec: project.DurationSec,
		Tone:        project.Tone,
		Language:    project.Language,
		SourceURLs:  sourceURLs,
		SourceNotes: sourceNotes,
	}
	script, err := w.content.Generate(ctx, req)
	if err != nil {
		return fmt.Errorf("generate script: %w", err)
	}

	contentJSON, err := json.Marshal(script)
	if err != nil {
		return err
	}

	s := &db.Script{
		ID:          uuid.New(),
		ProjectID:   j.ProjectID,
		Title:       script.Title,
		Hook:        script.Hook,
		CTA:         script.CTA,
		Language:    script.Language,
		ContentJSON: contentJSON,
		CreatedAt:   time.Now().UTC(),
	}
	if err := w.db.CreateScript(ctx, s); err != nil {
		return err
	}
	return w.enqueueJob(ctx, j.ProjectID, db.JobTypeScriptValidate, j.Payload)
}

func (w *Worker) handleScriptValidate(ctx context.Context, j *db.Job) error {
	_ = w.db.UpdateProjectStatus(ctx, j.ProjectID, db.ProjectStatusProcessing, db.StageScriptValidation, "")
	script, err := w.db.GetScript(ctx, j.ProjectID)
	if err != nil {
		return err
	}
	if err := w.moderation.ValidateScript(ctx, script.ContentJSON); err != nil {
		return err
	}
	return w.enqueueJob(ctx, j.ProjectID, db.JobTypeMediaSearch, j.Payload)
}

func (w *Worker) handleMediaSearch(ctx context.Context, j *db.Job) error {
	_ = w.db.UpdateProjectStatus(ctx, j.ProjectID, db.ProjectStatusProcessing, db.StageMediaSearch, "")
	script, err := w.db.GetScript(ctx, j.ProjectID)
	if err != nil {
		return err
	}
	assets, err := w.media.SearchAssets(ctx, j.ProjectID, script.ContentJSON)
	if err != nil {
		return err
	}
	for _, a := range assets {
		if err := w.db.CreateAsset(ctx, a); err != nil {
			return err
		}
	}
	return w.enqueueJob(ctx, j.ProjectID, db.JobTypeMediaPrepare, j.Payload)
}

func (w *Worker) handleMediaPrepare(ctx context.Context, j *db.Job) error {
	_ = w.db.UpdateProjectStatus(ctx, j.ProjectID, db.ProjectStatusProcessing, db.StageMediaPrepare, "")
	assets, err := w.db.GetAssets(ctx, j.ProjectID)
	if err != nil {
		return err
	}
	if err := w.media.PrepareAssets(ctx, assets); err != nil {
		return err
	}
	return w.enqueueJob(ctx, j.ProjectID, db.JobTypeVoiceGenerate, j.Payload)
}

func (w *Worker) handleVoiceGenerate(ctx context.Context, j *db.Job) error {
	_ = w.db.UpdateProjectStatus(ctx, j.ProjectID, db.ProjectStatusProcessing, db.StageVoiceGeneration, "")
	script, err := w.db.GetScript(ctx, j.ProjectID)
	if err != nil {
		return err
	}
	track, err := w.voice.Generate(ctx, j.ProjectID, script.ContentJSON, script.Language)
	if err != nil {
		return err
	}
	if err := w.db.CreateAudioTrack(ctx, track); err != nil {
		return err
	}
	return w.enqueueJob(ctx, j.ProjectID, db.JobTypeSubtitleGenerate, j.Payload)
}

func (w *Worker) handleSubtitleGenerate(ctx context.Context, j *db.Job) error {
	_ = w.db.UpdateProjectStatus(ctx, j.ProjectID, db.ProjectStatusProcessing, db.StageSubtitleGeneration, "")
	tracks, err := w.db.GetAudioTracks(ctx, j.ProjectID)
	if err != nil {
		return err
	}
	if len(tracks) == 0 {
		return fmt.Errorf("no audio tracks found for project %s", j.ProjectID)
	}
	sub, err := w.subtitle.Generate(ctx, j.ProjectID, tracks[0])
	if err != nil {
		return err
	}
	if err := w.db.CreateSubtitle(ctx, sub); err != nil {
		return err
	}
	return w.enqueueJob(ctx, j.ProjectID, db.JobTypeMusicSelect, j.Payload)
}

func (w *Worker) handleMusicSelect(ctx context.Context, j *db.Job) error {
	_ = w.db.UpdateProjectStatus(ctx, j.ProjectID, db.ProjectStatusProcessing, db.StageMusicSelection, "")
	script, err := w.db.GetScript(ctx, j.ProjectID)
	if err != nil {
		return err
	}
	track, err := w.music.Select(ctx, j.ProjectID, script.ContentJSON)
	if err != nil {
		return err
	}
	if err := w.db.CreateAudioTrack(ctx, track); err != nil {
		return err
	}
	return w.enqueueJob(ctx, j.ProjectID, db.JobTypeTimelineBuild, j.Payload)
}

func (w *Worker) handleTimelineBuild(ctx context.Context, j *db.Job) error {
	_ = w.db.UpdateProjectStatus(ctx, j.ProjectID, db.ProjectStatusProcessing, db.StageTimelineBuild, "")
	if err := w.render.BuildTimeline(ctx, j.ProjectID); err != nil {
		return err
	}
	if !shouldAutoRender(j.Payload) {
		return w.db.UpdateProjectStatus(ctx, j.ProjectID, db.ProjectStatusDone, db.StageTimelineBuild, "")
	}
	return w.enqueueJob(ctx, j.ProjectID, db.JobTypeRenderPreview, j.Payload)
}

func (w *Worker) handleRenderPreview(ctx context.Context, j *db.Job) error {
	_ = w.db.UpdateProjectStatus(ctx, j.ProjectID, db.ProjectStatusProcessing, db.StageRenderPreview, "")
	if err := w.render.RenderPreview(ctx, j.ProjectID); err != nil {
		return err
	}
	if !shouldAutoRender(j.Payload) {
		return w.db.UpdateProjectStatus(ctx, j.ProjectID, db.ProjectStatusDone, db.StageRenderPreview, "")
	}
	return w.enqueueJob(ctx, j.ProjectID, db.JobTypeRenderFinal, j.Payload)
}

func (w *Worker) handleRenderFinal(ctx context.Context, j *db.Job) error {
	_ = w.db.UpdateProjectStatus(ctx, j.ProjectID, db.ProjectStatusProcessing, db.StageRenderFinal, "")
	if err := w.render.RenderFinal(ctx, j.ProjectID); err != nil {
		return err
	}
	return w.enqueueJob(ctx, j.ProjectID, db.JobTypeRenderThumbnail, j.Payload)
}

func (w *Worker) handleRenderThumbnail(ctx context.Context, j *db.Job) error {
	_ = w.db.UpdateProjectStatus(ctx, j.ProjectID, db.ProjectStatusProcessing, db.StageRenderThumbnail, "")
	if err := w.render.ExtractThumbnail(ctx, j.ProjectID); err != nil {
		log.Printf("[worker] thumbnail extraction warning project=%s: %v", j.ProjectID, err)
	}
	return w.enqueueJob(ctx, j.ProjectID, db.JobTypeProjectFinalize, j.Payload)
}

func (w *Worker) handleProjectFinalize(ctx context.Context, j *db.Job) error {
	return w.db.UpdateProjectStatus(ctx, j.ProjectID, db.ProjectStatusDone, db.StageFinalize, "")
}

func sourceMaterialContextFromAssets(assets []*db.Asset) ([]string, string) {
	urls := make([]string, 0)
	notes := make([]string, 0)
	seen := map[string]bool{}

	for _, asset := range assets {
		if asset == nil {
			continue
		}
		switch asset.Type {
		case "source_material":
			trimmed := strings.TrimSpace(asset.URL)
			if trimmed != "" && !seen[trimmed] {
				seen[trimmed] = true
				urls = append(urls, trimmed)
			}
			if len(asset.Metadata) == 0 {
				continue
			}
			var payload struct {
				Title            string   `json:"title"`
				ContentText      string   `json:"content_text"`
				Transcript       string   `json:"transcript_text"`
				FetchStatus      string   `json:"fetch_status"`
				TranscriptSource string   `json:"transcript_source"`
				GroundingQuality string   `json:"grounding_quality"`
				GroundingFacts   []string `json:"grounding_facts"`
			}
			if err := json.Unmarshal(asset.Metadata, &payload); err == nil {
				if title := strings.TrimSpace(payload.Title); title != "" {
					notes = append(notes, "Source title: "+title)
				}
				if quality := strings.TrimSpace(payload.GroundingQuality); quality != "" {
					notes = append(notes, "Grounding quality: "+quality)
				}
				if source := strings.TrimSpace(payload.TranscriptSource); source != "" {
					notes = append(notes, "Transcript source: "+source)
				}
				if len(payload.GroundingFacts) > 0 {
					notes = append(notes, "Grounded facts:\n- "+strings.Join(payload.GroundingFacts, "\n- "))
				}
				if excerpt := promptExcerpt(payload.ContentText, 1800); excerpt != "" {
					notes = append(notes, "Fetched source excerpt: "+excerpt)
				}
				if transcript := promptExcerpt(payload.Transcript, 2200); transcript != "" {
					notes = append(notes, "Fetched video transcript: "+transcript)
				}
			}
		case "source_note":
			if len(asset.Metadata) == 0 {
				continue
			}
			var payload struct {
				Notes string `json:"notes"`
				Text  string `json:"text"`
			}
			if err := json.Unmarshal(asset.Metadata, &payload); err == nil {
				if note := strings.TrimSpace(payload.Notes); note != "" {
					notes = append(notes, note)
					continue
				}
				if note := strings.TrimSpace(payload.Text); note != "" {
					notes = append(notes, note)
				}
			}
		}
	}

	return urls, strings.Join(notes, "\n\n")
}

func sourceMaterialAlreadyFetched(asset *db.Asset) bool {
	if asset == nil || len(asset.Metadata) == 0 {
		return false
	}
	var payload struct {
		FetchStatus string `json:"fetch_status"`
		ContentText string `json:"content_text"`
		Transcript  string `json:"transcript_text"`
	}
	if err := json.Unmarshal(asset.Metadata, &payload); err != nil {
		return false
	}
	return strings.EqualFold(payload.FetchStatus, "fetched") && (strings.TrimSpace(payload.ContentText) != "" || strings.TrimSpace(payload.Transcript) != "")
}

func mergeSourceAssetMetadata(existing []byte, result *source.Result, fetchErr error) []byte {
	payload := map[string]any{}
	if len(existing) > 0 {
		_ = json.Unmarshal(existing, &payload)
	}
	payload["fetched_at"] = time.Now().UTC().Format(time.RFC3339)
	if fetchErr != nil {
		payload["fetch_status"] = "failed"
		payload["fetch_error"] = fetchErr.Error()
	} else {
		payload["fetch_status"] = "fetched"
		delete(payload, "fetch_error")
	}
	if result != nil {
		if result.Provider != "" {
			payload["resolved_provider"] = result.Provider
		}
		if result.Title != "" {
			payload["title"] = result.Title
		}
		if result.Content != "" {
			payload["content_text"] = result.Content
		}
		if result.Transcript != "" {
			payload["transcript_text"] = result.Transcript
		}
		if result.TranscriptSource != "" {
			payload["transcript_source"] = result.TranscriptSource
		}
		if result.GroundingQuality != "" {
			payload["grounding_quality"] = result.GroundingQuality
		}
		if len(result.Facts) > 0 {
			payload["grounding_facts"] = result.Facts
		}
	}
	metadata, err := json.Marshal(payload)
	if err != nil {
		return existing
	}
	return metadata
}

func promptExcerpt(value string, maxLen int) string {
	trimmed := strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if trimmed == "" || maxLen <= 0 {
		return trimmed
	}
	runes := []rune(trimmed)
	if len(runes) <= maxLen {
		return trimmed
	}
	return string(runes[:maxLen]) + "…"
}

func shouldRetryJob(ctx context.Context, j *db.Job, err error) bool {
	if err == nil || j == nil || ctx.Err() != nil || j.Attempts >= j.MaxAttempts {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	for _, hint := range []string{"unknown job type", "unsupported step", "project not found", "template not found"} {
		if strings.Contains(message, hint) {
			return false
		}
	}
	return true
}

func retryDelayForAttempt(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := 5 * time.Second
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay >= 2*time.Minute {
			return 2 * time.Minute
		}
	}
	return delay
}

func shouldAutoRender(payload []byte) bool {
	if len(payload) == 0 {
		return true
	}
	var request struct {
		AutoRender *bool `json:"auto_render"`
	}
	if err := json.Unmarshal(payload, &request); err != nil || request.AutoRender == nil {
		return true
	}
	return *request.AutoRender
}

func stageForJobType(jobType string) string {
	switch jobType {
	case db.JobTypeProjectGenerate, db.JobTypeSourceFetch:
		return db.StageSourceFetch
	case db.JobTypeScriptGenerate:
		return db.StageScriptGeneration
	case db.JobTypeScriptValidate:
		return db.StageScriptValidation
	case db.JobTypeMediaSearch:
		return db.StageMediaSearch
	case db.JobTypeMediaPrepare:
		return db.StageMediaPrepare
	case db.JobTypeVoiceGenerate:
		return db.StageVoiceGeneration
	case db.JobTypeSubtitleGenerate:
		return db.StageSubtitleGeneration
	case db.JobTypeMusicSelect:
		return db.StageMusicSelection
	case db.JobTypeTimelineBuild:
		return db.StageTimelineBuild
	case db.JobTypeRenderPreview:
		return db.StageRenderPreview
	case db.JobTypeRenderFinal:
		return db.StageRenderFinal
	case db.JobTypeRenderThumbnail:
		return db.StageRenderThumbnail
	case db.JobTypeProjectFinalize:
		return db.StageFinalize
	default:
		return db.StageCreated
	}
}
