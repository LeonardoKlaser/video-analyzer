# UX Overhaul Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Mode-specific analysis forms, profile page for business context, deepened reference mode with neuromarketing output, post-mortem verdict fix.

**Architecture:** Backend adds `user_concept` to DB + Analysis model and updates Claude prompt builder; frontend replaces the monolithic AnalysisForm with a shell + three mode-specific sub-forms, a new ProfilePage component, and an enriched AnalysisResult for reference/post-mortem modes.

**Tech Stack:** Go (backend), React/TypeScript (frontend), PostgreSQL (new `user_concept TEXT` column)

---

## File Map

**New files:**
- `web/src/components/FormPrimitives.tsx` — shared form UI (VideoUpload, SectionTitle, Num, SubmitButton)
- `web/src/components/ReferenceForm.tsx` — reference mode sub-form
- `web/src/components/PrePostForm.tsx` — pre-post sub-form
- `web/src/components/PostMortemForm.tsx` — post-mortem sub-form
- `web/src/components/ProfilePage.tsx` — business context settings page

**Modified files:**
- `api/internal/db/init.sql` — add `user_concept TEXT` column migration
- `api/internal/models/analysis.go` — add `UserConcept string` field
- `api/internal/db/analyses.go` — persist + scan `user_concept`
- `api/internal/claude/types.go` — add `NeuromarketingRefs`, `ViralElements`
- `api/internal/claude/prompts.go` — updated mode descriptions + `userConcept` param
- `api/internal/claude/prompts_test.go` — new tests for reference/pre-post with concept
- `api/internal/handlers/analyze.go` — add `UserConcept` to request, relax BC validation
- `api/internal/handlers/analyze_test.go` — remove brand_name required test
- `api/internal/jobs/runner.go` — pass `a.UserConcept` to `BuildUserMessage`
- `web/src/types.ts` — `user_concept` in request, new Claude result fields, post-mortem verdicts
- `web/src/components/AnalysisForm.tsx` — refactor into shell rendering sub-forms
- `web/src/components/AnalysesSidebar.tsx` — add Configurações link, post-mortem verdict colors
- `web/src/components/AnalysisResult.tsx` — neuromarketing section, post-mortem verdicts
- `web/src/App.tsx` — profile view, first-run banner, wire userConcept

---

### Task 1: DB schema + Analysis model

**Files:**
- Modify: `api/internal/db/init.sql`
- Modify: `api/internal/models/analysis.go`
- Modify: `api/internal/db/analyses.go`

- [ ] **Step 1: Add migration to init.sql**

Append to the end of `/workspace/api/internal/db/init.sql`:
```sql
-- migrate: add user_concept for async job runner access
ALTER TABLE analyses ADD COLUMN IF NOT EXISTS user_concept TEXT;
```

- [ ] **Step 2: Add UserConcept to Analysis model**

In `/workspace/api/internal/models/analysis.go`, add the field after `MetricsInput`:
```go
MetricsInput    *Metrics        `json:"metrics_input,omitempty"`
UserConcept     string          `json:"user_concept,omitempty"`
```

- [ ] **Step 3: Update db.Insert to persist user_concept**

In `/workspace/api/internal/db/analyses.go`, replace the `Insert` query:
```go
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
```

- [ ] **Step 4: Update db.Get to scan user_concept**

In `/workspace/api/internal/db/analyses.go`, replace the `Get` function:
```go
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
```

- [ ] **Step 5: Run existing DB tests to verify no regression**

```bash
cd /workspace/api && go test ./internal/db/... -v -count=1 2>&1 | tail -20
```
Expected: all tests PASS (tests use real Postgres via docker-compose).

- [ ] **Step 6: Commit**

```bash
git add api/internal/db/init.sql api/internal/models/analysis.go api/internal/db/analyses.go
git commit -m "feat: add user_concept column to analyses + model"
```

---

### Task 2: Claude types + prompts

**Files:**
- Modify: `api/internal/claude/types.go`
- Modify: `api/internal/claude/prompts.go`
- Modify: `api/internal/claude/prompts_test.go`

- [ ] **Step 1: Write failing tests**

Replace the contents of `api/internal/claude/prompts_test.go` with:
```go
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
```

- [ ] **Step 2: Run tests — expect failures on new tests**

```bash
cd /workspace/api && go test ./internal/claude/... -v -run "TestBuildUserMessage_Reference|TestBuildUserMessage_PrePost_WithConcept|TestBuildUserMessage_PostMortem|TestParseResult_WithNeuro" 2>&1 | tail -20
```
Expected: FAIL (BuildUserMessage signature mismatch, missing fields).

- [ ] **Step 3: Update claude/types.go**

Replace `/workspace/api/internal/claude/types.go`:
```go
package claude

import "encoding/json"

// Result is the structured JSON we ask Claude to produce.
type Result struct {
	HookAnalysis struct {
		Score       int    `json:"score"`
		Why         string `json:"why"`
		Improvement string `json:"improvement"`
	} `json:"hook_analysis"`

	StructureAnalysis struct {
		FrameworkMatch  string   `json:"framework_match"`
		RetentionIssues []string `json:"retention_issues"`
	} `json:"structure_analysis"`

	VisualAnalysis struct {
		Rhythm         string   `json:"rhythm"`
		FirstFrame     string   `json:"first_frame"`
		DominantLabels []string `json:"dominant_labels"`
	} `json:"visual_analysis"`

	KeyInsights       []string `json:"key_insights"`
	ActionItems       []string `json:"action_items"`
	ReplicationScript string   `json:"replication_script,omitempty"`
	NeuromarketingRefs []string `json:"neuromarketing_refs,omitempty"`
	ViralElements      []string `json:"viral_elements,omitempty"`
	Verdict           string   `json:"verdict"`
	VerdictReason     string   `json:"verdict_reason"`
}

func (r *Result) AsRaw() (json.RawMessage, error) {
	return json.Marshal(r)
}
```

- [ ] **Step 4: Update claude/prompts.go**

Replace `/workspace/api/internal/claude/prompts.go`:
```go
package claude

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/leoklaser/video-analyzer/api/internal/models"
)

//go:embed content-strategy.md
var contentStrategyMD string

func SystemPrompt() string { return contentStrategyMD }

func modeDescription(m models.Mode) string {
	switch m {
	case models.ModePrePost:
		return "Vídeo ainda não postado. Avalie se vale postar como está. Foco: hook nos primeiros 3s, estrutura, ritmo de corte, CTA. Pergunta central: 'vale a pena postar?'."
	case models.ModeReference:
		return `Vídeo viral de terceiro usado como referência. Analise em três camadas:

1. POR QUE VIRALIZOU — identifique os mecanismos psicológicos e de neuromarketing que explicam a performance (ex: loop aberto, prova social, contraste, escassez, identidade tribal). Seja específico: qual frame/fala ativa cada princípio.

2. ELEMENTOS VIRAIS — liste os elementos concretos replicáveis (estrutura de hook, ritmo de corte, tipo de abertura, uso de texto na tela, CTA implícito/explícito).

3. ROTEIRO PERSONALIZADO — usando o conceito declarado pelo usuário e o contexto do negócio dele, escreva um roteiro completo que incorpora os elementos virais identificados. Justifique cada escolha estrutural com o princípio de neuromarketing correspondente.

