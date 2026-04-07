// Package render assembles the final video using FFmpeg and prepares Remotion-ready manifests.
package render

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ismobaga/synt/internal/api"
	"github.com/ismobaga/synt/internal/db"
	"github.com/ismobaga/synt/pkg/ffmpeg"
	"github.com/ismobaga/synt/pkg/s3util"
)

// Service assembles and renders video projects.
type Service struct {
	db      *db.DB
	ffmpeg  ffmpeg.Runner
	storage s3util.Client
}

// New creates a new render Service.
func New(database *db.DB, runner ffmpeg.Runner, storage ...s3util.Client) *Service {
	var store s3util.Client
	if len(storage) > 0 {
		store = storage[0]
	}
	return &Service{db: database, ffmpeg: runner, storage: store}
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
	inputPath := firstUsablePath(finalRender.StoragePath)
	if inputPath == "" {
		return fmt.Errorf("final render path unavailable for thumbnail extraction")
	}
	cmd := ffmpeg.Command{
		Args: []string{
			"-i", inputPath,
			"-ss", "00:00:01",
			"-vframes", "1",
			"-q:v", "2",
			thumbPath,
		},
	}
	if err := s.ffmpeg.Run(ctx, cmd); err != nil {
		return fmt.Errorf("extract thumbnail from %s: %w", inputPath, err)
	}

	finalPath := finalRender.StoragePath
	thumbnailPath := thumbPath
	if published, err := s.publishFile(ctx, projectID, filepath.Base(finalRender.StoragePath), finalRender.StoragePath, "video/mp4", "renders"); err == nil {
		finalPath = published
	}
	if published, err := s.publishFile(ctx, projectID, filepath.Base(thumbPath), thumbPath, "image/jpeg", "renders"); err == nil {
		thumbnailPath = published
	}
	return s.db.UpdateRenderStatus(ctx, finalRender.ID, "done", finalPath, thumbnailPath)
}

func (s *Service) renderAt(ctx context.Context, projectID uuid.UUID, kind, resolution string, fps int) error {
	outputPath := filepath.Join("/tmp/synt/renders", projectID.String(), fmt.Sprintf("%s.mp4", kind))
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create render dir: %w", err)
	}

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

	plan := buildRenderPlan(assets, tracks, subtitles)
	engine := renderEngineForTemplate(project.TemplateID)
	renderMetadata, _ := json.Marshal(map[string]any{
		"renderer":           engine,
		"template":           project.TemplateID,
		"template_base":      baseTemplateID(project.TemplateID),
		"composition":        "SyntProgrammaticVideo",
		"scene_count":        len(plan.Scenes),
		"has_voiceover":      plan.VoicePath != "",
		"has_music":          plan.MusicPath != "",
		"has_subtitles":      plan.SubtitlePath != "",
		"total_duration_sec": plan.TotalDuration,
	})

	render := &db.Render{
		ID:         uuid.New(),
		ProjectID:  projectID,
		Kind:       kind,
		Resolution: resolution,
		FPS:        fps,
		Status:     "processing",
		Metadata:   renderMetadata,
		CreatedAt:  time.Now().UTC(),
	}
	if err := s.db.CreateRender(ctx, render); err != nil {
		return err
	}

	cmd := buildFFmpegCommandForPlan(plan, outputPath, resolution, fps)
	if err := s.ffmpeg.Run(ctx, cmd); err != nil {
		_ = s.db.UpdateRenderStatus(ctx, render.ID, "failed", "", "")
		return fmt.Errorf("%s render failed at %s: %w [plan: %s] [args: %s]", kind, resolution, err, summarizeRenderPlan(plan), truncateForLog(strings.Join(cmd.Args, " "), 900))
	}
	if err := validateRenderedOutput(outputPath); err != nil {
		_ = s.db.UpdateRenderStatus(ctx, render.ID, "failed", "", "")
		return fmt.Errorf("%s render produced invalid output at %s: %w [plan: %s]", kind, resolution, err, summarizeRenderPlan(plan))
	}

	storedPath := outputPath
	if published, err := s.publishFile(ctx, projectID, fmt.Sprintf("%s.mp4", kind), outputPath, "video/mp4", "renders"); err == nil {
		storedPath = published
	} else if s.storage != nil {
		log.Printf("[render] upload warning for %s %s project=%s: %v", kind, resolution, projectID, err)
	}
	return s.db.UpdateRenderStatus(ctx, render.ID, "done", storedPath, "")
}

