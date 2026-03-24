// Package ffmpeg provides an interface for running FFmpeg commands.
package ffmpeg

import (
	"context"
	"fmt"
	"os/exec"
)

// Command represents an FFmpeg invocation.
type Command struct {
	Args []string
}

// Runner executes FFmpeg commands.
type Runner interface {
	Run(ctx context.Context, cmd Command) error
}

// LocalRunner runs FFmpeg from the system PATH.
type LocalRunner struct{}

// NewLocalRunner creates a new LocalRunner.
func NewLocalRunner() *LocalRunner {
	return &LocalRunner{}
}

// Run executes an FFmpeg command.
func (r *LocalRunner) Run(ctx context.Context, cmd Command) error {
	args := append([]string{"-hide_banner", "-loglevel", "error"}, cmd.Args...)
	c := exec.CommandContext(ctx, "ffmpeg", args...)
	if out, err := c.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg: %w\noutput: %s", err, out)
	}
	return nil
}

// StubRunner is a no-op runner for testing.
type StubRunner struct{}

// NewStubRunner creates a no-op runner.
func NewStubRunner() *StubRunner {
	return &StubRunner{}
}

// Run is a no-op.
func (r *StubRunner) Run(_ context.Context, cmd Command) error {
	return nil
}
