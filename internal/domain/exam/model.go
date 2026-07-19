// Package exam defines the Exam and ExamSession aggregates for the
// AEGIS exam delivery bounded context.
//
// An Exam represents a configured examination event with scheduling,
// sections, and delivery rules. An ExamSession tracks a single
// candidate's interaction with an exam, including timing, responses,
// and integrity controls.
package exam

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/aegis-platform/aegis/pkg/apperrors"
)

// ──────────────────────────────────────────────
//  Enums
// ──────────────────────────────────────────────

// ExamType defines the test delivery mode.
type ExamType string

const (
	ExamTypeFixedForm   ExamType = "FIXED_FORM"      // All candidates get the same (or parallel) fixed form
	ExamTypeLinearOnFly ExamType = "LINEAR_ON_FLY"    // Unique form assembled per candidate from item pool
	ExamTypeCAT         ExamType = "CAT"              // Computerized Adaptive Testing
	ExamTypeMultiStage  ExamType = "MULTI_STAGE"      // Multi-Stage Adaptive Testing (MST)
)

func (t ExamType) IsValid() bool {
	switch t {
	case ExamTypeFixedForm, ExamTypeLinearOnFly, ExamTypeCAT, ExamTypeMultiStage:
		return true
	}
	return false
}

// ExamStatus tracks the lifecycle of an exam.
type ExamStatus string

const (
	ExamStatusDraft           ExamStatus = "DRAFT"
	ExamStatusConfigured      ExamStatus = "CONFIGURED"
	ExamStatusPapersGenerated ExamStatus = "PAPERS_GENERATED"
	ExamStatusScheduled       ExamStatus = "SCHEDULED"
	ExamStatusActive          ExamStatus = "ACTIVE"
	ExamStatusCompleted       ExamStatus = "COMPLETED"
	ExamStatusCancelled       ExamStatus = "CANCELLED"
	ExamStatusArchived        ExamStatus = "ARCHIVED"
)

var examTransitions = map[ExamStatus][]ExamStatus{
	ExamStatusDraft:           {ExamStatusConfigured, ExamStatusCancelled},
	ExamStatusConfigured:      {ExamStatusPapersGenerated, ExamStatusDraft, ExamStatusCancelled},
	ExamStatusPapersGenerated: {ExamStatusScheduled, ExamStatusConfigured, ExamStatusCancelled},
	ExamStatusScheduled:       {ExamStatusActive, ExamStatusCancelled},
	ExamStatusActive:          {ExamStatusCompleted},
	ExamStatusCompleted:       {ExamStatusArchived},
	ExamStatusCancelled:       {},
	ExamStatusArchived:        {},
}

func (s ExamStatus) CanTransitionTo(target ExamStatus) bool {
	for _, t := range examTransitions[s] {
		if t == target {
			return true
		}
	}
	return false
}

// SessionStatus tracks the lifecycle of a candidate's exam session.
type SessionStatus string

const (
	SessionInitialized  SessionStatus = "INITIALIZED"
	SessionAuthenticated SessionStatus = "AUTHENTICATED"
	SessionInProgress   SessionStatus = "IN_PROGRESS"
	SessionPaused       SessionStatus = "PAUSED"
	SessionCompleted    SessionStatus = "COMPLETED"
	SessionTerminated   SessionStatus = "TERMINATED"
	SessionTimedOut     SessionStatus = "TIMED_OUT"
)

// NavigationType defines how candidates can navigate within a section.
type NavigationType string

const (
	NavSequential   NavigationType = "SEQUENTIAL"     // Must answer in order
	NavFree         NavigationType = "FREE"           // Can jump to any question
	NavSectionLocked NavigationType = "SECTION_LOCKED" // Free within section, can't go back to previous sections
)

// ──────────────────────────────────────────────
//  Exam Aggregate
// ──────────────────────────────────────────────

// Exam is the aggregate root for a configured examination.
type Exam struct {
	ID              uuid.UUID  `json:"id" db:"id"`
	OrganizationID  uuid.UUID  `json:"organization_id" db:"organization_id"`
	ExamCode        string     `json:"exam_code" db:"exam_code"`
	ExamName        string     `json:"exam_name" db:"exam_name"`
	ExamType        ExamType   `json:"exam_type" db:"exam_type"`
	Status          ExamStatus `json:"status" db:"status"`
	TotalMarks      int        `json:"total_marks" db:"total_marks"`
	TotalQuestions  int        `json:"total_questions" db:"total_questions"`
	DurationMinutes int        `json:"duration_minutes" db:"duration_minutes"`
	NegativeMarking bool       `json:"negative_marking" db:"negative_marking"`
	Sections        []Section  `json:"sections" db:"sections"`
	BlueprintID     uuid.UUID  `json:"blueprint_id" db:"blueprint_id"`
	Scheduling      Scheduling `json:"scheduling" db:"scheduling"`
	Config          ExamConfig `json:"config" db:"config"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	CreatedBy       uuid.UUID  `json:"created_by" db:"created_by"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
	UpdatedBy       uuid.UUID  `json:"updated_by" db:"updated_by"`
}

