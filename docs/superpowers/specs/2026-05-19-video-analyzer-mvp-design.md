# Video Analyzer — MVP Design (Deploy de 2026-05-19)

> Spec consolidado do brainstorm. Define o escopo, arquitetura, fluxo, código e deploy do MVP de um dia. Substitui (pra fins de implementação) as Sprints 1-4 do `video-analyzer-spec.md` original — que continua valendo como referência de produto, mas não como roadmap deste dia.

---

## 0. Escopo deste MVP

### O que entra

- Backend Go (`api`) + frontend React/Vite (`web`) + Postgres (Railway plugin) — **três serviços separados** no mesmo projeto Railway, espelhando o padrão do ScrapJobs do founder.
- Upload de vídeo MP4 **direto do browser pro GCS** via signed URL (vídeo nunca passa pelo processo Go).
- Análise via **Google Video Intelligence** reusando `analyze-video.js` como subprocess no container `api` — modificado pra ler `inputUri` do GCS em vez de `inputContent` base64.
- Insights via **Claude API** (modelo `claude-sonnet-4-6`), recebendo o JSON do GVI + contexto de negócio + métricas (quando aplicável).
- **Três modos de análise** (`pre_post`, `reference`, `post_mortem`) acessíveis via seletor no form.
- **Polling** de status pelo frontend (intervalo 3s) com mensagens de progresso visíveis.
- **Histórico** simples de análises anteriores (sidebar com lista, click → exibe).
- Deploy real no Railway com URL acessível.

### O que NÃO entra hoje

- **Auth / users** — single-user demo, sem JWT, sem login.
- **Pagamento (Pagar.me) e planos** — sem rate limit por usuário.
- **Persistência do `business_context`** em tabela própria — vai como snapshot JSONB dentro de cada análise.
- **yt-dlp / download por URL** — só upload de arquivo. URL pública fica pra V2.
- **Features pagas do GVI** (SPEECH_TRANSCRIPTION, TEXT_DETECTION) — só `LABEL_DETECTION` e `SHOT_CHANGE_DETECTION` (free tier).
- **Notificações por email, dashboards de analytics, integração TikTok/Instagram API** — fora de escopo.

### Critério de aceite ("deployado hoje")

1. URL pública do `web` no Railway acessível pela internet.
2. Founder consegue: subir um MP4 de teste → ver progress → receber `claude_result` estruturado em <5min.
3. Vídeo apagado do GCS após análise.
4. Row persistida no Postgres com `status='done'`.
5. Refresh da página mostra o histórico, click numa entrada exibe o resultado anterior.

---

## 1. Arquitetura

```
┌──────────────── Railway project: video-analyzer ────────────────┐
│                                                                   │
│   service: web (React/Vite)         service: api (Go + Node)     │
│       │                                  │                       │
│       │     plugin: postgres ◀───────────┤                       │
│       │                                  │                       │
└───────┼──────────────────────────────────┼──────────────────────┘
        │                                  │
        │ HTTPS                            │ HTTPS
        │                                  │
        │   ┌──── browser ────┐            │
        └──▶│  1. GET signed  │◀── api: cloud.google.com/go/storage
            │     URL         │            │
            │  2. PUT file ───┼───────────┐│
            │     direto pro  │           ││
            │     GCS         │           ▼▼
            │  3. POST /api/  │   ┌──────────────┐
            │     analyze     │   │ GCS bucket   │
            │     {gcs_uri}   │   │ video-       │
            │  4. poll status │   │ analyzer-tmp │
            └─────────────────┘   │ (lifecycle:  │
                                  │  delete 24h) │
                                  └──────┬───────┘
                                         │ inputUri
                                         ▼
                                   Google Video API
                                         │
                                         ▼
                                   resp JSON pequena
                                         │
                                         ▼
                                   api → Claude API → Postgres
```

### Princípios

- **Vídeo não passa pelo processo Go.** Upload direto browser→GCS via signed URL elimina o problema de RAM do `analyze-video.js` atual (que carregava o vídeo + base64 em memória).
- **Subprocess, não microsserviço.** `analyze-video.js` vive dentro do container `api` e é invocado via `exec.Command`. Sem 4º container, sem hop de rede.
- **Polling sem fila/worker.** Goroutine no mesmo processo Go faz o trabalho assíncrono. `analyses.status/progress_msg` é o único "broker" de estado.
- **Snapshot do contexto na análise.** `business_context` mora dentro da row da análise como JSONB. Imutabilidade histórica + adiamento da decisão de modelar.
- **Container fail-safe.** Em qualquer erro, `defer` apaga o objeto GCS. Lifecycle do bucket (24h) é a segunda linha de defesa.