func (s *Service) publishFile(ctx context.Context, projectID uuid.UUID, fileName, localPath, contentType, category string) (string, error) {
	if s.storage == nil {
		return localPath, fmt.Errorf("storage not configured")
	}
	trimmed := strings.TrimSpace(localPath)
	if trimmed == "" || strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return localPath, fmt.Errorf("local file not available for upload")
	}
	file, err := os.Open(trimmed)
	if err != nil {
		return localPath, fmt.Errorf("open artifact for upload: %w", err)
	}
	defer file.Close()

	objectPath := fmt.Sprintf("projects/%s/%s/%s", projectID, strings.Trim(category, "/"), fileName)
	url, err := s.storage.Upload(ctx, objectPath, file, contentType)
	if err != nil {
		return localPath, fmt.Errorf("upload artifact: %w", err)
	}
	return url, nil
}

func buildFFmpegCommand(assets []*db.Asset, tracks []*db.AudioTrack, subtitles []*db.Subtitle, output, resolution string, fps int) ffmpeg.Command {
	return buildFFmpegCommandForPlan(buildRenderPlan(assets, tracks, subtitles), output, resolution, fps)
}

func buildFFmpegCommandForPlan(plan renderPlan, output, resolution string, fps int) ffmpeg.Command {
	args := []string{"-y"}
	w, h := parseResolution(resolution)
	if plan.TotalDuration <= 0 {
		plan.TotalDuration = 5
	}
	if len(plan.Scenes) == 0 {
		plan.Scenes = []renderScene{{Kind: "color", DurationSec: plan.TotalDuration}}
	}

	filterParts := make([]string, 0, len(plan.Scenes)+4)
	videoLabels := make([]string, 0, len(plan.Scenes))
	inputIndex := 0

	for i, scene := range plan.Scenes {
		duration := normalizeDuration(scene.DurationSec, 5)
		path := firstUsablePath(scene.Path)
		switch {
		case scene.Kind == "image" && path != "":
			args = append(args, "-loop", "1", "-t", formatDuration(duration), "-i", path)
		case path != "":
			args = append(args, "-t", formatDuration(duration), "-i", path)
		default:
			args = append(args, "-f", "lavfi", "-t", formatDuration(duration), "-i", fmt.Sprintf("color=c=black:s=%dx%d", w, h))
		}

		label := fmt.Sprintf("v%d", i)
		filterParts = append(filterParts,
			fmt.Sprintf("[%d:v]scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,setsar=1,fps=%d,format=yuv420p,trim=duration=%s,setpts=PTS-STARTPTS[%s]",
				inputIndex, w, h, w, h, fps, formatDuration(duration), label),
		)
		videoLabels = append(videoLabels, fmt.Sprintf("[%s]", label))
		inputIndex++
	}

	if len(videoLabels) == 1 {
		filterParts = append(filterParts, fmt.Sprintf("%snull[vcat]", videoLabels[0]))
	} else {
		filterParts = append(filterParts, fmt.Sprintf("%sconcat=n=%d:v=1:a=0[vcat]", strings.Join(videoLabels, ""), len(videoLabels)))
	}

	if plan.SubtitlePath != "" {
		filterParts = append(filterParts, fmt.Sprintf("[vcat]%s[vout]", buildSubtitleFilter(plan.SubtitlePath, plan.SubtitleStyle)))
	} else {
		filterParts = append(filterParts, "[vcat]null[vout]")
	}

	audioLabels := make([]string, 0, 2)
	if plan.VoicePath != "" {
		args = append(args, "-i", plan.VoicePath)
		filterParts = append(filterParts,
			fmt.Sprintf("[%d:a]aresample=44100,apad=pad_dur=%s,atrim=duration=%s[voice]", inputIndex, formatDuration(plan.TotalDuration), formatDuration(plan.TotalDuration)),
		)
		audioLabels = append(audioLabels, "[voice]")
		inputIndex++
	}
	if plan.MusicPath != "" {
		args = append(args, "-stream_loop", "-1", "-i", plan.MusicPath)
		filterParts = append(filterParts,
			fmt.Sprintf("[%d:a]aresample=44100,volume=%.2f,atrim=duration=%s[music]", inputIndex, plan.MusicVolume, formatDuration(plan.TotalDuration)),
		)
		audioLabels = append(audioLabels, "[music]")
		inputIndex++
	}

	switch len(audioLabels) {
	case 0:
		args = append(args, "-f", "lavfi", "-t", formatDuration(plan.TotalDuration), "-i", "anullsrc=r=44100:cl=stereo")
		filterParts = append(filterParts, fmt.Sprintf("[%d:a]atrim=duration=%s[aout]", inputIndex, formatDuration(plan.TotalDuration)))
	case 1:
		filterParts = append(filterParts, fmt.Sprintf("%sanull[aout]", audioLabels[0]))
	default:
		filterParts = append(filterParts,
			fmt.Sprintf("%samix=inputs=%d:duration=longest:dropout_transition=2,atrim=duration=%s[aout]", strings.Join(audioLabels, ""), len(audioLabels), formatDuration(plan.TotalDuration)),
		)
	}

	args = append(args,
		"-filter_complex", strings.Join(filterParts, ";"),
		"-map", "[vout]",
		"-map", "[aout]",
		"-r", fmt.Sprintf("%d", fps),
		"-c:v", "libx264",
		"-preset", "medium",
		"-crf", "23",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "128k",
		"-movflags", "+faststart",
		"-t", formatDuration(plan.TotalDuration),
		output,
	)

	return ffmpeg.Command{Args: args}
}

