# Visual QA Reliability Fix — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop real Thai text overflow in rendered clips (Track A), kill Visual-QA false positives from sampling timing / karaoke captions / verbatim-text comparison (Track B), and close publish-gate holes (Track C).

**Architecture:** Track A edits the single HTML template (`layout_multi_scene.html.tmpl`) — CSS wrap rules + a build-time auto-fit JS pass. Track B adds a two-strike confirm pass in the orchestrator (re-sample flagged scenes at a later timestamp, re-judge, fail only if both flag), clamps sampling away from entrance/transition windows, and updates the QA prompt via migration. Track C adds a fail-closed env flag, a status guard on `SetAutoReviewHeld`, and a handler guard on PATCH status.

**Tech Stack:** Go 1.x, html/template, GSAP (in-template JS), PostgreSQL migrations (plain SQL, auto-applied on Railway deploy), chi router.

**Spec:** `docs/superpowers/specs/2026-07-07-visual-qa-reliability-design.md`

## Global Constraints

- Repo root: `/Users/jaochai/Code/video-fb`. Verify with `go build ./...` and `go test ./...` (no network needed).
- Env-flag pattern is exactly `os.Getenv("X_ENABLED") == "true"` (see `internal/producer/hyperframes.go:32`).
- Migrations are plain SQL in `migrations/`, idempotent (guard with `WHERE ... NOT LIKE`), next number is **051**.
- Thai-safe rules in the template: NEVER negative letter-spacing; line-height ≥1.25 on wrapped Thai text.
- User-facing strings (log reasons written to DB, QA issues) are Thai; code identifiers/comments English.
- Do NOT touch: fonts/themes in `presets.go`, the human Override button, auto-review agent logic.
- `git commit` must run WITHOUT `--no-verify` (a hook blocks it).
- Each task commits on its own. Commit messages end with:
  `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>`

---

### Task 1: Template CSS — Thai word-break, stat box, wrap coverage

**Files:**
- Modify: `internal/producer/templates/layout_multi_scene.html.tmpl` (CSS block, lines ~56-143)
- Test: `internal/producer/template_overflow_test.go` (create)

**Interfaces:**
- Consumes: `RenderCompositionScenes(p ScenesParams) ([]byte, error)` (`internal/producer/composition.go:60`) — already exists.
- Produces: rendered HTML whose CSS Task 2's test also asserts against. No Go API changes.

- [ ] **Step 1: Write the failing test**

Create `internal/producer/template_overflow_test.go`:

```go
package producer

import (
	"strings"
	"testing"
)

// overflowTestParams is the minimal valid input for RenderCompositionScenes.
func overflowTestParams() ScenesParams {
	return ScenesParams{
		AspectRatio:     "9:16",
		DurationSeconds: 10,
		VoiceSrc:        "assets/voice.wav",
		Scenes:          []SceneSpec{{SceneNumber: 1, StartSec: 0, EndSec: 10}},
	}
}

// Kanit/Prompt heading fonts (Design Themes, PR #14) render wider Thai glyphs
// than the Sarabun these px sizes were tuned for. The template must (a) never
// use `overflow-wrap:anywhere` — it cuts Thai mid-word instead of letting
// Chromium's ICU dictionary break at word boundaries — and (b) not narrow the
// stat box below the default 56px gutters.
func TestTemplateThaiWrapRules(t *testing.T) {
	out, err := RenderCompositionScenes(overflowTestParams())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)

	if strings.Contains(html, "overflow-wrap:anywhere") {
		t.Error("template still uses overflow-wrap:anywhere (cuts Thai mid-word)")
	}
	if strings.Contains(html, `left:110px`) {
		t.Error("stat box is still narrowed to 110px gutters (overflows 230px digits)")
	}
	if !strings.Contains(html, "overflow-wrap:break-word") {
		t.Error("missing overflow-wrap:break-word last-resort rule")
	}
	// Unit must scale with the stat digits (auto-fit shrinks the parent font-size).
	if !strings.Contains(html, ".stat .unit{font-size:.37em") {
		t.Error(".stat .unit is not em-based (won't shrink with auto-fit)")
	}
	// Stat is a number + unit: it must not wrap (auto-fit shrinks it instead).
	if !strings.Contains(html, "white-space:nowrap;font-variant-numeric:tabular-nums") {
		t.Error(".stat must be white-space:nowrap so auto-fit (not wrapping) handles overflow")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/producer/ -run TestTemplateThaiWrapRules -v`
Expected: FAIL (template still has `overflow-wrap:anywhere`, `left:110px`, px-based unit).

- [ ] **Step 3: Edit the template CSS (4 edits)**

In `internal/producer/templates/layout_multi_scene.html.tmpl`:

**Edit 3a** — delete the stat-narrowing rule. Replace:

```
      .scene[data-layout="hook"] .scene-content{bottom:560px}                 /* raise the opener high on frame */
      .scene[data-layout="stat"] .scene-content{left:110px;right:110px}       /* narrow to spotlight the number card */
      .scene[data-layout="step"] .scene-content,
```

with:

```
      .scene[data-layout="hook"] .scene-content{bottom:560px}                 /* raise the opener high on frame */
      .scene[data-layout="step"] .scene-content,
```

**Edit 3b** — make `.stat` nowrap (auto-fit in Task 2 shrinks it; wrapping a 230px number is never acceptable). Replace:

```
      .stat{font-weight:800;font-size:230px;line-height:.9;color:var(--amber-bright);text-align:center;
        font-variant-numeric:tabular-nums;text-shadow:0 8px 44px rgba(255,180,84,.3)}
```

