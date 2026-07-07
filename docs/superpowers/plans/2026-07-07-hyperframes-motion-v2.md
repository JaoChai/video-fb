# Hyperframes Motion v2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add three flag-gated motion upgrades to the hyperframes render template — mid-scene parallax drift, entrance variety, and stat count-up — so clips feel less static.

**Architecture:** One env flag `SCENE_MOTION_V2_ENABLED` flows through Go (`ScenesParams.MotionV2` → `scenesTemplateData.MotionV2` → template `const MOTION_V2`). The entrance variant is chosen by a pure, unit-tested Go helper and carried per-scene as `sc.entrance`. Parallax and count-up are template JS, guarded at runtime by `if(MOTION_V2)`. The JS is always present in the template (so it is assertable by the existing render tests); only its execution is gated, so flag-off is byte-for-byte the current runtime behavior.

**Tech Stack:** Go, `html/template`, GSAP 3 (vendored). Existing test seam: `RenderCompositionScenes` renders the template to HTML and `composition_scenes_test.go` asserts on the string (via `assertRenderContains`), catching template parse/execute errors and letting us assert new JS tokens.

## Global Constraints

- Flag: `SCENE_MOTION_V2_ENABLED`, env, default off — `os.Getenv("SCENE_MOTION_V2_ENABLED") == "true"` (matches `AudioMotionEnabled` at `internal/producer/audio.go:17`).
- Flag off ⇒ current runtime behavior exactly (all new tweens are behind `if(MOTION_V2)`).
- No schema changes / no migration. Only `internal/producer/*.go` + the one template.
- Template must never leak `{{` directives — `TestRenderCompositionScenes_9x16` (`composition_scenes_test.go:54`) fails if it does.
- Renders must be deterministic (no `Math.random`, no wall-clock): entrance variant is index-based; count-up/parallax are GSAP timeline tweens (seek-safe).
- Go commands: run with dangerouslyDisableSandbox — `go build`/`go test`/`go vet` fail under the default sandbox in this repo.
- Every commit message ends with: `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`

---

### Task 1: Go plumbing — flag, entrance helper, wiring

**Files:**
- Modify: `internal/producer/audio.go` (add `SceneMotionV2Enabled`)
- Modify: `internal/producer/scene_adapter.go` (add `entranceForScene`, assign `Content.Entrance`)
- Modify: `internal/producer/composition_types.go` (add `SceneContent.Entrance`, `ScenesParams.MotionV2`)
- Modify: `internal/producer/composition.go` (add `scenesTemplateData.MotionV2`, populate it)
- Modify: `internal/producer/producer.go` (set `params.MotionV2`)
- Test: `internal/producer/scene_adapter_test.go` (add `TestEntranceForScene*`)

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `func SceneMotionV2Enabled() bool`
  - `func entranceForScene(idx int) string` → `"punch"` | `"rise"` | `"slide"`
  - `SceneContent.Entrance string` (JSON key `entrance`)
  - `ScenesParams.MotionV2 bool`, `scenesTemplateData.MotionV2 bool`

- [ ] **Step 1: Write the failing test**

Add to `internal/producer/scene_adapter_test.go`:

```go
func TestEntranceForSceneRotates(t *testing.T) {
	got := make([]string, 0, 7)
	for i := 0; i < 7; i++ {
		got = append(got, entranceForScene(i))
	}
	want := []string{"punch", "rise", "slide", "punch", "rise", "slide", "punch"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("idx %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestEntranceForSceneNoConsecutiveRepeat(t *testing.T) {
	for i := 1; i < 30; i++ {
		if entranceForScene(i) == entranceForScene(i-1) {
			t.Fatalf("idx %d and %d have the same entrance %q", i-1, i, entranceForScene(i))
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/producer/ -run TestEntranceForScene -v`
Expected: FAIL — `undefined: entranceForScene`.

- [ ] **Step 3: Add the helper**

In `internal/producer/scene_adapter.go`, near `speedForLayout` (around line 95), add:

```go
// entranceForScene picks a rotating entrance geometry (punch/rise/slide) so
// consecutive scenes never enter identically. Index-based, so a render is
// deterministic. Scene 0 (the hook) gets "punch" for a snappy open.
func entranceForScene(idx int) string {
	variants := []string{"punch", "rise", "slide"}
	return variants[((idx % 3) + 3) % 3]
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/producer/ -run TestEntranceForScene -v`
Expected: PASS (both tests).

- [ ] **Step 5: Add the flag**

In `internal/producer/audio.go`, below `AudioMotionEnabled` (line 17), add:

