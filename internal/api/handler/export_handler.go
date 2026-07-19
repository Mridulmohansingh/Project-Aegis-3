// Package handler provides HTTP endpoints for CSV export/download.
package handler

import (
	"fmt"
	"net/http"

	"go.uber.org/zap"

	"github.com/aegis-platform/aegis/pkg/logging"
)

// ExportHandler handles CSV export requests.
type ExportHandler struct {
	logger *zap.Logger
}

// NewExportHandler creates a new export handler.
func NewExportHandler(logger *zap.Logger) *ExportHandler {
	return &ExportHandler{logger: logger.With(zap.String("component", "export_handler"))}
}

// RegisterRoutes registers export endpoints.
func (h *ExportHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/export/items", h.ExportItems)
	mux.HandleFunc("GET /api/v1/export/exams/{id}/responses", h.ExportResponseMatrix)
	mux.HandleFunc("GET /api/v1/export/exams/{id}/statistics", h.ExportClassicalStats)
	mux.HandleFunc("GET /api/v1/export/exams/{id}/scores", h.ExportScores)
	mux.HandleFunc("GET /api/v1/export/exams/{id}/dif", h.ExportDIF)
	mux.HandleFunc("GET /api/v1/export/exams/{id}/person-fit", h.ExportPersonFit)
	mux.HandleFunc("GET /api/v1/export/exams/{id}/summary", h.ExportSummary)
	mux.HandleFunc("GET /api/v1/export/exams/{id}/all", h.ExportAll)
}

// ExportItems streams the item bank as CSV.
func (h *ExportHandler) ExportItems(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())
	log.Info("exporting item bank CSV")

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="item_bank.csv"`)

	// In production: fetch items from database, call CSVExporter.ExportItemBank
	// For now, return a sample
	w.Write([]byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM
	fmt.Fprintln(w, "item_id,external_id,subject_id,item_type,status,irt_a,irt_b,irt_c,p_value,discrimination")
	fmt.Fprintln(w, "550e8400-e29b-41d4-a716-446655440001,PHY-2026-00142,Physics,MCQ_SINGLE,ACTIVE,1.85,1.20,0.18,0.38,0.62")
	fmt.Fprintln(w, "550e8400-e29b-41d4-a716-446655440002,CHM-2026-00089,Chemistry,MCQ_SINGLE,ACTIVE,1.45,0.30,0.22,0.55,0.48")
}

// ExportResponseMatrix streams the response matrix as CSV.
func (h *ExportHandler) ExportResponseMatrix(w http.ResponseWriter, r *http.Request) {
	examID := r.PathValue("id")
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="response_matrix_%s.csv"`, examID))
	w.Write([]byte{0xEF, 0xBB, 0xBF})
	fmt.Fprintln(w, "candidate_id,item_001,item_002,item_003,total_score,total_time_ms")
	fmt.Fprintln(w, "cand-001,1,0,1,2,45000")
	fmt.Fprintln(w, "cand-002,1,1,0,2,52000")
}

// ExportClassicalStats streams classical item statistics as CSV.
func (h *ExportHandler) ExportClassicalStats(w http.ResponseWriter, r *http.Request) {
	examID := r.PathValue("id")
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="classical_stats_%s.csv"`, examID))
	w.Write([]byte{0xEF, 0xBB, 0xBF})
	fmt.Fprintln(w, "item_id,total_responses,p_value,discrimination_index,point_biserial,flagged,flag_reasons")
}

// ExportScores streams candidate scores as CSV.
func (h *ExportHandler) ExportScores(w http.ResponseWriter, r *http.Request) {
	examID := r.PathValue("id")
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="candidate_scores_%s.csv"`, examID))
	w.Write([]byte{0xEF, 0xBB, 0xBF})
	fmt.Fprintln(w, "candidate_id,raw_score,theta_mle,theta_eap,scaled_score,percentile,rank")
}

// ExportDIF streams DIF analysis results as CSV.
func (h *ExportHandler) ExportDIF(w http.ResponseWriter, r *http.Request) {
	examID := r.PathValue("id")
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="dif_analysis_%s.csv"`, examID))
	w.Write([]byte{0xEF, 0xBB, 0xBF})
	fmt.Fprintln(w, "item_id,grouping_variable,delta_mh,chi_square,p_value,ets_category,flagged")
}

// ExportPersonFit streams person-fit analysis as CSV.
func (h *ExportHandler) ExportPersonFit(w http.ResponseWriter, r *http.Request) {
	examID := r.PathValue("id")
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="person_fit_%s.csv"`, examID))
	w.Write([]byte{0xEF, 0xBB, 0xBF})
	fmt.Fprintln(w, "candidate_id,theta,lz_statistic,lz_p_value,flagged,flag_reason")
}

// ExportSummary streams exam summary as CSV.
func (h *ExportHandler) ExportSummary(w http.ResponseWriter, r *http.Request) {
	examID := r.PathValue("id")
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="exam_summary_%s.csv"`, examID))
	w.Write([]byte{0xEF, 0xBB, 0xBF})
	fmt.Fprintln(w, "exam_id,total_appeared,mean_raw,median_raw,std_raw,cronbach_alpha,mean_theta,std_theta")
}

// ExportAll generates a ZIP containing all CSV files.
func (h *ExportHandler) ExportAll(w http.ResponseWriter, r *http.Request) {
	examID := r.PathValue("id")
	// In production: generate all CSVs, zip them, stream
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="aegis_export_%s.zip"`, examID))
	w.WriteHeader(http.StatusOK)
}
