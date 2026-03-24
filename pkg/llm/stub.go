// Package llm provides a stub LLM client for development and testing.
package llm

import (
	"context"
	"fmt"
)

// StubClient returns pre-built script JSON for testing.
type StubClient struct{}

// NewStubClient creates a new StubClient.
func NewStubClient() *StubClient {
	return &StubClient{}
}

// Complete returns a hardcoded script JSON for testing.
func (c *StubClient) Complete(_ context.Context, prompt string) (string, error) {
	_ = prompt
	return fmt.Sprintf(`{
  "title": "AI Tools for Small Business",
  "hook": "Most small businesses waste time on tasks AI can do instantly.",
  "duration_sec": 30,
  "language": "en",
  "cta": "Follow for more AI business tips.",
  "music_mood": "upbeat corporate",
  "scenes": [
    {
      "index": 1,
      "duration_sec": 5,
      "narration": "Most small businesses waste time on tasks AI can do instantly.",
      "caption": "Stop wasting time",
      "visual_query": "small business owner working late laptop office",
      "overlay_style": "hook"
    },
    {
      "index": 2,
      "duration_sec": 8,
      "narration": "AI can write emails, content, and product descriptions in seconds.",
      "caption": "Write faster with AI",
      "visual_query": "person using AI assistant on laptop",
      "overlay_style": "main"
    },
    {
      "index": 3,
      "duration_sec": 7,
      "narration": "Automate your social media posting and save hours every week.",
      "caption": "Automate social media",
      "visual_query": "social media dashboard analytics screen",
      "overlay_style": "main"
    },
    {
      "index": 4,
      "duration_sec": 5,
      "narration": "Use AI chatbots to handle customer support 24/7.",
      "caption": "24/7 customer support",
      "visual_query": "customer service chatbot interface",
      "overlay_style": "main"
    },
    {
      "index": 5,
      "duration_sec": 5,
      "narration": "Follow for more AI business tips.",
      "caption": "Follow for more tips",
      "visual_query": "subscribe button social media notification",
      "overlay_style": "cta"
    }
  ]
}
`), nil
}
