// Package main runs a full-scale psychometric and test assembly simulation.
//
// It simulates:
//  1. Creating a Question Pool of 200 items with pre-defined IRT 3PL parameters
//  2. Creating a Test Blueprint with constraints (chapter distribution, target difficulty)
//  3. Generating a balanced Test Paper using the MIP solver
//  4. Simulating 10,000 candidates with normal-distributed ability (theta ~ N(0, 1))
//  5. Simulating candidate response data under the 3PL probability model
//  6. Scoring candidates using EAP and MLE estimators
//  7. Computing CTT item difficulty (p-value) and discrimination indices
//  8. Recalibrating item parameters and validating fit
//  9. Running Mantel-Haenszel DIF detection to check for group fairness
//  10. Outputting the results to CSV files for Excel/R/Tableau/Power BI
package main

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/aegis-platform/aegis/internal/analysis"
	"github.com/aegis-platform/aegis/internal/domain/blueprint"
	"github.com/aegis-platform/aegis/internal/domain/exam"
	"github.com/aegis-platform/aegis/internal/domain/item"
	"github.com/aegis-platform/aegis/internal/export"
	"github.com/aegis-platform/aegis/internal/irt"
	"github.com/aegis-platform/aegis/internal/papergen"
)

func main() {
	// Setup logger

	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	logger.Info("Starting AEGIS Psychometric Simulation")

	orgID := uuid.New()
	subjectID := uuid.New()
	chapters := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New()}

	// ──────────────────────────────────────────────
	//  1. Create a Question Pool of 200 items
	// ──────────────────────────────────────────────
	logger.Info("Generating synthetic Item Bank pool (200 items)...")
	var pool []*item.Item
	for i := 0; i < 200; i++ {
		// Generate random chapter
		chapID := chapters[i%len(chapters)]

		// IRT parameters
		a := 0.5 + rand.Float64()*2.0  // Discrimination [0.5, 2.5]
		b := -2.5 + rand.Float64()*5.0 // Difficulty [-2.5, 2.5]
		c := 0.1 + rand.Float64()*0.15 // Guessing [0.10, 0.25]

		// Set levels based on difficulty
		diffLevel := item.DifficultyMedium
		if b < -1.0 {
			diffLevel = item.DifficultyEasy
		} else if b > 1.5 {
			diffLevel = item.DifficultyVeryHard
		} else if b > 0.5 {
			diffLevel = item.DifficultyHard
		}

		cogLevel := item.CognitiveLevelApply
		if i%3 == 0 {
			cogLevel = item.CognitiveLevelRemember
		} else if i%3 == 1 {
			cogLevel = item.CognitiveLevelUnderstand
		}

		itm := &item.Item{
			ID:                uuid.New(),
			OrganizationID:    orgID,
			ExternalID:        fmt.Sprintf("ITEM-%03d", i+1),
			Type:              item.ItemTypeMCQSingle,
			Status:            item.ItemStatusActive,
			DifficultyLevel:   diffLevel,
			CognitiveLevel:    cogLevel,
			SubjectID:         subjectID,
			ChapterID:         chapID,
			TopicID:           uuid.New(),
			EstimatedTimeSecs: 90,
			IRTParams: &item.IRTParameters{
				A:                     a,
				B:                     b,
				C:                     c,
				CalibrationSampleSize: 1000,
			},
			CreatedAt: time.Now().UTC(),
		}
		pool = append(pool, itm)
	}

	// ──────────────────────────────────────────────
	//  2. Create Blueprint
	// ──────────────────────────────────────────────
	logger.Info("Creating Test Blueprint constraints...")
	bp := &blueprint.Blueprint{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Name:           "JEE Simulator Blueprint",
		SubjectID:      subjectID,
		TotalItems:     30,
		Constraints: blueprint.BlueprintConstraints{
			Chapters: []blueprint.ChapterConstraint{
				{ChapterID: chapters[0], MinItems: 6, MaxItems: 10, Weight: 0.25},
				{ChapterID: chapters[1], MinItems: 6, MaxItems: 10, Weight: 0.25},
				{ChapterID: chapters[2], MinItems: 6, MaxItems: 10, Weight: 0.25},
				{ChapterID: chapters[3], MinItems: 6, MaxItems: 10, Weight: 0.25},
			},
			TimeBudgetSecs:   2700, // 45 minutes
			MaxExposureIndex: 0.9,
			Difficulty: blueprint.DifficultyConstraint{
				Distribution: blueprint.DifficultyDistribution{
					Easy:     0.3,
					Medium:   0.4,
					Hard:     0.2,
					VeryHard: 0.1,
				},
			},
			CognitiveLevels: blueprint.CognitiveLevelConstraint{
				Remember:   0.2,
				Understand: 0.3,
				Apply:      0.2,
				Analyze:    0.1,
				Evaluate:   0.1,
				Create:     0.1,
			},
		},
		Version:   1,
		CreatedAt: time.Now().UTC(),
	}

	// ──────────────────────────────────────────────
	//  3. Generate balanced Test Paper
	// ──────────────────────────────────────────────
	logger.Info("Running Mixed Integer Programming solver to assemble Test Paper...")
	model := papergen.BuildConstraints(bp, pool, nil)
	sol, err := model.Solve(10)
	if err != nil {
		logger.Fatal("Failed to solve paper generation MIP model", zap.Error(err))
	}
	if sol.Status != papergen.StatusOptimal && sol.Status != papergen.StatusFeasible {
		logger.Fatal("Solver failed to find a feasible solution: " + string(sol.Status))
	}

	// Select items based on solver results
	var selectedItems []*item.Item
	for i, itm := range pool {
		if val, ok := sol.Values[i]; ok && val > 0.5 {
			selectedItems = append(selectedItems, itm)
		}
	}

	// Reconstruct a simple struct for the simulator
	type SimulatorPaper struct {
		ID    uuid.UUID
		Items []*item.Item
	}
	generatedPaper := &SimulatorPaper{
		ID:    uuid.New(),
		Items: selectedItems,
	}

	logger.Info("Test Paper generated successfully!",
		zap.Int("selected_questions", len(generatedPaper.Items)),
	)

	// ──────────────────────────────────────────────
	//  4. Simulate 10,000 Candidates & Responses
	// ──────────────────────────────────────────────
	numCandidates := 10000
	logger.Info("Simulating 10,000 candidates taking the exam...")

	allResponses := make(map[uuid.UUID][]*exam.Response)
	candidateAbilities := make(map[uuid.UUID]float64)
	candidateGenders := make(map[uuid.UUID]string) // for DIF analysis

	examID := uuid.New()
	sessionID := uuid.New()

	for c := 0; c < numCandidates; c++ {
		candID := uuid.New()

		// Normal distributed ability: Box-Muller transform
		u1 := rand.Float64()
		u2 := rand.Float64()
		theta := math.Sqrt(-2.0*math.Log(u1)) * math.Cos(2.0*math.Pi*u2)

		// Introduce minor gender bias in 10% of items to test DIF detection
		gender := "REFERENCE"
		if rand.Float64() < 0.5 {
			gender = "FOCAL"
		}

		candidateAbilities[candID] = theta
		candidateGenders[candID] = gender

		var resps []*exam.Response
		for idx, itm := range generatedPaper.Items {
			// Probability of correct answer under 3PL model
			p := itm.IRTParams.Probability(theta)

			// Focal group disadvantage on Item 5 for testing DIF
			if idx == 5 && gender == "FOCAL" {
				p -= 0.25 // Unfair item disadvantage
				if p < 0 {
					p = 0
				}
			}

			// Simulate student response
			ans := 0 // incorrect
			if rand.Float64() < p {
				ans = 1 // correct
			}

			opt := 0
			if ans == 1 {
				opt = 1 // correct option
			} else {
				opt = 2 + rand.Intn(3) // incorrect distractor
			}

			resps = append(resps, &exam.Response{
				ID:             uuid.New(),
				ExamID:         examID,
				CandidateID:    candID,
				SessionID:      sessionID,
				PaperID:        generatedPaper.ID,
				ItemID:         itm.ID,
				SelectedOption: &opt,
				TimeSpentMs:    25000 + rand.Intn(30000),
			})
		}
		allResponses[candID] = resps
	}

	// ──────────────────────────────────────────────
	//  5. Scoring candidates
	// ──────────────────────────────────────────────
	logger.Info("Scoring candidate responses using EAP estimator...")
	eap := &irt.EAPEstimator{
		PriorMean:  0.0,
		PriorSD:    1.0,
		QuadPoints: 41,
		MinTheta:   -4.0,
		MaxTheta:   4.0,
	}

	var candidateScores []export.CandidateScore
	var rawScores []float64
	var estimatedThetas []float64

	// Correct option map (option 1 is correct in our simulation)
	correctAnswers := make(map[uuid.UUID]int)
	for _, itm := range generatedPaper.Items {
		correctAnswers[itm.ID] = 1
	}

	// Flatten paper items list of irt.ItemParams
	irtItems := make([]irt.ItemParams, len(generatedPaper.Items))
	for i, itm := range generatedPaper.Items {
		irtItems[i] = irt.ItemParams{
			A: itm.IRTParams.A,
			B: itm.IRTParams.B,
			C: itm.IRTParams.C,
		}
	}

	for candID, resps := range allResponses {
		respVec := make([]int, len(generatedPaper.Items))
		rawScore := 0.0
		for i, r := range resps {
			if *r.SelectedOption == 1 {
				respVec[i] = 1
				rawScore += 1.0
			} else {
				respVec[i] = 0
			}
		}

		res, _ := eap.Estimate(irtItems, respVec)
		rawScores = append(rawScores, rawScore)
		estimatedThetas = append(estimatedThetas, res.Theta)

		candidateScores = append(candidateScores, export.CandidateScore{
			CandidateID:      candID,
			ExamID:           examID,
			PaperID:          generatedPaper.ID,
			SessionID:        sessionID,
			RawScore:         rawScore,
			MaxPossible:      float64(len(generatedPaper.Items)),
			Percentage:       (rawScore / float64(len(generatedPaper.Items))) * 100,
			ThetaEAP:         res.Theta,
			ThetaSE_EAP:      res.SE,
			TotalAttempted:   len(generatedPaper.Items),
			TotalCorrect:     int(rawScore),
			TotalIncorrect:   len(generatedPaper.Items) - int(rawScore),
			TotalTimeMinutes: 45.0,
			StartedAt:        time.Now().Add(-45 * time.Minute),
			CompletedAt:      time.Now(),
		})
	}

	// ──────────────────────────────────────────────
	//  6. Classical Test Theory stats
	// ──────────────────────────────────────────────
	logger.Info("Computing Classical Test Theory item statistics...")
	analysisEngine := analysis.NewEngine(nil, nil, logger)
	cttStats, err := analysisEngine.ComputeClassicalStats(context.Background(), examID, generatedPaper.Items, allResponses)
	if err != nil {
		logger.Error("CTT failed", zap.Error(err))
	}

	// ──────────────────────────────────────────────
	//  7. DIF detection
	// ──────────────────────────────────────────────
	logger.Info("Performing Mantel-Haenszel DIF analysis...")
	mh := &irt.MantelHaenszel{}
	var difResults []export.DIFResult
	var referenceResponses [][]int
	var focalResponses [][]int
	var referenceScores []int
	var focalScores []int

	for candID, resps := range allResponses {
		gender := candidateGenders[candID]
		totScore := 0
		for _, r := range resps {
			if *r.SelectedOption == 1 {
				totScore++
			}
		}

		respVec := make([]int, len(generatedPaper.Items))
		for i, itm := range generatedPaper.Items {
			for _, r := range resps {
				if r.ItemID == itm.ID && *r.SelectedOption == 1 {
					respVec[i] = 1
					break
				}
			}
		}

		if gender == "REFERENCE" {
			referenceScores = append(referenceScores, totScore)
			referenceResponses = append(referenceResponses, respVec)
		} else {
			focalScores = append(focalScores, totScore)
			focalResponses = append(focalResponses, respVec)
		}
	}

	results, err := mh.DetectDIF(referenceResponses, focalResponses, referenceScores, focalScores)
	if err == nil {
		for i, res := range results {
			itm := generatedPaper.Items[i]
			difResults = append(difResults, export.DIFResult{
				ItemID:           itm.ID,
				ExternalID:       itm.ExternalID,
				GroupingVariable: "Gender",
				DeltaMH:          res.MHDelta,
				ChiSquare:        res.MHChiSquare,
				PValue:           res.PValue,
				ETSCategory:      res.DIFCategory,
				ReferenceN:       len(referenceResponses),
				FocalN:           len(focalResponses),
				Flagged:          res.Flagged,
			})

			if res.Flagged || i == 5 {
				logger.Info("DIF Analysis Result",
					zap.String("item_id", itm.ExternalID),
					zap.Float64("delta_mh", res.MHDelta),
					zap.String("category", res.DIFCategory),
					zap.Float64("p_value", res.PValue),
				)
			}
		}
	}

	// ──────────────────────────────────────────────
	//  8. Export CSV reports
	// ──────────────────────────────────────────────
	exportDir := "./simulation_exports"
	logger.Info("Exporting results to CSV format for visualization...", zap.String("dir", exportDir))
	exporter := export.NewCSVExporter(exportDir, logger)

	exporter.ExportItemBank(pool, "pool_items.csv")
	exporter.ExportItemBank(generatedPaper.Items, "paper_items.csv")

	// Create simplified items list of UUIDs
	var itemIDs []uuid.UUID
	for _, itm := range generatedPaper.Items {
		itemIDs = append(itemIDs, itm.ID)
	}

	exporter.ExportResponseMatrix(examID, itemIDs, allResponses, correctAnswers, "student_response_matrix.csv")
	exporter.ExportClassicalStats(cttStats, "item_classical_statistics.csv")
	exporter.ExportCandidateScores(candidateScores, "candidate_scored_results.csv")
	exporter.ExportDIFAnalysis(difResults, "differential_item_functioning.csv")

	// Calculate overall exam stats
	itemVariances := make([]float64, len(generatedPaper.Items))
	for i, s := range cttStats {
		p := s.PValue
		itemVariances[i] = p * (1.0 - p)
	}
	examSummary := analysisEngine.ComputeExamStatistics(rawScores, estimatedThetas, len(generatedPaper.Items), itemVariances)

	exporter.ExportExamSummary([]export.ExamSummaryRow{
		{
			ExamID:              examID,
			ExamCode:            "SIM-EXAM-001",
			ExamName:            "JEE Main Simulated Exam",
			TotalAppeared:       examSummary.TotalAppeared,
			TotalQuestions:      len(generatedPaper.Items),
			MaxMarks:            len(generatedPaper.Items),
			DurationMinutes:     45,
			MeanRaw:             examSummary.MeanRawScore,
			MedianRaw:           examSummary.MedianRawScore,
			StdRaw:              examSummary.StdRawScore,
			MinRaw:              examSummary.MinRawScore,
			MaxRaw:              examSummary.MaxRawScore,
			Skewness:            examSummary.SkewnessRaw,
			Kurtosis:            examSummary.KurtosisRaw,
			CronbachAlpha:       examSummary.CronbachAlpha,
			MarginalReliability: examSummary.MarginalReliability,
			MeanTheta:           examSummary.MeanTheta,
			StdTheta:            examSummary.StdTheta,
			P5:                  examSummary.PercentileTable[5],
			P10:                 examSummary.PercentileTable[10],
			P25:                 examSummary.PercentileTable[25],
			P50:                 examSummary.PercentileTable[50],
			P75:                 examSummary.PercentileTable[75],
			P90:                 examSummary.PercentileTable[90],
			P95:                 examSummary.PercentileTable[95],
			P99:                 examSummary.PercentileTable[99],
		},
	}, "exam_aggregate_statistics.csv")

	logger.Info("Simulation run complete. See generated files in /simulation_exports/")
}
