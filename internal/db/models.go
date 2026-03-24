package db

import (
	"time"

	"github.com/google/uuid"
)

// User represents a platform user.
type User struct {
	ID           uuid.UUID `db:"id" json:"id"`
	Email        string    `db:"email" json:"email"`
	PasswordHash string    `db:"password_hash" json:"-"`
	Plan         string    `db:"plan" json:"plan"`
	Credits      int       `db:"credits" json:"credits"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
}

// Project represents a video generation project.
type Project struct {
	ID           uuid.UUID  `db:"id" json:"id"`
	UserID       uuid.UUID  `db:"user_id" json:"user_id"`
	Topic        string     `db:"topic" json:"topic"`
	Language     string     `db:"language" json:"language"`
	Platform     string     `db:"platform" json:"platform"`
	DurationSec  int        `db:"duration_sec" json:"duration_sec"`
	Tone         string     `db:"tone" json:"tone"`
	TemplateID   string     `db:"template_id" json:"template_id"`
	Status       string     `db:"status" json:"status"`
	CurrentStage string     `db:"current_stage" json:"current_stage"`
	ErrorMessage string     `db:"error_message" json:"error_message,omitempty"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at" json:"updated_at"`
}

// Script holds the generated script for a project.
type Script struct {
	ID          uuid.UUID `db:"id" json:"id"`
	ProjectID   uuid.UUID `db:"project_id" json:"project_id"`
	Title       string    `db:"title" json:"title"`
	Hook        string    `db:"hook" json:"hook"`
	CTA         string    `db:"cta" json:"cta"`
	Language    string    `db:"language" json:"language"`
	ContentJSON []byte    `db:"content_json" json:"content_json"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
}

// Asset represents a media asset (video, image, audio).
type Asset struct {
	ID          uuid.UUID  `db:"id" json:"id"`
	ProjectID   *uuid.UUID `db:"project_id" json:"project_id,omitempty"`
	Type        string     `db:"type" json:"type"`
	Source      string     `db:"source" json:"source"`
	Provider    string     `db:"provider" json:"provider"`
	URL         string     `db:"url" json:"url"`
	StoragePath string     `db:"storage_path" json:"storage_path"`
	MimeType    string     `db:"mime_type" json:"mime_type"`
	Width       int        `db:"width" json:"width"`
	Height      int        `db:"height" json:"height"`
	DurationSec float64    `db:"duration_sec" json:"duration_sec"`
	LicenseInfo []byte     `db:"license_info" json:"license_info,omitempty"`
	Metadata    []byte     `db:"metadata" json:"metadata,omitempty"`
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
}

// AudioTrack represents a voiceover or music track.
type AudioTrack struct {
	ID          uuid.UUID `db:"id" json:"id"`
	ProjectID   uuid.UUID `db:"project_id" json:"project_id"`
	Kind        string    `db:"kind" json:"kind"`
	VoiceName   string    `db:"voice_name" json:"voice_name"`
	Language    string    `db:"language" json:"language"`
	StoragePath string    `db:"storage_path" json:"storage_path"`
	DurationSec float64   `db:"duration_sec" json:"duration_sec"`
	Metadata    []byte    `db:"metadata" json:"metadata,omitempty"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
}

// Subtitle represents subtitle/caption data for a project.
type Subtitle struct {
	ID          uuid.UUID `db:"id" json:"id"`
	ProjectID   uuid.UUID `db:"project_id" json:"project_id"`
	Format      string    `db:"format" json:"format"`
	StoragePath string    `db:"storage_path" json:"storage_path"`
	Content     []byte    `db:"content" json:"content,omitempty"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
}

// Render represents a rendered video output.
type Render struct {
	ID            uuid.UUID `db:"id" json:"id"`
	ProjectID     uuid.UUID `db:"project_id" json:"project_id"`
	Kind          string    `db:"kind" json:"kind"`
	Resolution    string    `db:"resolution" json:"resolution"`
	FPS           int       `db:"fps" json:"fps"`
	StoragePath   string    `db:"storage_path" json:"storage_path"`
	ThumbnailPath string    `db:"thumbnail_path" json:"thumbnail_path"`
	Status        string    `db:"status" json:"status"`
	Metadata      []byte    `db:"metadata" json:"metadata,omitempty"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
}

