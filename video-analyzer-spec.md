# Video Analyzer — Especificação Completa do Projeto

> Documento gerado em maio/2026. Base para desenvolvimento com Claude Code.

---

## 1. Visão Geral do Produto

### O Que É

SaaS de análise de vídeos para criadores de conteúdo. O usuário cola a URL de um vídeo (ou faz upload do arquivo) e o sistema extrai dados técnicos via Google Video Intelligence API, combina com o contexto de negócio do usuário, e usa Claude API para gerar insights acionáveis: por que o vídeo funcionou, o que melhorar, como replicar uma referência no próprio contexto.

### Casos de Uso (Feature 1 — MVP)

1. **Pré-postagem:** Usuário gravou um vídeo, ainda não postou. Quer saber se tem problema, o que melhorar antes de publicar.
2. **Análise de referência:** Usuário cola um vídeo viral de outra pessoa. Quer entender por que viralizou e como replicar a técnica no próprio contexto de negócio.
3. **Post-mortem:** Usuário já postou, tem as métricas (views, watch time, completion rate). Quer saber por que flopou ou por que deu certo.

### Feature 2 (Pós-MVP)

Integração com Instagram Graph API (OAuth) para análise automática do perfil completo do usuário — puxar todos os vídeos + métricas e gerar diagnóstico do canal inteiro.

---

## 2. Stack Tecnológica

| Camada | Tecnologia | Justificativa |
|--------|-----------|---------------|
| Backend | Go | Já dominado pelo founder |
| Frontend | React | Já dominado pelo founder |
| Hosting | Railway (nova conta, free trial) | Custo zero no início |
| Banco de dados | PostgreSQL (Railway) | Já disponível no Railway |
| Storage temporário | Cloudflare R2 ou `/tmp` local | Vídeos são deletados pós-análise |
| Download de URLs | yt-dlp (CLI Python) | Baixa vídeos públicos de TikTok e Instagram |
| Análise de vídeo | Google Video Intelligence API | Label detection, shot changes, transcrição, OCR |
| IA de interpretação | Claude API (claude-sonnet-4-20250514) | Interpretação dos dados + insights |
| Pagamento | Pagar.me | Já utilizado em outros produtos do founder |
| Email transacional | Resend | Já utilizado |

### Dependências externas a instalar

```bash
# yt-dlp (no servidor Railway, via Dockerfile ou nixpacks)
pip install yt-dlp

# SDK Google Cloud no backend Go
go get cloud.google.com/go/videointelligence/apiv1
```

---

## 3. Arquitetura do Sistema

### Fluxo completo de análise

```
[Usuário]
    │
    ├── Cola URL pública (TikTok/Instagram)
    │       └── Backend: yt-dlp baixa o .mp4 → /tmp/videos/{job_id}.mp4
    │
    └── Faz upload de arquivo
            └── Backend: salva em /tmp/videos/{job_id}.mp4
                            │
                            ▼
              Google Video Intelligence API
              (submit → retorna operation_id)
                            │
                    poll a cada 5s até done
                            │
                            ▼
              JSON com: labels, shot changes,
              transcrição (se ativada), OCR
                            │
                            ▼
              Concatena com business_context do usuário
                            │
                            ▼
              Claude API (system prompt = content-strategy)
                            │
                            ▼
              Insights estruturados → salva no Postgres
                            │
                            ▼
              Frontend exibe resultado
                            │
              /tmp/{job_id}.mp4 é deletado
```

### Endpoints da API

```
POST   /api/auth/register
POST   /api/auth/login
GET    /api/auth/me

PUT    /api/user/context          # salva questionário de contexto de negócio

POST   /api/analyze               # inicia análise → retorna { job_id }
GET    /api/analyze/:job_id       # polling → { status, progress_msg, result? }
GET    /api/analyses              # histórico de análises do usuário

POST   /api/subscribe             # integração Pagar.me
GET    /api/subscription/status
```

