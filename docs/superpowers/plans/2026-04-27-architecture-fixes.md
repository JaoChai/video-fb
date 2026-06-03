# Architecture Verification Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix all discrepancies found between CLAUDE.md documentation and actual code, fix CORS misconfiguration, complete Analytics frontend, and remove dead code.

**Architecture:** 10 issues split into 7 tasks — documentation fixes grouped together, code changes isolated per component. Tasks 1-3 are documentation-only (zero risk), Tasks 4-6 are code changes, Task 7 is cleanup.

**Tech Stack:** Go 1.25 (chi router), React 19 (TanStack Query), PostgreSQL

---

## File Structure

| Action | File | Responsibility |
|--------|------|---------------|
| Modify | `CLAUDE.md` | Fix all documentation gaps (Tasks 1-3) |
| Modify | `.env.example` | Add missing ZERNIO_API_KEY (Task 3) |
| Modify | `internal/router/router.go:18-24` | Fix CORS config (Task 4) |
| Modify | `frontend/src/pages/Analytics.tsx` | Add real analytics data (Task 5) |
| Modify | `frontend/src/api.ts` | Add analytics API type (Task 5) |
| Delete | `internal/repository/topics.go` | Remove dead code (Task 6) |
| Modify | `internal/agent/question.go` | Verify no import of TopicsRepo (Task 6) |

---

### Task 1: Fix CLAUDE.md — API Endpoints & Routes

**Files:**
- Modify: `CLAUDE.md:56-68`

- [ ] **Step 1: Update API Endpoints section**

Replace lines 56-68 in `CLAUDE.md` with:

```markdown
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
```

- [ ] **Step 2: Verify the update matches router.go**

Run: `grep -n "r\.\(Get\|Post\|Put\|Patch\|Delete\|Route\)" internal/router/router.go`

Expected: Every route in router.go should now be documented in CLAUDE.md.

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add missing API endpoints to CLAUDE.md (settings, health, embed, scenes delete)"
```

---

### Task 2: Fix CLAUDE.md — Stack, CLI, Scheduler, Frontend Pages

**Files:**
- Modify: `CLAUDE.md:8` (Stack line)
- Modify: `CLAUDE.md:19-23` (CLI modes)
- Modify: `CLAUDE.md:48` (Scheduler description)
- Modify: `CLAUDE.md:51` (Frontend pages list)

- [ ] **Step 1: Fix External stack description**

Replace line 8:
```markdown
- **External:** Claude API (scripts), Kie AI (video + voice via ElevenLabs), OpenRouter (embeddings), Jina AI (web scraping), Zernio (publishing)
```

This corrects: ElevenLabs is accessed through Kie AI (model `elevenlabs/text-to-dialogue-v3` in `internal/producer/kieai.go:73`), not as a direct integration. Also adds OpenRouter and Jina AI which were missing.

- [ ] **Step 2: Add `-migrate` CLI flag**

Replace lines 19-23 with:
```bash
# CLI modes (alternative to server)
go run cmd/server/main.go -migrate         # Run database migrations
go run cmd/server/main.go -crawl           # Crawl knowledge sources
go run cmd/server/main.go -produce 7       # Produce N clips
go run cmd/server/main.go -publish         # Publish ready clips
go run cmd/server/main.go -analytics       # Fetch analytics
```

- [ ] **Step 3: Fix scheduler description**

Replace line 48:
```
  scheduler/                # Background jobs (daily publish, weekly produce/crawl/analytics)
```

- [ ] **Step 4: Fix frontend pages list**

Replace line 51:
```
  pages/                    # Content, Schedules, Agents, Knowledge, Analytics, Settings
```

- [ ] **Step 5: Verify all changes are accurate**

Run: `grep -c "migrate\|crawl\|produce\|publish\|analytics" cmd/server/main.go`

Expected: 5 (one flag per CLI mode)

Run: `ls frontend/src/pages/`

Expected: Analytics.tsx, Agents.tsx, Content.tsx, Knowledge.tsx, Schedules.tsx, Settings.tsx (6 pages)

- [ ] **Step 6: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: fix CLAUDE.md stack description, add -migrate flag, scheduler analytics job, Settings page"
```

