// Package llm provides a client interface for LLM completions.
package llm

import "context"

// Client is the interface for LLM text completion.
type Client interface {
	// Complete sends a prompt and returns the completion.
	Complete(ctx context.Context, prompt string) (string, error)
}
