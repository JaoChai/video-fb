# Multi-Scene Phase 1 — Script Agent Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** ทำให้ script agent ออกได้ 3-6 ฉาก (content-driven) แทนการล็อก 1 ฉาก โดยระบบ validate/normalize ฉากปลอดภัย และ pipeline เดิมยังทำงานไม่ crash

**Architecture:** ขยาย `GeneratedScene` ด้วย `SceneType` (enum) + `BgHint`, เพิ่ม method `(*GeneratedScript).Normalize()` (clamp 1-6 ฉาก, renumber, default scene_type เพี้ยน), migration ใหม่แทน skills single-scene ของ 009 เป็น multi-scene 3-6 ฉาก ≤60s, เรียก Normalize + guard 0-ฉาก ใน orchestrator. การ render multi-scene จริงอยู่ Phase ถัดไป — Phase 1 ส่ง/เก็บ/loop ฉากได้ปลอดภัยก่อน

**Tech Stack:** Go 1.25, pgx, LLM JSON (OpenRouter), SQL migration (golang-migrate style numbered files)

**Scope (Phase 1 เท่านั้น):** ไม่แตะ template/composition/render ยังเป็น dev branch `feat/multi-scene-video` ไม่ deploy จนครบทุก phase. `BgHint` ผลิต+พกใน memory เฟสนี้ ใช้จริง/persist เฟส 4

---

### Task 1: Scene types + BgHint + Normalize ใน agent package

**Files:**
- Modify: `internal/agent/script.go:33-48`
- Test (create): `internal/agent/script_test.go`

- [ ] **Step 1: เขียน failing test สำหรับ Normalize**

สร้าง `internal/agent/script_test.go`:
```go
package agent

import "testing"

func TestNormalize_ClampsAndRenumbers(t *testing.T) {
	s := &GeneratedScript{Scenes: []GeneratedScene{
		{SceneNumber: 5, SceneType: "hook", VoiceText: "a"},
		{SceneNumber: 9, SceneType: "weird", VoiceText: "b"},
		{SceneNumber: 1, SceneType: "step", VoiceText: "c"},
		{SceneNumber: 2, SceneType: "win", VoiceText: "d"},
		{SceneNumber: 3, SceneType: "cta", VoiceText: "e"},
		{SceneNumber: 4, SceneType: "problem", VoiceText: "f"},
		{SceneNumber: 6, SceneType: "step", VoiceText: "g"},
	}}
	s.Normalize()

	if len(s.Scenes) != maxScenes {
		t.Fatalf("want %d scenes, got %d", maxScenes, len(s.Scenes))
	}
	for i, sc := range s.Scenes {
		if sc.SceneNumber != i+1 {
			t.Errorf("scene[%d] number = %d, want %d", i, sc.SceneNumber, i+1)
		}
		if !validSceneTypes[sc.SceneType] {
			t.Errorf("scene[%d] type %q not normalized", i, sc.SceneType)
		}
	}
}

func TestNormalize_DefaultsInvalidType(t *testing.T) {
	s := &GeneratedScript{Scenes: []GeneratedScene{
		{SceneNumber: 1, SceneType: "", VoiceText: "x"},
		{SceneNumber: 2, SceneType: "STEP", VoiceText: "y"},
	}}
	s.Normalize()
	if s.Scenes[0].SceneType != SceneStep || s.Scenes[1].SceneType != SceneStep {
		t.Errorf("invalid scene types not defaulted to %q: %+v", SceneStep, s.Scenes)
	}
}
```

- [ ] **Step 2: รัน test ให้เห็นว่า fail**

Run: `go test ./internal/agent/ -run TestNormalize -v`
Expected: FAIL — `undefined: maxScenes`, `undefined: validSceneTypes`, `s.Normalize undefined`

- [ ] **Step 3: เพิ่ม constants + field + Normalize ใน script.go**

แก้ `internal/agent/script.go` — เพิ่ม field `BgHint` ใน `GeneratedScene`:
```go
type GeneratedScene struct {
	SceneNumber     int             `json:"scene_number"`
	SceneType       string          `json:"scene_type"`
	TextContent     string          `json:"text_content"`
	VoiceText       string          `json:"voice_text"`
	BgHint          string          `json:"bg_hint"`
	DurationSeconds float64         `json:"duration_seconds"`
	TextOverlays    json.RawMessage `json:"text_overlays"`
}
```

