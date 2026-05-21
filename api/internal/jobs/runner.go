// Package jobs orchestrates the per-analysis goroutine: analyzer subprocess,
// Claude API call, DB state transitions, and GCS cleanup.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/leoklaser/video-analyzer/api/internal/analyzer"
	"github.com/leoklaser/video-analyzer/api/internal/claude"
	"github.com/leoklaser/video-analyzer/api/internal/db"
	"github.com/leoklaser/video-analyzer/api/internal/gcs"
)

type Runner struct {
	DB       *db.DB
	Analyzer *analyzer.Runner
	Claude   *claude.Client
	GCS      *gcs.Client
}

// Run executes the full pipeline for the given analysis id.
// Designed to be called as `go r.Run(ctx, id)` from the handler.
func (r *Runner) Run(ctx context.Context, id uuid.UUID) {
	jobCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	a, err := db.Get(jobCtx, r.DB, id)
	if err != nil {
		slog.Error("job: get analysis", "id", id, "err", err)
		return
	}
	gcsURI := a.GCSURI

	defer func() {
		if err := r.GCS.Delete(jobCtx, gcsURI); err != nil {
			slog.Warn("job: gcs delete failed (lifecycle rule will catch it)", "uri", gcsURI, "err", err)
		}
	}()

	if err := db.UpdateProgress(jobCtx, r.DB, id, "Analisando estrutura visual..."); err != nil {
		slog.Error("job: update progress", "id", id, "err", err)
		return
	}

	gvi, err := r.Analyzer.Run(jobCtx, gcsURI)
	if err != nil {
		slog.Error("job: analyzer failed", "id", id, "err", err)
		_ = db.SetError(jobCtx, r.DB, id, friendlyAnalyzerError(err))
		return
	}
	if err := db.SetGVI(jobCtx, r.DB, id, gvi); err != nil {
		slog.Error("job: save gvi", "id", id, "err", err)
		_ = db.SetError(jobCtx, r.DB, id, "Falha ao salvar dados de análise.")
		return
	}

	if err := db.UpdateProgress(jobCtx, r.DB, id, "Gerando insights com IA..."); err != nil {
		slog.Error("job: update progress 2", "id", id, "err", err)
	}

	user := claude.BuildUserMessage(a.Mode, a.BusinessContext, a.MetricsInput, gvi, a.UserConcept)
	raw, err := r.Claude.Analyze(jobCtx, claude.SystemPrompt(), user)
	if err != nil {
		slog.Error("job: claude failed", "id", id, "err", err)
		_ = db.SetError(jobCtx, r.DB, id, "Falha ao gerar insights. Tente novamente.")
		return
	}

	result, err := claude.ParseResult(raw)
	if err != nil {
		slog.Warn("job: claude json invalid, retrying", "id", id, "err", err, "raw_prefix", string(raw[:min(len(raw), 200)]))
		raw, err = r.Claude.Analyze(jobCtx, claude.SystemPrompt(), user+"\n\nATENÇÃO: sua resposta anterior foi rejeitada por não ser JSON válido. Responda APENAS o objeto JSON, sem markdown, sem texto adicional.")
		if err != nil {
			_ = db.SetError(jobCtx, r.DB, id, "Falha ao gerar insights. Tente novamente.")
			return
		}
		result, err = claude.ParseResult(raw)
		if err != nil {
			slog.Error("job: claude json invalid after retry", "id", id, "err", err, "raw_prefix", string(raw[:min(len(raw), 200)]))
			_ = db.SetError(jobCtx, r.DB, id, "Resposta da IA inválida após nova tentativa.")
			return
		}
	}

	rawNormalized, _ := result.AsRaw()
	if err := db.MarkDone(jobCtx, r.DB, id, rawNormalized); err != nil {
		slog.Error("job: mark done", "id", id, "err", err)
		_ = db.SetError(jobCtx, r.DB, id, "Falha ao salvar resultado.")
		return
	}
	slog.Info("job: done", "id", id)
}

func friendlyAnalyzerError(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "Análise demorou demais. Tente um vídeo menor."
	}
	return fmt.Sprintf("Falha ao analisar vídeo: %s", err.Error())
}