Os campos neuromarketing_refs, viral_elements e replication_script são OBRIGATÓRIOS neste modo.`
	case models.ModePostMortem:
		return `Vídeo já postado. Diagnóstico do que funcionou ou não. Compare métricas com benchmarks (Caso 4 do system prompt). Foco: aprendizado pra próximos. Para o campo "verdict", use EXCLUSIVAMENTE: "performou bem" | "na média" | "abaixo do esperado".`
	}
	return string(m)
}

func formatMetrics(m *models.Metrics, mode models.Mode) string {
	if m == nil {
		return "(não fornecidas)"
	}
	var b strings.Builder
	if m.Views != nil {
		fmt.Fprintf(&b, "- Views: %d\n", *m.Views)
	}
	if m.Likes != nil {
		fmt.Fprintf(&b, "- Likes: %d\n", *m.Likes)
	}
	if m.AvgWatchTime != nil {
		fmt.Fprintf(&b, "- Avg watch time (s): %g\n", *m.AvgWatchTime)
	}
	if m.CompletionRate != nil {
		fmt.Fprintf(&b, "- Completion rate: %g\n", *m.CompletionRate)
	}
	if m.FollowersGained != nil {
		fmt.Fprintf(&b, "- Followers ganhos: %d\n", *m.FollowersGained)
	}
	if b.Len() == 0 {
		return "(não fornecidas)"
	}
	if mode == models.ModeReference {
		return "Performance do vídeo de referência (use para calibrar o peso da análise):\n" + b.String()
	}
	return b.String()
}

const outputSchemaInstruction = `Responda APENAS em JSON válido, sem texto antes ou depois, sem markdown.
Os dados do vídeo incluem:
- "speech": transcrição de áudio via Whisper. Use "speech.hookText" para analisar o hook verbal nos primeiros 5s e "speech.fullTranscript" para avaliar estrutura narrativa, CTA e ritmo. Se null, o vídeo não tem fala detectável.
- "textDetection": texto visível na tela (overlays, legendas, CTAs visuais). Use "textDetection.hookTexts" para analisar o hook textual nos primeiros 5s.
Estrutura obrigatória (campos com nomes EXATOS):
{
  "hook_analysis": { "score": 1-10, "why": "...", "improvement": "..." },
  "structure_analysis": { "framework_match": "...", "retention_issues": ["..."] },
  "visual_analysis": { "rhythm": "...", "first_frame": "...", "dominant_labels": ["..."] },
  "key_insights": ["...", "...", "..."],
  "action_items": ["...", "..."],
  "replication_script": "...",
  "neuromarketing_refs": ["..."],
  "viral_elements": ["..."],
  "verdict": "vai bombar" | "ok" | "vai flopar" | "performou bem" | "na média" | "abaixo do esperado",
  "verdict_reason": "..."
}`

func BuildUserMessage(mode models.Mode, bc models.BusinessContext, metrics *models.Metrics, gvi json.RawMessage, userConcept string) string {
	platforms := strings.Join(bc.Platforms, ", ")
	if platforms == "" {
		platforms = "(não informadas)"
	}

	conceptSection := ""
	if userConcept != "" {
		switch mode {
		case models.ModeReference:
			conceptSection = fmt.Sprintf("\n## Conceito que o criador quer gravar\n%s\n", userConcept)
		case models.ModePrePost:
			conceptSection = fmt.Sprintf("\n## Conceito/gancho planejado pelo criador\n%s\nAvalie se o hook executado bate com essa intenção.\n", userConcept)
		}
	}

	return fmt.Sprintf(`## Contexto do negócio
- Marca: %s
- O que faz: %s
- Público-alvo: %s
- Plataformas: %s
- Dor do cliente: %s
- O que já funcionou (histórico): %s

## Modo de análise
%s (slug: %s)

## Métricas
%s
%s
## Dados extraídos do vídeo (Google Video Intelligence)
%s

---

%s`,
		bc.BrandName,
		bc.Description,
		bc.TargetAudience,
		platforms,
		bc.MainPain,
		bc.ContentHistory,
		modeDescription(mode),
		string(mode),
		formatMetrics(metrics, mode),
		conceptSection,
		string(gvi),
		outputSchemaInstruction,
	)
}

func ParseResult(raw []byte) (*Result, error) {
	var r Result
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("unmarshal claude json: %w", err)
	}
	if r.Verdict == "" {
		return nil, fmt.Errorf("missing required field 'verdict'")
	}
	if r.HookAnalysis.Why == "" {
		return nil, fmt.Errorf("missing required field 'hook_analysis.why'")
	}
	if len(r.KeyInsights) == 0 {
		return nil, fmt.Errorf("missing required field 'key_insights'")
	}
	if len(r.ActionItems) == 0 {
		return nil, fmt.Errorf("missing required field 'action_items'")
	}
	return &r, nil
}
```

- [ ] **Step 5: Run all claude tests**

```bash
cd /workspace/api && go test ./internal/claude/... -v -count=1 2>&1 | tail -30
```
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add api/internal/claude/types.go api/internal/claude/prompts.go api/internal/claude/prompts_test.go
git commit -m "feat: neuromarketing output fields + enriched reference/post-mortem prompts"
```

---

### Task 3: Handler + runner

**Files:**
- Modify: `api/internal/handlers/analyze.go`
- Modify: `api/internal/handlers/analyze_test.go`
- Modify: `api/internal/jobs/runner.go`

- [ ] **Step 1: Write failing test**

Replace the test cases slice in `api/internal/handlers/analyze_test.go`:
```go
package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStart_ValidationErrors(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"empty body", ``, "invalid JSON"},
		{"missing gcs_uri", `{"mode":"pre_post","business_context":{}}`, "gcs_uri"},
		{"wrong bucket", `{"gcs_uri":"gs://other-bucket/x.mp4","mode":"pre_post","business_context":{}}`, "gcs_uri"},
		{"bad mode", `{"gcs_uri":"gs://video-analyzer-tmp/x.mp4","mode":"weird","business_context":{}}`, "mode"},
		{"metrics without post_mortem", `{"gcs_uri":"gs://video-analyzer-tmp/x.mp4","mode":"pre_post","business_context":{},"metrics":{"views":1}}`, "metrics"},
	}
	h := &AnalyzeHandler{Bucket: "video-analyzer-tmp"}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/analyze", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			h.Start(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status: got %d want 400 (body: %s)", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tc.want) {
				t.Errorf("body missing %q: %s", tc.want, rec.Body.String())
			}
		})
	}
}
```

- [ ] **Step 2: Run test — expect FAIL on "missing gcs_uri" (brand_name now optional)**

```bash
cd /workspace/api && go test ./internal/handlers/... -v -run TestStart_ValidationErrors 2>&1
```
Expected: FAIL because handler still requires brand_name.

- [ ] **Step 3: Update analyze.go**

Replace `/workspace/api/internal/handlers/analyze.go`:
```go
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
	UserConcept     string                 `json:"user_concept,omitempty"`
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
		UserConcept:     req.UserConcept,
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
```

- [ ] **Step 4: Run handler tests**

```bash
cd /workspace/api && go test ./internal/handlers/... -v -run TestStart_ValidationErrors 2>&1
```
Expected: all PASS.

- [ ] **Step 5: Update runner.go to pass UserConcept**

In `/workspace/api/internal/jobs/runner.go`, find the line:
```go
user := claude.BuildUserMessage(a.Mode, a.BusinessContext, a.MetricsInput, gvi)
```
Replace with:
```go
user := claude.BuildUserMessage(a.Mode, a.BusinessContext, a.MetricsInput, gvi, a.UserConcept)
```

- [ ] **Step 6: Build to verify no compile errors**

```bash
cd /workspace/api && go build ./... 2>&1
```
Expected: no output (clean build).

- [ ] **Step 7: Run all backend tests**

```bash
cd /workspace/api && go test ./... -count=1 2>&1 | tail -20
```
Expected: all PASS (DB tests require docker-compose postgres to be up).

- [ ] **Step 8: Commit**

```bash
git add api/internal/handlers/analyze.go api/internal/handlers/analyze_test.go api/internal/jobs/runner.go
git commit -m "feat: handler accepts user_concept, relax BC validation, runner passes concept to prompt"
```

---

### Task 4: TypeScript types

**Files:**
- Modify: `web/src/types.ts`

- [ ] **Step 1: Update types.ts**

Replace `/workspace/web/src/types.ts`:
```typescript
export type Mode = 'pre_post' | 'reference' | 'post_mortem';

