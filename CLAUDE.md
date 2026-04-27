# video-fb

Automated video content pipeline for Ads Vance — Go backend + React dashboard.

## Stack
- **Backend:** Go 1.25, chi router, pgx/v5 (Neon PostgreSQL), robfig/cron (scheduler)
- **Frontend:** React 19, Vite 8, TanStack Query, React Router
- **External:** Claude API (scripts), Kie AI (video + voice via ElevenLabs), OpenRouter (embeddings), Jina AI (web scraping), Zernio (publishing)

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
go run cmd/server/main.go -analytics       # Fetch analytics

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
  models/                   # Shared domain types (Clip, Scene, etc.)
  router/                   # chi routes — all under /api/v1/*
  handler/                  # HTTP handlers + API key middleware
  repository/               # DB queries (clips, scenes, themes, agents, etc.)
  agent/                    # Claude API agents (question, script, image)
  analyzer/                 # Analytics-driven agent self-improvement (weekly)
  rag/                      # RAG engine for knowledge retrieval
  crawler/                  # Knowledge source crawler
  orchestrator/             # Pipeline: question → script → image → produce
  producer/                 # Video production (Kie AI + FFmpeg assembly)
  publisher/                # Zernio publishing + analytics
  scheduler/                # Cron-based scheduler (robfig/cron, reads config from DB, Asia/Bangkok timezone)
frontend/src/
  App.tsx                   # Main layout with sidebar navigation (QueryClient: staleTime 30s)
  pages/                    # Content, Schedules, Agents, Knowledge, Analytics, Settings
  api.ts                    # API client for backend endpoints
migrations/                 # SQL migration files (001–006)
.github/workflows/          # GitHub Actions — auto-deploy to Railway on push to master
```

## API Endpoints

All routes require `Authorization` header with API_KEY (except `/health`).
Base: `/api/v1/`

- `GET /health` — Health check (no auth required)
- `clips/` — CRUD for video clips
- `clips/{clipId}/scenes/` — Scenes per clip (GET, POST)
- `scenes/{id}` — Delete scene (DELETE, standalone route)
- `knowledge/sources/` — RAG knowledge sources (list summaries: GET, create: POST)
- `knowledge/sources/{id}` — Single source with full content (GET), update (PUT), toggle (PATCH), delete (DELETE)
- `knowledge/sources/{id}/embed` — Trigger embedding for source (POST)
- `agents/` — Agent configurations (GET, PATCH)
- `schedules/` — Scheduler settings (GET, PATCH)
- `themes/` — Visual themes (GET, GET /active, PATCH)
- `clips/{clipId}/analytics` — Per-clip analytics (GET)
- `settings/` — Global settings key-value store (GET, PUT)
- `settings/test-key` — Test OpenRouter API key connectivity (POST)
- `orchestrator/produce` — Trigger weekly production (POST)

## Environment Variables

See `.env.example`. Required: `DATABASE_URL`, `API_KEY`, `CLAUDE_API_KEY`, `KIE_API_KEY`, `ZERNIO_API_KEY`.
Optional with defaults: `PORT` (8080), `FFMPEG_PATH` (ffmpeg), `ELEVENLABS_VOICE` (Adam).

Note: OpenRouter API key is managed ONLY via the Settings page (database `settings` table), not via env vars. Kie, ElevenLabs, and Zernio keys can also be managed at runtime via Settings; env vars are used at startup, database settings override at runtime where applicable.

## Pipeline Flow

QuestionAgent → ScriptAgent → ImageAgent → Producer (Kie + FFmpeg) → Publisher (Zernio)
Weekly: Analyzer fetches YouTube analytics → Claude API analyzes → auto-tunes agent prompts

## Deployment
- **Auto-deploy:** Push to `master` → GitHub Actions runs `railway up` for both `adsvance-v2` and `adsvance-frontend`
- **Manual deploy:** `railway up --service adsvance-v2` / `railway up --service adsvance-frontend`
- **Region:** `asia-southeast1-eqsg3a` (Singapore)

## Gotchas
- Server and CLI modes are mutually exclusive — flags like `-produce` exit after completion
- Frontend has no lint script — only `tsc && vite build` for type checking
- CORS allows all origins (`*`) without credentials — tighten `AllowedOrigins` for production
- Scheduler reads cron config from `schedules` table — changing cron/enabled via API requires server restart to take effect
- ImageAgent must NOT generate logo/mascot/watermark — enforced in code + DB agent config
- Analyzer requires at least 3 published clips with analytics before running — otherwise skips silently
- Knowledge list endpoint returns summaries only (no content field) — use GET `/{id}` for full content
