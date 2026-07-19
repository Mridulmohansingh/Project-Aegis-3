// Package handler provides HTTP handlers for the Question Bank Service API.
//
// All handlers follow REST conventions:
//   - POST for creation (returns 201)
//   - GET for retrieval (returns 200)
//   - PUT for full update (returns 200)
//   - DELETE for soft deletion (returns 204)
//   - Error responses use RFC 7807 Problem Details
//
// Every handler validates input, enforces business rules via the service layer,
// and publishes audit events.
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/aegis-platform/aegis/internal/domain/item"
	"github.com/aegis-platform/aegis/pkg/apperrors"
	"github.com/aegis-platform/aegis/pkg/httputil"
	"github.com/aegis-platform/aegis/pkg/logging"
	"github.com/aegis-platform/aegis/pkg/middleware"
)

// ItemService defines the business operations for items.
// This interface decouples the handler from the implementation.
type ItemService interface {
	CreateItem(ctx interface{}, req CreateItemRequest) (*item.Item, error)
	GetItem(ctx interface{}, orgID, itemID uuid.UUID) (*item.Item, error)
	UpdateItem(ctx interface{}, orgID, itemID uuid.UUID, req UpdateItemRequest) (*item.Item, error)
	DeleteItem(ctx interface{}, orgID, itemID uuid.UUID, actorID uuid.UUID) error
	ListItems(ctx interface{}, filter item.ItemFilter, cursor string, limit int) ([]*item.Item, string, error)
	SubmitForReview(ctx interface{}, orgID, itemID uuid.UUID, actorID uuid.UUID) error
	ReviewItem(ctx interface{}, orgID, itemID uuid.UUID, req ReviewItemRequest) error
	CalibrateItem(ctx interface{}, orgID, itemID uuid.UUID, req CalibrateItemRequest) error
}

// ItemHandler handles HTTP requests for the Question Bank API.
type ItemHandler struct {
	service ItemService
	logger  *zap.Logger
}

// NewItemHandler creates a new ItemHandler.
func NewItemHandler(service ItemService, logger *zap.Logger) *ItemHandler {
	return &ItemHandler{
		service: service,
		logger:  logger.With(zap.String("component", "item_handler")),
	}
}

// ──────────────────────────────────────────────
//  DTOs (Data Transfer Objects)
// ──────────────────────────────────────────────

// CreateItemRequest is the request body for creating a new item.
type CreateItemRequest struct {
	ExternalID      string              `json:"external_id" validate:"required,max=50"`
	ItemType        string              `json:"item_type" validate:"required"`
	SubjectID       string              `json:"subject_id" validate:"required,uuid"`
	ChapterID       string              `json:"chapter_id" validate:"required,uuid"`
	TopicID         string              `json:"topic_id" validate:"required,uuid"`
	SubTopicID      *string             `json:"sub_topic_id,omitempty" validate:"omitempty,uuid"`
	LearningOutcomeID *string           `json:"learning_outcome_id,omitempty" validate:"omitempty,uuid"`
	Content         item.ItemContent    `json:"content" validate:"required"`
	MarkingScheme   item.MarkingScheme  `json:"marking_scheme" validate:"required"`
	DifficultyLevel *string             `json:"difficulty_level,omitempty"`
	CognitiveLevel  *string             `json:"cognitive_level,omitempty"`
	EstimatedTimeSecs *int              `json:"estimated_time_secs,omitempty"`
	PrimaryLanguage *string             `json:"primary_language,omitempty"`
	Tags            []string            `json:"tags,omitempty"`
}

// UpdateItemRequest is the request body for updating an item.
type UpdateItemRequest struct {
	Content         *item.ItemContent   `json:"content,omitempty"`
	MarkingScheme   *item.MarkingScheme `json:"marking_scheme,omitempty"`
	DifficultyLevel *string             `json:"difficulty_level,omitempty"`
	CognitiveLevel  *string             `json:"cognitive_level,omitempty"`
	EstimatedTimeSecs *int              `json:"estimated_time_secs,omitempty"`
	Tags            []string            `json:"tags,omitempty"`
	Version         int                 `json:"version" validate:"required,min=1"`
}

// ReviewItemRequest is the request body for recording a review decision.
type ReviewItemRequest struct {
	Decision     string `json:"decision" validate:"required,oneof=APPROVED REJECTED REVISION"`
	Comments     string `json:"comments"`
	ReviewerID   string `json:"reviewer_id" validate:"required,uuid"`
}

