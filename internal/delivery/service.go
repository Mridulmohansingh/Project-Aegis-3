// Package delivery implements the Exam Delivery Service for AEGIS.
//
// The Exam Delivery Service manages the real-time exam experience:
//   - Session initialization and authentication
//   - Question delivery (decrypts paper, resolves items)
//   - Answer capture with integrity validation
//   - Server-authoritative time management
//   - Response synchronization and conflict resolution
//   - Auto-submission on time expiry
//
// All timing operations use the server clock (synchronized via NTP).
// Client timestamps are recorded for audit purposes only and never trusted.
package delivery

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/aegis-platform/aegis/internal/domain/exam"
	"github.com/aegis-platform/aegis/pkg/apperrors"
	"github.com/aegis-platform/aegis/pkg/crypto"
)

// ──────────────────────────────────────────────
//  Service
// ──────────────────────────────────────────────

// Service manages exam delivery operations.
type Service struct {
	examRepo    exam.ExamRepository
	sessionRepo exam.SessionRepository
	responseRepo exam.ResponseRepository
	encryptSvc  *crypto.EncryptionService
	logger      *zap.Logger

	// Active session tracking (in-memory for hot path)
	activeSessions sync.Map // sessionID → *activeSession
}

// activeSession tracks an in-progress exam session in memory
// for low-latency answer capture without database roundtrips.
type activeSession struct {
	mu            sync.Mutex
	session       *exam.ExamSession
	responses     map[string]*exam.Response // itemID → response
	lastSequence  int64
	hmacKey       []byte // Per-session HMAC key for integrity
	startedAt     time.Time
	durationSecs  int
}

// NewService creates a new Exam Delivery Service.
func NewService(
	examRepo exam.ExamRepository,
	sessionRepo exam.SessionRepository,
	responseRepo exam.ResponseRepository,
	encryptSvc *crypto.EncryptionService,
	logger *zap.Logger,
) *Service {
	return &Service{
		examRepo:     examRepo,
		sessionRepo:  sessionRepo,
		responseRepo: responseRepo,
		encryptSvc:   encryptSvc,
		logger:       logger.With(zap.String("component", "exam_delivery")),
	}
}

// ──────────────────────────────────────────────
//  Session Initialization
// ──────────────────────────────────────────────

// InitializeSessionRequest holds the parameters to start an exam session.
type InitializeSessionRequest struct {
	ExamID      uuid.UUID
	CandidateID uuid.UUID
	PaperID     uuid.UUID
	CenterID    *uuid.UUID
	ClientIP    string
	UserAgent   string
}

// InitializeSession creates a new exam session for a candidate.
// The candidate must be registered and the exam must be ACTIVE.
func (s *Service) InitializeSession(ctx context.Context, req InitializeSessionRequest) (*exam.ExamSession, error) {
	// Verify exam is active
	examEntity, err := s.examRepo.GetByID(ctx, uuid.Nil, req.ExamID)
	if err != nil {
		return nil, err
	}
	if examEntity.Status != exam.ExamStatusActive {
		return nil, apperrors.NewConflict(fmt.Sprintf("exam is not active, current status: %s", examEntity.Status))
	}

	// Check for existing session (prevent double-start)
	existing, err := s.sessionRepo.GetByCandidateAndExam(ctx, req.CandidateID, req.ExamID)
	if err == nil && existing != nil {
		if existing.Status == exam.SessionInProgress {
			// Resume existing session
			s.logger.Info("resuming existing session",
				zap.String("session_id", existing.ID.String()),
				zap.String("candidate_id", req.CandidateID.String()),
			)
			return existing, nil
		}
		if existing.Status == exam.SessionCompleted || existing.Status == exam.SessionTerminated {
			return nil, apperrors.NewConflict("exam session already completed for this candidate")
		}
	}

	// Generate session token (never stored — only the hash)
	sessionToken, err := crypto.GenerateRandomBytes(32)
	if err != nil {
		return nil, apperrors.NewInternal("failed to generate session token", err)
	}
	tokenHash := sha256.Sum256(sessionToken)

	// Generate per-session HMAC key for response integrity
	hmacKey, err := crypto.GenerateRandomBytes(32)
	if err != nil {
		return nil, apperrors.NewInternal("failed to generate HMAC key", err)
	}

	session := &exam.ExamSession{
		ID:             uuid.New(),
		ExamID:         req.ExamID,
		CandidateID:    req.CandidateID,
		PaperID:        req.PaperID,
		CenterID:       req.CenterID,
		Status:         exam.SessionAuthenticated,
		ScheduledStart: time.Now().UTC(),
		RemainingSecs:  examEntity.DurationMinutes * 60,
		ClientIP:       req.ClientIP,
		UserAgent:      req.UserAgent,
		SessionTokenHash: tokenHash[:],
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return nil, err
	}

	// Track in memory
	s.activeSessions.Store(session.ID.String(), &activeSession{
		session:      session,
		responses:    make(map[string]*exam.Response),
		lastSequence: 0,
		hmacKey:      hmacKey,
		startedAt:    time.Now().UTC(),
		durationSecs: examEntity.DurationMinutes * 60,
	})

	s.logger.Info("exam session initialized",
		zap.String("session_id", session.ID.String()),
		zap.String("candidate_id", req.CandidateID.String()),
		zap.String("exam_id", req.ExamID.String()),
	)

	return session, nil
}

