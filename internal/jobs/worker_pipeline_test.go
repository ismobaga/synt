package jobs

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ismobaga/synt/internal/db"
)

func TestRetryDelayForAttemptUsesBackoff(t *testing.T) {
	if got := retryDelayForAttempt(1); got != 5*time.Second {
		t.Fatalf("attempt 1 delay = %v, want %v", got, 5*time.Second)
	}
	if got := retryDelayForAttempt(3); got != 20*time.Second {
		t.Fatalf("attempt 3 delay = %v, want %v", got, 20*time.Second)
	}
	if got := retryDelayForAttempt(8); got != 2*time.Minute {
		t.Fatalf("attempt 8 delay should be capped at 2m, got %v", got)
	}
}

func TestBuildPipelineStepStatusesUsesLatestJobAndTiming(t *testing.T) {
	projectID := uuid.New()
	started := time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC)
	finished := started.Add(1500 * time.Millisecond)

	steps := buildPipelineStepStatuses([]*db.Job{
		{
			ID:          uuid.New(),
			ProjectID:   projectID,
			JobType:     db.JobTypeSourceFetch,
			Status:      db.JobStatusDone,
			Attempts:    1,
			MaxAttempts: 5,
			StartedAt:   &started,
			FinishedAt:  &finished,
			CreatedAt:   started,
		},
		{
			ID:          uuid.New(),
			ProjectID:   projectID,
			JobType:     db.JobTypeRenderPreview,
			Status:      db.JobStatusRetrying,
			Attempts:    2,
			MaxAttempts: 5,
			LastError:   "temporary timeout",
			StartedAt:   &started,
			FinishedAt:  &finished,
			CreatedAt:   finished,
		},
	})

	if len(steps) == 0 {
		t.Fatal("expected pipeline steps to be returned")
	}

	var sourceStep, previewStep *PipelineStepStatus
	for i := range steps {
		switch steps[i].Stage {
		case db.StageSourceFetch:
			sourceStep = &steps[i]
		case db.StageRenderPreview:
			previewStep = &steps[i]
		}
	}

	if sourceStep == nil {
		t.Fatal("expected source_fetch step in summary")
	}
	if sourceStep.Status != db.JobStatusDone {
		t.Fatalf("source status = %q, want %q", sourceStep.Status, db.JobStatusDone)
	}
	if sourceStep.DurationMs != 1500 {
		t.Fatalf("source duration = %dms, want 1500ms", sourceStep.DurationMs)
	}

	if previewStep == nil {
		t.Fatal("expected render_preview step in summary")
	}
	if previewStep.Attempts != 2 || previewStep.MaxAttempts != 5 {
		t.Fatalf("unexpected preview attempts: %+v", previewStep)
	}
	if previewStep.LastError != "temporary timeout" {
		t.Fatalf("unexpected last error: %q", previewStep.LastError)
	}
}
