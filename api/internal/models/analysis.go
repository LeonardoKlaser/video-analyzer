package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusProcessing Status = "processing"
	StatusDone       Status = "done"
	StatusError      Status = "error"
)

type Mode string

const (
	ModePrePost    Mode = "pre_post"
	ModeReference  Mode = "reference"
	ModePostMortem Mode = "post_mortem"
)

type BusinessContext struct {
	BrandName      string   `json:"brand_name"`
	Description    string   `json:"description"`
	TargetAudience string   `json:"target_audience"`
	Platforms      []string `json:"platforms"`
	MainPain       string   `json:"main_pain"`
	ContentHistory string   `json:"content_history"`
}

type Metrics struct {
	Views           *int     `json:"views,omitempty"`
	Likes           *int     `json:"likes,omitempty"`
	AvgWatchTime    *float64 `json:"avg_watch_time,omitempty"`
	CompletionRate  *float64 `json:"completion_rate,omitempty"`
	FollowersGained *int     `json:"followers_gained,omitempty"`
}

type Analysis struct {
	ID              uuid.UUID       `json:"id"`
	Status          Status          `json:"status"`
	Mode            Mode            `json:"mode"`
	GCSURI          string          `json:"gcs_uri"`
	OriginalName    string          `json:"original_name,omitempty"`
	UserID          uuid.UUID       `json:"-"`
	BusinessContext BusinessContext `json:"business_context"`
	MetricsInput    *Metrics        `json:"metrics_input,omitempty"`
	GVIResult       json.RawMessage `json:"gvi_result,omitempty"`
	ClaudeResult    json.RawMessage `json:"claude_result,omitempty"`
	ProgressMsg     string          `json:"progress_msg,omitempty"`
	ErrorMsg        string          `json:"error_msg,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	CompletedAt     *time.Time      `json:"completed_at,omitempty"`
}