```go
// SceneMotionV2Enabled turns on the v2 scene motion (mid-scene parallax drift,
// entrance variety, stat count-up). Off → current motion behavior.
func SceneMotionV2Enabled() bool { return os.Getenv("SCENE_MOTION_V2_ENABLED") == "true" }
```

(Confirm `os` is already imported in `audio.go` — it is, used by `AudioMotionEnabled`.)

- [ ] **Step 6: Add the struct fields**

In `internal/producer/composition_types.go`, add to `SceneContent` (after `Speed` at line 46):

```go
	Entrance string `json:"entrance,omitempty"` // punch|rise|slide — entrance geometry (MOTION_V2)
```

And to `ScenesParams` (after `AudioMotion bool` at line 116):

```go
	MotionV2 bool // enable v2 scene motion (parallax drift, entrance variety, count-up)
```

In `internal/producer/composition.go`, add to the `scenesTemplateData` struct (after `AudioMotion bool` at line 32):

```go
	MotionV2 bool
```

And populate it in the `data := scenesTemplateData{...}` literal (after `AudioMotion: p.AudioMotion,` at line 138):

```go
		MotionV2: p.MotionV2,
```

- [ ] **Step 7: Assign the entrance per scene**

In `internal/producer/scene_adapter.go`, inside `buildSceneSpecs`'s loop (`for i := 0; i < n; i++`, line 58), after the `SceneSpec` is built with `Content: buildSceneContent(s, b)`, set the entrance from the loop index. Locate the spec assignment (line ~74–82) and, immediately after the spec literal that ends with `Content: buildSceneContent(s, b),`, add a line setting the entrance. If the loop appends to a slice `specs[i] = SceneSpec{...}`, then after that statement add:

```go
		specs[i].Content.Entrance = entranceForScene(i)
```

(Read lines 58–90 of `scene_adapter.go` first to match the exact variable name — it may be `specs[i] = ...` or `out = append(out, ...)`. Set `.Content.Entrance = entranceForScene(i)` on the element that was just built for index `i`.)

- [ ] **Step 8: Set the flag on params**

In `internal/producer/producer.go`, in `AssembleHyperframes916` where `params := ScenesParams{...}` is built (line 365), add after the struct literal (near the `if AudioMotionEnabled()` block at line 379):

```go
	params.MotionV2 = SceneMotionV2Enabled()
```

- [ ] **Step 9: Verify build + tests**

Run: `go test ./internal/producer/ -run 'TestEntranceForScene|TestRenderCompositionScenes' -v` then `go build ./...`
Expected: PASS, build clean. (The render tests still pass — `entrance` now appears in the marshaled ScenesJSON but no assertion breaks.)

- [ ] **Step 10: Commit**