export interface User {
  id: string;
  email: string;
  business_context?: BusinessContext;
  created_at: string;
}

export interface AuthResponse {
  token: string;
  user: User;
}

export type Platform = 'tiktok' | 'instagram' | 'youtube' | 'other';

export interface BusinessContext {
  brand_name: string;
  description: string;
  target_audience: string;
  platforms: Platform[];
  main_pain: string;
  content_history: string;
}

export interface Metrics {
  views?: number;
  likes?: number;
  avg_watch_time?: number;
  completion_rate?: number;
  followers_gained?: number;
}

export interface SignedURLResponse {
  put_url: string;
  gcs_uri: string;
  expires_at: string;
}

export interface StartAnalyzeRequest {
  gcs_uri: string;
  original_name: string;
  mode: Mode;
  business_context: BusinessContext;
  metrics?: Metrics;
  user_concept?: string;
}

export interface AnalysisStatus {
  id: string;
  status: 'processing' | 'done' | 'error';
  mode: Mode;
  progress_msg: string;
  created_at: string;
  completed_at?: string;
  result?: ClaudeResult;
  error?: string;
}

export interface ClaudeResult {
  hook_analysis: { score: number; why: string; improvement: string };
  structure_analysis: { framework_match: string; retention_issues: string[] };
  visual_analysis: { rhythm: string; first_frame: string; dominant_labels: string[] };
  key_insights: string[];
  action_items: string[];
  replication_script?: string;
  neuromarketing_refs?: string[];
  viral_elements?: string[];
  verdict:
    | 'vai bombar' | 'ok' | 'vai flopar'
    | 'performou bem' | 'na média' | 'abaixo do esperado'
    | string;
  verdict_reason: string;
}

export interface AnalysisListItem {
  id: string;
  mode: Mode;
  status: 'processing' | 'done' | 'error';
  original_name?: string;
  verdict?: string;
  created_at: string;
}
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd /workspace/web && npx tsc --noEmit 2>&1
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/types.ts
git commit -m "feat: add user_concept, neuromarketing_refs, viral_elements to TS types"
```

---

### Task 5: Shared form primitives + ProfilePage

**Files:**
- Create: `web/src/components/FormPrimitives.tsx`
- Create: `web/src/components/ProfilePage.tsx`

- [ ] **Step 1: Create FormPrimitives.tsx**

Create `/workspace/web/src/components/FormPrimitives.tsx`:
```tsx
import type { Metrics } from '../types';

export function SectionTitle({ n, title }: { n: string; title: string }) {
  return (
    <div className="flex items-baseline gap-3 mb-3">
      <span className="font-mono text-xs text-zinc-600">{n}</span>
      <h2 className="font-display text-sm font-semibold uppercase tracking-widest text-zinc-300">{title}</h2>
    </div>
  );
}

export function FieldLabel({ children }: { children: React.ReactNode }) {
  return <label className="block text-xs uppercase tracking-wider text-zinc-500 mb-1.5">{children}</label>;
}

export function TextField({
  label, value, onChange, disabled, placeholder, required,
}: {
  label: string; value: string; onChange: (v: string) => void;
  disabled?: boolean; placeholder?: string; required?: boolean;
}) {
  return (
    <div>
      <FieldLabel>{label}</FieldLabel>
      <input
        type="text" value={value} onChange={(e) => onChange(e.target.value)}
        disabled={disabled} placeholder={placeholder} required={required}
        className="block w-full rounded-md border border-zinc-800 bg-zinc-900/40 px-3 py-2 text-sm placeholder-zinc-600 focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500 transition"
      />
    </div>
  );
}

export function NumField({
  label, value, onChange, step = 1, disabled,
}: {
  label: string; value?: number; onChange: (v: number | undefined) => void;
  step?: number; disabled?: boolean;
}) {
  return (
    <div>
      <FieldLabel>{label}</FieldLabel>
      <input
        type="number" step={step} value={value ?? ''}
        onChange={(e) => onChange(e.target.value === '' ? undefined : Number(e.target.value))}
        disabled={disabled}
        className="block w-full rounded-md border border-zinc-800 bg-zinc-900/40 px-3 py-2 text-sm font-mono focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500 transition"
      />
    </div>
  );
}

export function VideoUpload({
  file, onChange, disabled,
}: {
  file: File | null; onChange: (f: File | null) => void; disabled?: boolean;
}) {
  return (
    <>
      <label
        htmlFor="video-file"
        className={`flex items-center justify-center gap-3 rounded-md border-2 border-dashed p-6 text-sm cursor-pointer transition ${
          file
            ? 'border-emerald-500 bg-emerald-500/5 text-emerald-300'
            : 'border-zinc-800 hover:border-zinc-600 text-zinc-400'
        }`}
      >
        <span className="text-lg">{file ? '✓' : '↑'}</span>
        <span className="font-mono">
          {file ? `${file.name} · ${(file.size / 1024 / 1024).toFixed(1)} MB` : 'escolha um arquivo .mp4'}
        </span>
      </label>
      <input
        id="video-file" type="file" accept="video/mp4,video/*"
        onChange={(e) => onChange(e.target.files?.[0] || null)}
        disabled={disabled} className="sr-only"
      />
    </>
  );
}

export function SubmitButton({ disabled, label = 'Analisar →' }: { disabled?: boolean; label?: string }) {
  return (
    <button
      type="submit" disabled={disabled}
      className="w-full py-3 rounded-md bg-emerald-500 hover:bg-emerald-400 disabled:opacity-50 disabled:cursor-not-allowed font-display font-semibold text-zinc-950 tracking-wide transition"
    >
      {label}
    </button>
  );
}

