# Orchestrator Hyperframes Go-Live (Plan 2b-5a) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the orchestrator actually produce multi-scene hyperframes videos end-to-end by swapping the **back half** of the per-clip pipeline: keep the proven front half (topic/question → `ScriptAgent` narration + YouTube metadata), then break the narration into 6–10 animated scenes with `SceneAgent`, render the 9:16 MP4 via `Producer.AssembleHyperframes916` (Plan 2b-4), extract a thumbnail, upload both to kie.ai, and mark the clip ready.

**Architecture:** Low-churn, surgical swap. The orchestrator keeps `ScriptAgent` (Q&A topic → narration) and `validateScript`/metadata as-is; only `produceClipWithID`'s body after script-generation changes to `SceneAgent.Generate(narration) → ProduceHyperframes916`. A new `Producer.ProduceHyperframes916` wraps the 2b-4 `AssembleHyperframes916` with thumbnail extraction + kie upload, mirroring the old `Produce` result shape. The legacy `image` agent, `producer.Produce` static path, and the resume helpers stay in place as **dead code** (uncalled) — removing them, renaming `image`→`imageprompt`, dropping the `question` agent, the topic-driven research flow, and the frontend/registry alignment are all deferred to **Plan 2b-5b**. This keeps the diff minimal and master green.

**Tech Stack:** Go 1.25 (`github.com/jaochai/video-fb`). Reuses: `SceneAgent.Generate` (Claude, agent pkg), `Producer.AssembleHyperframes916`/`EnableHyperframes` (Plan 2b-4), `KieClient.UploadFile`, `FFmpegAssembler` (thumbnail), `sanitizeVoiceText`/`validateScript` (orchestrator). DB scene persistence already supports the 2b fields (`CreateSceneRequest` has `LayoutVariant`/`OnScreenText`/`EmphasisWords`/`Beat`/`CaptionStyle`).

---

## What "done" looks like

`ProduceWeekly` runs: pick category → `QuestionAgent` (topic + dedup, unchanged) → per clip: `ScriptAgent` → `SceneAgent` (6–10 scenes) → `ProduceHyperframes916` (per-scene TTS + gpt-image-2 + render + thumbnail + upload) → clip `ready` with `Video916URL` + `ThumbnailURL`. `go build ./...` passes; the real end-to-end run produces an MP4 (gated on Node/Chrome/keys, run manually).

## Explicitly OUT of scope (Plan 2b-5b)

- Removing the `question` agent + the Q&A/questioner concept; the topic-driven `research → script` front half.
- Renaming the `image` agent → `imageprompt`; deleting the now-dead resume helpers / `producer.Produce` static path / `imageAgent` field.
- Wiring `MetadataAgent` (the `ScriptAgent` keeps producing metadata for now).
- Migration 031 (agent_configs registry surgery) and the frontend `TEMPLATE_VARS`/`STEP_LABELS`/model-placeholder alignment.

These are left intact-but-unused so this increment stays surgical and green.

---

## File Structure

```
internal/producer/
  ffmpeg.go        MODIFY — add ExtractThumbnail(videoPath, outPath)
  producer.go      MODIFY — add ProduceHyperframes916(ctx, clipID, scenes) (*ProduceResult, error)
internal/progress/
  tracker.go       MODIFY — stepNames → {question, script, scene, assembly, upload, complete}
internal/orchestrator/
  orchestrator.go  MODIFY — add scriptNarration helper (+ test); add sceneAgent field + New param;
                            rewrite produceClipWithID back-half; simplify RetryClip to the new flow
  orchestrator_test.go  NEW/APPEND — TestScriptNarration (pure)
cmd/server/
  main.go          MODIFY — construct SceneAgent, EnableHyperframes(absolute fontsDir), pass sceneAgent to orchestrator.New
```

**Dead code left in place (removed in 2b-5b):** `imageAgent` field + `o.imageAgent` uses inside `resumeFromImagePrompts`; `resumeFromProduction`, `runProduction`, `saveImagePrompts`, `scenesToGenerated`, `buildVoiceScript`, `parseImagePrompts`; `producer.Produce`. These compile fine uncalled (Go permits unused methods/functions). The `imageCfg` parameter threaded through `produceClip`/`produceClipWithID` becomes an unused parameter (also permitted).

