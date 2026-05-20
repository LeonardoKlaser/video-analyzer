package claude

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/leoklaser/video-analyzer/api/internal/models"
)

func TestSystemPrompt_NotEmpty(t *testing.T) {
	sp := SystemPrompt()
	if len(sp) < 1000 {
		t.Fatalf("system prompt suspiciously short: %d chars", len(sp))
	}
	if !strings.Contains(sp, "Analista") {
		t.Errorf("expected content-strategy.md to be embedded")
	}
}

func TestBuildUserMessage_Pre_Post(t *testing.T) {
	bc := models.BusinessContext{
		BrandName:      "ScrapJobs",
		Description:    "Motor de busca",
		TargetAudience: "Devs",
		Platforms:      []string{"tiktok", "instagram"},
		MainPain:       "Vagas fecham rápido",
		ContentHistory: "Storytelling funciona",
	}
	gvi := json.RawMessage(`{"shotChanges":{"total":5}}`)

	msg := BuildUserMessage(models.ModePrePost, bc, nil, gvi)

	for _, want := range []string{
		"ScrapJobs",
		"Devs",
		"tiktok, instagram",
		"Vagas fecham rápido",
		"Storytelling funciona",
		"pre_post",
		`"shotChanges"`,
		"JSON válido",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("expected substring %q not found", want)
		}
	}
}

func TestBuildUserMessage_PostMortem_WithMetrics(t *testing.T) {
	bc := models.BusinessContext{BrandName: "X"}
	views := 100
	completion := 0.42
	metrics := &models.Metrics{Views: &views, CompletionRate: &completion}
	msg := BuildUserMessage(models.ModePostMortem, bc, metrics, json.RawMessage("{}"))

	if !strings.Contains(msg, "100") {
		t.Errorf("expected views in prompt")
	}
	if !strings.Contains(msg, "0.42") {
		t.Errorf("expected completion rate in prompt")
	}
}

func TestParseResult_Happy(t *testing.T) {
	raw := []byte(`{
	  "hook_analysis": {"score": 8, "why": "x", "improvement": "y"},
	  "structure_analysis": {"framework_match": "hook→...", "retention_issues": ["a"]},
	  "visual_analysis": {"rhythm": "fast", "first_frame": "x", "dominant_labels": ["a"]},
	  "key_insights": ["a","b","c"],
	  "action_items": ["a","b"],
	  "verdict": "ok",
	  "verdict_reason": "y"
	}`)

	res, err := ParseResult(raw)
	if err != nil {
		t.Fatalf("ParseResult: %v", err)
	}
	if res.HookAnalysis.Score != 8 {
		t.Errorf("Score: %d", res.HookAnalysis.Score)
	}
	if res.Verdict != "ok" {
		t.Errorf("Verdict: %q", res.Verdict)
	}
}

func TestParseResult_MissingRequired(t *testing.T) {
	raw := []byte(`{"hook_analysis":{"score":1,"why":"","improvement":""}}`)
	_, err := ParseResult(raw)
	if err == nil {
		t.Fatal("expected error on missing required fields")
	}
}