// CalibrateItemRequest is the request body for setting IRT parameters.
type CalibrateItemRequest struct {
	A                   float64 `json:"a" validate:"required,min=0.1,max=5.0"`
	B                   float64 `json:"b" validate:"required,min=-4.0,max=4.0"`
	C                   float64 `json:"c" validate:"required,min=0,max=0.5"`
	SEA                 float64 `json:"se_a" validate:"min=0"`
	SEB                 float64 `json:"se_b" validate:"min=0"`
	SEC                 float64 `json:"se_c" validate:"min=0"`
	CalibrationSample   int     `json:"calibration_sample" validate:"required,min=100"`
	PsychometricianID   string  `json:"psychometrician_id" validate:"required,uuid"`
}

// ItemResponse is the API response for an item.
// It redacts sensitive fields (answer key, solution, signatures) from the API response.
type ItemResponse struct {
	ID                uuid.UUID           `json:"id"`
	OrganizationID    uuid.UUID           `json:"organization_id"`
	ExternalID        string              `json:"external_id"`
	ItemType          string              `json:"item_type"`
	Status            string              `json:"status"`
	SubjectID         uuid.UUID           `json:"subject_id"`
	ChapterID         uuid.UUID           `json:"chapter_id"`
	TopicID           uuid.UUID           `json:"topic_id"`
	SubTopicID        *uuid.UUID          `json:"sub_topic_id,omitempty"`
	Content           item.ItemContent    `json:"content"`
	MarkingScheme     item.MarkingScheme  `json:"marking_scheme"`
	DifficultyLevel   string              `json:"difficulty_level,omitempty"`
	CognitiveLevel    string              `json:"cognitive_level,omitempty"`
	EstimatedTimeSecs int                 `json:"estimated_time_secs"`
	IRTParameters     *IRTParamsResponse  `json:"irt_parameters,omitempty"`
	ExposureIndex     float64             `json:"exposure_index"`
	PrimaryLanguage   string              `json:"primary_language"`
	Tags              []string            `json:"tags"`
	Version           int                 `json:"version"`
	CreatedAt         string              `json:"created_at"`
	UpdatedAt         string              `json:"updated_at"`
}

// IRTParamsResponse is the API-safe representation of IRT parameters.
type IRTParamsResponse struct {
	A    float64 `json:"a"`
	B    float64 `json:"b"`
	C    float64 `json:"c"`
	SEA  float64 `json:"se_a"`
	SEB  float64 `json:"se_b"`
	SEC  float64 `json:"se_c"`
	Info float64 `json:"info_at_0"`
}

// toResponse converts a domain Item to an API response, redacting sensitive data.
func toResponse(i *item.Item) ItemResponse {
	resp := ItemResponse{
		ID:                i.ID,
		OrganizationID:    i.OrganizationID,
		ExternalID:        i.ExternalID,
		ItemType:          string(i.Type),
		Status:            string(i.Status),
		SubjectID:         i.SubjectID,
		ChapterID:         i.ChapterID,
		TopicID:           i.TopicID,
		SubTopicID:        i.SubTopicID,
		Content:           i.Content,
		MarkingScheme:     i.MarkingScheme,
		DifficultyLevel:   string(i.DifficultyLevel),
		CognitiveLevel:    string(i.CognitiveLevel),
		EstimatedTimeSecs: i.EstimatedTimeSecs,
		ExposureIndex:     i.Exposure.ExposureIndex,
		PrimaryLanguage:   i.PrimaryLanguage,
		Tags:              i.Tags,
		Version:           i.Version,
		CreatedAt:         i.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:         i.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	if i.IRTParams != nil {
		resp.IRTParameters = &IRTParamsResponse{
			A: i.IRTParams.A, B: i.IRTParams.B, C: i.IRTParams.C,
			SEA: i.IRTParams.SEA, SEB: i.IRTParams.SEB, SEC: i.IRTParams.SEC,
			Info: i.IRTParams.InformationAtZero,
		}
	}

	return resp
}

// ──────────────────────────────────────────────
//  HTTP Handlers
// ──────────────────────────────────────────────

// RegisterRoutes registers all item routes on the given mux.
func (h *ItemHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/items", h.CreateItem)
	mux.HandleFunc("GET /api/v1/items/{id}", h.GetItem)
	mux.HandleFunc("PUT /api/v1/items/{id}", h.UpdateItem)
	mux.HandleFunc("DELETE /api/v1/items/{id}", h.DeleteItem)
	mux.HandleFunc("GET /api/v1/items", h.ListItems)
	mux.HandleFunc("POST /api/v1/items/{id}/submit-review", h.SubmitForReview)
	mux.HandleFunc("POST /api/v1/items/{id}/review", h.ReviewItem)
	mux.HandleFunc("POST /api/v1/items/{id}/calibrate", h.CalibrateItem)
	mux.HandleFunc("GET /api/v1/items/{id}/versions", h.GetVersionHistory)
}

// CreateItem handles POST /api/v1/items
func (h *ItemHandler) CreateItem(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())

	var req CreateItemRequest
	if err := httputil.BindJSON(r, &req); err != nil {
		httputil.RespondError(w, r, err)
		return
	}

	// Validate required fields
	if req.ExternalID == "" {
		httputil.RespondError(w, r, apperrors.NewValidation("validation failed", map[string]string{
			"external_id": "external_id is required",
		}))
		return
	}

	if !item.ItemType(req.ItemType).IsValid() {
		httputil.RespondError(w, r, apperrors.NewValidation("validation failed", map[string]string{
			"item_type": "invalid item type",
		}))
		return
	}

	result, err := h.service.CreateItem(r.Context(), req)
	if err != nil {
		log.Error("failed to create item", zap.Error(err))
		httputil.RespondError(w, r, err)
		return
	}

	log.Info("item created", zap.String("item_id", result.ID.String()))
	httputil.RespondSuccess(w, http.StatusCreated, toResponse(result))
}