---

## Conventions

- Run `go build`/`go test` with the sandbox disabled on a Go build-cache "permission denied" error (known repo quirk; a real failure is different — report it).
- This plan is mostly wiring; the primary automated gate is `go build ./...` (it catches the cross-file signature changes). One pure unit test (`scriptNarration`) is added; the binding end-to-end MP4 is a documented manual smoke (needs DB + kie/openrouter keys + Node + Chrome).

---

## Task 1: Producer — thumbnail + ProduceHyperframes916

Add the orchestrator-facing wrapper that turns scenes into an uploaded 9:16 video. Additive — compiles independently of the orchestrator.

**Files:**
- Modify: `internal/producer/ffmpeg.go`
- Modify: `internal/producer/producer.go`

- [ ] **Step 1: Add `ExtractThumbnail` to `ffmpeg.go`**

Append this method to `internal/producer/ffmpeg.go` (it mirrors the existing `assembleSingleWithScale` exec pattern — `exec.Command(f.ffmpegPath, ...)`, `cmd.Stderr = os.Stderr`):

```go
// ExtractThumbnail writes the first video frame of videoPath as a PNG at outPath.
func (f *FFmpegAssembler) ExtractThumbnail(videoPath, outPath string) error {
	os.MkdirAll(filepath.Dir(outPath), 0755)
	args := []string{"-i", videoPath, "-vframes", "1", "-y", outPath}
	cmd := exec.Command(f.ffmpegPath, args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg thumbnail failed: %w", err)
	}
	return nil
}
```

(`ffmpeg.go` already imports `fmt`, `os`, `os/exec`, `path/filepath` — confirm with `go build`; add any that are missing.)

- [ ] **Step 2: Add `ProduceHyperframes916` to `producer.go`**

Append to `internal/producer/producer.go` (uses `AssembleHyperframes916` from Plan 2b-4, the existing `ProduceResult` struct, `p.ffmpeg`, `p.kie.UploadFile`, and `p.tracker`):

```go
// ProduceHyperframes916 assembles a 9:16 multi-scene MP4 from scenes (per-scene
// TTS + gpt-image-2 + render via AssembleHyperframes916), extracts a thumbnail
// from the first frame, uploads both to kie.ai, and returns their URLs. It is the
// multi-scene counterpart to the static Produce. Requires EnableHyperframes and a
// non-nil tracker (the production path always provides one).
func (p *Producer) ProduceHyperframes916(ctx context.Context, clipID string, scenes []agent.GeneratedScene) (*ProduceResult, error) {
	p.tracker.StartStep("assembly")
	mp4Path, err := p.AssembleHyperframes916(ctx, clipID, scenes)
	if err != nil {
		p.tracker.FailStep("assembly", err)
		return nil, fmt.Errorf("assemble hyperframes: %w", err)
	}
	p.tracker.CompleteStep("assembly")

	thumbPath := filepath.Join(filepath.Dir(mp4Path), "thumbnail.png")
	if err := p.ffmpeg.ExtractThumbnail(mp4Path, thumbPath); err != nil {
		return nil, fmt.Errorf("extract thumbnail: %w", err)
	}

	p.tracker.StartStep("upload")
	uploadDir := "adsvance/" + clipID
	video916URL, err := p.kie.UploadFile(ctx, mp4Path, uploadDir)
	if err != nil {
		p.tracker.FailStep("upload", err)
		return nil, fmt.Errorf("upload video: %w", err)
	}
	thumbnailURL, err := p.kie.UploadFile(ctx, thumbPath, uploadDir)
	if err != nil {
		p.tracker.FailStep("upload", err)
		return nil, fmt.Errorf("upload thumbnail: %w", err)
	}
	p.tracker.CompleteStep("upload")

	return &ProduceResult{Video916URL: video916URL, ThumbnailURL: thumbnailURL}, nil
}
```

- [ ] **Step 3: Build the package**

```bash
go build ./internal/producer/
```
Expected: clean. (`ProduceResult` already has `Video916URL`/`ThumbnailURL`; `AssembleHyperframes916` exists from Plan 2b-4.)

