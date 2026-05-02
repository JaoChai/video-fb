# video-fb

Automated video content pipeline for Ads Vance — Go backend + React dashboard.

## Stack
- **Backend:** Go 1.25, chi router, pgx/v5 (Neon PostgreSQL), robfig/cron (scheduler)
- **Frontend:** React 19, Vite 8, TanStack Query, React Router
- **External:** OpenRouter (LLM), Kie AI (video), OpenRouter/Gemini TTS (voice), Jina AI (scraping), Zernio (publishing + analytics)

## Commands

```bash
# Backend
make run              # Start server on :8080
make build            # Build binary to bin/server
make test             # Run all Go tests
make migrate          # Run DB migrations

# CLI modes (alternative to server)
go run cmd/server/main.go -migrate         # Run database migrations
go run cmd/server/main.go -crawl           # Crawl knowledge sources
go run cmd/server/main.go -produce 7       # Produce N clips
go run cmd/server/main.go -publish         # Publish ready clips
go run cmd/server/main.go -analytics       # Fetch analytics from Zernio

# Frontend
cd frontend && npm install
cd frontend && npm run dev                 # Vite dev server
cd frontend && npm run build               # TypeScript check + Vite build
```

## Architecture

```
cmd/server/main.go          # Entry point — server or CLI mode via flags
internal/
  config/                   # Env var loading (.env via godotenv)
  database/                 # pgxpool connection + migration runner
  models/                   # Shared domain types (Clip, Scene, ClipAnalytics, etc.)
  router/                   # chi routes — all under /api/v1/* (see router.go for full list)
  handler/                  # HTTP handlers + API key middleware
  repository/               # DB queries (clips, scenes, themes, agents, analytics, etc.)
  agent/                    # LLM agents via OpenRouter (question, script, image)
  analyzer/                 # Analytics-driven agent self-improvement (weekly)
  rag/                      # RAG engine for knowledge retrieval
  crawler/                  # Knowledge source crawler
  orchestrator/             # Pipeline: question → script → image → produce
  producer/                 # Video production (Kie AI + TTS + FFmpeg assembly)
  publisher/                # Zernio publishing + analytics fetching
  scheduler/                # Cron-based scheduler (robfig/cron, DB config, Asia/Bangkok)
frontend/src/
  App.tsx                   # Main layout with sidebar (QueryClient: staleTime 30s)
  pages/                    # Content, Schedules, Agents, Knowledge, Analytics, PromptHistory, Settings
  api.ts                    # API client for backend endpoints
migrations/                 # SQL migration files (001-012)
.github/workflows/          # GitHub Actions — auto-deploy to Railway on push to master
```

## Environment Variables

See `.env.example`. Required: `DATABASE_URL`, `API_KEY`.
Optional: `PORT` (8080), `FFMPEG_PATH` (ffmpeg).
API keys for OpenRouter, Kie, and Zernio are managed via the Settings page (DB `settings` table), not env vars.

## Pipeline Flow

QuestionAgent → ScriptAgent → ImageAgent → Producer (Kie + FFmpeg) → Publisher (Zernio)
Weekly: `fetch_analytics` pulls YouTube stats via Zernio → Analyzer sends to LLM → auto-tunes agent prompts

## Deployment
- **Auto-deploy:** Push to `master` → GitHub Actions → Railway (`adsvance-v2` + `adsvance-frontend`)
- **Manual:** `railway up --service adsvance-v2` / `railway up --service adsvance-frontend`
- **Region:** `asia-southeast1-eqsg3a` (Singapore)

## Gotchas
- Server and CLI modes are mutually exclusive — flags like `-produce` exit after completion
- Frontend has no lint script — only `tsc && vite build` for type checking
- Scheduler reads cron config from `schedules` table — changing via API requires server restart
- ImageAgent must NOT generate logo/mascot/watermark — enforced in code + DB agent config
- Analyzer requires at least 3 published clips with analytics before running — skips silently
- Knowledge list endpoint returns summaries only (no content field) — use GET `/{id}` for full content
- Migrations can delete/recreate schedules — always verify `fetch_analytics` exists after new migrations
