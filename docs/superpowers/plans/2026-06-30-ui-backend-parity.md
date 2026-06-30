# UI ↔ Backend Parity Closure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Surface backend capabilities that have no UI (stop/tiktok actions, style presets, brand theme editor, kie credits, critic/learner observability) and make DB-only settings editable — additive only, no breaking changes.

**Architecture:** Phase 1 adds thin Go read-endpoints + 4 settings-allowlist keys. Phases 2-4 add React UI in `frontend/` that consumes them, following the existing `apiFetch<T>` client and page/handler/repo patterns. Backend ships first (additive, safe), then frontend.

**Tech Stack:** Go + chi + pgx (backend), React 19 + Vite + TypeScript + shadcn `ui/*` + lucide (frontend).

## Global Constraints

- All API routes live under `/api/v1`, behind the existing API-key middleware, returning the `models.APIResponse{data,error,message}` envelope. Handlers use `writeJSON(w, status, models.APIResponse{...})`. — backend pattern.
- Frontend calls go through `apiFetch<T>(path, options?)` in `frontend/src/api.ts`, which unwraps `json.data`. NO raw `fetch` outside api.ts. — frontend pattern.
- Feature flags `STYLE_PRESETS_ENABLED` / `STYLE_PRESETS_PERFORMANCE_ENABLED` stay env-managed; UI displays their state read-only, never toggles them. — spec decision.
- Additive only: do not change existing endpoints, response shapes, or routes. New nav entries go in `frontend/src/components/sidebar.tsx` (`PIPELINE_NAV`/`CONFIG_NAV`) + `frontend/src/lib/routes.ts` + `App.tsx`. — spec.
- Backend build gate: `go build ./...` + `go vet`. Frontend build gate: `cd frontend && npm run build` (`tsc && vite build`) must pass. — spec.
- `clips.content_format` column already exists (migration 017, `NOT NULL DEFAULT 'qa'`); no migration needed for B7. — verified.

---

## File Structure

**Backend (Go):**
- `internal/handler/settings.go` — add 4 allowlist keys (B1).
- `internal/models/clip.go` + `internal/repository/clips.go` — `content_format` in Clip (B7).
- `internal/handler/presets.go` (new) — `GET /presets`, `GET /presets/performance` (B2, B3).
- `internal/handler/status.go` (new) — `GET /status/kie-credits` (B4).
- `internal/handler/critiques.go` (new) — `GET /clips/{id}/critique` (B5) + `CritiquesRepo.GetByClip`.
- `internal/handler/skill_revisions.go` (new) — `GET /agents/skill-revisions` (B6) + `SkillRevisionsRepo.List`.
- `internal/router/router.go` + `cmd/server/main.go` — wire the new handlers.

**Frontend (TS/React):**
- `frontend/src/api.ts` — new client functions (all phases).
- `frontend/src/pages/Content.tsx` + components — F1/F2.
- `frontend/src/pages/Theme.tsx` (new) + routes/sidebar/App — F3.
- `frontend/src/pages/Analytics.tsx`, `Settings.tsx`, `PromptHistory.tsx`, `components/ReviewDialog.tsx` — F4-F7.

---

## Task 1 (backend): Settings allowlist + content_format on Clip

**Files:**
- Modify: `internal/handler/settings.go` (the `allowed` map ~line 55)
- Modify: `internal/models/clip.go` (Clip struct)
- Modify: `internal/repository/clips.go` (`clipColumns`, `scanClip`)

**Interfaces:**
- Produces: `Clip.ContentFormat string` (json `content_format`) in `GET /clips`; settings PUT accepts the 4 new keys.

- [ ] **Step 1: Add the 4 settings keys**

In `internal/handler/settings.go`, inside the `allowed := map[string]bool{...}` literal, add:

```go
		"audience_persona":          true,
		"zernio_tiktok_account_id":  true,
		"content_preview_confirmed": true,
		"express_consent_given":     true,
```

- [ ] **Step 2: Add `content_format` to the Clip model**

In `internal/models/clip.go`, add to the `Clip` struct (match sibling field style):