### Schema do Postgres

```sql
CREATE TABLE users (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email       TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE business_contexts (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id       UUID REFERENCES users(id) ON DELETE CASCADE,
  brand_name    TEXT,
  description   TEXT,          -- "o que você faz em uma frase"
  target_audience TEXT,
  platforms     TEXT[],        -- ['tiktok', 'instagram']
  goals         TEXT[],        -- seleções do questionário
  main_pain     TEXT,          -- dor do cliente final
  content_history TEXT,        -- o que já funcionou
  updated_at    TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE analyses (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id       UUID REFERENCES users(id),
  status        TEXT DEFAULT 'processing', -- processing | done | error
  input_type    TEXT,          -- 'url' | 'upload'
  input_url     TEXT,
  mode          TEXT,          -- 'pre_post' | 'reference' | 'post_mortem'
  metrics_input JSONB,         -- views, watch_time, completion_rate (manual)
  gvi_result    JSONB,         -- raw output da Google Video Intelligence
  claude_result JSONB,         -- insights estruturados
  progress_msg  TEXT,
  error_msg     TEXT,
  created_at    TIMESTAMPTZ DEFAULT now(),
  completed_at  TIMESTAMPTZ
);

CREATE TABLE subscriptions (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     UUID REFERENCES users(id),
  status      TEXT,            -- trial | active | cancelled
  plan        TEXT,            -- 'basic' | 'pro'
  trial_ends_at TIMESTAMPTZ,
  created_at  TIMESTAMPTZ DEFAULT now()
);
```

---

## 4. Download de Vídeos via yt-dlp

### Chamada no backend Go

```go
func downloadVideo(url string, jobID string) (string, error) {
    outputPath := fmt.Sprintf("/tmp/videos/%s.mp4", jobID)
    
    cmd := exec.Command("yt-dlp",
        "--output", outputPath,
        "--format", "mp4/bestvideo[ext=mp4]+bestaudio[ext=m4a]/best[ext=mp4]",
        "--max-filesize", "500m",
        "--quiet",
        url,
    )
    
    if err := cmd.Run(); err != nil {
        return "", fmt.Errorf("yt-dlp failed: %w", err)
    }
    
    return outputPath, nil
}
```

### Considerações sobre yt-dlp

- Funciona para vídeos **públicos** de TikTok e Instagram
- Viola ToS das plataformas — risco aceitável para MVP com volume baixo
- Vídeos privados ou não publicados: usuário faz upload direto do arquivo
- Sempre deletar o arquivo após análise (`defer os.Remove(filePath)`)
- Limite de 500MB por vídeo é mais que suficiente para qualquer vídeo de TikTok/Reels

---

## 5. Integração Google Video Intelligence API

### Features e custos

| Feature | Free tier | Custo após free |
|---------|-----------|----------------|
| LABEL_DETECTION | 1.000 min/mês | ~$0.001/min |
| SHOT_CHANGE_DETECTION | 1.000 min/mês | ~$0.001/min |
| SPEECH_TRANSCRIPTION | Pago | ~$0.048/min |
| TEXT_DETECTION (OCR) | Pago | ~$0.015/15s |

### Estratégia de features

**MVP (créditos gratuitos Google Cloud):**
- LABEL_DETECTION + SHOT_CHANGE_DETECTION ativos
- SPEECH_TRANSCRIPTION e TEXT_DETECTION comentados (ativar quando créditos acabarem e tiver receita)

**Estimativa de custo por análise (vídeo de 60s):**
- Labels + Shots: grátis até 1k min/mês, depois ~$0.002
- Speech: ~$0.048
- OCR: ~$0.06
- Claude API: ~$0.005-0.015
- **Total: ~$0.11-0.13 por análise (com todas as features)**

**Com free tier Google + créditos iniciais:** praticamente R$0 para os primeiros meses.

### Código de referência (já existente)