---

### Task 3: Fix Environment Variables Documentation

**Files:**
- Modify: `CLAUDE.md:70-73`
- Modify: `.env.example`

- [ ] **Step 1: Update CLAUDE.md Environment Variables section**

Replace lines 70-73 with:
```markdown
## Environment Variables

See `.env.example`. Required: `DATABASE_URL`, `API_KEY`, `CLAUDE_API_KEY`, `KIE_API_KEY`, `ZERNIO_API_KEY`.
Optional with defaults: `PORT` (8080), `FFMPEG_PATH` (ffmpeg), `ELEVENLABS_VOICE` (Adam).

Note: API keys for OpenRouter, Kie, ElevenLabs, and Zernio can also be managed at runtime via the Settings page (stored in database `settings` table). Env vars are used at startup; database settings override at runtime where applicable.
```

- [ ] **Step 2: Add ZERNIO_API_KEY to .env.example**

Add to end of `.env.example`:
```
ZERNIO_API_KEY=zernio-xxx
```

- [ ] **Step 3: Verify config.go loads all keys**

Run: `grep "os.Getenv" internal/config/config.go`

Expected: Should see DATABASE_URL, PORT, API_KEY, CLAUDE_API_KEY, KIE_API_KEY, ELEVENLABS_VOICE, FFMPEG_PATH, ZERNIO_API_KEY (8 vars)

- [ ] **Step 4: Commit**

```bash
git add CLAUDE.md .env.example
git commit -m "docs: add ZERNIO_API_KEY to .env.example, document runtime settings dual-config"
```

---

### Task 4: Fix CORS Misconfiguration

**Files:**
- Modify: `internal/router/router.go:18-24`

- [ ] **Step 1: Understand the issue**

Current code (`router.go:18-24`):
```go
r.Use(cors.Handler(cors.Options{
    AllowedOrigins:   []string{"*"},
    AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
    AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
    AllowCredentials: true,
    MaxAge:           300,
}))
```

Problem: `AllowedOrigins: *` + `AllowCredentials: true` violates CORS spec. Browsers will reject credentialed requests. Since this app uses `Authorization` header (Bearer token), not cookies, we don't need `AllowCredentials`.

- [ ] **Step 2: Fix CORS — remove AllowCredentials**

Replace the CORS block:
```go
r.Use(cors.Handler(cors.Options{
    AllowedOrigins:   []string{"*"},
    AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
    AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
    MaxAge:           300,
}))
```

- [ ] **Step 3: Verify build passes**

Run: `cd /Users/jaochai/Code/video-fb && go build ./...`

Expected: No errors.

- [ ] **Step 4: Update CLAUDE.md gotcha**

Replace the CORS gotcha line:
```markdown
- CORS allows all origins (`*`) without credentials — tighten `AllowedOrigins` for production
```

- [ ] **Step 5: Commit**

```bash
git add internal/router/router.go CLAUDE.md
git commit -m "fix: remove AllowCredentials from CORS config — incompatible with wildcard origin"
```

---

### Task 5: Complete Analytics Frontend Page

**Files:**
- Modify: `frontend/src/pages/Analytics.tsx` (complete rewrite)

- [ ] **Step 1: Check backend response shape**

Run: `grep -A 20 "func.*ListByClip" internal/repository/analytics.go`

This confirms the data shape returned by `GET /api/v1/clips/{clipId}/analytics`.

- [ ] **Step 2: Rewrite Analytics.tsx with real analytics data**

Replace entire `frontend/src/pages/Analytics.tsx` with:

```tsx
import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { apiFetch } from '../api';

interface Clip { id: string; title: string; status: string; category: string; }
interface ClipAnalytics {
  id: string; clip_id: string; platform: string;
  views: number; likes: number; comments: number; shares: number;
  watch_time_seconds: number; retention_rate: number; fetched_at: string;
}

export default function AnalyticsPage() {
  const [selectedClipId, setSelectedClipId] = useState<string | null>(null);

  const { data: clips, isLoading } = useQuery({
    queryKey: ['clips'],
    queryFn: () => apiFetch<Clip[]>('/api/v1/clips'),
  });

  const { data: analytics, isLoading: analyticsLoading } = useQuery({
    queryKey: ['clip-analytics', selectedClipId],
    queryFn: () => apiFetch<ClipAnalytics[]>(`/api/v1/clips/${selectedClipId}/analytics`),
    enabled: !!selectedClipId,
  });

  const published = clips?.filter(c => c.status === 'published') || [];
  const stats = [
    { label: 'Total', value: clips?.length || 0 },
    { label: 'Published', value: published.length },
    { label: 'Ready', value: clips?.filter(c => c.status === 'ready').length || 0 },
    { label: 'Draft', value: clips?.filter(c => c.status === 'draft').length || 0 },
  ];

  const platforms = ['youtube', 'tiktok', 'instagram', 'facebook'];

  return (
    <div>
      <h1 style={{ fontSize: 20, fontWeight: 600, marginBottom: 32 }}>Analytics</h1>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 12, marginBottom: 40 }}>
        {stats.map(({ label, value }) => (
          <div key={label} style={{
            background: '#111', borderRadius: 8, padding: '20px 24px',
            border: '1px solid #1a1a1a',
          }}>
            <div style={{ fontSize: 12, color: '#555', marginBottom: 8, textTransform: 'uppercase', letterSpacing: '0.05em' }}>{label}</div>
            <div style={{ fontSize: 28, fontWeight: 700, fontVariantNumeric: 'tabular-nums' }}>{value}</div>
          </div>
        ))}
      </div>

      <h2 style={{ fontSize: 15, fontWeight: 600, marginBottom: 16 }}>Published Clips</h2>
      {isLoading ? <p style={{ color: '#555' }}>Loading...</p> : published.length === 0 ? (
        <p style={{ color: '#555', fontSize: 14 }}>No published clips yet.</p>
      ) : (
        <div style={{ display: 'grid', gap: 8, marginBottom: 32 }}>
          {published.map(clip => (
            <div key={clip.id} onClick={() => setSelectedClipId(clip.id === selectedClipId ? null : clip.id)} style={{
              background: clip.id === selectedClipId ? '#1a1a1a' : '#111',
              borderRadius: 6, padding: '12px 16px',
              border: clip.id === selectedClipId ? '1px solid #333' : '1px solid #1a1a1a',
              display: 'flex', justifyContent: 'space-between', alignItems: 'center',
              cursor: 'pointer', transition: 'background 0.15s',
            }}>
              <span style={{ fontSize: 14 }}>{clip.title}</span>
              <span style={{ fontSize: 12, color: '#555' }}>{clip.category}</span>
            </div>
          ))}
        </div>
      )}

      {selectedClipId && (
        <div>
          <h2 style={{ fontSize: 15, fontWeight: 600, marginBottom: 16 }}>
            Platform Analytics
          </h2>
          {analyticsLoading ? <p style={{ color: '#555' }}>Loading analytics...</p> :
           !analytics || analytics.length === 0 ? (
            <p style={{ color: '#555', fontSize: 14 }}>No analytics data yet for this clip.</p>
          ) : (
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: 12 }}>
              {platforms.map(platform => {
                const data = analytics.find(a => a.platform === platform);
                if (!data) return null;
                return (
                  <div key={platform} style={{
                    background: '#111', borderRadius: 8, padding: 20,
                    border: '1px solid #1a1a1a',
                  }}>
                    <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 16, textTransform: 'capitalize' }}>
                      {platform}
                    </div>
                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
                      {[
                        { label: 'Views', value: data.views.toLocaleString() },
                        { label: 'Likes', value: data.likes.toLocaleString() },
                        { label: 'Comments', value: data.comments.toLocaleString() },
                        { label: 'Shares', value: data.shares.toLocaleString() },
                        { label: 'Watch Time', value: `${Math.round(data.watch_time_seconds / 60)}m` },
                        { label: 'Retention', value: `${(data.retention_rate * 100).toFixed(1)}%` },
                      ].map(({ label, value }) => (
                        <div key={label}>
                          <div style={{ fontSize: 11, color: '#555', marginBottom: 4 }}>{label}</div>
                          <div style={{ fontSize: 16, fontWeight: 600, fontVariantNumeric: 'tabular-nums' }}>{value}</div>
                        </div>
                      ))}
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 3: Verify frontend builds**

Run: `cd /Users/jaochai/Code/video-fb/frontend && npm run build`

Expected: No TypeScript or build errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/pages/Analytics.tsx
git commit -m "feat: complete Analytics page with per-clip platform metrics (views, likes, retention)"
```

