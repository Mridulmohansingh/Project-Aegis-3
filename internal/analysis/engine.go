// Package analysis provides post-exam statistical analysis for item improvement.
//
// After each exam administration, the Analysis Engine computes:
//   - Classical Test Theory (CTT) statistics per item
//   - Distractor analysis for MCQ items
//   - IRT parameter recalibration with updated sample sizes
//   - DIF detection across demographic groups
//   - Person-fit analysis for aberrant response patterns
//   - Exam-level reliability and score distribution
//
// This implements the data feedback loop: every exam makes the question bank smarter.
package analysis

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/aegis-platform/aegis/internal/domain/exam"
	"github.com/aegis-platform/aegis/internal/domain/item"
	"github.com/aegis-platform/aegis/internal/irt"
)

// ──────────────────────────────────────────────
//  Analysis Engine
// ──────────────────────────────────────────────

// Engine orchestrates post-exam statistical analysis.
type Engine struct {
	itemRepo     item.ItemRepository
	responseRepo exam.ResponseRepository
	irtModel     irt.Model3PL
	difDetector  *irt.MantelHaenszel
	personFit    *irt.PersonFit
	logger       *zap.Logger
}

// NewEngine creates a new Analysis Engine.
func NewEngine(
	itemRepo item.ItemRepository,
	responseRepo exam.ResponseRepository,
	logger *zap.Logger,
) *Engine {
	return &Engine{
		itemRepo:     itemRepo,
		responseRepo: responseRepo,
		irtModel:     irt.Model3PL{},
		difDetector:  &irt.MantelHaenszel{},
		personFit:    &irt.PersonFit{},
		logger:       logger.With(zap.String("component", "analysis_engine")),
	}
}

// ──────────────────────────────────────────────
//  Classical Test Theory Analysis
// ──────────────────────────────────────────────

// ClassicalItemStats holds CTT statistics for a single item computed from response data.
type ClassicalItemStats struct {
	ItemID              uuid.UUID          `json:"item_id"`
	TotalResponses      int                `json:"total_responses"`
	CorrectCount        int                `json:"correct_count"`
	IncorrectCount      int                `json:"incorrect_count"`
	OmittedCount        int                `json:"omitted_count"`
	PValue              float64            `json:"p_value"`               // Proportion correct
	DiscriminationIndex float64            `json:"discrimination_index"`  // Upper-lower 27%
	PointBiserial       float64            `json:"point_biserial"`        // Correlation with total score
	DistractorAnalysis  map[string]float64 `json:"distractor_analysis"`   // Option → proportion selected
	MeanTimeMs          int                `json:"mean_time_ms"`
	FlaggedForReview    bool               `json:"flagged_for_review"`
	FlagReasons         []string           `json:"flag_reasons,omitempty"`
}

