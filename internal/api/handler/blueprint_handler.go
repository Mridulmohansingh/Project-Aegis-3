// Package handler provides HTTP handlers for the Blueprint Management API.
package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/aegis-platform/aegis/internal/domain/blueprint"
	"github.com/aegis-platform/aegis/pkg/apperrors"
	"github.com/aegis-platform/aegis/pkg/httputil"
	"github.com/aegis-platform/aegis/pkg/logging"
	"github.com/aegis-platform/aegis/pkg/middleware"
)

// BlueprintService defines the business operations for blueprints.
type BlueprintService interface {
	CreateBlueprint(ctx interface{}, req CreateBlueprintRequest) (*blueprint.Blueprint, error)
	GetBlueprint(ctx interface{}, orgID, id uuid.UUID) (*blueprint.Blueprint, error)
	UpdateBlueprint(ctx interface{}, orgID, id uuid.UUID, req UpdateBlueprintRequest) (*blueprint.Blueprint, error)
	ListBlueprints(ctx interface{}, orgID uuid.UUID, cursor string, limit int) ([]*blueprint.Blueprint, string, error)
	ValidateBlueprint(ctx interface{}, orgID, id uuid.UUID) (*BlueprintValidationResult, error)
}

// BlueprintHandler handles HTTP requests for the Blueprint API.
type BlueprintHandler struct {
	service BlueprintService
	logger  *zap.Logger
}

// NewBlueprintHandler creates a new BlueprintHandler.
func NewBlueprintHandler(service BlueprintService, logger *zap.Logger) *BlueprintHandler {
	return &BlueprintHandler{
		service: service,
		logger:  logger.With(zap.String("component", "blueprint_handler")),
	}
}

// ──────────────────────────────────────────────
//  DTOs
// ──────────────────────────────────────────────

// CreateBlueprintRequest defines the API request for creating a blueprint.
type CreateBlueprintRequest struct {
	Name       string                  `json:"name" validate:"required"`
	SubjectID  string                  `json:"subject_id" validate:"required,uuid"`
	TotalItems int                     `json:"total_items" validate:"required,min=1"`
	Constraints BlueprintConstraintDTO `json:"constraints" validate:"required"`
}

// UpdateBlueprintRequest defines the API request for updating a blueprint.
type UpdateBlueprintRequest struct {
	Name        *string                  `json:"name,omitempty"`
	TotalItems  *int                     `json:"total_items,omitempty"`
	Constraints *BlueprintConstraintDTO  `json:"constraints,omitempty"`
	Status      *string                  `json:"status,omitempty"`
}

// BlueprintConstraintDTO represents the constraint specification in the API layer.
type BlueprintConstraintDTO struct {
	ChapterConstraints    []ChapterConstraintDTO    `json:"chapter_constraints"`
	DifficultyConstraints []DifficultyConstraintDTO `json:"difficulty_constraints"`
	CognitiveConstraints  []CognitiveConstraintDTO  `json:"cognitive_constraints"`
	MaxTimeSecs           int                       `json:"max_time_secs"`
	InformationTargets    []InformationTargetDTO    `json:"information_targets,omitempty"`
	AnswerKeyBalance      *AnswerKeyBalanceDTO      `json:"answer_key_balance,omitempty"`
}

// ChapterConstraintDTO specifies chapter coverage requirements.
type ChapterConstraintDTO struct {
	ChapterID string `json:"chapter_id"`
	MinItems  int    `json:"min_items"`
	MaxItems  int    `json:"max_items"`
}

// DifficultyConstraintDTO specifies difficulty level distribution.
type DifficultyConstraintDTO struct {
	Level    string `json:"level"` // EASY, MEDIUM, HARD, VERY_HARD
	MinItems int    `json:"min_items"`
	MaxItems int    `json:"max_items"`
}

// CognitiveConstraintDTO specifies cognitive level distribution.
type CognitiveConstraintDTO struct {
	Level    string `json:"level"` // REMEMBER, UNDERSTAND, APPLY, ANALYZE, EVALUATE, CREATE
	MinItems int    `json:"min_items"`
	MaxItems int    `json:"max_items"`
}

// InformationTargetDTO specifies target TIF at a theta point.
type InformationTargetDTO struct {
	Theta  float64 `json:"theta"`
	Target float64 `json:"target"`
	Weight float64 `json:"weight"`
}

// AnswerKeyBalanceDTO specifies answer key distribution constraints.
type AnswerKeyBalanceDTO struct {
	OptionsCount int `json:"options_count"` // Number of options (e.g., 4)
	MinPerOption int `json:"min_per_option"`
	MaxPerOption int `json:"max_per_option"`
}

// BlueprintValidationResult holds the result of blueprint feasibility validation.
type BlueprintValidationResult struct {
	Valid              bool     `json:"valid"`
	AvailableItems     int      `json:"available_items"`
	RequiredItems      int      `json:"required_items"`
	SatisfiedConstraints []string `json:"satisfied_constraints"`
	ViolatedConstraints  []string `json:"violated_constraints,omitempty"`
	Warnings           []string `json:"warnings,omitempty"`
}

// BlueprintResponse is the API response for a blueprint.
type BlueprintResponse struct {
	ID             uuid.UUID              `json:"id"`
	OrganizationID uuid.UUID             `json:"organization_id"`
	Name           string                 `json:"name"`
	SubjectID      uuid.UUID              `json:"subject_id"`
	TotalItems     int                    `json:"total_items"`
	Constraints    BlueprintConstraintDTO `json:"constraints"`
	Status         string                 `json:"status"`
	Version        int                    `json:"version"`
	CreatedAt      string                 `json:"created_at"`
	UpdatedAt      string                 `json:"updated_at"`
}