with:

```
      .stat{font-weight:800;font-size:230px;line-height:.9;color:var(--amber-bright);text-align:center;
        white-space:nowrap;font-variant-numeric:tabular-nums;text-shadow:0 8px 44px rgba(255,180,84,.3)}
```

**Edit 3c** — unit scales with the stat font-size. Replace:

```
      .stat .unit{font-size:84px;color:#fff;margin-left:10px}
```

with:

```
      .stat .unit{font-size:.37em;color:#fff;margin-left:10px}
```

(84/230 ≈ .365 — visually identical at full size, and it now shrinks with auto-fit.)

**Edit 3d** — Thai-safe wrapping. Replace:

```
      .cap-phrase{position:absolute;bottom:0;width:920px;text-align:center;
        font-weight:700;font-size:48px;line-height:1.34;color:#fff;
        background:rgba(8,24,64,.86);border:2px solid var(--amber-bright);border-radius:24px;
        padding:26px 40px;opacity:0;box-shadow:0 18px 50px rgba(0,0,0,.5);word-break:break-word}
      .cap-word{display:inline;white-space:normal;overflow-wrap:anywhere}
      h1.title,.stat-label,.row .rt{overflow-wrap:anywhere;word-break:break-word}
      .cta,.pill{max-width:100%;overflow-wrap:anywhere}
```

with:

```
      .cap-phrase{position:absolute;bottom:0;width:920px;text-align:center;
        font-weight:700;font-size:48px;line-height:1.34;color:#fff;
        background:rgba(8,24,64,.86);border:2px solid var(--amber-bright);border-radius:24px;
        padding:26px 40px;opacity:0;box-shadow:0 18px 50px rgba(0,0,0,.5)}
      /* Thai line breaking: no `anywhere` / `word-break:break-word` — Chromium's
         ICU dictionary breaks Thai at word boundaries by default; break-word is
         kept only as the last-resort escape for a truly unbreakable run. */
      .cap-word{display:inline;white-space:normal;overflow-wrap:break-word}
      h1.title,.stat-label,.row .rt,.step-title,.step-of,.kicker,.sub,.brandbig,
      .chip .t{overflow-wrap:break-word}
      .cta,.pill{max-width:100%;overflow-wrap:break-word}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/producer/ -run TestTemplateThaiWrapRules -v`
Expected: PASS

- [ ] **Step 5: Run the full producer package tests (template regressions)**

Run: `go test ./internal/producer/`
Expected: PASS (fixtures like `composition_scenes_render_test.go` must not break).

- [ ] **Step 6: Commit**

```bash
git add internal/producer/templates/layout_multi_scene.html.tmpl internal/producer/template_overflow_test.go
git commit -m "fix(template): Thai word-boundary wrapping + restore stat box width

overflow-wrap:anywhere cut Thai mid-word ('ประ' alone on a line); Chromium's
ICU dictionary breaks Thai correctly when we stop forcing anywhere/break-word.
Stat box back to 56px gutters, unit em-based, stat nowrap (auto-fit next).

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 2: Template auto-fit JS for nowrap elements

**Files:**
- Modify: `internal/producer/templates/layout_multi_scene.html.tmpl` (JS, content-build section ~line 198-227)
- Test: `internal/producer/template_overflow_test.go` (extend)

**Interfaces:**
- Consumes: the DOM built by the `SCENES.forEach` content builder in the same template; `.stat-num` span emitted for MOTION_V2 stat count-up.
- Produces: `fitText` JS function + a `data-final` attribute on `.stat-num`. No Go API changes.

- [ ] **Step 1: Write the failing test**

Append to `internal/producer/template_overflow_test.go`:

```go
// The auto-fit pass shrinks nowrap text (.stat, .chip .n) that overflows its
// box — Kanit/Prompt digits+unit run wider than Sarabun. The MOTION_V2 stat
// counts up from "0", so it must be measured at its widest (final) value.
func TestTemplateHasAutoFit(t *testing.T) {
	out, err := RenderCompositionScenes(overflowTestParams())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)
	if !strings.Contains(html, "function fitText") {
		t.Error("template is missing the fitText auto-fit pass")
	}
	if !strings.Contains(html, `data-final=`) {
		t.Error("stat-num span is missing data-final (auto-fit would measure the count-up '0')")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/producer/ -run TestTemplateHasAutoFit -v`
Expected: FAIL ("missing the fitText auto-fit pass")

- [ ] **Step 3: Edit the template JS (2 edits)**

**Edit 3a** — carry the final stat value for measurement. Replace:

```
          const numHTML=meta?'<span class="stat-num">0</span>':(sc.stat||"");
```

with:

```
          const numHTML=meta?'<span class="stat-num" data-final="'+sc.stat+'">0</span>':(sc.stat||"");
```

**Edit 3b** — add the auto-fit pass right after the content-build loop. Replace:

```
        else if(sc.type==="cta"){
          c.appendChild(el("h1","title",sc.title));
          c.appendChild(el("div","cta",sc.cta));
          c.appendChild(el("div","brandbig",sc.brand));
          if(sc.sub)c.appendChild(el("div","sub",sc.sub));
        }
      });
