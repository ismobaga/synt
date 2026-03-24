package api

import "time"

// CreateProjectRequest is the request body for POST /v1/projects.
type CreateProjectRequest struct {
	Topic      string `json:"topic"`
	Language   string `json:"language"`
	Platform   string `json:"platform"`
	DurationSec int   `json:"duration_sec"`
	Tone       string `json:"tone"`
	TemplateID string `json:"template_id"`
	BrandKitID string `json:"brand_kit_id,omitempty"`
}

// CreateProjectResponse is returned after creating a project.
type CreateProjectResponse struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	CurrentStage string `json:"current_stage"`
}

// GenerateRequest is the request body for POST /v1/projects/:id/generate.
type GenerateRequest struct {
	AutoRender bool `json:"auto_render"`
}

// GenerateResponse is returned when generation is triggered.
type GenerateResponse struct {
	Status       string `json:"status"`
	CurrentStage string `json:"current_stage"`
}

// ProjectStatusResponse holds project status details.
type ProjectStatusResponse struct {
	ID           string    `json:"id"`
	Status       string    `json:"status"`
	CurrentStage string    `json:"current_stage"`
	ErrorMessage string    `json:"error_message,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// SceneContent describes one scene in the generated script.
type SceneContent struct {
	Index       int    `json:"index"`
	DurationSec int    `json:"duration_sec"`
	Narration   string `json:"narration"`
	Caption     string `json:"caption"`
	VisualQuery string `json:"visual_query"`
	OverlayStyle string `json:"overlay_style"`
}

// ScriptContent is the structured JSON output from the content service.
type ScriptContent struct {
	Title       string         `json:"title"`
	Hook        string         `json:"hook"`
	DurationSec int            `json:"duration_sec"`
	Language    string         `json:"language"`
	CTA         string         `json:"cta"`
	MusicMood   string         `json:"music_mood"`
	Scenes      []SceneContent `json:"scenes"`
}

// UpdateScriptRequest is the request body for PUT /v1/projects/:id/script.
type UpdateScriptRequest struct {
	Content ScriptContent `json:"content"`
}

// ErrorResponse is a standard error envelope.
type ErrorResponse struct {
	Error string `json:"error"`
}

// CreateBrandKitRequest is the request body for POST /v1/brand-kits.
type CreateBrandKitRequest struct {
	Name           string `json:"name"`
	LogoPath       string `json:"logo_path,omitempty"`
	PrimaryColor   string `json:"primary_color,omitempty"`
	SecondaryColor string `json:"secondary_color,omitempty"`
	FontFamily     string `json:"font_family,omitempty"`
	OutroText      string `json:"outro_text,omitempty"`
}

// RenderRequest is the request body for render endpoints.
type RenderRequest struct {
	Kind string `json:"kind"` // "preview" or "final"
}

// TimelineManifest is the JSON manifest for the render engine.
type TimelineManifest struct {
	ProjectID   string               `json:"project_id"`
	Resolution  ManifestResolution   `json:"resolution"`
	FPS         int                  `json:"fps"`
	DurationSec float64              `json:"duration_sec"`
	Template    string               `json:"template"`
	Scenes      []ManifestScene      `json:"scenes"`
	Music       ManifestMusic        `json:"music"`
	Branding    ManifestBranding     `json:"branding"`
}

// ManifestResolution holds video resolution.
type ManifestResolution struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// ManifestScene describes a single scene in the render manifest.
type ManifestScene struct {
	Index      int                  `json:"index"`
	StartSec   float64              `json:"start_sec"`
	EndSec     float64              `json:"end_sec"`
	Media      ManifestMedia        `json:"media"`
	Voiceover  ManifestVoiceover    `json:"voiceover"`
	Captions   []ManifestCaption    `json:"captions"`
	TextOverlays []ManifestTextOverlay `json:"text_overlays"`
	TransitionOut string            `json:"transition_out"`
}

// ManifestMedia holds media reference for a scene.
type ManifestMedia struct {
	Type    string `json:"type"`
	Path    string `json:"path"`
	FitMode string `json:"fit_mode"`
}

// ManifestVoiceover holds voiceover reference for a scene.
type ManifestVoiceover struct {
	Path     string  `json:"path"`
	StartSec float64 `json:"start_sec"`
}

// ManifestCaption holds a single caption entry.
type ManifestCaption struct {
	StartSec float64 `json:"start_sec"`
	EndSec   float64 `json:"end_sec"`
	Text     string  `json:"text"`
}

// ManifestTextOverlay holds a text overlay entry.
type ManifestTextOverlay struct {
	Text     string `json:"text"`
	Position string `json:"position"`
	Style    string `json:"style"`
}

// ManifestMusic holds the music track configuration.
type ManifestMusic struct {
	Path       string  `json:"path"`
	Volume     float64 `json:"volume"`
	FadeInSec  float64 `json:"fade_in_sec"`
	FadeOutSec float64 `json:"fade_out_sec"`
}

// ManifestBranding holds branding configuration.
type ManifestBranding struct {
	LogoPath  string `json:"logo_path"`
	Watermark string `json:"watermark"`
}

// Supported platforms.
const (
	PlatformTikTok        = "tiktok"
	PlatformInstagramReels = "instagram_reels"
	PlatformYouTubeShorts = "youtube_shorts"
)

// Supported tones.
const (
	ToneEducational = "educational"
	ToneEntertaining = "entertaining"
	ToneInspirational = "inspirational"
	TonePromotional = "promotional"
	ToneProfessional = "professional"
	ToneCasual = "casual"
)
