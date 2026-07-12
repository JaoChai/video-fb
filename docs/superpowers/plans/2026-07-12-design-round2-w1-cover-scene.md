# Design Round 2 · Workstream 1 — Cover Scene Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** ทำให้ **เฟรม 0** ของคลิป (ที่ Reels/Shorts/TikTok ดึงไปเป็นปก/preview) แสดงฉากแรกครบสมบูรณ์ทันที แทนที่จะเป็นจอ navy ว่างเปล่า เพื่อดัน 3-second retention → ยอดวิว

**Architecture:** ต่อยอด `layout_multi_scene.html.tmpl` + `ScenesParams` เดิม (แนวทาง A) เพิ่ม flag `COVER_SCENE_ENABLED` ส่งผ่าน `ScenesParams.Cover` → `scenesTemplateData.Cover` → template const `COVER` เมื่อ `COVER` เปิด ฉาก index 0 เริ่มที่ `opacity:1` ตั้งแต่ t=0 (ทั้ง scene wrapper และ content) แล้ว settle ด้วย scale เบาๆ (ไม่มี opacity:0→1 fade) ปิด flag = พฤติกรรมวันนี้เป๊ะ

**Tech Stack:** Go 1.x (`html/template`), GSAP 3.14 (vendored), Hyperframes CLI 0.6.70, render offline (ห้าม CDN / ห้าม non-determinism ใน render JS)

**นี่คือ Plan 1 จาก 4** ของ spec `docs/superpowers/specs/2026-07-12-design-round2-retention-design.md` (W1 Cover · W2 Premium Hero Image · W3 Motion v2 · W4 Composition Variety) แต่ละ workstream เป็น plan อิสระ flag-gated

## Global Constraints

- **Feature flag default OFF:** `COVER_SCENE_ENABLED` อ่านผ่าน `os.Getenv(...) == "true"` เท่านั้น; ปิด = output ปัจจุบันไม่เปลี่ยนแม้แต่ไบต์เดียว
- **Render offline:** GSAP โหลดจาก `assets/gsap.min.js` (bundled) — ห้ามเพิ่ม CDN; **ห้าม** `Math.random()`/`Date.now()`/`new Date()` ใน render JS (trip `non_deterministic_code` lint → render fall back เป็นภาพนิ่ง)
- **ห้ามเขียน `-->` ใน `<script>` ของ template** (`html/template` ตัดบรรทัดที่มี `-->` ใน script → JS syntax error → ทุกฉากค้าง opacity:0 — regression จริงที่เคยเกิด commit 8eb202f)
- **Brand invariants คงเดิม:** badge ADS VANCE, ส้ม accent จุดเดียว, safe-zone, Thai line-height ≥1.32, ห้าม negative letter-spacing
- **Branch:** `feat/design-round2-retention`
- **สั่งเทส Go จาก repo root:** `go test ./internal/producer/...`

---

### Task 1: เพิ่ม flag `COVER_SCENE_ENABLED` + plumb `Cover` ผ่าน params

**Files:**
- Modify: `internal/producer/composition_types.go` (เพิ่ม field `Cover bool` ใน `ScenesParams`, ต่อจากบรรทัด 119 `MotionV2 bool`)
- Modify: `internal/producer/composition.go` (เพิ่ม field `Cover bool` ใน `scenesTemplateData` ~บรรทัด 34, และ map ค่าใน `data := scenesTemplateData{...}` ~บรรทัด 141)
- Modify: `internal/producer/audio.go` (เพิ่ม `CoverSceneEnabled()` ข้าง `SceneMotionV2Enabled()` บรรทัด 21)
- Test: `internal/producer/composition_scenes_test.go`

**Interfaces:**
- Produces: `func CoverSceneEnabled() bool` — อ่าน `os.Getenv("COVER_SCENE_ENABLED") == "true"`
- Produces: `ScenesParams.Cover bool` — เปิด cover behavior ให้ฉาก index 0
- Produces: `scenesTemplateData.Cover bool` — ส่งเข้า template เป็น const `COVER`

- [ ] **Step 1: เขียน test ที่ fail** — cover on ⇒ template มี `const COVER = true` และมี guard `COVER && idx===0`; cover off ⇒ `const COVER = false`

เพิ่มใน `internal/producer/composition_scenes_test.go`:

```go
// COVER_SCENE: the JS const must reflect the ScenesParams.Cover flag, and the
// frame-0 cover branch must be present in the entrance code.
func TestRenderCompositionScenes_Cover(t *testing.T) {
	on := baseScenesParams()
	on.Cover = true
	assertRenderContains(t, on, "const COVER = true", "COVER && idx===0")

	off := baseScenesParams()
	off.Cover = false
	assertRenderContains(t, off, "const COVER = false")
}
```

