# UX Overhaul — Design Spec
_2026-05-20_

## Context

The current form mixes business context (brand, audience, platforms) with every analysis submission. All three modes share one monolithic form. The reference mode lacks the depth needed to generate a useful replication script. The post-mortem verdict label doesn't fit a video that's already been posted.

## Goals

1. Separate business context from the analysis flow — fill once, reuse forever
2. Give each mode its own focused form
3. Make the reference mode the anchor of the user journey and dramatically improve its output quality
4. Fix the post-mortem verdict and surface benchmark comparisons

---

## Section 1 — Profile & First-Run Experience

### Profile page
- New route accessible via a "Configurações" link in the sidebar, below the user email and above "sair"
- Contains the existing business context form: brand name, description, target audience, platforms, main pain, content history
- Save button calls `PUT /api/auth/me` (already implemented)
- On success: shows inline confirmation, no redirect

### First-run banner
- Condition: user is logged in AND `user.business_context` is null or all fields empty
- Renders a dismissible top banner on the main view: _"Configure seu perfil para análises personalizadas →"_ linking to the profile page
- Banner disappears permanently once the profile is saved (re-check `user.business_context` after save)
- User can ignore the banner and submit analyses — Claude receives empty context and produces a generic analysis. This is acceptable.

---

## Section 2 — Form Architecture

### Mode ordering (updated to match the natural creator journey)
1. **Referência** — watch a viral video, plan your content
2. **Pré-postagem** — recorded, evaluate before posting
3. **Post-mortem** — already posted, diagnose results

### Component structure
`AnalysisForm` becomes a shell that renders one of three sub-forms based on the selected mode:

| Component | Fields |
|---|---|
| `ReferenceForm` | Video upload, views (optional), likes (optional), "O que você quer gravar?" (required) |
| `PrePostForm` | Video upload, "Qual era o conceito/gancho planejado?" (optional) |
| `PostMortemForm` | Video upload, views, likes, avg watch time, completion rate, followers gained (all optional) |

Business context is **not shown in any form** — always read from saved profile. If profile is empty, a soft inline note appears: _"Perfil não configurado — análise será genérica"_.

### API request changes
`StartAnalyzeRequest` gains one new optional field:
```
user_concept?: string   // "what you want to create" (reference) or "planned hook" (pre-post)
```
Sent as-is to the backend; not stored in the DB — only used to build the Claude prompt.

---

## Section 3 — Reference Mode (Deepest Changes)

### Form
- Upload field: label clarified to "Vídeo viral de referência"
- Views + likes: already present, kept as optional
- New required field: **"O que você quer gravar?"** — free text, ~2-3 sentences. Placeholder: _"Ex: quero falar sobre como economizar R$500/mês com compras no mercado, meu público é jovens adultos que querem organizar as finanças"_

### Claude prompt update (`modeDescription` for `reference`)
Current: _"Vídeo viral de terceiro pra usar como referência. Foco: por que viralizou + como replicar no contexto do usuário."_

New:
```
Vídeo viral de terceiro usado como referência. Analise em três camadas:

1. POR QUE VIRALIZOU — identifique os mecanismos psicológicos e de neuromarketing
   que explicam a performance (ex: loop aberto, prova social, contraste, escassez,
   identidade tribal). Seja específico: qual frame/fala ativa cada princípio.

2. ELEMENTOS VIRAIS — liste os elementos concretos replicáveis (estrutura de hook,
   ritmo de corte, tipo de abertura, uso de texto na tela, CTA implícito/explícito).

3. ROTEIRO PERSONALIZADO — usando o conceito declarado pelo usuário e o contexto
   do negócio dele, escreva um roteiro completo que incorpora os elementos virais
   identificados. Justifique cada escolha estrutural com o princípio de neuromarketing
   correspondente.

O campo replication_script é OBRIGATÓRIO neste modo.
```

