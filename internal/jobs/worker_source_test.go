package jobs

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ismobaga/synt/internal/db"
)

func TestSourceMaterialContextFromAssetsIncludesFetchedContent(t *testing.T) {
	projectID := uuid.New()
	assets := []*db.Asset{
		{
			ID:        uuid.New(),
			ProjectID: &projectID,
			Type:      "source_material",
			URL:       "https://example.com/article",
			Metadata:  []byte(`{"title":"AI Article","content_text":"This article explains practical automation ideas.","grounding_facts":["Automation can remove repetitive manual tasks.","Small teams benefit from lightweight tooling."],"fetch_status":"fetched"}`),
			CreatedAt: time.Now().UTC(),
		},
		{
			ID:        uuid.New(),
			ProjectID: &projectID,
			Type:      "source_note",
			Metadata:  []byte(`{"notes":"Keep the script concise and actionable."}`),
			CreatedAt: time.Now().UTC(),
		},
	}

	urls, notes := sourceMaterialContextFromAssets(assets)
	if len(urls) != 1 || urls[0] != "https://example.com/article" {
		t.Fatalf("unexpected source urls: %#v", urls)
	}
	for _, want := range []string{"AI Article", "practical automation ideas", "Automation can remove repetitive manual tasks", "Keep the script concise"} {
		if !strings.Contains(notes, want) {
			t.Fatalf("expected notes to contain %q, got %q", want, notes)
		}
	}
}