---

## 2. Schema Postgres

```sql
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE analyses (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  status           TEXT NOT NULL DEFAULT 'processing'
                     CHECK (status IN ('processing', 'done', 'error')),
  mode             TEXT NOT NULL
                     CHECK (mode IN ('pre_post', 'reference', 'post_mortem')),

  gcs_uri          TEXT NOT NULL,
  original_name    TEXT,

  business_context JSONB NOT NULL,
  -- shape: { brand_name, description, target_audience, platforms[], main_pain, content_history }

  metrics_input    JSONB,
  -- shape: { views, avg_watch_time, completion_rate, followers_gained } — só pra post_mortem

  gvi_result       JSONB,
  claude_result    JSONB,

  progress_msg     TEXT,
  error_msg        TEXT,

  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at     TIMESTAMPTZ
);

CREATE INDEX idx_analyses_created_at ON analyses (created_at DESC);

CREATE INDEX idx_analyses_status_updated ON analyses (status, updated_at)
  WHERE status = 'processing';
```

### Aplicação

- `internal/db/init.sql` embebido via `go:embed`, executado no startup com `CREATE TABLE IF NOT EXISTS`. Idempotente.
- Quando o schema começar a evoluir, migrar pra `golang-migrate` ou `goose` com arquivos numerados.

### Validações no código Go (não no banco)

- Campos obrigatórios em `business_context`: `brand_name`, `description`, `target_audience`, `main_pain`. Validados no handler, retorno 400 com mensagem clara.
- `metrics_input` só é aceito quando `mode = 'post_mortem'`.
- `gcs_uri` precisa começar com `gs://${GCS_BUCKET}/`.
- `mode` é um dos três valores do CHECK.

### Upgrade path

| Quando | O que muda no schema |
|---|---|
| Entra auth | `CREATE TABLE users (...)`; `ALTER TABLE analyses ADD COLUMN user_id UUID REFERENCES users(id)` nullable; backfill ou seed |
| Persistir contexto reusável | `CREATE TABLE business_contexts (...)`; `ALTER TABLE analyses ADD COLUMN business_context_id UUID REFERENCES business_contexts(id)` nullable; manter `business_context JSONB` como snapshot histórico |
| Entra Pagar.me | `CREATE TABLE subscriptions (...)` |

---

## 3. API HTTP

### `POST /api/uploads/signed-url`

```json
// req
{ "filename": "meu-video.mp4", "content_type": "video/mp4" }

// resp 200
{
  "put_url": "https://storage.googleapis.com/...?X-Goog-Signature=...",
  "gcs_uri": "gs://video-analyzer-tmp/2026-05-19/a3f8...mp4",
  "expires_at": "2026-05-19T22:30:00Z"
}
```

- Gera object key com prefixo `YYYY-MM-DD/{uuid}{ext}` pra facilitar inspeção.
- Signed URL válido por 15 minutos.
- Sem side-effects no banco — só assinatura.

### `POST /api/analyze`

```json
// req
{
  "gcs_uri": "gs://video-analyzer-tmp/2026-05-19/a3f8...mp4",
  "original_name": "meu-video.mp4",
  "mode": "pre_post",
  "business_context": {
    "brand_name": "ScrapJobs",
    "description": "Motor de busca de vagas tech com IA pra devs",
    "target_audience": "Devs pleno/sênior buscando vagas remotas em big techs",
    "platforms": ["tiktok", "instagram"],
    "main_pain": "Devs perdem vagas que fecham em horas",
    "content_history": "Storytelling pessoal funciona mais que demo"
  },
  "metrics": null
}

// resp 200
{ "id": "uuid-v4", "status": "processing" }
```

- Valida payload, INSERT row, dispara `go runAnalysis(id)`.
- `metrics` é obrigatório como `null` quando não-`post_mortem`, ou objeto com pelo menos um campo quando `post_mortem`.

### `GET /api/analyze/:id`