- [ ] **Step 4: Commit**

```bash
git add internal/producer/ffmpeg.go internal/producer/producer.go
git commit -m "feat(producer): ProduceHyperframes916 (assemble + thumbnail + upload) + ExtractThumbnail"
```

---

## Task 2: Tracker step names for the new flow

Point the progress steps at the new pipeline. `StartStep` only marks a step that already exists in the per-clip step list (initialized from `stepNames` in `StartClip`), so the new step names must live here or progress calls silently no-op.

**Files:**
- Modify: `internal/progress/tracker.go`

- [ ] **Step 1: Replace the `stepNames` slice**

In `internal/progress/tracker.go`, change the `stepNames` declaration (currently `{"question", "script", "image_prompts", "voice", "images", "assembly", "upload", "complete"}`) to:

```go
var stepNames = []string{"question", "script", "scene", "assembly", "upload", "complete"}
```

(`question`/`script` = front half; `scene` = SceneAgent breakdown; `assembly`/`upload` = `ProduceHyperframes916`; `complete` = per-clip done. The dropped `image_prompts`/`voice`/`images` were only used by the now-dead static path. The frontend `STEP_LABELS` will show the raw `scene` string until Plan 2b-5b adds its label — graceful.)

- [ ] **Step 2: Build**

```bash
go build ./...
```
Expected: clean (the dead `producer.Produce` still calls `StartStep("voice")` etc. — those become harmless no-ops; nothing references the removed names by symbol).

- [ ] **Step 3: Commit**

```bash
git add internal/progress/tracker.go
git commit -m "feat(progress): tracker steps for the hyperframes flow (question/script/scene/assembly/upload)"
```

---

## Task 3: `scriptNarration` helper (orchestrator) — TDD

The `SceneAgent` takes the full narration text; the legacy `ScriptAgent` returns it as a single scene's `voice_text` (or, defensively, several). This pure helper joins them. Write the test first.

**Files:**
- Modify: `internal/orchestrator/orchestrator.go` (add the helper)
- Test: `internal/orchestrator/orchestrator_test.go` (create if absent, else append)

- [ ] **Step 1: Write the failing test**

Create/append `internal/orchestrator/orchestrator_test.go`:

```go
package orchestrator

import (
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

func TestScriptNarration(t *testing.T) {
	t.Run("single scene", func(t *testing.T) {
		s := &agent.GeneratedScript{Scenes: []agent.GeneratedScene{{VoiceText: "สวัสดีครับ วันนี้มาเล่าเรื่องแอด"}}}
		if got := scriptNarration(s); got != "สวัสดีครับ วันนี้มาเล่าเรื่องแอด" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("joins multiple, trims, skips empty", func(t *testing.T) {
		s := &agent.GeneratedScript{Scenes: []agent.GeneratedScene{
			{VoiceText: "  ตอนแรก  "}, {VoiceText: ""}, {VoiceText: "ตอนสอง"},
		}}
		if got := scriptNarration(s); got != "ตอนแรก ตอนสอง" {
			t.Errorf("got %q, want %q", got, "ตอนแรก ตอนสอง")
		}
	})
	t.Run("no scenes", func(t *testing.T) {
		if got := scriptNarration(&agent.GeneratedScript{}); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}
```

- [ ] **Step 2: Run it — expect FAIL (scriptNarration undefined)**

```bash
go test ./internal/orchestrator/ -run TestScriptNarration -v
```
Expected: compile error — `scriptNarration` undefined.

- [ ] **Step 3: Add the helper to `orchestrator.go`**

Add near the other helpers (e.g. just below `sanitizeVoiceText`). `strings` is already imported.

```go
// scriptNarration joins all scene voice_texts into the full narration the
// SceneAgent breaks down. The legacy ScriptAgent emits a single scene whose
// voice_text is the whole narration; joining is defensive against multi-scene.
func scriptNarration(script *agent.GeneratedScript) string {
	parts := make([]string, 0, len(script.Scenes))
	for _, s := range script.Scenes {
		if t := strings.TrimSpace(s.VoiceText); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, " ")
}
```

- [ ] **Step 4: Run it — expect PASS**

