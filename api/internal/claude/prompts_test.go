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

	msg := BuildUserMessage(models.ModePrePost, bc, nil, gvi, "")

	for _, want := range []string{
		"ScrapJobs", "Devs", "tiktok, instagram",
		"Vagas fecham rápido", "Storytelling funciona",
		"pre_post", `"shotChanges"`, "JSON válido",
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
	msg := BuildUserMessage(models.ModePostMortem, bc, metrics, json.RawMessage("{}"), "")

	if !strings.Contains(msg, "100") {
		t.Errorf("expected views in prompt")
	}
	if !strings.Contains(msg, "0.42") {
		t.Errorf("expected completion rate in prompt")
	}
	if !strings.Contains(msg, "performou bem") {
		t.Errorf("expected post-mortem verdict labels in prompt")
	}
}

func TestBuildUserMessage_Reference_WithConcept(t *testing.T) {
	bc := models.BusinessContext{BrandName: "Marca"}
	msg := BuildUserMessage(models.ModeReference, bc, nil, json.RawMessage("{}"), "quero falar sobre finanças pessoais para jovens")

	if !strings.Contains(msg, "quero falar sobre finanças pessoais para jovens") {
		t.Errorf("expected user concept in prompt")
	}
	if !strings.Contains(msg, "neuromarketing") {
		t.Errorf("expected neuromarketing instruction in reference mode prompt")
	}
	if !strings.Contains(msg, "replication_script") {
		t.Errorf("expected replication_script mention in reference mode prompt")
	}
}

func TestBuildUserMessage_Reference_NoConcept(t *testing.T) {
	bc := models.BusinessContext{BrandName: "Marca"}
	msg := BuildUserMessage(models.ModeReference, bc, nil, json.RawMessage("{}"), "")

	if strings.Contains(msg, "Conceito que o criador") {
		t.Errorf("should not include concept section when concept is empty")
	}
}

func TestBuildUserMessage_PrePost_WithConcept(t *testing.T) {
	bc := models.BusinessContext{BrandName: "Marca"}
	msg := BuildUserMessage(models.ModePrePost, bc, nil, json.RawMessage("{}"), "queria criar urgência com prazo")

	if !strings.Contains(msg, "queria criar urgência com prazo") {
		t.Errorf("expected planned hook in prompt")
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

func TestParseResult_WithNeuromarketing(t *testing.T) {
	raw := []byte(`{
	  "hook_analysis": {"score": 9, "why": "x", "improvement": "y"},
	  "structure_analysis": {"framework_match": "a", "retention_issues": []},
	  "visual_analysis": {"rhythm": "fast", "first_frame": "x", "dominant_labels": []},
	  "key_insights": ["a"],
	  "action_items": ["a"],
	  "neuromarketing_refs": ["Loop aberto", "Prova social"],
	  "viral_elements": ["Abertura com pergunta"],
	  "replication_script": "1. ...",
	  "verdict": "vai bombar",
	  "verdict_reason": "y"
	}`)

	res, err := ParseResult(raw)
	if err != nil {
		t.Fatalf("ParseResult: %v", err)
	}
	if len(res.NeuromarketingRefs) != 2 {
		t.Errorf("expected 2 neuromarketing_refs, got %d", len(res.NeuromarketingRefs))
	}
	if len(res.ViralElements) != 1 {
		t.Errorf("expected 1 viral_element, got %d", len(res.ViralElements))
	}
}

func TestParseResult_MissingRequired(t *testing.T) {
	raw := []byte(`{"hook_analysis":{"score":1,"why":"","improvement":""}}`)
	_, err := ParseResult(raw)
	if err == nil {
		t.Fatal("expected error on missing required fields")
	}
}
