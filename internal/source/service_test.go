package source

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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
	if result.TranscriptSource != "page_description" {
		t.Fatalf("expected page-description transcript source, got %#v", result)
	}
}

func TestFetchYouTubeUsesYtDlpTranscriptFallback(t *testing.T) {
	svc := New(&http.Client{
		Timeout: 5 * time.Second,
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case strings.Contains(req.URL.Host, "youtube.com") && strings.Contains(req.URL.Path, "/watch"):
				body := `<html><head><title>Sample Video</title><meta name="description" content="Fallback description"></head><body>No caption tracks here.</body></html>`
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			case req.URL.Host == "captions.local":
				body := `{"events":[{"segs":[{"utf8":"A real transcript line from yt-dlp."}]},{"segs":[{"utf8":"Second grounded fact."}]}]}`
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			default:
				return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("not found")), Header: make(http.Header)}, nil
			}
		}),
	})
	svc.commandRunner = commandRunnerFunc(func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "yt-dlp" {
			t.Fatalf("expected yt-dlp command, got %s", name)
		}
		return []byte(`{"title":"Sample Video","description":"Fallback description","automatic_captions":{"en":[{"ext":"json3","url":"https://captions.local/transcript.json3"}]}}`), nil
	})

	result, err := svc.Fetch(context.Background(), "https://www.youtube.com/watch?v=abc123")
	if err != nil {
		t.Fatalf("expected yt-dlp transcript fallback to succeed, got error: %v", err)
	}
	if result == nil || !strings.Contains(result.Transcript, "A real transcript line") {
		t.Fatalf("expected transcript from yt-dlp fallback, got %#v", result)
	}
	if result.TranscriptSource != "yt-dlp" {
		t.Fatalf("expected yt-dlp transcript source, got %#v", result)
	}
	if len(result.Facts) == 0 {
		t.Fatalf("expected structured grounding facts, got %#v", result)
	}
}

func TestBuildGroundingFactsProducesUsefulBullets(t *testing.T) {
	facts := buildGroundingFacts("Mars Mission", "Mars is the fourth planet from the Sun. Scientists study Mars for signs of ancient water. The rover collected rock samples for analysis. Scientists study Mars for signs of ancient water.", 4)
	if len(facts) < 3 {
		t.Fatalf("expected multiple grounded facts, got %#v", facts)
	}
	joined := strings.Join(facts, " | ")
	for _, want := range []string{"Mars Mission", "fourth planet", "rock samples"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected facts to contain %q, got %#v", want, facts)
		}
	}
}

func TestPreferredSubtitleURLsIncludesTranslatedEnglishTracks(t *testing.T) {
	urls := preferredSubtitleURLs(nil, map[string][]ytDLPSubtitleTrack{
		"ab-en": {{Ext: "json3", URL: "https://captions.local/auto-en.json3"}},
	})
	if len(urls) == 0 || urls[0] != "https://captions.local/auto-en.json3" {
		t.Fatalf("expected translated-English auto captions to be selected, got %#v", urls)
	}
}

func TestFetchYouTubeUsesDownloadedYtDlpSubtitlesWhenURLFetchFails(t *testing.T) {
	svc := New(&http.Client{
		Timeout: 5 * time.Second,
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.Host, "youtube.com") && strings.Contains(req.URL.Path, "/watch") {
				body := `<html><head><title>Sample Video</title><meta name="description" content="Fallback description"></head><body>No caption tracks here.</body></html>`
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			}
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("not found")), Header: make(http.Header)}, nil
		}),
	})
	svc.commandRunner = commandRunnerFunc(func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "yt-dlp" {
			t.Fatalf("expected yt-dlp command, got %s", name)
		}
		for i := 0; i < len(args)-1; i++ {
			if args[i] == "--output" || args[i] == "-o" {
				path := strings.ReplaceAll(args[i+1], "%(id)s", "abc123")
				path = strings.ReplaceAll(path, "%(ext)s", "en.json3")
				if err := os.WriteFile(path, []byte(`{"events":[{"segs":[{"utf8":"Downloaded transcript from yt-dlp."}]}]}`), 0o600); err != nil {
					t.Fatalf("write subtitle file: %v", err)
				}
				break
			}
		}
		return []byte(`{"automatic_captions":{}}`), nil
	})

	result, err := svc.Fetch(context.Background(), "https://www.youtube.com/watch?v=abc123")
	if err != nil {
		t.Fatalf("expected downloaded yt-dlp subtitle fallback to succeed, got error: %v", err)
	}
	if result == nil || !strings.Contains(result.Transcript, "Downloaded transcript from yt-dlp") {
		t.Fatalf("expected transcript from downloaded yt-dlp subtitle file, got %#v", result)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

type commandRunnerFunc func(context.Context, string, ...string) ([]byte, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
