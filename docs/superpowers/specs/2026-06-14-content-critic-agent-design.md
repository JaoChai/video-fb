# Content Critic Agent — Design Spec (Phase 1)

**Date:** 2026-06-14
**Status:** Approved (brainstorming) — pending implementation plan
**Author:** brainstorming session

## Context

The video pipeline has 6 LLM agents (Question, Script, Scene, Image, Metadata,
Research). All are **content generators** — none reviews or improves another's
output. Layout/styling is fixed in `internal/producer/templates/layout_multi_scene.html.tmpl`
and producer Go code; agents only fill content slots.

The user wants an **improvement agent** that raises video quality. That request
decomposes into 4 subsystems, to be built in phases:

1. **Phase 1 — Content Critic** (this spec): review + improve generated content
   (script, scenes, image prompts, metadata) BEFORE render. Text-only, cheap.
2. **Phase 2 — Visual QA**: inspect rendered frames, catch visual bugs
   (caption overflow, off-brand, ugly images). Separate spec later.
3. **Phase 3 — Learning loop**: evolve agent prompts/skills from accumulated
   critique signal. Separate spec later. Depends on Phase 1's critique data.

Phases 2 and 3 are explicitly **out of scope** for this spec.

## Goal

Insert one **Content Critic agent** into the production pipeline that reviews the
fully-generated content bundle and revises it in place ("Approach A:
revise-in-place"), so the rendered video is built from improved content — with a
fail-safe that guarantees output is **never worse** than today.

### Success criteria

- A `critic` agent reviews `{script narration, scenes[], metadata}` and returns
  an improved bundle + per-dimension score + changelog.
- Improved content flows to render; on ANY failure (LLM error, malformed output,
  validation failure, `enabled=false`) the **original content is used unchanged**.
- Critic edits are bounded: it may change content fields only, never timing or
  layout-type fields.
- Each review's score + changelog is persisted (quality signal for Phase 3).
- Fail-safe behavior is covered by unit tests.

## Non-goals (YAGNI)

- No visual / rendered-frame inspection (Phase 2).
- No critique→regenerate retry loop (Approach B).
- No automatic prompt/skill evolution (Phase 3).
- No human-review UI.
- No re-render after critique (critique happens before the single render).

## Architecture

### Hook point

In `orchestrator.produceClipWithID`, insert the critic **after** `sceneAgent.Generate`
+ `sanitizeVoiceText`, and **before** persisting scenes / metadata and calling
`producer.ProduceHyperframes916`.

```
scriptAgent ─► sceneAgent ─► sanitize ─┐
                                        ▼
                              CriticAgent.Review
                              in:  narration + scenes[] + metadata
                              out: scenes' + metadata' + score + changes
                                        │
                    ┌───────────────────┴────────────────────┐
              output valid?                            invalid / error / disabled
                    │ yes                                      │  (fail-safe)
                    ▼                                          ▼
            use scenes' + metadata'                   use original scenes + metadata
                    └──────────────────┬───────────────────────┘
                                       ▼
                  persist scenes + metadata + clip_critiques row
                                       ▼
                        producer.ProduceHyperframes916()
```

The critic is an **optional gate**: if `agent_configs['critic']` is missing or
`enabled=false`, the orchestrator skips it silently and renders the original
content.

### Component: `CriticAgent`

New file `internal/agent/critic.go`, following the exact pattern of `SceneAgent`:

```go
type CriticAgent struct { llm *KieLLMClient }
func NewCriticAgent(llm *KieLLMClient) *CriticAgent
func (a *CriticAgent) Review(ctx context.Context, in CriticInput, cfg *models.AgentConfig) (*CriticOutput, error)
```

Uses the existing `llm.GenerateJSON(ctx, cfg.Model, cfg.BuildSystemPrompt(), userPrompt, cfg.Temperature, &out)`.

**CriticInput:** source question, narration, `scenes []GeneratedScene` (each with
voice_text, layout, image_prompt, on-screen text, emphasis), metadata
(youtube_title / description / tags). (Image prompts are per-scene
`scene.ImagePrompt`, so reviewing image prompts is part of scene review.)

**CriticOutput (structured JSON):**
```jsonc
{
  "scenes":   [ /* revised scenes — same shape, same count, same SceneNumbers */ ],
  "metadata": { "youtube_title": "...", "youtube_description": "...", "youtube_tags": [...] },
  "score":    { "hook": 8, "clarity": 7, "brand_fit": 9, "overall": 8 },  // each 1-10
  "changes":  [ { "field": "scene[0].voice_text", "reason": "hook ไม่ดึงใน 2 วิแรก" } ]
}
```

### Edit boundaries (validation after LLM, before use)

Critic may edit **content only**. The following are immutable and reverted to the
original value if the critic changed them:

- `DurationSeconds` (affects TTS timing / caption sync)
- `SceneType` and `LayoutVariant` (affect which template layout renders)
- `SceneNumber` (identity / ordering)

Editable: `VoiceText`, `ImagePrompt`, `OnScreenText`, `TextContent`,
`EmphasisWords`, and metadata fields.

### Fail-safe validation rules

`Review` returns the **original** bundle (no error propagated to the pipeline) when:

- LLM call errors, or output is not valid JSON for the schema.
- Scene count differs from input, or any `SceneNumber` is missing/duplicated.
- Any scene has empty `VoiceText` after revision.
- Score values fall outside 1–10.

When an immutable field was changed, that field alone is reverted; the rest of the
valid revision is kept.

## Data: `clip_critiques` (append-only)

New table (new migration):

| column      | type        | note                          |
|-------------|-------------|-------------------------------|
| id          | uuid pk     |                               |
| clip_id     | text/uuid   | FK to clips                   |
| score       | jsonb       | the score object              |
| changes     | jsonb       | the changes array             |
| created_at  | timestamptz | default now()                 |

Phase 1 only **writes** this table. Phase 3 reads it to find recurring low-score
patterns and update upstream agents' `skills`.

## Config: `agent_configs['critic']` row (new migration)

| column        | value                                                              |
|---------------|-------------------------------------------------------------------|
| agent_name    | `critic`                                                          |
| system_prompt | review rubric: hook strength, Thai naturalness/flow, persona & brand fit, image-prompt brand alignment, metadata search-intent |
| skills        | editable guidelines — Phase 3's write target                      |
| model         | Claude Sonnet (DB-configurable)                                    |
| temperature   | ~0.3 (stable critique)                                            |
| enabled       | `true`                                                            |

`enabled=false` is a free kill switch — no redeploy needed.

## Wiring

- `internal/agent/critic.go` — new agent + input/output types + validation.
- `internal/orchestrator/orchestrator.go` — add `criticAgent` field, add to
  `NewOrchestrator` params, insert review block in `produceClipWithID`, load cfg
  via `o.agentsRepo.GetByName(ctx, "critic")`.
- `internal/repository/` — small repo method to insert a `clip_critiques` row.
- `migrations/033_*.sql` — `clip_critiques` table.
- `migrations/034_*.sql` — `agent_configs['critic']` seed row.
  (Exact numbers/ordering finalized in the implementation plan; latest is 032.)

## Testing

Following the existing mock-LLM pattern in `internal/agent/scene_test.go`:

- Malformed JSON from LLM → `Review` returns original bundle, no error.
- Critic changes `DurationSeconds` → reverted to original.
- Scene count mismatch / missing SceneNumber → original bundle returned.
- Happy path → output passes schema, scores within 1–10, changelog non-nil.

## Cost / performance

One extra LLM call per clip, inside the batch `ProduceWeekly` loop. Negligible
vs the render step (≈20 min, CPU-bound). No re-render added.

## Open items for the implementation plan

- Exact `GeneratedScene` JSON field tags the critic schema must mirror.
- Whether `clip_critiques` is written even on fail-safe (proposal: yes, with a
  flag/empty changes, so Phase 3 can see "review ran but kept original").
- Final migration numbers.
