package db

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/leoklaser/video-analyzer/api/internal/models"
)

func TestInsertAndGet(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	a := &models.Analysis{
		Status:       models.StatusProcessing,
		Mode:         models.ModePrePost,
		GCSURI:       "gs://video-analyzer-tmp/test.mp4",
		OriginalName: "test.mp4",
		BusinessContext: models.BusinessContext{
			BrandName:      "X",
			Description:    "Y",
			TargetAudience: "Z",
			Platforms:      []string{"tiktok"},
			MainPain:       "P",
			ContentHistory: "H",
		},
	}
	if err := Insert(ctx, d, a); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if a.ID.String() == "" {
		t.Fatal("ID not populated")
	}

	got, err := Get(ctx, d, a.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.GCSURI != a.GCSURI {
		t.Errorf("GCSURI: got %q want %q", got.GCSURI, a.GCSURI)
	}
	if got.BusinessContext.BrandName != "X" {
		t.Errorf("BusinessContext.BrandName: got %q", got.BusinessContext.BrandName)
	}
}

func TestUpdateProgress(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()
	a := minimalAnalysis()
	if err := Insert(ctx, d, a); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if err := UpdateProgress(ctx, d, a.ID, "Step 2..."); err != nil {
		t.Fatalf("UpdateProgress: %v", err)
	}
	got, _ := Get(ctx, d, a.ID)
	if got.ProgressMsg != "Step 2..." {
		t.Errorf("ProgressMsg: %q", got.ProgressMsg)
	}
}

func TestSetError(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()
	a := minimalAnalysis()
	_ = Insert(ctx, d, a)
	if err := SetError(ctx, d, a.ID, "boom"); err != nil {
		t.Fatalf("SetError: %v", err)
	}
	got, _ := Get(ctx, d, a.ID)
	if got.Status != models.StatusError {
		t.Errorf("Status: got %q want error", got.Status)
	}
	if got.ErrorMsg != "boom" {
		t.Errorf("ErrorMsg: %q", got.ErrorMsg)
	}
}

func TestMarkDone(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()
	a := minimalAnalysis()
	_ = Insert(ctx, d, a)
	gvi := json.RawMessage(`{"x":1}`)
	claude := json.RawMessage(`{"verdict":"ok"}`)
	if err := SetGVI(ctx, d, a.ID, gvi); err != nil {
		t.Fatalf("SetGVI: %v", err)
	}
	if err := MarkDone(ctx, d, a.ID, claude); err != nil {
		t.Fatalf("MarkDone: %v", err)
	}
	got, _ := Get(ctx, d, a.ID)
	if got.Status != models.StatusDone {
		t.Errorf("Status: %q", got.Status)
	}
	if got.CompletedAt == nil {
		t.Error("CompletedAt not set")
	}
}

func TestList(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		_ = Insert(ctx, d, minimalAnalysis())
	}
	list, err := List(ctx, d, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("len: %d want 3", len(list))
	}
}

func TestStaleProcessing(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()
	a := minimalAnalysis()
	_ = Insert(ctx, d, a)
	_, _ = d.Exec(ctx,
		"UPDATE analyses SET updated_at = now() - interval '10 minutes' WHERE id = $1",
		a.ID)

	ids, err := StaleProcessing(ctx, d, 8)
	if err != nil {
		t.Fatalf("StaleProcessing: %v", err)
	}
	if len(ids) != 1 || ids[0] != a.ID {
		t.Errorf("StaleProcessing: %v", ids)
	}
}

func minimalAnalysis() *models.Analysis {
	return &models.Analysis{
		Status: models.StatusProcessing,
		Mode:   models.ModePrePost,
		GCSURI: "gs://b/x.mp4",
		BusinessContext: models.BusinessContext{
			BrandName: "x", Description: "x", TargetAudience: "x",
			MainPain: "x", ContentHistory: "x",
			Platforms: []string{"tiktok"},
		},
	}
}
