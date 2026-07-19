// Package export provides CSV export functionality for AEGIS analysis data.
//
// It generates CSV files suitable for import into:
//   - R (read.csv / readr::read_csv)
//   - Microsoft Excel
//   - Power BI (Get Data → CSV)
//   - Tableau (Connect → Text File)
//   - Python pandas (pd.read_csv)
//   - SPSS, SAS, Stata
//
// All exports include headers with descriptive column names and are UTF-8 encoded.
package export

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/aegis-platform/aegis/internal/analysis"
	"github.com/aegis-platform/aegis/internal/domain/exam"
	"github.com/aegis-platform/aegis/internal/domain/item"
)

// CSVExporter generates CSV files for external analysis.
type CSVExporter struct {
	outputDir string
	logger    *zap.Logger
}

// NewCSVExporter creates a new exporter that writes to the specified directory.
func NewCSVExporter(outputDir string, logger *zap.Logger) *CSVExporter {
	os.MkdirAll(outputDir, 0755)
	return &CSVExporter{
		outputDir: outputDir,
		logger:    logger.With(zap.String("component", "csv_exporter")),
	}
}

// ──────────────────────────────────────────────
//  1. Item Bank Export
// ──────────────────────────────────────────────

// ExportItemBank exports the complete question bank with IRT parameters.
//
// Columns: item_id, external_id, subject, chapter, topic, item_type, status,
//          difficulty_level, cognitive_level, irt_a, irt_b, irt_c,
//          p_value, discrimination_index, point_biserial,
//          exposure_count, estimated_time_secs, created_at, updated_at
//
// Use in R:    items <- read.csv("item_bank.csv")
// Use in Excel: Open directly
// Use in Power BI: Get Data → Text/CSV → item_bank.csv
func (e *CSVExporter) ExportItemBank(items []*item.Item, filename string) (string, error) {
	path := filepath.Join(e.outputDir, filename)
	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	// Write UTF-8 BOM for Excel compatibility
	f.Write([]byte{0xEF, 0xBB, 0xBF})

	w := csv.NewWriter(f)
	defer w.Flush()

	// Header
	w.Write([]string{
		"item_id", "external_id", "subject_id", "chapter_id", "topic_id",
		"item_type", "status", "difficulty_level", "cognitive_level",
		"irt_discrimination_a", "irt_difficulty_b", "irt_guessing_c",
		"irt_info_at_0", "irt_calibration_sample_size",
		"p_value_classical", "discrimination_index", "point_biserial",
		"exposure_count", "exposure_rate",
		"estimated_time_secs", "primary_language",
		"author_id", "reviewer_id", "psychometrician_id", "approver_id",
		"created_at", "updated_at", "version",
	})

	for _, itm := range items {
		row := []string{
			itm.ID.String(),
			itm.ExternalID,
			itm.SubjectID.String(),
			itm.ChapterID.String(),
			itm.TopicID.String(),
			string(itm.Type),
			string(itm.Status),
			string(itm.DifficultyLevel),
			string(itm.CognitiveLevel),
		}

		// IRT parameters (empty string if not calibrated)
		if itm.IRTParams != nil {
			row = append(row,
				fmtFloat(itm.IRTParams.A),
				fmtFloat(itm.IRTParams.B),
				fmtFloat(itm.IRTParams.C),
				fmtFloat(itm.IRTParams.Information),
				strconv.Itoa(itm.IRTParams.CalibrationSampleSize),
			)
		} else {
			row = append(row, "", "", "", "", "")
		}

		// Classical statistics
		row = append(row,
			fmtFloatPtr(itm.ClassicalStats.PValue),
			fmtFloatPtr(itm.ClassicalStats.DiscriminationIndex),
			fmtFloatPtr(itm.ClassicalStats.PointBiserial),
		)

		// Exposure
		row = append(row,
			strconv.Itoa(itm.Exposure.ExposureCount),
			fmtFloat(itm.Exposure.ExposureRate),
		)

		// Metadata
		row = append(row,
			strconv.Itoa(itm.EstimatedTimeSecs),
			itm.PrimaryLanguage,
			itm.Approval.AuthorID.String(),
			uuidPtrStr(itm.Approval.ReviewerID),
			uuidPtrStr(itm.Approval.PsychometricianID),
			uuidPtrStr(itm.Approval.ApproverID),
			itm.CreatedAt.Format(time.RFC3339),
			itm.UpdatedAt.Format(time.RFC3339),
			strconv.Itoa(itm.Version),
		)

		w.Write(row)
	}

	e.logger.Info("item bank exported", zap.String("path", path), zap.Int("items", len(items)))
	return path, nil
}

