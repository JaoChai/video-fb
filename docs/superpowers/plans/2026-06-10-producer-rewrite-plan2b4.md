# Producer Rewrite — Scenes → Hyperframes MP4 (Plan 2b-4) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a new `Producer.AssembleHyperframes916` path that turns `[]agent.GeneratedScene` (the SceneAgent's output) into a 9:16 multi-scene MP4 via the Plan-2b-3 render engine: per-scene TTS (measured by WAV duration) → per-scene gpt-image-2 backgrounds (kie.ai) → fill the multi-scene template → `npx hyperframes render`. **Additive only** — the existing static-image `Produce` path and `NewProducer` signature are untouched, nothing is wired into the orchestrator yet, so master stays green.

**Architecture:** Reuse the proven scene→render glue from the rolled-back branch `redesign/hyperframes-video-engine` ([[project_hyperframes_rollback]]), adapting it from that branch's `CompositionAgent`/`SceneDesign` model to the current master's `SceneAgent`/`GeneratedScene` model. Three helpers port **verbatim** (they already take `agent.GeneratedScene`): caption-segment building, per-scene voice synthesis, and the WAV-timing/bounds math. One helper is **rewritten**: `buildSceneSpecs` maps `GeneratedScene → SceneSpec` (the render engine's input), building a single headline slot per scene from `on_screen_text` + `emphasis_words` and normalizing `layout_variant` to a template-supported value. The engine ([[project_kie_hyperframes_redesign]] Plan 2b-3, in `internal/producer/`: `CompositionBuilder.BuildScenes` + `HyperframesRenderer.Render`) is wired into `Producer` via an additive `EnableHyperframes` setter.

**Tech Stack:** Go 1.25 (module `github.com/jaochai/video-fb`), `package producer`. Existing clients reused: `OpenRouterClient.GenerateVoice` (TTS, unchanged per design), `KieClient.GenerateImage` (gpt-image-2 9:16 via kie.ai), `KieClient.UploadFile`. Plan-2b-3 engine: `CompositionBuilder`, `HyperframesRenderer`, `buildScenePrompt`, `highlightTitle`, `SceneSpec`/`SlotSpec`/`ScenesParams`, `Brand`.

**Source ref for verbatim ports:** `redesign/hyperframes-video-engine` (existing local branch; `git show <ref>:<path>` reads its blob).

---

## Design decisions (read before implementing)

**1. GeneratedScene → SceneSpec mapping (the core adaptation).** The current `SceneAgent` emits, per scene: `scene_number`, `voice_text`, `on_screen_text` (one line), `emphasis_words[]`, `layout_variant`, `caption_style`, `duration_seconds`, `image_prompt`, `beat`. The render engine's `SceneSpec` wants `Slots []SlotSpec`, `AccentColor`, `AnimationSpeed`, `StartSec/EndSec`, `BackgroundMode`, `CaptionStyle`, `LayoutVariant`. The mapping:
- **Slots:** one `{Role:"headline", HTML: highlightTitle(on_screen_text, emphasis_words)}`. The SceneAgent emits a single on-screen line per scene, so the template's multi-slot layouts (`list_steps`, `stat_reveal`, `compare_two`) degrade gracefully to a headline. (Richer per-slot content would require expanding the SceneAgent's JSON contract — deferred; out of scope here.)
- **LayoutVariant:** normalized to the **template-supported** set `{hook_big, hook_punch, list_steps, stat_reveal, compare_two, quote_cta}`. The SceneAgent's seeded enum (migration 030) also lists `phrase_block`, `word_pop`, `static`, `intro`, `outro` — those are NOT scene layouts the template's CSS styles (`phrase_block`/`word_pop` are *caption* styles; `static`/`intro`/`outro` are bumper concepts), so they normalize to `hook_big` (the default centered column). This is a known SceneAgent-prompt/template enum mismatch; normalizing here is the surgical fix (correcting the seeded prompt is optional polish for Plan 2b-5).
- **CaptionStyle:** clamped to `{word_pop, phrase_block}` (default `phrase_block`).
- **AccentColor:** `Brand.Orange` (the SceneAgent does not choose per-scene colors).
- **AnimationSpeed:** `"normal"` (the SceneAgent does not emit speed).
- **BackgroundMode:** `"image"` when `image_prompt` is non-empty, else `"css"`; the actual image is generated per-scene and a missing/failed one downgrades to `"css"` inside `BuildScenes`.
- **StartSec/EndSec:** from the measured per-scene audio bounds (`computeBounds`), the source of truth for timing.

**2. Images = gpt-image-2 via kie.ai** (`KieClient.GenerateImage`, design §2/§4.4), not the branch's OpenAI image client and not OpenRouter. Per-scene background art; failures are non-fatal (css fallback).

**3. Captions from ground-truth `voice_text`, not ASR.** `captionSegmentsFromScenes` divides each scene's measured `[start,end)` window across its phrases by rune count — captions are correct by construction (the branch's fix for Whisper mangling Thai).