```bash
git add internal/producer/audio.go internal/producer/scene_adapter.go internal/producer/scene_adapter_test.go internal/producer/composition_types.go internal/producer/composition.go internal/producer/producer.go
git commit -m "feat(motion): Go plumbing for scene motion v2 (flag, entrance variant)

Adds SceneMotionV2Enabled flag, entranceForScene rotation helper (punch/rise/
slide, index-based, no consecutive repeat), SceneContent.Entrance, and MotionV2
wiring through ScenesParams → scenesTemplateData. Template not yet using them.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: Template — MOTION_V2 const + entrance variety

**Files:**
- Modify: `internal/producer/templates/layout_multi_scene.html.tmpl`
- Test: `internal/producer/composition_scenes_test.go` (add `TestRenderCompositionScenes_MotionV2`)

**Interfaces:**
- Consumes: `scenesTemplateData.MotionV2` (Task 1), `sc.entrance` in ScenesJSON (Task 1).
- Produces: template JS `const MOTION_V2`, JS helper `contentEntrance(sc, motionUp)`.

- [ ] **Step 1: Write the failing test**

Add to `internal/producer/composition_scenes_test.go`:

```go
func TestRenderCompositionScenes_MotionV2(t *testing.T) {
	on := sampleScenesParams("9:16")
	on.MotionV2 = true
	assertRenderContains(t, on, "const MOTION_V2 = true", "function contentEntrance")

	off := sampleScenesParams("9:16")
	off.MotionV2 = false
	s := assertRenderContains(t, off, "const MOTION_V2 = false")
	// helper JS is always present (runtime-gated), only the const value differs:
	if !strings.Contains(s, "function contentEntrance") {
		t.Error("contentEntrance helper should always be emitted (runtime-gated by MOTION_V2)")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/producer/ -run TestRenderCompositionScenes_MotionV2 -v`
Expected: FAIL — output lacks `const MOTION_V2` / `contentEntrance`.

- [ ] **Step 3: Add the MOTION_V2 const**

In `internal/producer/templates/layout_multi_scene.html.tmpl`, after the `MOTION_UP` const (line 166), add:

```html
      const MOTION_V2 = {{if .MotionV2}}true{{else}}false{{end}};
```

- [ ] **Step 4: Add the contentEntrance helper**

In the template's `<script>`, above the per-scene animation loop (before `SCENES.forEach((sc,idx)=>{` at line 235), add:

```html
      // contentEntrance returns the from/to vars for a scene's content entrance.
      // MOTION_V2 off → the legacy rise (y-based). MOTION_V2 on → per-scene
      // geometry from sc.entrance (punch/rise/slide); slide alternates direction.
      function contentEntrance(sc, motionUp){
        const base = motionUp ? 60 : 46;
        if(!MOTION_V2){ return {from:{y:base,opacity:0}, to:{y:0,opacity:1}}; }
        const v = sc.entrance || "rise";
        if(v==="slide"){ const dx = (sc.scene % 2 === 0) ? -80 : 80; return {from:{x:dx,opacity:0}, to:{x:0,opacity:1}}; }
        if(v==="punch"){ return {from:{scale:0.9,opacity:0}, to:{scale:1,opacity:1}}; }
        return {from:{y:base,opacity:0}, to:{y:0,opacity:1}};
      }
```

- [ ] **Step 5: Apply it in both entrance branches**

Replace the content-entrance tween in the `MOTION_UP` branch (line 250–252):

```html
          if(content){
            tl.fromTo(content,{y:60,opacity:0},{y:0,opacity:1,duration:durIn,ease:ENTRANCE_EASE},sc.start+0.08);
          }
```

with:

```html
          if(content){
            const ce=contentEntrance(sc,true);
            tl.fromTo(content,ce.from,Object.assign({},ce.to,{duration:durIn,ease:ENTRANCE_EASE}),sc.start+0.08);
          }
```

And in the legacy branch (line 258–260):

```html
          if(content){
            tl.fromTo(content,{y:46,opacity:0},{y:0,opacity:1,duration:contentDurIn,ease:"power3.out"},sc.start+0.1);
          }
```

with:

```html
          if(content){
            const ce=contentEntrance(sc,false);
            tl.fromTo(content,ce.from,Object.assign({},ce.to,{duration:contentDurIn,ease:"power3.out"}),sc.start+0.1);
          }
```

(When `MOTION_V2` is false, `contentEntrance` returns the same `{y:base}` literals as before → identical behavior.)

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/producer/ -run 'TestRenderCompositionScenes' -v`
Expected: PASS (the new test + all existing render tests, including the no-`{{`-leak check).

- [ ] **Step 7: Commit**

```bash
git add internal/producer/templates/layout_multi_scene.html.tmpl internal/producer/composition_scenes_test.go
git commit -m "feat(motion): entrance variety (punch/rise/slide) behind MOTION_V2

Adds const MOTION_V2 + contentEntrance() helper; the content entrance now uses
per-scene geometry from sc.entrance when the flag is on. Flag off returns the
legacy y-based rise, unchanged.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Template — mid-scene parallax drift

**Files:**
- Modify: `internal/producer/templates/layout_multi_scene.html.tmpl`
- Test: `internal/producer/composition_scenes_test.go` (extend `TestRenderCompositionScenes_MotionV2` or add one)

**Interfaces:**
- Consumes: `MOTION_V2` const (Task 2).
- Produces: a parallax `tl.to(content, {y:-12 ...})` gated by `MOTION_V2`.

- [ ] **Step 1: Write the failing test**

Add to `internal/producer/composition_scenes_test.go`:

```go
func TestRenderCompositionScenes_ParallaxDrift(t *testing.T) {
	p := sampleScenesParams("9:16")
	p.MotionV2 = true
	assertRenderContains(t, p, "MOTION_V2 && content", "y:-12")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/producer/ -run TestRenderCompositionScenes_ParallaxDrift -v`
Expected: FAIL — no `y:-12` drift in output.

- [ ] **Step 3: Add the drift tween**

In `internal/producer/templates/layout_multi_scene.html.tmpl`, inside the per-scene animation loop, AFTER the per-child stagger block (after line 273's closing `}` for the `if(content){ ... }` block, and before `tl.set(sceneEl,{opacity:0},sc.end);` at line 274), add:

```html
        // MOTION_V2: after the entrance settles, drift the whole content block
        // slowly upward (opposite the bg ken-burns) so the focal content is never
        // dead-still. Starts after the entrance so it never fights the entrance's
        // own y/scale tween.
        if(MOTION_V2 && content){
          const driftStart = sc.start + 0.7;
          const driftDur = sc.end - driftStart;
          if(driftDur > 0.3){
            tl.to(content,{y:-12,duration:driftDur,ease:"none"},driftStart);
          }
        }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/producer/ -run 'TestRenderCompositionScenes' -v`
Expected: PASS (drift present; existing tests unaffected).

- [ ] **Step 5: Commit**

```bash
git add internal/producer/templates/layout_multi_scene.html.tmpl internal/producer/composition_scenes_test.go
git commit -m "feat(motion): mid-scene parallax drift behind MOTION_V2

After the entrance settles, the content block drifts y:0→-12px over the rest of
the scene (ease none), opposite the bg ken-burns, so focal content isn't frozen.
Starts after the entrance to avoid a same-property tween conflict.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: Template — stat count-up

**Files:**
- Modify: `internal/producer/templates/layout_multi_scene.html.tmpl`
- Test: `internal/producer/composition_scenes_test.go`

**Interfaces:**
- Consumes: `MOTION_V2` const (Task 2), the stat DOM (`sc.stat`, `sc.unit`).
- Produces: JS `parseStatNumber`, `formatStat`, `.stat-num` span, and a count-up tween.

- [ ] **Step 1: Write the failing test**

Add to `internal/producer/composition_scenes_test.go`:

```go
func TestRenderCompositionScenes_CountUp(t *testing.T) {
	p := sampleScenesParams("9:16")
	p.MotionV2 = true
	assertRenderContains(t, p, "function parseStatNumber", "stat-num")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/producer/ -run TestRenderCompositionScenes_CountUp -v`
Expected: FAIL — no `parseStatNumber` / `stat-num` in output.

- [ ] **Step 3: Add the parse/format helpers**

In the template `<script>`, near `contentEntrance` (added in Task 2), add:

```html
      // parseStatNumber extracts the leading number of a stat string ("80",
      // "1,200", "3.5") → {value, decimals, grouped}, or null if non-numeric.
      function parseStatNumber(s){
        const m = String(s||"").match(/^\s*([\d.,]+)/);
        if(!m) return null;
        const raw = m[1];
        const grouped = raw.indexOf(",") >= 0;
        const clean = raw.replace(/,/g,"");
        const value = parseFloat(clean);
        if(!isFinite(value)) return null;
        const dot = clean.indexOf(".");
        const decimals = dot >= 0 ? clean.length - dot - 1 : 0;
        return {value:value, decimals:decimals, grouped:grouped};
      }
      function formatStat(v, meta){
        let s = meta.decimals > 0 ? v.toFixed(meta.decimals) : String(Math.round(v));
        if(meta.grouped){ const parts = s.split("."); parts[0] = parts[0].replace(/\B(?=(\d{3})+(?!\d))/g, ","); s = parts.join("."); }
        return s;
      }
```

- [ ] **Step 4: Build the stat number as a separate span (DOM pass)**

In the DOM builder's `stat` branch (line 203–209), replace:

```html
        else if(sc.type==="stat"){
          const card=el("div","card");
          card.appendChild(el("div","stat",(sc.stat||"")+(sc.unit?'<span class="unit">'+sc.unit+'</span>':'')));
          card.appendChild(el("div","stat-label",sc.statLabel));
          c.appendChild(card);
          if(sc.chips) c.appendChild(chipsBlock(sc.chips));
        }
```

with:

```html
        else if(sc.type==="stat"){
          const card=el("div","card");
          const meta=MOTION_V2?parseStatNumber(sc.stat):null;
          const numHTML=meta?'<span class="stat-num">0</span>':(sc.stat||"");
          card.appendChild(el("div","stat",numHTML+(sc.unit?'<span class="unit">'+sc.unit+'</span>':'')));
          card.appendChild(el("div","stat-label",sc.statLabel));
          c.appendChild(card);
          if(sc.chips) c.appendChild(chipsBlock(sc.chips));
        }
```

(When `MOTION_V2` is off, or the stat is non-numeric, `meta` is null → the number renders statically exactly as before.)

- [ ] **Step 5: Add the count-up tween (animation pass)**

In the per-scene animation loop, inside the `if(MOTION_V2 && content){ ... }` region you added in Task 3 (or as its own `if`), add a count-up for stat scenes. Place this right after the parallax drift block:

```html
        if(MOTION_V2 && sc.type==="stat"){
          const numEl = sceneEl.querySelector(".stat-num");
          const meta = parseStatNumber(sc.stat);
          if(numEl && meta){
            const proxy = {v:0};
            tl.to(proxy,{v:meta.value,duration:Math.min(1.1,Math.max(0.5,span*0.5)),ease:"power1.out",
              onUpdate:function(){ numEl.textContent = formatStat(proxy.v, meta); }},sc.start+0.3);
          }
        }
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/producer/ -run 'TestRenderCompositionScenes' -v`
Expected: PASS (count-up present; existing tests unaffected).

- [ ] **Step 7: Commit**

```bash
git add internal/producer/templates/layout_multi_scene.html.tmpl internal/producer/composition_scenes_test.go
git commit -m "feat(motion): stat count-up behind MOTION_V2

Numeric stat values animate 0→N (GSAP proxy tween, seek-safe) while keeping the
unit span; non-numeric stats and flag-off render statically as before.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: Verify, render-eyeball, simplify

**Files:** none (verification), plus `/simplify` fixups if any.

- [ ] **Step 1: Full suite + build + vet**

Run: `go test ./...` then `go build ./...` then `go vet ./internal/producer/`
Expected: all PASS/clean.

- [ ] **Step 2: Confirm flag default off**

Run: `grep -rn 'SCENE_MOTION_V2_ENABLED' internal/`
Expected: appears only in `SceneMotionV2Enabled` (`os.Getenv(...) == "true"`) and tests — no default-true wiring.

- [ ] **Step 3: Render a real clip with the flag ON and eyeball**

This is the primary aesthetic gate (template JS has no unit test for motion feel). Produce one clip end-to-end with `SCENE_MOTION_V2_ENABLED=1` — either via the manual smoke `TestAssembleHyperframes916_Smoke` (needs `HF_RENDER=1` + `DATABASE_URL` + Node/Chrome) or a local render harness. Extract several frames per scene (`ffmpeg -ss`), then confirm:
- a stat scene's number differs across its frames (count-up runs);
- `.scene-content` shifts position across a scene's frames (parallax);
- consecutive scenes enter differently (entrance variety);
- no overflow / blank / broken layout.

If the dev environment cannot render (no DB keys / no Chrome), STOP and report: eyeball must then happen on one prod clip with the flag on. Do NOT claim the motion works without having observed a render.

- [ ] **Step 4: Render with the flag OFF — regression**

Render (or at least `RenderCompositionScenes` with `MotionV2:false`) and confirm the output matches pre-change behavior (const `MOTION_V2 = false`, legacy entrance literals, no drift/count-up executed).

- [ ] **Step 5: /simplify the diff**

Invoke `/simplify` on the branch diff. Apply reductions, re-run `go test ./...`, commit any changes as `refactor: simplify motion v2 diff`.

---

## Self-Review

**Spec coverage:**
- Mid-scene parallax drift → Task 3. ✓
- Entrance variety (rise/slide/punch rotated, Go-assigned) → Task 1 (helper + assign) + Task 2 (apply). ✓
- Stat count-up (0→N, preserve unit, graceful non-numeric) → Task 4. ✓
- Single flag `SCENE_MOTION_V2_ENABLED`, default off, off = unchanged → Task 1 (flag) + all template tasks runtime-gate on `MOTION_V2`. ✓
- No migration → confirmed (no `migrations/` files). ✓
- Verification via Go render tests + eyeball → Tasks 2/3/4 assert JS tokens; Task 5 eyeball. ✓

**Placeholder scan:** No TBD/TODO. Every code step shows complete code. Task 1 Step 7 asks the implementer to read the loop to match the exact slice-variable name before assigning `.Content.Entrance` — that is a real disambiguation instruction (the loop's write form isn't fixed by the plan), not a placeholder; the assignment line itself is given verbatim.

**Type consistency:**
- `entranceForScene(idx int) string` returns `punch/rise/slide`; consumed by `sc.entrance` in `contentEntrance` (Task 2) — values match. ✓
- `MotionV2` field name identical across `ScenesParams`, `scenesTemplateData`, and the `data` literal (Task 1) and the template `.MotionV2` (Tasks 2). ✓
- `SceneContent.Entrance` JSON key `entrance` matches `sc.entrance` read in the template. ✓
- `parseStatNumber`/`formatStat`/`.stat-num` names consistent between Task 4 Steps 3–5. ✓
- Flag string `SCENE_MOTION_V2_ENABLED` identical in `SceneMotionV2Enabled` and the Global Constraints. ✓