```bash
go test ./internal/orchestrator/ -run TestScriptNarration -v
```
Expected: PASS (3 subtests). (The helper is unused elsewhere until Task 4 — fine for a package-level func.)

- [ ] **Step 5: Commit**

```bash
git add internal/orchestrator/orchestrator.go internal/orchestrator/orchestrator_test.go
git commit -m "feat(orchestrator): scriptNarration helper (join scene voice_texts for SceneAgent)"
```

---

## Task 4: Wire SceneAgent + hyperframes into the orchestrator (and main.go)

The core swap. These changes are compile-coupled (the `New` signature change must land with the `main.go` call), so they are one task ending in a clean `go build ./...`.

**Files:**
- Modify: `internal/orchestrator/orchestrator.go`
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Add the `sceneAgent` field to the `Orchestrator` struct**

In `internal/orchestrator/orchestrator.go`, in the `Orchestrator` struct, add a field next to `imageAgent` (keep `imageAgent` — it stays for the dead resume path):

```go
	sceneAgent    *agent.SceneAgent
```

- [ ] **Step 2: Add the `sa *agent.SceneAgent` parameter to `New` and assign it**

Change the `New(...)` signature to add `sa *agent.SceneAgent` (place it right after `ia *agent.ImageAgent`) and set `sceneAgent: sa` in the returned struct literal. The full updated constructor:

```go
func New(
	qa *agent.QuestionAgent,
	sa *agent.ScriptAgent,
	ia *agent.ImageAgent,
	sca *agent.SceneAgent,
	prod *producer.Producer,
	clips *repository.ClipsRepo,
	scenes *repository.ScenesRepo,
	themes *repository.ThemesRepo,
	agents *repository.AgentsRepo,
	settings *repository.SettingsRepo,
	formats *repository.FormatsRepo,
	tracker *progress.Tracker,
) *Orchestrator {
	return &Orchestrator{
		settingsRepo: settings, formatsRepo: formats, questionAgent: qa, scriptAgent: sa, imageAgent: ia,
		sceneAgent: sca,
		producer:   prod, clipsRepo: clips, scenesRepo: scenes,
		themesRepo: themes, agentsRepo: agents, tracker: tracker,
	}
}
```

> Note: the existing `New` already names the `ScriptAgent` param `sa`. To avoid a clash, the `SceneAgent` param is named `sca` here. Keep `scriptAgent: sa` (ScriptAgent) and add `sceneAgent: sca` (SceneAgent).

- [ ] **Step 3: Replace the body of `produceClipWithID` after script generation**

In `produceClipWithID`, keep the signature and the script-generation block (the `o.tracker.StartStep("script")` … `o.tracker.CompleteStep("script")` lines) unchanged. Replace **everything from the scene-persistence loop through the `return o.runProduction(...)`** (the old `for _, scene := range script.Scenes` save, the `image_prompts` block, the `fullVoice` assembly, the `UpsertMetadata`, and the `runProduction` return) with:

