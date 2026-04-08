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
func (s *Service) SearchAssets(ctx context.Context, projectID uuid.UUID, scriptJSON []byte, existingAssets ...*db.Asset) ([]*db.Asset, error) {
	var script content.ScriptContent
	if err := json.Unmarshal(scriptJSON, &script); err != nil {
		return nil, fmt.Errorf("parse script: %w", err)
	}

	var assets []*db.Asset
	for index, scene := range script.Scenes {
		if preservedAssetForScene(existingAssets, scene, index) != nil {
			continue
		}
		asset, err := s.searchForScene(ctx, projectID, scene)
		if err != nil {
			// fallback: use a placeholder
			asset = s.placeholderAsset(projectID, scene)
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
			StoragePath: best.URL,
			MimeType:    mimeTypeForAsset(best.Type),
			Width:       best.Width,
			Height:      best.Height,
			DurationSec: best.DurationSec,
			LicenseInfo: licenseJSON,
			Metadata:    sceneAssetMetadata(scene, false),
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

func (s *Service) placeholderAsset(projectID uuid.UUID, scene content.SceneContent) *db.Asset {
	return &db.Asset{
		ID:        uuid.New(),
		ProjectID: &projectID,
		Type:      "image",
		Source:    "placeholder",
		Provider:  "internal",
		Metadata:  sceneAssetMetadata(scene, true),
		CreatedAt: time.Now().UTC(),
	}
}

func sceneAssetMetadata(scene content.SceneContent, placeholder bool) []byte {
	payload := map[string]any{
		"query":        scene.VisualQuery,
		"scene_index":  scene.Index,
		"scene_locked": scene.Locked,
	}
	if placeholder {
		payload["type"] = "placeholder"
	}
	if len(scene.SourceFactIDs) > 0 {
		payload["source_fact_ids"] = scene.SourceFactIDs
	}
	if len(scene.SourceFacts) > 0 {
		payload["source_facts"] = scene.SourceFacts
	}
	meta, _ := json.Marshal(payload)
	return meta
}

func preservedAssetForScene(existingAssets []*db.Asset, scene content.SceneContent, fallbackIndex int) *db.Asset {
	targetIndex := scene.Index
	if targetIndex <= 0 {
		targetIndex = fallbackIndex + 1
	}
	var preserved *db.Asset
	for _, asset := range existingAssets {
		if asset == nil || (asset.Type != "video" && asset.Type != "image") {
			continue
		}
		if sceneIndexFromAssetMetadata(asset.Metadata, fallbackIndex+1) != targetIndex {
			continue
		}
		if !scene.Locked && !assetMetadataBool(asset.Metadata, "manual_override") && !assetMetadataBool(asset.Metadata, "scene_locked") {
			continue
		}
		if preserved == nil || asset.CreatedAt.After(preserved.CreatedAt) {
			preserved = asset
		}
	}
	return preserved
}

func sceneIndexFromAssetMetadata(metadata []byte, fallback int) int {
	if len(metadata) == 0 {
		return fallback
	}
	var payload map[string]any
	if err := json.Unmarshal(metadata, &payload); err != nil {
		return fallback
	}
	if value, ok := payload["scene_index"]; ok {
		switch typed := value.(type) {
		case float64:
			if typed > 0 {
				return int(typed)
			}
		case int:
			if typed > 0 {
				return typed
			}
		}
	}
	return fallback
}

func assetMetadataBool(metadata []byte, key string) bool {
	if len(metadata) == 0 {
		return false
	}
	var payload map[string]any
	if err := json.Unmarshal(metadata, &payload); err != nil {
		return false
	}
	value, ok := payload[key]
	if !ok {
		return false
	}
	flag, _ := value.(bool)
	return flag
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
	// Here we keep the direct media URL as the render input when no local file exists yet.
	if a.Type == "" {
		return fmt.Errorf("asset %s missing type", a.ID)
	}
	if a.StoragePath == "" {
		a.StoragePath = a.URL
	}
	if a.MimeType == "" {
		a.MimeType = mimeTypeForAsset(a.Type)
	}
	return nil
}

func mimeTypeForAsset(assetType string) string {
	switch assetType {
	case "video":
		return "video/mp4"
	case "image":
		return "image/jpeg"
	case "manifest":
		return "application/json"
	default:
		return "application/octet-stream"
	}
}
