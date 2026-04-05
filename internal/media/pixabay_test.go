package media

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPixabayProviderSearchImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/" {
			t.Fatalf("expected /api/, got %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("q"); got != "city skyline" {
			t.Fatalf("expected q=city skyline, got %q", got)
		}
		_, _ = w.Write([]byte(`{
			"hits": [{
				"id": 101,
				"pageURL": "https://pixabay.com/photos/city-101/",
				"largeImageURL": "https://cdn.pixabay.com/photo-101.jpg",
				"imageWidth": 1080,
				"imageHeight": 1920,
				"user": "pixabay-user",
				"tags": "city, skyline"
			}]
		}`))
	}))
	defer server.Close()

	provider := NewPixabayProvider("test-key")
	provider.baseURL = server.URL
	provider.httpClient = server.Client()

	results, err := provider.Search(context.Background(), "city skyline", "image")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Provider != "pixabay" {
		t.Fatalf("expected provider pixabay, got %s", results[0].Provider)
	}
	if results[0].Type != "image" {
		t.Fatalf("expected type image, got %s", results[0].Type)
	}
	if results[0].URL != "https://cdn.pixabay.com/photo-101.jpg" {
		t.Fatalf("unexpected image URL: %s", results[0].URL)
	}
	if results[0].Height != 1920 || results[0].Width != 1080 {
		t.Fatalf("unexpected dimensions: %dx%d", results[0].Width, results[0].Height)
	}
}

func TestPixabayProviderSearchVideo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/videos/" {
			t.Fatalf("expected /api/videos/, got %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"hits": [{
				"id": 202,
				"pageURL": "https://pixabay.com/videos/id-202/",
				"duration": 12,
				"user": "pixabay-user",
				"tags": "marketing, phone",
				"videos": {
					"tiny": {"url": "https://cdn.pixabay.com/video-tiny.mp4", "width": 360, "height": 640},
					"medium": {"url": "https://cdn.pixabay.com/video-medium.mp4", "width": 720, "height": 1280},
					"large": {"url": "https://cdn.pixabay.com/video-large.mp4", "width": 1080, "height": 1920}
				}
			}]
		}`))
	}))
	defer server.Close()

	provider := NewPixabayProvider("test-key")
	provider.baseURL = server.URL
	provider.httpClient = server.Client()

	results, err := provider.Search(context.Background(), "marketing phone", "video")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].URL != "https://cdn.pixabay.com/video-large.mp4" {
		t.Fatalf("unexpected video URL: %s", results[0].URL)
	}
	if results[0].DurationSec != 12 {
		t.Fatalf("unexpected duration: %v", results[0].DurationSec)
	}
	if results[0].Type != "video" {
		t.Fatalf("expected type video, got %s", results[0].Type)
	}
}
