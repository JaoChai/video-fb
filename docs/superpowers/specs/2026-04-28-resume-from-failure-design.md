# Resume from Failure — Design Spec

## Problem

When a clip fails during production, `RetryClip()` deletes all scenes and metadata then restarts from scratch. This wastes Claude API credits that were already spent on script generation and image prompt generation.

## Solution

Derive resume point from existing data. No new database columns or migrations required.

### Resume Logic

```
RetryClip starts
  ├─ scenes exist for clip?
  │   ├─ NO  → run full pipeline (script → image_prompts → production)
  │   └─ YES → skip script ✓
  │       ├─ scenes.image_prompt populated?
  │       │   ├─ NO  → run image_prompts → production
  │       │   └─ YES → skip image_prompts ✓ → run production only
```

Three phases:
1. **Script** — Claude API generates script, saves scenes to DB
2. **Image Prompts** — Claude API generates prompts, saves to `scenes.image_prompt` as JSON
3. **Production** — Kie AI voice + images, FFmpeg assembly, upload

### Data Persistence at Each Phase

| Phase | Output | Where Stored | Currently Persisted? |
|-------|--------|-------------|---------------------|
| Script | scenes array | `scenes` table | Yes |
| Image Prompts | 169 + 916 prompts | `scenes.image_prompt` | **No — fix this** |
| Production | voice, images, video | Kie AI upload URLs | Yes (on success) |

## Changes

### 1. Save image prompts to scenes (orchestrator.go)

After `imageAgent.GeneratePrompts()` succeeds, update each scene's `image_prompt` column with JSON:

```json
{"169": "prompt for 16:9 image...", "916": "prompt for 9:16 image..."}
```

### 2. Add UpdateImagePrompt to ScenesRepo (scenes.go)

```go
func (r *ScenesRepo) UpdateImagePrompt(ctx context.Context, clipID string, sceneNumber int, prompt string) error
```

Updates `scenes.image_prompt` WHERE `clip_id` AND `scene_number` match.

### 3. Rewrite RetryClip (orchestrator.go)

Current behavior (DELETE everything, start over):
```go
DELETE FROM scenes WHERE clip_id = $1
DELETE FROM clip_metadata WHERE clip_id = $1
// re-run full pipeline
```

New behavior (check existing data, resume):
```go
scenes := scenesRepo.ListByClip(ctx, clipID)
if len(scenes) == 0 {
    // Phase 1: full pipeline
    return produceClipWithID(ctx, clipID, q, theme, scriptCfg, imageCfg)
}

// Script done — rebuild GeneratedScene from DB scenes
if scenes[0].ImagePrompt == "" {
    // Phase 2: generate image prompts + production
    return resumeFromImagePrompts(ctx, clipID, scenes, theme, imageCfg)
}

// Image prompts done — parse from JSON
// Phase 3: production only
return resumeFromProduction(ctx, clipID, scenes)
```

### 4. New helper methods in orchestrator

- `resumeFromImagePrompts(ctx, clipID, scenes, theme, imageCfg)` — generates image prompts from existing scenes, saves to DB, then runs production
- `resumeFromProduction(ctx, clipID, scenes)` — parses image prompts from `scenes.image_prompt` JSON, rebuilds voice script from scenes, runs producer

### 5. Rebuild data from scenes

When resuming, reconstruct the data that `produceClipWithID` normally gets from agents:

- `GeneratedScene` array: rebuild from `scenes` table rows (scene_number, scene_type, text_content, voice_text, duration_seconds, text_overlays)
- `SceneImagePrompts`: parse from `scenes.image_prompt` JSON
- `voiceScript`: concatenate `voice_text` from all scenes

## Files Changed

| File | Change |
|------|--------|
| `internal/orchestrator/orchestrator.go` | Save image prompts after generation, rewrite RetryClip with resume logic, add helper methods |
| `internal/repository/scenes.go` | Add `UpdateImagePrompt()` method |

## No Changes Needed

- No new migrations (uses existing `scenes.image_prompt` column)
- No model changes (Scene.ImagePrompt already exists as string)
- No frontend changes
- No API changes
- Scheduler retry logic stays the same (just calls RetryClip which now resumes)

## Edge Cases

- **Clip has scenes but some are corrupted/partial**: treated as "script done" — image prompts regenerated, which is cheap insurance
- **Image prompt JSON is malformed**: fall back to regenerating image prompts (log warning)
- **Production fails again on retry**: normal fail_clip flow, retry_count increments, next scheduler tick tries again with same resume logic
- **Clip metadata already exists**: `ON CONFLICT DO UPDATE` in produceClipWithID handles this already
