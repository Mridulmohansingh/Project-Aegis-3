// Package handler provides HTTP handlers for the Exam Management API.
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/aegis-platform/aegis/internal/domain/exam"
	"github.com/aegis-platform/aegis/pkg/apperrors"
	"github.com/aegis-platform/aegis/pkg/httputil"
	"github.com/aegis-platform/aegis/pkg/logging"
	"github.com/aegis-platform/aegis/pkg/middleware"
)

// ExamService defines the business operations for exams.
type ExamService interface {
	CreateExam(ctx interface{}, req CreateExamRequest) (*exam.Exam, error)
	GetExam(ctx interface{}, orgID, examID uuid.UUID) (*exam.Exam, error)
	UpdateExam(ctx interface{}, orgID, examID uuid.UUID, req UpdateExamRequest) (*exam.Exam, error)
	ListExams(ctx interface{}, orgID uuid.UUID, status *exam.ExamStatus, cursor string, limit int) ([]*exam.Exam, string, error)
	TransitionExam(ctx interface{}, orgID, examID uuid.UUID, targetStatus exam.ExamStatus, actorID uuid.UUID) error
	GeneratePapers(ctx interface{}, orgID, examID uuid.UUID, formCount int, actorID uuid.UUID) (uuid.UUID, error)
}

// ExamHandler handles HTTP requests for the Exam Management API.
type ExamHandler struct {
	service ExamService
	logger  *zap.Logger
}

// NewExamHandler creates a new ExamHandler.
func NewExamHandler(service ExamService, logger *zap.Logger) *ExamHandler {
	return &ExamHandler{
		service: service,
		logger:  logger.With(zap.String("component", "exam_handler")),
	}
}

// ──────────────────────────────────────────────
//  DTOs
// ──────────────────────────────────────────────

// CreateExamRequest is the request body for creating a new exam.
type CreateExamRequest struct {
	ExamCode        string        `json:"exam_code" validate:"required,max=50"`
	ExamName        string        `json:"exam_name" validate:"required,max=200"`
	ExamType        string        `json:"exam_type" validate:"required"`
	TotalMarks      int           `json:"total_marks" validate:"required,min=1"`
	TotalQuestions  int           `json:"total_questions" validate:"required,min=1"`
	DurationMinutes int           `json:"duration_minutes" validate:"required,min=1"`
	NegativeMarking bool          `json:"negative_marking"`
	Sections        []SectionDTO  `json:"sections"`
	BlueprintID     string        `json:"blueprint_id" validate:"required,uuid"`
	Scheduling      SchedulingDTO `json:"scheduling"`
	Config          ExamConfigDTO `json:"config"`
}

// UpdateExamRequest is the request body for updating an exam.
type UpdateExamRequest struct {
	ExamName        *string        `json:"exam_name,omitempty"`
	TotalMarks      *int           `json:"total_marks,omitempty"`
	TotalQuestions  *int           `json:"total_questions,omitempty"`
	DurationMinutes *int           `json:"duration_minutes,omitempty"`
	NegativeMarking *bool          `json:"negative_marking,omitempty"`
	Sections        []SectionDTO   `json:"sections,omitempty"`
	Scheduling      *SchedulingDTO `json:"scheduling,omitempty"`
	Config          *ExamConfigDTO `json:"config,omitempty"`
}

// SectionDTO represents a section in the API layer.
type SectionDTO struct {
	Name            string `json:"name"`
	QuestionCount   int    `json:"question_count"`
	DurationMinutes int    `json:"duration_minutes"`
	Navigation      string `json:"navigation"`
	Mandatory       bool   `json:"mandatory"`
	SubjectID       string `json:"subject_id,omitempty"`
}

// SchedulingDTO represents exam scheduling in the API layer.
type SchedulingDTO struct {
	Windows []ExamWindowDTO `json:"windows"`
}

// ExamWindowDTO represents an exam window.
type ExamWindowDTO struct {
	StartTime string `json:"start_time"` // ISO 8601
	EndTime   string `json:"end_time"`   // ISO 8601
	ShiftName string `json:"shift_name"`
	Capacity  int    `json:"capacity"`
}

// ExamConfigDTO represents exam delivery configuration.
type ExamConfigDTO struct {
	ShuffleQuestions bool `json:"shuffle_questions"`
	ShuffleOptions   bool `json:"shuffle_options"`
	ShowCalculator   bool `json:"show_calculator"`
	AllowMarkReview  bool `json:"allow_mark_review"`
	AutoSubmit       bool `json:"auto_submit"`
	ShowTimer        bool `json:"show_timer"`
	ShowProgress     bool `json:"show_progress"`
}