export function NoProfileBanner() {
  return (
    <div className="rounded-md border border-amber-700/40 bg-amber-950/30 px-4 py-3 text-xs text-amber-300">
      Perfil não configurado — análise será genérica. Configure em <strong>Configurações</strong> no sidebar.
    </div>
  );
}
```

- [ ] **Step 2: Create ProfilePage.tsx**

Create `/workspace/web/src/components/ProfilePage.tsx`:
```tsx
import { useState } from 'react';
import { updateMe } from '../api';
import type { BusinessContext, Platform, User } from '../types';
import { TextField, FieldLabel } from './FormPrimitives';

const PLATFORMS: Platform[] = ['tiktok', 'instagram', 'youtube', 'other'];

interface Props {
  user: User;
  onSaved: (bc: BusinessContext) => void;
}

export function ProfilePage({ user, onSaved }: Props) {
  const [bc, setBC] = useState<BusinessContext>(
    user.business_context ?? {
      brand_name: '', description: '', target_audience: '',
      platforms: ['tiktok'], main_pain: '', content_history: '',
    }
  );
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);

  function togglePlatform(p: Platform) {
    setBC((prev) => ({
      ...prev,
      platforms: prev.platforms.includes(p)
        ? prev.platforms.filter((x) => x !== p)
        : [...prev.platforms, p],
    }));
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    try {
      await updateMe(bc);
      setSaved(true);
      onSaved(bc);
      setTimeout(() => setSaved(false), 3000);
    } catch {
      // ignore — user can retry
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="max-w-xl">
      <h1 className="font-display text-xl font-bold mb-1">Perfil</h1>
      <p className="text-sm text-zinc-500 mb-8">
        Preenchido uma vez, usado em todas as análises para personalizar os insights.
      </p>
      <form onSubmit={handleSubmit} className="space-y-4">
        <TextField
          label="Marca / Criador" value={bc.brand_name}
          onChange={(v) => setBC({ ...bc, brand_name: v })}
          placeholder="Ex: @perfil, Minha Marca"
        />
        <TextField
          label="O que faz / vende" value={bc.description}
          onChange={(v) => setBC({ ...bc, description: v })}
          placeholder="Ex: curso de design, loja de roupas"
        />
        <TextField
          label="Público-alvo" value={bc.target_audience}
          onChange={(v) => setBC({ ...bc, target_audience: v })}
          placeholder="Ex: mulheres 25–35 interessadas em moda"
        />
        <div>
          <FieldLabel>Plataformas</FieldLabel>
          <div className="flex gap-2 flex-wrap">
            {PLATFORMS.map((p) => (
              <button
                key={p} type="button" onClick={() => togglePlatform(p)}
                className={`px-3 py-1 text-xs rounded-full border transition ${
                  bc.platforms.includes(p)
                    ? 'border-emerald-500 bg-emerald-500/10 text-emerald-300'
                    : 'border-zinc-700 text-zinc-400 hover:border-zinc-500'
                }`}
              >
                {p}
              </button>
            ))}
          </div>
        </div>
        <TextField
          label="Principal dor do seu público" value={bc.main_pain}
          onChange={(v) => setBC({ ...bc, main_pain: v })}
          placeholder="Ex: não tem tempo para cozinhar saudável"
        />
        <TextField
          label="O que já funcionou no seu conteúdo" value={bc.content_history}
          onChange={(v) => setBC({ ...bc, content_history: v })}
          placeholder="Ex: vídeos com antes/depois têm mais retenção"
        />
        <button
          type="submit" disabled={saving}
          className="w-full py-3 rounded-md bg-emerald-500 hover:bg-emerald-400 disabled:opacity-50 font-display font-semibold text-zinc-950 tracking-wide transition"
        >
          {saved ? '✓ Salvo' : saving ? 'Salvando...' : 'Salvar perfil'}
        </button>
      </form>
    </div>
  );
}
```

- [ ] **Step 3: Verify TypeScript**

```bash
cd /workspace/web && npx tsc --noEmit 2>&1
```
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add web/src/components/FormPrimitives.tsx web/src/components/ProfilePage.tsx
git commit -m "feat: ProfilePage + shared FormPrimitives components"
```

---

### Task 6: Mode-specific sub-forms

**Files:**
- Create: `web/src/components/ReferenceForm.tsx`
- Create: `web/src/components/PrePostForm.tsx`
- Create: `web/src/components/PostMortemForm.tsx`

- [ ] **Step 1: Create ReferenceForm.tsx**

Create `/workspace/web/src/components/ReferenceForm.tsx`:
```tsx
import { useState } from 'react';
import type { Metrics } from '../types';
import { SectionTitle, VideoUpload, NumField, SubmitButton } from './FormPrimitives';

interface Props {
  disabled?: boolean;
  onSubmit: (data: { file: File; metrics?: Metrics; userConcept: string }) => void;
}

export function ReferenceForm({ disabled, onSubmit }: Props) {
  const [file, setFile] = useState<File | null>(null);
  const [metrics, setMetrics] = useState<Metrics>({});
  const [userConcept, setUserConcept] = useState('');

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!file) { alert('Selecione o vídeo viral de referência'); return; }
    const m = Object.values(metrics).some((v) => v !== undefined) ? metrics : undefined;
    onSubmit({ file, metrics: m, userConcept });
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-8">
      <section>
        <SectionTitle n="02" title="Vídeo viral de referência" />
        <VideoUpload file={file} onChange={setFile} disabled={disabled} />
      </section>

      <section>
        <SectionTitle n="03" title="O que você quer gravar?" />
        <p className="text-xs text-zinc-500 mb-3">
          Descreva brevemente seu conteúdo planejado. A análise vai montar um roteiro
          personalizado usando os elementos do viral.
        </p>
        <textarea
          value={userConcept}
          onChange={(e) => setUserConcept(e.target.value)}
          disabled={disabled}
          rows={3}
          placeholder="Ex: quero falar sobre como economizar R$500/mês com compras no mercado, meu público é jovens adultos que querem organizar as finanças"
          required
          className="block w-full rounded-md border border-zinc-800 bg-zinc-900/40 px-3 py-2 text-sm placeholder-zinc-600 focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500 transition resize-none"
        />
      </section>

      <section>
        <SectionTitle n="04" title="Performance do viral (opcional)" />
        <div className="grid grid-cols-2 gap-3">
          <NumField label="Views" value={metrics.views} onChange={(v) => setMetrics({ ...metrics, views: v })} disabled={disabled} />
          <NumField label="Likes" value={metrics.likes} onChange={(v) => setMetrics({ ...metrics, likes: v })} disabled={disabled} />
        </div>
      </section>

      <SubmitButton disabled={disabled} />
    </form>
  );
}
```

- [ ] **Step 2: Create PrePostForm.tsx**