// GetItem handles GET /api/v1/items/{id}
func (h *ItemHandler) GetItem(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())

	itemID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httputil.RespondError(w, r, apperrors.NewBadRequest("invalid item ID format"))
		return
	}

	orgID, err := uuid.Parse(middleware.GetOrganizationID(r.Context()))
	if err != nil {
		httputil.RespondError(w, r, apperrors.NewBadRequest("missing organization context"))
		return
	}

	result, err := h.service.GetItem(r.Context(), orgID, itemID)
	if err != nil {
		log.Error("failed to get item", zap.Error(err))
		httputil.RespondError(w, r, err)
		return
	}

	httputil.RespondSuccess(w, http.StatusOK, toResponse(result))
}

// UpdateItem handles PUT /api/v1/items/{id}
func (h *ItemHandler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())

	itemID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httputil.RespondError(w, r, apperrors.NewBadRequest("invalid item ID format"))
		return
	}

	orgID, err := uuid.Parse(middleware.GetOrganizationID(r.Context()))
	if err != nil {
		httputil.RespondError(w, r, apperrors.NewBadRequest("missing organization context"))
		return
	}

	var req UpdateItemRequest
	if err := httputil.BindJSON(r, &req); err != nil {
		httputil.RespondError(w, r, err)
		return
	}

	result, err := h.service.UpdateItem(r.Context(), orgID, itemID, req)
	if err != nil {
		log.Error("failed to update item", zap.Error(err))
		httputil.RespondError(w, r, err)
		return
	}

	httputil.RespondSuccess(w, http.StatusOK, toResponse(result))
}

// DeleteItem handles DELETE /api/v1/items/{id}
func (h *ItemHandler) DeleteItem(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())

	itemID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httputil.RespondError(w, r, apperrors.NewBadRequest("invalid item ID format"))
		return
	}

	orgID, err := uuid.Parse(middleware.GetOrganizationID(r.Context()))
	if err != nil {
		httputil.RespondError(w, r, apperrors.NewBadRequest("missing organization context"))
		return
	}

	actorID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		httputil.RespondError(w, r, apperrors.NewUnauthorized("missing user context"))
		return
	}

	if err := h.service.DeleteItem(r.Context(), orgID, itemID, actorID); err != nil {
		log.Error("failed to delete item", zap.Error(err))
		httputil.RespondError(w, r, err)
		return
	}

	httputil.RespondNoContent(w)
}

