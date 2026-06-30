# Audio + Motion Upgrade Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add scene-transition sound effects, a quiet looping ambient music bed, and a safe GSAP motion upgrade to the live hyperframes (9:16) video pipeline — all gated behind a feature flag, with no changes to the content-generation agents.

**Architecture:** This is a render-template + asset-library change in `internal/producer`, mirroring the Style Presets pattern. Audio assets are embedded via `go:embed`. The producer picks an ambient bed (random avoid-last, persisted in `settings.last_ambient`) and transition SFX, prepares a duration-matched ambient track with ffmpeg, and the `CompositionBuilder` copies the chosen audio into each per-clip Hyperframes project. The template renders extra `<audio>` tracks (reusing the engine's existing `data-track-index` multi-track support) and applies upgraded transitions. Everything is no-op when `AUDIO_MOTION_ENABLED` is unset.

**Tech Stack:** Go 1.x, `html/template`, `go:embed`, ffmpeg (via existing `FFmpegAssembler`), GSAP 3.14 (vendored), Hyperframes CLI 0.6.70 (pinned).

## Global Constraints

- Feature flag: `AUDIO_MOTION_ENABLED` env var; the whole feature is a no-op unless it equals exactly `"true"`. Read pattern mirrors `producer.StylePresetsEnabled()` (`internal/producer/presets.go:109`).
- When the flag is off, rendered output behavior MUST be identical to today (voice-only, current motion).
- No changes to `internal/agent/*` (scene/content generation untouched).
- Live pipeline is `ProduceHyperframes916` → `AssembleHyperframes916` (`internal/producer/producer.go`). The static `Produce` / `Assemble*` paths are legacy and out of scope.
- Hyperframes render JS must stay deterministic — no `Math.random()`/`Date.now()` in template JS (the lint gate `non_deterministic_code` falls a render back to a static image). All randomness happens server-side in Go.
- Ambient volume under voice: `data-volume="0.15"`. Voice stays `data-volume="1"`.
- Audio track indices must not collide with existing ones: root composition, badges=2, voice=5, progress=7, captions=9, scenes=10+i. Use **ambient=3**, **transition SFX=4**.
- **Every `<audio>` element MUST have a unique `id`** — verified in the Task 0 spike: the Hyperframes renderer discovers media by `id`, and an audio element without one renders SILENT (lint `media_missing_id`). Ambient uses `id="amb"`; each JS-created SFX element uses `id="sfx<index>"`.
- Commit after every task. Run `gofmt`/`go build ./...` before each commit.

---

## Task 0: Spike — verify the Hyperframes CLI mixes multiple audio tracks

**This is a decision gate.** The whole template-track approach assumes the pinned Hyperframes CLI mixes more than one `<audio>` element into the rendered MP4. Only the single voice track is exercised today. Verify before building anything else.

**Files:**
- Create (throwaway): `/tmp/hf-audio-spike/` project dir (do NOT commit)

**Steps:**

- [ ] **Step 1: Generate three short test audio files with ffmpeg**

```bash
mkdir -p /tmp/hf-audio-spike/assets
cd /tmp/hf-audio-spike
# voice-like tone (3s, 440Hz)
ffmpeg -f lavfi -i "sine=frequency=440:duration=3" -ar 44100 -y assets/voice.wav
# ambient bed (3s, low 110Hz)
ffmpeg -f lavfi -i "sine=frequency=110:duration=3" -ar 44100 -y assets/amb.mp3
# sfx blip (0.4s, 880Hz)
ffmpeg -f lavfi -i "sine=frequency=880:duration=0.4" -ar 44100 -y assets/sfx.mp3
```

- [ ] **Step 2: Hand-write a minimal Hyperframes project with 3 audio tracks**

Create `/tmp/hf-audio-spike/index.html`:

```html
<!doctype html><html><head><meta charset="utf-8"></head><body>
<div id="root" data-composition-id="main" data-start="0" data-duration="3" data-width="1080" data-height="1920">
  <div class="clip" data-start="0" data-duration="3" data-track-index="1" style="width:1080px;height:1920px;background:#123"></div>
  <audio data-start="0" data-duration="3" data-track-index="5" src="assets/voice.wav" data-volume="1"></audio>
  <audio data-start="0" data-duration="3" data-track-index="3" src="assets/amb.mp3" data-volume="0.3"></audio>
  <audio data-start="1.0" data-duration="0.4" data-track-index="4" src="assets/sfx.mp3" data-volume="0.8"></audio>
</div></body></html>
```

Create `/tmp/hf-audio-spike/package.json`:

```json
{ "name": "spike", "private": true, "type": "module",
  "scripts": { "render": "npx --yes hyperframes@0.6.70 render" } }
```

Create `/tmp/hf-audio-spike/hyperframes.json`:

```json
{ "$schema": "https://hyperframes.heygen.com/schema/hyperframes.json",
  "paths": { "blocks": "compositions", "components": "compositions/components", "assets": "assets" } }
```

- [ ] **Step 3: Render**

Run: `cd /tmp/hf-audio-spike && npx --yes hyperframes@0.6.70 render --output out.mp4 --quality standard --fps 24 -w 1`
Expected: `out.mp4` is produced (exit 0).

- [ ] **Step 4: Inspect the mixed audio**

Run: `ffprobe -v error -show_streams -select_streams a out.mp4` then
`ffmpeg -i out.mp4 -map 0:a -af "volumedetect" -f null - 2>&1 | grep -E "mean_volume|max_volume"`
Expected: exactly one audio stream whose levels are non-silent. Then sanity-check audibly that the bed + blip are present under the tone (open `out.mp4`).

- [ ] **Step 5: Record the verdict in the plan and decide**

- **If all three sources are audibly mixed → PASS.** Proceed to Task 1. Append a note "Task 0: PASS — engine mixes multi-track audio" to this file and commit only that note.
- **If only the voice plays (or render errors on extra audio) → FAIL.** STOP. Do not implement Tasks 4–7 as written. Report back: the fallback is to mix ambient + SFX with `ffmpeg amix` after the render in `AssembleHyperframes916` (the producer already wraps ffmpeg). Re-plan Tasks 3–7 around a post-render mux before continuing.

```bash
git commit --allow-empty -m "chore: Task 0 spike result — hyperframes multi-track audio [PASS|FAIL]"
```

---

## Task 1: Audio asset library + embed

**Files:**
- Create: `internal/producer/assets/audio/ambient/bed1.mp3`, `bed2.mp3`, `bed3.mp3`
- Create: `internal/producer/assets/audio/sfx/transition/whoosh1.mp3`, `whoosh2.mp3`, `swish1.mp3`
- Create: `internal/producer/audio_assets.go`
- Create: `internal/producer/audio_assets_test.go`

> **Note on asset quality:** The ffmpeg commands below synthesize simple, royalty-free placeholder audio so the pipeline is fully testable and shippable today. They are intentionally generic. A human can later drop curated royalty-free `.mp3` files into the same folders (same names or new names) with zero code changes — selection reads whatever is present.

- [ ] **Step 1: Generate the ambient beds (≈40s, loopable tones)**

```bash
mkdir -p internal/producer/assets/audio/ambient internal/producer/assets/audio/sfx/transition
# bed1: warm low pad
ffmpeg -f lavfi -i "sine=frequency=110:duration=40" -f lavfi -i "sine=frequency=164.81:duration=40" \
  -filter_complex "amix=inputs=2,tremolo=f=0.15:d=0.25,afade=t=in:d=2,afade=t=out:st=38:d=2,volume=0.6" \
  -ar 44100 -b:a 128k -y internal/producer/assets/audio/ambient/bed1.mp3
# bed2: brighter pad
ffmpeg -f lavfi -i "sine=frequency=146.83:duration=40" -f lavfi -i "sine=frequency=220:duration=40" \
  -filter_complex "amix=inputs=2,tremolo=f=0.2:d=0.3,afade=t=in:d=2,afade=t=out:st=38:d=2,volume=0.6" \
  -ar 44100 -b:a 128k -y internal/producer/assets/audio/ambient/bed2.mp3
# bed3: deep cinematic
ffmpeg -f lavfi -i "sine=frequency=98:duration=40" -f lavfi -i "sine=frequency=130.81:duration=40" \
  -filter_complex "amix=inputs=2,tremolo=f=0.1:d=0.2,afade=t=in:d=2,afade=t=out:st=38:d=2,volume=0.6" \
  -ar 44100 -b:a 128k -y internal/producer/assets/audio/ambient/bed3.mp3
```

- [ ] **Step 2: Generate the transition SFX (short whoosh/swish from filtered noise)**

```bash
ffmpeg -f lavfi -i "anoisesrc=d=0.6:c=pink:a=0.6" \
  -af "highpass=f=300,lowpass=f=6000,afade=t=in:d=0.25,afade=t=out:st=0.3:d=0.3,volume=0.8" \
  -ar 44100 -b:a 128k -y internal/producer/assets/audio/sfx/transition/whoosh1.mp3
ffmpeg -f lavfi -i "anoisesrc=d=0.5:c=white:a=0.5" \
  -af "highpass=f=800,lowpass=f=9000,afade=t=in:d=0.2,afade=t=out:st=0.25:d=0.25,volume=0.7" \
  -ar 44100 -b:a 128k -y internal/producer/assets/audio/sfx/transition/whoosh2.mp3
ffmpeg -f lavfi -i "anoisesrc=d=0.4:c=white:a=0.4" \
  -af "highpass=f=1500,lowpass=f=11000,afade=t=in:d=0.15,afade=t=out:st=0.2:d=0.2,volume=0.6" \
  -ar 44100 -b:a 128k -y internal/producer/assets/audio/sfx/transition/swish1.mp3
```

- [ ] **Step 3: Create the embed FS**

`internal/producer/audio_assets.go`:

```go
package producer

import "embed"

// audioAssetsFS holds the bundled royalty-free SFX and ambient beds. Embedding
// (rather than copying from disk like fonts) means selection + project assembly
// work from any working dir with no path resolution, matching the vendored GSAP.
//
//go:embed assets/audio
var audioAssetsFS embed.FS
```

- [ ] **Step 4: Write the failing test**

`internal/producer/audio_assets_test.go`:

```go
package producer

import (
	"io/fs"
	"strings"
	"testing"
)

func TestAudioAssetsEmbedded(t *testing.T) {
	var ambient, sfx int
	err := fs.WalkDir(audioAssetsFS, "assets/audio", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".mp3") {
			return err
		}
		switch {
		case strings.Contains(p, "/ambient/"):
			ambient++
		case strings.Contains(p, "/sfx/transition/"):
			sfx++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk embed: %v", err)
	}
	if ambient < 1 {
		t.Errorf("want >=1 ambient bed, got %d", ambient)
	}
	if sfx < 1 {
		t.Errorf("want >=1 transition sfx, got %d", sfx)
	}
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/producer/ -run TestAudioAssetsEmbedded -v`
Expected: PASS (3 ambient + 3 sfx embedded).

- [ ] **Step 6: Commit**

```bash
go build ./... && gofmt -l internal/producer/
git add internal/producer/assets/audio internal/producer/audio_assets.go internal/producer/audio_assets_test.go
git commit -m "feat(producer): embed royalty-free ambient + transition SFX asset library"
```

---

## Task 2: Audio selection logic (flag + avoid-last ambient + SFX picker)

**Files:**
- Create: `internal/producer/audio.go`
- Create: `internal/producer/audio_test.go`

**Interfaces:**
- Consumes: `audioAssetsFS` (Task 1).
- Produces (relied on by Tasks 4 & 7):
  - `func AudioMotionEnabled() bool`
  - `func AmbientFiles() []string` — base names of embedded ambient beds, sorted.
  - `func SfxTransitionFiles() []string` — base names of embedded transition SFX, sorted.
  - `func PickAmbient(lastName string, rng func(int) int) string` — avoid-last; returns base name or `""` if none embedded.
  - `func PickTransitionSFX(n int, rng func(int) int) []string` — `n` base names (with repeats allowed when `n` > pool size); empty slice when `n<=0` or none embedded.

- [ ] **Step 1: Write the failing tests**

`internal/producer/audio_test.go`:

```go
package producer

import "testing"

func TestAmbientAndSfxFilesNonEmpty(t *testing.T) {
	if len(AmbientFiles()) == 0 {
		t.Error("AmbientFiles empty")
	}
	if len(SfxTransitionFiles()) == 0 {
		t.Error("SfxTransitionFiles empty")
	}
}

func TestPickAmbientAvoidsLast(t *testing.T) {
	files := AmbientFiles()
	if len(files) < 2 {
		t.Skip("need >=2 ambient beds to test avoid-last")
	}
	last := files[0]
	// rng always returns 0 → would pick candidates[0]; candidates exclude `last`.
	got := PickAmbient(last, func(int) int { return 0 })
	if got == last {
		t.Errorf("PickAmbient returned the avoided last %q", got)
	}
}

func TestPickAmbientSingleOrEmpty(t *testing.T) {
	// With an unknown last and a forced index, it still returns a real file.
	if got := PickAmbient("", func(int) int { return 0 }); got == "" {
		t.Error("PickAmbient returned empty with assets present")
	}
}

func TestPickTransitionSFXCount(t *testing.T) {
	if got := PickTransitionSFX(0, func(int) int { return 0 }); len(got) != 0 {
		t.Errorf("n=0 want 0 cues, got %d", len(got))
	}
	got := PickTransitionSFX(5, func(int) int { return 0 })
	if len(got) != 5 {
		t.Errorf("n=5 want 5 cues, got %d", len(got))
	}
	for _, name := range got {
		if name == "" {
			t.Error("empty sfx name in result")
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/producer/ -run 'TestAmbient|TestPickAmbient|TestPickTransition' -v`
Expected: FAIL — `undefined: AmbientFiles` etc.

- [ ] **Step 3: Implement `audio.go`**

```go
package producer

import (
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"
)

const (
	ambientDir = "assets/audio/ambient"
	sfxDir     = "assets/audio/sfx/transition"
)

// AudioMotionEnabled reports whether the audio + motion upgrade is on. Off ⇒ the
// hyperframes path renders voice-only with today's motion (see presets.go pattern).
func AudioMotionEnabled() bool { return os.Getenv("AUDIO_MOTION_ENABLED") == "true" }

func listAudio(dir string) []string {
	entries, err := fs.ReadDir(audioAssetsFS, dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".mp3") {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

// AmbientFiles returns the embedded ambient bed base names, sorted.
func AmbientFiles() []string { return listAudio(ambientDir) }

// SfxTransitionFiles returns the embedded transition SFX base names, sorted.
func SfxTransitionFiles() []string { return listAudio(sfxDir) }

// PickAmbient chooses an ambient bed at random, excluding lastName when more than
// one exists so two clips in a row never share a bed. rng(n) returns an int in
// [0,n) — pass rand.Intn in production. Returns "" when no beds are embedded.
func PickAmbient(lastName string, rng func(int) int) string {
	files := AmbientFiles()
	if len(files) == 0 {
		return ""
	}
	if len(files) == 1 {
		return files[0]
	}
	candidates := make([]string, 0, len(files))
	for _, f := range files {
		if f != lastName {
			candidates = append(candidates, f)
		}
	}
	if len(candidates) == 0 {
		return files[0]
	}
	return candidates[rng(len(candidates))]
}

// PickTransitionSFX returns n SFX base names (one per scene boundary). When n
// exceeds the pool size, names repeat. Returns an empty slice for n<=0 or when no
// SFX are embedded.
func PickTransitionSFX(n int, rng func(int) int) []string {
	if n <= 0 {
		return nil
	}
	files := SfxTransitionFiles()
	if len(files) == 0 {
		return nil
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, files[rng(len(files))])
	}
	return out
}

var _ = path.Base // keep path import available for future relative-path helpers
```

> Remove the trailing `var _ = path.Base` line and the `"path"` import if `go build` flags them unused — they are only a guard against an accidental missing import while editing. Prefer deleting both.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/producer/ -run 'TestAmbient|TestPickAmbient|TestPickTransition' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
go build ./... && gofmt -w internal/producer/audio.go
git add internal/producer/audio.go internal/producer/audio_test.go
git commit -m "feat(producer): audio selection (flag, avoid-last ambient, sfx picker)"
```

---

## Task 3: ffmpeg helper to build a duration-matched ambient bed

The ambient bed (~40s) is shorter than a clip (~75s), so it must be looped, trimmed to the clip duration, and faded out. This produces a ready `ambient.mp3` in the clip dir (the same pattern as `voice.wav`).

**Files:**
- Modify: `internal/producer/ffmpeg.go` (add method)
- Create: `internal/producer/ffmpeg_ambient_test.go`

**Interfaces:**
- Produces (relied on by Task 7): `func (f *FFmpegAssembler) BuildAmbientBed(srcPath, outPath string, durationSec float64) error`

- [ ] **Step 1: Write the failing test (skips when ffmpeg is absent)**

`internal/producer/ffmpeg_ambient_test.go`:

```go
package producer

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBuildAmbientBed(t *testing.T) {
	ff, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "src.mp3")
	// 2s source tone.
	if out, err := exec.Command(ff, "-f", "lavfi", "-i", "sine=frequency=120:duration=2",
		"-ar", "44100", "-y", src).CombinedOutput(); err != nil {
		t.Fatalf("make src: %v\n%s", err, out)
	}
	a := NewFFmpegAssembler(ff, "")
	out := filepath.Join(dir, "ambient.mp3")
	if err := a.BuildAmbientBed(src, out, 6); err != nil {
		t.Fatalf("BuildAmbientBed: %v", err)
	}
	fi, err := os.Stat(out)
	if err != nil || fi.Size() == 0 {
		t.Fatalf("output missing/empty: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/producer/ -run TestBuildAmbientBed -v`
Expected: FAIL — `a.BuildAmbientBed undefined`.

- [ ] **Step 3: Implement the method in `ffmpeg.go`**

```go
// BuildAmbientBed loops srcPath to at least durationSec, trims to exactly
// durationSec, and applies a 1.5s tail fade so the bed ends cleanly under the
// outro. Output is an mp3 at outPath. Used for the per-clip background ambient.
func (f *FFmpegAssembler) BuildAmbientBed(srcPath, outPath string, durationSec float64) error {
	if durationSec <= 0 {
		return fmt.Errorf("durationSec must be > 0, got %v", durationSec)
	}
	os.MkdirAll(filepath.Dir(outPath), 0755)
	fadeStart := durationSec - 1.5
	if fadeStart < 0 {
		fadeStart = 0
	}
	args := []string{
		"-stream_loop", "-1", "-i", srcPath,
		"-t", fmt.Sprintf("%.3f", durationSec),
		"-af", fmt.Sprintf("afade=t=out:st=%.3f:d=1.5", fadeStart),
		"-ar", "44100", "-b:a", "128k",
		"-y", outPath,
	}
	cmd := exec.Command(f.ffmpegPath, args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg ambient bed failed: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/producer/ -run TestBuildAmbientBed -v`
Expected: PASS (or SKIP if ffmpeg missing locally — it is present on the render host).

- [ ] **Step 5: Commit**

```bash
go build ./... && gofmt -w internal/producer/ffmpeg.go
git add internal/producer/ffmpeg.go internal/producer/ffmpeg_ambient_test.go
git commit -m "feat(producer): ffmpeg BuildAmbientBed (loop+trim+fade to clip duration)"
```

---

## Task 4: Types + builder — copy audio into the project, set relative paths

**Files:**
- Modify: `internal/producer/composition_types.go` (add `TransitionCue`, extend `ScenesParams`)
- Modify: `internal/producer/composition.go` (extend `scenesTemplateData`, plumb new fields)
- Modify: `internal/producer/composition_builder.go` (`BuildScenes`: copy ambient + sfx)
- Modify: `internal/producer/composition_builder_test.go` (or create a new test)

**Interfaces:**
- Consumes: `audioAssetsFS` (Task 1), `sfxDir` (Task 2).
- Produces (relied on by Tasks 5 & 7):
  - `type TransitionCue struct { Name string; Src string; AtSec float64 }`
  - `ScenesParams` new fields: `AmbientLocalPath string`, `AmbientSrc string`, `TransitionCues []TransitionCue`, `AudioMotion bool`
  - `BuildScenes` copies `AmbientLocalPath` → `assets/ambient.mp3` (sets `AmbientSrc`), and each cue's embedded `Name` → `assets/sfx/<name>` (sets cue `Src`).

- [ ] **Step 1: Add the type + fields**

In `internal/producer/composition_types.go`, add:

```go
// TransitionCue is one scene-transition sound effect placement. Name is the
// embedded SFX base name (input); Src is the project-relative asset path the
// builder fills in; AtSec is the timeline start in seconds.
type TransitionCue struct {
	Name  string  `json:"-"`
	Src   string  `json:"src"`
	AtSec float64 `json:"at"`
}
```

And extend `ScenesParams` (append inside the struct, after `BrandCSS string`):

```go
	// Audio + motion upgrade (gated by AUDIO_MOTION_ENABLED). All zero ⇒ today's
	// voice-only, current-motion output.
	AmbientLocalPath string          // absolute path to a prepared ambient.mp3 (input; builder copies it)
	AmbientSrc       string          // project-relative path; set by BuildScenes
	TransitionCues   []TransitionCue // scene-transition SFX placements
	AudioMotion      bool            // enable upgraded GSAP transitions
```

- [ ] **Step 2: Write the failing builder test**

Add to `internal/producer/composition_builder_test.go`:

```go
func TestBuildScenesCopiesAudio(t *testing.T) {
	projectDir := t.TempDir()
	// A throwaway voice + ambient source file.
	clipDir := t.TempDir()
	voice := filepath.Join(clipDir, "voice.wav")
	if err := os.WriteFile(voice, []byte("RIFFfake"), 0o644); err != nil {
		t.Fatal(err)
	}
	amb := filepath.Join(clipDir, "ambient.mp3")
	if err := os.WriteFile(amb, []byte("ID3fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	sfx := SfxTransitionFiles()
	if len(sfx) == 0 {
		t.Skip("no sfx embedded")
	}
	params := ScenesParams{
		AspectRatio:      "9:16",
		BrandName:        "Ads Vance",
		DurationSeconds:  10,
		VoiceSrc:         "assets/voice.wav",
		AmbientLocalPath: amb,
		TransitionCues:   []TransitionCue{{Name: sfx[0], AtSec: 3.2}},
		AudioMotion:      true,
		Scenes: []SceneSpec{{
			SceneNumber: 1, StartSec: 0, EndSec: 10, BackgroundMode: "css",
			Content: SceneContent{SceneNumber: 1, Start: 0, End: 10, Layout: "hero", Title: "Hi"},
		}},
	}
	b := NewCompositionBuilder(testFontsDir(t)) // existing helper in this test file
	if _, err := b.BuildScenes(params, "clipX", projectDir, voice, nil); err != nil {
		t.Fatalf("BuildScenes: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "assets", "ambient.mp3")); err != nil {
		t.Errorf("ambient.mp3 not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "assets", "sfx", sfx[0])); err != nil {
		t.Errorf("sfx %s not copied: %v", sfx[0], err)
	}
}
```

> If `composition_builder_test.go` has no `testFontsDir` helper, use the fonts dir the existing tests already reference (check the top of that file) or point at `assets/fonts`. Match whatever the neighboring tests do.

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/producer/ -run TestBuildScenesCopiesAudio -v`
Expected: FAIL — fields undefined / assets not copied.

- [ ] **Step 4: Implement the copy logic in `BuildScenes`**

In `internal/producer/composition_builder.go`, after the voice copy block (`params.VoiceSrc = "assets/voice.wav"`, around line 59), add:

```go
	// Ambient bed: copy the prepared duration-matched file into the project.
	if params.AmbientLocalPath != "" {
		if err := copyFile(params.AmbientLocalPath, filepath.Join(assetsDir, "ambient.mp3")); err != nil {
			return "", fmt.Errorf("copy ambient: %w", err)
		}
		params.AmbientSrc = "assets/ambient.mp3"
	}

	// Transition SFX: copy each referenced embedded file into assets/sfx/ and set
	// its project-relative Src. Mutate a local copy of the slice.
	if len(params.TransitionCues) > 0 {
		sfxDst := filepath.Join(assetsDir, "sfx")
		if err := os.MkdirAll(sfxDst, 0o755); err != nil {
			return "", fmt.Errorf("mkdir sfx: %w", err)
		}
		cues := make([]TransitionCue, len(params.TransitionCues))
		copy(cues, params.TransitionCues)
		copied := map[string]bool{}
		for i := range cues {
			name := cues[i].Name
			if name == "" {
				continue
			}
			if !copied[name] {
				data, err := audioAssetsFS.ReadFile(sfxDir + "/" + name)
				if err != nil {
					return "", fmt.Errorf("read embedded sfx %s: %w", name, err)
				}
				if err := os.WriteFile(filepath.Join(sfxDst, name), data, 0o644); err != nil {
					return "", fmt.Errorf("write sfx %s: %w", name, err)
				}
				copied[name] = true
			}
			cues[i].Src = "assets/sfx/" + name
		}
		params.TransitionCues = cues
	}
```

- [ ] **Step 5: Plumb the fields through `RenderCompositionScenes`**

In `internal/producer/composition.go`, extend `scenesTemplateData` (add fields):

```go
	AmbientSrc        string
	AudioMotion       bool
	TransitionCuesJSON template.JS
```

Then in `RenderCompositionScenes`, before building `data`, marshal the cues:

```go
	cuesJSON, err := json.Marshal(p.TransitionCues)
	if err != nil {
		return nil, fmt.Errorf("marshal transition cues: %w", err)
	}
```

And set the new fields inside the `data := scenesTemplateData{...}` literal:

```go
		AmbientSrc:         p.AmbientSrc,
		AudioMotion:        p.AudioMotion,
		TransitionCuesJSON: template.JS(cuesJSON),
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/producer/ -run TestBuildScenesCopiesAudio -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
go build ./... && gofmt -w internal/producer/composition_types.go internal/producer/composition.go internal/producer/composition_builder.go
git add internal/producer/composition_types.go internal/producer/composition.go internal/producer/composition_builder.go internal/producer/composition_builder_test.go
git commit -m "feat(producer): builder copies ambient + transition SFX into project assets"
```

---

## Task 5: Template — render ambient + transition SFX tracks

**Files:**
- Modify: `internal/producer/templates/layout_multi_scene.html.tmpl`
- Create: `internal/producer/composition_audio_render_test.go`

**Interfaces:**
- Consumes: `scenesTemplateData.AmbientSrc`, `.TransitionCuesJSON` (Task 4).

- [ ] **Step 1: Write the failing render test**

`internal/producer/composition_audio_render_test.go`:

```go
package producer

import (
	"strings"
	"testing"
)

func baseAudioParams() ScenesParams {
	return ScenesParams{
		AspectRatio:     "9:16",
		BrandName:       "Ads Vance",
		DurationSeconds: 10,
		VoiceSrc:        "assets/voice.wav",
		Scenes: []SceneSpec{{
			SceneNumber: 1, StartSec: 0, EndSec: 10, BackgroundMode: "css",
			Content: SceneContent{SceneNumber: 1, Start: 0, End: 10, Layout: "hero", Title: "Hi"},
		}},
	}
}

func TestRenderIncludesAmbientAndCues(t *testing.T) {
	p := baseAudioParams()
	p.AmbientSrc = "assets/ambient.mp3"
	p.TransitionCues = []TransitionCue{{Src: "assets/sfx/whoosh1.mp3", AtSec: 3.2}}
	html, err := RenderCompositionScenes(p)
	if err != nil {
		t.Fatal(err)
	}
	s := string(html)
	if !strings.Contains(s, `src="assets/ambient.mp3"`) {
		t.Error("ambient audio tag missing")
	}
	if !strings.Contains(s, `data-track-index="3"`) {
		t.Error("ambient track index missing")
	}
	if !strings.Contains(s, "assets/sfx/whoosh1.mp3") {
		t.Error("transition cue not in cues JSON")
	}
	if !strings.Contains(s, `a.id = "sfx"`) {
		t.Error("SFX audio elements must get a unique id (else silent — Task 0 finding)")
	}
}

func TestRenderOmitsAudioWhenAbsent(t *testing.T) {
	html, err := RenderCompositionScenes(baseAudioParams())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(html), "assets/ambient.mp3") {
		t.Error("ambient tag present when AmbientSrc empty")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/producer/ -run 'TestRenderIncludesAmbient|TestRenderOmitsAudio' -v`
Expected: FAIL — strings not found.

- [ ] **Step 3: Add the ambient `<audio>` tag**

In `layout_multi_scene.html.tmpl`, immediately after the voice `<audio id="vo" ...>` line (line 126), add:

```html
      {{if .AmbientSrc}}<audio id="amb" data-start="0" data-duration="{{.DurationSeconds}}" data-track-index="3" src="{{.AmbientSrc}}" data-volume="0.15"></audio>{{end}}
```

- [ ] **Step 4: Inject SFX cues + build their audio elements in JS**

In the same template, in the `<script>` block, after `const SEGMENTS = {{.SegmentsJSON}};` (line 152), add:

```javascript
      // ── transition SFX cues (server-picked; appended as engine audio tracks) ──
      const CUES = {{.TransitionCuesJSON}};
      (CUES || []).forEach((cue, ci) => {
        const a = document.createElement("audio");
        a.id = "sfx" + ci;                 // REQUIRED: no id ⇒ silent (Task 0 finding)
        a.setAttribute("data-start", cue.at);
        a.setAttribute("data-duration", "1.0");
        a.setAttribute("data-track-index", 4);
        a.setAttribute("data-volume", "0.5");
        a.setAttribute("src", cue.src);
        root.appendChild(a);
      });
```

> `root` is already defined at line 138 (`const root = document.getElementById("root");`), which precedes this insertion point. `{{.TransitionCuesJSON}}` renders `null` when there are no cues, so `(CUES || [])` is safe. Each SFX audio element gets a unique `id` (`sfx0`, `sfx1`, …) — without it the renderer treats the element as undiscoverable and the SFX is silent (Task 0 spike finding).

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/producer/ -run 'TestRenderIncludesAmbient|TestRenderOmitsAudio' -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
go build ./...
git add internal/producer/templates/layout_multi_scene.html.tmpl internal/producer/composition_audio_render_test.go
git commit -m "feat(producer): render ambient bed + transition SFX audio tracks in template"
```

---

## Task 6: Template — safe GSAP motion upgrade (gated)

Upgrade the per-scene entrance to a slide + scale + fade transition and add accent motion, **only when `AudioMotion` is true**. Transform/opacity only (GPU-friendly). When false, the exact current tweens run.

**Files:**
- Modify: `internal/producer/templates/layout_multi_scene.html.tmpl`
- Modify: `internal/producer/composition_audio_render_test.go` (add a flag-presence test)

**Interfaces:**
- Consumes: `scenesTemplateData.AudioMotion` (Task 4).

- [ ] **Step 1: Write the failing test**

Add to `internal/producer/composition_audio_render_test.go`:

```go
func TestRenderMotionFlag(t *testing.T) {
	on := baseAudioParams()
	on.AudioMotion = true
	h, err := RenderCompositionScenes(on)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(h), "const MOTION_UP = true") {
		t.Error("MOTION_UP=true not emitted when AudioMotion on")
	}
	off, err := RenderCompositionScenes(baseAudioParams())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(off), "const MOTION_UP = false") {
		t.Error("MOTION_UP=false not emitted when AudioMotion off")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/producer/ -run TestRenderMotionFlag -v`
Expected: FAIL — `MOTION_UP` not found.

- [ ] **Step 3: Emit the flag + branch the scene entrance**

In `layout_multi_scene.html.tmpl`, just after `const TOTAL = {{.DurationSeconds}};` (line 131), add:

```javascript
      const MOTION_UP = {{if .AudioMotion}}true{{else}}false{{end}};
```

Then replace the scene-entrance tween block (lines 201–205, the `tl.fromTo(sceneEl,...)`, the `if(bg)...` ken-burns, and the `content` entrance) with a branch:

```javascript
        if (MOTION_UP) {
          // Upgraded: incoming scene slides up + scales in while fading; bg ken-burns.
          tl.fromTo(sceneEl,{opacity:0,scale:1.04},{opacity:1,scale:1,duration:idx===0?0.5:0.6,ease:"power3.out"},inAt);
          if(bg) tl.fromTo(bg,{scale:1.04},{scale:1.10,duration:span,ease:"none"},inAt);
          if(content){
            tl.fromTo(content,{y:60,opacity:0},{y:0,opacity:1,duration:0.65,ease:"power3.out"},sc.start+0.08);
          }
        } else {
          // Current behavior (unchanged).
          tl.fromTo(sceneEl,{opacity:0},{opacity:1,duration:idx===0?0.5:0.55,ease:"power2.out"},inAt);
          if(bg) tl.fromTo(bg,{scale:1.0},{scale:1.06,duration:span,ease:"none"},inAt);
          if(content){
            tl.fromTo(content,{y:46,opacity:0},{y:0,opacity:1,duration:0.6,ease:"power3.out"},sc.start+0.1);
          }
        }
```

> Keep the existing per-child loop (`const kids=content.children; ...`, lines 206–214) and `tl.set(sceneEl,{opacity:0},sc.end)` exactly as they are — they run after this block for both branches. Only the three lines named above move inside the branch; do not duplicate the `kids` loop.

- [ ] **Step 4: Run the full producer test suite to verify nothing regressed**

Run: `go test ./internal/producer/ -run 'TestRender|TestBuildScenes' -v`
Expected: PASS (motion flag + audio + existing render tests).

- [ ] **Step 5: Commit**

```bash
go build ./...
git add internal/producer/templates/layout_multi_scene.html.tmpl internal/producer/composition_audio_render_test.go
git commit -m "feat(producer): gated GSAP motion upgrade (slide+scale entrance, stronger ken-burns)"
```

---

## Task 7: Wire audio + motion into `AssembleHyperframes916`

Select the ambient bed (avoid-last via `settings.last_ambient`), prepare the duration-matched bed, build transition cues at scene boundaries, and pass everything to `BuildScenes` — all gated by `AudioMotionEnabled()`. Self-contained in the producer; the orchestrator is untouched.

**Files:**
- Modify: `internal/producer/producer.go` (`AssembleHyperframes916`; add two settings helpers)
- Create: `internal/producer/producer_audio_test.go`

**Interfaces:**
- Consumes: `AudioMotionEnabled`, `PickAmbient`, `PickTransitionSFX` (Task 2); `BuildAmbientBed` (Task 3); `ScenesParams` audio fields (Task 4); `audioAssetsFS`, `ambientDir` (Tasks 1–2).

- [ ] **Step 1: Write the failing test for boundary-cue construction**

Extract the cue-timing into a pure helper so it is unit-testable without a DB or ffmpeg. `internal/producer/producer_audio_test.go`:

```go
package producer

import "testing"

func TestTransitionCuesAtBoundaries(t *testing.T) {
	specs := []SceneSpec{
		{SceneNumber: 1, StartSec: 0, EndSec: 3},
		{SceneNumber: 2, StartSec: 3, EndSec: 6},
		{SceneNumber: 3, StartSec: 6, EndSec: 9},
	}
	names := []string{"a.mp3", "b.mp3"} // one per boundary (scenes-1 = 2)
	cues := buildTransitionCues(specs, names)
	if len(cues) != 2 {
		t.Fatalf("want 2 cues (boundaries), got %d", len(cues))
	}
	// Cue fires slightly before the incoming scene start.
	if cues[0].AtSec < 2.5 || cues[0].AtSec > 3.0 {
		t.Errorf("cue0 AtSec=%v, want just before 3.0", cues[0].AtSec)
	}
	if cues[0].Name != "a.mp3" || cues[1].Name != "b.mp3" {
		t.Errorf("cue names mismatch: %+v", cues)
	}
}

func TestTransitionCuesSingleScene(t *testing.T) {
	if cues := buildTransitionCues([]SceneSpec{{SceneNumber: 1, StartSec: 0, EndSec: 5}}, nil); len(cues) != 0 {
		t.Errorf("single scene → 0 boundaries, got %d", len(cues))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/producer/ -run TestTransitionCues -v`
Expected: FAIL — `buildTransitionCues` undefined.

- [ ] **Step 3: Implement the helper + settings accessors in `producer.go`**

Add near the other helpers:

```go
// buildTransitionCues places one SFX per scene boundary (scene 2..N), firing
// 0.2s before the incoming scene's start so the whoosh leads the visual cut.
// names[i] is used for boundary i; extra names/specs are ignored. Returns nil
// when there are <2 scenes or no names.
func buildTransitionCues(specs []SceneSpec, names []string) []TransitionCue {
	if len(specs) < 2 || len(names) == 0 {
		return nil
	}
	var cues []TransitionCue
	for i := 1; i < len(specs); i++ {
		if i-1 >= len(names) {
			break
		}
		at := specs[i].StartSec - 0.2
		if at < 0 {
			at = 0
		}
		cues = append(cues, TransitionCue{Name: names[i-1], AtSec: at})
	}
	return cues
}

func (p *Producer) lastAmbient(ctx context.Context) string {
	var v string
	_ = p.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = 'last_ambient'`).Scan(&v)
	return v
}

func (p *Producer) saveLastAmbient(ctx context.Context, name string) {
	if _, err := p.pool.Exec(ctx,
		`INSERT INTO settings (key, value) VALUES ('last_ambient', $1)
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, name); err != nil {
		log.Printf("saveLastAmbient: %v", err)
	}
}
```

> Verify `settings` has a unique/PK on `key` (it is used as a key/value store throughout, e.g. `getVoice`). If `ON CONFLICT (key)` errors at runtime because there is no unique constraint, add a one-line migration `CREATE UNIQUE INDEX IF NOT EXISTS settings_key_uniq ON settings(key);` as a prerequisite step and re-run.

- [ ] **Step 4: Run the helper test to verify it passes**

Run: `go test ./internal/producer/ -run TestTransitionCues -v`
Expected: PASS.

- [ ] **Step 5: Wire selection into `AssembleHyperframes916`**

In `internal/producer/producer.go`, after `params` is built (after line 308, before the `projectDir` block at line 310), add:

```go
	if AudioMotionEnabled() {
		params.AudioMotion = true

		// Ambient bed: avoid-last pick, extracted from embed, looped to clip length.
		lastAmb := p.lastAmbient(ctx)
		ambName := PickAmbient(lastAmb, rand.Intn)
		if ambName != "" && total > 0 {
			rawAmb := filepath.Join(clipDir, "ambient-src.mp3")
			if data, rerr := audioAssetsFS.ReadFile(ambientDir + "/" + ambName); rerr == nil {
				if werr := os.WriteFile(rawAmb, data, 0o644); werr == nil {
					preparedAmb := filepath.Join(clipDir, "ambient.mp3")
					if berr := p.ffmpeg.BuildAmbientBed(rawAmb, preparedAmb, total); berr == nil {
						params.AmbientLocalPath = preparedAmb
						p.saveLastAmbient(ctx, ambName)
					} else {
						log.Printf("AssembleHyperframes916: ambient bed prep failed (continuing without): %v", berr)
					}
				}
			}
		}

		// Transition SFX: one per scene boundary.
		sfxNames := PickTransitionSFX(len(specs)-1, rand.Intn)
		params.TransitionCues = buildTransitionCues(specs, sfxNames)
	}
```

Add imports if missing: `"math/rand"` (and confirm `"os"`, `"path/filepath"`, `"log"` are already imported — they are).

- [ ] **Step 6: Build + run the full producer suite**

Run: `go build ./... && go test ./internal/producer/ -v`
Expected: PASS (render-gated tests SKIP without `HF_RENDER=1`).

- [ ] **Step 7: Commit**

```bash
gofmt -w internal/producer/producer.go
git add internal/producer/producer.go internal/producer/producer_audio_test.go
git commit -m "feat(producer): wire ambient (avoid-last) + transition SFX + motion into hyperframes assembly"
```

---

## Task 8: End-to-end render verification + rollout docs

**Files:**
- Modify: `docs/superpowers/specs/2026-06-30-audio-motion-upgrade-design.md` (append a "Verification" note) — optional
- No new code; this task proves the feature on a real render and documents the flag.

- [ ] **Step 1: Render one clip locally with the flag on**

Use the existing opt-in render test harness (`composition_scenes_render_test.go` runs with `HF_RENDER=1`). Run with the flag and a real voice file:

Run:
```bash
AUDIO_MOTION_ENABLED=true HF_RENDER=1 \
  HF_ASPECT=9:16 HF_FONT_SRC=internal/producer/assets/fonts \
  HF_VOICE_SRC=<path/to/a/real/voice.wav> \
  go test ./internal/producer/ -run TestRenderCompositionScenes -v
```
Expected: a rendered MP4 in the kept dir (`HF_KEEP_DIR` if the harness supports it). If this specific harness does not exercise `AssembleHyperframes916`, instead trigger one real production clip on a staging/local run of the server with `AUDIO_MOTION_ENABLED=true` and inspect its `composition-916/output.mp4`.

- [ ] **Step 2: Verify the output audibly**

Open the MP4 and confirm: (1) quiet ambient bed under the voice, (2) a whoosh at each scene cut, (3) upgraded slide+scale scene entrances, (4) voice still clear and dominant. Run `ffprobe -v error -show_streams -select_streams a output.mp4` to confirm a single mixed audio stream.

- [ ] **Step 3: Verify flag-off is unchanged**

Run the same render with `AUDIO_MOTION_ENABLED` unset. Confirm no ambient/SFX and the original motion. This proves the no-op guarantee.

- [ ] **Step 4: Confirm render budget is intact**

Check the render log: 3 workers, completes within the 20m timeout, memory within the container envelope (per the known prod limits). If render time grew materially, reduce SFX count or simplify the motion before rollout.

- [ ] **Step 5: Document the rollout**

Append to the design doc (or a short note in the PR description): enable on prod by setting Railway env var `AUDIO_MOTION_ENABLED=true` on the backend service; rollback by removing it (no redeploy of code needed). Do not deploy while a clip is producing (known prod constraint).

- [ ] **Step 6: Commit any doc changes**

```bash
git add docs/superpowers/specs/2026-06-30-audio-motion-upgrade-design.md
git commit -m "docs: audio+motion verification notes and rollout steps"
```

---

## Self-Review

**Spec coverage:**
- Transition SFX → Tasks 1,2,4,5,7. ✅
- Ambient music (asset library, avoid-last, low volume) → Tasks 1,2,3,4,5,7. ✅
- Safe motion upgrade (transform/opacity) → Task 6. ✅
- Asset-library source (no AI gen) → Task 1. ✅
- No agent changes → confirmed; all work in `internal/producer`. ✅
- Feature flag `AUDIO_MOTION_ENABLED`, default off, no-op when off → Tasks 2,5,6,7 + Task 8 Step 3. ✅
- Multi-track mixing risk + ffmpeg-amix fallback → Task 0 gate. ✅
- Rollout/rollback → Task 8. ✅
- Track indices non-colliding (ambient=3, sfx=4) → Global Constraints + Task 5. ✅

**Placeholder scan:** No "TBD"/"add error handling"-style steps; every code step shows the code. The two `>`-quoted caveats (settings unique key; testFontsDir helper) are explicit verification instructions, not deferred work. ✅

**Type consistency:** `TransitionCue{Name,Src,AtSec}` defined in Task 4 and used identically in Tasks 5 (`cue.src`,`cue.at` JSON tags), 7 (`buildTransitionCues`). `ScenesParams` audio fields defined in Task 4, consumed in Tasks 5,6,7. `BuildAmbientBed(src,out,dur)` defined in Task 3, called in Task 7. `PickAmbient(last, rng)` / `PickTransitionSFX(n, rng)` signatures consistent across Tasks 2 and 7. ✅
