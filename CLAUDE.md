# video-fb

Automated video content pipeline for Ads Vance — Go backend + React dashboard.

## Stack
- **Backend:** Go 1.25, chi router, pgx/v5 (Neon PostgreSQL)
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
  rag/                      # RAG engine for knowledge retrieval
  crawler/                  # Knowledge source crawler
  orchestrator/             # Pipeline: question → script → image → produce
  producer/                 # Video production (Kie AI + FFmpeg assembly)
  publisher/                # Zernio publishing + analytics
  scheduler/                # Background jobs (daily publish, weekly produce/crawl/analytics)
frontend/src/
  App.tsx                   # Main layout with sidebar navigation
  pages/                    # Content, Schedules, Agents, Knowledge, Analytics, Settings
  api.ts                    # API client for backend endpoints
migrations/                 # SQL migration files (001_initial_schema.sql)
```

## API Endpoints

All routes require `Authorization` header with API_KEY (except `/health`).
Base: `/api/v1/`

- `GET /health` — Health check (no auth required)
- `clips/` — CRUD for video clips
- `clips/{clipId}/scenes/` — Scenes per clip (GET, POST)
- `scenes/{id}` — Delete scene (DELETE, standalone route)
- `knowledge/sources/` — RAG knowledge sources (GET, POST, PUT, PATCH, DELETE)
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

Note: API keys for OpenRouter, Kie, ElevenLabs, and Zernio can also be managed at runtime via the Settings page (stored in database `settings` table). Env vars are used at startup; database settings override at runtime where applicable.

## Pipeline Flow

QuestionAgent → ScriptAgent → ImageAgent → Producer (Kie + FFmpeg) → Publisher (Zernio)

## Gotchas
- Server and CLI modes are mutually exclusive — flags like `-produce` exit after completion
- Frontend has no lint script — only `tsc && vite build` for type checking
- CORS allows all origins (`*`) without credentials — tighten `AllowedOrigins` for production
- Scheduler runs in goroutines — no graceful shutdown signal handling