O arquivo `tools/analyze-video.js` (anexado na conversa original) já implementa:
- Submit do vídeo para a API
- Processamento de labels por shot e por frame
- Processamento de shot changes e cálculo de ritmo
- Geração de `contentInsights` estruturado

Portar essa lógica para Go ou chamar via subprocess Node.js como wrapper temporário.

### Output estruturado que vai para o Claude

```json
{
  "metadata": {
    "duration": "47.3s",
    "features": ["LABEL_DETECTION", "SHOT_CHANGE_DETECTION"]
  },
  "shotChanges": {
    "total": 18,
    "averageDuration": 2.63,
    "rhythm": "medium",
    "cutsPerSecond": 0.38
  },
  "labels": {
    "videoLevel": ["person", "smartphone", "text", "indoor"],
    "firstShotLabels": ["face", "person", "selfie"],
    "firstFrameLabels": ["face", "person", "indoor", "clothing"]
  },
  "contentInsights": {
    "firstShotLabels": ["face", "person"],
    "dominantLabels": ["person", "text", "smartphone"],
    "rhythm": "medium"
  }
}
```

---

## 6. Sistema de Polling (sem fila, sem Redis)

### Backend Go — job runner

```go
// Quando POST /api/analyze é chamado:
func startAnalysis(jobID string, userID string, inputPath string, mode string, metrics map[string]interface{}) {
    // Atualiza status no Postgres
    go func() {
        updateJob(jobID, "processing", "Enviando vídeo para análise...")
        
        gviResult, err := runGoogleVideoIntelligence(inputPath)
        if err != nil {
            updateJobError(jobID, err.Error())
            return
        }
        
        updateJob(jobID, "processing", "Gerando insights com IA...")
        
        ctx := getUserBusinessContext(userID)
        claudeResult, err := runClaudeAnalysis(gviResult, ctx, mode, metrics)
        if err != nil {
            updateJobError(jobID, err.Error())
            return
        }
        
        updateJobDone(jobID, gviResult, claudeResult)
        os.Remove(inputPath) // limpa arquivo temporário
    }()
}
```

### Frontend React — polling

```javascript
const pollAnalysis = async (jobId) => {
  const interval = setInterval(async () => {
    const res = await fetch(`/api/analyze/${jobId}`)
    const data = await res.json()
    
    setProgressMsg(data.progress_msg)
    
    if (data.status === 'done') {
      clearInterval(interval)
      setResult(data.result)
    }
    
    if (data.status === 'error') {
      clearInterval(interval)
      setError(data.error_msg)
    }
  }, 3000)
}
```

### Mensagens de progresso sugeridas

```
"Baixando vídeo..."          (yt-dlp)
"Analisando estrutura visual..."  (GVI submetido)
"Identificando padrões de corte..." (aguardando GVI)
"Gerando insights com IA..."  (Claude rodando)
"Finalizando análise..."      (salvando resultado)
```

---

## 7. Prompt Engineering — Claude API

### System prompt base

O arquivo `content-strategy.md` (anexado na conversa) vira o system prompt. Ele contém:
- Persona: Analista de Marketing Sênior
- Frameworks de análise (hook, estrutura, neuromarketing)
- Benchmarks de vídeos anteriores
- Regras de conteúdo
- Instruções para cada caso de uso

### Como concatenar contexto

```go
systemPrompt := contentStrategyMD // conteúdo do arquivo content-strategy.md

userMessage := fmt.Sprintf(`
## Contexto do negócio do usuário
- Marca: %s
- O que faz: %s
- Público-alvo: %s
- Plataforma principal: %s
- Dor do cliente: %s
- O que já funcionou: %s

## Modo de análise
%s

## Métricas (se fornecidas)
%s

## Dados extraídos do vídeo (Google Video Intelligence)
%s

---