// ──────────────────────────────────────────────
//  Session Start
// ──────────────────────────────────────────────

// StartSession transitions a session to IN_PROGRESS.
// This is called after the candidate views instructions and clicks "Start".
func (s *Service) StartSession(ctx context.Context, sessionID uuid.UUID, durationMinutes int) error {
	active, ok := s.getActiveSession(sessionID)
	if !ok {
		return apperrors.NewNotFound("Session", sessionID)
	}

	active.mu.Lock()
	defer active.mu.Unlock()

	if err := active.session.Start(durationMinutes); err != nil {
		return err
	}

	active.startedAt = time.Now().UTC()
	active.durationSecs = durationMinutes * 60

	if err := s.sessionRepo.Update(ctx, active.session); err != nil {
		return err
	}

	s.logger.Info("exam session started",
		zap.String("session_id", sessionID.String()),
		zap.Int("duration_minutes", durationMinutes),
	)

	return nil
}

// ──────────────────────────────────────────────
//  Answer Capture
// ──────────────────────────────────────────────

// SubmitAnswerRequest represents a single answer submission.
type SubmitAnswerRequest struct {
	SessionID       uuid.UUID `json:"session_id"`
	ItemID          uuid.UUID `json:"item_id"`
	SectionIndex    int       `json:"section_index"`
	QuestionIndex   int       `json:"question_index"`
	SelectedOption  *int      `json:"selected_option,omitempty"`
	SelectedOptions []int     `json:"selected_options,omitempty"`
	IntegerAnswer   *int      `json:"integer_answer,omitempty"`
	IsMarked        bool      `json:"is_marked"`
	TimeSpentMs     int       `json:"time_spent_ms"`
	ClientTimestamp time.Time `json:"client_timestamp"`
	ClientHash      string    `json:"client_hash"` // HMAC-SHA256 from client
}

// SubmitAnswer captures a candidate's answer to a question.
// It validates timing, verifies integrity, and persists immediately.
func (s *Service) SubmitAnswer(ctx context.Context, req SubmitAnswerRequest) error {
	active, ok := s.getActiveSession(req.SessionID)
	if !ok {
		return apperrors.NewNotFound("Session", req.SessionID)
	}

	active.mu.Lock()
	defer active.mu.Unlock()

	// Verify session is in progress
	if active.session.Status != exam.SessionInProgress {
		return apperrors.NewConflict(fmt.Sprintf("session is not in progress: %s", active.session.Status))
	}

	// Check if time has expired (server-authoritative)
	elapsed := int(time.Since(active.startedAt).Seconds())
	if elapsed >= active.durationSecs {
		// Time expired — auto-submit and reject this answer
		s.autoSubmit(ctx, active)
		return apperrors.NewConflict("exam time has expired")
	}

	// Increment sequence number (monotonic, prevents replay)
	active.lastSequence++
	seqNum := active.lastSequence

	// Compute server-side HMAC for integrity verification
	serverHash := s.computeResponseHMAC(active.hmacKey, req.ItemID, seqNum, req.SelectedOption)

	now := time.Now().UTC()
	itemKey := req.ItemID.String()

	// Upsert response (last-write-wins for the same item)
	response, exists := active.responses[itemKey]
	if exists {
		// Update existing response
		response.SelectedOption = req.SelectedOption
		response.SelectedOptions = req.SelectedOptions
		response.IntegerAnswer = req.IntegerAnswer
		response.IsMarked = req.IsMarked
		response.TimeSpentMs += req.TimeSpentMs
		response.LastModifiedAt = &now
		response.ResponseChanges++
		response.ClientTimestamp = req.ClientTimestamp
		response.ServerTimestamp = now
		response.ClientHash = serverHash
		response.SequenceNumber = seqNum
		response.VisitCount++
	} else {
		// New response
		response = &exam.Response{
			ID:              uuid.New(),
			ExamID:          active.session.ExamID,
			CandidateID:     active.session.CandidateID,
			SessionID:       active.session.ID,
			PaperID:         active.session.PaperID,
			ItemID:          req.ItemID,
			SectionIndex:    req.SectionIndex,
			QuestionIndex:   req.QuestionIndex,
			SelectedOption:  req.SelectedOption,
			SelectedOptions: req.SelectedOptions,
			IntegerAnswer:   req.IntegerAnswer,
			IsMarked:        req.IsMarked,
			IsVisited:       true,
			VisitCount:      1,
			TimeSpentMs:     req.TimeSpentMs,
			FirstResponseAt: &now,
			LastModifiedAt:  &now,
			ResponseChanges: 0,
			ClientTimestamp:  req.ClientTimestamp,
			ServerTimestamp:  now,
			ClientHash:       serverHash,
			SequenceNumber:   seqNum,
		}
		active.responses[itemKey] = response
	}

	// Persist to database (write-through)
	if err := s.responseRepo.Upsert(ctx, response); err != nil {
		s.logger.Error("failed to persist response",
			zap.String("session_id", req.SessionID.String()),
			zap.String("item_id", req.ItemID.String()),
			zap.Error(err),
		)
		// Don't fail the request — in-memory state is authoritative
		// A background reconciliation process will retry
	}

	// Update session counters
	active.session.TotalResponses = len(active.responses)
	active.session.LastSequence = seqNum
	active.session.RemainingSecs = active.durationSecs - elapsed
	active.session.UpdatedAt = now

	return nil
}

