// Package media searches and prepares visual assets for scenes.
package media

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ismobaga/synt/internal/content"
	"github.com/ismobaga/synt/internal/db"
)

// Provider is a source of stock media.
type Provider interface {
	// Search returns asset candidates matching the query.
	Search(ctx context.Context, query string, assetType string) ([]*Candidate, error)
}

// Candidate is a media search result.
type Candidate struct {
	URL         string
	Provider    string
	Type        string // "video" or "image"
	Width       int
	Height      int
	DurationSec float64
	LicenseInfo map[string]any
}

// Service manages media asset search and preparation.
type Service struct {
	providers []Provider
}

// New creates a new media Service.
func New(providers ...Provider) *Service {
	return &Service{providers: providers}
}

// SearchAssets finds media for each scene in the script.
func (s *Service) SearchAssets(ctx context.Context, projectID uuid.UUID, scriptJSON []byte) ([]*db.Asset, error) {
	var script content.ScriptContent
	if err := json.Unmarshal(scriptJSON, &script); err != nil {
		return nil, fmt.Errorf("parse script: %w", err)
	}

	var assets []*db.Asset
	for _, scene := range script.Scenes {
		asset, err := s.searchForScene(ctx, projectID, scene)
		if err != nil {
			// fallback: use a placeholder
			asset = s.placeholderAsset(projectID, scene.VisualQuery)
		}
		assets = append(assets, asset)
	}
	return assets, nil
}

func (s *Service) searchForScene(ctx context.Context, projectID uuid.UUID, scene content.SceneContent) (*db.Asset, error) {
	for _, p := range s.providers {
		candidates, err := p.Search(ctx, scene.VisualQuery, "video")
		if err != nil {
			continue
		}
		if len(candidates) == 0 {
			// try images
			candidates, err = p.Search(ctx, scene.VisualQuery, "image")
			if err != nil || len(candidates) == 0 {
				continue
			}
		}
		best := rankCandidates(candidates)
		licenseJSON, _ := json.Marshal(best.LicenseInfo)
		return &db.Asset{
			ID:          uuid.New(),
			ProjectID:   &projectID,
			Type:        best.Type,
			Source:      "stock",
			Provider:    best.Provider,
			URL:         best.URL,
			Width:       best.Width,
			Height:      best.Height,
			DurationSec: best.DurationSec,
			LicenseInfo: licenseJSON,
			CreatedAt:   time.Now().UTC(),
		}, nil
	}
	return nil, fmt.Errorf("no media found for query: %s", scene.VisualQuery)
}

// rankCandidates selects the best candidate based on vertical friendliness and quality.
func rankCandidates(candidates []*Candidate) *Candidate {
	best := candidates[0]
	for _, c := range candidates[1:] {
		// prefer vertical or square
		if c.Height > c.Width && best.Height <= best.Width {
			best = c
			continue
		}
		// prefer higher resolution
		if c.Width*c.Height > best.Width*best.Height {
			best = c
		}
	}
	return best
}

func (s *Service) placeholderAsset(projectID uuid.UUID, query string) *db.Asset {
	meta, _ := json.Marshal(map[string]string{"query": query, "type": "placeholder"})
	return &db.Asset{
		ID:        uuid.New(),
		ProjectID: &projectID,
		Type:      "image",
		Source:    "placeholder",
		Provider:  "internal",
		Metadata:  meta,
		CreatedAt: time.Now().UTC(),
	}
}

// PrepareAssets preprocesses downloaded assets for the render pipeline.
func (s *Service) PrepareAssets(ctx context.Context, assets []*db.Asset) error {
	for _, a := range assets {
		if err := s.prepareAsset(ctx, a); err != nil {
			return fmt.Errorf("prepare asset %s: %w", a.ID, err)
		}
	}
	return nil
}

func (s *Service) prepareAsset(_ context.Context, a *db.Asset) error {
	// In production: download, transcode, reframe to 9:16, generate proxy.
	// Here we validate the asset has required fields.
	if a.Type == "" {
		return fmt.Errorf("asset %s missing type", a.ID)
	}
	return nil
}