> หมายเหตุ: `baseScenesParams()` / `assertRenderContains` เป็น helper ที่มีอยู่แล้วในไฟล์นี้ (ใช้โดย `TestRenderCompositionScenes_MotionV2`) — เปิดไฟล์ยืนยันชื่อ helper จริงก่อนใช้ ถ้าชื่อต่างให้ใช้ชื่อจริงในไฟล์

- [ ] **Step 2: รัน test ให้ fail**

Run: `go test ./internal/producer/ -run TestRenderCompositionScenes_Cover -v`
Expected: FAIL — `const COVER` ยังไม่มีใน output (template ยังไม่รู้จัก `.Cover`)

- [ ] **Step 3: เพิ่ม field + flag (ยังไม่แตะ template)**

ใน `internal/producer/composition_types.go` ต่อจากบรรทัด `MotionV2 bool // enable v2 scene motion ...`:

```go
	Cover bool // enable frame-0 cover behavior for scene index 0 (COVER_SCENE_ENABLED)
```

ใน `internal/producer/composition.go` — ใน struct `scenesTemplateData` ต่อจาก `MotionV2 bool` (บรรทัด 34):

```go
	Cover          bool
```

และใน `data := scenesTemplateData{` ต่อจาก `MotionV2: p.MotionV2,` (บรรทัด 141):

```go
		Cover:           p.Cover,
```

ใน `internal/producer/audio.go` ต่อจาก `SceneMotionV2Enabled()`:

```go
// CoverSceneEnabled turns on the frame-0 cover: scene index 0 renders fully at
// opacity:1 from frame 0 (the poster the platform grabs) instead of fading in
// from a blank navy frame. Off ⇒ today's behavior.
func CoverSceneEnabled() bool { return os.Getenv("COVER_SCENE_ENABLED") == "true" }
```

> ยืนยัน `audio.go` import `"os"` อยู่แล้ว (ใช้โดย `SceneMotionV2Enabled`) — ไม่ต้องเพิ่ม import

- [ ] **Step 4: เพิ่ม `const COVER` ใน template (ยังไม่แก้ entrance logic)**

ใน `internal/producer/templates/layout_multi_scene.html.tmpl` ต่อจากบรรทัด 170 `const MOTION_V2 = {{if .MotionV2}}true{{else}}false{{end}};`:

```javascript
      const COVER = {{if .Cover}}true{{else}}false{{end}};
```

- [ ] **Step 5: รัน test — `const COVER = true/false` ผ่าน, ส่วน `COVER && idx===0` ยัง fail**

Run: `go test ./internal/producer/ -run TestRenderCompositionScenes_Cover -v`
Expected: ยัง FAIL ที่ substring `COVER && idx===0` (ยังไม่แก้ entrance) — นี่คือ setup ให้ Task 2

- [ ] **Step 6: Commit (setup)**

```bash
git add internal/producer/composition_types.go internal/producer/composition.go internal/producer/audio.go internal/producer/templates/layout_multi_scene.html.tmpl internal/producer/composition_scenes_test.go
git commit -m "feat(cover): plumb COVER_SCENE_ENABLED flag through ScenesParams to template const"
```

---

### Task 2: Template — ฉาก index 0 แสดงเต็มตั้งแต่เฟรม 0 เมื่อ cover เปิด

**Files:**
- Modify: `internal/producer/templates/layout_multi_scene.html.tmpl` (บล็อก entrance ต่อฉาก ~บรรทัด 314-334)
- Test: `internal/producer/composition_scenes_test.go` (Task 1 test ผ่านครบ) + เพิ่ม assertion เฟรม-0

**Interfaces:**
- Consumes: `const COVER` (Task 1), `sceneEl`, `content`, `entranceEnd`, `ENTRANCE_EASE`, `MOTION_V2` (มีอยู่ในสโคป loop เดิม)

**บริบทโค้ดปัจจุบัน (อ่านแล้ว ~บรรทัด 314-334):** ในลูปต่อฉากมีสองสาขา — `if(MOTION_V2){...}` (บรรทัด 318-327) และ `else {...}` (บรรทัด 328-347) ทั้งสองเรียก `tl.fromTo(sceneEl,{opacity:0,...},{opacity:1,...duration:idx===0?0.5:...})` → ฉาก 0 เริ่มที่ opacity:0 เสมอ เราเพิ่มสาขา cover **ก่อน** สองสาขานี้

- [ ] **Step 1: เขียน test ที่ fail** — cover on ⇒ output มี `tl.set(sceneEl,{opacity:1},0)` (ฉาก 0 มองเห็นที่ t=0); cover off ⇒ ไม่มี

เพิ่มใน `TestRenderCompositionScenes_Cover` (ต่อจาก assertion เดิม):