```json
// resp 200 (in-progress)
{
  "id": "uuid",
  "status": "processing",
  "mode": "pre_post",
  "progress_msg": "Analisando estrutura visual...",
  "created_at": "2026-05-19T22:00:00Z"
}

// resp 200 (done)
{
  "id": "uuid",
  "status": "done",
  "mode": "pre_post",
  "created_at": "...",
  "completed_at": "...",
  "result": { /* claude_result, ver §5 */ }
}

// resp 200 (error)
{
  "id": "uuid",
  "status": "error",
  "mode": "pre_post",
  "created_at": "...",
  "error": "Falha ao analisar vídeo: ..."
}

// resp 404 — id desconhecido
```

### `GET /api/analyses`

```json
// resp 200
[
  {
    "id": "uuid",
    "mode": "pre_post",
    "status": "done",
    "created_at": "...",
    "original_name": "meu-video.mp4",
    "verdict": "ok"     // pulled from claude_result.verdict (null se status != done)
  },
  ...
]
```

- Ordem `created_at DESC`.
- Sem paginação hoje (single-user, volume baixo).

---

## 4. Fluxo de execução do job

```go
// internal/jobs/runner.go (esqueleto)
func RunAnalysis(ctx context.Context, id uuid.UUID) {
    defer func() {
        // 1ª camada de cleanup do GCS — apaga em sucesso OU erro
        if uri := getGcsUri(id); uri != "" {
            _ = gcs.DeleteObject(uri)
        }
    }()

    setProgress(id, "Analisando estrutura visual...")
    gviJSON, err := analyzer.Run(ctx, getGcsUri(id))  // exec.Command com timeout 4min
    if err != nil {
        setError(id, fmt.Errorf("falha gvi: %w", err))
        return
    }
    saveGVI(id, gviJSON)

    setProgress(id, "Gerando insights com IA...")
    claudeResult, err := claude.Analyze(ctx, gviJSON, getContext(id))  // timeout 90s + 1 retry
    if err != nil {
        setError(id, fmt.Errorf("falha claude: %w", err))
        return
    }

    markDone(id, claudeResult)
}
```

### Transições de status

| De | Pra | Trigger |
|---|---|---|
| (insert) | `processing` (msg: "Iniciando análise...") | handler `POST /api/analyze` |
| `processing` | `processing` (msg: "Analisando estrutura visual...") | antes do exec do analyze-video.js |
| `processing` | `processing` (msg: "Gerando insights com IA...") | antes da chamada Claude |
| `processing` | `done` | Claude retornou JSON válido |
| `processing` | `error` | qualquer falha capturada |
| `processing` (>8min sem update) | `error` (msg: "Análise interrompida (timeout)") | watchdog |

### Watchdog

Goroutine que roda a cada 1min:

```sql
UPDATE analyses
SET status='error',
    error_msg='Análise interrompida (timeout)',
    updated_at=now()
WHERE status='processing'
  AND updated_at < now() - interval '8 minutes';
```

Cobre crashes do processo, restart do container e jobs órfãos.

### Tratamento de erro detalhado

| Cenário | O que faz |
|---|---|
| `node analyze-video.js` retorna não-zero | `status=error`, `error_msg="Falha ao analisar vídeo: <stderr resumido>"`, apaga GCS |
| GVI > 4min (timeout do exec) | `status=error`, `error_msg="Análise demorou demais. Tente um vídeo menor."` |
| Claude > 90s ou 5xx | retry 1x com backoff 5s; se falhar: `status=error`, `error_msg="Falha ao gerar insights. Tente novamente."` |
| Claude retorna JSON malformado | retry 1x com instrução reforçada; se falhar: `status=error` |
| Postgres cai no meio | goroutine loga e morre; row fica órfã → watchdog limpa em até 9min |

---

## 5. Integração Claude API

### System prompt

`content-strategy.md` do repo (já existe, é o analista sênior) é **copiado** para `api/internal/claude/content-strategy.md` no momento do build (ou mantido lá como cópia versionada) e embebido via `//go:embed content-strategy.md` dentro de `prompts.go`. Vai como `system` da API call do Claude. Como é embed, o conteúdo é baked no binário — não precisa estar no filesystem em runtime.

### User message — builder por modo

Estrutura comum:

```
## Contexto do negócio
- Marca: {brand_name}
- O que faz: {description}
- Público-alvo: {target_audience}
- Plataformas: {platforms joined ", "}
- Dor do cliente: {main_pain}
- O que já funcionou (histórico): {content_history}

## Modo de análise
{mode_description}      ← muda por modo, ver tabela abaixo

## Métricas
{formatted_metrics or "(não fornecidas)"}

## Dados extraídos do vídeo (Google Video Intelligence)
{gvi_result JSON}

---

Responda APENAS em JSON válido, sem texto antes ou depois, sem markdown.
Estrutura obrigatória: {schema_json}
```

| Modo | `mode_description` |
|---|---|
| `pre_post` | "Vídeo ainda não postado. Avalie se vale postar como está. Foco: hook nos primeiros 3s, estrutura, ritmo de corte, CTA. Pergunta central: 'vale a pena postar?'." |
| `reference` | "Vídeo viral de terceiro pra usar como referência. Foco: por que viralizou + como replicar no contexto do usuário. Inclua um `replication_script` com roteiro adaptado." |
| `post_mortem` | "Vídeo já postado. Diagnóstico do que funcionou ou não. Compare métricas com benchmarks (Caso 4 do system prompt). Foco: aprendizado pra próximos." |

### Output esperado do Claude (schema)

```json
{
  "hook_analysis": {
    "score": 1-10,
    "why": "...",
    "improvement": "..."
  },
  "structure_analysis": {
    "framework_match": "...",
    "retention_issues": ["..."]
  },
  "visual_analysis": {
    "rhythm": "...",
    "first_frame": "...",
    "dominant_labels": ["..."]
  },
  "key_insights": ["...", "...", "..."],
  "action_items": ["...", "..."],
  "replication_script": "...",   // só presente quando mode = "reference"
  "verdict": "vai bombar" | "ok" | "vai flopar",
  "verdict_reason": "..."
}
```

Parse no Go via `json.Unmarshal` em struct tipada. Validação leve: garante que campos obrigatórios existem; se faltar, retry com instrução reforçada.

---

## 6. Estrutura de código

### Monorepo

```
/workspace
├── api/
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── config/config.go
│   │   ├── db/{init.sql, db.go, analyses.go}
│   │   ├── handlers/{router.go, uploads.go, analyze.go, analyses.go}
│   │   ├── gcs/gcs.go
│   │   ├── analyzer/analyzer.go
│   │   ├── claude/{client.go, prompts.go, types.go}
│   │   ├── jobs/{runner.go, watchdog.go}
│   │   └── models/analysis.go
│   ├── tools/
│   │   ├── analyze-video.js       # ajustado pra inputUri
│   │   └── package.json
│   ├── internal/claude/content-strategy.md   # cópia versionada, embebida no binário
│   ├── go.mod / go.sum
│   └── Dockerfile
├── web/
│   ├── src/
│   │   ├── main.tsx / App.tsx
│   │   ├── api.ts / types.ts
│   │   └── components/{AnalysisForm, UploadProgress, AnalysisRunning, AnalysisResult, AnalysesSidebar}.tsx
│   ├── index.html / index.css
│   ├── package.json / vite.config.ts
│   ├── tailwind.config.ts / postcss.config.js
│   └── Dockerfile
├── docs/superpowers/specs/
│   └── 2026-05-19-video-analyzer-mvp-design.md  # este arquivo
├── CLAUDE.md
├── video-analyzer-spec.md
├── content-strategy.md
├── business-strategy.md
├── .gitignore                     # *.json (credentials), .DS_Store, node_modules, dist, /tmp
└── .env.example
```

**Observação:** o `tools/analyze-video.js` original em `/workspace/tools/` será **movido pra `/workspace/api/tools/`**. CLI local do founder continua funcionando, só muda o `cd`.

### Stack escolhida

#### Backend (Go)

| Coisa | Escolha |
|---|---|
| Router HTTP | `github.com/go-chi/chi/v5` |
| Postgres pool | `github.com/jackc/pgx/v5/pgxpool` |
| Logging | `log/slog` (stdlib) com JSON handler |
| GCS | `cloud.google.com/go/storage` |
| Claude | HTTP cru com `net/http` + `encoding/json` (Anthropic Go SDK ainda imaturo) |
| Validação | manual nos handlers |
| Migrations | `init.sql` via `go:embed`, idempotente |

#### Frontend (React)

| Coisa | Escolha |
|---|---|
| Bundler | Vite + TypeScript |
| Styling | Tailwind via PostCSS plugin oficial |
| Estado | `useState` / `useReducer` |
| Form | `useState` puro |
| Upload | `XMLHttpRequest` (necessário pra `upload.onprogress`) |
| Polling | `setInterval` em `useEffect` com cleanup |
| Serve container | Caddy 2 com `file_server` |

