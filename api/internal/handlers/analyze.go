package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/leoklaser/video-analyzer/api/internal/db"
	"github.com/leoklaser/video-analyzer/api/internal/jobs"
	"github.com/leoklaser/video-analyzer/api/internal/models"
)

type AnalyzeHandler struct {
	DB     *db.DB
	Bucket string
	Runner *jobs.Runner
}

type startRequest struct {
	GCSURI          string                 `json:"gcs_uri"`
	OriginalName    string                 `json:"original_name"`
	Mode            string                 `json:"mode"`
	BusinessContext models.BusinessContext `json:"business_context"`
	Metrics         *models.Metrics        `json:"metrics,omitempty"`
}

func (h *AnalyzeHandler) Start(w http.ResponseWriter, r *http.Request) {
	var req startRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := h.validate(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	a := &models.Analysis{
		Status:          models.StatusProcessing,
		Mode:            models.Mode(req.Mode),
		GCSURI:          req.GCSURI,
		OriginalName:    req.OriginalName,
		UserID:          userIDFromCtx(r),
		BusinessContext: req.BusinessContext,
		MetricsInput:    req.Metrics,
	}
	if err := db.Insert(r.Context(), h.DB, a); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create analysis: "+err.Error())
		return
	}
	_ = db.UpdateProgress(r.Context(), h.DB, a.ID, "Iniciando análise...")

	if h.Runner != nil {
		go h.Runner.Run(context.Background(), a.ID)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":     a.ID,
		"status": a.Status,
	})
}

func (h *AnalyzeHandler) validate(req *startRequest) error {
	expectedPrefix := "gs://" + h.Bucket + "/"
	if !strings.HasPrefix(req.GCSURI, expectedPrefix) {
		return fmt.Errorf("gcs_uri must start with %s", expectedPrefix)
	}
	switch req.Mode {
	case "pre_post", "reference", "post_mortem":
	default:
		return fmt.Errorf("mode must be pre_post|reference|post_mortem (got %q)", req.Mode)
	}
	bc := req.BusinessContext
	missing := []string{}
	if strings.TrimSpace(bc.BrandName) == "" {
		missing = append(missing, "brand_name")
	}
	if strings.TrimSpace(bc.Description) == "" {
		missing = append(missing, "description")
	}
	if strings.TrimSpace(bc.TargetAudience) == "" {
		missing = append(missing, "target_audience")
	}
	if strings.TrimSpace(bc.MainPain) == "" {
		missing = append(missing, "main_pain")
	}
	if strings.TrimSpace(bc.ContentHistory) == "" {
		missing = append(missing, "content_history")
	}
	if len(missing) > 0 {
		return fmt.Errorf("business_context missing: %s", strings.Join(missing, ", "))
	}
	if req.Metrics != nil && req.Mode != "post_mortem" && req.Mode != "reference" {
		return fmt.Errorf("metrics only allowed when mode = post_mortem or reference")
	}
	return nil
}

func (h *AnalyzeHandler) Get(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	a, err := db.Get(r.Context(), h.DB, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := map[string]any{
		"id":           a.ID,
		"status":       a.Status,
		"mode":         a.Mode,
		"progress_msg": a.ProgressMsg,
		"created_at":   a.CreatedAt,
	}
	if a.CompletedAt != nil {
		resp["completed_at"] = a.CompletedAt
	}
	switch a.Status {
	case models.StatusDone:
		resp["result"] = json.RawMessage(a.ClaudeResult)
	case models.StatusError:
		resp["error"] = a.ErrorMsg
	}
	writeJSON(w, http.StatusOK, resp)
}
