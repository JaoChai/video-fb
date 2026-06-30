# UI ↔ Backend Parity Closure — Design

**Date:** 2026-06-30
**Status:** Approved scope (user: "ทำทั้งหมด จัดการได้เลย"), pre-implementation
**Source:** the agent-team parity audit (this session). Closes the gaps where the backend
has a capability with no UI, plus a few small new read-endpoints and settings.

## Goal

Surface every meaningful backend capability in the React UI, and make settings that are
read-only-in-DB editable. No contract bugs were found (UI↔backend endpoints all match), so
this is additive: new endpoints + new UI, no breaking changes.

## Design decisions (made up-front; defaults chosen for "จัดการได้เลย")

- **Feature flags stay env-managed.** `STYLE_PRESETS_ENABLED` / `STYLE_PRESETS_PERFORMANCE_ENABLED`
  remain Railway env vars (toggling deploy-coupled flags from the UI is risky). The UI only
  *displays* their current state read-only.
- **No new "page" where a card on an existing page suffices.** Preset performance → a card on
  Analytics; skill-revisions → a tab on Prompt History; kie credits → a badge on Settings.
  Theme editing is substantial enough to get its own page.
- **Reuse the existing API client** (`apiFetch<T>` unwraps `json.data`) and the
  `APIResponse{data,error,message}` envelope, the chi router groups, and shadcn `ui/*` +
  lucide icons. Match existing page/handler/repo patterns exactly.

## Phases

### Phase 1 — Backend endpoints + settings (enables the UI work)

All under `/api/v1`, behind the existing API-key middleware, returning the standard envelope.

| ID | Endpoint | Handler/Repo work |
|----|----------|-------------------|
| B1 | `PUT /settings` allowlist += `audience_persona`, `zernio_tiktok_account_id`, `content_preview_confirmed`, `express_consent_given` | add 4 keys to the `allowed` map in `handler/settings.go:55` |
| B2 | `GET /presets` → `{presets:[{key,display_name,primary_color,accent_color}], style_presets_enabled, performance_enabled}` | new `handler/presets.go`; reads `producer.Presets` + `producer.StylePresetsEnabled()` / `StylePresetsPerformanceEnabled()` |
| B3 | `GET /presets/performance` → `[]PresetScore` | calls `analyticsRepo.PresetRetention(producer.DefaultWindowDays)` |
| B4 | `GET /status/kie-credits` → `{credits:int}` | calls `producer.KieCredits(ctx)`; on error return `{credits:-1,error}` (non-fatal) |
| B5 | `GET /clips/{id}/critique` → latest `clip_critiques` row `{score,changes,applied,created_at}` or null | add `CritiquesRepo.GetByClip(ctx, clipID)`; new handler method |
| B6 | `GET /agents/skill-revisions` → `[]{agent_name,rationale,critique_window,created_at}` (exclude full old/new skills text from list; keep it lean) | add `SkillRevisionsRepo.List(ctx, limit)`; new handler |
| B7 | `content_format` added to `Clip` model + `clipColumns`/`scanClip` so `GET /clips` returns it | `models/clip.go` + `repository/clips.go` |

Notes: B2/B3 need the presets handler wired with the `analyticsRepo` + producer reference in
`router.go`/`main.go`. B4 needs the producer reference. Keep handlers thin.

### Phase 2 — Frontend quick wins (Content page)

| ID | Change |
|----|--------|
| F1 | **Stop** button (POST `/orchestrator/stop`) shown only when `prodStatus.active`; **Publish TikTok** button (POST `/orchestrator/publish-tiktok`) beside the existing Publish-YouTube button |
| F2 | Content clips table: show `style_preset` as a small badge; show `content_format`; add a `producing` filter tab |

`api.ts` gains: `stopProduction()`, `publishTikTok()`. Clip type gains `style_preset`,
`content_format`.

### Phase 3 — Theme page

| ID | Change |
|----|--------|
| F3 | New **Theme** page (CONFIG_NAV, `/theme`): loads `GET /themes/active`, edits via `PATCH /themes/{id}`. Form: primary/secondary/accent color inputs (with swatch), `font_name`, `mascot_description`, `image_style`, `logo_url`. Below the form, a **read-only Style Presets panel** (from `GET /presets`): each preset as a swatch chip (primary+accent) + display name, plus the two flag states shown as on/off badges. |

`api.ts` gains: `getActiveTheme()`, `updateTheme(id, body)`, `getPresets()`.

### Phase 4 — Observability

| ID | Change |
|----|--------|
| F4 | **Analytics page**: a "By Style Preset" card — table of preset / avg-retention% / N from `GET /presets/performance`. Empty-state when no data yet (explains it warms up). |
| F5 | **Settings page**: a kie.ai credit badge from `GET /status/kie-credits` (green/amber/red by balance; "ดูไม่ได้" on error). Plus new editable fields for the B1 settings keys (audience_persona textarea; TikTok account id + two consent toggles). |
| F6 | **ReviewDialog**: show the latest critic critique (`GET /clips/{id}/critique`) — score + applied-changes summary, when present. |
| F7 | **Prompt History page**: a second tab/section listing skill-revisions (`GET /agents/skill-revisions`) — agent, rationale, date. |

`api.ts` gains: `getPresetPerformance()`, `getKieCredits()`, `getClipCritique(id)`,
`getSkillRevisions()`.

## Out of scope (explicitly not doing)

- UI toggles for env feature flags (display-only by decision above).
- Endpoints to read full `clip_critiques`/`skill_revisions` text bodies (lists stay lean;
  full-text drill-down can come later if wanted).
- Schedule create/delete (backend lacks it too; symmetric — separate future work).
- Knowledge source enable/disable toggle in UI (backend `PATCH .../{id}{enabled}` exists; minor — deferred unless asked).

## Error handling / safety

- Every new endpoint returns the standard envelope; failures return a non-2xx with `error`
  and the UI shows a toast (existing `useMutationWithToast` / `apiFetch` throw path).
- B4 kie-credits and F6 critique are best-effort reads — a failure shows a muted "unavailable"
  state, never blocks the page.
- No DB schema changes except B7 (which only reads an existing `clips.content_format` column —
  verify it exists; if the column is missing, B7 needs a migration, otherwise none).
- All changes additive; existing endpoints/contracts untouched.

## Testing

- Backend: unit/build for handlers; the two new repo methods (`CritiquesRepo.GetByClip`,
  `SkillRevisionsRepo.List`) verified on a seeded Neon branch by the controller.
- Frontend: `npm run build` (tsc) must pass; manual smoke of each new control against the
  live API after deploy.

## Rollout

Backend + frontend ship together (frontend is a separate Railway service). Phase 1 backend
deploys first (additive, safe); then frontend phases. Nothing is flag-gated — these are
additive reads + new UI, so they're safe on deploy.
