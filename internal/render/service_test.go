package render

import (
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