### Skill `frontend-design`

Durante implementação, invocar `frontend-design` pra:

- Definir o shape visual da página (hierarquia, espaçamento, tipografia)
- Renderização dos cards de resultado (hook score, action items, verdict — cada um com tratamento visual distinto)
- Estado de loading que dê dignidade ao tempo de espera (60-120s)

Os componentes definidos aqui são esqueleto funcional; a aparência vem da skill.

### Env vars (`.env.example`)

```bash
# api
DATABASE_URL=postgresql://...
PORT=8080
ALLOWED_ORIGINS=http://localhost:5173,https://web-xxxx.up.railway.app

GOOGLE_APPLICATION_CREDENTIALS_JSON=<base64 do JSON da service account>
GCS_BUCKET=video-analyzer-tmp
GCP_PROJECT_ID=gen-lang-client-0123498892

ANTHROPIC_API_KEY=<rotacionar primeiro; depois colar como Railway secret>
ANTHROPIC_MODEL=claude-sonnet-4-6

# web (build-time, injetado no Dockerfile via ARG)
VITE_API_URL=http://localhost:8080
```

**Workaround Railway pra service account:** carregar `GOOGLE_APPLICATION_CREDENTIALS_JSON` como string base64 numa env var, decodificar no startup do Go pra `/tmp/gcp-sa.json` e setar `GOOGLE_APPLICATION_CREDENTIALS` apontando pra esse path.

---

## 7. Testes

Estratégia honesta pra 1 dia. **Não é TDD pleno em tudo** — seria inviável. Cobertura cirúrgica nos pontos de maior risco.

| Camada | Estratégia |
|---|---|
| `internal/claude` — builder de prompt + parse de resposta | **TDD** com table-driven tests |
| `internal/db` — CRUD da `analyses` | **TDD** com Postgres real via `dockertest` ou DB de dev |
| `internal/handlers` — validação de payload | **Tests-after** focados em 4xx |
| `internal/gcs` — signed URL | **Test-after** com fake credentials |
| `internal/analyzer` — exec do subprocess | **Smoke test** com vídeo curto de fixture |
| `internal/jobs/runner` — orquestração e2e | **Integration test** com vídeo de 5s real (gasta ~$0.02) |
| Frontend | **Sem testes hoje.** Validação manual + iteração visual via skill |

### Regras

- **Sem mocks de Postgres.** Container real ou DB de dev.
- **Sem mocks de Claude/GVI em integration tests** — usa vídeo de fixture pequeno.
- **OK mockar Claude/GVI em unit tests** quando o foco é a lógica do nosso código.

---

## 8. Deploy (Railway)

```
1. Criar projeto Railway: video-analyzer

2. Adicionar plugin: PostgreSQL
   → DATABASE_URL gerado automaticamente

3. Criar service: api
   - Source: GitHub repo, root = /api
   - Build: Dockerfile auto-detectado
   - Env vars:
       DATABASE_URL=${{Postgres.DATABASE_URL}}
       ANTHROPIC_API_KEY=<chave rotacionada>
       ANTHROPIC_MODEL=claude-sonnet-4-6
       GOOGLE_APPLICATION_CREDENTIALS_JSON=<base64 do SA>
       GCS_BUCKET=video-analyzer-tmp
       GCP_PROJECT_ID=gen-lang-client-...
       ALLOWED_ORIGINS=<a definir depois do passo 4>
       PORT=8080
   - Generate domain → copiar URL (vira API_URL do web)

4. Criar service: web
   - Source: mesmo repo, root = /web
   - Build: Dockerfile com ARG VITE_API_URL=<url do api>
   - Generate domain → copiar URL e atualizar ALLOWED_ORIGINS do api

5. Configurar bucket GCS:
   - Criar gs://video-analyzer-tmp no projeto do SA
   - CORS:
       [{
         "origin": ["https://web-xxxx.up.railway.app"],
         "method": ["PUT"],
         "responseHeader": ["Content-Type"],
         "maxAgeSeconds": 3600
       }]
   - Lifecycle rule: deletar objetos com idade >24h

6. Service account permissions:
   - `roles/storage.objectAdmin` granted **no bucket** video-analyzer-tmp
   - `roles/iam.serviceAccountTokenCreator` granted **na própria service account** (gotcha:
     a SA precisa poder gerar tokens em nome de si mesma pra assinar signed URL v4 sem
     chave privada local — comando: `gcloud iam service-accounts add-iam-policy-binding
     <sa-email> --member="serviceAccount:<sa-email>" --role="roles/iam.serviceAccountTokenCreator"`)
   - Video Intelligence API habilitada no projeto
   - Confirmar billing ativado

7. Smoke test manual:
   - Abrir o domínio do web
   - Subir um MP4 de teste curto (5-15s)
   - Acompanhar progress
   - Ver claude_result renderizado
   - Verificar no Postgres: row com status='done', claude_result populado
   - Verificar no GCS: objeto deletado
   - Recarregar página → sidebar mostra a análise → clicar exibe resultado
```