Create `/workspace/web/src/components/PrePostForm.tsx`:
```tsx
import { useState } from 'react';
import { SectionTitle, VideoUpload, SubmitButton, NoProfileBanner } from './FormPrimitives';

interface Props {
  disabled?: boolean;
  hasProfile: boolean;
  onSubmit: (data: { file: File; userConcept?: string }) => void;
}

export function PrePostForm({ disabled, hasProfile, onSubmit }: Props) {
  const [file, setFile] = useState<File | null>(null);
  const [userConcept, setUserConcept] = useState('');

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!file) { alert('Selecione um arquivo de vídeo'); return; }
    onSubmit({ file, userConcept: userConcept.trim() || undefined });
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-8">
      {!hasProfile && <NoProfileBanner />}

      <section>
        <SectionTitle n="02" title="Vídeo" />
        <VideoUpload file={file} onChange={setFile} disabled={disabled} />
      </section>

      <section>
        <SectionTitle n="03" title="Conceito planejado (opcional)" />
        <p className="text-xs text-zinc-500 mb-3">
          O que você tentou fazer? Claude avalia se o hook executado bate com a intenção.
        </p>
        <textarea
          value={userConcept}
          onChange={(e) => setUserConcept(e.target.value)}
          disabled={disabled}
          rows={2}
          placeholder="Ex: queria criar urgência falando que só funciona nessa época do ano"
          className="block w-full rounded-md border border-zinc-800 bg-zinc-900/40 px-3 py-2 text-sm placeholder-zinc-600 focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500 transition resize-none"
        />
      </section>

      <SubmitButton disabled={disabled} />
    </form>
  );
}
```

- [ ] **Step 3: Create PostMortemForm.tsx**

Create `/workspace/web/src/components/PostMortemForm.tsx`:
```tsx
import { useState } from 'react';
import type { Metrics } from '../types';
import { SectionTitle, VideoUpload, NumField, SubmitButton, NoProfileBanner } from './FormPrimitives';

interface Props {
  disabled?: boolean;
  hasProfile: boolean;
  onSubmit: (data: { file: File; metrics?: Metrics }) => void;
}

export function PostMortemForm({ disabled, hasProfile, onSubmit }: Props) {
  const [file, setFile] = useState<File | null>(null);
  const [metrics, setMetrics] = useState<Metrics>({});

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!file) { alert('Selecione o vídeo já postado'); return; }
    const m = Object.values(metrics).some((v) => v !== undefined) ? metrics : undefined;
    onSubmit({ file, metrics: m });
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-8">
      {!hasProfile && <NoProfileBanner />}

      <section>
        <SectionTitle n="02" title="Vídeo postado" />
        <VideoUpload file={file} onChange={setFile} disabled={disabled} />
      </section>

      <section>
        <SectionTitle n="03" title="Métricas (opcional)" />
        <div className="grid grid-cols-2 gap-3">
          <NumField label="Views" value={metrics.views} onChange={(v) => setMetrics({ ...metrics, views: v })} disabled={disabled} />
          <NumField label="Likes" value={metrics.likes} onChange={(v) => setMetrics({ ...metrics, likes: v })} disabled={disabled} />
          <NumField label="Avg watch time (s)" value={metrics.avg_watch_time} onChange={(v) => setMetrics({ ...metrics, avg_watch_time: v })} disabled={disabled} />
          <NumField label="Completion rate (0–1)" step={0.01} value={metrics.completion_rate} onChange={(v) => setMetrics({ ...metrics, completion_rate: v })} disabled={disabled} />
          <NumField label="Followers ganhos" value={metrics.followers_gained} onChange={(v) => setMetrics({ ...metrics, followers_gained: v })} disabled={disabled} />
        </div>
      </section>

      <SubmitButton disabled={disabled} />
    </form>
  );
}
```

- [ ] **Step 4: Verify TypeScript**

```bash
cd /workspace/web && npx tsc --noEmit 2>&1
```
Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/ReferenceForm.tsx web/src/components/PrePostForm.tsx web/src/components/PostMortemForm.tsx
git commit -m "feat: mode-specific sub-forms (Reference, PrePost, PostMortem)"
```

---

### Task 7: AnalysisForm shell refactor

**Files:**
- Modify: `web/src/components/AnalysisForm.tsx`

- [ ] **Step 1: Replace AnalysisForm.tsx**

Replace `/workspace/web/src/components/AnalysisForm.tsx`:
```tsx
import { useState } from 'react';
import type { BusinessContext, Metrics, Mode, User } from '../types';
import { ReferenceForm } from './ReferenceForm';
import { PrePostForm } from './PrePostForm';
import { PostMortemForm } from './PostMortemForm';

interface Props {
  user: User;
  disabled?: boolean;
  onSubmit: (data: {
    file: File;
    mode: Mode;
    businessContext: BusinessContext;
    metrics?: Metrics;
    userConcept?: string;
  }) => void;
}

const MODE_OPTIONS: { value: Mode; label: string; sub: string }[] = [
  { value: 'reference',   label: 'Referência',    sub: 'vídeo viral de terceiro' },
  { value: 'pre_post',    label: 'Pré-postagem',  sub: 'vale postar como está?' },
  { value: 'post_mortem', label: 'Post-mortem',   sub: 'diagnóstico de já postado' },
];

export function AnalysisForm({ user, disabled, onSubmit }: Props) {
  const [mode, setMode] = useState<Mode>('reference');

  const bc: BusinessContext = user.business_context ?? {
    brand_name: '', description: '', target_audience: '',
    platforms: [], main_pain: '', content_history: '',
  };
  const hasProfile = !!user.business_context?.brand_name;

  return (
    <div className="space-y-8">
      <section>
        <div className="flex items-baseline gap-3 mb-3">
          <span className="font-mono text-xs text-zinc-600">01</span>
          <h2 className="font-display text-sm font-semibold uppercase tracking-widest text-zinc-300">Modo</h2>
        </div>
        <div className="grid grid-cols-3 gap-2">
          {MODE_OPTIONS.map((opt) => (
            <button
              key={opt.value} type="button"
              onClick={() => setMode(opt.value)}
              disabled={disabled}
              className={`rounded-md border p-3 text-left transition ${
                mode === opt.value
                  ? 'border-emerald-500 bg-emerald-500/5'
                  : 'border-zinc-800 hover:border-zinc-600'
              }`}
            >
              <div className="font-display text-sm font-semibold">{opt.label}</div>
              <div className="text-xs text-zinc-500 mt-0.5">{opt.sub}</div>
            </button>
          ))}
        </div>
      </section>

      {mode === 'reference' && (
        <ReferenceForm
          disabled={disabled}
          onSubmit={({ file, metrics, userConcept }) =>
            onSubmit({ file, mode, businessContext: bc, metrics, userConcept })
          }
        />
      )}
      {mode === 'pre_post' && (
        <PrePostForm
          disabled={disabled}
          hasProfile={hasProfile}
          onSubmit={({ file, userConcept }) =>
            onSubmit({ file, mode, businessContext: bc, userConcept })
          }
        />
      )}
      {mode === 'post_mortem' && (
        <PostMortemForm
          disabled={disabled}
          hasProfile={hasProfile}
          onSubmit={({ file, metrics }) =>
            onSubmit({ file, mode, businessContext: bc, metrics })
          }
        />
      )}
    </div>
  );
}
```

- [ ] **Step 2: Verify TypeScript**

```bash
cd /workspace/web && npx tsc --noEmit 2>&1
```
Expected: no errors (App.tsx will have errors until Task 8 — that's fine for now; check only form-related errors).

- [ ] **Step 3: Commit**

```bash
git add web/src/components/AnalysisForm.tsx
git commit -m "refactor: AnalysisForm into shell rendering mode-specific sub-forms"
```

---

### Task 8: Sidebar + App.tsx wiring

**Files:**
- Modify: `web/src/components/AnalysesSidebar.tsx`
- Modify: `web/src/App.tsx`

- [ ] **Step 1: Update AnalysesSidebar.tsx**

Replace `/workspace/web/src/components/AnalysesSidebar.tsx`:
```tsx
import type { AnalysisListItem } from '../types';

