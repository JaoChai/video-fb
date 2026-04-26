# video-fb

Automated video content pipeline for Ads Vance — Go backend + React dashboard.

## Stack
- **Backend:** Go 1.25, chi router, pgx/v5 (Neon PostgreSQL)
- **Frontend:** React 19, Vite 8, TanStack Query, React Router
- **External:** Claude API (scripts), Kie AI (video), ElevenLabs (voice), Zernio (publishing)

## Commands

```bash
# Backend
make run              # Start server on :8080
make build            # Build binary to bin/server
make test             # Run all Go tests
make migrate          # Run DB migrations

# CLI modes (alternative to server)
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
  scheduler/                # Background jobs (daily publish, weekly produce/crawl)
frontend/src/
  App.tsx                   # Main layout with sidebar navigation
  pages/                    # Content, Schedules, Agents, Knowledge, Analytics
  api.ts                    # API client for backend endpoints
migrations/                 # SQL migration files (001_initial_schema.sql)
```

## API Endpoints

All routes require `Authorization` header with API_KEY.
Base: `/api/v1/`

- `clips/` — CRUD for video clips
- `clips/{clipId}/scenes/` — Scenes per clip
- `knowledge/sources/` — RAG knowledge sources
- `agents/` — Agent configurations
- `schedules/` — Scheduler settings
- `themes/` — Visual themes (list, active, update)
- `clips/{clipId}/analytics` — Per-clip analytics
- `orchestrator/produce` — Trigger weekly production (POST)

## Environment Variables

See `.env.example`. Required: `DATABASE_URL`, `CLAUDE_API_KEY`.
Optional with defaults: `PORT` (8080), `FFMPEG_PATH` (ffmpeg), `ELEVENLABS_VOICE` (Adam).

## Pipeline Flow

QuestionAgent → ScriptAgent → ImageAgent → Producer (Kie + FFmpeg) → Publisher (Zernio)

## Gotchas
- Server and CLI modes are mutually exclusive — flags like `-produce` exit after completion
- Frontend has no lint script — only `tsc && vite build` for type checking
- CORS is set to allow all origins (`*`) — tighten for production
- Scheduler runs in goroutines — no graceful shutdown signal handling