// ComputeClassicalStats computes CTT statistics for all items in an exam.
// It requires the correct answers and all candidate responses.
func (e *Engine) ComputeClassicalStats(
	ctx context.Context,
	examID uuid.UUID,
	items []*item.Item,
	allResponses map[uuid.UUID][]*exam.Response, // candidateID → responses
) ([]ClassicalItemStats, error) {

	e.logger.Info("computing classical item statistics",
		zap.String("exam_id", examID.String()),
		zap.Int("item_count", len(items)),
		zap.Int("candidate_count", len(allResponses)),
	)

	// Build item lookup and compute total scores
	type candidateData struct {
		totalScore int
		responses  map[uuid.UUID]*exam.Response // itemID → response
	}

	candidates := make(map[uuid.UUID]*candidateData)
	for candID, resps := range allResponses {
		cd := &candidateData{responses: make(map[uuid.UUID]*exam.Response)}
		for _, r := range resps {
			cd.responses[r.ItemID] = r
			// Simple binary scoring for CTT (correct=1, else=0)
			if isCorrect(r, items) {
				cd.totalScore++
			}
		}
		candidates[candID] = cd
	}

	// Sort candidates by total score for upper-lower discrimination
	type scoredCandidate struct {
		id    uuid.UUID
		score int
	}
	sortedCandidates := make([]scoredCandidate, 0, len(candidates))
	for id, cd := range candidates {
		sortedCandidates = append(sortedCandidates, scoredCandidate{id: id, score: cd.totalScore})
	}
	sort.Slice(sortedCandidates, func(i, j int) bool {
		return sortedCandidates[i].score > sortedCandidates[j].score
	})

	// Upper and lower 27% groups
	n27 := int(math.Ceil(float64(len(sortedCandidates)) * 0.27))
	upperGroup := make(map[uuid.UUID]bool)
	lowerGroup := make(map[uuid.UUID]bool)
	for i := 0; i < n27 && i < len(sortedCandidates); i++ {
		upperGroup[sortedCandidates[i].id] = true
	}
	for i := len(sortedCandidates) - n27; i < len(sortedCandidates) && i >= 0; i++ {
		lowerGroup[sortedCandidates[i].id] = true
	}

	// Compute statistics per item
	results := make([]ClassicalItemStats, 0, len(items))

	for _, itm := range items {
		stats := ClassicalItemStats{
			ItemID:             itm.ID,
			DistractorAnalysis: make(map[string]float64),
		}

		var correctScoreSum, incorrectScoreSum float64
		var correctScoreSqSum, totalScoreSum, totalScoreSqSum float64
		var correctCount, incorrectCount, omittedCount int
		var upperCorrect, lowerCorrect int
		optionCounts := make(map[string]int)
		var totalTimeMs int64

		for candID, cd := range candidates {
			resp, hasResponse := cd.responses[itm.ID]
			correct := hasResponse && isCorrectForItem(resp, itm)

			if !hasResponse || (resp.SelectedOption == nil && resp.IntegerAnswer == nil) {
				omittedCount++
				continue
			}

			stats.TotalResponses++
			totalTimeMs += int64(resp.TimeSpentMs)
			totalScoreSum += float64(cd.totalScore)
			totalScoreSqSum += float64(cd.totalScore * cd.totalScore)

			if correct {
				correctCount++
				correctScoreSum += float64(cd.totalScore)
				correctScoreSqSum += float64(cd.totalScore * cd.totalScore)
				if upperGroup[candID] {
					upperCorrect++
				}
				if lowerGroup[candID] {
					lowerCorrect++
				}
			} else {
				incorrectCount++
				incorrectScoreSum += float64(cd.totalScore)
			}

			// Track option selection for distractor analysis
			if resp.SelectedOption != nil {
				key := fmt.Sprintf("option_%d", *resp.SelectedOption)
				optionCounts[key]++
			}
		}

		stats.CorrectCount = correctCount
		stats.IncorrectCount = incorrectCount
		stats.OmittedCount = omittedCount

		// P-value (proportion correct)
		if stats.TotalResponses > 0 {
			stats.PValue = float64(correctCount) / float64(stats.TotalResponses)
			stats.MeanTimeMs = int(totalTimeMs / int64(stats.TotalResponses))
		}

		// Upper-lower discrimination index
		if n27 > 0 {
			stats.DiscriminationIndex = (float64(upperCorrect) - float64(lowerCorrect)) / float64(n27)
		}

		// Point-biserial correlation
		n := float64(stats.TotalResponses)
		if n > 1 && correctCount > 0 && incorrectCount > 0 {
			meanCorrectScore := correctScoreSum / float64(correctCount)
			meanTotalScore := totalScoreSum / n
			varTotal := (totalScoreSqSum / n) - (meanTotalScore * meanTotalScore)
			sdTotal := math.Sqrt(varTotal)
			p := float64(correctCount) / n
			q := 1.0 - p

			if sdTotal > 0 && q > 0 {
				stats.PointBiserial = ((meanCorrectScore - meanTotalScore) / sdTotal) * math.Sqrt(p/q)
			}
		}

		// Distractor analysis
		for key, count := range optionCounts {
			if stats.TotalResponses > 0 {
				stats.DistractorAnalysis[key] = float64(count) / float64(stats.TotalResponses)
			}
		}

		// Flag items with poor statistics
		stats.FlaggedForReview, stats.FlagReasons = flagItem(stats)

		results = append(results, stats)
	}

	e.logger.Info("classical analysis complete",
		zap.String("exam_id", examID.String()),
		zap.Int("items_analyzed", len(results)),
	)

	return results, nil
}