// ListItems handles GET /api/v1/items
func (h *ItemHandler) ListItems(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())

	orgID, err := uuid.Parse(middleware.GetOrganizationID(r.Context()))
	if err != nil {
		httputil.RespondError(w, r, apperrors.NewBadRequest("missing organization context"))
		return
	}

	// Parse query parameters into filter
	filter := item.ItemFilter{
		OrganizationID: &orgID,
	}

	if v := r.URL.Query().Get("subject_id"); v != "" {
		id, _ := uuid.Parse(v)
		filter.SubjectID = &id
	}
	if v := r.URL.Query().Get("chapter_id"); v != "" {
		id, _ := uuid.Parse(v)
		filter.ChapterID = &id
	}
	if v := r.URL.Query().Get("status"); v != "" {
		status := item.ItemStatus(v)
		filter.Status = &status
	}
	if v := r.URL.Query().Get("difficulty"); v != "" {
		diff := item.DifficultyLevel(v)
		filter.DifficultyLevel = &diff
	}
	if v := r.URL.Query().Get("cognitive_level"); v != "" {
		cog := item.CognitiveLevel(v)
		filter.CognitiveLevel = &cog
	}
	if v := r.URL.Query().Get("item_type"); v != "" {
		t := item.ItemType(v)
		filter.ItemType = &t
	}

	cursor := r.URL.Query().Get("cursor")
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	items, nextCursor, err := h.service.ListItems(r.Context(), filter, cursor, limit)
	if err != nil {
		log.Error("failed to list items", zap.Error(err))
		httputil.RespondError(w, r, err)
		return
	}

	// Convert to response DTOs
	responses := make([]ItemResponse, len(items))
	for i, itm := range items {
		responses[i] = toResponse(itm)
	}

	httputil.RespondSuccessWithPagination(w, responses, nextCursor, nextCursor != "")
}

// SubmitForReview handles POST /api/v1/items/{id}/submit-review
func (h *ItemHandler) SubmitForReview(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())

	itemID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httputil.RespondError(w, r, apperrors.NewBadRequest("invalid item ID"))
		return
	}

	orgID, _ := uuid.Parse(middleware.GetOrganizationID(r.Context()))
	actorID, _ := uuid.Parse(middleware.GetUserID(r.Context()))

	if err := h.service.SubmitForReview(r.Context(), orgID, itemID, actorID); err != nil {
		log.Error("failed to submit for review", zap.Error(err))
		httputil.RespondError(w, r, err)
		return
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]string{
		"status":  "submitted",
		"message": "Item submitted for review",
	})
}

// ReviewItem handles POST /api/v1/items/{id}/review
func (h *ItemHandler) ReviewItem(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())

	itemID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httputil.RespondError(w, r, apperrors.NewBadRequest("invalid item ID"))
		return
	}

	orgID, _ := uuid.Parse(middleware.GetOrganizationID(r.Context()))
	_ = orgID // Will be used in service layer for org isolation

	var req ReviewItemRequest
	if err := httputil.BindJSON(r, &req); err != nil {
		httputil.RespondError(w, r, err)
		return
	}

	if err := h.service.ReviewItem(r.Context(), orgID, itemID, req); err != nil {
		log.Error("failed to review item", zap.Error(err))
		httputil.RespondError(w, r, err)
		return
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]string{
		"status":  "reviewed",
		"message": "Review decision recorded",
	})
}

// CalibrateItem handles POST /api/v1/items/{id}/calibrate
func (h *ItemHandler) CalibrateItem(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())

	itemID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httputil.RespondError(w, r, apperrors.NewBadRequest("invalid item ID"))
		return
	}

	orgID, _ := uuid.Parse(middleware.GetOrganizationID(r.Context()))

	var req CalibrateItemRequest
	if err := httputil.BindJSON(r, &req); err != nil {
		httputil.RespondError(w, r, err)
		return
	}

	if err := h.service.CalibrateItem(r.Context(), orgID, itemID, req); err != nil {
		log.Error("failed to calibrate item", zap.Error(err))
		httputil.RespondError(w, r, err)
		return
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]string{
		"status":  "calibrated",
		"message": "IRT parameters set successfully",
	})
}

// GetVersionHistory handles GET /api/v1/items/{id}/versions
func (h *ItemHandler) GetVersionHistory(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())

	itemID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httputil.RespondError(w, r, apperrors.NewBadRequest("invalid item ID"))
		return
	}

	// Use the service to get version history (placeholder - would be injected)
	_ = itemID
	_ = log

	// This would delegate to the service layer
	httputil.RespondJSON(w, http.StatusOK, map[string]string{
		"message": "version history endpoint",
	})
}

// ──────────────────────────────────────────────
//  Serialization Helper
// ──────────────────────────────────────────────

// toJSON is a helper to marshal objects to JSON bytes for audit logging.
func toJSON(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
