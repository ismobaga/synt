// Package tts provides a client interface for text-to-speech synthesis.
package tts

import "context"

// SynthesizeRequest holds TTS parameters.
type SynthesizeRequest struct {
	Text     string
	Language string
	Voice    string
	SpeedX   float64 // 1.0 = normal
}

// SynthesizeResult holds the output of a TTS synthesis.
type SynthesizeResult struct {
	StoragePath string
	DurationSec float64
	VoiceName   string
}

// Client is the interface for TTS providers.
type Client interface {
	Synthesize(ctx context.Context, req SynthesizeRequest) (*SynthesizeResult, error)
}