```go
	// Cover on: scene 0 is pinned visible at t=0 (no opacity:0 fade on the poster frame).
	assertRenderContains(t, on, "tl.set(sceneEl,{opacity:1},0)")
	assertRenderNotContains(t, off, "tl.set(sceneEl,{opacity:1},0)")
```

> ถ้าไม่มี `assertRenderNotContains` ในไฟล์ ให้เพิ่ม helper สั้นๆ ข้าง `assertRenderContains`:
> ```go
> func assertRenderNotContains(t *testing.T, p ScenesParams, sub string) {
> 	t.Helper()
> 	out, err := RenderCompositionScenes(p)
> 	if err != nil { t.Fatalf("render: %v", err) }
> 	if strings.Contains(string(out), sub) { t.Errorf("expected output NOT to contain %q", sub) }
> }
> ```
> (ยืนยันว่าไฟล์ import `"strings"` แล้ว — ถ้ายังไม่มีให้เพิ่ม)

- [ ] **Step 2: รัน test ให้ fail**

Run: `go test ./internal/producer/ -run TestRenderCompositionScenes_Cover -v`
Expected: FAIL — ยังไม่มี `tl.set(sceneEl,{opacity:1},0)` ใน output

- [ ] **Step 3: เพิ่มสาขา cover ใน entrance loop**

ใน `internal/producer/templates/layout_multi_scene.html.tmpl` แทนที่หัวของบล็อก entrance เดิม (บรรทัด ~316-318 ที่ขึ้นต้น `let entranceEnd = sc.start + 0.7;` ตามด้วย `if(MOTION_V2){`) ให้แทรกสาขา cover ก่อน `if(MOTION_V2)`:

```javascript
        let entranceEnd = sc.start + 0.7;
        if(COVER && idx===0){
          // Frame-0 cover: the platform grabs frame 0 as the poster, so scene 0
          // and its content must be fully visible at t=0 — NEVER opacity:0. Only a
          // subtle scale settle from a visible state (no fade-in). Content is also
          // pinned visible so the hook reads immediately.
          const durIn = ENTRANCE_DUR * (SPEED_FACTOR || 1);
          tl.set(sceneEl,{opacity:1},0);
          tl.fromTo(sceneEl,{scale:1.03},{scale:1,duration:Math.max(0.5,durIn),ease:ENTRANCE_EASE},0);
          if(content){ tl.set(content,{opacity:1,x:0,y:0,scale:1},0); }
          entranceEnd = sc.start + Math.max(0.5,durIn);
        } else if(MOTION_V2){
```

> **สำคัญ:** เดิมบรรทัด 318 คือ `if(MOTION_V2){` — เปลี่ยนเป็น `} else if(MOTION_V2){` (ปิดสาขา cover ก่อน) และตรวจว่าตัวแปร `SPEED_FACTOR` ถูกประกาศก่อนจุดนี้ในลูป (อ่านโค้ดจริง ~บรรทัด 268-318 เพื่อยืนยันชื่อ) ถ้าชื่อ factor ต่างให้ใช้ชื่อจริง; ถ้ายังไม่ถูกคำนวณ ณ จุดนี้ ใช้ `ENTRANCE_DUR` ตรงๆ แทน `durIn`
> **ห้าม** ใส่ `-->` ในบล็อกนี้

- [ ] **Step 4: รัน test ให้ผ่าน**

Run: `go test ./internal/producer/ -run TestRenderCompositionScenes_Cover -v`
Expected: PASS ทุก assertion

- [ ] **Step 5: รัน regression เต็ม producer + ยืนยัน motion/off ไม่พัง**

Run: `go test ./internal/producer/...`
Expected: PASS ทั้งหมด (โดยเฉพาะ `TestRenderCompositionScenes_MotionV2`, template overflow, render-sample) — cover off ต้องไม่เปลี่ยน output เดิม

- [ ] **Step 6: Render-verify จริง (render 1 คลิปตัวอย่าง cover on) + ตรวจเฟรม 0**

Run (ถ้ามี render-sample harness ในไฟล์ `render_sample_test.go` — ยืนยันชื่อ test จริงก่อน):
`COVER_SCENE_ENABLED=true go test ./internal/producer/ -run RenderSample -v`

จากนั้นดึงเฟรมแรกด้วย ffmpeg แล้ว eyeball ว่าไม่ใช่จอ navy ว่าง:
```bash
ffmpeg -y -i <output.mp4> -vf "select=eq(n\,0)" -frames:v 1 /tmp/frame0.png
```
Expected: `/tmp/frame0.png` แสดงฉากแรกครบ (hook + รูป) ไม่ใช่พื้น navy เปล่า

- [ ] **Step 7: Commit**