เพิ่มท้ายไฟล์ (หลัง struct):
```go
// Scene types the composition agent maps to layout variants downstream.
const (
	SceneHook    = "hook"
	SceneProblem = "problem"
	SceneStep    = "step"
	SceneWin     = "win"
	SceneCTA     = "cta"
)

const (
	minScenes = 1
	maxScenes = 6
)

var validSceneTypes = map[string]bool{
	SceneHook: true, SceneProblem: true, SceneStep: true, SceneWin: true, SceneCTA: true,
}

// Normalize keeps an LLM-produced script safe for the pipeline: caps the scene
// count, renumbers scenes 1..N in arrival order, and defaults any unrecognized
// scene_type to SceneStep. It does not fabricate scenes (0-scene is the caller's
// error to handle).
func (s *GeneratedScript) Normalize() {
	if len(s.Scenes) > maxScenes {
		s.Scenes = s.Scenes[:maxScenes]
	}
	for i := range s.Scenes {
		s.Scenes[i].SceneNumber = i + 1
		if !validSceneTypes[s.Scenes[i].SceneType] {
			s.Scenes[i].SceneType = SceneStep
		}
	}
}
```

- [ ] **Step 4: รัน test ให้ผ่าน**

Run: `go test ./internal/agent/ -run TestNormalize -v`
Expected: PASS (ทั้ง 2 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/agent/script.go internal/agent/script_test.go
git commit -m "feat(script): scene types + BgHint + Normalize for multi-scene"
```

---

### Task 2: Migration แทน skills single-scene เป็น multi-scene

**Files:**
- Create: `migrations/022_script_agent_multi_scene.sql`

- [ ] **Step 1: เขียน migration**

สร้าง `migrations/022_script_agent_multi_scene.sql`:
```sql
-- Multi-scene script agent: replace the single-scene lock (009) with content-driven
-- 3-6 scenes. Each scene carries its own headline + voice portion + background hint.
-- Brand voice / TTS-safety rules from 009 are retained.

UPDATE agent_configs
SET
    skills = $script_sk$- โครงสร้างวิดีโอ: แตกคำตอบเป็น "ฉาก" 3-6 ฉาก เล่าเรื่องต่อเนื่อง (content-driven) — ห้ามเกิน 6 ฉาก
- รวม voice_text ทุกฉากต้องพูดจบใน 60 วินาที (≈ไม่เกิน 150 คำไทยรวมทุกฉาก) — กระชับ
- แต่ละฉากมี:
  - scene_number: ลำดับเริ่มที่ 1
  - scene_type: เลือกจาก hook | problem | step | win | cta (hook=เปิดสะดุด, problem=ปัญหา/สาเหตุ, step=ขั้นตอนแก้, win=ผลลัพธ์, cta=ปิดท้ายชวนติดต่อ)
  - headline: ข้อความสั้นมากสำหรับขึ้นจอในฉากนั้น (≤8 คำ) — ใส่ใน field "text_content"
  - voice_text: บทพากย์เฉพาะฉากนั้น
  - bg_hint: บรรยายบรรยากาศพื้นหลังของฉากสั้นๆ (เช่น "แดชบอร์ดโฆษณาเรืองแสงสีส้ม") — ไม่มีตัวหนังสือในภาพ
- ฉากแรกควรเป็น hook, ฉากท้ายควรเป็น cta
- voice_text ห้ามมีอักขระ "@" และห้ามมี URL ใดๆ เด็ดขาด (TTS อ่านลิงก์ไม่ออก เสียงจะตัด)
- เรียกแบรนด์ในเสียงพากย์ว่า "แอดส์แวนซ์" (ห้ามเขียน Adsvance, @adsvance, AdsVance, Ads Vance ใน voice_text)
- CTA ปิดท้ายให้พูดทำนองนี้: "ติดต่อทีมงานแอดส์แวนซ์ทางไลน์ ไอดีแอดส์แวนซ์ หรือเข้ากลุ่มเทเลแกรมแอดส์แวนซ์ได้เลยครับ"
- ใช้ ... สำหรับจังหวะหายใจระหว่างประโยค
- duration_seconds ต่อฉาก = ประมาณความยาวฉากนั้น, total_duration_seconds รวม ≤ 60 วินาที
- youtube_title: ดึงดูด สั้น ไม่เกิน 70 ตัวอักษร ลงท้ายด้วย {Ads Vance}
- youtube_description: ใช้แค่ 2 บรรทัดนี้เท่านั้น (URL/handle อยู่ตรงนี้ได้):
  "ติดต่อทีมงาน line id : @adsvance" (ขึ้นบรรทัดใหม่)
  "เข้ากลุ่มเทเรแกรมเพื่อรับข่าวสาร : https://t.me/adsvancech"
- youtube_tags: array ของ tag ภาษาไทย+อังกฤษ
- ห้ามแนะนำการทำผิดนโยบาย Facebook$script_sk$
WHERE agent_name = 'script';
```

- [ ] **Step 2: ตรวจ migration apply ได้ (local DB หรือ dry parse)**

Run: `grep -c "script_sk" migrations/022_script_agent_multi_scene.sql`
Expected: `2` (เปิด-ปิด dollar-quote ครบ) — ยืนยันไม่มี syntax dollar-quote ค้าง

- [ ] **Step 3: Commit**

```bash
git add migrations/022_script_agent_multi_scene.sql
git commit -m "feat(script): migration for multi-scene 3-6 scenes (<=60s)"
```

---

### Task 3: เรียก Normalize + guard 0-ฉาก ใน orchestrator

**Files:**
- Modify: `internal/orchestrator/orchestrator.go:215-223`

- [ ] **Step 1: เพิ่มการเรียก Normalize + guard หลัง Generate**

ใน `produceClipWithID` หลังบรรทัด `script, err := o.scriptAgent.Generate(...)` และก่อน `validateScript(script)` (รอบบรรทัด 215-220) แทรก:
```go
	if err != nil {
		return fmt.Errorf("generate script: %w", err)
	}
	script.Normalize()
	if len(script.Scenes) < 1 {
		return fmt.Errorf("script produced 0 scenes for clip %s", clipID)
	}
	validateScript(script)
```
(หมายเหตุ: เก็บ error-check เดิมไว้ — เพิ่มเฉพาะ `script.Normalize()` + guard 0-ฉาก ก่อน `validateScript`)

- [ ] **Step 2: build + test ทั้งโปรเจคไม่พัง**

Run: `go build ./... && go test ./internal/agent/ ./internal/orchestrator/ -v 2>&1 | tail -20`
Expected: build ผ่าน, agent tests PASS, orchestrator package compile/test ผ่าน (ไม่มี test เดิมก็ต้อง `ok`/`no test files`)

- [ ] **Step 3: ยืนยัน voice concat รองรับ N ฉากอยู่แล้ว (อ่าน ไม่แก้)**

Run: `sed -n '246,261p' internal/orchestrator/orchestrator.go`
Expected: เห็น loop `for _, s := range script.Scenes` ที่รวม `fullVoice` จากทุกฉาก (ยืนยันว่า N ฉากถูกพากย์ครบ ไม่ต้องแก้)

- [ ] **Step 4: Commit**

```bash
git add internal/orchestrator/orchestrator.go
git commit -m "feat(orchestrator): normalize + guard multi-scene script output"
```

---

## Self-Review

**Spec coverage (Phase 1 portion):** §4 script agent multi-scene (3-6 ฉาก, scene_type, headline, voice_text, bg_hint, ≤60s) → Task 1+2 ครบ. §2 per-scene fields → struct ใน Task 1. Render/composition = phase ถัดไป (นอก scope Phase 1, ระบุชัด).

**Placeholder scan:** ไม่มี TBD/TODO — โค้ด+test+SQL+คำสั่งครบทุก step.

**Type consistency:** `SceneStep`/`validSceneTypes`/`minScenes`/`maxScenes`/`Normalize`/`BgHint` นิยามใน Task 1 และอ้างถึงใน Task 1 tests + Task 3 ตรงกันหมด. `minScenes` ประกาศไว้ (ใช้เชิงเอกสาร/อนาคต) — guard ใน Task 3 ใช้ `< 1` ตรงกับเจตนา.

**หมายเหตุ:** Phase 1 ทำให้ script ออก N ฉากแต่ render ยังใช้ scene[0] (composition) — เป็น dev branch ไม่ deploy จนครบ phase 3-4 จึงไม่กระทบ production
