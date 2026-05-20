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
		return "Vídeo viral de terceiro pra usar como referência. Foco: por que viralizou + como replicar no contexto do usuário. Inclua um replication_script com roteiro adaptado."
	case models.ModePostMortem:
		return "Vídeo já postado. Diagnóstico do que funcionou ou não. Compare métricas com benchmarks (Caso 4 do system prompt). Foco: aprendizado pra próximos."
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
  "verdict": "vai bombar" | "ok" | "vai flopar",
  "verdict_reason": "..."
}`

func BuildUserMessage(mode models.Mode, bc models.BusinessContext, metrics *models.Metrics, gvi json.RawMessage) string {
	platforms := strings.Join(bc.Platforms, ", ")
	if platforms == "" {
		platforms = "(não informadas)"
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
