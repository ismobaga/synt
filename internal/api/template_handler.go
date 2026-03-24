package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/ismobaga/synt/internal/db"
)

// TemplateHandler handles template-related endpoints.
type TemplateHandler struct {
	db *db.DB
}

// NewTemplateHandler creates a new TemplateHandler.
func NewTemplateHandler(database *db.DB) *TemplateHandler {
	return &TemplateHandler{db: database}
}

// List handles GET /v1/templates.
func (h *TemplateHandler) List(w http.ResponseWriter, r *http.Request) {
	templates, err := h.db.ListTemplates(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list templates")
		return
	}
	writeJSON(w, http.StatusOK, templates)
}

// BrandKitHandler handles brand kit endpoints.
type BrandKitHandler struct {
	db *db.DB
}

// NewBrandKitHandler creates a new BrandKitHandler.
func NewBrandKitHandler(database *db.DB) *BrandKitHandler {
	return &BrandKitHandler{db: database}
}

// List handles GET /v1/brand-kits.
func (h *BrandKitHandler) List(w http.ResponseWriter, r *http.Request) {
	kits, err := h.db.ListBrandKits(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list brand kits")
		return
	}
	writeJSON(w, http.StatusOK, kits)
}

// Create handles POST /v1/brand-kits.
func (h *BrandKitHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateBrandKitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	kit := &db.BrandKit{
		ID:             uuid.New(),
		UserID:         uuid.New(), // placeholder until auth
		Name:           req.Name,
		LogoPath:       req.LogoPath,
		PrimaryColor:   req.PrimaryColor,
		SecondaryColor: req.SecondaryColor,
		FontFamily:     req.FontFamily,
		OutroText:      req.OutroText,
		CreatedAt:      time.Now().UTC(),
	}
	if err := h.db.CreateBrandKit(r.Context(), kit); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create brand kit")
		return
	}
	writeJSON(w, http.StatusCreated, kit)
}