interface Props {
  items: AnalysisListItem[];
  currentId?: string;
  onSelect: (id: string) => void;
  onNew: () => void;
  onGoToProfile: () => void;
  userEmail?: string;
  onLogout?: () => void;
}

const STATUS_STYLE: Record<string, string> = {
  processing: 'bg-amber-400 animate-pulse',
  done: 'bg-emerald-500',
  error: 'bg-rose-500',
};

const MODE_LABEL: Record<string, string> = {
  pre_post: 'PRÉ',
  reference: 'REF',
  post_mortem: 'POST',
};

const VERDICT_ICON: Record<string, string> = {
  'vai bombar':          '↑',
  ok:                    '→',
  'vai flopar':          '↓',
  'performou bem':       '↑',
  'na média':            '→',
  'abaixo do esperado':  '↓',
};

const VERDICT_COLOR: Record<string, string> = {
  'vai bombar':          'text-emerald-400',
  ok:                    'text-amber-400',
  'vai flopar':          'text-rose-400',
  'performou bem':       'text-emerald-400',
  'na média':            'text-amber-400',
  'abaixo do esperado':  'text-rose-400',
};

export function AnalysesSidebar({ items, currentId, onSelect, onNew, onGoToProfile, userEmail, onLogout }: Props) {
  return (
    <aside className="w-64 h-full border-r border-zinc-900 flex flex-col bg-zinc-950/50">
      <div className="p-4 border-b border-zinc-900">
        <div className="flex items-baseline justify-between mb-3">
          <h1 className="font-display text-base font-bold tracking-tight">
            video<span className="text-emerald-500">.</span>analyzer
          </h1>
          <span className="font-mono text-[10px] text-zinc-600">v0.1</span>
        </div>
        <button
          onClick={onNew}
          className="w-full py-2 rounded-md bg-zinc-800 hover:bg-zinc-700 text-xs font-medium tracking-wide transition"
        >
          + nova análise
        </button>
      </div>
      <div className="flex-1 overflow-y-auto scrollbar-thin">
        {items.length === 0 && (
          <div className="p-4 text-xs text-zinc-600">Nenhuma análise ainda.</div>
        )}
        {items.map((it) => (
          <button
            key={it.id}
            onClick={() => onSelect(it.id)}
            className={`w-full text-left p-3 border-b border-zinc-900/60 hover:bg-zinc-900/40 text-sm flex items-center gap-2 transition ${
              currentId === it.id ? 'bg-zinc-900/60' : ''
            }`}
          >
            <span className={`inline-block w-1.5 h-1.5 rounded-full ${STATUS_STYLE[it.status] || 'bg-zinc-600'}`} />
            <span className="font-mono text-[10px] text-zinc-500 w-10 shrink-0">{MODE_LABEL[it.mode]}</span>
            <span className="flex-1 truncate text-zinc-300">{it.original_name || it.id.slice(0, 8)}</span>
            {it.verdict && (
              <span className={`text-[10px] font-mono shrink-0 ${VERDICT_COLOR[it.verdict] || 'text-zinc-500'}`}>
                {VERDICT_ICON[it.verdict] || '·'}
              </span>
            )}
          </button>
        ))}
      </div>
      {userEmail && onLogout && (
        <div className="p-3 border-t border-zinc-900">
          <button
            onClick={onGoToProfile}
            className="w-full text-left text-[10px] text-zinc-500 hover:text-zinc-300 font-mono mb-2 transition"
          >
            ⚙ Configurações
          </button>
          <div className="flex items-center gap-2">
            <span className="flex-1 truncate text-[10px] text-zinc-600 font-mono">{userEmail}</span>
            <button
              onClick={onLogout}
              className="text-[10px] text-zinc-600 hover:text-zinc-400 transition shrink-0"
            >
              sair
            </button>
          </div>
        </div>
      )}
    </aside>
  );
}
```

- [ ] **Step 2: Replace App.tsx**

Replace `/workspace/web/src/App.tsx`:
```tsx
import { useEffect, useRef, useState } from 'react';
import {
  clearToken,
  getAnalysis,
  getMe,
  getSignedURL,
  getToken,
  listAnalyses,
  startAnalyze,
  uploadToGCS,
} from './api';
import { AuthPage } from './components/AuthPage';
import { AnalysesSidebar } from './components/AnalysesSidebar';
import { AnalysisForm } from './components/AnalysisForm';
import { AnalysisResult } from './components/AnalysisResult';
import { AnalysisRunning } from './components/AnalysisRunning';
import { ProfilePage } from './components/ProfilePage';
import type { AnalysisListItem, AnalysisStatus, BusinessContext, Mode, User } from './types';

type View =
  | { kind: 'form' }
  | { kind: 'uploading'; pct: number }
  | { kind: 'running'; id: string; progressMsg: string }
  | { kind: 'result'; status: AnalysisStatus }
  | { kind: 'profile' }
  | { kind: 'error'; msg: string };