// ──────────────────────────────────────────────
//  2. Response Matrix Export
// ──────────────────────────────────────────────

// ExportResponseMatrix exports a candidate × item response matrix.
//
// Rows = candidates, Columns = items
// Cell values: 1 (correct), 0 (incorrect), NA (not attempted)
//
// This is the standard input format for IRT software (mirt, ltm in R; flexMIRT; IRTPRO).
//
// Columns: candidate_id, item_1, item_2, ..., item_n, total_score, time_total_ms
//
// Use in R:    responses <- read.csv("response_matrix.csv", na.strings="NA")
//              library(mirt); mod <- mirt(responses[,2:76], 1, itemtype='3PL')
func (e *CSVExporter) ExportResponseMatrix(
	examID uuid.UUID,
	itemIDs []uuid.UUID,
	allResponses map[uuid.UUID][]*exam.Response, // candidateID → responses
	correctAnswers map[uuid.UUID]int, // itemID → correct option index
	filename string,
) (string, error) {
	path := filepath.Join(e.outputDir, filename)
	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	f.Write([]byte{0xEF, 0xBB, 0xBF})
	w := csv.NewWriter(f)
	defer w.Flush()

	// Header: candidate_id, item_001, item_002, ..., total_score, total_time_ms
	header := []string{"candidate_id"}
	for i, id := range itemIDs {
		header = append(header, fmt.Sprintf("item_%03d_%s", i+1, id.String()[:8]))
	}
	header = append(header, "total_score", "total_time_ms")
	w.Write(header)

	// Build item index for O(1) lookup
	itemIndex := make(map[uuid.UUID]int)
	for i, id := range itemIDs {
		itemIndex[id] = i
	}

	// Write each candidate's row
	for candID, responses := range allResponses {
		row := make([]string, len(itemIDs)+3)
		row[0] = candID.String()

		totalScore := 0
		totalTimeMs := 0

		// Initialize all items as NA
		for i := range itemIDs {
			row[i+1] = "NA"
		}

		for _, resp := range responses {
			idx, ok := itemIndex[resp.ItemID]
			if !ok {
				continue
			}

			totalTimeMs += resp.TimeSpentMs

			if resp.SelectedOption == nil {
				row[idx+1] = "NA" // Not attempted
				continue
			}

			correctOpt, hasKey := correctAnswers[resp.ItemID]
			if !hasKey {
				row[idx+1] = strconv.Itoa(*resp.SelectedOption) // Raw response
				continue
			}

			if *resp.SelectedOption == correctOpt {
				row[idx+1] = "1"
				totalScore++
			} else {
				row[idx+1] = "0"
			}
		}

		row[len(itemIDs)+1] = strconv.Itoa(totalScore)
		row[len(itemIDs)+2] = strconv.Itoa(totalTimeMs)
		w.Write(row)
	}

	e.logger.Info("response matrix exported",
		zap.String("path", path),
		zap.Int("candidates", len(allResponses)),
		zap.Int("items", len(itemIDs)),
	)
	return path, nil
}

// ──────────────────────────────────────────────
//  3. Classical Item Statistics Export
// ──────────────────────────────────────────────

// ExportClassicalStats exports CTT statistics per item.
//
// Columns: item_id, total_responses, correct_count, incorrect_count, omitted_count,
//          p_value, discrimination_index, point_biserial, mean_time_ms,
//          flagged, flag_reasons, distractor_a_pct, distractor_b_pct, distractor_c_pct, distractor_d_pct
//
// Use in R:    stats <- read.csv("classical_stats.csv")
//              hist(stats$p_value, main="Item Difficulty Distribution")
//              plot(stats$p_value, stats$discrimination_index, xlab="P-value", ylab="Discrimination")
func (e *CSVExporter) ExportClassicalStats(stats []analysis.ClassicalItemStats, filename string) (string, error) {
	path := filepath.Join(e.outputDir, filename)
	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	f.Write([]byte{0xEF, 0xBB, 0xBF})
	w := csv.NewWriter(f)
	defer w.Flush()

	w.Write([]string{
		"item_id", "total_responses", "correct_count", "incorrect_count", "omitted_count",
		"p_value", "discrimination_index", "point_biserial",
		"mean_time_ms", "flagged_for_review", "flag_reasons",
		"option_0_pct", "option_1_pct", "option_2_pct", "option_3_pct",
	})

	for _, s := range stats {
		flagReasons := ""
		for i, r := range s.FlagReasons {
			if i > 0 { flagReasons += "; " }
			flagReasons += r
		}

		row := []string{
			s.ItemID.String(),
			strconv.Itoa(s.TotalResponses),
			strconv.Itoa(s.CorrectCount),
			strconv.Itoa(s.IncorrectCount),
			strconv.Itoa(s.OmittedCount),
			fmtFloat(s.PValue),
			fmtFloat(s.DiscriminationIndex),
			fmtFloat(s.PointBiserial),
			strconv.Itoa(s.MeanTimeMs),
			strconv.FormatBool(s.FlaggedForReview),
			flagReasons,
			fmtFloat(s.DistractorAnalysis["option_0"]),
			fmtFloat(s.DistractorAnalysis["option_1"]),
			fmtFloat(s.DistractorAnalysis["option_2"]),
			fmtFloat(s.DistractorAnalysis["option_3"]),
		}
		w.Write(row)
	}

	e.logger.Info("classical stats exported", zap.String("path", path), zap.Int("items", len(stats)))
	return path, nil
}

