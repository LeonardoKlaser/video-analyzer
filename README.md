# Video Analyzer

SaaS de análise de vídeos pra criadores. Backend Go + frontend React + Postgres + GCS + Claude.

## Quick start (dev)

```bash
# Postgres local
docker compose up -d postgres

# Backend
cd api && cp ../.env.example ./.env && go run ./cmd/server

# Frontend
cd web && npm install && npm run dev
```

Specs: `docs/superpowers/specs/`. Plan: `docs/superpowers/plans/`.