```go
	ContentFormat string `json:"content_format"`
```

- [ ] **Step 3: Add `content_format` to clipColumns + scanClip**

In `internal/repository/clips.go`: append `, content_format` to the `clipColumns` const, and add `&c.ContentFormat` to the `scanClip` `Scan(...)` arg list in the SAME position (i.e. last, matching the column order). Read both first to place it correctly (the existing tail is `... retry_count, style_preset`; add `content_format` after `style_preset` in BOTH the const and the scan).

- [ ] **Step 4: Build + vet**

Run: `go build ./... && go vet ./internal/handler/ ./internal/repository/ ./internal/models/`
Expected: clean.

> Controller verifies on a Neon branch that `GET /clips` returns `content_format` and PUT accepts the new keys (not an implementer step).

- [ ] **Step 5: Commit**

```bash
git add internal/handler/settings.go internal/models/clip.go internal/repository/clips.go
git commit -m "feat(api): settings allowlist for persona/tiktok + content_format on Clip"
```

---

## Task 2 (backend): Presets endpoints

**Files:**
- Create: `internal/handler/presets.go`
- Modify: `internal/router/router.go`, `cmd/server/main.go`

**Interfaces:**
- Consumes: `producer.Presets`, `producer.StylePresetsEnabled()`, `producer.StylePresetsPerformanceEnabled()`, `producer.DefaultWindowDays`, `(*repository.AnalyticsRepo).PresetRetention` (exists), `models.PresetScore`.
- Produces:
  - `GET /api/v1/presets` → `{"presets":[{"key","display_name","primary_color","accent_color"}],"style_presets_enabled":bool,"performance_enabled":bool}`
  - `GET /api/v1/presets/performance` → `[]models.PresetScore`

- [ ] **Step 1: Write the handler**

```go
// internal/handler/presets.go
package handler

import (
	"net/http"

	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/producer"
	"github.com/jaochai/video-fb/internal/repository"
)

type PresetsHandler struct {
	analytics *repository.AnalyticsRepo
}

func NewPresetsHandler(analytics *repository.AnalyticsRepo) *PresetsHandler {
	return &PresetsHandler{analytics: analytics}
}

type presetInfo struct {
	Key          string `json:"key"`
	DisplayName  string `json:"display_name"`
	PrimaryColor string `json:"primary_color"`
	AccentColor  string `json:"accent_color"`
}

func (h *PresetsHandler) List(w http.ResponseWriter, r *http.Request) {
	infos := make([]presetInfo, 0, len(producer.Presets))
	for _, p := range producer.Presets {
		infos = append(infos, presetInfo{
			Key: p.Key, DisplayName: p.DisplayName,
			PrimaryColor: p.Palette.Navy, AccentColor: p.Palette.Orange,
		})
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: map[string]any{
		"presets":               infos,
		"style_presets_enabled": producer.StylePresetsEnabled(),
		"performance_enabled":   producer.StylePresetsPerformanceEnabled(),
	}})
}

func (h *PresetsHandler) Performance(w http.ResponseWriter, r *http.Request) {
	scores, err := h.analytics.PresetRetention(r.Context(), producer.DefaultWindowDays)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: scores})
}
```

(Confirm the `models.APIResponse` field is named `Data` — grep `type APIResponse` in internal/models; if the field/tag differs, match it. Confirm `writeJSON` signature from an existing handler.)

- [ ] **Step 2: Wire routes + construct handler**

NOTE: `router.New(pool, apiKey, ragEngine, tracker, pub, scheduleReload)` builds its own repos from `pool` (e.g. `repository.NewAnalyticsRepo(pool)` at router.go:83). Full paths include the `/api/v1` prefix. In `internal/router/router.go`, near the analytics routes, add:

```go
	presets := handler.NewPresetsHandler(repository.NewAnalyticsRepo(pool))
	r.Route("/api/v1/presets", func(r chi.Router) {
		r.Get("/", presets.List)
		r.Get("/performance", presets.Performance)
	})
```

- [ ] **Step 3: Build + vet**

