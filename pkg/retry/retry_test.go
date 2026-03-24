package retry_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ismobaga/synt/pkg/retry"
)

func TestDoSuccess(t *testing.T) {
	calls := 0
	err := retry.Do(context.Background(), retry.Config{
		MaxAttempts: 3,
		InitialWait: time.Millisecond,
		MaxWait:     time.Millisecond * 10,
		Multiplier:  2.0,
	}, func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestDoRetries(t *testing.T) {
	calls := 0
	target := errors.New("temporary error")
	err := retry.Do(context.Background(), retry.Config{
		MaxAttempts: 3,
		InitialWait: time.Millisecond,
		MaxWait:     time.Millisecond * 10,
		Multiplier:  2.0,
	}, func() error {
		calls++
		if calls < 3 {
			return target
		}
		return nil
	})
	if err != nil {
		t.Errorf("expected success on 3rd attempt, got %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestDoAllFail(t *testing.T) {
	target := errors.New("persistent error")
	err := retry.Do(context.Background(), retry.Config{
		MaxAttempts: 3,
		InitialWait: time.Millisecond,
		MaxWait:     time.Millisecond * 10,
		Multiplier:  2.0,
	}, func() error {
		return target
	})
	if err == nil {
		t.Error("expected error after all attempts")
	}
	if !errors.Is(err, target) {
		t.Errorf("expected wrapped target error, got %v", err)
	}
}

func TestDoContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := retry.Do(ctx, retry.Config{
		MaxAttempts: 3,
		InitialWait: time.Second, // long wait to trigger context check
		MaxWait:     time.Second,
		Multiplier:  2.0,
	}, func() error {
		return errors.New("fail")
	})
	if err == nil {
		t.Error("expected error when context cancelled")
	}
}
