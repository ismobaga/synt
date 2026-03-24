// Package moderation validates content for safety and policy compliance.
package moderation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Service validates scripts and media for safety.
type Service struct{}

// New creates a new moderation Service.
func New() *Service {
	return &Service{}
}

// ValidateScript checks a generated script for content policy violations.
func (s *Service) ValidateScript(ctx context.Context, scriptJSON []byte) error {
	var script struct {
		Title  string `json:"title"`
		Hook   string `json:"hook"`
		CTA    string `json:"cta"`
		Scenes []struct {
			Narration string `json:"narration"`
			Caption   string `json:"caption"`
		} `json:"scenes"`
	}
	if err := json.Unmarshal(scriptJSON, &script); err != nil {
		return fmt.Errorf("parse script for moderation: %w", err)
	}

	texts := []string{script.Title, script.Hook, script.CTA}
	for _, sc := range script.Scenes {
		texts = append(texts, sc.Narration, sc.Caption)
	}

	for _, text := range texts {
		if err := s.checkText(text); err != nil {
			return fmt.Errorf("content policy violation: %w", err)
		}
	}
	return nil
}

// checkText validates a single text string.
func (s *Service) checkText(text string) error {
	// Basic check: block obviously harmful patterns.
	// In production, integrate a real content moderation API.
	blocked := []string{
		"<script>", "javascript:", "data:text/html",
	}
	lower := strings.ToLower(text)
	for _, pattern := range blocked {
		if strings.Contains(lower, pattern) {
			return fmt.Errorf("blocked pattern detected: %q", pattern)
		}
	}
	return nil
}
