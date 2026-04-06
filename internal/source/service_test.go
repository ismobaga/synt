package source

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestExtractReadableTextStripsMarkup(t *testing.T) {
	html := `
		<html>
		  <head>
		    <title>AI Article</title>
		    <style>.hidden { display:none; }</style>
		  </head>
		  <body>
		    <article>
		      <h1>Three AI tools</h1>
		      <p>These save time for small teams.</p>
		    </article>
		    <script>window.ignore = true;</script>
		  </body>
		</html>`

	if title := extractHTMLTitle(html); title != "AI Article" {
		t.Fatalf("expected title %q, got %q", "AI Article", title)
	}
	text := extractReadableText(html)
	for _, want := range []string{"Three AI tools", "These save time for small teams."} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected text to contain %q, got %q", want, text)
		}
	}
	if strings.Contains(text, "window.ignore") {
		t.Fatalf("expected scripts to be stripped, got %q", text)
	}
}

func TestExtractYouTubeCaptionTrackURL(t *testing.T) {
	page := `{"captions":{"playerCaptionsTracklistRenderer":{"captionTracks":[{"baseUrl":"https://www.youtube.com/api/timedtext?v=abc123\u0026lang=en","languageCode":"en"},{"baseUrl":"https://www.youtube.com/api/timedtext?v=abc123\u0026lang=fr","languageCode":"fr"}]}}}`
	got := extractYouTubeCaptionTrackURL(page)
	if !strings.Contains(got, "lang=en") {
		t.Fatalf("expected english caption URL, got %q", got)
	}
	if !strings.Contains(got, "fmt=") {
		t.Fatalf("expected caption URL to request transcript format, got %q", got)
	}
}

func TestExtractYouTubeTranscriptText(t *testing.T) {
	body := `<?xml version="1.0" encoding="utf-8" ?><transcript><text start="0" dur="1.2">Hello &amp; welcome</text><text start="1.2" dur="1.0">to the video.</text></transcript>`
	got := extractYouTubeTranscriptText(body)
	if got != "Hello & welcome to the video." {
		t.Fatalf("unexpected transcript text: %q", got)
	}
}

func TestFetchWebpageRetriesOnTLSVerificationFailure(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `<html><head><title>Secure Source</title></head><body><article><p>Trusted fallback article content.</p></article></body></html>`)
	}))
	defer server.Close()

	svc := New(&http.Client{Timeout: 5 * time.Second})
	result, err := svc.Fetch(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("expected TLS fallback to succeed, got error: %v", err)
	}
	if result == nil || !strings.Contains(result.Content, "Trusted fallback article content") {
		t.Fatalf("expected fetched webpage content after TLS fallback, got %#v", result)
	}
}

func TestFetchYouTubeFallsBackToPageContentWhenTranscriptUnavailable(t *testing.T) {
	svc := New(&http.Client{
		Timeout: 5 * time.Second,
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case strings.Contains(req.URL.Host, "youtube.com") && strings.Contains(req.URL.Path, "/watch"):
				body := `<html><head><title>Sample Video</title><meta name="description" content="This description helps ground the script when captions are unavailable."></head><body>{"captions":{"playerCaptionsTracklistRenderer":{"captionTracks":[{"baseUrl":"https:\/\/www.youtube.com\/api\/timedtext?v=abc123\u0026lang=en","languageCode":"en"}]}}}</body></html>`
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			case strings.Contains(req.URL.Path, "/api/timedtext"):
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
			default:
				return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("not found")), Header: make(http.Header)}, nil
			}
		}),
	})

	result, err := svc.Fetch(context.Background(), "https://www.youtube.com/watch?v=abc123")
	if err != nil {
		t.Fatalf("expected YouTube source fallback to succeed, got error: %v", err)
	}
	if result == nil || !strings.Contains(result.Content, "This description helps ground the script") {
		t.Fatalf("expected fallback page content to be preserved, got %#v", result)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