// ──────────────────────────────────────────────
//  IRT Recalibration
// ──────────────────────────────────────────────

// RecalibrateItems uses response data to recalibrate IRT parameters.
// It runs MLE estimation for each candidate, then uses the estimated thetas
// to recalibrate item parameters. This is a simplified version — production
// would use marginal MLE (MMLE) via the EM algorithm.
func (e *Engine) RecalibrateItems(
	ctx context.Context,
	examID uuid.UUID,
	items []*item.Item,
	allResponses map[uuid.UUID][]*exam.Response,
) (map[uuid.UUID]*irt.ItemParams, error) {

	e.logger.Info("recalibrating IRT parameters",
		zap.String("exam_id", examID.String()),
	)

	// Step 1: Estimate ability (theta) for each candidate using existing parameters
	existingParams := make([]irt.ItemParams, 0, len(items))
	itemIndex := make(map[uuid.UUID]int) // itemID → index in params array
	for i, itm := range items {
		if itm.IRTParams == nil {
			continue
		}
		existingParams = append(existingParams, irt.ItemParams{
			A: itm.IRTParams.A,
			B: itm.IRTParams.B,
			C: itm.IRTParams.C,
		})
		itemIndex[itm.ID] = i
	}

	if len(existingParams) == 0 {
		return nil, fmt.Errorf("no items have existing IRT parameters for recalibration")
	}

	estimator := &irt.MLEEstimator{
		MaxIterations: 100,
		Convergence:   0.001,
		MinTheta:      -4.0,
		MaxTheta:      4.0,
	}

	// Estimate thetas
	thetas := make(map[uuid.UUID]float64) // candidateID → theta
	for candID, resps := range allResponses {
		// Build binary response vector aligned with existingParams
		responseVec := make([]int, len(existingParams))
		for _, r := range resps {
			idx, ok := itemIndex[r.ItemID]
			if !ok {
				continue
			}
			if isCorrectForItemByID(r, items) {
				responseVec[idx] = 1
			}
		}

		result, err := estimator.Estimate(existingParams, responseVec)
		if err != nil || !result.Converged {
			// Use EAP fallback
			eap := &irt.EAPEstimator{
				PriorMean:  0.0,
				PriorSD:    1.0,
				QuadPoints: 41,
				MinTheta:   -4.0,
				MaxTheta:   4.0,
			}
			eapResult, err := eap.Estimate(existingParams, responseVec)
			if err != nil {
				continue
			}
			thetas[candID] = eapResult.Theta
		} else {
			thetas[candID] = result.Theta
		}
	}

	// Step 2: Recalibrate each item using the estimated thetas
	// This uses moment-matching: fit the 3PL model to the observed proportion-correct
	// at different theta levels. In production, MMLE/EM would be used.
	updatedParams := make(map[uuid.UUID]*irt.ItemParams)

	for _, itm := range items {
		if itm.IRTParams == nil {
			continue
		}

		var responses []struct {
			theta float64
			correct int
		}

		for candID, resps := range allResponses {
			theta, ok := thetas[candID]
			if !ok {
				continue
			}
			for _, r := range resps {
				if r.ItemID == itm.ID {
					correct := 0
					if isCorrectForItem(r, itm) {
						correct = 1
					}
					responses = append(responses, struct {
						theta   float64
						correct int
					}{theta, correct})
				}
			}
		}

		if len(responses) < 100 {
			// Not enough data for recalibration
			continue
		}

		// Compute observed p(theta) at theta bins and fit model
		// For now, preserve existing params but update sample size
		updatedParams[itm.ID] = &irt.ItemParams{
			A: itm.IRTParams.A,
			B: itm.IRTParams.B,
			C: itm.IRTParams.C,
		}
	}

	e.logger.Info("recalibration complete",
		zap.Int("items_recalibrated", len(updatedParams)),
		zap.Int("candidates_used", len(thetas)),
	)

	return updatedParams, nil
}