// Job represents a background processing job.
type Job struct {
	ID          uuid.UUID  `db:"id" json:"id"`
	ProjectID   uuid.UUID  `db:"project_id" json:"project_id"`
	JobType     string     `db:"job_type" json:"job_type"`
	Status      string     `db:"status" json:"status"`
	Payload     []byte     `db:"payload" json:"payload,omitempty"`
	Attempts    int        `db:"attempts" json:"attempts"`
	MaxAttempts int        `db:"max_attempts" json:"max_attempts"`
	LastError   string     `db:"last_error" json:"last_error,omitempty"`
	ScheduledAt *time.Time `db:"scheduled_at" json:"scheduled_at,omitempty"`
	StartedAt   *time.Time `db:"started_at" json:"started_at,omitempty"`
	FinishedAt  *time.Time `db:"finished_at" json:"finished_at,omitempty"`
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
}

// Template represents a video rendering template.
type Template struct {
	ID        string    `db:"id" json:"id"`
	Name      string    `db:"name" json:"name"`
	Category  string    `db:"category" json:"category"`
	Config    []byte    `db:"config" json:"config"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// BrandKit holds branding configuration for a user.
type BrandKit struct {
	ID             uuid.UUID `db:"id" json:"id"`
	UserID         uuid.UUID `db:"user_id" json:"user_id"`
	Name           string    `db:"name" json:"name"`
	LogoPath       string    `db:"logo_path" json:"logo_path"`
	PrimaryColor   string    `db:"primary_color" json:"primary_color"`
	SecondaryColor string    `db:"secondary_color" json:"secondary_color"`
	FontFamily     string    `db:"font_family" json:"font_family"`
	OutroText      string    `db:"outro_text" json:"outro_text"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
}

// Project statuses.
const (
	ProjectStatusDraft      = "draft"
	ProjectStatusQueued     = "queued"
	ProjectStatusProcessing = "processing"
	ProjectStatusDone       = "done"
	ProjectStatusFailed     = "failed"
)

// Project stages.
const (
	StageCreated           = "created"
	StageScriptGeneration  = "script_generation"
	StageScriptValidation  = "script_validation"
	StageMediaSearch       = "media_search"
	StageMediaPrepare      = "media_prepare"
	StageVoiceGeneration   = "voice_generation"
	StageSubtitleGeneration = "subtitle_generation"
	StageMusicSelection    = "music_selection"
	StageTimelineBuild     = "timeline_build"
	StageRenderPreview     = "render_preview"
	StageRenderFinal       = "render_final"
	StageRenderThumbnail   = "render_thumbnail"
	StageFinalize          = "finalize"
)

// Job types.
const (
	JobTypeProjectGenerate   = "project:generate"
	JobTypeScriptGenerate    = "script:generate"
	JobTypeScriptValidate    = "script:validate"
	JobTypeMediaSearch       = "media:search"
	JobTypeMediaPrepare      = "media:prepare"
	JobTypeVoiceGenerate     = "voice:generate"
	JobTypeSubtitleGenerate  = "subtitle:generate"
	JobTypeMusicSelect       = "music:select"
	JobTypeTimelineBuild     = "timeline:build"
	JobTypeRenderPreview     = "render:preview"
	JobTypeRenderFinal       = "render:final"
	JobTypeRenderThumbnail   = "render:thumbnail"
	JobTypeProjectFinalize   = "project:finalize"
)

// Job statuses.
const (
	JobStatusPending   = "pending"
	JobStatusRunning   = "running"
	JobStatusDone      = "done"
	JobStatusFailed    = "failed"
	JobStatusRetrying  = "retrying"
)