export default function App() {
  const [user, setUser] = useState<User | null>(null);
  const [authChecked, setAuthChecked] = useState(false);
  const [view, setView] = useState<View>({ kind: 'form' });
  const [list, setList] = useState<AnalysisListItem[]>([]);
  const [currentId, setCurrentId] = useState<string | undefined>();
  const pollRef = useRef<number | null>(null);

  useEffect(() => {
    const token = getToken();
    if (!token) { setAuthChecked(true); return; }
    getMe()
      .then((u) => setUser(u))
      .catch(() => { clearToken(); setAuthChecked(true); });
  }, []);

  useEffect(() => {
    if (user) { refreshList(); setAuthChecked(true); }
  }, [user]);

  useEffect(() => {
    return () => { if (pollRef.current) window.clearInterval(pollRef.current); };
  }, []);

  async function refreshList() {
    try { setList(await listAnalyses()); } catch { /* ignore */ }
  }

  function stopPolling() {
    if (pollRef.current) { window.clearInterval(pollRef.current); pollRef.current = null; }
  }

  function startPolling(id: string) {
    stopPolling();
    pollRef.current = window.setInterval(async () => {
      try {
        const s = await getAnalysis(id);
        if (s.status === 'processing') {
          setView({ kind: 'running', id, progressMsg: s.progress_msg });
        } else {
          stopPolling();
          setView({ kind: 'result', status: s });
          refreshList();
        }
      } catch (e) {
        stopPolling();
        setView({ kind: 'error', msg: String(e) });
      }
    }, 3000);
  }

  async function handleSubmit(data: {
    file: File;
    mode: Mode;
    businessContext: BusinessContext;
    metrics?: Metrics;
    userConcept?: string;
  }) {
    try {
      setView({ kind: 'uploading', pct: 0 });
      const signed = await getSignedURL(data.file.name, data.file.type || 'video/mp4');
      await uploadToGCS(signed.put_url, data.file, (pct) => setView({ kind: 'uploading', pct }));
      const { id } = await startAnalyze({
        gcs_uri: signed.gcs_uri,
        original_name: data.file.name,
        mode: data.mode,
        business_context: data.businessContext,
        metrics: data.metrics,
        user_concept: data.userConcept,
      });
      setCurrentId(id);
      setView({ kind: 'running', id, progressMsg: 'Iniciando análise...' });
      startPolling(id);
    } catch (e) {
      setView({ kind: 'error', msg: String(e) });
    }
  }

  async function handleSelect(id: string) {
    stopPolling();
    setCurrentId(id);
    try {
      const s = await getAnalysis(id);
      if (s.status === 'processing') {
        setView({ kind: 'running', id, progressMsg: s.progress_msg });
        startPolling(id);
      } else {
        setView({ kind: 'result', status: s });
      }
    } catch (e) {
      setView({ kind: 'error', msg: String(e) });
    }
  }

  function handleNew() {
    stopPolling();
    setCurrentId(undefined);
    setView({ kind: 'form' });
  }

  function handleLogout() {
    stopPolling();
    clearToken();
    setUser(null);
    setAuthChecked(true);
    setList([]);
    setView({ kind: 'form' });
  }

  function handleProfileSaved(bc: BusinessContext) {
    setUser((u) => u ? { ...u, business_context: bc } : u);
  }

  if (!authChecked) {
    return (
      <div className="min-h-screen bg-zinc-950 flex items-center justify-center">
        <div className="text-zinc-600 text-sm font-mono">carregando...</div>
      </div>
    );
  }

  if (!user) {
    return <AuthPage onAuth={(u) => setUser(u)} />;
  }

  const hasProfile = !!user.business_context?.brand_name;

  return (
    <div className="h-full flex">
      <AnalysesSidebar
        items={list}
        currentId={currentId}
        onSelect={handleSelect}
        onNew={handleNew}
        onGoToProfile={() => { stopPolling(); setCurrentId(undefined); setView({ kind: 'profile' }); }}
        userEmail={user.email}
        onLogout={handleLogout}
      />
      <main className="flex-1 overflow-y-auto scrollbar-thin">
        <div className="max-w-3xl mx-auto px-8 py-12 w-full">
          {!hasProfile && view.kind === 'form' && (
            <div className="mb-8 rounded-md border border-amber-700/40 bg-amber-950/30 px-4 py-3 text-xs text-amber-300 flex items-center justify-between">
              <span>Configure seu perfil para análises personalizadas</span>
              <button
                onClick={() => setView({ kind: 'profile' })}
                className="ml-4 underline hover:text-amber-200 transition shrink-0"
              >
                Configurar →
              </button>
            </div>
          )}

          {view.kind === 'form' && (
            <AnalysisForm
              user={user}
              onSubmit={handleSubmit}
            />
          )}

          {view.kind === 'uploading' && (
            <AnalysisRunning progressMsg="Subindo vídeo..." uploadPct={view.pct} />
          )}

          {view.kind === 'running' && <AnalysisRunning progressMsg={view.progressMsg} />}

          {view.kind === 'result' && view.status.status === 'done' && view.status.result && (
            <AnalysisResult result={view.status.result} mode={view.status.mode} />
          )}

          {view.kind === 'result' && view.status.status === 'error' && (
            <ErrorBanner msg={view.status.error || 'Erro desconhecido'} onBack={handleNew} />
          )}

          {view.kind === 'error' && <ErrorBanner msg={view.msg} onBack={handleNew} />}

          {view.kind === 'profile' && (
            <ProfilePage user={user} onSaved={handleProfileSaved} />
          )}
        </div>
      </main>
    </div>
  );
}

function ErrorBanner({ msg, onBack }: { msg: string; onBack: () => void }) {
  return (
    <div className="rounded-md border border-rose-700/60 bg-rose-950/40 p-5">
      <div className="flex items-baseline gap-2 mb-2">
        <span className="font-mono text-xs uppercase tracking-widest text-rose-400">erro</span>
      </div>
      <p className="text-sm text-rose-200 leading-relaxed mb-4">{msg}</p>
      <button
        onClick={onBack}
        className="text-xs font-mono uppercase tracking-wider text-rose-300 hover:text-rose-200 underline"
      >
        ← nova análise
      </button>
    </div>
  );
}
```

- [ ] **Step 3: Verify TypeScript**

```bash
cd /workspace/web && npx tsc --noEmit 2>&1
```
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add web/src/components/AnalysesSidebar.tsx web/src/App.tsx
git commit -m "feat: profile view, first-run banner, Configurações in sidebar, wire userConcept"
```

---

### Task 9: AnalysisResult — neuromarketing sections + post-mortem verdicts

**Files:**
- Modify: `web/src/components/AnalysisResult.tsx`

- [ ] **Step 1: Replace AnalysisResult.tsx**

