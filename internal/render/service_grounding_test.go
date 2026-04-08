package render

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ismobaga/synt/internal/db"
)

func TestBuildManifestMatchesAssetsBySceneIndexMetadata(t *testing.T) {
	projectID := uuid.New()
	project := &db.Project{ID: projectID, DurationSec: 10, TemplateID: "fast_caption_v1"}
	script := &db.Script{
		ID:        uuid.New(),
		ProjectID: projectID,
		ContentJSON: []byte(`{
			"scenes": [
				{"index": 1, "duration_sec": 5, "caption": "First", "narration": "First narration", "visual_query": "scene one"},
				{"index": 2, "duration_sec": 5, "caption": "Second", "narration": "Second narration", "visual_query": "scene two"}
			]
		}`),
	}
	assets := []*db.Asset{
		{ID: uuid.New(), ProjectID: &projectID, Type: "image", StoragePath: "https://cdn.example.com/scene-two.jpg", Metadata: []byte(`{"scene_index":2}`), CreatedAt: time.Now().UTC()},
		{ID: uuid.New(), ProjectID: &projectID, Type: "image", StoragePath: "https://cdn.example.com/scene-one.jpg", Metadata: []byte(`{"scene_index":1}`), CreatedAt: time.Now().UTC()},
	}

	manifest := buildManifest(project, assets, nil, nil, script)
	if len(manifest.Scenes) != 2 {
		t.Fatalf("expected 2 scenes, got %d", len(manifest.Scenes))
	}
	if got := manifest.Scenes[0].Media.Path; got != "https://cdn.example.com/scene-one.jpg" {
		t.Fatalf("scene 1 should use scene-one asset, got %q", got)
	}
	if got := manifest.Scenes[1].Media.Path; got != "https://cdn.example.com/scene-two.jpg" {
		t.Fatalf("scene 2 should use scene-two asset, got %q", got)
	}
}
