// Package jobs provides the job queue worker.
package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/ismobaga/synt/internal/content"
	"github.com/ismobaga/synt/internal/db"
	"github.com/ismobaga/synt/internal/media"
	"github.com/ismobaga/synt/internal/moderation"
	"github.com/ismobaga/synt/internal/music"
	"github.com/ismobaga/synt/internal/render"
	"github.com/ismobaga/synt/internal/subtitle"
	"github.com/ismobaga/synt/internal/voice"
)

// Worker processes jobs from the queue.
type Worker struct {
	db          *db.DB
	content     *content.Service
	media       *media.Service
	voice       *voice.Service
	subtitle    *subtitle.Service
	music       *music.Service
	render      *render.Service
	moderation  *moderation.Service
	pollInterval time.Duration
}

// Config holds worker configuration.
type Config struct {
	PollInterval time.Duration
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
			log.Printf("[worker] job %s (%s) failed: %v", j.ID, j.JobType, err)
			_ = w.db.UpdateJobStatus(ctx, j.ID, db.JobStatusFailed, err.Error())
		}
	}
}

func (w *Worker) process(ctx context.Context, j *db.Job) error {
	log.Printf("[worker] processing job %s type=%s project=%s", j.ID, j.JobType, j.ProjectID)
	_ = w.db.UpdateJobStatus(ctx, j.ID, db.JobStatusRunning, "")

	var err error
	switch j.JobType {
	case db.JobTypeProjectGenerate:
		err = w.handleProjectGenerate(ctx, j)
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
		return err
	}
	return w.db.UpdateJobStatus(ctx, j.ID, db.JobStatusDone, "")
}

func (w *Worker) handleProjectGenerate(ctx context.Context, j *db.Job) error {
	// Full pipeline: script -> media -> voice -> subtitle -> music -> timeline -> render
	steps := []string{
		db.JobTypeScriptGenerate,
		db.JobTypeScriptValidate,
		db.JobTypeMediaSearch,
		db.JobTypeMediaPrepare,
		db.JobTypeVoiceGenerate,
		db.JobTypeSubtitleGenerate,
		db.JobTypeMusicSelect,
		db.JobTypeTimelineBuild,
		db.JobTypeRenderPreview,
		db.JobTypeRenderFinal,
		db.JobTypeRenderThumbnail,
		db.JobTypeProjectFinalize,
	}
	for _, step := range steps {
		stepJob := &db.Job{
			ID:          uuid.New(),
			ProjectID:   j.ProjectID,
			JobType:     step,
			Status:      db.JobStatusPending,
			Payload:     j.Payload,
			Attempts:    0,
			MaxAttempts: 5,
			CreatedAt:   time.Now().UTC(),
		}
		if err := w.db.CreateJob(ctx, stepJob); err != nil {
			return fmt.Errorf("create step job %s: %w", step, err)
		}
	}
	return nil
}

func (w *Worker) handleScriptGenerate(ctx context.Context, j *db.Job) error {
	project, err := w.db.GetProject(ctx, j.ProjectID)
	if err != nil {
		return err
	}
	_ = w.db.UpdateProjectStatus(ctx, j.ProjectID, db.ProjectStatusProcessing, db.StageScriptGeneration, "")

	req := content.GenerateRequest{
		Topic:       project.Topic,
		Platform:    project.Platform,
		DurationSec: project.DurationSec,
		Tone:        project.Tone,
		Language:    project.Language,
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
	return w.db.CreateScript(ctx, s)
}

func (w *Worker) handleScriptValidate(ctx context.Context, j *db.Job) error {
	_ = w.db.UpdateProjectStatus(ctx, j.ProjectID, db.ProjectStatusProcessing, db.StageScriptValidation, "")
	script, err := w.db.GetScript(ctx, j.ProjectID)
	if err != nil {
		return err
	}
	return w.moderation.ValidateScript(ctx, script.ContentJSON)
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
	return nil
}

func (w *Worker) handleMediaPrepare(ctx context.Context, j *db.Job) error {
	_ = w.db.UpdateProjectStatus(ctx, j.ProjectID, db.ProjectStatusProcessing, db.StageMediaPrepare, "")
	assets, err := w.db.GetAssets(ctx, j.ProjectID)
	if err != nil {
		return err
	}
	return w.media.PrepareAssets(ctx, assets)
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
	return w.db.CreateAudioTrack(ctx, track)
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
	return w.db.CreateSubtitle(ctx, sub)
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
	return w.db.CreateAudioTrack(ctx, track)
}

func (w *Worker) handleTimelineBuild(ctx context.Context, j *db.Job) error {
	_ = w.db.UpdateProjectStatus(ctx, j.ProjectID, db.ProjectStatusProcessing, db.StageTimelineBuild, "")
	return w.render.BuildTimeline(ctx, j.ProjectID)
}

func (w *Worker) handleRenderPreview(ctx context.Context, j *db.Job) error {
	_ = w.db.UpdateProjectStatus(ctx, j.ProjectID, db.ProjectStatusProcessing, db.StageRenderPreview, "")
	return w.render.RenderPreview(ctx, j.ProjectID)
}

func (w *Worker) handleRenderFinal(ctx context.Context, j *db.Job) error {
	_ = w.db.UpdateProjectStatus(ctx, j.ProjectID, db.ProjectStatusProcessing, db.StageRenderFinal, "")
	return w.render.RenderFinal(ctx, j.ProjectID)
}

func (w *Worker) handleRenderThumbnail(ctx context.Context, j *db.Job) error {
	_ = w.db.UpdateProjectStatus(ctx, j.ProjectID, db.ProjectStatusProcessing, db.StageRenderThumbnail, "")
	return w.render.ExtractThumbnail(ctx, j.ProjectID)
}

func (w *Worker) handleProjectFinalize(ctx context.Context, j *db.Job) error {
	return w.db.UpdateProjectStatus(ctx, j.ProjectID, db.ProjectStatusDone, db.StageFinalize, "")
}
