package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// CreateProject inserts a new project.
func (db *DB) CreateProject(ctx context.Context, p *Project) error {
	q := `INSERT INTO projects
		(id, user_id, topic, language, platform, duration_sec, tone, template_id, status, current_stage, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`
	_, err := db.ExecContext(ctx, q,
		p.ID, p.UserID, p.Topic, p.Language, p.Platform,
		p.DurationSec, p.Tone, p.TemplateID, p.Status, p.CurrentStage,
		p.CreatedAt, p.UpdatedAt,
	)
	return err
}

// GetProject retrieves a project by ID.
func (db *DB) GetProject(ctx context.Context, id uuid.UUID) (*Project, error) {
	q := `SELECT id, user_id, topic, language, platform, duration_sec, tone, template_id,
		status, current_stage, COALESCE(error_message,''), created_at, updated_at
		FROM projects WHERE id = $1`
	p := &Project{}
	err := db.QueryRowContext(ctx, q, id).Scan(
		&p.ID, &p.UserID, &p.Topic, &p.Language, &p.Platform,
		&p.DurationSec, &p.Tone, &p.TemplateID,
		&p.Status, &p.CurrentStage, &p.ErrorMessage, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get project %s: %w", id, err)
	}
	return p, nil
}

// ListProjects returns all projects.
func (db *DB) ListProjects(ctx context.Context) ([]*Project, error) {
	q := `SELECT id, user_id, topic, language, platform, duration_sec, tone, template_id,
		status, current_stage, COALESCE(error_message,''), created_at, updated_at
		FROM projects ORDER BY created_at DESC`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var projects []*Project
	for rows.Next() {
		p := &Project{}
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.Topic, &p.Language, &p.Platform,
			&p.DurationSec, &p.Tone, &p.TemplateID,
			&p.Status, &p.CurrentStage, &p.ErrorMessage, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// UpdateProjectStatus updates project status and stage.
func (db *DB) UpdateProjectStatus(ctx context.Context, id uuid.UUID, status, stage, errMsg string) error {
	q := `UPDATE projects SET status=$2, current_stage=$3, error_message=$4, updated_at=NOW() WHERE id=$1`
	_, err := db.ExecContext(ctx, q, id, status, stage, errMsg)
	return err
}

// DeleteProject removes a project by ID.
func (db *DB) DeleteProject(ctx context.Context, id uuid.UUID) error {
	_, err := db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, id)
	return err
}

// CreateScript inserts a script record.
func (db *DB) CreateScript(ctx context.Context, s *Script) error {
	q := `INSERT INTO scripts (id, project_id, title, hook, cta, language, content_json, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`
	_, err := db.ExecContext(ctx, q,
		s.ID, s.ProjectID, s.Title, s.Hook, s.CTA, s.Language, s.ContentJSON, s.CreatedAt,
	)
	return err
}

// GetScript retrieves the script for a project.
func (db *DB) GetScript(ctx context.Context, projectID uuid.UUID) (*Script, error) {
	q := `SELECT id, project_id, COALESCE(title,''), COALESCE(hook,''), COALESCE(cta,''), language, content_json, created_at
		FROM scripts WHERE project_id=$1 ORDER BY created_at DESC LIMIT 1`
	s := &Script{}
	err := db.QueryRowContext(ctx, q, projectID).Scan(
		&s.ID, &s.ProjectID, &s.Title, &s.Hook, &s.CTA, &s.Language, &s.ContentJSON, &s.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get script for project %s: %w", projectID, err)
	}
	return s, nil
}

// UpdateScript updates the content JSON of the latest script for a project.
func (db *DB) UpdateScript(ctx context.Context, projectID uuid.UUID, contentJSON []byte) error {
	q := `UPDATE scripts SET content_json=$2
		WHERE id=(SELECT id FROM scripts WHERE project_id=$1 ORDER BY created_at DESC LIMIT 1)`
	_, err := db.ExecContext(ctx, q, projectID, contentJSON)
	return err
}

// GetAssets retrieves all assets for a project.
func (db *DB) GetAssets(ctx context.Context, projectID uuid.UUID) ([]*Asset, error) {
	q := `SELECT id, project_id, type, source, COALESCE(provider,''), COALESCE(url,''),
		COALESCE(storage_path,''), COALESCE(mime_type,''), COALESCE(width,0), COALESCE(height,0),
		COALESCE(duration_sec,0), license_info, metadata, created_at
		FROM assets WHERE project_id=$1 ORDER BY created_at`
	rows, err := db.QueryContext(ctx, q, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var assets []*Asset
	for rows.Next() {
		a := &Asset{}
		if err := rows.Scan(
			&a.ID, &a.ProjectID, &a.Type, &a.Source, &a.Provider, &a.URL,
			&a.StoragePath, &a.MimeType, &a.Width, &a.Height,
			&a.DurationSec, &a.LicenseInfo, &a.Metadata, &a.CreatedAt,
		); err != nil {
			return nil, err
		}
		assets = append(assets, a)
	}
	return assets, rows.Err()
}

// CreateAsset inserts a new asset record.
func (db *DB) CreateAsset(ctx context.Context, a *Asset) error {
	q := `INSERT INTO assets (id, project_id, type, source, provider, url, storage_path,
		mime_type, width, height, duration_sec, license_info, metadata, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`
	_, err := db.ExecContext(ctx, q,
		a.ID, a.ProjectID, a.Type, a.Source, a.Provider, a.URL, a.StoragePath,
		a.MimeType, a.Width, a.Height, a.DurationSec, a.LicenseInfo, a.Metadata, a.CreatedAt,
	)
	return err
}

// GetAudioTracks retrieves audio tracks for a project.
func (db *DB) GetAudioTracks(ctx context.Context, projectID uuid.UUID) ([]*AudioTrack, error) {
	q := `SELECT id, project_id, kind, COALESCE(voice_name,''), COALESCE(language,''),
		storage_path, COALESCE(duration_sec,0), metadata, created_at
		FROM audio_tracks WHERE project_id=$1 ORDER BY created_at`
	rows, err := db.QueryContext(ctx, q, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tracks []*AudioTrack
	for rows.Next() {
		t := &AudioTrack{}
		if err := rows.Scan(
			&t.ID, &t.ProjectID, &t.Kind, &t.VoiceName, &t.Language,
			&t.StoragePath, &t.DurationSec, &t.Metadata, &t.CreatedAt,
		); err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}

// CreateAudioTrack inserts an audio track record.
func (db *DB) CreateAudioTrack(ctx context.Context, t *AudioTrack) error {
	q := `INSERT INTO audio_tracks (id, project_id, kind, voice_name, language, storage_path, duration_sec, metadata, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`
	_, err := db.ExecContext(ctx, q,
		t.ID, t.ProjectID, t.Kind, t.VoiceName, t.Language,
		t.StoragePath, t.DurationSec, t.Metadata, t.CreatedAt,
	)
	return err
}

// GetSubtitles retrieves subtitles for a project.
func (db *DB) GetSubtitles(ctx context.Context, projectID uuid.UUID) ([]*Subtitle, error) {
	q := `SELECT id, project_id, format, storage_path, content, created_at
		FROM subtitles WHERE project_id=$1 ORDER BY created_at`
	rows, err := db.QueryContext(ctx, q, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var subs []*Subtitle
	for rows.Next() {
		s := &Subtitle{}
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.Format, &s.StoragePath, &s.Content, &s.CreatedAt); err != nil {
			return nil, err
		}
		subs = append(subs, s)
	}
	return subs, rows.Err()
}

// CreateSubtitle inserts a subtitle record.
func (db *DB) CreateSubtitle(ctx context.Context, s *Subtitle) error {
	q := `INSERT INTO subtitles (id, project_id, format, storage_path, content, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)`
	_, err := db.ExecContext(ctx, q, s.ID, s.ProjectID, s.Format, s.StoragePath, s.Content, s.CreatedAt)
	return err
}

// GetRenders retrieves render records for a project.
func (db *DB) GetRenders(ctx context.Context, projectID uuid.UUID) ([]*Render, error) {
	q := `SELECT id, project_id, kind, resolution, fps,
		COALESCE(storage_path,''), COALESCE(thumbnail_path,''), status, metadata, created_at
		FROM renders WHERE project_id=$1 ORDER BY created_at DESC`
	rows, err := db.QueryContext(ctx, q, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var renders []*Render
	for rows.Next() {
		rv := &Render{}
		if err := rows.Scan(
			&rv.ID, &rv.ProjectID, &rv.Kind, &rv.Resolution, &rv.FPS,
			&rv.StoragePath, &rv.ThumbnailPath, &rv.Status, &rv.Metadata, &rv.CreatedAt,
		); err != nil {
			return nil, err
		}
		renders = append(renders, rv)
	}
	return renders, rows.Err()
}

// CreateRender inserts a render record.
func (db *DB) CreateRender(ctx context.Context, rv *Render) error {
	q := `INSERT INTO renders (id, project_id, kind, resolution, fps, storage_path, thumbnail_path, status, metadata, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`
	_, err := db.ExecContext(ctx, q,
		rv.ID, rv.ProjectID, rv.Kind, rv.Resolution, rv.FPS,
		rv.StoragePath, rv.ThumbnailPath, rv.Status, rv.Metadata, rv.CreatedAt,
	)
	return err
}

// UpdateRenderStatus updates the status of a render.
func (db *DB) UpdateRenderStatus(ctx context.Context, id uuid.UUID, status, storagePath, thumbPath string) error {
	q := `UPDATE renders SET status=$2, storage_path=$3, thumbnail_path=$4 WHERE id=$1`
	_, err := db.ExecContext(ctx, q, id, status, storagePath, thumbPath)
	return err
}

// CreateJob inserts a job record.
func (db *DB) CreateJob(ctx context.Context, j *Job) error {
	payload, err := json.Marshal(j.Payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	q := `INSERT INTO jobs (id, project_id, job_type, status, payload, attempts, max_attempts, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`
	_, err = db.ExecContext(ctx, q,
		j.ID, j.ProjectID, j.JobType, j.Status, payload,
		j.Attempts, j.MaxAttempts, j.CreatedAt,
	)
	return err
}

// UpdateJobStatus updates the status of a job.
func (db *DB) UpdateJobStatus(ctx context.Context, id uuid.UUID, status, lastErr string) error {
	q := `UPDATE jobs SET status=$2, last_error=$3 WHERE id=$1`
	_, err := db.ExecContext(ctx, q, id, status, lastErr)
	return err
}

// ListPendingJobs returns jobs eligible for processing.
func (db *DB) ListPendingJobs(ctx context.Context, limit int) ([]*Job, error) {
	q := `SELECT id, project_id, job_type, status, COALESCE(payload,'{}'), attempts, max_attempts,
		COALESCE(last_error,''), scheduled_at, started_at, finished_at, created_at
		FROM jobs
		WHERE status IN ($1,$2) AND attempts < max_attempts
		AND (scheduled_at IS NULL OR scheduled_at <= NOW())
		ORDER BY created_at ASC LIMIT $3`
	rows, err := db.QueryContext(ctx, q, JobStatusPending, JobStatusRetrying, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var jobs []*Job
	for rows.Next() {
		j := &Job{}
		if err := rows.Scan(
			&j.ID, &j.ProjectID, &j.JobType, &j.Status, &j.Payload,
			&j.Attempts, &j.MaxAttempts, &j.LastError,
			&j.ScheduledAt, &j.StartedAt, &j.FinishedAt, &j.CreatedAt,
		); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// ListTemplates returns all templates.
func (db *DB) ListTemplates(ctx context.Context) ([]*Template, error) {
	q := `SELECT id, name, COALESCE(category,''), config, created_at FROM templates ORDER BY name`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var templates []*Template
	for rows.Next() {
		t := &Template{}
		if err := rows.Scan(&t.ID, &t.Name, &t.Category, &t.Config, &t.CreatedAt); err != nil {
			return nil, err
		}
		templates = append(templates, t)
	}
	return templates, rows.Err()
}

// ListBrandKits returns all brand kits.
func (db *DB) ListBrandKits(ctx context.Context) ([]*BrandKit, error) {
	q := `SELECT id, user_id, name, COALESCE(logo_path,''), COALESCE(primary_color,''),
		COALESCE(secondary_color,''), COALESCE(font_family,''), COALESCE(outro_text,''), created_at
		FROM brand_kits ORDER BY created_at DESC`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var kits []*BrandKit
	for rows.Next() {
		k := &BrandKit{}
		if err := rows.Scan(
			&k.ID, &k.UserID, &k.Name, &k.LogoPath,
			&k.PrimaryColor, &k.SecondaryColor, &k.FontFamily, &k.OutroText, &k.CreatedAt,
		); err != nil {
			return nil, err
		}
		kits = append(kits, k)
	}
	return kits, rows.Err()
}

// CreateBrandKit inserts a brand kit.
func (db *DB) CreateBrandKit(ctx context.Context, k *BrandKit) error {
	q := `INSERT INTO brand_kits (id, user_id, name, logo_path, primary_color, secondary_color, font_family, outro_text, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`
	_, err := db.ExecContext(ctx, q,
		k.ID, k.UserID, k.Name, k.LogoPath,
		k.PrimaryColor, k.SecondaryColor, k.FontFamily, k.OutroText, k.CreatedAt,
	)
	return err
}
