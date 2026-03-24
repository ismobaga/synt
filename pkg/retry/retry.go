// Package retry provides retry utilities with exponential backoff.
package retry

import (
	"context"
	"fmt"
	"time"
)

// Config controls retry behaviour.
type Config struct {
	MaxAttempts int
	InitialWait time.Duration
	MaxWait     time.Duration
	Multiplier  float64
}

// DefaultConfig returns sensible retry defaults.
func DefaultConfig() Config {
	return Config{
		MaxAttempts: 5,
		InitialWait: time.Second,
		MaxWait:     30 * time.Second,
		Multiplier:  2.0,
	}
}

// Do runs fn with retries according to cfg.
func Do(ctx context.Context, cfg Config, fn func() error) error {
	wait := cfg.InitialWait
	var lastErr error
	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if attempt == cfg.MaxAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
		wait = time.Duration(float64(wait) * cfg.Multiplier)
		if wait > cfg.MaxWait {
			wait = cfg.MaxWait
		}
	}
	return fmt.Errorf("all %d attempts failed: %w", cfg.MaxAttempts, lastErr)
}
