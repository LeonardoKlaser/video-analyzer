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

## Como identificar o hook
Os dados do vídeo incluem duas fontes de informação sobre os primeiros segundos:
- "speech": transcrição de áudio via Google Speech-to-Text pt-BR. "speech.hookText" contém as palavras faladas nos primeiros 8s. "speech.fullTranscript" cobre o vídeo inteiro. Se speech for null ou hookText for null, não há fala detectável.
- "textDetection.hookTexts": textos sobrepostos na tela detectados nos primeiros 8s.

O gancho real é frequentemente a interseção ou complemento entre o que é FALADO e o que está ESCRITO NA TELA simultaneamente nos primeiros 5-8s. Se speech.hookText e textDetection.hookTexts se sobrepõem em tema ou palavras, esse é o gancho. Priorize speech.hookText quando disponível — é o que o viewer ouve primeiro.

## Regra absoluta para visual_analysis
NUNCA inclua nomes técnicos de labels de visão computacional (ex: "desktop computer", "facial expression", "font", "screenshot") diretamente na análise. Esses são dados brutos internos. Sempre traduza para o que o VIEWER vê e experimenta: "O vídeo passa X segundos mostrando apenas a tela do computador sem rosto, criando risco de queda de atenção nesse trecho" — não "labels: desktop computer, flat panel display".

## Estrutura obrigatória (campos com nomes EXATOS):
{
  "hook_analysis": { "score": 1-10, "why": "...", "improvement": "..." },
  "structure_analysis": { "framework_match": "...", "retention_issues": ["..."] },
  "visual_analysis": {
    "rhythm": "descrição do ritmo de cortes e o que isso significa para retenção (ex: 'Ritmo lento: média de 14s por shot, bem abaixo do mínimo de 3s para TikTok. O shot entre 7s-33s mostra apenas texto sem rosto, risco alto de abandono.')",
    "first_frame": "o que o viewer vê e sente no primeiro segundo — elementos presentes e impacto no CTR",
    "dominant_visual": "o que domina visualmente a maior parte do vídeo e o que isso significa para engajamento"
  },
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