```

with:

```
        else if(sc.type==="cta"){
          c.appendChild(el("h1","title",sc.title));
          c.appendChild(el("div","cta",sc.cta));
          c.appendChild(el("div","brandbig",sc.brand));
          if(sc.sub)c.appendChild(el("div","sub",sc.sub));
        }
      });

      // ── auto-fit: Kanit/Prompt Thai glyphs run wider than the Sarabun these px
      // sizes were tuned for. Nowrap blocks (.stat, .chip .n) can overflow their
      // box; shrink font-size until they fit (floor 55%). Layout is computed even
      // while scenes sit at opacity:0, so measuring here is safe. The MOTION_V2
      // stat counts up from "0", so measure at its widest (final) value first.
      function fitText(el){
        let size=parseFloat(getComputedStyle(el).fontSize);
        const min=size*0.55;
        let guard=40;
        while(guard-->0&&size>min&&el.scrollWidth>el.clientWidth+1){
          size=Math.max(min,size-2);
          el.style.fontSize=size+"px";
        }
      }
      document.querySelectorAll(".scene-content .stat,.scene-content .chip .n").forEach(function(el){
        const num=el.querySelector(".stat-num");
        if(num)num.textContent=num.getAttribute("data-final")||"0";
        fitText(el);
        if(num)num.textContent="0";
      });
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/producer/ -run 'TestTemplateHasAutoFit|TestTemplateThaiWrapRules' -v`
Expected: PASS (both)

- [ ] **Step 5: Run the full producer package**

Run: `go test ./internal/producer/`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/producer/templates/layout_multi_scene.html.tmpl internal/producer/template_overflow_test.go
git commit -m "fix(template): auto-fit shrinks overflowing stat/chip numbers

Kanit/Prompt digits+unit overflow boxes sized for Sarabun ('40,000 บาท' cut
at the right edge). Measure at build time and shrink font-size to fit;
MOTION_V2 count-up is measured at its final value via data-final.

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 3: QA sampling — entrance/transition guards

**Files:**
- Modify: `internal/orchestrator/orchestrator.go` (`sceneAwareTimestamps`, ~line 776-800)
- Test: `internal/orchestrator/qa_frames_test.go` (extend)

**Interfaces:**
- Consumes: `sceneAwareTimestamps(durations []float64, probedDur, frac float64) []float64` — signature unchanged.
- Produces: same signature; new constants `qaEntranceGuardSec = 1.6`, `qaExitGuardSec = 0.4` used only inside. Task 5 relies on the exit guard keeping its `frac=0.85` samples off transitions.

- [ ] **Step 1: Write the failing tests**

Append to `internal/orchestrator/qa_frames_test.go`:

```go
// Content entrance animations run up to ~1.5s into a scene; a sample inside that
// window sees elements mid-motion (looks cropped/overlapping → false positive).
// Samples must sit at least min(1.6s, half the scene) after the scene start.
func TestSceneAwareTimestamps_EntranceGuard(t *testing.T) {
	durs := []float64{3, 10}
	ts := sceneAwareTimestamps(durs, 13, 0.45) // raw scene0 = 1.35 < guard 1.5
	if ts[0] < 1.5-1e-9 {
		t.Errorf("scene 0 sampled at %.3f, want >= 1.5 (entrance guard)", ts[0])
	}
	if ts[0] >= 3 {
		t.Errorf("scene 0 sampled at %.3f, must stay inside the scene (< 3)", ts[0])
	}
}