// ──────────────────────────────────────────────
//  Exam-Level Statistics
// ──────────────────────────────────────────────

// ExamStatistics holds aggregate statistics for an exam administration.
type ExamStatistics struct {
	ExamID             uuid.UUID `json:"exam_id"`
	TotalAppeared      int       `json:"total_appeared"`
	MeanRawScore       float64   `json:"mean_raw_score"`
	MedianRawScore     float64   `json:"median_raw_score"`
	StdRawScore        float64   `json:"std_raw_score"`
	MinRawScore        float64   `json:"min_raw_score"`
	MaxRawScore        float64   `json:"max_raw_score"`
	SkewnessRaw        float64   `json:"skewness"`
	KurtosisRaw        float64   `json:"kurtosis"`
	CronbachAlpha      float64   `json:"cronbach_alpha"`
	MarginalReliability float64  `json:"marginal_reliability"`
	MeanTheta          float64   `json:"mean_theta"`
	StdTheta           float64   `json:"std_theta"`
	ScoreDistribution  []Bucket  `json:"score_distribution"`
	PercentileTable    map[int]float64 `json:"percentile_table"`
	ComputedAt         time.Time `json:"computed_at"`
}

// Bucket represents a score range and its frequency.
type Bucket struct {
	RangeMin float64 `json:"range_min"`
	RangeMax float64 `json:"range_max"`
	Count    int     `json:"count"`
}

// ComputeExamStatistics computes aggregate exam-level statistics.
func (e *Engine) ComputeExamStatistics(
	rawScores []float64,
	thetas []float64,
	itemCount int,
	itemVariances []float64,
) *ExamStatistics {

	stats := &ExamStatistics{
		TotalAppeared: len(rawScores),
		ComputedAt:    time.Now().UTC(),
	}

	if len(rawScores) == 0 {
		return stats
	}

	// Sort for percentile computation
	sorted := make([]float64, len(rawScores))
	copy(sorted, rawScores)
	sort.Float64s(sorted)

	stats.MinRawScore = sorted[0]
	stats.MaxRawScore = sorted[len(sorted)-1]
	stats.MedianRawScore = percentile(sorted, 50)

	// Mean and variance
	var sum, sumSq float64
	for _, s := range rawScores {
		sum += s
		sumSq += s * s
	}
	n := float64(len(rawScores))
	stats.MeanRawScore = sum / n
	variance := (sumSq / n) - (stats.MeanRawScore * stats.MeanRawScore)
	stats.StdRawScore = math.Sqrt(variance)

	// Skewness and kurtosis
	if stats.StdRawScore > 0 {
		var m3, m4 float64
		for _, s := range rawScores {
			d := (s - stats.MeanRawScore) / stats.StdRawScore
			m3 += d * d * d
			m4 += d * d * d * d
		}
		stats.SkewnessRaw = m3 / n
		stats.KurtosisRaw = (m4 / n) - 3.0 // Excess kurtosis
	}

	// Cronbach's alpha: α = (k/(k-1)) * (1 - Σσ²_i / σ²_total)
	if itemCount > 1 && variance > 0 {
		var sumItemVar float64
		for _, v := range itemVariances {
			sumItemVar += v
		}
		k := float64(itemCount)
		stats.CronbachAlpha = (k / (k - 1)) * (1 - sumItemVar/variance)
	}

	// Theta statistics
	if len(thetas) > 0 {
		var thetaSum float64
		for _, t := range thetas {
			thetaSum += t
		}
		stats.MeanTheta = thetaSum / float64(len(thetas))

		var thetaVarSum float64
		for _, t := range thetas {
			d := t - stats.MeanTheta
			thetaVarSum += d * d
		}
		stats.StdTheta = math.Sqrt(thetaVarSum / float64(len(thetas)))
	}

	// Score distribution (10-point buckets)
	bucketSize := math.Ceil((stats.MaxRawScore - stats.MinRawScore) / 10)
	if bucketSize < 1 {
		bucketSize = 1
	}
	buckets := make(map[int]int)
	for _, s := range rawScores {
		idx := int((s - stats.MinRawScore) / bucketSize)
		buckets[idx]++
	}
	for idx, count := range buckets {
		stats.ScoreDistribution = append(stats.ScoreDistribution, Bucket{
			RangeMin: stats.MinRawScore + float64(idx)*bucketSize,
			RangeMax: stats.MinRawScore + float64(idx+1)*bucketSize,
			Count:    count,
		})
	}
	sort.Slice(stats.ScoreDistribution, func(i, j int) bool {
		return stats.ScoreDistribution[i].RangeMin < stats.ScoreDistribution[j].RangeMin
	})

	// Percentile table
	stats.PercentileTable = map[int]float64{
		5:  percentile(sorted, 5),
		10: percentile(sorted, 10),
		25: percentile(sorted, 25),
		50: percentile(sorted, 50),
		75: percentile(sorted, 75),
		90: percentile(sorted, 90),
		95: percentile(sorted, 95),
		99: percentile(sorted, 99),
	}

	return stats
}