Replace `/workspace/web/src/components/AnalysisResult.tsx`:
```tsx
import type { ClaudeResult, Mode } from '../types';

interface Props {
  result: ClaudeResult;
  mode?: Mode;
}

const VERDICT_STYLE: Record<string, { bg: string; ring: string; icon: string }> = {
  'vai bombar':         { bg: 'bg-emerald-500 text-zinc-950', ring: 'ring-emerald-500/40', icon: '↑' },
  ok:                   { bg: 'bg-amber-400 text-zinc-950',   ring: 'ring-amber-400/40',   icon: '→' },
  'vai flopar':         { bg: 'bg-rose-500 text-white',       ring: 'ring-rose-500/40',     icon: '↓' },
  'performou bem':      { bg: 'bg-emerald-500 text-zinc-950', ring: 'ring-emerald-500/40', icon: '↑' },
  'na média':           { bg: 'bg-amber-400 text-zinc-950',   ring: 'ring-amber-400/40',   icon: '→' },
  'abaixo do esperado': { bg: 'bg-rose-500 text-white',       ring: 'ring-rose-500/40',     icon: '↓' },
};

export function AnalysisResult({ result, mode }: Props) {
  const style = VERDICT_STYLE[result.verdict] || {
    bg: 'bg-zinc-600 text-white', ring: 'ring-zinc-600/40', icon: '·',
  };

  const isReference = mode === 'reference';

  return (
    <div className="space-y-6">
      <div className={`rounded-lg p-6 ring-2 ${style.ring} ${style.bg} relative overflow-hidden`}>
        <div className="absolute top-2 right-3 font-mono text-xs opacity-50">VEREDITO</div>
        <div className="flex items-baseline gap-3">
          <span className="font-display text-5xl font-bold leading-none">{style.icon}</span>
          <span className="font-display text-3xl font-bold tracking-tight">{result.verdict}</span>
        </div>
        <p className="text-sm mt-2 opacity-90 leading-relaxed">{result.verdict_reason}</p>
      </div>

      <div className="grid grid-cols-3 gap-3">
        <ScoreBlock label="Hook" value={`${result.hook_analysis.score}`} suffix="/10" />
        <InfoBlock label="Ritmo" value={result.visual_analysis.rhythm} />
        <InfoBlock label="Issues" value={`${result.structure_analysis.retention_issues.length}`} />
      </div>

      <Section title="Hook" accent="emerald">
        <p className="text-sm leading-relaxed mb-3">
          <span className="text-zinc-500 font-mono text-xs uppercase tracking-wider mr-2">por quê</span>
          {result.hook_analysis.why}
        </p>
        <p className="text-sm leading-relaxed">
          <span className="text-zinc-500 font-mono text-xs uppercase tracking-wider mr-2">melhorar</span>
          {result.hook_analysis.improvement}
        </p>
      </Section>

      <Section title="Estrutura" accent="amber">
        <p className="text-sm leading-relaxed mb-3">
          <span className="text-zinc-500 font-mono text-xs uppercase tracking-wider mr-2">framework</span>
          {result.structure_analysis.framework_match}
        </p>
        {result.structure_analysis.retention_issues.length > 0 && (
          <ul className="text-sm space-y-1.5 mt-3">
            {result.structure_analysis.retention_issues.map((x, i) => (
              <li key={i} className="flex gap-2">
                <span className="text-rose-400 font-mono shrink-0">⚠</span>
                <span>{x}</span>
              </li>
            ))}
          </ul>
        )}
      </Section>

      <Section title="Análise visual" accent="sky">
        <dl className="text-sm space-y-2">
          <Row label="Primeiro frame" value={result.visual_analysis.first_frame} />
          <Row
            label="Labels dominantes"
            value={
              <span className="font-mono text-xs">
                {result.visual_analysis.dominant_labels.join(' · ')}
              </span>
            }
          />
        </dl>
      </Section>

      {isReference && result.neuromarketing_refs && result.neuromarketing_refs.length > 0 && (
        <Section title="Por que viralizou" accent="violet">
          <div className="flex flex-wrap gap-2 mb-4">
            {result.neuromarketing_refs.map((ref, i) => (
              <span
                key={i}
                className="px-2.5 py-1 rounded text-xs bg-violet-500/10 border border-violet-500/20 text-violet-300"
              >
                {ref}
              </span>
            ))}
          </div>
          {result.viral_elements && result.viral_elements.length > 0 && (
            <>
              <p className="text-zinc-500 font-mono text-xs uppercase tracking-wider mb-2">elementos replicáveis</p>
              <ol className="space-y-1.5">
                {result.viral_elements.map((el, i) => (
                  <li key={i} className="flex gap-2 text-sm">
                    <span className="text-violet-400 font-mono shrink-0">{String(i + 1).padStart(2, '0')}</span>
                    <span>{el}</span>
                  </li>
                ))}
              </ol>
            </>
          )}
        </Section>
      )}

      <Section title="Insights" accent="violet">
        <ul className="space-y-2">
          {result.key_insights.map((x, i) => (
            <li key={i} className="flex gap-3 text-sm leading-relaxed">
              <span className="text-violet-400 font-mono shrink-0">{String(i + 1).padStart(2, '0')}</span>
              <span>{x}</span>
            </li>
          ))}
        </ul>
      </Section>

      <Section title="Próximas ações" accent="emerald">
        <ol className="space-y-2">
          {result.action_items.map((x, i) => (
            <li key={i} className="flex gap-3 text-sm leading-relaxed">
              <span className="text-emerald-400 font-mono shrink-0">→</span>
              <span>{x}</span>
            </li>
          ))}
        </ol>
      </Section>

      {result.replication_script && (
        <Section title="Roteiro adaptado" accent="rose">
          <pre className="whitespace-pre-wrap text-sm bg-zinc-900/60 border border-zinc-800 p-4 rounded font-mono leading-relaxed">
            {result.replication_script}
          </pre>
        </Section>
      )}
    </div>
  );
}

const ACCENT_BORDER: Record<string, string> = {
  emerald: 'border-l-emerald-500',
  amber:   'border-l-amber-500',
  sky:     'border-l-sky-500',
  violet:  'border-l-violet-500',
  rose:    'border-l-rose-500',
};

function Section({ title, children, accent }: { title: string; children: React.ReactNode; accent: string }) {
  return (
    <section className={`rounded-md bg-zinc-900/30 border border-zinc-800 border-l-2 ${ACCENT_BORDER[accent] || 'border-l-zinc-700'} p-5`}>
      <h3 className="font-display text-xs font-semibold uppercase tracking-widest text-zinc-400 mb-4">{title}</h3>
      {children}
    </section>
  );
}

function ScoreBlock({ label, value, suffix }: { label: string; value: string; suffix?: string }) {
  return (
    <div className="rounded-md bg-zinc-900/30 border border-zinc-800 p-4">
      <div className="font-mono text-xs uppercase tracking-wider text-zinc-500 mb-1">{label}</div>
      <div className="flex items-baseline gap-1">
        <span className="font-display text-3xl font-bold">{value}</span>
        {suffix && <span className="text-sm text-zinc-500">{suffix}</span>}
      </div>
    </div>
  );
}

function InfoBlock({ label, value, suffix }: { label: string; value: string; suffix?: string }) {
  return (
    <div className="rounded-md bg-zinc-900/30 border border-zinc-800 p-4">
      <div className="font-mono text-xs uppercase tracking-wider text-zinc-500 mb-1">{label}</div>
      <div className="font-display text-lg font-semibold capitalize">
        {value}
        {suffix && <span className="text-sm text-zinc-500 font-normal ml-1">{suffix}</span>}
      </div>
    </div>
  );
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex gap-3">
      <dt className="font-mono text-xs uppercase tracking-wider text-zinc-500 w-32 shrink-0 pt-0.5">{label}</dt>
      <dd className="flex-1">{value}</dd>
    </div>
  );
}
```

- [ ] **Step 2: Verify TypeScript**

```bash
cd /workspace/web && npx tsc --noEmit 2>&1
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/components/AnalysisResult.tsx
git commit -m "feat: neuromarketing section in result, post-mortem verdict labels"
```

---

### Task 10: Deploy + smoke test

- [ ] **Step 1: Push to GitHub**

```bash
git push origin main
```

- [ ] **Step 2: Deploy API to Railway**

```bash
export PATH="$HOME/.railway/bin:$PATH"
railway up ./api --path-as-root --service api --no-gitignore --detach
```

- [ ] **Step 3: Deploy web to Railway**

```bash
railway up ./web --path-as-root --service web --no-gitignore --detach
```

- [ ] **Step 4: Wait for builds and verify API**

```bash
sleep 90
curl -s https://api-production-62ac.up.railway.app/healthz
```
Expected: `ok`

- [ ] **Step 5: Smoke test auth + reference mode**

```bash
TOKEN=$(curl -s -X POST https://api-production-62ac.up.railway.app/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"smoke@test.com","password":"test1234"}' | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
echo "Token: ${TOKEN:0:20}..."
curl -s https://api-production-62ac.up.railway.app/api/analyses \
  -H "Authorization: Bearer $TOKEN" | head -c 100
```
Expected: token returned, analyses list (JSON array).

- [ ] **Step 6: Manual browser smoke test**

Open https://web-production-57fab.up.railway.app and verify:
- Login works
- Mode selector shows: Referência | Pré-postagem | Post-mortem (in this order)
- Referência form has video upload + concept textarea + metrics
- Pré-postagem form has video upload + optional concept textarea
- Post-mortem form has video upload + full metrics
- "Configurações" link in sidebar opens profile page
- First-run banner shows when no profile set

- [ ] **Step 7: Final commit if any fixups needed**

```bash
git add -A
git commit -m "fix: post-deploy fixups" 2>/dev/null || echo "nothing to fix"
git push origin main
```
