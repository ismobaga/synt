// Package render assembles the final video using FFmpeg.
package render

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ismobaga/synt/internal/api"
	"github.com/ismobaga/synt/internal/db"
	"github.com/ismobaga/synt/pkg/ffmpeg"
)

// Service assembles and renders video projects.
type Service struct {
	db     *db.DB
	ffmpeg ffmpeg.Runner
}

// New creates a new render Service.
func New(database *db.DB, runner ffmpeg.Runner) *Service {
	return &Service{db: database, ffmpeg: runner}
}

// BuildTimeline creates the render manifest for a project.
func (s *Service) BuildTimeline(ctx context.Context, projectID uuid.UUID) error {
	project, err := s.db.GetProject(ctx, projectID)
	if err != nil {
		return err
	}
	assets, err := s.db.GetAssets(ctx, projectID)
	if err != nil {
		return err
	}
	tracks, err := s.db.GetAudioTracks(ctx, projectID)
	if err != nil {
		return err
	}
	subtitles, err := s.db.GetSubtitles(ctx, projectID)
	if err != nil {
		return err
	}
	script, err := s.db.GetScript(ctx, projectID)
	if err != nil {
		return err
	}

	manifest := buildManifest(project, assets, tracks, subtitles, script)
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	// Store manifest as an asset
	manifestAsset := &db.Asset{
		ID:          uuid.New(),
		ProjectID:   &projectID,
		Type:        "manifest",
		Source:      "generated",
		Provider:    "internal",
		StoragePath: fmt.Sprintf("projects/%s/manifest.json", projectID),
		MimeType:    "application/json",
		Metadata:    manifestJSON,
		CreatedAt:   time.Now().UTC(),
	}
	return s.db.CreateAsset(ctx, manifestAsset)
}

// RenderPreview renders a low-resolution preview.
func (s *Service) RenderPreview(ctx context.Context, projectID uuid.UUID) error {
	return s.renderAt(ctx, projectID, "preview", "720x1280", 24)
}

// RenderFinal renders the final HD video.
func (s *Service) RenderFinal(ctx context.Context, projectID uuid.UUID) error {
	return s.renderAt(ctx, projectID, "final", "1080x1920", 30)
}

// ExtractThumbnail extracts a thumbnail from the rendered video.
func (s *Service) ExtractThumbnail(ctx context.Context, projectID uuid.UUID) error {
	renders, err := s.db.GetRenders(ctx, projectID)
	if err != nil {
		return err
	}
	var finalRender *db.Render
	for _, r := range renders {
		if r.Kind == "final" && r.Status == "done" {
			finalRender = r
			break
		}
	}
	if finalRender == nil {
		return fmt.Errorf("no final render found for project %s", projectID)
	}

	thumbPath := filepath.Join("/tmp/synt/renders", projectID.String(), "thumbnail.jpg")
	if err := os.MkdirAll(filepath.Dir(thumbPath), 0o755); err != nil {
		return fmt.Errorf("create thumbnail dir: %w", err)
	}
	cmd := ffmpeg.Command{
		Args: []string{
			"-i", finalRender.StoragePath,
			"-ss", "00:00:01",
			"-vframes", "1",
			"-q:v", "2",
			thumbPath,
		},
	}
	if err := s.ffmpeg.Run(ctx, cmd); err != nil {
		return fmt.Errorf("extract thumbnail: %w", err)
	}
	return s.db.UpdateRenderStatus(ctx, finalRender.ID, "done", finalRender.StoragePath, thumbPath)
}

func (s *Service) renderAt(ctx context.Context, projectID uuid.UUID, kind, resolution string, fps int) error {
	outputPath := filepath.Join("/tmp/synt/renders", projectID.String(), fmt.Sprintf("%s.mp4", kind))
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create render dir: %w", err)
	}

	render := &db.Render{
		ID:         uuid.New(),
		ProjectID:  projectID,
		Kind:       kind,
		Resolution: resolution,
		FPS:        fps,
		Status:     "processing",
		CreatedAt:  time.Now().UTC(),
	}
	if err := s.db.CreateRender(ctx, render); err != nil {
		return err
	}

	assets, err := s.db.GetAssets(ctx, projectID)
	if err != nil {
		return err
	}
	tracks, err := s.db.GetAudioTracks(ctx, projectID)
	if err != nil {
		return err
	}
	subtitles, err := s.db.GetSubtitles(ctx, projectID)
	if err != nil {
		return err
	}

	cmd := buildFFmpegCommand(assets, tracks, subtitles, outputPath, resolution, fps)
	if err := s.ffmpeg.Run(ctx, cmd); err != nil {
		_ = s.db.UpdateRenderStatus(ctx, render.ID, "failed", "", "")
		return fmt.Errorf("ffmpeg render: %w", err)
	}

	return s.db.UpdateRenderStatus(ctx, render.ID, "done", outputPath, "")
}

