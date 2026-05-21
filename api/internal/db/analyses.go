package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/leoklaser/video-analyzer/api/internal/models"
)

func Insert(ctx context.Context, d *DB, a *models.Analysis) error {
	bc, err := json.Marshal(a.BusinessContext)
	if err != nil {
		return fmt.Errorf("marshal business_context: %w", err)
	}
	var mi []byte
	if a.MetricsInput != nil {
		mi, err = json.Marshal(a.MetricsInput)
		if err != nil {
			return fmt.Errorf("marshal metrics_input: %w", err)
		}
	}
	var userID *uuid.UUID
	if a.UserID != uuid.Nil {
		userID = &a.UserID
	}
	var userConcept *string
	if a.UserConcept != "" {
		userConcept = &a.UserConcept
	}
	row := d.QueryRow(ctx, `
		INSERT INTO analyses (status, mode, gcs_uri, original_name, user_id, business_context, metrics_input, user_concept)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at`,
		a.Status, a.Mode, a.GCSURI, a.OriginalName, userID, bc, mi, userConcept,
	)
	return row.Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt)
}

func Get(ctx context.Context, d *DB, id uuid.UUID) (*models.Analysis, error) {
	row := d.QueryRow(ctx, `
		SELECT id, status, mode, gcs_uri, COALESCE(original_name,''),
		       business_context, metrics_input,
		       gvi_result, claude_result,
		       COALESCE(progress_msg,''), COALESCE(error_msg,''),
		       COALESCE(user_concept,''),
		       created_at, updated_at, completed_at
		FROM analyses WHERE id = $1`, id)

	var a models.Analysis
	var bc []byte
	var mi []byte
	if err := row.Scan(
		&a.ID, &a.Status, &a.Mode, &a.GCSURI, &a.OriginalName,
		&bc, &mi,
		&a.GVIResult, &a.ClaudeResult,
		&a.ProgressMsg, &a.ErrorMsg,
		&a.UserConcept,
		&a.CreatedAt, &a.UpdatedAt, &a.CompletedAt,
	); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(bc, &a.BusinessContext); err != nil {
		return nil, fmt.Errorf("unmarshal business_context: %w", err)
	}
	if len(mi) > 0 {
		a.MetricsInput = &models.Metrics{}
		if err := json.Unmarshal(mi, a.MetricsInput); err != nil {
			return nil, fmt.Errorf("unmarshal metrics_input: %w", err)
		}
	}
	return &a, nil
}

func UpdateProgress(ctx context.Context, d *DB, id uuid.UUID, msg string) error {
	_, err := d.Exec(ctx, `
		UPDATE analyses SET progress_msg = $2, updated_at = now() WHERE id = $1`,
		id, msg)
	return err
}

func SetGVI(ctx context.Context, d *DB, id uuid.UUID, gvi json.RawMessage) error {
	_, err := d.Exec(ctx, `
		UPDATE analyses SET gvi_result = $2, updated_at = now() WHERE id = $1`,
		id, []byte(gvi))
	return err
}

func MarkDone(ctx context.Context, d *DB, id uuid.UUID, claude json.RawMessage) error {
	_, err := d.Exec(ctx, `
		UPDATE analyses
		SET claude_result = $2, status = 'done', completed_at = now(), updated_at = now(),
		    progress_msg = ''
		WHERE id = $1`,
		id, []byte(claude))
	return err
}

func SetError(ctx context.Context, d *DB, id uuid.UUID, msg string) error {
	_, err := d.Exec(ctx, `
		UPDATE analyses SET status = 'error', error_msg = $2, updated_at = now()
		WHERE id = $1`, id, msg)
	return err
}

type ListItem struct {
	ID           uuid.UUID `json:"id"`
	Mode         string    `json:"mode"`
	Status       string    `json:"status"`
	OriginalName string    `json:"original_name,omitempty"`
	Verdict      string    `json:"verdict,omitempty"`
	CreatedAt    string    `json:"created_at"`
}

func List(ctx context.Context, d *DB, userID uuid.UUID, limit int) ([]ListItem, error) {
	rows, err := d.Query(ctx, `
		SELECT id, mode, status, COALESCE(original_name,''),
		       COALESCE(claude_result->>'verdict',''),
		       to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM analyses
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ListItem{}
	for rows.Next() {
		var it ListItem
		if err := rows.Scan(&it.ID, &it.Mode, &it.Status, &it.OriginalName, &it.Verdict, &it.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func StaleProcessing(ctx context.Context, d *DB, minutesOld int) ([]uuid.UUID, error) {
	rows, err := d.Query(ctx, fmt.Sprintf(`
		SELECT id FROM analyses
		WHERE status = 'processing'
		  AND updated_at < now() - interval '%d minutes'`, minutesOld))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