**4. Scope boundary — what 2b-4 does NOT do.** No upload / thumbnail / clip-status update (that's orchestrator-level, Plan 2b-5). No orchestrator wiring, no `main.go` change, no `image`→`imageprompt` rename, no `question` removal, no frontend. `AssembleHyperframes916` returns the **local** `output.mp4` path; Plan 2b-5 calls it, uploads, sets `Video916URL`/`ThumbnailURL`. The existing `Produce` static path stays fully intact.

---

## File Structure

```
internal/producer/
  captions.go            NEW  — captionSegmentsFromScenes + splitCaptionPhrases (verbatim port)
  captions_test.go       NEW  — pure caption tests (verbatim port)
  scene_timing.go        NEW  — sceneBound, computeBounds, wavDurationSeconds, readWAVPCM (verbatim port)
  scene_timing_test.go   NEW  — TestComputeBounds (verbatim) + a wav-duration test
  scene_adapter.go       NEW  — buildSceneSpecs(scenes, bounds) + normalizeLayout/normalizeCaptionStyle (REWRITTEN for GeneratedScene)
  scene_adapter_test.go  NEW  — pure adapter tests (new)
  scene_voice.go         NEW  — synthScenesVoice (per-scene TTS → combined voice.wav + bounds) (verbatim port)
  producer.go            MODIFY — add `hf *hyperframesDeps` field, EnableHyperframes, AssembleHyperframes916
  producer_hyperframes_test.go  NEW — HF_RENDER-gated end-to-end smoke (skips in CI)
```

**Why this stays green:** `captions.go`, `scene_*.go` are new files. `producer.go` only *adds* a struct field (zero-value `nil`), one setter, and one method — the existing `Produce`/`NewProducer`/`getVoice` are unchanged, so `main.go` (which calls `NewProducer` + `Produce`) compiles and runs exactly as before. `go test ./...` passes because the new logic is unit-tested and the network/Chrome end-to-end is env-gated.

---

## Conventions

- `REF` = `redesign/hyperframes-video-engine`. "Extract verbatim" = `git show REF:<path> > <dest>` then make only the named edits.
- Run `go build`/`go test` with the sandbox disabled if you hit a Go build-cache permission error (known repo quirk; a real assertion failure is different — report it).
- TDD where new logic is written (Task 3 adapter, Task 2 timing): test first, watch it fail, implement, watch it pass.

---

## Task 1: Caption segments from scene narration (`captions.go`)

Port verbatim — `captionSegmentsFromScenes` already takes `[]agent.GeneratedScene` and needs no change. It splits each scene's `VoiceText` into caption-sized Thai phrases and spreads them across the scene's measured `[start,end)` window by rune weight.

**Files:**
- Create: `internal/producer/captions.go`
- Test: `internal/producer/captions_test.go`

- [ ] **Step 1: Extract `captions.go` and `captions_test.go` verbatim**

```bash
git show redesign/hyperframes-video-engine:internal/producer/captions.go      > internal/producer/captions.go
git show redesign/hyperframes-video-engine:internal/producer/captions_test.go > internal/producer/captions_test.go
```

`captions.go` defines `const captionMaxRunes = 42`, `captionSegmentsFromScenes(scenes []agent.GeneratedScene, bounds []sceneBound) []TranscriptSegment`, and `splitCaptionPhrases(text string) []string`. It imports `math`, `strings`, and `github.com/jaochai/video-fb/internal/agent`. (It references `sceneBound` and `TranscriptSegment` — `TranscriptSegment` exists from Plan 2b-3; `sceneBound` is added in Task 2. So this file compiles only after Task 2. Run the test in Task 2.)

- [ ] **Step 2: Verify verbatim**

```bash
git show redesign/hyperframes-video-engine:internal/producer/captions.go | diff - internal/producer/captions.go && echo "captions.go IDENTICAL"
git show redesign/hyperframes-video-engine:internal/producer/captions_test.go | diff - internal/producer/captions_test.go && echo "captions_test.go IDENTICAL"
```
Expected: both IDENTICAL (empty diff). (The test asserts caption text is byte-for-byte the ground-truth `VoiceText` with nothing invented, timing monotonic within bounds, last segment pinned to the final boundary, and empty/zero-width scenes skipped.)

> Do NOT commit yet — `captions.go` references `sceneBound` from Task 2 and won't compile alone. Commit at the end of Task 2.

---

## Task 2: Scene timing + WAV helpers (`scene_timing.go`)

Port verbatim the timing math: `sceneBound`, `computeBounds`, `wavDurationSeconds`, `readWAVPCM`. These have no `agent` dependency and are pure (except file reads). After this task, Task 1's `captions.go` compiles, so commit both together.

**Files:**
- Create: `internal/producer/scene_timing.go`
- Test: `internal/producer/scene_timing_test.go`

- [ ] **Step 1: Create `scene_timing.go` with the four symbols (extracted from REF's `multiscene.go`)**

Extract just these symbols from `git show redesign/hyperframes-video-engine:internal/producer/multiscene.go` — `sceneBound`, `computeBounds`, `wavDurationSeconds`, `readWAVPCM` — into a new `internal/producer/scene_timing.go` with this exact content (verbatim from the branch; package + imports adjusted for the focused file):

```go
package producer

import (
	"encoding/binary"
	"fmt"
	"os"
)

// sceneBound is one scene's [start, end) window on the combined audio timeline.
type sceneBound struct{ Start, End float64 }

// computeBounds turns per-scene durations into cumulative [start, end) windows.
// Example: [8, 11, 5] → [{0,8},{8,19},{19,24}]
func computeBounds(durations []float64) []sceneBound {
	if len(durations) == 0 {
		return nil
	}
	bounds := make([]sceneBound, len(durations))
	var cursor float64
	for i, d := range durations {
		bounds[i] = sceneBound{Start: cursor, End: cursor + d}
		cursor += d
	}
	return bounds
}

// wavDurationSeconds reads the PCM data-chunk size from a WAV file header and
// converts it to seconds using the same 24 kHz / 16-bit / mono parameters
// that GenerateVoice writes.
func wavDurationSeconds(path string) (float64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open wav %s: %w", path, err)
	}
	defer f.Close()

	// Minimal WAV header: 44 bytes.
	// Bytes 40-43: PCM data chunk size (little-endian uint32).
	var header [44]byte
	if _, err := f.Read(header[:]); err != nil {
		return 0, fmt.Errorf("read wav header %s: %w", path, err)
	}
	// Sanity check RIFF / WAVE markers.
	if string(header[0:4]) != "RIFF" || string(header[8:12]) != "WAVE" {
		return 0, fmt.Errorf("not a valid WAV file: %s", path)
	}

	dataSize := binary.LittleEndian.Uint32(header[40:44])
	const sampleRate = 24000
	const bytesPerSample = 2 // 16-bit mono
	duration := float64(dataSize) / float64(sampleRate*bytesPerSample)
	return duration, nil
}

// readWAVPCM reads just the PCM payload (everything after the 44-byte header)
// from a WAV file written by GenerateVoice.
func readWAVPCM(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read wav %s: %w", path, err)
	}
	if len(data) < 44 {
		return nil, fmt.Errorf("wav file too small: %s", path)
	}
	return data[44:], nil
}
```

- [ ] **Step 2: Write the timing tests**

Create `internal/producer/scene_timing_test.go`. Port `TestComputeBounds` verbatim from the branch's `multiscene_test.go` and add a round-trip WAV-duration test using the existing `wrapPCMAsWAV` (in `openrouter.go`):

```go
package producer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestComputeBounds(t *testing.T) {
	t.Run("cumulative windows", func(t *testing.T) {
		got := computeBounds([]float64{8, 11, 5})
		want := []sceneBound{{0, 8}, {8, 19}, {19, 24}}
		if len(got) != len(want) {
			t.Fatalf("len = %d, want %d", len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("bound %d = %v, want %v", i, got[i], want[i])
			}
		}
	})
	t.Run("nil and empty", func(t *testing.T) {
		if got := computeBounds(nil); got != nil {
			t.Errorf("computeBounds(nil) = %v, want nil", got)
		}
		if got := computeBounds([]float64{}); got != nil {
			t.Errorf("computeBounds([]) = %v, want nil", got)
		}
	})
	t.Run("single", func(t *testing.T) {
		got := computeBounds([]float64{12.5})
		if len(got) != 1 || got[0] != (sceneBound{0, 12.5}) {
			t.Errorf("got %v, want [{0 12.5}]", got)
		}
	})
}

// TestWavDurationSeconds round-trips a known PCM payload through wrapPCMAsWAV
// (24 kHz, 16-bit, mono) and asserts wavDurationSeconds recovers its length.
func TestWavDurationSeconds(t *testing.T) {
	// 24000 samples * 2 bytes = 48000 bytes = exactly 1.0 s of audio.
	pcm := make([]byte, 24000*2)
	wav := wrapPCMAsWAV(pcm, 24000, 1, 16)
	dir := t.TempDir()
	path := filepath.Join(dir, "one-second.wav")
	if err := os.WriteFile(path, wav, 0o644); err != nil {
		t.Fatalf("write wav: %v", err)
	}
	dur, err := wavDurationSeconds(path)
	if err != nil {
		t.Fatalf("wavDurationSeconds: %v", err)
	}
	if dur < 0.99 || dur > 1.01 {
		t.Errorf("duration = %v s, want ~1.0 s", dur)
	}

	// readWAVPCM must return exactly the PCM payload (header stripped).
	got, err := readWAVPCM(path)
	if err != nil {
		t.Fatalf("readWAVPCM: %v", err)
	}
	if len(got) != len(pcm) {
		t.Errorf("readWAVPCM len = %d, want %d", len(got), len(pcm))
	}
}
```

> Note: `wrapPCMAsWAV` writes a 44-byte canonical header, so `wavDurationSeconds` (which reads bytes 40–43 as the data-chunk size) recovers the exact length. If `t.TempDir()` is blocked by the sandbox, run the test with the sandbox disabled.

- [ ] **Step 3: Run the timing + caption tests (now both compile)**

```bash
go test ./internal/producer/ -run 'TestComputeBounds|TestWavDurationSeconds|TestCaptionSegmentsFromScenes' -v
```
Expected: PASS (3 ComputeBounds subtests + WavDuration + 3 CaptionSegments tests).

- [ ] **Step 4: Commit Tasks 1 + 2 together**

```bash
git add internal/producer/captions.go internal/producer/captions_test.go internal/producer/scene_timing.go internal/producer/scene_timing_test.go
git commit -m "feat(producer): caption segments + scene-timing/WAV helpers (from GeneratedScene)"
```

---

## Task 3: GeneratedScene → SceneSpec adapter (`scene_adapter.go`)

The one genuinely new piece. TDD: write the adapter test first. Maps each `agent.GeneratedScene` + its measured bound into a render-ready `SceneSpec` with a single headline slot, normalized layout/caption, brand accent, and image/css background mode. Index-matched with `bounds`; the shorter slice wins (never panics on a short LLM response).

**Files:**
- Create: `internal/producer/scene_adapter.go`
- Test: `internal/producer/scene_adapter_test.go`

- [ ] **Step 1: Write the failing adapter test**

Create `internal/producer/scene_adapter_test.go`:

```go
package producer

import (
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

func TestBuildSceneSpecs_MapsFieldsAndTiming(t *testing.T) {
	scenes := []agent.GeneratedScene{
		{SceneNumber: 1, LayoutVariant: "hook_big", OnScreenText: "บัญชีโดนแบน",
			EmphasisWords: []string{"แบน"}, CaptionStyle: "word_pop", ImagePrompt: "a banned ad account"},
		{SceneNumber: 2, LayoutVariant: "quote_cta", OnScreenText: "ทักแอดส์แวนซ์",
			CaptionStyle: "phrase_block", ImagePrompt: ""},
	}
	bounds := []sceneBound{{Start: 0, End: 8}, {Start: 8, End: 20}}

	specs := buildSceneSpecs(scenes, bounds)
	if len(specs) != 2 {
		t.Fatalf("len = %d, want 2", len(specs))
	}

	s0 := specs[0]
	if s0.SceneNumber != 1 || s0.LayoutVariant != "hook_big" || s0.CaptionStyle != "word_pop" {
		t.Errorf("scene 0 fields wrong: %+v", s0)
	}
	if s0.StartSec != 0 || s0.EndSec != 8 {
		t.Errorf("scene 0 timing = [%v,%v], want [0,8]", s0.StartSec, s0.EndSec)
	}
	if s0.AccentColor != Brand.Orange {
		t.Errorf("scene 0 accent = %q, want %q", s0.AccentColor, Brand.Orange)
	}
	if s0.AnimationSpeed != "normal" {
		t.Errorf("scene 0 speed = %q, want normal", s0.AnimationSpeed)
	}
	if s0.BackgroundMode != "image" { // has image_prompt
		t.Errorf("scene 0 bgMode = %q, want image", s0.BackgroundMode)
	}
	if len(s0.Slots) != 1 || s0.Slots[0].Role != "headline" {
		t.Fatalf("scene 0 slots = %+v, want one headline", s0.Slots)
	}
	if !strings.Contains(string(s0.Slots[0].HTML), `<span class="hl">แบน</span>`) {
		t.Errorf("scene 0 headline missing emphasis span: %q", s0.Slots[0].HTML)
	}

	if specs[1].BackgroundMode != "css" { // empty image_prompt
		t.Errorf("scene 1 bgMode = %q, want css", specs[1].BackgroundMode)
	}
}

func TestBuildSceneSpecs_NormalizesLayoutAndCaption(t *testing.T) {
	// SceneAgent enum values that are NOT template scene-layouts must fall back to hook_big.
	cases := map[string]string{
		"hook_big": "hook_big", "hook_punch": "hook_punch", "list_steps": "list_steps",
		"stat_reveal": "stat_reveal", "compare_two": "compare_two", "quote_cta": "quote_cta",
		"phrase_block": "hook_big", "word_pop": "hook_big", "static": "hook_big",
		"intro": "hook_big", "outro": "hook_big", "": "hook_big", "garbage": "hook_big",
	}
	for in, want := range cases {
		specs := buildSceneSpecs(
			[]agent.GeneratedScene{{SceneNumber: 1, LayoutVariant: in, OnScreenText: "x", CaptionStyle: "weird"}},
			[]sceneBound{{0, 5}},
		)
		if specs[0].LayoutVariant != want {
			t.Errorf("layout %q normalized to %q, want %q", in, specs[0].LayoutVariant, want)
		}
		if specs[0].CaptionStyle != "phrase_block" { // "weird" → default
			t.Errorf("caption %q not clamped to phrase_block", specs[0].CaptionStyle)
		}
	}
}

func TestBuildSceneSpecs_LengthMismatchAndEmpty(t *testing.T) {
	if got := buildSceneSpecs(nil, nil); got != nil {
		t.Errorf("empty input = %v, want nil", got)
	}
	// More scenes than bounds: only min(len) produced, no panic.
	scenes := []agent.GeneratedScene{{SceneNumber: 1, OnScreenText: "a"}, {SceneNumber: 2, OnScreenText: "b"}}
	specs := buildSceneSpecs(scenes, []sceneBound{{0, 5}})
	if len(specs) != 1 {
		t.Errorf("len = %d, want 1 (min of 2 scenes, 1 bound)", len(specs))
	}
}

func TestBuildSceneSpecs_EmptyOnScreenTextYieldsNoSlot(t *testing.T) {
	specs := buildSceneSpecs(
		[]agent.GeneratedScene{{SceneNumber: 1, OnScreenText: "  ", LayoutVariant: "hook_big"}},
		[]sceneBound{{0, 5}},
	)
	if len(specs) != 1 || len(specs[0].Slots) != 0 {
		t.Errorf("blank on_screen_text should yield 0 slots, got %+v", specs[0].Slots)
	}
}
```

- [ ] **Step 2: Run the test — it fails (buildSceneSpecs undefined)**

```bash
go test ./internal/producer/ -run TestBuildSceneSpecs -v
```
Expected: compile error / FAIL — `buildSceneSpecs` not defined.

- [ ] **Step 3: Implement `scene_adapter.go`**

Create `internal/producer/scene_adapter.go`:

```go
package producer

import (
	"strings"

	"github.com/jaochai/video-fb/internal/agent"
)

// templateSceneLayouts is the set of layout_variant values the multi-scene
// template (layout_multi_scene.html.tmpl) actually styles as scene layouts.
var templateSceneLayouts = map[string]bool{
	"hook_big": true, "hook_punch": true, "list_steps": true,
	"stat_reveal": true, "compare_two": true, "quote_cta": true,
}

// normalizeLayout maps a SceneAgent layout_variant to a template-supported scene
// layout. The SceneAgent's seeded enum also includes caption styles
// (phrase_block, word_pop) and bumper names (static, intro, outro) that are not
// scene layouts; those — and any unknown value — fall back to hook_big (the
// template's default centered column).
func normalizeLayout(v string) string {
	if templateSceneLayouts[v] {
		return v
	}
	return "hook_big"
}

// normalizeCaptionStyle clamps to the two styles the template's caption driver
// understands; anything else becomes phrase_block.
func normalizeCaptionStyle(s string) string {
	if s == "word_pop" {
		return "word_pop"
	}
	return "phrase_block"
}

// buildSceneSpecs maps the SceneAgent's GeneratedScene[] plus the measured
// per-scene audio bounds into render-ready []SceneSpec for the multi-scene
// template. Each scene becomes a single headline slot built from on_screen_text
// + emphasis_words (the SceneAgent emits one on-screen line per scene), with the
// layout/caption normalized, the brand accent, and an image/css background mode
// keyed off whether the scene has an image_prompt.
//
// scenes and bounds are index-matched; the shorter slice wins so a short LLM
// response never panics. Returns nil when either is empty.
func buildSceneSpecs(scenes []agent.GeneratedScene, bounds []sceneBound) []SceneSpec {
	n := len(scenes)
	if nb := len(bounds); nb < n {
		n = nb
	}
	if n == 0 {
		return nil
	}

	specs := make([]SceneSpec, n)
	for i := 0; i < n; i++ {
		s := scenes[i]
		b := bounds[i]

		bgMode := "css"
		if strings.TrimSpace(s.ImagePrompt) != "" {
			bgMode = "image"
		}

		var slots []SlotSpec
		if txt := strings.TrimSpace(s.OnScreenText); txt != "" {
			slots = []SlotSpec{{Role: "headline", HTML: highlightTitle(txt, s.EmphasisWords)}}
		}

		specs[i] = SceneSpec{
			SceneNumber:    s.SceneNumber,
			LayoutVariant:  normalizeLayout(s.LayoutVariant),
			AccentColor:    Brand.Orange,
			AnimationSpeed: "normal",
			StartSec:       b.Start,
			EndSec:         b.End,
			BackgroundMode: bgMode,
			CaptionStyle:   normalizeCaptionStyle(s.CaptionStyle),
			Slots:          slots,
		}
	}
	return specs
}
```

- [ ] **Step 4: Run the adapter tests — they pass**

```bash
go test ./internal/producer/ -run TestBuildSceneSpecs -v
```
Expected: PASS (4 tests). (`highlightTitle` from Plan 2b-3 wraps `แบน` in `<span class="hl">`; `Brand.Orange` is `#F0A030`.)

- [ ] **Step 5: Commit**

```bash
git add internal/producer/scene_adapter.go internal/producer/scene_adapter_test.go
git commit -m "feat(producer): GeneratedScene→SceneSpec adapter (single headline slot, layout normalization)"
```

---

## Task 4: Per-scene voice synthesis (`scene_voice.go`)

Port `synthScenesVoice` verbatim — it already takes `[]agent.GeneratedScene`, TTSes each scene's `VoiceText` via `p.openRouter.GenerateVoice`, measures each WAV, concatenates the PCM into one `voice.wav`, and returns the path + per-scene `[start,end)` bounds. It uses `wrapPCMAsWAV` (openrouter.go), `readWAVPCM`/`wavDurationSeconds` (Task 2). No automated unit test (it calls the live TTS endpoint); it is exercised by the Task 5 gated smoke and its helpers are already unit-tested.

**Files:**
- Create: `internal/producer/scene_voice.go`

- [ ] **Step 1: Create `scene_voice.go` with `synthScenesVoice` (extracted verbatim from REF's `multiscene.go`)**

```bash
# Extract the multiscene.go blob to inspect, then copy ONLY the synthScenesVoice method.
git show redesign/hyperframes-video-engine:internal/producer/multiscene.go > /tmp/ref_multiscene.go  # if /tmp blocked, use ./.tmp
```

Create `internal/producer/scene_voice.go` with exactly this content (the `synthScenesVoice` method verbatim from the branch, with the focused import set):

```go
package producer

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/jaochai/video-fb/internal/agent"
)

// synthScenesVoice TTSes each scene's VoiceText, concatenates them into one
// voice.wav at clipDir/voice.wav, and returns its path plus per-scene [start,end)
// boundaries on the combined timeline.
func (p *Producer) synthScenesVoice(ctx context.Context, scenes []agent.GeneratedScene, voice, clipDir string) (string, []sceneBound, error) {
	if len(scenes) == 0 {
		return "", nil, fmt.Errorf("no scenes")
	}

	// TTS each scene into a temporary per-scene WAV.
	tmpPaths := make([]string, len(scenes))
	for i, scene := range scenes {
		tmpPath := filepath.Join(clipDir, fmt.Sprintf("voice-scene%d.wav", i+1))
		tmpPaths[i] = tmpPath

		if scene.VoiceText == "" {
			// Write a zero-duration placeholder (minimal valid WAV with no PCM).
			wavData := wrapPCMAsWAV(nil, 24000, 1, 16)
			if err := os.WriteFile(tmpPath, wavData, 0644); err != nil {
				return "", nil, fmt.Errorf("write silent wav for scene %d: %w", i+1, err)
			}
			log.Printf("synthScenesVoice: scene %d has empty VoiceText — writing silent placeholder", i+1)
			continue
		}

		log.Printf("synthScenesVoice: TTS scene %d/%d (%d chars)", i+1, len(scenes), len([]rune(scene.VoiceText)))
		if err := p.openRouter.GenerateVoice(ctx, scene.VoiceText, voice, tmpPath); err != nil {
			return "", nil, fmt.Errorf("TTS scene %d: %w", i+1, err)
		}
	}

	// Measure per-scene durations and concatenate all PCM.
	durations := make([]float64, len(scenes))
	var allPCM []byte
	for i, tmpPath := range tmpPaths {
		dur, err := wavDurationSeconds(tmpPath)
		if err != nil {
			return "", nil, fmt.Errorf("measure duration scene %d: %w", i+1, err)
		}
		durations[i] = dur

		pcm, err := readWAVPCM(tmpPath)
		if err != nil {
			return "", nil, fmt.Errorf("read PCM scene %d: %w", i+1, err)
		}
		allPCM = append(allPCM, pcm...)
	}

	// Write the combined WAV.
	outPath := filepath.Join(clipDir, "voice.wav")
	const sampleRate = 24000
	wavData := wrapPCMAsWAV(allPCM, sampleRate, 1, 16)
	if err := os.MkdirAll(clipDir, 0755); err != nil {
		return "", nil, fmt.Errorf("create clipDir: %w", err)
	}
	if err := os.WriteFile(outPath, wavData, 0644); err != nil {
		return "", nil, fmt.Errorf("write combined voice.wav: %w", err)
	}

	bounds := computeBounds(durations)

	total := 0.0
	for _, d := range durations {
		total += d
	}
	log.Printf("synthScenesVoice: %d scenes, total %.1fs → %s", len(scenes), total, outPath)
	for i, b := range bounds {
		log.Printf("  scene %d: [%.2fs, %.2fs) (%.2fs)", i+1, b.Start, b.End, durations[i])
	}

	// Clean up per-scene temp WAVs.
	for _, tmpPath := range tmpPaths {
		os.Remove(tmpPath)
	}

	return outPath, bounds, nil
}
```

- [ ] **Step 2: Confirm the package compiles**

```bash
go build ./internal/producer/
```
Expected: builds clean (uses `wrapPCMAsWAV`, `readWAVPCM`, `wavDurationSeconds`, `computeBounds`, `p.openRouter`).

- [ ] **Step 3: Commit**

```bash
git add internal/producer/scene_voice.go
git commit -m "feat(producer): per-scene TTS synthesis (synthScenesVoice → measured bounds)"
```

---

## Task 5: Wire the engine into Producer (`AssembleHyperframes916`)

Add the multi-scene assembly method and the additive plumbing to reach the Plan-2b-3 engine. This glues already-tested pieces: `synthScenesVoice` → per-scene `kie.GenerateImage` → `buildSceneSpecs` → `captionSegmentsFromScenes` → `BuildScenes` → `Render`.

**Files:**
- Modify: `internal/producer/producer.go`
- Test: `internal/producer/producer_hyperframes_test.go`

- [ ] **Step 1: Add the `hf` field to the `Producer` struct**

In `internal/producer/producer.go`, add one field to the struct (leave `NewProducer` unchanged — the field defaults to `nil`):

```go
type Producer struct {
	pool         *pgxpool.Pool
	kie          *KieClient
	openRouter   *OpenRouterClient
	ffmpeg       *FFmpegAssembler
	defaultVoice string
	workDir      string
	tracker      *progress.Tracker
	hf           *hyperframesDeps // nil until EnableHyperframes; only the multi-scene path uses it
}
```

- [ ] **Step 2: Add the deps struct, the `EnableHyperframes` setter, and `AssembleHyperframes916`**

Append to `internal/producer/producer.go` (these reference `CompositionBuilder`, `HyperframesRenderer`, `buildScenePrompt`, `BrandName` from Plan 2b-3, and `synthScenesVoice`/`buildSceneSpecs`/`captionSegmentsFromScenes` from Tasks 1–4):

```go
// hyperframesDeps bundles the multi-scene render engine; set via EnableHyperframes.
type hyperframesDeps struct {
	builder  *CompositionBuilder
	renderer *HyperframesRenderer
}

// EnableHyperframes wires the multi-scene render engine into the producer.
// fontsDir holds the Sarabun .ttf files (internal/producer/assets/fonts).
// Additive: the static-image Produce path does not use p.hf.
func (p *Producer) EnableHyperframes(fontsDir string) {
	p.hf = &hyperframesDeps{
		builder:  NewCompositionBuilder(fontsDir),
		renderer: NewHyperframesRenderer(),
	}
}

// AssembleHyperframes916 turns SceneAgent scenes into a 9:16 multi-scene MP4 via
// the hyperframes engine and returns the LOCAL output.mp4 path. Steps: per-scene
// TTS (measured) → per-scene gpt-image-2 backgrounds (kie.ai; missing/failed →
// css) → GeneratedScene→SceneSpec → fill the multi-scene template → render.
// Upload / thumbnail / clip-status are the caller's job (orchestrator).
// Requires EnableHyperframes to have been called.
func (p *Producer) AssembleHyperframes916(ctx context.Context, clipID string, scenes []agent.GeneratedScene) (string, error) {
	if p.hf == nil {
		return "", fmt.Errorf("hyperframes not enabled (call EnableHyperframes)")
	}
	if len(scenes) == 0 {
		return "", fmt.Errorf("no scenes")
	}

	clipDir := filepath.Join(p.workDir, clipID)
	if err := os.MkdirAll(clipDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir clipDir: %w", err)
	}
	voice := p.getVoice(ctx)

	// 1) per-scene TTS → combined voice.wav + measured [start,end) bounds.
	voicePath, bounds, err := p.synthScenesVoice(ctx, scenes, voice, clipDir)
	if err != nil {
		return "", fmt.Errorf("synth scenes voice: %w", err)
	}

	// 2) per-scene gpt-image-2 backgrounds (kie.ai). A missing/failed image is
	//    non-fatal — BuildScenes downgrades that scene to a css background.
	bgPaths := map[int]string{}
	for _, s := range scenes {
		if strings.TrimSpace(s.ImagePrompt) == "" {
			continue
		}
		bgFile := filepath.Join(clipDir, fmt.Sprintf("bg-scene%d.png", s.SceneNumber))
		if !fileExists(bgFile) {
			prompt := buildScenePrompt(s.ImagePrompt, "9:16")
			if genErr := p.kie.GenerateImage(ctx, prompt, "9:16", bgFile); genErr != nil {
				log.Printf("AssembleHyperframes916: scene %d image gen failed, using css: %v", s.SceneNumber, genErr)
				continue
			}
		}
		if fileExists(bgFile) {
			bgPaths[s.SceneNumber] = bgFile
		}
	}

	// 3) map scenes → SceneSpec, build captions, assemble ScenesParams.
	specs := buildSceneSpecs(scenes, bounds)
	if len(specs) == 0 {
		return "", fmt.Errorf("buildSceneSpecs returned empty (scenes=%d bounds=%d)", len(scenes), len(bounds))
	}
	segments := captionSegmentsFromScenes(scenes, bounds)
	total := 0.0
	if len(bounds) > 0 {
		total = bounds[len(bounds)-1].End
	}
	params := ScenesParams{
		AspectRatio:     "9:16",
		BrandName:       BrandName,
		CTAText:         BrandCTA,
		VoiceSrc:        "assets/voice.wav",
		DurationSeconds: total,
		Scenes:          specs,
		Segments:        segments,
	}

	// 4) build the project dir and render the MP4.
	projectDir := filepath.Join(clipDir, "composition-916")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir projectDir: %w", err)
	}
	if _, err := p.hf.builder.BuildScenes(params, clipID, projectDir, voicePath, bgPaths); err != nil {
		return "", fmt.Errorf("build scenes: %w", err)
	}
	if err := p.hf.renderer.Render(ctx, projectDir, "output.mp4"); err != nil {
		return "", fmt.Errorf("render: %w", err)
	}
	return filepath.Join(projectDir, "output.mp4"), nil
}
```

Then ensure `producer.go`'s import block includes everything used (it already imports `context`, `fmt`, `log`, `os`, `path/filepath`, `strings`, `agent`, `progress`, `pgxpool` for the existing code — confirm `strings` and `path/filepath` are present; add if `go build` flags them missing).

- [ ] **Step 3: Confirm the additive change compiles and the whole package still builds**

```bash
go build ./... && go vet ./internal/producer/
```
Expected: clean. `main.go` is unchanged (still `NewProducer(...)` + `Produce(...)`); the new method is unreferenced outside tests — fine for an exported method.

- [ ] **Step 4: Add a render-gated end-to-end smoke test**

Create `internal/producer/producer_hyperframes_test.go`. This is the only test that needs the full toolchain; it self-skips unless `HF_RENDER=1`, and additionally needs a real DB + live API keys, so it is a manual smoke, not a CI gate:

```go
package producer

import (
	"context"
	"os"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

// TestAssembleHyperframes916_Smoke is a MANUAL end-to-end check. It needs
// HF_RENDER=1 plus a real DATABASE_URL (for the kie/openrouter API keys in the
// settings table) and Node+Chrome on PATH. It produces a real MP4. CI skips it.
func TestAssembleHyperframes916_Smoke(t *testing.T) {
	if os.Getenv("HF_RENDER") != "1" {
		t.Skip("set HF_RENDER=1 (and DATABASE_URL + Node/Chrome) to run the end-to-end smoke")
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL required for the end-to-end smoke (kie/openrouter keys live in settings)")
	}

	// Construct a real Producer against the DB and enable the engine.
	p, cleanup := newSmokeProducer(t, dbURL) // helper: opens pool, NewKieClient/NewOpenRouterClient, NewProducer, EnableHyperframes("assets/fonts")
	defer cleanup()

	scenes := []agent.GeneratedScene{
		{SceneNumber: 1, LayoutVariant: "hook_big", VoiceText: "บัญชีโฆษณาโดนแบนถาวรเพราะอะไร",
			OnScreenText: "บัญชีโดนแบน", EmphasisWords: []string{"แบน"}, CaptionStyle: "word_pop", ImagePrompt: ""},
		{SceneNumber: 2, LayoutVariant: "quote_cta", VoiceText: "อย่ารอให้สาย ทักแอดส์แวนซ์ได้เลย",
			OnScreenText: "ทักแอดส์แวนซ์", CaptionStyle: "phrase_block", ImagePrompt: ""},
	}

	out, err := p.AssembleHyperframes916(context.Background(), "smoke-clip", scenes)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	fi, err := os.Stat(out)
	if err != nil || fi.Size() < 10_000 {
		t.Fatalf("expected a non-trivial MP4 at %s (size=%d, err=%v)", out, fi.Size(), err)
	}
	t.Logf("rendered %s (%d bytes)", out, fi.Size())
}
```

Write the `newSmokeProducer(t, dbURL)` helper in the same test file: open a `pgxpool` from `dbURL`, build `NewKieClient(pool, DefaultKieConfig())`, `NewOpenRouterClient(pool)`, `NewFFmpegAssembler(...)` (path can be `"ffmpeg"` — unused by the hyperframes path), `NewProducer(pool, kie, or, ffmpeg, "", t.TempDir(), nil)`, call `p.EnableHyperframes("assets/fonts")`, and return a cleanup that closes the pool. Use `t.TempDir()` as the workDir so the smoke writes under a temp dir.

> **Honesty note:** with no API keys / Node / Chrome (e.g. the sandbox), this test is un-runnable and correctly SKIPs — the binding end-to-end MP4 proof happens on a real machine or the Plan-2b-6 Docker image. Do NOT claim a render that wasn't run. The CI-meaningful coverage for this task is `go build` + the pure adapter/caption/timing tests from Tasks 1–4.

- [ ] **Step 5: Verify it self-skips under normal test, and the package is green**

```bash
go test ./internal/producer/ -run TestAssembleHyperframes916_Smoke -v   # expect SKIP
go test ./internal/producer/ -v                                          # expect PASS, smoke SKIPPED
```
Expected: smoke SKIPs; all pure tests pass.

- [ ] **Step 6: (Optional, real machine) run the smoke**

```bash
cd internal/producer && HF_RENDER=1 DATABASE_URL=<neon-url> go test ./ -run TestAssembleHyperframes916_Smoke -v ; cd -
```
Expected (only with keys+Node+Chrome): a non-trivial `output.mp4` is produced. Report honestly whether it ran.

- [ ] **Step 7: Commit**

```bash
git add internal/producer/producer.go internal/producer/producer_hyperframes_test.go
git commit -m "feat(producer): AssembleHyperframes916 — scenes to 9:16 MP4 via hyperframes engine"
```

---

## Task 6: Full verification (additive contract)

**Files:** none (verification only)

- [ ] **Step 1: Build, vet, and test the whole module**

```bash
go build ./... && go vet ./... && go test ./...
```
Expected: all PASS; `TestAssembleHyperframes916_Smoke` SKIPPED.

- [ ] **Step 2: Confirm the existing static path is untouched**

```bash
git diff d9cdeae..HEAD -- internal/producer/producer.go | grep -E '^[-+]' | grep -vE '^[-+]{3}' | grep -E '^-' || echo "✓ no lines removed from producer.go — purely additive"
```
Expected: `✓ no lines removed from producer.go` (the change only ADDS the `hf` field, `hyperframesDeps`, `EnableHyperframes`, `AssembleHyperframes916`; `Produce`/`NewProducer`/`getVoice` are unchanged). If any `-` line appears, confirm it is only the struct-field insertion context, not a change to existing logic.

- [ ] **Step 3: Confirm main.go / orchestrator were NOT touched**

```bash
git diff d9cdeae..HEAD --name-only | grep -E 'cmd/server/main.go|internal/orchestrator|internal/agent|frontend|migrations' && echo "UNEXPECTED — investigate" || echo "✓ only internal/producer touched"
```
Expected: `✓ only internal/producer touched`.

- [ ] **Step 4: Confirm the new method is reachable but unwired**

```bash
grep -rn 'AssembleHyperframes916\|EnableHyperframes' --include=*.go . | grep -v '_test.go' | grep -v 'internal/producer/producer.go'
```
Expected: no output (the method is defined in producer.go and called only by the gated test — the orchestrator wires it in Plan 2b-5).

---

## Self-Review Notes

- **Spec coverage:** Implements design §4.4 (Producer: per-scene TTS + measured timing + per-scene gpt-image-2 + render) and the image half of §2 (gpt-image-2 via kie.ai). Captions-from-narration (§4.4.d) and the GeneratedScene→render mapping are covered. Upload/thumbnail (§4.4 g/h) and the resume-from-failure refinement are deferred to Plan 2b-5 wiring (the method returns the local MP4; `fileExists` guards already make image gen resumable within a clipDir). Orchestrator (§4.5), agent renames/frontend (§4.6/§4.8), Dockerfile (§6) are later plans.
- **Reuse over rewrite (user's decision):** caption/timing/voice helpers port verbatim from `redesign/hyperframes-video-engine` (they already consume `agent.GeneratedScene`); only `buildSceneSpecs` is rewritten because that branch sourced its design from a `CompositionAgent`/`SceneDesign` we dropped — the current `SceneAgent`/`GeneratedScene` is the single source now.
- **Green at every step:** Tasks 1+2 commit together (captions.go needs sceneBound). Producer change is strictly additive (Task 6 Step 2 guard). `main.go` untouched → master builds and the old static path still runs. The only non-pure test self-skips.
- **Known limitation (surfaced, not hidden):** the single-headline-slot mapping means the template's multi-slot layouts (list_steps/stat_reveal/compare_two) render as a headline only, and the SceneAgent's non-layout enum values normalize to hook_big. Producing richer slots needs a SceneAgent JSON-contract expansion — a deliberate future step, not silently assumed away.
- **No placeholders:** every new file's full content is inline (adapter, voice, timing, producer additions, both tests); verbatim ports are named git-show extractions with a verbatim-diff check; every step ends in a runnable command with expected output.
- **Honesty flag:** the pure tests prove the mapping/caption/timing logic; they do NOT prove a real MP4 renders end-to-end. That needs TTS + kie image + Chrome (gated `HF_RENDER=1` + DB). The binding §10.1 success criterion is met when Plan 2b-5 wires this into the orchestrator and it runs on a real machine / the Plan-2b-6 Docker image. Report any smoke run honestly; never claim an unrun render.

---

## Execution Handoff

**Plan complete and saved to `docs/superpowers/plans/2026-06-10-producer-rewrite-plan2b4.md`. Two execution options:**

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints.

**Which approach?**