// GeneratePapersRequest is the request body for paper generation.
type GeneratePapersRequest struct {
	FormCount int `json:"form_count" validate:"required,min=1,max=50"`
}

// TransitionRequest is the request body for status transitions.
type TransitionRequest struct {
	TargetStatus string `json:"target_status" validate:"required"`
}

// ExamResponse is the API response for an exam.
type ExamResponse struct {
	ID              uuid.UUID     `json:"id"`
	OrganizationID  uuid.UUID     `json:"organization_id"`
	ExamCode        string        `json:"exam_code"`
	ExamName        string        `json:"exam_name"`
	ExamType        string        `json:"exam_type"`
	Status          string        `json:"status"`
	TotalMarks      int           `json:"total_marks"`
	TotalQuestions  int           `json:"total_questions"`
	DurationMinutes int           `json:"duration_minutes"`
	NegativeMarking bool          `json:"negative_marking"`
	Sections        []SectionDTO  `json:"sections"`
	BlueprintID     uuid.UUID     `json:"blueprint_id"`
	Config          ExamConfigDTO `json:"config"`
	CreatedAt       string        `json:"created_at"`
	UpdatedAt       string        `json:"updated_at"`
}

func toExamResponse(e *exam.Exam) ExamResponse {
	sections := make([]SectionDTO, len(e.Sections))
	for i, s := range e.Sections {
		sections[i] = SectionDTO{
			Name:            s.Name,
			QuestionCount:   s.QuestionCount,
			DurationMinutes: s.DurationMinutes,
			Navigation:      string(s.Navigation),
			Mandatory:       s.Mandatory,
		}
		if s.SubjectID != nil {
			sections[i].SubjectID = s.SubjectID.String()
		}
	}

	return ExamResponse{
		ID:              e.ID,
		OrganizationID:  e.OrganizationID,
		ExamCode:        e.ExamCode,
		ExamName:        e.ExamName,
		ExamType:        string(e.ExamType),
		Status:          string(e.Status),
		TotalMarks:      e.TotalMarks,
		TotalQuestions:  e.TotalQuestions,
		DurationMinutes: e.DurationMinutes,
		NegativeMarking: e.NegativeMarking,
		Sections:        sections,
		BlueprintID:     e.BlueprintID,
		Config: ExamConfigDTO{
			ShuffleQuestions: e.Config.ShuffleQuestions,
			ShuffleOptions:   e.Config.ShuffleOptions,
			ShowCalculator:   e.Config.ShowCalculator,
			AllowMarkReview:  e.Config.AllowMarkReview,
			AutoSubmit:       e.Config.AutoSubmit,
			ShowTimer:        e.Config.ShowTimer,
			ShowProgress:     e.Config.ShowProgress,
		},
		CreatedAt: e.CreatedAt.Format(time.RFC3339),
		UpdatedAt: e.UpdatedAt.Format(time.RFC3339),
	}
}

// ──────────────────────────────────────────────
//  Routes
// ──────────────────────────────────────────────

// RegisterRoutes registers exam routes.
func (h *ExamHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/exams", h.CreateExam)
	mux.HandleFunc("GET /api/v1/exams/{id}", h.GetExam)
	mux.HandleFunc("PUT /api/v1/exams/{id}", h.UpdateExam)
	mux.HandleFunc("GET /api/v1/exams", h.ListExams)
	mux.HandleFunc("POST /api/v1/exams/{id}/transition", h.TransitionExam)
	mux.HandleFunc("POST /api/v1/exams/{id}/generate-papers", h.GeneratePapers)
}

// CreateExam handles POST /api/v1/exams
func (h *ExamHandler) CreateExam(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())

	var req CreateExamRequest
	if err := httputil.BindJSON(r, &req); err != nil {
		httputil.RespondError(w, r, err)
		return
	}

	if req.ExamCode == "" || req.ExamName == "" {
		httputil.RespondError(w, r, apperrors.NewValidation("validation failed", map[string]string{
			"exam_code": "required",
			"exam_name": "required",
		}))
		return
	}

	if !exam.ExamType(req.ExamType).IsValid() {
		httputil.RespondError(w, r, apperrors.NewValidation("validation failed", map[string]string{
			"exam_type": "invalid exam type",
		}))
		return
	}

	result, err := h.service.CreateExam(r.Context(), req)
	if err != nil {
		log.Error("failed to create exam", zap.Error(err))
		httputil.RespondError(w, r, err)
		return
	}

	httputil.RespondSuccess(w, http.StatusCreated, toExamResponse(result))
}