Run: `go build ./... && go vet ./internal/handler/ ./internal/router/`
Expected: clean.

- [ ] **Step 4: Commit**

```bash
git add internal/handler/presets.go internal/router/router.go cmd/server/main.go
git commit -m "feat(api): GET /presets + /presets/performance endpoints"
```

---

## Task 3 (backend): kie-credits, critique, skill-revisions endpoints

**Files:**
- Create: `internal/handler/status.go`, `internal/handler/critiques.go`, `internal/handler/skill_revisions.go`
- Modify: `internal/repository/critiques.go` (add `GetByClip`), `internal/repository/skill_revisions.go` (add `List`)
- Modify: `internal/router/router.go`, `cmd/server/main.go`

**Interfaces:**
- Consumes: `(*producer.Producer).KieCredits(ctx) (int, error)` (exists).
- Produces:
  - `GET /api/v1/status/kie-credits` → `{"credits":int}` (credits -1 + error string on failure, HTTP 200)
  - `GET /api/v1/clips/{id}/critique` → `{score,changes,applied,created_at}` or `null`
  - `GET /api/v1/agents/skill-revisions` → `[]{agent_name,rationale,critique_window,created_at}`
  - `CritiquesRepo.GetByClip(ctx, clipID string) (*models.ClipCritique, error)` (nil,nil when none)
  - `SkillRevisionsRepo.List(ctx, limit int) ([]models.SkillRevision, error)`

- [ ] **Step 1: kie-credits handler**

```go
// internal/handler/status.go
package handler

import (
	"net/http"

	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/producer"
)

type StatusHandler struct{ prod *producer.Producer }

func NewStatusHandler(p *producer.Producer) *StatusHandler { return &StatusHandler{prod: p} }

func (h *StatusHandler) KieCredits(w http.ResponseWriter, r *http.Request) {
	credits, err := h.prod.KieCredits(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, models.APIResponse{Data: map[string]any{"credits": -1, "error": err.Error()}})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: map[string]any{"credits": credits}})
}
```

- [ ] **Step 2: CritiquesRepo.GetByClip + handler**

