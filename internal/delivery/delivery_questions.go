package delivery

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/aegis-platform/aegis/internal/domain/exam"
	"github.com/aegis-platform/aegis/internal/domain/item"
	"github.com/aegis-platform/aegis/pkg/apperrors"
)

// DeliveredOption represents a safe, candidate-facing answer option (no answer key).
type DeliveredOption struct {
	Label   string `json:"label"`   // A, B, C, D
	Content string `json:"content"` // Option text
}

// DeliveredQuestion represents an assessment item delivered to a candidate.
// Crucially, it REDACTS correct answer keys, solutions, and psychometric statistics.
type DeliveredQuestion struct {
	ItemID        uuid.UUID          `json:"item_id"`
	SectionIndex  int                `json:"section_index"`
	QuestionIndex int                `json:"question_index"`
	ItemType      string             `json:"item_type"`
	Stem          string             `json:"stem"` // Question text with candidate watermark
	Options       []DeliveredOption  `json:"options,omitempty"`
	MarkingScheme item.MarkingScheme `json:"marking_scheme"`
	Watermark     string             `json:"watermark"` // Traceability signature
}

// DeliveredPaperResponse contains the complete questions package for an active session.
type DeliveredPaperResponse struct {
	SessionID     uuid.UUID           `json:"session_id"`
	ExamID        uuid.UUID           `json:"exam_id"`
	PaperID       uuid.UUID           `json:"paper_id"`
	RemainingSecs int                 `json:"remaining_secs"`
	Questions     []DeliveredQuestion `json:"questions"`
}

// GetQuestions delivers the examination questions for an authenticated, in-progress session.
// It verifies:
// 1. Session exists and is IN_PROGRESS.
// 2. Scheduled exam release time T has passed.
// 3. Exam time has not expired.
// 4. Stems are watermarked with candidate ID for leak deterrence (A6).
func (s *Service) GetQuestions(ctx context.Context, sessionID uuid.UUID, items []item.Item) (*DeliveredPaperResponse, error) {
	active, ok := s.getActiveSession(sessionID)
	if !ok {
		return nil, apperrors.NewNotFound("Session", sessionID)
	}

	active.mu.Lock()
	defer active.mu.Unlock()

	// 1. Enforce session state
	if active.session.Status != exam.SessionInProgress {
		return nil, apperrors.NewForbidden(fmt.Sprintf("cannot retrieve questions: session status is %s (must be IN_PROGRESS)", active.session.Status))
	}

	// 2. Enforce scheduled release time T
	now := time.Now().UTC()
	if now.Before(active.session.ScheduledStart.UTC()) {
		return nil, apperrors.NewForbidden(fmt.Sprintf("cannot release questions before scheduled start time %s", active.session.ScheduledStart.Format(time.RFC3339)))
	}

	// 3. Enforce remaining duration
	elapsed := int(time.Since(active.startedAt).Seconds())
	remaining := active.durationSecs - elapsed
	if remaining <= 0 {
		s.autoSubmit(ctx, active)
		return nil, apperrors.NewConflict("exam time has expired")
	}

	// 4. Package questions and embed candidate watermark
	deliveredQuestions := make([]DeliveredQuestion, 0, len(items))
	candidateID := active.session.CandidateID.String()

	for idx, itm := range items {
		// Embed cryptographic watermark (e.g., zero-width/auditable watermark token)
		watermarkToken := fmt.Sprintf("AEGIS-AUDIT:[%s:%s]", candidateID[:8], itm.ID.String()[:8])
		watermarkedStem := fmt.Sprintf("%s\n<!-- %s -->", itm.Content.Stem, watermarkToken)

		opts := make([]DeliveredOption, 0, len(itm.Content.Options))
		for _, o := range itm.Content.Options {
			opts = append(opts, DeliveredOption{
				Label:   o.Label,
				Content: o.Content,
			})
		}

		deliveredQuestions = append(deliveredQuestions, DeliveredQuestion{
			ItemID:        itm.ID,
			SectionIndex:  0,
			QuestionIndex: idx + 1,
			ItemType:      string(itm.Type),
			Stem:          watermarkedStem,
			Options:       opts,
			MarkingScheme: itm.MarkingScheme,
			Watermark:     watermarkToken,
		})
	}

	return &DeliveredPaperResponse{
		SessionID:     sessionID,
		ExamID:        active.session.ExamID,
		PaperID:       active.session.PaperID,
		RemainingSecs: remaining,
		Questions:     deliveredQuestions,
	}, nil
}
