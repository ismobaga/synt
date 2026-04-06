// Package source fetches user-supplied reference material like webpages and YouTube transcripts.
package source

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	htmlTitlePattern        = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	ogTitlePattern          = regexp.MustCompile(`(?is)<meta[^>]+property=["']og:title["'][^>]+content=["']([^"']+)["']`)
	metaDescriptionPattern  = regexp.MustCompile(`(?is)<meta[^>]+name=["']description["'][^>]+content=["']([^"']+)["']`)
	shortDescriptionPattern = regexp.MustCompile(`"shortDescription":"((?:\\.|[^"])*)"`)
	scriptStylePattern      = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>|<style[^>]*>.*?</style>|<noscript[^>]*>.*?</noscript>`)
	htmlTagPattern          = regexp.MustCompile(`(?s)<[^>]+>`)
	whitespacePattern       = regexp.MustCompile(`\s+`)
	captionTracksPattern    = regexp.MustCompile(`"captionTracks":(\[(?s:.*?)\])`)
)

// Result holds fetched reference content for a source URL.
type Result struct {
	URL        string    `json:"url"`
	Provider   string    `json:"provider"`
	Title      string    `json:"title,omitempty"`
	Content    string    `json:"content,omitempty"`
	Transcript string    `json:"transcript,omitempty"`
	FetchedAt  time.Time `json:"fetched_at"`
}

// Service fetches webpage content and YouTube transcripts.
type Service struct {
	client *http.Client
}

// New creates a new source fetching service.
func New(client *http.Client) *Service {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &Service{client: client}
}

// Fetch loads reference content from either a webpage or YouTube video.
func (s *Service) Fetch(ctx context.Context, rawURL string) (*Result, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil, fmt.Errorf("source url is empty")
	}
	if isYouTubeURL(trimmed) {
		return s.fetchYouTube(ctx, trimmed)
	}
	return s.fetchWebpage(ctx, trimmed)
}

func (s *Service) fetchWebpage(ctx context.Context, rawURL string) (*Result, error) {
	body, err := s.fetchText(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	title := extractHTMLTitle(body)
	content := excerptText(extractReadableText(body), 8000)
	if content == "" {
		return nil, fmt.Errorf("webpage content was empty")
	}
	return &Result{
		URL:       rawURL,
		Provider:  "webpage",
		Title:     title,
		Content:   content,
		FetchedAt: time.Now().UTC(),
	}, nil
}

func (s *Service) fetchYouTube(ctx context.Context, rawURL string) (*Result, error) {
	videoID := extractYouTubeVideoID(rawURL)
	if videoID == "" {
		return nil, fmt.Errorf("could not determine youtube video id")
	}
	watchURL := "https://www.youtube.com/watch?v=" + videoID + "&hl=en"
	body, err := s.fetchText(ctx, watchURL)
	if err != nil {
		return nil, err
	}
	title := extractHTMLTitle(body)
	transcript, transcriptErr := s.fetchYouTubeTranscript(ctx, body)
	if transcript != "" {
		return &Result{
			URL:        rawURL,
			Provider:   "youtube",
			Title:      title,
			Content:    transcript,
			Transcript: transcript,
			FetchedAt:  time.Now().UTC(),
		}, nil
	}

	fallbackContent := excerptText(extractYouTubeFallbackContent(body), 4000)
	if fallbackContent != "" {
		return &Result{
			URL:       rawURL,
			Provider:  "youtube",
			Title:     title,
			Content:   fallbackContent,
			FetchedAt: time.Now().UTC(),
		}, nil
	}
	if transcriptErr != nil {
		return nil, transcriptErr
	}
	return nil, fmt.Errorf("youtube transcript was empty")
}

func (s *Service) fetchText(ctx context.Context, rawURL string) (string, error) {
	body, err := s.doFetchText(ctx, s.client, rawURL)
	if err == nil {
		return body, nil
	}
	if shouldRetryWithoutTLSVerify(rawURL, err) {
		insecureBody, insecureErr := s.doFetchText(ctx, insecureClientFrom(s.client), rawURL)
		if insecureErr == nil {
			return insecureBody, nil
		}
		return "", fmt.Errorf("fetch source: %v (after insecure TLS retry: %w)", err, insecureErr)
	}
	return "", fmt.Errorf("fetch source: %w", err)
}

func (s *Service) doFetchText(ctx context.Context, client *http.Client, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("build source request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; synt-source-fetcher/1.0; +https://github.com/ismobaga/synt)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", fmt.Errorf("read source response: %w", err)
	}
	if resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("source fetch returned %s", resp.Status)
	}
	return string(body), nil
}

func shouldRetryWithoutTLSVerify(rawURL string, err error) bool {
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(rawURL)), "https://") || err == nil {
		return false
	}
	var unknownAuthErr x509.UnknownAuthorityError
	if errors.As(err, &unknownAuthErr) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "x509:") || strings.Contains(message, "certificate signed by unknown authority")
}

func insecureClientFrom(base *http.Client) *http.Client {
	if base == nil {
		base = &http.Client{Timeout: 20 * time.Second}
	}
	transport, ok := base.Transport.(*http.Transport)
	if !ok || transport == nil {
		if defaultTransport, ok := http.DefaultTransport.(*http.Transport); ok {
			transport = defaultTransport.Clone()
		} else {
			transport = &http.Transport{}
		}
	} else {
		transport = transport.Clone()
	}
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	} else {
		transport.TLSClientConfig = transport.TLSClientConfig.Clone()
		transport.TLSClientConfig.InsecureSkipVerify = true
	}
	return &http.Client{Timeout: base.Timeout, Transport: transport}
}

func isYouTubeURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Host)
	return strings.Contains(host, "youtube.com") || strings.Contains(host, "youtu.be")
}

func extractYouTubeVideoID(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	host := strings.ToLower(parsed.Host)
	switch {
	case strings.Contains(host, "youtu.be"):
		return strings.Trim(strings.TrimSpace(parsed.Path), "/")
	case strings.Contains(host, "youtube.com"):
		if id := strings.TrimSpace(parsed.Query().Get("v")); id != "" {
			return id
		}
		parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		for i := 0; i < len(parts)-1; i++ {
			if parts[i] == "shorts" || parts[i] == "embed" || parts[i] == "v" {
				return parts[i+1]
			}
		}
	}
	return ""
}

func extractHTMLTitle(body string) string {
	if match := ogTitlePattern.FindStringSubmatch(body); len(match) > 1 {
		return excerptText(html.UnescapeString(match[1]), 180)
	}
	if match := htmlTitlePattern.FindStringSubmatch(body); len(match) > 1 {
		return excerptText(html.UnescapeString(match[1]), 180)
	}
	return ""
}

func extractMetaDescription(body string) string {
	if match := metaDescriptionPattern.FindStringSubmatch(body); len(match) > 1 {
		return excerptText(html.UnescapeString(match[1]), 2000)
	}
	if match := shortDescriptionPattern.FindStringSubmatch(body); len(match) > 1 {
		if value, err := strconv.Unquote(`"` + match[1] + `"`); err == nil {
			return excerptText(html.UnescapeString(value), 2000)
		}
	}
	return ""
}

func extractYouTubeFallbackContent(body string) string {
	parts := make([]string, 0, 2)
	if title := extractHTMLTitle(body); title != "" {
		parts = append(parts, "Video title: "+title)
	}
	if desc := extractMetaDescription(body); desc != "" {
		parts = append(parts, desc)
	}
	joined := strings.TrimSpace(strings.Join(parts, "\n\n"))
	if joined != "" {
		return joined
	}
	return extractReadableText(body)
}

func extractReadableText(body string) string {
	withoutScripts := scriptStylePattern.ReplaceAllString(body, " ")
	text := htmlTagPattern.ReplaceAllString(withoutScripts, " ")
	text = html.UnescapeString(text)
	text = whitespacePattern.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

type youtubeCaptionTrack struct {
	BaseURL      string `json:"baseUrl"`
	LanguageCode string `json:"languageCode"`
}

func extractYouTubeCaptionTrackURL(body string) string {
	match := captionTracksPattern.FindStringSubmatch(body)
	if len(match) < 2 {
		return ""
	}

	raw := strings.ReplaceAll(match[1], `\u0026`, "&")
	raw = strings.ReplaceAll(raw, `\/`, "/")

	var tracks []youtubeCaptionTrack
	if err := json.Unmarshal([]byte(raw), &tracks); err != nil || len(tracks) == 0 {
		return ""
	}

	selected := tracks[0]
	for _, track := range tracks {
		if strings.HasPrefix(strings.ToLower(track.LanguageCode), "en") {
			selected = track
			break
		}
	}

	baseURL := html.UnescapeString(strings.TrimSpace(selected.BaseURL))
	if baseURL == "" {
		return ""
	}
	if !strings.Contains(baseURL, "fmt=") {
		if strings.Contains(baseURL, "?") {
			baseURL += "&fmt=srv3"
		} else {
			baseURL += "?fmt=srv3"
		}
	}
	return baseURL
}

type transcriptDocument struct {
	Texts []struct {
		Text string `xml:",chardata"`
	} `xml:"text"`
}

type jsonTranscriptDocument struct {
	Events []struct {
		Segs []struct {
			Text string `json:"utf8"`
		} `json:"segs"`
	} `json:"events"`
}

func (s *Service) fetchYouTubeTranscript(ctx context.Context, watchPage string) (string, error) {
	captionURL := extractYouTubeCaptionTrackURL(watchPage)
	if captionURL == "" {
		return "", fmt.Errorf("youtube transcript track unavailable")
	}
	variants := []string{captionURL}
	if strings.Contains(captionURL, "fmt=") {
		variants = append(variants,
			strings.Replace(captionURL, "fmt=srv3", "fmt=json3", 1),
			strings.Replace(captionURL, "fmt=srv3", "fmt=vtt", 1),
		)
	} else if strings.Contains(captionURL, "?") {
		variants = append(variants, captionURL+"&fmt=json3", captionURL+"&fmt=vtt")
	} else {
		variants = append(variants, captionURL+"?fmt=json3", captionURL+"?fmt=vtt")
	}

	seen := map[string]bool{}
	var lastErr error
	for _, transcriptURL := range variants {
		transcriptURL = strings.TrimSpace(transcriptURL)
		if transcriptURL == "" || seen[transcriptURL] {
			continue
		}
		seen[transcriptURL] = true
		transcriptBody, err := s.fetchText(ctx, transcriptURL)
		if err != nil {
			lastErr = err
			continue
		}
		transcript := excerptText(extractYouTubeTranscriptText(transcriptBody), 12000)
		if transcript != "" {
			return transcript, nil
		}
		lastErr = fmt.Errorf("youtube transcript was empty")
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("youtube transcript was empty")
}

func extractYouTubeTranscriptText(body string) string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return ""
	}

	if strings.HasPrefix(trimmed, "{") {
		var transcript jsonTranscriptDocument
		if err := json.Unmarshal([]byte(trimmed), &transcript); err == nil {
			parts := make([]string, 0, len(transcript.Events))
			for _, event := range transcript.Events {
				for _, seg := range event.Segs {
					text := strings.TrimSpace(html.UnescapeString(seg.Text))
					if text != "" {
						parts = append(parts, text)
					}
				}
			}
			if len(parts) > 0 {
				return strings.Join(parts, " ")
			}
		}
	}

	if strings.HasPrefix(trimmed, "WEBVTT") {
		parts := make([]string, 0)
		for _, line := range strings.Split(trimmed, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "WEBVTT") || strings.Contains(line, "-->") || strings.HasPrefix(line, "NOTE") {
				continue
			}
			parts = append(parts, html.UnescapeString(line))
		}
		if len(parts) > 0 {
			return strings.Join(parts, " ")
		}
	}

	var transcript transcriptDocument
	if err := xml.Unmarshal([]byte(trimmed), &transcript); err != nil {
		return ""
	}
	parts := make([]string, 0, len(transcript.Texts))
	for _, item := range transcript.Texts {
		text := strings.TrimSpace(html.UnescapeString(item.Text))
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, " ")
}

func excerptText(value string, maxLen int) string {
	trimmed := whitespacePattern.ReplaceAllString(strings.TrimSpace(value), " ")
	if trimmed == "" || maxLen <= 0 {
		return trimmed
	}
	runes := []rune(trimmed)
	if len(runes) <= maxLen {
		return trimmed
	}
	return string(runes[:maxLen]) + "…"
}
