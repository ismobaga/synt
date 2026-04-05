package media

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const defaultPixabayBaseURL = "https://pixabay.com"

// PixabayProvider searches Pixabay for stock images and videos.
type PixabayProvider struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

type pixabayImageResponse struct {
	Hits []pixabayImageHit `json:"hits"`
}

type pixabayImageHit struct {
	ID            int    `json:"id"`
	PageURL       string `json:"pageURL"`
	LargeImageURL string `json:"largeImageURL"`
	WebformatURL  string `json:"webformatURL"`
	ImageWidth    int    `json:"imageWidth"`
	ImageHeight   int    `json:"imageHeight"`
	User          string `json:"user"`
	Tags          string `json:"tags"`
}

type pixabayVideoResponse struct {
	Hits []pixabayVideoHit `json:"hits"`
}

type pixabayVideoHit struct {
	ID       int               `json:"id"`
	PageURL  string            `json:"pageURL"`
	Duration int               `json:"duration"`
	User     string            `json:"user"`
	Tags     string            `json:"tags"`
	Videos   pixabayVideoFiles `json:"videos"`
}

type pixabayVideoFiles struct {
	Large  *pixabayVideoFile `json:"large"`
	Medium *pixabayVideoFile `json:"medium"`
	Small  *pixabayVideoFile `json:"small"`
	Tiny   *pixabayVideoFile `json:"tiny"`
}

type pixabayVideoFile struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Size   int    `json:"size"`
}

// NewPixabayProvider creates a Pixabay media provider.
func NewPixabayProvider(apiKey string) *PixabayProvider {
	return &PixabayProvider{
		apiKey:  strings.TrimSpace(apiKey),
		baseURL: defaultPixabayBaseURL,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

// NewPixabayProviderFromEnv creates a Pixabay provider when PIXABAY_API_KEY is configured.
func NewPixabayProviderFromEnv() *PixabayProvider {
	apiKey := strings.TrimSpace(os.Getenv("PIXABAY_API_KEY"))
	if apiKey == "" {
		return nil
	}
	provider := NewPixabayProvider(apiKey)
	if baseURL := strings.TrimSpace(os.Getenv("PIXABAY_BASE_URL")); baseURL != "" {
		provider.baseURL = strings.TrimRight(baseURL, "/")
	}
	return provider
}

// Search returns Pixabay candidates for the requested asset type.
func (p *PixabayProvider) Search(ctx context.Context, query string, assetType string) ([]*Candidate, error) {
	if p == nil || p.apiKey == "" {
		return nil, fmt.Errorf("pixabay api key is not configured")
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}

	switch strings.ToLower(assetType) {
	case "image":
		return p.searchImages(ctx, query)
	case "video":
		return p.searchVideos(ctx, query)
	default:
		return nil, fmt.Errorf("unsupported asset type %q", assetType)
	}
}

func (p *PixabayProvider) searchImages(ctx context.Context, query string) ([]*Candidate, error) {
	var response pixabayImageResponse
	if err := p.doRequest(ctx, "/api/", query, &response); err != nil {
		return nil, err
	}

	results := make([]*Candidate, 0, len(response.Hits))
	for _, hit := range response.Hits {
		imageURL := firstNonEmpty(hit.LargeImageURL, hit.WebformatURL)
		if imageURL == "" {
			continue
		}
		results = append(results, &Candidate{
			URL:      imageURL,
			Provider: "pixabay",
			Type:     "image",
			Width:    hit.ImageWidth,
			Height:   hit.ImageHeight,
			LicenseInfo: map[string]any{
				"license":     "Pixabay Content License",
				"page_url":    hit.PageURL,
				"author":      hit.User,
				"tags":        hit.Tags,
				"provider_id": hit.ID,
			},
		})
	}
	return results, nil
}

func (p *PixabayProvider) searchVideos(ctx context.Context, query string) ([]*Candidate, error) {
	var response pixabayVideoResponse
	if err := p.doRequest(ctx, "/api/videos/", query, &response); err != nil {
		return nil, err
	}

	results := make([]*Candidate, 0, len(response.Hits))
	for _, hit := range response.Hits {
		file := choosePixabayVideoFile(hit.Videos)
		if file == nil || file.URL == "" {
			continue
		}
		results = append(results, &Candidate{
			URL:         file.URL,
			Provider:    "pixabay",
			Type:        "video",
			Width:       file.Width,
			Height:      file.Height,
			DurationSec: float64(hit.Duration),
			LicenseInfo: map[string]any{
				"license":     "Pixabay Content License",
				"page_url":    hit.PageURL,
				"author":      hit.User,
				"tags":        hit.Tags,
				"provider_id": hit.ID,
			},
		})
	}
	return results, nil
}

func (p *PixabayProvider) doRequest(ctx context.Context, path string, query string, target any) error {
	endpoint, err := url.Parse(strings.TrimRight(p.baseURL, "/") + path)
	if err != nil {
		return fmt.Errorf("build pixabay url: %w", err)
	}

	params := endpoint.Query()
	params.Set("key", p.apiKey)
	params.Set("q", query)
	params.Set("safesearch", "true")
	params.Set("per_page", "10")
	params.Set("order", "popular")
	endpoint.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return fmt.Errorf("create pixabay request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("pixabay request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read pixabay response: %w", err)
	}
	if resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("pixabay returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode pixabay response: %w", err)
	}
	return nil
}

func choosePixabayVideoFile(files pixabayVideoFiles) *pixabayVideoFile {
	candidates := []*pixabayVideoFile{files.Large, files.Medium, files.Small, files.Tiny}
	var best *pixabayVideoFile
	for _, file := range candidates {
		if file == nil || file.URL == "" {
			continue
		}
		if best == nil || file.Width*file.Height > best.Width*best.Height {
			best = file
		}
	}
	return best
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