### Output schema additions (reference mode only)
Two new fields added to the JSON schema and to `claude.Result`:
```json
"neuromarketing_refs": ["Loop aberto no hook", "Prova social implícita", "Contraste visual"],
"viral_elements": ["Abertura com pergunta retórica", "Corte a cada 2s nos primeiros 10s", "CTA embutido na penúltima cena"]
```
Both are arrays of strings. Required when mode is `reference`, omitted otherwise.

### Result display (reference mode)
New section **"Por que viralizou"** rendered before "Roteiro adaptado":
- `neuromarketing_refs` displayed as tagged pills
- `viral_elements` as a numbered list
- "Roteiro adaptado" section gets a larger heading and more visual weight since it's the primary output

---

## Section 4 — Pre-Post & Post-Mortem Improvements

### Pre-post
- New optional field: **"Qual era o conceito/gancho planejado?"** (`user_concept`)
- When provided, appended to the Claude prompt: _"O criador planejava: {user_concept}. Avalie se o hook executado bate com essa intenção."_
- No schema changes — this surfaces in `hook_analysis.why` and `action_items` naturally

### Post-mortem — verdict labels
Replace `"vai bombar" | "ok" | "vai flopar"` with `"performou bem" | "na média" | "abaixo do esperado"` for post-mortem analyses only.

- Backend: `modeDescription` for `post_mortem` instructs Claude to use the new labels
- Frontend: `AnalysisResult` detects `mode === 'post_mortem'` and maps verdict to the correct style/icon. The `VERDICT_STYLE` map gains aliases for the three new labels (same green/amber/red palette).

### Post-mortem — benchmark comparison in result
- The Claude prompt for post-mortem already includes benchmarks in the system prompt
- Add the user's metric values explicitly to the user message alongside the benchmark targets so Claude can compare inline
- `verdict_reason` will naturally include the comparison
- Frontend: the metrics block in `AnalysisResult` for post-mortem shows each provided metric with its benchmark value below it (small zinc-500 text: _"benchmark TikTok: ~45%"_). Benchmarks are hardcoded on the frontend per platform.

---

## What Does Not Change

- Auth flow, JWT, sidebar history — untouched
- GCS upload, signed URL flow — untouched
- `analyses` DB schema — no new columns (user_concept is prompt-only, not persisted)
- `business_context` on analysis row — still saved per-analysis as today (snapshot at submit time)
- Visual design language (zinc/emerald palette, font-mono labels) — preserved throughout

---

## Files Affected

| File | Change |
|---|---|
| `web/src/types.ts` | Add `user_concept?: string` to `StartAnalyzeRequest`; add `neuromarketing_refs`, `viral_elements` to `ClaudeResult`; update `Mode` order convention; add post-mortem verdict union type |
| `web/src/api.ts` | Pass `user_concept` in `startAnalyze` |
| `web/src/App.tsx` | Add profile page view state; first-run banner logic; wire `user_concept` through `handleSubmit` |
| `web/src/components/AnalysisForm.tsx` | Refactor into shell + 3 sub-form components |
| `web/src/components/ReferenceForm.tsx` | New |
| `web/src/components/PrePostForm.tsx` | New |
| `web/src/components/PostMortemForm.tsx` | New |
| `web/src/components/ProfilePage.tsx` | New — extracted business context form |
| `web/src/components/AnalysesSidebar.tsx` | Add "Configurações" link |
| `web/src/components/AnalysisResult.tsx` | Add neuromarketing_refs + viral_elements sections; post-mortem verdict mapping; benchmark display |
| `api/internal/claude/prompts.go` | Update `modeDescription` for reference and post-mortem; add `user_concept` to `BuildUserMessage` |
| `api/internal/claude/types.go` | Add `NeuromarketingRefs`, `ViralElements` to `Result` |
| `api/internal/models/analysis.go` | Add `UserConcept string` to request model |
| `api/internal/handlers/analyze.go` | Read `user_concept` from request body, pass to prompt builder |