// A sample too close to the scene end lands on the crossfade into the next
// scene (blank/blended frame). Keep at least 0.4s clear of the scene end.
func TestSceneAwareTimestamps_ExitGuard(t *testing.T) {
	durs := []float64{2, 10}
	ts := sceneAwareTimestamps(durs, 12, 0.9) // raw scene0 = 1.8 > hi 1.6
	if ts[0] > 2-0.4+1e-9 {
		t.Errorf("scene 0 sampled at %.3f, want <= 1.6 (exit guard)", ts[0])
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/orchestrator/ -run 'TestSceneAwareTimestamps_EntranceGuard|TestSceneAwareTimestamps_ExitGuard' -v`
Expected: FAIL (no clamping yet: 1.35 < 1.5 and 1.8 > 1.6).

- [ ] **Step 3: Implement the clamp**

In `internal/orchestrator/orchestrator.go`, add the constants next to `qaSceneFrac` (~line 763):

```go
// qaEntranceGuardSec / qaExitGuardSec clamp each per-scene sample away from the
// content entrance animation (runs up to ~1.5s into a scene — a mid-animation
// frame looks cropped/overlapping to vision QA) and away from the scene-end
// crossfade. The entrance guard is capped at half the scene so a short scene
// still gets sampled inside its own window.
const (
	qaEntranceGuardSec = 1.6
	qaExitGuardSec     = 0.4
)
```

Then replace the loop body of `sceneAwareTimestamps`:

```go
	ts := make([]float64, len(durations))
	var acc float64
	for i, d := range durations {
		if d < 0 {
			d = 0
		}
		ts[i] = (acc + d*frac) * scale
		acc += d
	}
	return ts
```

with:

```go
	ts := make([]float64, len(durations))
	var acc float64
	for i, d := range durations {
		if d < 0 {
			d = 0
		}
		t := acc + d*frac
		lo := acc + math.Min(qaEntranceGuardSec, d*0.5)
		hi := acc + d - qaExitGuardSec
		if hi < lo {
			hi = lo
		}
		if t < lo {
			t = lo
		}
		if t > hi {
			t = hi
		}
		ts[i] = t * scale
		acc += d
	}
	return ts
```

(`math` is already imported by `orchestrator.go`; if not, add it.)

- [ ] **Step 4: Run the whole test file (old invariants must hold)**

Run: `go test ./internal/orchestrator/ -run 'TestSceneAware|TestEvenFrame|TestQAFrame' -v`
Expected: PASS — including the pre-existing `LandsInsideEachScene`, `RescalesToProbed` (durations there are long enough that the clamps don't move them), and `QAandAutoReviewDiffer`.

- [ ] **Step 5: Commit**

```bash
git add internal/orchestrator/orchestrator.go internal/orchestrator/qa_frames_test.go
git commit -m "fix(qa): clamp frame samples away from entrance/transition windows

A sample inside the first ~1.5s catches elements mid-entrance (reads as
cropped/overlapping text to vision QA); one within 0.4s of scene end lands
on the crossfade. Clamp per-scene to [start+min(1.6,d/2), end-0.4].

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 4: agent.ConfirmMerge — two-strike verdict merge (pure logic)

**Files:**
- Modify: `internal/agent/visualqa.go` (append)
- Test: `internal/agent/visualqa_confirm_test.go` (create)

**Interfaces:**
- Consumes: `SceneVerdict`, `VisualQAResult`, `summarizeVerdicts` (all already in `visualqa.go`).
- Produces: `func ConfirmMerge(first, confirm VisualQAResult) VisualQAResult` — Task 5 calls exactly this.

- [ ] **Step 1: Write the failing tests**

Create `internal/agent/visualqa_confirm_test.go`:

```go
package agent

import (
	"strings"
	"testing"
)

func v(scene int, ok bool, issues ...string) SceneVerdict {
	return SceneVerdict{SceneNumber: scene, OK: ok, Issues: issues}
}

func TestConfirmMerge_PassedFirstIsUntouched(t *testing.T) {
	first := VisualQAResult{Verdicts: []SceneVerdict{v(1, true), v(2, true)}, Passed: true}
	got := ConfirmMerge(first, VisualQAResult{})
	if !got.Passed || len(got.Verdicts) != 2 {
		t.Fatalf("passed result must be returned unchanged, got %+v", got)
	}
}

// A defect visible at BOTH sample points is real → scene stays failed, issues merged.
func TestConfirmMerge_BothFlag_StaysFailed(t *testing.T) {
	first := VisualQAResult{Verdicts: []SceneVerdict{v(1, false, "ล้นกรอบ")}, Passed: false}
	confirm := VisualQAResult{Verdicts: []SceneVerdict{v(1, false, "ยังล้นกรอบ")}, Passed: false}
	got := ConfirmMerge(first, confirm)
	if got.Passed {
		t.Fatal("scene flagged by both passes must keep the clip failed")
	}
	if len(got.Verdicts[0].Issues) != 2 {
		t.Errorf("issues must merge both passes, got %v", got.Verdicts[0].Issues)
	}
}

// Flagged only at the first sample (mid-animation / karaoke phrase) → cleared.
func TestConfirmMerge_ConfirmOK_Clears(t *testing.T) {
	first := VisualQAResult{Verdicts: []SceneVerdict{v(1, false, "ข้อความถูกตัด")}, Passed: false}
	confirm := VisualQAResult{Verdicts: []SceneVerdict{v(1, true)}, Passed: true}
	got := ConfirmMerge(first, confirm)
	if !got.Passed {
		t.Fatal("scene cleared by the confirm pass must pass the clip")
	}
	if !got.Verdicts[0].OK {
		t.Error("verdict must flip to OK")
	}
	if len(got.Verdicts[0].Issues) == 0 || !strings.Contains(got.Verdicts[0].Issues[0], "recheck") {
		t.Errorf("cleared verdict must keep an audit note, got %v", got.Verdicts[0].Issues)
	}
}

// No confirm frame for the scene (extraction failed) → fail-open: cleared.
func TestConfirmMerge_MissingConfirmFrame_Clears(t *testing.T) {
	first := VisualQAResult{Verdicts: []SceneVerdict{v(3, false, "จอว่าง")}, Passed: false}
	got := ConfirmMerge(first, VisualQAResult{Passed: true})
	if !got.Passed {
		t.Fatal("missing confirm frame must fail open (scene cleared)")
	}
}

func TestConfirmMerge_Mixed(t *testing.T) {
	first := VisualQAResult{
		Verdicts: []SceneVerdict{v(1, true), v(2, false, "ล้น"), v(3, false, "ตัด")},
		Passed:   false,
	}
	confirm := VisualQAResult{Verdicts: []SceneVerdict{v(2, false, "ล้นจริง"), v(3, true)}, Passed: false}
	got := ConfirmMerge(first, confirm)
	if got.Passed {
		t.Fatal("scene 2 confirmed failed → clip must fail")
	}
	if got.Verdicts[1].OK || !got.Verdicts[2].OK || !got.Verdicts[0].OK {
		t.Errorf("want scene2 failed, scenes 1&3 ok; got %+v", got.Verdicts)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/agent/ -run TestConfirmMerge -v`
Expected: FAIL with "undefined: ConfirmMerge"

- [ ] **Step 3: Implement ConfirmMerge**

Append to `internal/agent/visualqa.go`:

```go
// ConfirmMerge resolves a two-strike QA: a scene flagged by the first pass stays
// failed only if the confirm pass (a frame sampled later in the same scene) also
// flagged it. This kills timing false positives — karaoke captions mid-reveal,
// entrance animations still settling — while a baked-in defect (an overflowing
// headline is wrong at every timestamp) survives both passes. A flagged scene
// the confirm pass has no verdict for (frame extraction failed) is cleared:
// fail-open, matching reviewFrame's infra policy.
func ConfirmMerge(first, confirm VisualQAResult) VisualQAResult {
	if first.Passed {
		return first
	}
	confirmFailed := make(map[int][]string)
	for _, v := range confirm.Verdicts {
		if !v.OK {
			confirmFailed[v.SceneNumber] = v.Issues
		}
	}
	out := make([]SceneVerdict, len(first.Verdicts))
	for i, v := range first.Verdicts {
		if v.OK {
			out[i] = v
			continue
		}
		if issues, still := confirmFailed[v.SceneNumber]; still {
			merged := append(append([]string{}, v.Issues...), issues...)
			out[i] = SceneVerdict{SceneNumber: v.SceneNumber, OK: false, Issues: merged}
			continue
		}
		note := append([]string{"เฟรมยืนยัน (recheck) ไม่พบปัญหา — เคลียร์ผลรอบแรก"}, v.Issues...)
		out[i] = SceneVerdict{SceneNumber: v.SceneNumber, OK: true, Issues: note}
	}
	return VisualQAResult{Verdicts: out, Passed: summarizeVerdicts(out)}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/agent/ -run TestConfirmMerge -v`
Expected: PASS (all five)

- [ ] **Step 5: Commit**

```bash
git add internal/agent/visualqa.go internal/agent/visualqa_confirm_test.go
git commit -m "feat(qa): ConfirmMerge — two-strike verdict merge for visual QA

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 5: Orchestrator — confirm pass wiring (re-sample flagged scenes at 85%)

**Files:**
- Modify: `internal/orchestrator/orchestrator.go` (`extractQAFrames` ~line 834, QA block in `renderAndFinalize` ~line 509-526, consts ~line 763)

**Interfaces:**
- Consumes: `agent.ConfirmMerge` (Task 4), guarded `sceneAwareTimestamps` (Task 3 — the exit guard keeps `frac=0.85` off transitions).
- Produces: `extractQAFramesAt(clipID, mp4Path string, scenes []agent.GeneratedScene, frac float64, only map[int]bool) []agent.QAFrame`; `extractQAFrames` becomes a thin wrapper. Behavior change only when the first QA pass fails.

- [ ] **Step 1: Add the recheck constant**

Next to `qaSceneFrac` (with the Task 3 consts):

```go
// qaRecheckSceneFrac positions the confirm-pass sample late in the scene — past
// every entrance animation and on a different karaoke caption phrase than the
// first pass — so a defect must be visible at two independent times to fail.
const qaRecheckSceneFrac = 0.85
```

- [ ] **Step 2: Generalize the frame extractor**

Replace the whole `extractQAFrames` function with:

```go
// extractQAFramesAt extracts one PNG frame per scene at `frac` into each scene
// and pairs it with the scene's text. When `only` is non-nil, scenes not in it
// are skipped (the confirm pass re-samples just the flagged scenes). A per-scene
// extraction failure is logged and that frame is dropped (fail-open).
func (o *Orchestrator) extractQAFramesAt(clipID, mp4Path string, scenes []agent.GeneratedScene, frac float64, only map[int]bool) []agent.QAFrame {
	durs := make([]float64, len(scenes))
	for i, s := range scenes {
		durs[i] = s.DurationSeconds
	}
	mids := o.qaFrameTimestamps(mp4Path, durs, frac)
	targets := qaFrameTargets(durs)
	frames := make([]agent.QAFrame, 0, len(scenes))
	for i, s := range scenes {
		if i >= len(mids) {
			break // sampler returned no usable timestamps (missing durations + probe fail) — fail-open
		}
		if !targets[i] {
			continue // zero-duration scene: sampling it would hit a transition boundary
		}
		if only != nil && !only[s.SceneNumber] {
			continue
		}
		outPath := filepath.Join(filepath.Dir(mp4Path), fmt.Sprintf("qa-scene%d-%02.0f.png", s.SceneNumber, frac*100))
		if err := o.producer.FFmpeg().ExtractFrameAt(mp4Path, outPath, mids[i]); err != nil {
			log.Printf("visualqa: clip %s scene %d frame extract failed (skip): %v", clipID, s.SceneNumber, err)
			continue
		}
		png, err := os.ReadFile(outPath)
		os.Remove(outPath) // bytes are in memory now; don't leave QA frame PNGs on disk
		if err != nil {
			log.Printf("visualqa: clip %s scene %d frame read failed (skip): %v", clipID, s.SceneNumber, err)
			continue
		}
		frames = append(frames, agent.QAFrame{
			SceneNumber:  s.SceneNumber,
			PNG:          png,
			OnScreenText: s.OnScreenText,
			VoiceText:    s.VoiceText,
		})
	}
	return frames
}

// extractQAFrames is the first-pass extractor (qaSceneFrac, all scenes).
func (o *Orchestrator) extractQAFrames(clipID, mp4Path string, scenes []agent.GeneratedScene) []agent.QAFrame {
	return o.extractQAFramesAt(clipID, mp4Path, scenes, qaSceneFrac, nil)
}
```

(Note the frame filename now includes the frac so first-pass and confirm-pass files never collide.)

- [ ] **Step 3: Wire the confirm pass in renderAndFinalize**

In the QA block, replace:

```go
		qaRes := o.visualQAAgent.Review(ctx, agent.VisualQAInput{
			Question: q.Question,
			Frames:   frames,
			Fast:     producer.PipelineFastEnabled(),
		}, qaCfg)
		if wErr := o.visualQARepo.Create(ctx, clipID, qaRes.Passed, agent.MarshalVerdicts(qaRes.Verdicts)); wErr != nil {
```

with:

```go
		qaRes := o.visualQAAgent.Review(ctx, agent.VisualQAInput{
			Question: q.Question,
			Frames:   frames,
			Fast:     producer.PipelineFastEnabled(),
		}, qaCfg)
		if !qaRes.Passed {
			// Two-strike confirm: re-sample every flagged scene later in the scene
			// (past the entrance animation, on a different caption phrase) and
			// re-judge. A scene only stays failed when BOTH frames show the defect.
			flagged := make(map[int]bool)
			for _, v := range qaRes.Verdicts {
				if !v.OK {
					flagged[v.SceneNumber] = true
				}
			}
			confirmFrames := o.extractQAFramesAt(clipID, result.LocalVideo916Path, scenes, qaRecheckSceneFrac, flagged)
			confirmRes := o.visualQAAgent.Review(ctx, agent.VisualQAInput{
				Question: q.Question,
				Frames:   confirmFrames,
				Fast:     producer.PipelineFastEnabled(),
			}, qaCfg)
			qaRes = agent.ConfirmMerge(qaRes, confirmRes)
			log.Printf("visualqa: clip %s confirm pass done — %d scene(s) rechecked, passed=%v",
				clipID, len(confirmFrames), qaRes.Passed)
		}
		if wErr := o.visualQARepo.Create(ctx, clipID, qaRes.Passed, agent.MarshalVerdicts(qaRes.Verdicts)); wErr != nil {
```

- [ ] **Step 4: Build and run the orchestrator + agent packages**

Run: `go build ./... && go test ./internal/orchestrator/ ./internal/agent/`
Expected: build OK, tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/orchestrator/orchestrator.go
git commit -m "feat(qa): two-strike confirm pass — re-sample flagged scenes at 85%

First-pass flags re-judged on a second frame later in the scene; a scene
fails only if both frames show the defect. Kills karaoke-caption and
entrance-animation false positives without losing real overflow defects.

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 6: Migration 051 — QA prompt understands captions/animation/on_screen_text

**Files:**
- Create: `migrations/051_visual_qa_caption_context.sql`

**Interfaces:**
- Consumes: `agent_configs` row `agent_name='visual_qa'` seeded by migration 036, extended by 047 (idempotent-append pattern).
- Produces: updated `system_prompt` / `skills` / `prompt_template` on deploy (Railway auto-migrates).

- [ ] **Step 1: Write the migration**

Create `migrations/051_visual_qa_caption_context.sql`:

```sql
-- 051_visual_qa_caption_context.sql
-- Teach visual_qa three runtime facts that were causing false positives:
-- 1) the bottom caption box is a karaoke subtitle: it reveals the narration one
--    SHORT PHRASE at a time — a partial phrase is normal, not truncated text;
-- 2) on_screen_text is scene context (what the scene should convey), NOT a
--    verbatim spec — the screen renders from structured Content/VoiceText;
-- 3) a frame may be captured mid entrance/exit animation — judge only defects
--    that are static (e.g. a fully-settled headline overflowing its box).

UPDATE agent_configs SET
  system_prompt = system_prompt || E'\n\nข้อเท็จจริงของระบบ render (สำคัญมาก):\n- กล่องแคปชั่นล่างสุด (กรอบขอบส้ม) คือซับไตเติลคาราโอเกะ: แสดงบทพากย์ทีละ "วลีสั้นๆ" ไม่ใช่ประโยคเต็ม — วลีสั้น/ขึ้นต้นกลางประโยค = ปกติ ห้ามตีความว่า "ข้อความถูกตัด/อ่านไม่ครบ".\n- on_screen_text คือ "สาระที่ซีนควรสื่อ" ไม่ใช่ข้อความที่ต้องปรากฏคำต่อคำ — ห้ามตั้ง ok=false เพียงเพราะข้อความบนจอไม่ตรง on_screen_text.\n- เฟรมอาจถูกจับระหว่างอนิเมชันเข้า/ออก: องค์ประกอบที่กำลังเลื่อน/จาง/ยังไม่นิ่ง = ปกติ. ตั้ง ok=false เฉพาะตำหนิที่ "นิ่งค้าง" เช่น หัวข้อหลักล้นกรอบ/ถูกครอปทั้งที่แสดงเต็มที่แล้ว.',
  skills = skills || E'\n- แคปชั่นคาราโอเกะล่างจอขึ้นทีละวลี — วลีบางส่วน ≠ ข้อความถูกตัด.\n- on_screen_text = context ไม่ใช่ spec คำต่อคำ — ไม่ตรง ≠ พัง.'
WHERE agent_name = 'visual_qa' AND system_prompt NOT LIKE '%คาราโอเกะ%';

-- Reword the per-frame prompt so the model stops treating on_screen_text as a
-- verbatim requirement. replace() is idempotent (no-op once rewritten).
UPDATE agent_configs SET
  prompt_template = replace(prompt_template,
    E'ข้อความบนจอที่ "ควร" จะเห็น (on_screen_text): ',
    'สาระที่ซีนนี้ควรสื่อ (context — ไม่จำเป็นต้องตรงคำต่อคำ): ')
WHERE agent_name = 'visual_qa';
```

- [ ] **Step 2: Sanity-check the SQL locally (syntax only)**

Run: `grep -c "UPDATE agent_configs" migrations/051_visual_qa_caption_context.sql`
Expected: `2`
(Real execution happens via the app's auto-migrate on deploy; the seed text being replaced is exactly as in `migrations/036_visual_qa_agent_config.sql` — verify the `replace()` needle matches 036 byte-for-byte before committing.)

- [ ] **Step 3: Commit**

```bash
git add migrations/051_visual_qa_caption_context.sql
git commit -m "feat(qa): migration 051 — QA prompt learns karaoke captions & context-only on_screen_text

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 7: Fail-closed flag for QA infra failures

**Files:**
- Create: `internal/producer/qa_flags.go`
- Test: `internal/producer/qa_flags_test.go` (create)
- Modify: `internal/orchestrator/orchestrator.go` (QA block in `renderAndFinalize`)

**Interfaces:**
- Consumes: env var `QA_FAIL_CLOSED_ENABLED` (Railway service variable; default unset = off).
- Produces: `producer.QAFailClosedEnabled() bool` — orchestrator calls it in two places.

- [ ] **Step 1: Write the failing test**

Create `internal/producer/qa_flags_test.go`:

```go
package producer

import "testing"

// Fail-closed must be OFF by default — flipping publish policy silently on
// deploy is exactly the class of surprise this repo's flags exist to prevent.
func TestQAFailClosedEnabledDefaultOff(t *testing.T) {
	t.Setenv("QA_FAIL_CLOSED_ENABLED", "")
	if QAFailClosedEnabled() {
		t.Error("must default to off")
	}
	t.Setenv("QA_FAIL_CLOSED_ENABLED", "true")
	if !QAFailClosedEnabled() {
		t.Error("'true' must enable")
	}
	t.Setenv("QA_FAIL_CLOSED_ENABLED", "1")
	if QAFailClosedEnabled() {
		t.Error("only the literal 'true' enables (repo flag convention)")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/producer/ -run TestQAFailClosedEnabled -v`
Expected: FAIL with "undefined: QAFailClosedEnabled"

- [ ] **Step 3: Implement the flag**

Create `internal/producer/qa_flags.go`:

```go
package producer

import "os"

// QAFailClosedEnabled flips the visual-QA infrastructure policy from fail-open
// to fail-closed: when the QA gate is enabled but cannot actually inspect the
// clip (agent config fetch failed, or zero frames could be extracted), the clip
// routes to needs_review instead of publishing unseen. Off (default) keeps the
// historical fail-open behavior. Per-scene vision errors stay fail-open either
// way — only "QA saw nothing at all" is gated here.
func QAFailClosedEnabled() bool { return os.Getenv("QA_FAIL_CLOSED_ENABLED") == "true" }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/producer/ -run TestQAFailClosedEnabled -v`
Expected: PASS

- [ ] **Step 5: Wire into renderAndFinalize**

In `internal/orchestrator/orchestrator.go`, the QA block currently opens with:

```go
	// Visual QA is an optional gate; disabled/absent or any infra error => fail-OPEN (status stays "ready", never blocks publish).
	if qaCfg, qErr := o.agentsRepo.GetByName(ctx, "visual_qa"); qErr == nil && qaCfg.Enabled && result.LocalVideo916Path != "" {
		o.tracker.StartStep("visual_qa")
		frames := o.extractQAFrames(clipID, result.LocalVideo916Path, scenes)
```

Replace with (and note the new `else if` after the block's closing brace):

```go
	// Visual QA gate. Historically fail-OPEN on any infra error; with
	// QA_FAIL_CLOSED_ENABLED=true a clip QA couldn't see at all (config fetch
	// error / zero frames) routes to needs_review instead of publishing unseen.
	if qaCfg, qErr := o.agentsRepo.GetByName(ctx, "visual_qa"); qErr == nil && qaCfg.Enabled && result.LocalVideo916Path != "" {
		o.tracker.StartStep("visual_qa")
		frames := o.extractQAFrames(clipID, result.LocalVideo916Path, scenes)
		if len(frames) == 0 && producer.QAFailClosedEnabled() && status == "ready" {
			status = "needs_review"
			log.Printf("visualqa: clip %s produced no QA frames — fail-closed → needs_review", clipID)
		}
```

and after the QA block's closing `}` (right before the `// A hyperframes layout-inspector flag ...` comment) add:

```go
	} else if qErr != nil && producer.QAFailClosedEnabled() && status == "ready" {
		status = "needs_review"
		log.Printf("visualqa: clip %s config unavailable (%v) — fail-closed → needs_review", clipID, qErr)
	}
```

(The existing `if` closes with a plain `}` today — attach this `else if` to it. `qErr != nil` distinguishes a real fetch error from "QA disabled", which must stay untouched.)

- [ ] **Step 6: Build + run tests**

Run: `go build ./... && go test ./internal/producer/ ./internal/orchestrator/`
Expected: build OK, PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/producer/qa_flags.go internal/producer/qa_flags_test.go internal/orchestrator/orchestrator.go
git commit -m "feat(qa): QA_FAIL_CLOSED_ENABLED — clips QA cannot inspect route to needs_review

Off by default (flag pattern like RENDER_ERROR_GATE_ENABLED). When on, a QA
config fetch error or zero extracted frames no longer publishes unseen.

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 8: SetAutoReviewHeld status guard (race fix)

**Files:**
- Modify: `internal/repository/clips.go:193-197`

**Interfaces:**
- Consumes/Produces: `SetAutoReviewHeld(ctx, id)` signature unchanged; it now no-ops on clips that left `needs_review` while the auto-review judge was running (observed in prod: a hold written onto an already-published clip).

- [ ] **Step 1: Apply the guard**

Replace:

```go
// SetAutoReviewHeld marks a clip as held so the auto-review tick stops re-judging it.
func (r *ClipsRepo) SetAutoReviewHeld(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE clips SET auto_review_held = TRUE, updated_at = NOW() WHERE id = $1`, id)
	return err
}
```

with:

```go
// SetAutoReviewHeld marks a clip as held so the auto-review tick stops re-judging
// it. Guarded to status='needs_review': the judge snapshots its batch before a
// slow vision call, so by the time it decides "hold" a human may already have
// approved/published the clip — a late hold must not clobber that (seen in prod:
// a published clip carrying auto_review_held=TRUE).
func (r *ClipsRepo) SetAutoReviewHeld(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE clips SET auto_review_held = TRUE, updated_at = NOW()
		 WHERE id = $1 AND status = 'needs_review'`, id)
	return err
}
```

- [ ] **Step 2: Build + run repo-adjacent tests**

Run: `go build ./... && go test ./internal/agent/ -run TestAutoReview -v && go test ./internal/orchestrator/`
Expected: build OK, PASS. (No DB-backed repo tests exist in this repo — the guard is verified by review + prod behavior; keep the change surgical.)

- [ ] **Step 3: Commit**

```bash
git add internal/repository/clips.go
git commit -m "fix(auto-review): guard SetAutoReviewHeld to needs_review (late-hold race)

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 9: PATCH guard — pipeline-only statuses rejected

**Files:**
- Modify: `internal/handler/clips.go` (`Update`, ~line 53)
- Test: `internal/handler/clips_update_guard_test.go` (create)

**Interfaces:**
- Consumes: `models.UpdateClipRequest.Status *string` (json tag `status`), chi router.
- Produces: `PATCH /clips/{id}` returns 400 for `status` ∈ {`published`, `producing`}; all other PATCH behavior unchanged (frontend uses `{status:'ready'}` for human approve — must keep working).

- [ ] **Step 1: Write the failing test**

Create `internal/handler/clips_update_guard_test.go`:

```go
package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// 'published' must only ever be written by the publisher (it's what marks a
// real Zernio upload) and 'producing' only by the orchestrator. A raw PATCH
// setting either would bypass the publish gate entirely.
func TestUpdateRejectsPipelineOnlyStatus(t *testing.T) {
	h := NewClipsHandler(nil) // guard rejects before the repo is touched
	r := chi.NewRouter()
	r.Patch("/clips/{id}", h.Update)

	for _, s := range []string{"published", "producing"} {
		req := httptest.NewRequest(http.MethodPatch, "/clips/abc",
			strings.NewReader(`{"status":"`+s+`"}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("status %q: got %d, want 400", s, w.Code)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/handler/ -run TestUpdateRejectsPipelineOnlyStatus -v`
Expected: FAIL — nil-repo panic or non-400 (the handler currently forwards straight to the repo).

- [ ] **Step 3: Implement the guard**

In `internal/handler/clips.go`, inside `Update` right after the decode error check, add:

```go
	// 'published' is written only by the publisher (it records a real Zernio
	// upload) and 'producing' only by the orchestrator — a raw PATCH to either
	// would bypass the publish gate / production tracker.
	if req.Status != nil && (*req.Status == "published" || *req.Status == "producing") {
		writeJSON(w, http.StatusBadRequest, models.APIResponse{
			Error: "status '" + *req.Status + "' ตั้งได้โดย pipeline เท่านั้น"})
		return
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/handler/ -run TestUpdateRejectsPipelineOnlyStatus -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/handler/clips.go internal/handler/clips_update_guard_test.go
git commit -m "fix(api): reject PATCH status=published/producing (pipeline-only transitions)

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 10: Full verification + deploy notes

**Files:**
- None (verification only)

- [ ] **Step 1: Full build + test + vet**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: all PASS, no vet findings in touched packages.

- [ ] **Step 2: Gofmt check on touched files**

Run: `gofmt -l internal/`
Expected: empty output.

- [ ] **Step 3: Review the diff against the spec**

Run: `git log --oneline master@{u}..HEAD 2>/dev/null || git log --oneline -12`
Confirm one commit per task (spec + plan docs + tasks 1-9).

- [ ] **Step 4: Report deploy/rollout checklist to the user (do NOT deploy without asking)**

- Push to master → Railway auto-deploys backend+frontend, migration 051 auto-applies.
- After deploy: **do not** set `QA_FAIL_CLOSED_ENABLED` yet — watch one produce cycle first.
- Eyeball 1 fresh clip end-to-end (fonts/stat boxes on every theme) — Track A is CSS/JS; the unit tests only guard the strings, not the pixels.
- Watch `visual_qa` pass rate for ~2-3 days (expect fail rate to drop from ~70% toward 20-30%).
- Then flip `QA_FAIL_CLOSED_ENABLED=true` on the Railway backend service.
- Rollback: revert the commits (Tracks A/B have no flag); migration 051 is append-only — reverse with an UPDATE trimming the appended text; Track C1 = unset the flag.

---

## Self-review notes (already applied)

- **Spec coverage:** A1/A2 → Tasks 1-2; B1 → Tasks 4-5; B2 → Task 6; B3 → Task 3; C1 → Task 7; C2 → Task 8; C3 → Task 9. Human-Override behavior intentionally unchanged (out of scope).
- **Type consistency:** `ConfirmMerge(first, confirm VisualQAResult) VisualQAResult` defined in Task 4, called with the same signature in Task 5. `extractQAFramesAt(..., frac float64, only map[int]bool)` defined and called consistently. Constants `qaEntranceGuardSec`/`qaExitGuardSec`/`qaRecheckSceneFrac` defined once (Tasks 3/5).
- **Ordering:** Task 3 before Task 5 (the 0.85 recheck relies on the exit guard). Task 1 before Task 2 (Task 2's test file appends to Task 1's).