// Section defines a section within an exam.
type Section struct {
	Name            string         `json:"name"`
	QuestionCount   int            `json:"question_count"`
	DurationMinutes int            `json:"duration_minutes"` // 0 = shares exam duration
	Navigation      NavigationType `json:"navigation"`
	Mandatory       bool           `json:"mandatory"`
	SubjectID       *uuid.UUID     `json:"subject_id,omitempty"`
}

// Scheduling holds exam window and shift configuration.
type Scheduling struct {
	Windows []ExamWindow `json:"windows"`
}

// ExamWindow defines a time window during which the exam can be taken.
type ExamWindow struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	ShiftName string    `json:"shift_name"` // e.g., "Morning", "Afternoon"
	Capacity  int       `json:"capacity"`   // Max candidates in this window
}

// ExamConfig holds delivery-time configuration.
type ExamConfig struct {
	ShuffleQuestions bool `json:"shuffle_questions"`
	ShuffleOptions   bool `json:"shuffle_options"`
	ShowCalculator   bool `json:"show_calculator"`
	AllowMarkReview  bool `json:"allow_mark_review"`
	AutoSubmit       bool `json:"auto_submit"` // Auto-submit when time runs out
	ShowTimer        bool `json:"show_timer"`
	ShowProgress     bool `json:"show_progress"` // Show question count progress
}

// TransitionTo changes the exam status after validation.
func (e *Exam) TransitionTo(newStatus ExamStatus, actorID uuid.UUID) error {
	if !e.Status.CanTransitionTo(newStatus) {
		return apperrors.NewInvalidStateTransition("Exam", string(e.Status), string(newStatus))
	}
	e.Status = newStatus
	e.UpdatedAt = time.Now().UTC()
	e.UpdatedBy = actorID
	return nil
}

// ──────────────────────────────────────────────
//  Exam Session Aggregate
// ──────────────────────────────────────────────

// ExamSession tracks a single candidate's exam interaction.
// All timing is server-authoritative to prevent client-side manipulation.
type ExamSession struct {
	ID               uuid.UUID     `json:"id" db:"id"`
	ExamID           uuid.UUID     `json:"exam_id" db:"exam_id"`
	CandidateID      uuid.UUID     `json:"candidate_id" db:"candidate_id"`
	PaperID          uuid.UUID     `json:"paper_id" db:"paper_id"`
	CenterID         *uuid.UUID    `json:"center_id,omitempty" db:"center_id"`
	Status           SessionStatus `json:"status" db:"status"`

	// Timing (all server-authoritative)
	ScheduledStart   time.Time  `json:"scheduled_start" db:"scheduled_start"`
	ActualStart      *time.Time `json:"actual_start,omitempty" db:"actual_start"`
	ActualEnd        *time.Time `json:"actual_end,omitempty" db:"actual_end"`
	RemainingSecs    int        `json:"remaining_secs" db:"remaining_secs"`
	PauseCount       int        `json:"pause_count" db:"pause_count"`
	TotalPauseSecs   int        `json:"total_pause_secs" db:"total_pause_secs"`

	// Security
	ClientIP         string `json:"client_ip" db:"client_ip"`
	UserAgent        string `json:"user_agent" db:"user_agent"`
	DeviceFingerprint []byte `json:"-" db:"device_fingerprint"`
	SessionTokenHash []byte `json:"-" db:"session_token_hash"`

	// Integrity
	TotalResponses   int   `json:"total_responses" db:"total_responses"`
	LastSequence     int64 `json:"last_sequence" db:"last_sequence"`

	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}

// Start transitions the session to IN_PROGRESS and records the start time.
func (s *ExamSession) Start(durationMinutes int) error {
	if s.Status != SessionAuthenticated {
		return apperrors.NewConflict(fmt.Sprintf("session must be AUTHENTICATED to start, current: %s", s.Status))
	}
	now := time.Now().UTC()
	s.Status = SessionInProgress
	s.ActualStart = &now
	s.RemainingSecs = durationMinutes * 60
	s.UpdatedAt = now
	return nil
}

