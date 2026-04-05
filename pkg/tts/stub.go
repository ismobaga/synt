// Package tts provides a stub TTS client for development and testing.
package tts

import (
	"context"
	"fmt"
)

// StubClient returns stub TTS results.
type StubClient struct{}

// NewStubClient creates a new StubClient.
func NewStubClient() *StubClient {
	return &StubClient{}
}

// Synthesize returns a stub audio result.
func (c *StubClient) Synthesize(_ context.Context, req SynthesizeRequest) (*SynthesizeResult, error) {
	meta, duration := buildMetadataForProvider("stub", req.Text, req.SpeedX)
	if duration == 0 {
		duration = 28.5
	}
	return &SynthesizeResult{
		StoragePath: fmt.Sprintf("audio/%s_voiceover.wav", req.Language),
		DurationSec: duration,
		VoiceName:   req.Voice,
		Metadata:    meta,
	}, nil
}