```bash
git add internal/producer/templates/layout_multi_scene.html.tmpl internal/producer/composition_scenes_test.go
git commit -m "feat(cover): render scene 0 fully at frame 0 (no opacity:0 poster fade) when COVER on"
```

---

### Task 3: producer ตั้ง `Cover` จาก flag (เปิดใช้จริงใน pipeline)

**Files:**
- Modify: `internal/producer/producer.go` (จุดที่สร้าง `ScenesParams` / ตั้ง `params.MotionV2 = SceneMotionV2Enabled()` — บรรทัด ~434)
- Test: `internal/producer/composition_scenes_test.go` (ครอบด้วย Task 1-2 แล้ว; ขั้นนี้เป็น wiring)

**Interfaces:**
- Consumes: `CoverSceneEnabled()` (Task 1), `ScenesParams.Cover` (Task 1)

- [ ] **Step 1: อ่านบริบทจริง** — เปิด `internal/producer/producer.go` รอบบรรทัด 434 ยืนยันว่า `params` ที่ตั้ง `MotionV2` คือ `ScenesParams` ตัวเดียวกับที่ส่งเข้า `RenderCompositionScenes`/`BuildScenes`

Run: `grep -n "params.MotionV2\|ScenesParams{\|\.Cover\|RenderCompositionScenes\|BuildScenes" internal/producer/producer.go`
Expected: เห็นบรรทัด `params.MotionV2 = SceneMotionV2Enabled()` และตัวแปร params เดียวกัน

- [ ] **Step 2: ตั้ง `Cover` ข้าง `MotionV2`**

ใน `internal/producer/producer.go` ต่อจากบรรทัด `params.MotionV2 = SceneMotionV2Enabled()`:

```go
	params.Cover = CoverSceneEnabled()
```

- [ ] **Step 3: รัน build + test เต็ม**

Run: `go build ./... && go test ./internal/producer/...`
Expected: build ผ่าน, test PASS ทั้งหมด

- [ ] **Step 4: Commit**

```bash
git add internal/producer/producer.go
git commit -m "feat(cover): wire CoverSceneEnabled() into the scene render params"
```

---

## Self-Review (ผู้เขียน plan ตรวจกับ spec §4)

**Spec coverage (spec §4 Workstream 1):**
- "ฉากแรก render เต็มตั้งแต่เฟรม 0 (ไม่เริ่มจาก opacity:0)" → Task 2 ✅
- "punch-in เบาๆ จากสถานะที่มองเห็นแล้ว (scale 1.03→1.0)" → Task 2 Step 3 ✅
- "Flag `COVER_SCENE_ENABLED`, off → พฤติกรรมเดิม" → Task 1 + Global Constraints ✅
- "ทดสอบ transition ฉาก 0→1 ไม่กระตุก" → Task 2 Step 6 (render-verify + eyeball) ✅
- **ยังไม่ครอบใน plan นี้ (เจตนา — เป็น follow-on):** ดีไซน์ปกระดับ thumbnail (giant Kanit Black, hero image full-bleed, ส้มเน้นคำเดียว) ใช้สไตล์ hook เดิมไปก่อน การยกระดับ *ดีไซน์* ปกทำหลังพิสูจน์ frame-0 fix ได้ผล (แยก task/plan) — บันทึกไว้เป็น open item

**Placeholder scan:** ไม่มี TBD/TODO; ทุก step มี code จริง; จุดที่ต้อง "ยืนยันชื่อ helper/ตัวแปรจริงในไฟล์" ระบุคำสั่ง grep/action ชัด ไม่ใช่ placeholder

**Type consistency:** `Cover bool` ชื่อเดียวกันทั้ง `ScenesParams` / `scenesTemplateData` / template `.Cover` / const `COVER`; `CoverSceneEnabled()` ชื่อเดียวใช้ครบ ✅

---

## Roadmap — Workstream 2-4 (เขียน plan just-in-time ทีละอันตอนจะทำ)

จะเขียนแต่ละ plan ตอนเริ่มทำ workstream นั้น หลังอ่าน context จริง (เลี่ยง placeholder):
- **W2 — Premium Hero Image** (`PREMIUM_HERO_IMAGE_ENABLED`): ต้องเช็ค kie catalog จริงก่อน (มี model ดีกว่า gpt-image-2 ไหม) + `buildScenePrompt` house-style/negative/seed + circuit breaker ต่อ chain เดิม
- **W3 — Motion v2** (`SCENE_MOTION_V2_ENABLED` เดิม): flag-flip + continuous bg drift + eyeball 1 คลิป/ธีม
- **W4 — Composition Variety** (`COMPOSITION_VARIETY_ENABLED`): เพิ่ม layout subject-left/right/full-bleed/type-only ใน template + scene agent เลือก + no-repeat rule + migration prompt