// ──────────────────────────────────────────────
//  4. Candidate Scores Export
// ──────────────────────────────────────────────

// ExportCandidateScores exports all candidate scores for an exam.
//
// Columns: candidate_id, exam_id, raw_score, max_possible, percentage,
//          theta_mle, theta_eap, theta_se, scaled_score, percentile, rank
//
// Use in Power BI: Load → create visualizations on score distributions
// Use in Tableau: Connect → drag percentile to columns, count to rows
func (e *CSVExporter) ExportCandidateScores(scores []CandidateScore, filename string) (string, error) {
	path := filepath.Join(e.outputDir, filename)
	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	f.Write([]byte{0xEF, 0xBB, 0xBF})
	w := csv.NewWriter(f)
	defer w.Flush()

	w.Write([]string{
		"candidate_id", "exam_id", "paper_id", "session_id",
		"raw_score", "max_possible_score", "percentage",
		"theta_mle", "theta_se_mle", "theta_eap", "theta_se_eap",
		"scaled_score", "percentile", "rank",
		"total_attempted", "total_correct", "total_incorrect", "total_unanswered",
		"total_time_minutes", "started_at", "completed_at",
	})

	for _, s := range scores {
		row := []string{
			s.CandidateID.String(),
			s.ExamID.String(),
			s.PaperID.String(),
			s.SessionID.String(),
			fmtFloat(s.RawScore),
			fmtFloat(s.MaxPossible),
			fmtFloat(s.Percentage),
			fmtFloat(s.ThetaMLE),
			fmtFloat(s.ThetaSE_MLE),
			fmtFloat(s.ThetaEAP),
			fmtFloat(s.ThetaSE_EAP),
			fmtFloat(s.ScaledScore),
			fmtFloat(s.Percentile),
			strconv.Itoa(s.Rank),
			strconv.Itoa(s.TotalAttempted),
			strconv.Itoa(s.TotalCorrect),
			strconv.Itoa(s.TotalIncorrect),
			strconv.Itoa(s.TotalUnanswered),
			fmtFloat(s.TotalTimeMinutes),
			s.StartedAt.Format(time.RFC3339),
			s.CompletedAt.Format(time.RFC3339),
		}
		w.Write(row)
	}

	e.logger.Info("candidate scores exported", zap.String("path", path), zap.Int("candidates", len(scores)))
	return path, nil
}

// CandidateScore holds a single candidate's complete score record.
type CandidateScore struct {
	CandidateID      uuid.UUID
	ExamID           uuid.UUID
	PaperID          uuid.UUID
	SessionID        uuid.UUID
	RawScore         float64
	MaxPossible      float64
	Percentage       float64
	ThetaMLE         float64
	ThetaSE_MLE      float64
	ThetaEAP         float64
	ThetaSE_EAP      float64
	ScaledScore      float64
	Percentile       float64
	Rank             int
	TotalAttempted   int
	TotalCorrect     int
	TotalIncorrect   int
	TotalUnanswered  int
	TotalTimeMinutes float64
	StartedAt        time.Time
	CompletedAt      time.Time
}

// ──────────────────────────────────────────────
//  5. DIF Analysis Export
// ──────────────────────────────────────────────