type renderPlan struct {
	Scenes        []renderScene
	VoicePath     string
	MusicPath     string
	SubtitlePath  string
	SubtitleStyle api.SubtitleStyle
	MusicVolume   float64
	TotalDuration float64
}

type renderScene struct {
	Kind        string
	Path        string
	DurationSec float64
}

func buildRenderPlan(assets []*db.Asset, tracks []*db.AudioTrack, subtitles []*db.Subtitle) renderPlan {
	plan := renderPlan{MusicVolume: 0.18}
	if len(subtitles) > 0 {
		plan.SubtitlePath = firstUsablePath(subtitles[0].StoragePath)
	}
	for _, track := range tracks {
		switch track.Kind {
		case "voiceover":
			if plan.VoicePath == "" {
				plan.VoicePath = firstUsablePath(track.StoragePath)
			}
		case "music":
			if plan.MusicPath == "" {
				plan.MusicPath = firstUsablePath(track.StoragePath)
			}
		}
	}

	if manifest, ok := latestManifestAsset(assets); ok {
		for _, scene := range manifest.Scenes {
			duration := normalizeDuration(scene.EndSec-scene.StartSec, 5)
			plan.Scenes = append(plan.Scenes, renderScene{
				Kind:        inferMediaType(scene.Media.Type, scene.Media.Path),
				Path:        scene.Media.Path,
				DurationSec: duration,
			})
			plan.TotalDuration += duration
		}
		if plan.MusicPath == "" {
			plan.MusicPath = firstUsablePath(manifest.Music.Path)
		}
		if manifest.Music.Volume > 0 {
			plan.MusicVolume = manifest.Music.Volume
		}
		plan.SubtitleStyle = manifest.SubtitleStyle
	}

	if len(plan.Scenes) == 0 {
		for _, asset := range filterRenderableAssets(assets) {
			path := firstUsablePath(asset.StoragePath, asset.URL)
			plan.Scenes = append(plan.Scenes, renderScene{
				Kind:        inferMediaType(asset.Type, path),
				Path:        path,
				DurationSec: normalizeDuration(asset.DurationSec, 5),
			})
			plan.TotalDuration += normalizeDuration(asset.DurationSec, 5)
		}
	}

	if plan.TotalDuration <= 0 {
		plan.TotalDuration = 5
	}
	return plan
}

