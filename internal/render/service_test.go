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
