# Video Analyzer

## Stack
- Backend: Go
- Frontend: React
- DB: PostgreSQL (Railway)
- Hosting: Railway

## Comandos principais
- `go run ./cmd/server` — sobe o servidor
- `go test ./...` — roda os testes

## Arquitetura
- API REST em Go
- Jobs de análise processados em goroutines
- Polling via GET /api/analyze/:job_id

## Convenções
- Nomes de variáveis em inglês
- Erros sempre wrappados com fmt.Errorf("context: %w", err)
- Deletar arquivos de vídeo temporários sempre com defer

## Contexto
Ler video-analyzer-spec.md para entender toda a arquitetura do produto.