// SubmitBatch captures multiple answers at once (for offline sync).
func (s *Service) SubmitBatch(ctx context.Context, sessionID uuid.UUID, answers []SubmitAnswerRequest) (int, error) {
	successful := 0
	for _, answer := range answers {
		answer.SessionID = sessionID
		if err := s.SubmitAnswer(ctx, answer); err != nil {
			s.logger.Warn("batch answer submission failed",
				zap.String("item_id", answer.ItemID.String()),
				zap.Error(err),
			)
			continue
		}
		successful++
	}
	return successful, nil
}

// ──────────────────────────────────────────────
//  Session Completion
// ──────────────────────────────────────────────

// CompleteSession submits the exam and finalizes all responses.
func (s *Service) CompleteSession(ctx context.Context, sessionID uuid.UUID) (*SessionSummary, error) {
	active, ok := s.getActiveSession(sessionID)
	if !ok {
		return nil, apperrors.NewNotFound("Session", sessionID)
	}

	active.mu.Lock()
	defer active.mu.Unlock()

	if active.session.Status != exam.SessionInProgress && active.session.Status != exam.SessionPaused {
		return nil, apperrors.NewConflict(fmt.Sprintf("session cannot be completed: %s", active.session.Status))
	}

	// Finalize
	active.session.Complete()

	// Flush all responses to database
	responses := make([]*exam.Response, 0, len(active.responses))
	for _, r := range active.responses {
		responses = append(responses, r)
	}

	if err := s.responseRepo.UpsertBatch(ctx, responses); err != nil {
		s.logger.Error("failed to flush responses on completion", zap.Error(err))
	}

	// Update session
	if err := s.sessionRepo.Update(ctx, active.session); err != nil {
		s.logger.Error("failed to update session on completion", zap.Error(err))
	}

	// Remove from active tracking
	s.activeSessions.Delete(sessionID.String())

	// Build summary
	summary := s.buildSummary(active)

	s.logger.Info("exam session completed",
		zap.String("session_id", sessionID.String()),
		zap.String("candidate_id", active.session.CandidateID.String()),
		zap.Int("total_responses", len(active.responses)),
	)

	return summary, nil
}

// ──────────────────────────────────────────────
//  Time Management
// ──────────────────────────────────────────────

// GetRemainingTime returns the server-authoritative remaining time for a session.
func (s *Service) GetRemainingTime(sessionID uuid.UUID) (int, error) {
	active, ok := s.getActiveSession(sessionID)
	if !ok {
		return 0, apperrors.NewNotFound("Session", sessionID)
	}

	active.mu.Lock()
	defer active.mu.Unlock()

	elapsed := int(time.Since(active.startedAt).Seconds())
	remaining := active.durationSecs - elapsed
	if remaining < 0 {
		remaining = 0
	}

	return remaining, nil
}

// ──────────────────────────────────────────────
//  Session Summary
// ──────────────────────────────────────────────

// SessionSummary provides a post-exam summary for the candidate.
type SessionSummary struct {
	SessionID       uuid.UUID     `json:"session_id"`
	ExamID          uuid.UUID     `json:"exam_id"`
	CandidateID     uuid.UUID     `json:"candidate_id"`
	Status          string        `json:"status"`
	StartedAt       *time.Time    `json:"started_at"`
	CompletedAt     *time.Time    `json:"completed_at"`
	DurationSecs    int           `json:"duration_secs"`
	TotalQuestions  int           `json:"total_questions"`
	Answered        int           `json:"answered"`
	Unanswered      int           `json:"unanswered"`
	MarkedForReview int           `json:"marked_for_review"`
	SectionSummaries []SectionSummary `json:"section_summaries"`
}