func latestManifestAsset(assets []*db.Asset) (*api.TimelineManifest, bool) {
	for i := len(assets) - 1; i >= 0; i-- {
		asset := assets[i]
		if asset == nil || asset.Type != "manifest" || len(asset.Metadata) == 0 {
			continue
		}
		var manifest api.TimelineManifest
		if err := json.Unmarshal(asset.Metadata, &manifest); err == nil && len(manifest.Scenes) > 0 {
			return &manifest, true
		}
	}
	return nil, false
}

func filterRenderableAssets(assets []*db.Asset) []*db.Asset {
	filtered := make([]*db.Asset, 0, len(assets))
	seen := map[string]bool{}
	for _, asset := range assets {
		if asset == nil || (asset.Type != "video" && asset.Type != "image") {
			continue
		}
		path := firstUsablePath(asset.StoragePath, asset.URL)
		key := asset.Type + "|" + path
		if seen[key] {
			continue
		}
		seen[key] = true
		filtered = append(filtered, asset)
	}
	return filtered
}

func inferMediaType(assetType, path string) string {
	if trimmed := strings.TrimSpace(assetType); trimmed != "" {
		return trimmed
	}
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"), strings.HasSuffix(lower, ".png"), strings.HasSuffix(lower, ".webp"):
		return "image"
	default:
		return "video"
	}
}

func normalizeDuration(value, fallback float64) float64 {
	if value <= 0 {
		value = fallback
	}
	if value <= 0 {
		value = 5
	}
	return value
}

func formatDuration(value float64) string {
	return fmt.Sprintf("%.2f", normalizeDuration(value, 5))
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

func validateRenderedOutput(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("render file missing: %w", err)
	}
	if info.Size() <= 0 {
		return fmt.Errorf("render file is empty")
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open render file: %w", err)
	}
	defer file.Close()

	buffer := make([]byte, 64)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("read render file header: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("render file header is empty")
	}
	if !strings.Contains(string(buffer[:n]), "ftyp") {
		return fmt.Errorf("render file does not look like a playable mp4")
	}
	return nil
}

func summarizeRenderPlan(plan renderPlan) string {
	placeholderCount := 0
	for _, scene := range plan.Scenes {
		if firstUsablePath(scene.Path) == "" {
			placeholderCount++
		}
	}
	return fmt.Sprintf("scenes=%d placeholders=%d voice=%t music=%t subtitles=%t total_duration=%.2fs",
		len(plan.Scenes), placeholderCount, plan.VoicePath != "", plan.MusicPath != "", plan.SubtitlePath != "", normalizeDuration(plan.TotalDuration, 5),
	)
}

func truncateForLog(value string, max int) string {
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max] + "…"
}

func escapeFFmpegPath(path string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `:`, `\:`, `'`, `\\'`)
	return replacer.Replace(path)
}

func buildSubtitleFilter(path string, style api.SubtitleStyle) string {
	filter := fmt.Sprintf("subtitles='%s'", escapeFFmpegPath(path))
	fontSize := style.FontSize
	if fontSize <= 0 {
		switch strings.ToLower(strings.TrimSpace(style.Preset)) {
		case "bold", "hook":
			fontSize = 18
		default:
			fontSize = 14
		}
	}
	outline := 2
	if strings.EqualFold(strings.TrimSpace(style.Preset), "minimal") || strings.EqualFold(strings.TrimSpace(style.Preset), "clean") {
		outline = 1
	}
	styleParts := []string{
		fmt.Sprintf("Alignment=%d", subtitleAlignment(style.Position)),
		fmt.Sprintf("Fontsize=%d", fontSize),
		fmt.Sprintf("Outline=%d", outline),
		"Shadow=0",
		"BorderStyle=1",
	}
	if style.Bold {
		styleParts = append(styleParts, "Bold=1")
	}
	if color := assColorFromHex(style.PrimaryColor); color != "" {
		styleParts = append(styleParts, "PrimaryColour="+color)
	}
	if color := assColorFromHex(style.OutlineColor); color != "" {
		styleParts = append(styleParts, "OutlineColour="+color)
	}
	return filter + ":force_style='" + strings.Join(styleParts, ",") + "'"
}