---

### Task 6: Remove Dead Code — TopicsRepo

**Files:**
- Delete: `internal/repository/topics.go`
- Verify: `internal/agent/question.go` (uses direct SQL, not TopicsRepo)
- Verify: `cmd/server/main.go` (does not import TopicsRepo)

- [ ] **Step 1: Verify TopicsRepo is not imported anywhere**

Run: `grep -r "TopicsRepo\|NewTopicsRepo" /Users/jaochai/Code/video-fb --include="*.go" | grep -v "topics.go"`

Expected: No matches. This confirms TopicsRepo is dead code.

- [ ] **Step 2: Verify QuestionAgent uses direct SQL**

Confirm `internal/agent/question.go:41-53` queries `topic_history` directly via `a.pool.Query()`, not through TopicsRepo. (Already verified — line 41-43 shows `a.pool.Query(ctx, "SELECT title FROM topic_history...")`).

- [ ] **Step 3: Delete topics.go**

Run: `rm internal/repository/topics.go`

- [ ] **Step 4: Verify build still passes**

Run: `cd /Users/jaochai/Code/video-fb && go build ./...`

Expected: No errors. TopicsRepo was never used so removing it won't break anything.

- [ ] **Step 5: Commit**

```bash
git add internal/repository/topics.go
git commit -m "chore: remove unused TopicsRepo — QuestionAgent queries topic_history directly"
```

---

### Task 7: Final Verification

- [ ] **Step 1: Full backend build**

Run: `cd /Users/jaochai/Code/video-fb && go build ./...`

Expected: Clean build, no errors.

- [ ] **Step 2: Full frontend build**

Run: `cd /Users/jaochai/Code/video-fb/frontend && npm run build`

Expected: Clean build, no TypeScript errors.

- [ ] **Step 3: Verify CLAUDE.md completeness**

Run: `cat CLAUDE.md`

Verify checklist:
- [ ] Settings endpoint documented
- [ ] ElevenLabs described as "via Kie AI" not direct
- [ ] `-migrate` flag listed
- [ ] `weekly-analytics` in scheduler description
- [ ] Settings page in frontend pages list
- [ ] All routes documented
- [ ] ZERNIO_API_KEY in env vars
- [ ] Dual-config pattern noted
- [ ] CORS gotcha updated

- [ ] **Step 4: Verify .env.example has ZERNIO_API_KEY**

Run: `grep ZERNIO .env.example`

Expected: `ZERNIO_API_KEY=zernio-xxx`

---

## Summary

| Task | Type | Risk | Issues Fixed |
|------|------|------|-------------|
| 1 | Docs | None | #1 Settings endpoints, #10 undocumented routes |
| 2 | Docs | None | #2 ElevenLabs, #6 -migrate, #7 weekly-analytics, #8 Settings page |
| 3 | Docs | None | #5 ZERNIO_API_KEY + dual-config |
| 4 | Code (Go) | Low | #4 CORS violation |
| 5 | Code (React) | Low | #3 Analytics hollow page |
| 6 | Code (Go) | None | #9 dead code TopicsRepo |
| 7 | Verify | None | Final check all fixes |

**Estimated time:** ~20-30 minutes total