```go
	// ── Break the narration into 6-10 animated scenes (SceneAgent, Claude) ──
	o.tracker.StartStep("scene")
	sceneCfg, err := o.agentsRepo.GetByName(ctx, "scene")
	if err != nil {
		o.tracker.FailStep("scene", err)
		return o.failClip(ctx, clipID, fmt.Errorf("get scene config: %w", err))
	}
	narration := scriptNarration(script)
	scenes, err := o.sceneAgent.Generate(ctx, narration, targetSceneCount, targetDurationSec, theme, sceneCfg)
	if err != nil {
		o.tracker.FailStep("scene", err)
		return o.failClip(ctx, clipID, fmt.Errorf("scene breakdown: %w", err))
	}
	// Sanitize each scene's narration for TTS (brand aliases, strip URLs/@handles).
	for i := range scenes {
		scenes[i].VoiceText = sanitizeVoiceText(scenes[i].VoiceText, brandAliases)
	}
	o.tracker.CompleteStep("scene")

	// Persist scenes with the 2b layout/caption fields.
	for _, scene := range scenes {
		emphasis, mErr := json.Marshal(scene.EmphasisWords)
		if mErr != nil || len(emphasis) == 0 {
			emphasis = []byte("[]")
		}
		overlays := scene.TextOverlays
		if overlays == nil {
			overlays = []byte("[]")
		}
		o.scenesRepo.Create(ctx, models.CreateSceneRequest{
			ClipID:          clipID,
			SceneNumber:     scene.SceneNumber,
			SceneType:       scene.SceneType,
			TextContent:     scene.TextContent,
			ImagePrompt:     scene.ImagePrompt,
			VoiceText:       scene.VoiceText,
			DurationSeconds: scene.DurationSeconds,
			TextOverlays:    overlays,
			LayoutVariant:   scene.LayoutVariant,
			OnScreenText:    scene.OnScreenText,
			EmphasisWords:   emphasis,
			Beat:            scene.Beat,
			CaptionStyle:    scene.CaptionStyle,
		})
	}

	// Metadata from the validated script.
	o.clipsRepo.UpsertMetadata(ctx, models.ClipMetadata{
		ClipID:       clipID,
		YoutubeTitle: &script.YoutubeTitle,
		YoutubeDesc:  &script.YoutubeDescription,
		YoutubeTags:  script.YoutubeTags,
	})

	// ── Assemble the multi-scene 9:16 video + thumbnail + upload ──
	result, err := o.producer.ProduceHyperframes916(ctx, clipID, scenes)
	if err != nil {
		return o.failClip(ctx, clipID, fmt.Errorf("produce hyperframes: %w", err))
	}

	readyStatus := "ready"
	o.clipsRepo.Update(ctx, clipID, models.UpdateClipRequest{
		Status:       &readyStatus,
		Video916URL:  &result.Video916URL,
		ThumbnailURL: &result.ThumbnailURL,
		VoiceScript:  &narration,
		AnswerScript: &narration,
	})
	log.Printf("Clip ready (hyperframes): %s", clipID)
	return nil
```

> `imageCfg` (a `produceClipWithID` parameter) is no longer referenced in the body — that is a permitted unused parameter; do not remove it (keeping the signature avoids touching `produceClip`/`ProduceWeekly`/`RetryClip` call sites). `theme`, `scriptCfg`, `format`, `persona`, `brandAliases`, `q` all remain used (script-gen + scene-gen + sanitize).

- [ ] **Step 4: Add the scene-count / duration constants**

Add near the top of `orchestrator.go` (package level, by the other vars/consts):

```go
// Target shape for the multi-scene explainer (design: 60–90 s, 6–10 scenes).
const (
	targetSceneCount  = 8
	targetDurationSec = 75
)
```

- [ ] **Step 5: Simplify `RetryClip` to always re-run the new full pipeline**

Replace the entire `RetryClip` method body with this (it drops the `ListByClip` + `resumeFromImagePrompts`/`resumeFromProduction` branches; those methods stay defined-but-unused). It still fetches `imageCfg` so the `produceClipWithID` signature is satisfied (unused inside):

```go
func (o *Orchestrator) RetryClip(ctx context.Context, clip *models.Clip) error {
	log.Printf("Retrying failed clip %s: %s", clip.ID, clip.Title)

	brandAliases, err := o.settingsRepo.GetBrandAliases(ctx)
	if err != nil {
		return o.failClip(ctx, clip.ID, fmt.Errorf("read brand aliases: %w", err))
	}

	status := "producing"
	o.clipsRepo.Update(ctx, clip.ID, models.UpdateClipRequest{Status: &status})

	q := agent.GeneratedQuestion{
		Question:       clip.Question,
		QuestionerName: clip.QuestionerName,
		Category:       clip.Category,
	}

	theme, err := o.themesRepo.GetActive(ctx)
	if err != nil {
		return o.failClip(ctx, clip.ID, fmt.Errorf("get theme: %w", err))
	}
	scriptCfg, err := o.agentsRepo.GetByName(ctx, "script")
	if err != nil {
		return o.failClip(ctx, clip.ID, fmt.Errorf("get script config: %w", err))
	}
	imageCfg, err := o.agentsRepo.GetByName(ctx, "image")
	if err != nil {
		return o.failClip(ctx, clip.ID, fmt.Errorf("get image config: %w", err))
	}
	format, err := o.formatsRepo.GetByName(ctx, "qa")
	if err != nil {
		format = &models.ContentFormat{FormatName: "qa", DisplayName: "Q&A"}
	}
	persona, _ := o.settingsRepo.Get(ctx, "audience_persona")

	return o.produceClipWithID(ctx, clip.ID, q, theme, scriptCfg, imageCfg, brandAliases, format, persona)
}
```