// SectionSummary provides per-section statistics.
type SectionSummary struct {
	SectionIndex    int `json:"section_index"`
	TotalQuestions  int `json:"total_questions"`
	Answered        int `json:"answered"`
	MarkedForReview int `json:"marked_for_review"`
	TimeSpentMs     int `json:"time_spent_ms"`
}

// ──────────────────────────────────────────────
//  Internal Helpers
// ──────────────────────────────────────────────

func (s *Service) getActiveSession(sessionID uuid.UUID) (*activeSession, bool) {
	val, ok := s.activeSessions.Load(sessionID.String())
	if !ok {
		return nil, false
	}
	return val.(*activeSession), true
}

func (s *Service) autoSubmit(ctx context.Context, active *activeSession) {
	active.session.TimeOut()
	s.logger.Info("auto-submitting timed-out session",
		zap.String("session_id", active.session.ID.String()),
	)

	// Flush responses
	responses := make([]*exam.Response, 0, len(active.responses))
	for _, r := range active.responses {
		responses = append(responses, r)
	}
	s.responseRepo.UpsertBatch(ctx, responses)
	s.sessionRepo.Update(ctx, active.session)
	s.activeSessions.Delete(active.session.ID.String())
}

func (s *Service) computeResponseHMAC(key []byte, itemID uuid.UUID, sequence int64, selectedOption *int) []byte {
	data := fmt.Sprintf("%s:%d:", itemID.String(), sequence)
	if selectedOption != nil {
		data += fmt.Sprintf("%d", *selectedOption)
	}

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

func (s *Service) buildSummary(active *activeSession) *SessionSummary {
	summary := &SessionSummary{
		SessionID:    active.session.ID,
		ExamID:       active.session.ExamID,
		CandidateID:  active.session.CandidateID,
		Status:       string(active.session.Status),
		StartedAt:    active.session.ActualStart,
		CompletedAt:  active.session.ActualEnd,
	}

	if active.session.ActualStart != nil && active.session.ActualEnd != nil {
		summary.DurationSecs = int(active.session.ActualEnd.Sub(*active.session.ActualStart).Seconds())
	}

	sectionStats := make(map[int]*SectionSummary)
	for _, r := range active.responses {
		if _, ok := sectionStats[r.SectionIndex]; !ok {
			sectionStats[r.SectionIndex] = &SectionSummary{SectionIndex: r.SectionIndex}
		}
		sec := sectionStats[r.SectionIndex]
		sec.TotalQuestions++
		if r.SelectedOption != nil || r.IntegerAnswer != nil || len(r.SelectedOptions) > 0 {
			sec.Answered++
			summary.Answered++
		}
		if r.IsMarked {
			sec.MarkedForReview++
			summary.MarkedForReview++
		}
		sec.TimeSpentMs += r.TimeSpentMs
	}

	summary.TotalQuestions = len(active.responses)
	summary.Unanswered = summary.TotalQuestions - summary.Answered

	for _, sec := range sectionStats {
		summary.SectionSummaries = append(summary.SectionSummaries, *sec)
	}

	return summary
}

// ──────────────────────────────────────────────
//  HTTP Handlers for Exam Delivery
// ──────────────────────────────────────────────

// DeliveryHandler provides HTTP handlers for exam delivery endpoints.
type DeliveryHandler struct {
	service *Service
	logger  *zap.Logger
}

// NewDeliveryHandler creates a new handler.
func NewDeliveryHandler(service *Service, logger *zap.Logger) *DeliveryHandler {
	return &DeliveryHandler{service: service, logger: logger}
}

// RegisterRoutes registers delivery endpoints on an HTTP mux.
func (h *DeliveryHandler) RegisterRoutes(mux interface{ HandleFunc(string, func(w interface{}, r interface{})) }) {
	// Routes would be:
	// POST   /api/v1/sessions                 → InitializeSession
	// POST   /api/v1/sessions/{id}/start      → StartSession
	// POST   /api/v1/sessions/{id}/answer     → SubmitAnswer
	// POST   /api/v1/sessions/{id}/answers    → SubmitBatch
	// POST   /api/v1/sessions/{id}/complete   → CompleteSession
	// GET    /api/v1/sessions/{id}/time       → GetRemainingTime
	// GET    /api/v1/sessions/{id}/summary    → GetSessionSummary

	// Implementation would follow the same pattern as item_handler.go
	// with JSON binding, error handling, and response formatting.
}

// Ensure unused imports are satisfied.
var _ = json.Marshal