Add to `internal/repository/critiques.go` (read the file's existing scan style + the `clip_critiques` columns first — `id, clip_id, score, changes, applied, created_at`; `score`/`changes` are jsonb stored as `[]byte`):

```go
func (r *CritiquesRepo) GetByClip(ctx context.Context, clipID string) (*models.ClipCritique, error) {
	var c models.ClipCritique
	err := r.pool.QueryRow(ctx,
		`SELECT clip_id, score, changes, applied, created_at
		 FROM clip_critiques WHERE clip_id = $1 ORDER BY created_at DESC LIMIT 1`, clipID).
		Scan(&c.ClipID, &c.Score, &c.Changes, &c.Applied, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get critique by clip: %w", err)
	}
	return &c, nil
}
```

Add `models.ClipCritique` to `internal/models/` (new file or beside clip):

```go
type ClipCritique struct {
	ClipID    string          `json:"clip_id"`
	Score     json.RawMessage `json:"score"`
	Changes   json.RawMessage `json:"changes"`
	Applied   bool            `json:"applied"`
	CreatedAt time.Time       `json:"created_at"`
}
```

Handler `internal/handler/critiques.go`:

```go
package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
)

type CritiquesHandler struct{ repo *repository.CritiquesRepo }

func NewCritiquesHandler(repo *repository.CritiquesRepo) *CritiquesHandler {
	return &CritiquesHandler{repo: repo}
}

func (h *CritiquesHandler) GetByClip(w http.ResponseWriter, r *http.Request) {
	clipID := chi.URLParam(r, "clipId")
	c, err := h.repo.GetByClip(r.Context(), clipID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: c}) // c==nil → data:null
}
```

(Match the exact URL param name used by the existing visual-qa route — it registers `/clips/{clipId}/visual-qa`, so use `{clipId}` and `chi.URLParam(r,"clipId")` consistently.)

- [ ] **Step 3: SkillRevisionsRepo.List + handler**

Add to `internal/repository/skill_revisions.go` (columns: `id, agent_name, old_skills, new_skills, rationale, critique_window, created_at` — list returns the lean subset):

```go
func (r *SkillRevisionsRepo) List(ctx context.Context, limit int) ([]models.SkillRevision, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT agent_name, rationale, critique_window, created_at
		 FROM skill_revisions ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list skill revisions: %w", err)
	}
	defer rows.Close()
	var out []models.SkillRevision
	for rows.Next() {
		var s models.SkillRevision
		if err := rows.Scan(&s.AgentName, &s.Rationale, &s.CritiqueWindow, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan skill revision: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
```

`models.SkillRevision`:

```go
type SkillRevision struct {
	AgentName      string    `json:"agent_name"`
	Rationale      string    `json:"rationale"`
	CritiqueWindow int       `json:"critique_window"`
	CreatedAt      time.Time `json:"created_at"`
}
```

Handler `internal/handler/skill_revisions.go` (mirror CritiquesHandler shape; `List` reads `?limit=` default 50; method GET, no URL param).

- [ ] **Step 4: Wire routes (incl. adding `prod` to router.New)**

`router.New` does NOT currently receive the producer; kie-credits needs it. Add a `prod *producer.Producer` parameter to `func New(...)` in `internal/router/router.go` (import `github.com/jaochai/video-fb/internal/producer`), and pass `prod` from the single call site in `cmd/server/main.go` (the `router.New(pool, cfg.APIKey, ragEngine, tracker, pub, ...)` call at ~line 165 — `prod` is already in scope there, built at ~line 101). Then in router.New build the handlers from `pool`/`prod` and register full `/api/v1` paths:

```go
	critiques := handler.NewCritiquesHandler(repository.NewCritiquesRepo(pool))
	r.Get("/api/v1/clips/{clipId}/critique", critiques.GetByClip)

	skillRevs := handler.NewSkillRevisionsHandler(repository.NewSkillRevisionsRepo(pool))
	r.Get("/api/v1/agents/skill-revisions", skillRevs.List)

	status := handler.NewStatusHandler(prod)
	r.Get("/api/v1/status/kie-credits", status.KieCredits)
```

- [ ] **Step 5: Build + vet**

Run: `go build ./... && go vet ./internal/...`
Expected: clean.

> Controller verifies `GetByClip` + `List` SQL on a seeded Neon branch.

- [ ] **Step 6: Commit**

```bash
git add internal/handler/ internal/repository/critiques.go internal/repository/skill_revisions.go internal/models/ internal/router/router.go cmd/server/main.go
git commit -m "feat(api): kie-credits, clip critique, and skill-revisions read endpoints"
```

---

## Task 4 (frontend): Content page — Stop, Publish TikTok, preset badge, format, producing tab

**Files:**
- Modify: `frontend/src/api.ts`, `frontend/src/pages/Content.tsx` (+ its row/table + Review* untouched)

**Interfaces:**
- Consumes: `POST /orchestrator/stop`, `POST /orchestrator/publish-tiktok` (exist); `Clip.style_preset`, `Clip.content_format` (Task 1).

- [ ] **Step 1: Add api.ts functions + Clip fields**

In `frontend/src/api.ts` add (match the existing `apiFetch` call style used by `produce`/`publish` — find them; if those live in Content.tsx inline, add these inline there the same way). The two calls:

```ts
export const stopProduction = () => apiFetch('/api/v1/orchestrator/stop', { method: 'POST' });
export const publishTikTok = () => apiFetch('/api/v1/orchestrator/publish-tiktok', { method: 'POST' });
```

In the `Clip` type/interface used by Content.tsx, add `style_preset: string` and `content_format: string`.

- [ ] **Step 2: Wire the buttons**

In `Content.tsx`, beside the existing Produce/Publish buttons:
- Render a **Stop** button (destructive variant, lucide `Square`/`CircleStop`) that calls `stopProduction()` then refetches status — shown only when `prodStatus.active`.
- Render a **Publish TikTok** button beside Publish-YouTube, calling `publishTikTok()`, shown when `statusCounts.ready > 0` (same condition as Publish). Use the existing toast/mutation helper (`useMutationWithToast`) for both, matching how `handlePublish` is built.

- [ ] **Step 3: Table badge + format + producing tab**

- Add a `style_preset` badge cell to the clips table (reuse `status-badge`/a small `Badge` from `ui/`; show the preset key; hide if empty).
- Show `content_format` (small muted text near category).
- Add a `producing` tab to the status filter tabs row (the tabs already compute `statusCounts.producing`).

- [ ] **Step 4: Build**

Run: `cd frontend && npm run build`
Expected: `tsc` + `vite build` succeed, no type errors.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/api.ts frontend/src/pages/Content.tsx
git commit -m "feat(ui): Stop + Publish-TikTok buttons, preset badge, format, producing tab"
```

---

## Task 5 (frontend): Theme page + Style Presets panel

**Files:**
- Create: `frontend/src/pages/Theme.tsx`
- Modify: `frontend/src/api.ts`, `frontend/src/lib/routes.ts`, `frontend/src/components/sidebar.tsx`, `frontend/src/App.tsx`

**Interfaces:**
- Consumes: `GET /themes/active`, `PATCH /themes/{id}`, `GET /presets` (Task 2).

- [ ] **Step 1: api.ts functions**

```ts
export const getActiveTheme = () => apiFetch<BrandTheme>('/api/v1/themes/active');
export const updateTheme = (id: string, body: Partial<BrandTheme>) =>
  apiFetch(`/api/v1/themes/${id}`, { method: 'PATCH', body: JSON.stringify(body) });
export const getPresets = () => apiFetch<PresetsResponse>('/api/v1/presets');
```

Types: `BrandTheme = {id,name,primary_color,secondary_color,accent_color,font_name,logo_url,mascot_description,image_style,active}`; `PresetsResponse = {presets:{key,display_name,primary_color,accent_color}[], style_presets_enabled:boolean, performance_enabled:boolean}`.

- [ ] **Step 2: Theme page**

Create `Theme.tsx` modeled on `Settings.tsx` (read it as the template — card layout, dirty-tracking, save button, `useMutationWithToast`): load active theme, render inputs for `primary_color`/`secondary_color`/`accent_color` (each `<input type="color">` + hex text), `font_name`, `mascot_description` (textarea), `image_style` (textarea), `logo_url`. Save → `updateTheme(theme.id, dirtyFields)`. Below: a read-only **Style Presets** panel from `getPresets()` — each preset a chip showing two swatches (primary+accent) + `display_name`, and two badges for `style_presets_enabled` / `performance_enabled` (green "ON" / muted "OFF").

- [ ] **Step 3: Register route + nav**

- `routes.ts`: add `THEME: '/theme'`.
- `sidebar.tsx`: add `{ to: ROUTES.THEME, label: "Theme", icon: Palette }` to `CONFIG_NAV` (import `Palette` from lucide-react).
- `App.tsx`: add the `<Route path={ROUTES.THEME} element={<Theme/>} />` and import.

- [ ] **Step 4: Build**

Run: `cd frontend && npm run build`
Expected: success.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/pages/Theme.tsx frontend/src/api.ts frontend/src/lib/routes.ts frontend/src/components/sidebar.tsx frontend/src/App.tsx
git commit -m "feat(ui): Theme editor page + read-only style presets panel"
```

---

## Task 6 (frontend): Observability — preset perf card, kie badge + settings fields, critique, skill-revisions

**Files:**
- Modify: `frontend/src/api.ts`, `frontend/src/pages/Analytics.tsx`, `frontend/src/pages/Settings.tsx`, `frontend/src/pages/PromptHistory.tsx`, `frontend/src/components/ReviewDialog.tsx`

**Interfaces:**
- Consumes: `GET /presets/performance` (Task 2), `GET /status/kie-credits`, `GET /clips/{id}/critique`, `GET /agents/skill-revisions` (Task 3); the 4 new settings keys (Task 1).

- [ ] **Step 1: api.ts functions**

```ts
export const getPresetPerformance = () => apiFetch<PresetScore[]>('/api/v1/presets/performance');
export const getKieCredits = () => apiFetch<{credits:number; error?:string}>('/api/v1/status/kie-credits');
export const getClipCritique = (id: string) => apiFetch<ClipCritique|null>(`/api/v1/clips/${id}/critique`);
export const getSkillRevisions = () => apiFetch<SkillRevision[]>('/api/v1/agents/skill-revisions');
```

Types: `PresetScore = {preset,avg_retention,n}`; `ClipCritique = {clip_id,score,changes,applied,created_at}`; `SkillRevision = {agent_name,rationale,critique_window,created_at}`.

- [ ] **Step 2: Analytics — "By Style Preset" card**

In `Analytics.tsx`, add a card: fetch `getPresetPerformance()`, render a table preset / retention% (`(avg_retention*100).toFixed(1)`) / N, sorted by retention desc. Empty-state text when array empty: "ยังไม่มีข้อมูลพอ — ระบบกำลังสะสม retention ต่อธีม".

- [ ] **Step 3: Settings — kie credit badge + new editable fields**

- Add a kie.ai credits badge: fetch `getKieCredits()`; show number; color green if `>0`, red if `<=0`, muted "ดูไม่ได้" if `error` present or `credits===-1`.
- Add editable fields wired into the existing settings dirty-save (`PUT /settings`): `audience_persona` (textarea), `zernio_tiktok_account_id` (text), `content_preview_confirmed` + `express_consent_given` (these are stored as string "true"/"false" — render as toggles that write the string). These keys are now in the backend allowlist (Task 1).

- [ ] **Step 4: ReviewDialog — show critic critique**

In `ReviewDialog.tsx`, alongside the visual-QA section, fetch `getClipCritique(clip.id)`; when non-null, show the critic score (parse `score` JSON — show the overall/numeric fields present) + an "applied" badge. Keep it best-effort: on null/error render nothing.

- [ ] **Step 5: PromptHistory — skill-revisions section**

In `PromptHistory.tsx`, add a second section/tab "Skill Revisions" listing `getSkillRevisions()`: agent_name, rationale, relative date. Read-only.

- [ ] **Step 6: Build**

Run: `cd frontend && npm run build`
Expected: success.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/api.ts frontend/src/pages/Analytics.tsx frontend/src/pages/Settings.tsx frontend/src/pages/PromptHistory.tsx frontend/src/components/ReviewDialog.tsx
git commit -m "feat(ui): preset-performance card, kie badge, persona/tiktok settings, critique + skill-revisions views"
```

---

## Self-Review checklist (controller runs after writing)

- Spec coverage: B1+B7 (T1), B2+B3 (T2), B4+B5+B6 (T3), F1+F2 (T4), F3 (T5), F4-F7 (T6). All spec rows mapped. ✓
- Type consistency: `models.PresetScore{Preset,AvgRetention,N}` (json preset/avg_retention/n) used by T2 + T6; `Clip.content_format`/`style_preset` set in T1 and read in T4; `ClipCritique`/`SkillRevision` defined T3, consumed T6. ✓
- Placeholders: backend code is complete; frontend steps reference exact existing files as templates (Settings.tsx, handlePublish) rather than vague "implement" — acceptable for UI work where the pattern file is named. Implementers must read the named template file before writing.

## Verification notes for the controller

- After T1-T3 (backend): verify on a Neon branch — `content_format` returned by clips, the 4 settings keys accepted, `GetByClip`/`List`/`PresetRetention` SQL return sane rows. Confirm `models.APIResponse` data field name and `writeJSON` signature match what the new handlers use (grep before T2).
- Deploy backend (master push) BEFORE the frontend depends on the new endpoints live; the frontend service is separate.

## Rollout

1. Land + deploy backend (Tasks 1-3) → additive, safe.
2. Land frontend (Tasks 4-6); build must pass; deploy the frontend service.
3. Smoke-test each new control against the live API.
