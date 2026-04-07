package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ismobaga/synt/internal/db"
)

func TestBuildFFmpegCommandUsesSceneConcatAndAudioMix(t *testing.T) {
	projectID := uuid.New()
	manifest := `{
		"duration_sec": 10,
		"scenes": [
			{"index":0,"start_sec":0,"end_sec":5,"media":{"type":"video","path":"https://cdn.example.com/scene1.mp4"}},
			{"index":1,"start_sec":5,"end_sec":10,"media":{"type":"image","path":"https://cdn.example.com/scene2.jpg"}}
		],
		"music":{"path":"/tmp/music.mp3","volume":0.18}
	}`
	assets := []*db.Asset{
		{
			ID:          uuid.New(),
			ProjectID:   &projectID,
			Type:        "manifest",
			StoragePath: "projects/123/manifest.json",
			Metadata:    []byte(manifest),
			CreatedAt:   time.Now().UTC(),
		},
	}
	tracks := []*db.AudioTrack{
		{ID: uuid.New(), ProjectID: projectID, Kind: "voiceover", StoragePath: "https://cdn.example.com/voice.wav", DurationSec: 9.2},
		{ID: uuid.New(), ProjectID: projectID, Kind: "music", StoragePath: "https://cdn.example.com/music.mp3", DurationSec: 30},
	}
	subtitles := []*db.Subtitle{{ID: uuid.New(), ProjectID: projectID, Format: "srt", StoragePath: "/tmp/captions.srt"}}

	cmd := buildFFmpegCommand(assets, tracks, subtitles, "/tmp/out.mp4", "1080x1920", 30)
	joined := strings.Join(cmd.Args, " ")

	for _, want := range []string{"-filter_complex", "concat=n=2:v=1:a=0[vcat]", "amix=inputs=2", "-map [vout]", "-map [aout]"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected command to contain %q, got: %s", want, joined)
		}
	}
}

func TestBuildFFmpegCommandUsesValidPlaceholderSource(t *testing.T) {
	cmd := buildFFmpegCommand(nil, nil, nil, "/tmp/out.mp4", "720x1280", 24)
	joined := strings.Join(cmd.Args, " ")
	if !strings.Contains(joined, "color=c=black:s=720x1280") {
		t.Fatalf("expected placeholder source to use a valid size, got: %s", joined)
	}
}

func TestValidateRenderedOutputRejectsEmptyAndAcceptsMp4Header(t *testing.T) {
	tmpDir := t.TempDir()
	emptyPath := filepath.Join(tmpDir, "empty.mp4")
	if err := os.WriteFile(emptyPath, nil, 0o644); err != nil {
		t.Fatalf("write empty file: %v", err)
	}
	if err := validateRenderedOutput(emptyPath); err == nil {
		t.Fatal("expected empty file validation to fail")
	}

	goodPath := filepath.Join(tmpDir, "good.mp4")
	if err := os.WriteFile(goodPath, []byte("\x00\x00\x00\x18ftypisomsynt-render-test"), 0o644); err != nil {
		t.Fatalf("write sample mp4 file: %v", err)
	}
	if err := validateRenderedOutput(goodPath); err != nil {
		t.Fatalf("expected mp4 header validation to pass, got %v", err)
	}
}

func TestBuildManifestMarksRemotionProjects(t *testing.T) {
	projectID := uuid.New()
	project := &db.Project{
		ID:          projectID,
		DurationSec: 15,
		TemplateID:  "remotion_fast_caption_v1",
	}
	script := &db.Script{
		ID:        uuid.New(),
		ProjectID: projectID,
		ContentJSON: []byte(`{
			"scenes": [
				{"index": 0, "duration_sec": 5, "caption": "Hook", "narration": "Intro"},
				{"index": 1, "duration_sec": 5, "caption": "Value", "narration": "Middle"}
			]
		}`),
	}
	assets := []*db.Asset{{
		ID:          uuid.New(),
		ProjectID:   &projectID,
		Type:        "image",
		StoragePath: "https://cdn.example.com/scene.jpg",
		CreatedAt:   time.Now().UTC(),
	}}

	manifest := buildManifest(project, assets, nil, nil, script)
	if manifest.RenderEngine != "remotion" {
		t.Fatalf("expected remotion render engine, got %q", manifest.RenderEngine)
	}
	if manifest.Template != project.TemplateID {
		t.Fatalf("expected template %q, got %q", project.TemplateID, manifest.Template)
	}
	if len(manifest.Scenes) != 2 {
		t.Fatalf("expected 2 scenes, got %d", len(manifest.Scenes))
	}
}