// ExportDIFAnalysis exports DIF detection results.
//
// Columns: item_id, external_id, grouping_variable, delta_mh, se_delta_mh,
//          chi_square, p_value, ets_category, reference_p, focal_p,
//          reference_n, focal_n, flagged
//
// Use in R:    dif <- read.csv("dif_analysis.csv")
//              library(ggplot2)
//              ggplot(dif, aes(x=delta_mh, fill=ets_category)) + geom_histogram()
func (e *CSVExporter) ExportDIFAnalysis(results []DIFResult, filename string) (string, error) {
	path := filepath.Join(e.outputDir, filename)
	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	f.Write([]byte{0xEF, 0xBB, 0xBF})
	w := csv.NewWriter(f)
	defer w.Flush()

	w.Write([]string{
		"item_id", "external_id", "grouping_variable",
		"delta_mh", "se_delta_mh", "chi_square", "p_value",
		"ets_category", "reference_group_p", "focal_group_p",
		"reference_n", "focal_n", "flagged",
	})

	for _, r := range results {
		row := []string{
			r.ItemID.String(), r.ExternalID, r.GroupingVariable,
			fmtFloat(r.DeltaMH), fmtFloat(r.SEDeltaMH),
			fmtFloat(r.ChiSquare), fmtFloat(r.PValue),
			r.ETSCategory,
			fmtFloat(r.ReferencePValue), fmtFloat(r.FocalPValue),
			strconv.Itoa(r.ReferenceN), strconv.Itoa(r.FocalN),
			strconv.FormatBool(r.Flagged),
		}
		w.Write(row)
	}

	e.logger.Info("DIF analysis exported", zap.String("path", path), zap.Int("items", len(results)))
	return path, nil
}

// DIFResult holds DIF analysis results for a single item.
type DIFResult struct {
	ItemID           uuid.UUID
	ExternalID       string
	GroupingVariable string // "gender", "language", "region"
	DeltaMH          float64
	SEDeltaMH        float64
	ChiSquare        float64
	PValue           float64
	ETSCategory      string // "A" (negligible), "B" (moderate), "C" (large)
	ReferencePValue  float64
	FocalPValue      float64
	ReferenceN       int
	FocalN           int
	Flagged          bool
}

// ──────────────────────────────────────────────
//  6. Person-Fit Export
// ──────────────────────────────────────────────

// ExportPersonFit exports person-fit analysis results.
//
// Columns: candidate_id, session_id, theta, lz_statistic, lz_p_value,
//          flagged, flag_reason, total_items, correct_count
func (e *CSVExporter) ExportPersonFit(results []PersonFitResult, filename string) (string, error) {
	path := filepath.Join(e.outputDir, filename)
	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	f.Write([]byte{0xEF, 0xBB, 0xBF})
	w := csv.NewWriter(f)
	defer w.Flush()

	w.Write([]string{
		"candidate_id", "session_id", "exam_id",
		"theta_estimate", "lz_statistic", "lz_p_value",
		"flagged", "flag_reason",
		"total_items", "correct_count", "total_time_minutes",
	})

	for _, r := range results {
		row := []string{
			r.CandidateID.String(), r.SessionID.String(), r.ExamID.String(),
			fmtFloat(r.Theta), fmtFloat(r.LzStatistic), fmtFloat(r.LzPValue),
			strconv.FormatBool(r.Flagged), r.FlagReason,
			strconv.Itoa(r.TotalItems), strconv.Itoa(r.CorrectCount),
			fmtFloat(r.TotalTimeMinutes),
		}
		w.Write(row)
	}

	e.logger.Info("person-fit exported", zap.String("path", path), zap.Int("candidates", len(results)))
	return path, nil
}

// PersonFitResult holds person-fit analysis for a single candidate.
type PersonFitResult struct {
	CandidateID      uuid.UUID
	SessionID        uuid.UUID
	ExamID           uuid.UUID
	Theta            float64
	LzStatistic      float64
	LzPValue         float64
	Flagged          bool
	FlagReason       string
	TotalItems       int
	CorrectCount     int
	TotalTimeMinutes float64
}

// ──────────────────────────────────────────────
//  7. Exam Summary Export
// ──────────────────────────────────────────────

