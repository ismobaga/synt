// Package orchestrator coordinates the generation pipeline.
package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ismobaga/synt/internal/db"
)

// Orchestrator drives the project generation pipeline.
type Orchestrator struct {
	db    *db.DB
	queue Queue
}

// Queue is the interface for the job queue backend.
type Queue interface {
	Enqueue(ctx context.Context, jobType string, payload []byte) error
}

// New creates a new Orchestrator.
func New(database *db.DB, q Queue) *Orchestrator {
	return &Orchestrator{db: database, queue: q}
}

// TriggerGeneration starts the full generation pipeline for a project.
func (o *Orchestrator) TriggerGeneration(ctx context.Context, projectID uuid.UUID, autoRender bool) error {
	if err := o.db.UpdateProjectStatus(ctx, projectID, db.ProjectStatusQueued, db.StageScriptGeneration, ""); err != nil {
		return fmt.Errorf("update project status: %w", err)
	}

	payload := map[string]any{
		"project_id":  projectID.String(),
		"auto_render": autoRender,
	}
	data, _ := json.Marshal(payload)

	return o.EnqueueJob(ctx, projectID, db.JobTypeProjectGenerate, data)
}

// RetryProject re-enqueues a failed project.
func (o *Orchestrator) RetryProject(ctx context.Context, projectID uuid.UUID) error {
	project, err := o.db.GetProject(ctx, projectID)
	if err != nil {
		return fmt.Errorf("get project: %w", err)
	}
	if project.Status != db.ProjectStatusFailed {
		return fmt.Errorf("project is not in failed state")
	}
	return o.TriggerGeneration(ctx, projectID, true)
}

// EnqueueJob creates a job record and pushes it to the queue.
func (o *Orchestrator) EnqueueJob(ctx context.Context, projectID uuid.UUID, jobType string, payload []byte) error {
	if payload == nil {
		payload = []byte("{}")
	}
	now := time.Now().UTC()
	job := &db.Job{
		ID:          uuid.New(),
		ProjectID:   projectID,
		JobType:     jobType,
		Status:      db.JobStatusPending,
		Payload:     payload,
		Attempts:    0,
		MaxAttempts: 5,
		CreatedAt:   now,
	}
	if err := o.db.CreateJob(ctx, job); err != nil {
		return fmt.Errorf("create job record: %w", err)
	}
	return o.queue.Enqueue(ctx, jobType, payload)
}