Com base em tudo acima, gere a análise completa conforme o caso de uso identificado.
Seja direto, franco e acionável. Sem elogios vazios.
`,
    ctx.BrandName, ctx.Description, ctx.TargetAudience,
    ctx.Platforms, ctx.MainPain, ctx.ContentHistory,
    modeDescription(mode),
    formatMetrics(metrics),
    string(gviResultJSON),
)
```

### Output estruturado do Claude

Pedir resposta em JSON para facilitar renderização no frontend:

```
Responda APENAS em JSON válido, sem texto antes ou depois, sem markdown.
Estrutura:
{
  "hook_analysis": { "score": 1-10, "why": "...", "improvement": "..." },
  "structure_analysis": { "framework_match": "...", "retention_issues": [...] },
  "visual_analysis": { "rhythm": "...", "first_frame": "...", "dominant_labels": [...] },
  "key_insights": ["insight 1", "insight 2", "insight 3"],
  "action_items": ["ação 1", "ação 2"],
  "replication_script": "..." // apenas no modo 'reference'
  "verdict": "vai bombar | ok | vai flopar",
  "verdict_reason": "..."
}
```

---

## 8. Questionário de Contexto de Negócio

Preenchido uma vez pelo usuário no onboarding. Salvo na tabela `business_contexts`. Campos:

| Campo | Tipo | Exemplo |
|-------|------|---------|
| brand_name | texto | "ScrapJobs" |
| description | texto longo | "Motor de busca de vagas tech com IA para devs" |
| target_audience | texto longo | "Devs pleno/sênior buscando vagas remotas em big techs" |
| platforms | multi-select | ["tiktok", "instagram"] |
| goals | multi-select | ["entender por que meus vídeos não viralizam", ...] |
| main_pain | texto longo | "Devs perdem vagas porque chegam tarde, a vaga fecha em horas" |
| content_history | texto longo (opcional) | "Storytelling pessoal funciona muito mais que demo" |

---

## 9. Modos de Análise

O usuário seleciona o modo antes de submeter o vídeo:

### Modo 1: Pré-postagem (`pre_post`)
- Input: arquivo de vídeo (upload obrigatório — não postado ainda)
- Sem métricas
- Foco: hook, estrutura, ritmo, primeiros 3 segundos, CTA
- Pergunta central: "vale a pena postar esse vídeo como está?"

### Modo 2: Análise de referência (`reference`)
- Input: URL de vídeo viral de outra pessoa
- Sem métricas
- Foco: por que viralizou, o que é replicável, como adaptar ao contexto do usuário
- Output extra: roteiro sugerido adaptado ao negócio do usuário

### Modo 3: Post-mortem (`post_mortem`)
- Input: URL ou upload de vídeo já postado
- Métricas opcionais (views, avg watch time, completion rate, followers ganhos)
- Foco: diagnóstico do que funcionou ou não, comparação com benchmarks
- Pergunta central: "por que esse vídeo performou assim e o que fazer diferente?"

---

## 10. Regras de Negócio

### Limites por plano (a definir, sugestão inicial)

| Plano | Preço | Análises/mês | Modo reference |
|-------|-------|-------------|----------------|
| Free / Trial | R$0 (7 dias) | 3 análises | Sim |
| Básico | R$29/mês | 20 análises | Sim |
| Pro | R$49/mês | 60 análises | Sim |

### Regras gerais

- Arquivo de vídeo deletado imediatamente após análise (nunca armazenado permanentemente)
- Tamanho máximo de upload: 500MB
- Duração máxima de vídeo: sem limite técnico, mas GVI cobra por minuto
- Vídeos privados só via upload — yt-dlp não acessa vídeos privados
- Uma análise por vez por usuário (evitar abuso)
- Timeout de análise: 5 minutos — se GVI não responder, marca como erro

### Não implementar no MVP

- Histórico comparativo entre vídeos (V2)
- Análise de perfil completo via OAuth (V2)
- Integração TikTok API (V3 ou nunca — muito restritivo)
- Notificação por email quando análise terminar (desnecessário — usuário espera na tela)
- Dashboard de analytics do próprio produto (V2)