### Dockerfiles (esqueletos)

```dockerfile
# api/Dockerfile
FROM golang:1.22-alpine AS gobuild
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /server ./cmd/server

FROM node:20-alpine
WORKDIR /app
COPY tools/package*.json ./tools/
RUN cd tools && npm ci --omit=dev
COPY tools/analyze-video.js ./tools/
COPY --from=gobuild /server /server
# content-strategy.md já está embebido no binário via //go:embed — não precisa COPY
EXPOSE 8080
CMD ["/server"]
```

```dockerfile
# web/Dockerfile
FROM node:20-alpine AS build
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
ARG VITE_API_URL
ENV VITE_API_URL=$VITE_API_URL
RUN npm run build

FROM caddy:2-alpine
COPY --from=build /app/dist /usr/share/caddy
COPY Caddyfile /etc/caddy/Caddyfile
```

```caddyfile
# web/Caddyfile — SPA com fallback pra index.html
:80 {
  root * /usr/share/caddy
  try_files {path} /index.html
  file_server
  encode gzip
}
```

---

## 9. Observabilidade

- `slog` em formato JSON, escrito em stdout — Railway captura.
- Middleware de request logging no chi: latência, status, path, request_id.
- Erros em `runAnalysis` logados com `job_id`, etapa, erro completo.
- Sem APM/Sentry hoje. Railway logs cobre.

---

## 10. Riscos e mitigações

| Risco | Mitigação |
|---|---|
| Chave Anthropic colada no chat ainda válida | **Rotacionar antes** de subir env var no Railway. Tarefa #9 do plano. |
| GCS CORS errado → upload do browser falha | Testar com `curl -v -X PUT` antes de subir frontend. Validar primeiro com `gsutil cors set`. |
| Service account sem `iam.serviceAccountTokenCreator` | Signed URL v4 precisa dessa role. Verificar antes do deploy. |
| Container `api` cresce demais | Multi-stage com alpine; alvo ~150MB. |
| GVI tarifa inesperada se vídeo for grande | LABEL+SHOT estão no free tier de 1000min/mês; monitorar console GCP. |
| Goroutine morre sem cleanup | Watchdog + GCS lifecycle (24h) cobrem. |
| Postgres do Railway com pool limit baixo | Single instance, pool max 10 conexões. Long way to go. |

---

## 11. Upgrade path (pós-MVP)

| Quando | Mudança |
|---|---|
| Founder quer compartilhar com outro user | Adiciona auth (JWT simples), `users` table, `user_id` em `analyses` |
| URL pública de TikTok/Reels precisa funcionar | yt-dlp via novo container `downloader`, ou subprocess no `api` (mais simples), feature flag |
| Rate limit / cobrança | Tabela `subscriptions`, integração Pagar.me, middleware de rate limit por plano |
| Contexto persistido | Tabela `business_contexts`, FK em `analyses`, mantém JSONB snapshot |
| Upload signed URL pesado pro frontend | Endpoint `/api/uploads/multipart` com resumable upload do GCS |
| Volume cresce | Migrar de subprocess `analyze-video.js` pra service Node separado; depois pra Cloud Run / Pub-Sub se precisar paralelizar |

---

## 12. Próximos passos

1. **Founder rotaciona chave Anthropic** (tarefa #9 — pré-requisito de deploy).
2. **Esta spec é convertida em plano de implementação** via skill `writing-plans`, com tarefas executáveis e ordenadas.
3. **Implementação começa** seguindo o plano. Skill `frontend-design` entra na hora de definir a UI da página principal e dos cards de resultado.
