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
	return &SynthesizeResult{
		StoragePath: fmt.Sprintf("audio/%s_voiceover.wav", req.Language),
		DurationSec: 28.5,
		VoiceName:   req.Voice,
	}, nil
}