---

## 11. Infraestrutura Railway

### Dockerfile sugerido (para incluir yt-dlp)

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o server ./cmd/server

FROM python:3.12-slim
RUN pip install yt-dlp --break-system-packages
COPY --from=builder /app/server /server
RUN mkdir -p /tmp/videos
EXPOSE 8080
CMD ["/server"]
```

### Variáveis de ambiente necessárias

```env
DATABASE_URL=postgresql://...        # Railway provisiona automaticamente
GOOGLE_APPLICATION_CREDENTIALS=...  # JSON da service account GCP (base64 ou arquivo)
ANTHROPIC_API_KEY=...
PAGARME_API_KEY=...
RESEND_API_KEY=...
JWT_SECRET=...
PORT=8080
```

---

## 12. Sequência de Desenvolvimento Recomendada

### Sprint 1 — Core funcional (sem auth, sem pagamento)
1. Endpoint `POST /api/analyze` recebendo arquivo MP4
2. Integração Google Video Intelligence API (port do `analyze-video.js` para Go)
3. Integração Claude API com system prompt do `content-strategy.md`
4. Endpoint `GET /api/analyze/:job_id` com polling
5. Frontend mínimo: input de arquivo + loading com progress msg + exibição do resultado

### Sprint 2 — Download por URL + questionário
6. Integração yt-dlp para download por URL
7. Tela de questionário de contexto de negócio
8. Salvar `business_context` e usar nas análises
9. Três modos de análise (pre_post, reference, post_mortem)

### Sprint 3 — Auth + pagamento
10. Autenticação (JWT simples, sem OAuth por enquanto)
11. Integração Pagar.me (trial 7 dias + planos)
12. Rate limiting por plano (N análises/mês)
13. Histórico de análises do usuário

### Sprint 4 — Polish + deploy
14. UI/UX de resultado estruturado (cards por seção de insight)
15. Deploy no Railway com Dockerfile correto
16. Variáveis de ambiente em produção
17. Primeiro usuário real testando

---

## 13. Riscos e Mitigações

| Risco | Probabilidade | Mitigação |
|-------|--------------|-----------|
| yt-dlp bloqueado por rate limit | Média | Retry com delay; fallback para upload manual |
| GVI demora mais que 5 min | Baixa | Timeout + mensagem de erro amigável |
| Custo GVI escala inesperadamente | Baixa no início | Monitorar no console GCP; deixar speech/OCR desativado no MVP |
| Claude gera insight genérico sem contexto suficiente | Média | Questionar usuário se não preencheu o contexto; prompt engineering iterativo |
| Railway free tier tem limite de memória | Média | Deletar arquivo de vídeo imediatamente após análise; processar um job por vez |

---

## 14. Arquivos de Referência Disponíveis

- `tools/analyze-video.js` — implementação completa da integração com Google Video Intelligence API (Node.js). Base para port para Go.
- `docs/content-strategy.md` — system prompt completo do analista de conteúdo. Usar diretamente como system prompt da Claude API.
- `docs/business-strategy.md` — contexto de negócio do ScrapJobs (exemplo de cliente-alvo do produto).

---

## 15. Primeira Conversa com Claude Code

Ao abrir o Claude Code, compartilhar este documento e os três arquivos de referência. Sugestão de prompt inicial:

```
Tenho um projeto novo para construir. Leia o arquivo video-analyzer-spec.md 
completo antes de qualquer coisa. Vamos começar pela Sprint 1: 
estrutura de pastas do projeto Go + endpoint POST /api/analyze 
recebendo um arquivo MP4 e chamando a Google Video Intelligence API. 
O arquivo analyze-video.js tem a lógica de referência em Node.js para portar.
```

---

*Documento gerado em maio/2026 com base na conversa de arquitetura do produto.*