func buildFFmpegCommand(assets []*db.Asset, tracks []*db.AudioTrack, subtitles []*db.Subtitle, output, resolution string, fps int) ffmpeg.Command {
	args := []string{"-y"}
	w, h := parseResolution(resolution)
	videoInputs := 0

	for _, a := range assets {
		inputPath := firstUsablePath(a.StoragePath, a.URL)
		if inputPath == "" {
			continue
		}
		switch a.Type {
		case "video":
			args = append(args, "-i", inputPath)
			videoInputs++
		case "image":
			args = append(args, "-loop", "1", "-t", "30", "-i", inputPath)
			videoInputs++
		}
	}

	if videoInputs == 0 {
		args = append(args, "-f", "lavfi", "-i", fmt.Sprintf("color=c=black:s=%d:%d:d=30", w, h))
	}

	for _, t := range tracks {
		if t.Kind == "voiceover" {
			if inputPath := firstUsablePath(t.StoragePath); inputPath != "" {
				args = append(args, "-i", inputPath)
			}
			break
		}
	}

	for _, t := range tracks {
		if t.Kind == "music" {
			if inputPath := firstUsablePath(t.StoragePath); inputPath != "" {
				args = append(args, "-i", inputPath)
			}
			break
		}
	}

	videoFilter := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2", w, h, w, h)
	if len(subtitles) > 0 && firstUsablePath(subtitles[0].StoragePath) != "" {
		videoFilter += fmt.Sprintf(",subtitles=%s", escapeFFmpegPath(subtitles[0].StoragePath))
	}

	args = append(args,
		"-vf", videoFilter,
		"-r", fmt.Sprintf("%d", fps),
		"-c:v", "libx264",
		"-preset", "medium",
		"-crf", "23",
		"-c:a", "aac",
		"-b:a", "128k",
		"-shortest",
		output,
	)

	return ffmpeg.Command{Args: args}
}

func firstUsablePath(paths ...string) string {
	for _, path := range paths {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
			return trimmed
		}
		if _, err := os.Stat(trimmed); err == nil {
			return trimmed
		}
	}
	return ""
}

func escapeFFmpegPath(path string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `:`, `\:`, `'`, `\\'`)
	return replacer.Replace(path)
}

func parseResolution(res string) (int, int) {
	// Formats: "1080x1920", "720x1280"
	var w, h int
	if _, err := fmt.Sscanf(res, "%dx%d", &w, &h); err != nil {
		return 1080, 1920
	}
	return w, h
}

func buildManifest(project *db.Project, assets []*db.Asset, tracks []*db.AudioTrack, subtitles []*db.Subtitle, script *db.Script) *api.TimelineManifest {
	manifest := &api.TimelineManifest{
		ProjectID:   project.ID.String(),
		Resolution:  api.ManifestResolution{Width: 1080, Height: 1920},
		FPS:         30,
		DurationSec: float64(project.DurationSec),
		Template:    project.TemplateID,
	}

	// Parse scenes from script
	var sc struct {
		Scenes []struct {
			Index       int     `json:"index"`
			DurationSec float64 `json:"duration_sec"`
			Caption     string  `json:"caption"`
			Narration   string  `json:"narration"`
		} `json:"scenes"`
	}
	_ = json.Unmarshal(script.ContentJSON, &sc)

	var cursor float64
	for i, scene := range sc.Scenes {
		ms := &api.ManifestScene{
			Index:         scene.Index,
			StartSec:      cursor,
			EndSec:        cursor + scene.DurationSec,
			TransitionOut: "quick_fade",
		}
		// Attach media
		if i < len(assets) {
			ms.Media = api.ManifestMedia{
				Type:    assets[i].Type,
				Path:    assets[i].StoragePath,
				FitMode: "cover",
			}
		}
		// Attach voiceover timing
		for _, t := range tracks {
			if t.Kind == "voiceover" {
				ms.Voiceover = api.ManifestVoiceover{
					Path:     t.StoragePath,
					StartSec: cursor,
				}
				break
			}
		}
		// Caption
		if scene.Caption != "" {
			ms.Captions = append(ms.Captions, api.ManifestCaption{
				StartSec: cursor + 0.2,
				EndSec:   cursor + scene.DurationSec - 0.2,
				Text:     scene.Caption,
			})
		}
		manifest.Scenes = append(manifest.Scenes, *ms)
		cursor += scene.DurationSec
	}

	// Music
	for _, t := range tracks {
		if t.Kind == "music" {
			manifest.Music = api.ManifestMusic{
				Path:       t.StoragePath,
				Volume:     0.18,
				FadeInSec:  0.5,
				FadeOutSec: 1.0,
			}
			break
		}
	}

	return manifest
}