func toBlueprintResponse(b *blueprint.Blueprint) BlueprintResponse {
	resp := BlueprintResponse{
		ID:             b.ID,
		OrganizationID: b.OrganizationID,
		Name:           b.Name,
		SubjectID:      b.SubjectID,
		TotalItems:     b.TotalItems,
		Status:         string(b.Status),
		Version:        b.Version,
		CreatedAt:      b.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      b.UpdatedAt.Format(time.RFC3339),
	}

	// Map domain constraints to DTO
	var chapterConstraints []ChapterConstraintDTO
	for _, c := range b.Constraints.ChapterConstraints {
		chapterConstraints = append(chapterConstraints, ChapterConstraintDTO{
			ChapterID: c.ChapterID.String(),
			MinItems:  c.MinItems,
			MaxItems:  c.MaxItems,
		})
	}
	resp.Constraints.ChapterConstraints = chapterConstraints
	resp.Constraints.MaxTimeSecs = b.Constraints.MaxTimeSecs

	return resp
}

// ──────────────────────────────────────────────
//  Routes
// ──────────────────────────────────────────────

// RegisterRoutes registers blueprint routes.
func (h *BlueprintHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/blueprints", h.CreateBlueprint)
	mux.HandleFunc("GET /api/v1/blueprints/{id}", h.GetBlueprint)
	mux.HandleFunc("PUT /api/v1/blueprints/{id}", h.UpdateBlueprint)
	mux.HandleFunc("GET /api/v1/blueprints", h.ListBlueprints)
	mux.HandleFunc("POST /api/v1/blueprints/{id}/validate", h.ValidateBlueprint)
}

// CreateBlueprint handles POST /api/v1/blueprints
func (h *BlueprintHandler) CreateBlueprint(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())

	var req CreateBlueprintRequest
	if err := httputil.BindJSON(r, &req); err != nil {
		httputil.RespondError(w, r, err)
		return
	}

	if req.Name == "" || req.SubjectID == "" || req.TotalItems < 1 {
		httputil.RespondError(w, r, apperrors.NewValidation("validation failed", map[string]string{
			"name":        "required",
			"subject_id":  "required",
			"total_items": "must be at least 1",
		}))
		return
	}

	result, err := h.service.CreateBlueprint(r.Context(), req)
	if err != nil {
		log.Error("failed to create blueprint", zap.Error(err))
		httputil.RespondError(w, r, err)
		return
	}

	httputil.RespondSuccess(w, http.StatusCreated, toBlueprintResponse(result))
}

// GetBlueprint handles GET /api/v1/blueprints/{id}
func (h *BlueprintHandler) GetBlueprint(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())

	bpID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httputil.RespondError(w, r, apperrors.NewBadRequest("invalid blueprint ID"))
		return
	}

	orgID, _ := uuid.Parse(middleware.GetOrganizationID(r.Context()))

	result, err := h.service.GetBlueprint(r.Context(), orgID, bpID)
	if err != nil {
		log.Error("failed to get blueprint", zap.Error(err))
		httputil.RespondError(w, r, err)
		return
	}

	httputil.RespondSuccess(w, http.StatusOK, toBlueprintResponse(result))
}

// UpdateBlueprint handles PUT /api/v1/blueprints/{id}
func (h *BlueprintHandler) UpdateBlueprint(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())

	bpID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httputil.RespondError(w, r, apperrors.NewBadRequest("invalid blueprint ID"))
		return
	}

	orgID, _ := uuid.Parse(middleware.GetOrganizationID(r.Context()))

	var req UpdateBlueprintRequest
	if err := httputil.BindJSON(r, &req); err != nil {
		httputil.RespondError(w, r, err)
		return
	}

	result, err := h.service.UpdateBlueprint(r.Context(), orgID, bpID, req)
	if err != nil {
		log.Error("failed to update blueprint", zap.Error(err))
		httputil.RespondError(w, r, err)
		return
	}

	httputil.RespondSuccess(w, http.StatusOK, toBlueprintResponse(result))
}

// ListBlueprints handles GET /api/v1/blueprints
func (h *BlueprintHandler) ListBlueprints(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())

	orgID, _ := uuid.Parse(middleware.GetOrganizationID(r.Context()))

	cursor := r.URL.Query().Get("cursor")
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	bps, nextCursor, err := h.service.ListBlueprints(r.Context(), orgID, cursor, limit)
	if err != nil {
		log.Error("failed to list blueprints", zap.Error(err))
		httputil.RespondError(w, r, err)
		return
	}

	responses := make([]BlueprintResponse, len(bps))
	for i, bp := range bps {
		responses[i] = toBlueprintResponse(bp)
	}

	httputil.RespondSuccessWithPagination(w, responses, nextCursor, nextCursor != "")
}

// ValidateBlueprint handles POST /api/v1/blueprints/{id}/validate
func (h *BlueprintHandler) ValidateBlueprint(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())

	bpID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httputil.RespondError(w, r, apperrors.NewBadRequest("invalid blueprint ID"))
		return
	}

	orgID, _ := uuid.Parse(middleware.GetOrganizationID(r.Context()))

	result, err := h.service.ValidateBlueprint(r.Context(), orgID, bpID)
	if err != nil {
		log.Error("failed to validate blueprint", zap.Error(err))
		httputil.RespondError(w, r, err)
		return
	}

	httputil.RespondSuccess(w, http.StatusOK, result)
}