> The methods `resumeFromImagePrompts`, `resumeFromProduction`, `runProduction`, `saveImagePrompts` and the helpers `scenesToGenerated`, `buildVoiceScript`, `parseImagePrompts` are now uncalled but still compile (they reference the retained `imageAgent` field / `producer.Produce`). Leave them — Plan 2b-5b removes them with the `image`→`imageprompt` rename.

- [ ] **Step 6: Update `cmd/server/main.go` — construct SceneAgent, enable hyperframes, pass to `New`**

In `cmd/server/main.go`:

(a) After `imageAgent := agent.NewImageAgent(llm)` (keep that line), add:
```go
	sceneAgent := agent.NewSceneAgent(llm)
```

(b) After `prod := producer.NewProducer(...)`, enable the render engine with an **absolute** fonts dir (per the 2b-4 review caveat — the render runs in a per-clip dir, so a relative path would break):
```go
	fontsDir := os.Getenv("FONTS_DIR")
	if fontsDir == "" {
		fontsDir = "internal/producer/assets/fonts"
	}
	if abs, err := filepath.Abs(fontsDir); err == nil {
		fontsDir = abs
	}
	prod.EnableHyperframes(fontsDir)
```

(c) Update the `orchestrator.New(...)` call to pass `sceneAgent` right after `imageAgent`:
```go
	orch := orchestrator.New(questionAgent, scriptAgent, imageAgent, sceneAgent, prod,
		clipsRepo, scenesRepo, themesRepo, agentsRepo, settingsRepo, formatsRepo, tracker)
```

(d) Ensure `main.go` imports `path/filepath` (it already imports `os`). Add `"path/filepath"` to the import block if absent; `go build` will flag it.

> **Deploy note (for Plan 2b-6):** on Railway the binary runs from the repo root, so the default `internal/producer/assets/fonts` resolves; the Dockerfile should either preserve that layout or set `FONTS_DIR` to the image path. Without the real Sarabun fonts at `fontsDir`, the render falls back to a default font (Thai may render as boxes) — so this path must be correct in prod.

- [ ] **Step 7: Build the whole module**

```bash
go build ./... && go vet ./...
```
Expected: clean. This is the key gate — it proves the `New` signature change, the `main.go` call, the new `produceClipWithID` body, and `RetryClip` all line up.

- [ ] **Step 8: Run the existing tests**

```bash
go test ./...
```
Expected: PASS (including `TestScriptNarration`; producer/agent tests unaffected; the hyperframes smoke self-skips).

- [ ] **Step 9: Commit**

```bash
git add internal/orchestrator/orchestrator.go cmd/server/main.go
git commit -m "feat(orchestrator): wire SceneAgent → hyperframes render into the per-clip pipeline"
```

---

## Task 5: Full verification + manual end-to-end smoke

**Files:** none (verification only)

- [ ] **Step 1: Build, vet, test the whole module**

```bash
go build ./... && go vet ./... && go test ./...
```
Expected: all PASS.

- [ ] **Step 2: Confirm the live per-clip path no longer calls the static producer**

```bash
grep -n 'o.producer.Produce\b\|ProduceHyperframes916\|o.sceneAgent\|o.imageAgent' internal/orchestrator/orchestrator.go
```
Expected: `produceClipWithID` calls `o.producer.ProduceHyperframes916` and `o.sceneAgent.Generate`; the only `o.producer.Produce(` / `o.imageAgent.` references left are inside the dead `runProduction`/`resumeFromImagePrompts` methods (uncalled). No live path reaches the static `Produce`.

- [ ] **Step 3: Confirm scope — no registry/frontend/migration churn**

```bash
git diff d9cdeae..HEAD --name-only | grep -E 'migrations/|frontend/|internal/agent/' && echo "UNEXPECTED — investigate" || echo "✓ only orchestrator/producer/progress/main touched"
```
Expected: `✓ only orchestrator/producer/progress/main touched`.