// TickTime decrements the remaining time and returns true if time has expired.
func (s *ExamSession) TickTime(elapsedSecs int) bool {
	s.RemainingSecs -= elapsedSecs
	if s.RemainingSecs <= 0 {
		s.RemainingSecs = 0
		return true // Time expired
	}
	return false
}

// Complete marks the session as completed.
func (s *ExamSession) Complete() {
	now := time.Now().UTC()
	s.Status = SessionCompleted
	s.ActualEnd = &now
	s.UpdatedAt = now
}

// Terminate marks the session as terminated (e.g., due to violation).
func (s *ExamSession) Terminate(reason string) {
	now := time.Now().UTC()
	s.Status = SessionTerminated
	s.ActualEnd = &now
	s.UpdatedAt = now
}

// TimeOut marks the session as timed out.
func (s *ExamSession) TimeOut() {
	now := time.Now().UTC()
	s.Status = SessionTimedOut
	s.ActualEnd = &now
	s.RemainingSecs = 0
	s.UpdatedAt = now
}

// ──────────────────────────────────────────────
//  Response Entity
// ──────────────────────────────────────────────

// Response captures a single answer submission from a candidate.
type Response struct {
	ID              uuid.UUID  `json:"id" db:"id"`
	ExamID          uuid.UUID  `json:"exam_id" db:"exam_id"`
	CandidateID     uuid.UUID  `json:"candidate_id" db:"candidate_id"`
	SessionID       uuid.UUID  `json:"session_id" db:"session_id"`
	PaperID         uuid.UUID  `json:"paper_id" db:"paper_id"`
	ItemID          uuid.UUID  `json:"item_id" db:"item_id"`
	SectionIndex    int        `json:"section_index" db:"section_index"`
	QuestionIndex   int        `json:"question_index" db:"question_index"`

	// Response data
	SelectedOption  *int       `json:"selected_option,omitempty" db:"selected_option"`
	SelectedOptions []int      `json:"selected_options,omitempty" db:"selected_options"`
	IntegerAnswer   *int       `json:"integer_answer,omitempty" db:"integer_answer"`
	TextAnswer      []byte     `json:"-" db:"text_answer"` // Encrypted
	IsMarked        bool       `json:"is_marked" db:"is_marked"`
	IsVisited       bool       `json:"is_visited" db:"is_visited"`
	VisitCount      int        `json:"visit_count" db:"visit_count"`
	TimeSpentMs     int        `json:"time_spent_ms" db:"time_spent_ms"`

	// Timeline
	FirstResponseAt *time.Time `json:"first_response_at,omitempty" db:"first_response_at"`
	LastModifiedAt  *time.Time `json:"last_modified_at,omitempty" db:"last_modified_at"`
	ResponseChanges int        `json:"response_changes" db:"response_changes"`

	// Integrity
	ClientTimestamp time.Time `json:"client_timestamp" db:"client_timestamp"`
	ServerTimestamp time.Time `json:"server_timestamp" db:"server_timestamp"`
	ClientHash      []byte    `json:"-" db:"client_hash"`
	SequenceNumber  int64     `json:"sequence_number" db:"sequence_number"`
}

// ──────────────────────────────────────────────
//  Repository Interfaces
// ──────────────────────────────────────────────

// ExamRepository defines persistence operations for exams.
type ExamRepository interface {
	Create(ctx interface{}, exam *Exam) error
	GetByID(ctx interface{}, orgID, id uuid.UUID) (*Exam, error)
	Update(ctx interface{}, exam *Exam) error
	List(ctx interface{}, orgID uuid.UUID, status *ExamStatus, cursor string, limit int) ([]*Exam, string, error)
}

// SessionRepository defines persistence operations for exam sessions.
type SessionRepository interface {
	Create(ctx interface{}, session *ExamSession) error
	GetByID(ctx interface{}, id uuid.UUID) (*ExamSession, error)
	GetByCandidateAndExam(ctx interface{}, candidateID, examID uuid.UUID) (*ExamSession, error)
	Update(ctx interface{}, session *ExamSession) error
	CountActive(ctx interface{}, examID uuid.UUID) (int, error)
}

// ResponseRepository defines persistence operations for exam responses.
type ResponseRepository interface {
	Upsert(ctx interface{}, response *Response) error
	UpsertBatch(ctx interface{}, responses []*Response) error
	GetBySession(ctx interface{}, sessionID uuid.UUID) ([]*Response, error)
	GetByExamAndCandidate(ctx interface{}, examID, candidateID uuid.UUID) ([]*Response, error)
	GetByExamAndItem(ctx interface{}, examID, itemID uuid.UUID) ([]*Response, error)
}
