package voice_test

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/ismobaga/synt/internal/voice"
	"github.com/ismobaga/synt/pkg/tts"
)

type fakeTTSClient struct {
	outputPath string
}

func (f *fakeTTSClient) Synthesize(_ context.Context, req tts.SynthesizeRequest) (*tts.SynthesizeResult, error) {
	meta, _ := json.Marshal(map[string]any{
		"transcript": req.Text,
	})
	return &tts.SynthesizeResult{
		StoragePath: f.outputPath,
		DurationSec: 4.2,
		VoiceName:   req.Voice,
		Metadata:    meta,
	}, nil
}

type fakeStorageClient struct {
	uploadPath  string
	contentType string
	body        []byte
}

func (f *fakeStorageClient) Upload(_ context.Context, path string, r io.Reader, contentType string) (string, error) {
	data, _ := io.ReadAll(r)
	f.uploadPath = path
	f.contentType = contentType
	f.body = data
	return "http://files.example/" + path, nil
}

func (f *fakeStorageClient) Download(_ context.Context, _ string) (io.ReadCloser, error) {
	return nil, nil
}
func (f *fakeStorageClient) Delete(_ context.Context, _ string) error { return nil }
func (f *fakeStorageClient) URL(path string) string                   { return "http://files.example/" + path }

func TestGenerateUploadsVoiceoverAndPublishesURL(t *testing.T) {
	tmp, err := os.CreateTemp(t.TempDir(), "voiceover-*.wav")
	if err != nil {
		t.Fatalf("CreateTemp returned error: %v", err)
	}
	if _, err := tmp.WriteString("fake wav data"); err != nil {
		t.Fatalf("WriteString returned error: %v", err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	storage := &fakeStorageClient{}
	svc := voice.New(&fakeTTSClient{outputPath: tmp.Name()}, storage)

	scriptJSON := []byte(`{"scenes":[{"narration":"Hello world"},{"narration":"From Synt"}]}`)
	track, err := svc.Generate(context.Background(), uuid.New(), scriptJSON, "en")
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	if storage.uploadPath == "" {
		t.Fatal("expected generated audio to be uploaded")
	}
	if storage.contentType != "audio/wav" {
		t.Fatalf("unexpected content type: %s", storage.contentType)
	}
	if string(storage.body) != "fake wav data" {
		t.Fatalf("unexpected uploaded body: %q", string(storage.body))
	}
	if track.StoragePath != tmp.Name() {
		t.Fatalf("expected local storage path to be preserved, got %s", track.StoragePath)
	}

	var meta map[string]any
	if err := json.Unmarshal(track.Metadata, &meta); err != nil {
		t.Fatalf("Unmarshal metadata returned error: %v", err)
	}
	if meta["public_url"] != "http://files.example/"+storage.uploadPath {
		t.Fatalf("unexpected public_url: %#v", meta["public_url"])
	}
	if meta["local_path"] != tmp.Name() {
		t.Fatalf("unexpected local_path: %#v", meta["local_path"])
	}
}