// GetExam handles GET /api/v1/exams/{id}
func (h *ExamHandler) GetExam(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())

	examID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httputil.RespondError(w, r, apperrors.NewBadRequest("invalid exam ID"))
		return
	}

	orgID, _ := uuid.Parse(middleware.GetOrganizationID(r.Context()))

	result, err := h.service.GetExam(r.Context(), orgID, examID)
	if err != nil {
		log.Error("failed to get exam", zap.Error(err))
		httputil.RespondError(w, r, err)
		return
	}

	httputil.RespondSuccess(w, http.StatusOK, toExamResponse(result))
}

// UpdateExam handles PUT /api/v1/exams/{id}
func (h *ExamHandler) UpdateExam(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())

	examID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httputil.RespondError(w, r, apperrors.NewBadRequest("invalid exam ID"))
		return
	}

	orgID, _ := uuid.Parse(middleware.GetOrganizationID(r.Context()))

	var req UpdateExamRequest
	if err := httputil.BindJSON(r, &req); err != nil {
		httputil.RespondError(w, r, err)
		return
	}

	result, err := h.service.UpdateExam(r.Context(), orgID, examID, req)
	if err != nil {
		log.Error("failed to update exam", zap.Error(err))
		httputil.RespondError(w, r, err)
		return
	}

	httputil.RespondSuccess(w, http.StatusOK, toExamResponse(result))
}

// ListExams handles GET /api/v1/exams
func (h *ExamHandler) ListExams(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())

	orgID, _ := uuid.Parse(middleware.GetOrganizationID(r.Context()))

	var statusFilter *exam.ExamStatus
	if v := r.URL.Query().Get("status"); v != "" {
		s := exam.ExamStatus(v)
		statusFilter = &s
	}

	cursor := r.URL.Query().Get("cursor")
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	exams, nextCursor, err := h.service.ListExams(r.Context(), orgID, statusFilter, cursor, limit)
	if err != nil {
		log.Error("failed to list exams", zap.Error(err))
		httputil.RespondError(w, r, err)
		return
	}

	responses := make([]ExamResponse, len(exams))
	for i, e := range exams {
		responses[i] = toExamResponse(e)
	}

	httputil.RespondSuccessWithPagination(w, responses, nextCursor, nextCursor != "")
}

// TransitionExam handles POST /api/v1/exams/{id}/transition
func (h *ExamHandler) TransitionExam(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())

	examID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httputil.RespondError(w, r, apperrors.NewBadRequest("invalid exam ID"))
		return
	}

	orgID, _ := uuid.Parse(middleware.GetOrganizationID(r.Context()))
	actorID, _ := uuid.Parse(middleware.GetUserID(r.Context()))

	var req TransitionRequest
	if err := httputil.BindJSON(r, &req); err != nil {
		httputil.RespondError(w, r, err)
		return
	}

	targetStatus := exam.ExamStatus(req.TargetStatus)

	if err := h.service.TransitionExam(r.Context(), orgID, examID, targetStatus, actorID); err != nil {
		log.Error("failed to transition exam", zap.Error(err))
		httputil.RespondError(w, r, err)
		return
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]string{
		"status":  req.TargetStatus,
		"message": "Exam transitioned successfully",
	})
}

// GeneratePapers handles POST /api/v1/exams/{id}/generate-papers
func (h *ExamHandler) GeneratePapers(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())

	examID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httputil.RespondError(w, r, apperrors.NewBadRequest("invalid exam ID"))
		return
	}

	orgID, _ := uuid.Parse(middleware.GetOrganizationID(r.Context()))
	actorID, _ := uuid.Parse(middleware.GetUserID(r.Context()))

	var req GeneratePapersRequest
	if err := httputil.BindJSON(r, &req); err != nil {
		httputil.RespondError(w, r, err)
		return
	}

	if req.FormCount < 1 || req.FormCount > 50 {
		httputil.RespondError(w, r, apperrors.NewValidation("validation failed", map[string]string{
			"form_count": "must be between 1 and 50",
		}))
		return
	}

	jobID, err := h.service.GeneratePapers(r.Context(), orgID, examID, req.FormCount, actorID)
	if err != nil {
		log.Error("failed to generate papers", zap.Error(err))
		httputil.RespondError(w, r, err)
		return
	}

	httputil.RespondJSON(w, http.StatusAccepted, map[string]interface{}{
		"job_id":     jobID.String(),
		"status":     "PENDING",
		"form_count": req.FormCount,
		"message":    "Paper generation started. Poll the job status endpoint for progress.",
	})
}

// Ensure unused imports are resolved.
var _ = json.Marshal