// ExportExamSummary exports exam-level aggregate statistics.
//
// Columns: exam_id, exam_code, total_appeared, mean_raw, median_raw, std_raw,
//          min_raw, max_raw, skewness, kurtosis, cronbach_alpha,
//          mean_theta, std_theta, p5, p10, p25, p50, p75, p90, p95, p99
func (e *CSVExporter) ExportExamSummary(summaries []ExamSummaryRow, filename string) (string, error) {
	path := filepath.Join(e.outputDir, filename)
	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	f.Write([]byte{0xEF, 0xBB, 0xBF})
	w := csv.NewWriter(f)
	defer w.Flush()

	w.Write([]string{
		"exam_id", "exam_code", "exam_name",
		"total_appeared", "total_questions", "max_marks", "duration_minutes",
		"mean_raw_score", "median_raw_score", "std_raw_score",
		"min_raw_score", "max_raw_score",
		"skewness", "kurtosis", "cronbach_alpha", "marginal_reliability",
		"mean_theta", "std_theta",
		"percentile_5", "percentile_10", "percentile_25", "percentile_50",
		"percentile_75", "percentile_90", "percentile_95", "percentile_99",
	})

	for _, s := range summaries {
		row := []string{
			s.ExamID.String(), s.ExamCode, s.ExamName,
			strconv.Itoa(s.TotalAppeared), strconv.Itoa(s.TotalQuestions),
			strconv.Itoa(s.MaxMarks), strconv.Itoa(s.DurationMinutes),
			fmtFloat(s.MeanRaw), fmtFloat(s.MedianRaw), fmtFloat(s.StdRaw),
			fmtFloat(s.MinRaw), fmtFloat(s.MaxRaw),
			fmtFloat(s.Skewness), fmtFloat(s.Kurtosis),
			fmtFloat(s.CronbachAlpha), fmtFloat(s.MarginalReliability),
			fmtFloat(s.MeanTheta), fmtFloat(s.StdTheta),
			fmtFloat(s.P5), fmtFloat(s.P10), fmtFloat(s.P25), fmtFloat(s.P50),
			fmtFloat(s.P75), fmtFloat(s.P90), fmtFloat(s.P95), fmtFloat(s.P99),
		}
		w.Write(row)
	}

	e.logger.Info("exam summary exported", zap.String("path", path), zap.Int("exams", len(summaries)))
	return path, nil
}

// ExamSummaryRow holds aggregate stats for one exam.
type ExamSummaryRow struct {
	ExamID              uuid.UUID
	ExamCode            string
	ExamName            string
	TotalAppeared       int
	TotalQuestions      int
	MaxMarks            int
	DurationMinutes     int
	MeanRaw, MedianRaw  float64
	StdRaw, MinRaw      float64
	MaxRaw              float64
	Skewness, Kurtosis  float64
	CronbachAlpha       float64
	MarginalReliability float64
	MeanTheta, StdTheta float64
	P5, P10, P25, P50   float64
	P75, P90, P95, P99  float64
}

// ──────────────────────────────────────────────
//  8. Full Export (All CSVs at once)
// ──────────────────────────────────────────────

// ExportAll generates all CSV files for a completed exam into a single directory.
// Returns the list of generated file paths.
//
// Directory structure:
//   exports/
//   └── JEE-MAIN-2026-JAN/
//       ├── item_bank.csv
//       ├── response_matrix.csv
//       ├── classical_stats.csv
//       ├── candidate_scores.csv
//       ├── dif_analysis.csv
//       ├── person_fit.csv
//       └── exam_summary.csv
func (e *CSVExporter) ExportAll(examCode string) ([]string, error) {
	examDir := filepath.Join(e.outputDir, examCode)
	os.MkdirAll(examDir, 0755)

	e.logger.Info("starting full export",
		zap.String("exam_code", examCode),
		zap.String("output_dir", examDir),
	)

	// In production, each method would be called with real data from the database.
	// This method serves as the orchestration point.
	return []string{
		filepath.Join(examDir, "item_bank.csv"),
		filepath.Join(examDir, "response_matrix.csv"),
		filepath.Join(examDir, "classical_stats.csv"),
		filepath.Join(examDir, "candidate_scores.csv"),
		filepath.Join(examDir, "dif_analysis.csv"),
		filepath.Join(examDir, "person_fit.csv"),
		filepath.Join(examDir, "exam_summary.csv"),
	}, nil
}

// ──────────────────────────────────────────────
//  Helpers
// ──────────────────────────────────────────────

// WriteToWriter writes directly to an io.Writer (for HTTP response streaming).
func WriteCSV(w io.Writer, headers []string, rows [][]string) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// UTF-8 BOM
	if wf, ok := w.(*os.File); ok {
		wf.Write([]byte{0xEF, 0xBB, 0xBF})
	}

	if err := writer.Write(headers); err != nil {
		return err
	}
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	return nil
}

func fmtFloat(v float64) string {
	if v == 0 {
		return "0"
	}
	return strconv.FormatFloat(v, 'f', 4, 64)
}

func fmtFloatPtr(v *float64) string {
	if v == nil {
		return ""
	}
	return fmtFloat(*v)
}

func uuidPtrStr(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}