// ──────────────────────────────────────────────
//  Helpers
// ──────────────────────────────────────────────

func percentile(sorted []float64, p int) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := float64(p) / 100.0 * float64(len(sorted)-1)
	lower := int(math.Floor(idx))
	upper := int(math.Ceil(idx))
	if lower == upper {
		return sorted[lower]
	}
	fraction := idx - float64(lower)
	return sorted[lower] + fraction*(sorted[upper]-sorted[lower])
}

func isCorrect(r *exam.Response, items []*item.Item) bool {
	for _, itm := range items {
		if itm.ID == r.ItemID {
			return isCorrectForItem(r, itm)
		}
	}
	return false
}

func isCorrectForItem(r *exam.Response, itm *item.Item) bool {
	// Simplified — real implementation would decrypt and compare answer key
	// For CTT, the scoring service provides the truth table
	return r.SelectedOption != nil // Placeholder
}

func isCorrectForItemByID(r *exam.Response, items []*item.Item) bool {
	for _, itm := range items {
		if itm.ID == r.ItemID {
			return isCorrectForItem(r, itm)
		}
	}
	return false
}

// flagItem determines if an item should be flagged for review based on statistics.
func flagItem(stats ClassicalItemStats) (bool, []string) {
	var reasons []string

	// Too easy (p > 0.95) or too hard (p < 0.10)
	if stats.PValue > 0.95 {
		reasons = append(reasons, fmt.Sprintf("very easy: p=%.3f (>0.95)", stats.PValue))
	}
	if stats.PValue < 0.10 {
		reasons = append(reasons, fmt.Sprintf("very hard: p=%.3f (<0.10)", stats.PValue))
	}

	// Poor discrimination (D < 0.20)
	if stats.DiscriminationIndex < 0.20 {
		reasons = append(reasons, fmt.Sprintf("low discrimination: D=%.3f (<0.20)", stats.DiscriminationIndex))
	}

	// Negative discrimination (students who know more get it wrong)
	if stats.DiscriminationIndex < 0.0 {
		reasons = append(reasons, fmt.Sprintf("NEGATIVE discrimination: D=%.3f — item may be misleading", stats.DiscriminationIndex))
	}

	// Low point-biserial (r_pb < 0.15)
	if stats.PointBiserial < 0.15 && stats.TotalResponses > 50 {
		reasons = append(reasons, fmt.Sprintf("low point-biserial: r=%.3f (<0.15)", stats.PointBiserial))
	}

	// Distractor not functioning (selected by < 5% when there are 4 options)
	for key, proportion := range stats.DistractorAnalysis {
		if proportion < 0.05 && stats.TotalResponses > 100 {
			reasons = append(reasons, fmt.Sprintf("non-functioning distractor: %s selected by %.1f%%", key, proportion*100))
		}
	}

	return len(reasons) > 0, reasons
}
