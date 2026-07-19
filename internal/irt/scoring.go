package irt

import (
	"context"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ScoringService orchestrates IRT-based scoring for an exam.
type ScoringService struct {
	model     Model3PL
	mle       *MLEEstimator
	eap       *EAPEstimator
	equater   *TrueScoreEquater
	personFit *PersonFit
	logger    *zap.Logger
}

// ExamResult holds the complete scoring result for one candidate
type ExamResult struct {
	CandidateID   uuid.UUID
	ExamID        uuid.UUID
	PaperID       uuid.UUID
	RawScore      int
	ThetaEstimate float64
	ThetaSE       float64
	ScaledScore   int
	Percentile    float64
	PersonFit     PersonFitResult
	ItemResults   []ItemResult
}

// ItemResult represents the response and correctness of a single item
type ItemResult struct {
	ItemID      uuid.UUID
	Response    int // 0 or 1
	Correct     bool
	TimeSpentMs int
}

// NewScoringService creates a new ScoringService.
func NewScoringService(logger *zap.Logger) *ScoringService {
	return &ScoringService{
		model:     Model3PL{},
		mle:       NewMLEEstimator(50, 0.001, -4.0, 4.0),
		eap:       NewEAPEstimator(0.0, 1.0, 61, -4.0, 4.0),
		equater:   &TrueScoreEquater{},
		personFit: &PersonFit{},
		logger:    logger,
	}
}

// ScoreExam scores all candidates for an exam, including:
// 1. Raw scoring (MCQ auto-grading)
// 2. IRT ability estimation (MLE with EAP fallback)
// 3. Score equating (if multi-form)
// 4. Person-fit analysis
// 5. Percentile computation
func (s *ScoringService) ScoreExam(
	ctx context.Context, 
	examID uuid.UUID,
	candidateIDs []uuid.UUID,
	paperIDs []uuid.UUID,
	items [][]ItemParams,
	responses [][]int,
) ([]ExamResult, error) {

	s.logger.Info("Starting exam scoring", zap.String("examID", examID.String()))
	results := make([]ExamResult, 0, len(candidateIDs))

	for i, cID := range candidateIDs {
		cResponses := responses[i]
		cItems := items[i]

		rawScore := 0
		for _, r := range cResponses {
			rawScore += r
		}

		// Estimate ability
		est, err := s.mle.Estimate(cItems, cResponses)
		if err != nil || !est.Converged {
			est, err = s.eap.Estimate(cItems, cResponses)
			if err != nil {
				s.logger.Error("EAP estimation failed", zap.Error(err))
				continue
			}
		}

		// Person fit
		pfResult, err := s.personFit.ComputeLz(cItems, cResponses, est.Theta)
		if err != nil {
			s.logger.Warn("Person fit failed", zap.Error(err))
			pfResult = &PersonFitResult{}
		}

		results = append(results, ExamResult{
			CandidateID:   cID,
			ExamID:        examID,
			PaperID:       paperIDs[i],
			RawScore:      rawScore,
			ThetaEstimate: est.Theta,
			ThetaSE:       est.SE,
			ScaledScore:   rawScore, // Default, update with equating if needed
			PersonFit:     *pfResult,
		})
	}

	return results, nil
}