- [ ] **Step 4: Manual end-to-end smoke (real machine — document the result honestly)**

This is the binding proof; it needs a real DB (kie/openrouter keys in `settings`), Node + Chrome on PATH, and network. It cannot run in the sandbox/CI.

```bash
# 1. Point at the DB and run the server from the repo root (so fontsDir resolves):
DATABASE_URL=<neon-url> go run ./cmd/server &
# 2. Trigger a small production run via the existing API (1 clip):
curl -X POST localhost:8080/api/v1/orchestrator/produce -H 'Content-Type: application/json' -d '{"count":1}'
# 3. Watch logs for: question → script → scene → assembly (hyperframes render) → upload → "Clip ready (hyperframes)".
# 4. Check the clip row got Video916URL + ThumbnailURL set, and open the MP4: 9:16, 60–90 s, 6–10 animated scenes, Thai captions, brand colors, no scene-freeze.
```
**Honesty:** if Node/Chrome/keys are unavailable, report that the smoke could not run rather than implying a video was produced. The automated gate for this plan is `go build ./...` + `TestScriptNarration`; the MP4 itself is verified here (or on the Plan-2b-6 Docker image).

---

## Self-Review Notes

- **Spec coverage:** Implements design §4.5 (orchestrator runs script → scene → assemble per clip) for the **back half**, plus §4.4 g/h (upload + thumbnail) via `ProduceHyperframes916`. The topic-driven front half (research-selected topic, questioner removal), `image`→`imageprompt`, `MetadataAgent`, the agent-registry migration, and the frontend alignment are the explicit 2b-5b scope (the user chose "video out first").
- **Reuse / low churn:** keeps `ScriptAgent`, `validateScript`, `sanitizeVoiceText`, metadata, and the question front-half unchanged; the only behavioral change is the per-clip back-half and `RetryClip`. The static path and `image` agent are retained as dead code rather than risk a wide deletion — surgical diff, master green.
- **Green at every step:** Tasks 1–3 are additive and independently buildable/committable. Task 4 is the one compile-coupled change (orchestrator `New` ↔ `main.go`), landed and built together. `go build ./...` is the cross-file gate.
- **Type consistency:** `SceneAgent.Generate(ctx, script string, targetSceneCount, targetDurationSec int, theme *models.BrandTheme, cfg) ([]GeneratedScene, error)` — call matches. `ProduceHyperframes916(ctx, clipID, scenes) (*ProduceResult, error)` returns `Video916URL`/`ThumbnailURL`, consumed into `UpdateClipRequest`. `CreateSceneRequest` carries every 2b field; `EmphasisWords` is `json.RawMessage`, so `scene.EmphasisWords` (`[]string`) is `json.Marshal`ed. `ClipMetadata` uses `*string` for title/desc (hence `&script.YoutubeTitle`). `New` adds `sca *agent.SceneAgent` (named to avoid the existing `sa` ScriptAgent param).
- **Known limitation (surfaced):** the narration fed to `SceneAgent` comes from the Q&A `ScriptAgent` (single-scene, Q&A-shaped). The result is a Q&A answer rendered as a multi-scene animated video — correct and shippable, but not yet the topic-driven explainer of §4.5 (that's 2b-5b). `RetryClip` now fully re-runs script+scene (spends Claude credits) instead of resuming — acceptable for rare retries; noted.
- **No placeholders:** every modified function's new body is shown in full; the tracker change, the helper + its test, the producer methods, and the main.go edits are concrete; each step ends in a runnable command with expected output.
- **Honesty flag:** automated coverage = `go build` (signature wiring) + `TestScriptNarration` (pure). The end-to-end MP4 is a **manual** smoke (DB + keys + Node + Chrome); Task 5 Step 4 says to report a non-run honestly. The design §10.1 success criterion is met when that smoke (or the 2b-6 Docker deploy) actually renders a clip.

---

## Execution Handoff

**Plan complete and saved to `docs/superpowers/plans/2026-06-10-orchestrator-hyperframes-golive-plan2b5a.md`. Two execution options:**

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints.

**Which approach?**