func subtitleAlignment(position string) int {
	switch strings.ToLower(strings.TrimSpace(position)) {
	case "top":
		return 8
	case "center", "middle":
		return 5
	default:
		return 2
	}
}

func assColorFromHex(value string) string {
	trimmed := strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(trimmed) != 6 {
		return ""
	}
	return "&H00" + strings.ToUpper(trimmed[4:6]+trimmed[2:4]+trimmed[0:2]) + "&"
}

func parseResolution(res string) (int, int) {
	// Formats: "1080x1920", "720x1280"
	var w, h int
	if _, err := fmt.Sscanf(res, "%dx%d", &w, &h); err != nil {
		return 1080, 1920
	}
	return w, h
}

func renderEngineForTemplate(templateID string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(templateID)), "remotion_") {
		return api.RenderEngineRemotion
	}
	return api.RenderEngineFFmpeg
}

func baseTemplateID(templateID string) string {
	trimmed := strings.TrimSpace(templateID)
	if strings.HasPrefix(strings.ToLower(trimmed), "remotion_") && len(trimmed) > len("remotion_") {
		return trimmed[len("remotion_"):]
	}
	return trimmed
}

func buildManifest(project *db.Project, assets []*db.Asset, tracks []*db.AudioTrack, subtitles []*db.Subtitle, script *db.Script) *api.TimelineManifest {
	manifest := &api.TimelineManifest{
		ProjectID:    project.ID.String(),
		Resolution:   api.ManifestResolution{Width: 1080, Height: 1920},
		FPS:          30,
		DurationSec:  float64(project.DurationSec),
		Template:     project.TemplateID,
		RenderEngine: renderEngineForTemplate(project.TemplateID),
		SubtitleStyle: api.SubtitleStyle{
			Preset:       "clean",
			Position:     "bottom",
			FontSize:     14,
			PrimaryColor: "#FFFFFF",
			OutlineColor: "#111827",
		},
	}

	var sc api.ScriptContent
	_ = json.Unmarshal(script.ContentJSON, &sc)
	if sc.SubtitleStyle.Preset != "" {
		manifest.SubtitleStyle.Preset = sc.SubtitleStyle.Preset
	}
	if sc.SubtitleStyle.Position != "" {
		manifest.SubtitleStyle.Position = sc.SubtitleStyle.Position
	}
	if sc.SubtitleStyle.FontSize > 0 {
		manifest.SubtitleStyle.FontSize = sc.SubtitleStyle.FontSize
	}
	if sc.SubtitleStyle.PrimaryColor != "" {
		manifest.SubtitleStyle.PrimaryColor = sc.SubtitleStyle.PrimaryColor
	}
	if sc.SubtitleStyle.OutlineColor != "" {
		manifest.SubtitleStyle.OutlineColor = sc.SubtitleStyle.OutlineColor
	}
	manifest.SubtitleStyle.Bold = sc.SubtitleStyle.Bold

	mediaAssets := filterRenderableAssets(assets)
	var cursor float64
	for i, scene := range sc.Scenes {
		durationSec := normalizeDuration(float64(scene.DurationSec), 5)
		ms := &api.ManifestScene{
			Index:         scene.Index,
			StartSec:      cursor,
			EndSec:        cursor + durationSec,
			TransitionOut: "quick_fade",
		}
		// Attach media
		if i < len(mediaAssets) {
			ms.Media = api.ManifestMedia{
				Type:    mediaAssets[i].Type,
				Path:    firstUsablePath(mediaAssets[i].StoragePath, mediaAssets[i].URL),
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
				EndSec:   cursor + durationSec - 0.2,
				Text:     scene.Caption,
			})
			ms.TextOverlays = append(ms.TextOverlays, api.ManifestTextOverlay{
				Text:     scene.Caption,
				Position: manifest.SubtitleStyle.Position,
				Style:    scene.OverlayStyle,
			})
		}
		manifest.Scenes = append(manifest.Scenes, *ms)
		cursor += durationSec
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
