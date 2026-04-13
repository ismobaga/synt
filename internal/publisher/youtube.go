package publisher
package publisher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultYouTubeUploadURL = "https://www.googleapis.com/upload/youtube/v3/videos"
	defaultPrivacyStatus    = "private"
)

// Service publishes rendered videos to distribution channels.
type Service struct {
	youtubeUploadURL string
	youtubeToken     string
	httpClient       *http.Client
}

// YouTubePublishRequest describes a YouTube publish operation.
type YouTubePublishRequest struct {
	VideoPath     string
	Title         string
	Description   string
	PrivacyStatus string
	Tags          []string
}

// YouTubePublishResult is returned after a successful publish.
type YouTubePublishResult struct {
	VideoID     string
	WatchURL    string
	PublishedAt time.Time
	RawResponse []byte
}

// NewFromEnv creates a publisher service from environment variables.
func NewFromEnv() *Service {
	return &Service{
		youtubeUploadURL: strings.TrimSpace(envOrDefault("YOUTUBE_UPLOAD_URL", defaultYouTubeUploadURL)),
		youtubeToken:     strings.TrimSpace(os.Getenv("YOUTUBE_ACCESS_TOKEN")),
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// YouTubeConfigured reports whether YouTube publishing is configured.
func (s *Service) YouTubeConfigured() bool {
	return s != nil && s.youtubeToken != ""
}

// PublishToYouTube uploads a video to YouTube and returns the published video URL.
func (s *Service) PublishToYouTube(ctx context.Context, req YouTubePublishRequest) (*YouTubePublishResult, error) {
	if s == nil {
		return nil, fmt.Errorf("publisher service is not configured")
	}
	if strings.TrimSpace(s.youtubeToken) == "" {
		return nil, fmt.Errorf("youtube access token is not configured")
	}

	videoPath := strings.TrimSpace(req.VideoPath)
	if videoPath == "" {
		return nil, fmt.Errorf("video path is required")
	}

	localPath, cleanup, err := materializeLocalVideo(ctx, videoPath)
	if err != nil {
		return nil, err
	}
	if cleanup != nil {
		defer cleanup()
	}

	file, err := os.Open(localPath)
	if err != nil {
		return nil, fmt.Errorf("open video file: %w", err)
	}
	defer file.Close()

	metadata := map[string]any{
		"snippet": map[string]any{
			"title":       firstNonEmpty(strings.TrimSpace(req.Title), "Synt Generated Video"),
			"description": strings.TrimSpace(req.Description),
			"tags":        req.Tags,
			"categoryId":  "22",
		},
		"status": map[string]any{
			"privacyStatus": firstNonEmpty(strings.TrimSpace(req.PrivacyStatus), defaultPrivacyStatus),
		},
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal youtube metadata: %w", err)
	}

	requestBody := &bytes.Buffer{}
	writer := multipart.NewWriter(requestBody)

	jsonHeader := make(textproto.MIMEHeader)
	jsonHeader.Set("Content-Type", "application/json; charset=UTF-8")
	jsonPart, err := writer.CreatePart(jsonHeader)
	if err != nil {
		return nil, fmt.Errorf("create metadata part: %w", err)
	}
	if _, err := jsonPart.Write(metadataJSON); err != nil {
		return nil, fmt.Errorf("write metadata part: %w", err)
	}

	videoHeader := make(textproto.MIMEHeader)
	videoHeader.Set("Content-Type", "video/mp4")
	videoPart, err := writer.CreatePart(videoHeader)
	if err != nil {
		return nil, fmt.Errorf("create video part: %w", err)
	}
	if _, err := io.Copy(videoPart, file); err != nil {
		return nil, fmt.Errorf("write video part: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("finalize multipart body: %w", err)
	}

	uploadURL := fmt.Sprintf("%s?uploadType=multipart&part=snippet,status", strings.TrimRight(s.youtubeUploadURL, "/"))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, requestBody)
	if err != nil {
		return nil, fmt.Errorf("build upload request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+s.youtubeToken)
	httpReq.Header.Set("Content-Type", "multipart/related; boundary="+writer.Boundary())
	httpReq.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("upload to youtube: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("read youtube response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("youtube publish failed: %s: %s", resp.Status, strings.TrimSpace(string(responseBody)))
	}

	var response struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("decode youtube response: %w", err)
	}
	if strings.TrimSpace(response.ID) == "" {
		return nil, fmt.Errorf("youtube response did not include video id")
	}

	publishedAt := time.Now().UTC()
	return &YouTubePublishResult{
		VideoID:     response.ID,
		WatchURL:    "https://www.youtube.com/watch?v=" + response.ID,
		PublishedAt: publishedAt,
		RawResponse: responseBody,
	}, nil
}

func materializeLocalVideo(ctx context.Context, videoPath string) (string, func(), error) {
	if strings.HasPrefix(videoPath, "http://") || strings.HasPrefix(videoPath, "https://") {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, videoPath, nil)
		if err != nil {
			return "", nil, fmt.Errorf("build download request: %w", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", nil, fmt.Errorf("download remote video: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			return "", nil, fmt.Errorf("download remote video failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
		}

		tmpFile, err := os.CreateTemp("", "synt-youtube-*.mp4")
		if err != nil {
			return "", nil, fmt.Errorf("create temp video file: %w", err)
		}
		if _, err := io.Copy(tmpFile, resp.Body); err != nil {
			tmpFile.Close()
			_ = os.Remove(tmpFile.Name())
			return "", nil, fmt.Errorf("write temp video file: %w", err)
		}
		if err := tmpFile.Close(); err != nil {
			_ = os.Remove(tmpFile.Name())
			return "", nil, fmt.Errorf("close temp video file: %w", err)
		}
		cleanup := func() { _ = os.Remove(tmpFile.Name()) }
		return tmpFile.Name(), cleanup, nil
	}

	resolved := videoPath
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Clean(resolved)
	}
	if _, err := os.Stat(resolved); err != nil {
		return "", nil, fmt.Errorf("video file unavailable at %s: %w", resolved, err)
	}
	return resolved, nil, nil
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
