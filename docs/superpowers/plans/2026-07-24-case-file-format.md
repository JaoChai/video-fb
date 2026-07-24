# Case-File Format ("แฟ้มคดีเสี่ยงแบน") Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** เปลี่ยนคลิป hyperframes เป็น format "คดีสืบสวน" (เปิดแฟ้ม → หลักฐาน → ปิดคดี) แบบ flag-gated แทนที่ทุกคลิปเมื่อเปิด `CASE_FORMAT_ENABLED=true`

**Architecture:** ต่อยอด `layout_multi_scene.html.tmpl` เดิมผ่าน `data-format="case"` + layout ใหม่ 5 ตัว (casefile/comic/evidence/board/verdict); prompt เพิ่มแถวใหม่ `script_case`/`scene_case` ใน agent_configs (แถวเดิมไม่แตะ); ภาพ AI เจนเฉพาะซีน evidence ≤ 2 ใบ; เลขคดีนับจริงจาก DB (`clips.case_number`)

**Tech Stack:** Go 1.22+, html/template, GSAP (vendored), PostgreSQL (Neon), hyperframes CLI 0.6.70

**Spec:** `docs/superpowers/specs/2026-07-24-case-file-format-design.md`

## Global Constraints

- ห้ามเขียน `-->` ใน inline `<script>` ของ template (html/template ตัดบรรทัด → blank video — บทเรียน 8eb202f)
- Thai-safe: `letter-spacing: 0` ขึ้นไปเท่านั้น, `line-height ≥ 1.3` + `padding-top:.06em` บนหัวข้อ, ใช้ `overflow-wrap:break-word` เท่านั้น (ห้าม `anywhere`/`word-break`)
- Animation ผ่าน GSAP timeline เดียวเท่านั้น (seek-safe) — ห้าม CSS keyframes/transition ใน scene
- migration ต้อง `BEGIN;`/`COMMIT;` เอง (RunMigrations ไม่หุ้ม transaction)
- prompt_template ใช้ renderTemplate แบบ string-replace — **ห้ามใช้ `{{if}}`/`{{range}}`**
- flag ปิด (`CASE_FORMAT_ENABLED` ไม่ตั้งหรือ != "true") ⇒ ทุก code path เดิมทำงานเหมือนเดิม 100%
- ห้ามลบธีม/preset/layout เดิมใน PR นี้ (spec §9)
- ห้ามใส่ emoji ในทุก content field (StripEmoji บังคับอยู่แล้ว — อย่า bypass)

---

### Task 1: เพิ่ม 5 layout ใหม่ใน agent enum

**Files:**
- Modify: `internal/agent/scene_content.go:12`
- Test: `internal/agent/scene_content_test.go`

**Interfaces:**
- Produces: `agent.ClampLayout(v string) string` รับค่าใหม่ `casefile|comic|evidence|board|verdict` แล้วคืนค่าเดิม (ไม่โดนตีเป็น hero) — Task 2, 4, 6 พึ่ง behavior นี้

- [ ] **Step 1: เขียน failing test**

แก้ `internal/agent/scene_content_test.go` — ใน `TestClampLayout` เปลี่ยน slice ของ layout ที่ valid:

```go
func TestClampLayout(t *testing.T) {
	for _, v := range []string{"hook", "hero", "stat", "step", "tip", "cta",
		"casefile", "comic", "evidence", "board", "verdict"} {
		if ClampLayout(v) != v {
			t.Errorf("ClampLayout(%q) changed a valid layout", v)
		}
	}
	if ClampLayout("banana") != "hero" {
		t.Errorf("unknown layout must clamp to hero")
	}
	if ClampLayout("") != "hero" {
		t.Errorf("empty layout must clamp to hero")
	}
}
```

- [ ] **Step 2: รัน test ให้เห็นว่า fail**

Run: `go test ./internal/agent/ -run TestClampLayout -v`
Expected: FAIL — `ClampLayout("casefile") changed a valid layout`

- [ ] **Step 3: แก้ enum**

`internal/agent/scene_content.go` บรรทัด 12 แทนด้วย:

```go
var sceneLayouts = map[string]bool{
	"hook": true, "hero": true, "stat": true, "step": true, "tip": true, "cta": true,
	// case-file format (spec 2026-07-24): investigation storytelling layouts
	"casefile": true, "comic": true, "evidence": true, "board": true, "verdict": true,
}
```

- [ ] **Step 4: รัน test ให้ผ่าน**

Run: `go test ./internal/agent/ -run TestClampLayout -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/agent/scene_content.go internal/agent/scene_content_test.go
git commit -m "feat(case): add case-file layouts to scene layout enum"
```

---

### Task 2: SceneContent field ใหม่ + adapter parsing

**Files:**
- Modify: `internal/producer/composition_types.go` (struct SceneContent + type ใหม่)
- Modify: `internal/producer/scene_adapter.go` (`buildSceneContent`, `speedForLayout`)
- Test: `internal/producer/scene_adapter_case_test.go` (สร้างใหม่)

**Interfaces:**
- Consumes: `agent.ClampLayout` (Task 1), `agent.StripEmoji`, `agent.TruncateRunes` (มีอยู่แล้ว)
- Produces: `SceneContent{CaseNo, Stamp string; Panels []ContentPanel}` + `ContentPanel{Time, T, Quote string; Dark bool}` — Task 4 (template JSON keys `caseNo`, `stamp`, `panels[].{time,t,quote,dark}`) และ Task 6 พึ่งชื่อ field/JSON เหล่านี้เป๊ะ

- [ ] **Step 1: เขียน failing test**

สร้าง `internal/producer/scene_adapter_case_test.go`:

```go
package producer

import (
	"encoding/json"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

func caseScene(layout string, content string) agent.GeneratedScene {
	return agent.GeneratedScene{SceneNumber: 1, Layout: layout, OnScreenText: "fallback",
		Content: json.RawMessage(content)}
}

func TestBuildSceneContentCasefile(t *testing.T) {
	s := caseScene("casefile", `{"title":"คดีบัญชีฟาร์มปลิวใน 2 วัน",
		"rows":[{"t":"ผู้เสียหาย: มือใหม่ยิงครีม"},{"t":"ความเสียหาย: 30,000 บาท"}],
		"stamp":"ด่วนที่สุด"}`)
	c := buildSceneContent(s, sceneBound{Start: 0, End: 4})
	if c.Layout != "casefile" {
		t.Fatalf("layout = %q, want casefile", c.Layout)
	}
	if c.Title != "คดีบัญชีฟาร์มปลิวใน 2 วัน" || len(c.Rows) != 2 || c.Stamp != "ด่วนที่สุด" {
		t.Errorf("casefile content not parsed: %+v", c)
	}
	if c.Speed != "fast" {
		t.Errorf("casefile speed = %q, want fast", c.Speed)
	}
}

func TestBuildSceneContentComicPanels(t *testing.T) {
	s := caseScene("comic", `{"panels":[
		{"time":"วันที่ 1","t":"เปิดแอด งบ 15,000","quote":"ใบแรกต้องรุ่งแน่"},
		{"time":"คืนวันที่ 2","t":"บัญชีถูกปิด","dark":true}]}`)
	c := buildSceneContent(s, sceneBound{Start: 4, End: 9})
	if len(c.Panels) != 2 {
		t.Fatalf("panels = %d, want 2", len(c.Panels))
	}
	if c.Panels[0].Time != "วันที่ 1" || c.Panels[0].Quote != "ใบแรกต้องรุ่งแน่" {
		t.Errorf("panel[0] = %+v", c.Panels[0])
	}
	if !c.Panels[1].Dark {
		t.Errorf("panel[1].Dark must be true")
	}
}

func TestBuildSceneContentComicPanelCap(t *testing.T) {
	s := caseScene("comic", `{"panels":[{"t":"a"},{"t":"b"},{"t":"c"},{"t":"d"}]}`)
	c := buildSceneContent(s, sceneBound{Start: 0, End: 3})
	if len(c.Panels) != 3 {
		t.Errorf("panels must cap at 3, got %d", len(c.Panels))
	}
}

func TestBuildSceneContentEvidenceAndVerdict(t *testing.T) {
	ev := buildSceneContent(caseScene("evidence",
		`{"kicker":"หลักฐานชิ้นที่ 1","stamp":"REJECTED","sub":"ครีเอทีฟชุดเดิมที่เจนทับ"}`),
		sceneBound{Start: 9, End: 13})
	if ev.Layout != "evidence" || ev.Stamp != "REJECTED" || ev.Kicker == "" {
		t.Errorf("evidence content: %+v", ev)
	}
	vd := buildSceneContent(caseScene("verdict",
		`{"title":"ออกแบบใหม่ + วอร์มบัญชี","stamp":"ปิดคดี - รอดได้","cta":"ส่งเคสมาเลย","brand":"ADS VANCE"}`),
		sceneBound{Start: 13, End: 17})
	if vd.Layout != "verdict" || vd.Stamp == "" || vd.CTA != "ส่งเคสมาเลย" {
		t.Errorf("verdict content: %+v", vd)
	}
	// verdict มี content จริง ต้องไม่หลุดไป hero fallback
	if vd.Title != "ออกแบบใหม่ + วอร์มบัญชี" {
		t.Errorf("verdict title lost: %q", vd.Title)
	}
}

func TestBuildSceneContentStampOnlyNotHeroFallback(t *testing.T) {
	// ซีนที่มีแค่ stamp/panels ต้องไม่ถูกมองว่า "ว่าง" แล้ว degrade เป็น hero
	c := buildSceneContent(caseScene("comic", `{"panels":[{"t":"เหตุการณ์"}]}`),
		sceneBound{Start: 0, End: 3})
	if c.Layout != "comic" {
		t.Errorf("comic with panels degraded to %q", c.Layout)
	}
}
```

- [ ] **Step 2: รัน test ให้เห็นว่า fail**

Run: `go test ./internal/producer/ -run TestBuildSceneContent -v`
Expected: FAIL — compile error (`c.Stamp`, `c.Panels` undefined)

- [ ] **Step 3: เพิ่ม field ใน composition_types.go**

ใน `internal/producer/composition_types.go` — ใน struct `SceneContent` เพิ่มต่อจาก `Brand string` (บรรทัด 62):

```go
	// case-file format (spec 2026-07-24). CaseNo is Go-injected (never LLM).
	CaseNo string         `json:"caseNo,omitempty"` // "คดีที่ 91" — casefile + verdict only
	Stamp  string         `json:"stamp,omitempty"`  // "ด่วนที่สุด" / "REJECTED" / "ปิดคดี - รอดได้"
	Panels []ContentPanel `json:"panels,omitempty"` // comic layout only
```

และเพิ่ม type ใหม่ต่อท้าย `ContentChip`:

```go
// ContentPanel is one comic panel (case-file format). Dark=true renders the
// dramatic navy panel variant.
type ContentPanel struct {
	Time  string `json:"time,omitempty"`
	T     string `json:"t"`
	Quote string `json:"quote,omitempty"`
	Dark  bool   `json:"dark,omitempty"`
}
```

- [ ] **Step 4: แก้ scene_adapter.go**

(a) ใน `buildSceneContent` — ใน struct `raw` (บรรทัด 134-144) เพิ่ม field:

```go
	var raw struct {
		Kicker, Title, Sub, Stat, Unit, StatLabel, Num, Of, Pill, CTA, Brand string
		Stamp                                                                string
		Rows                                                                 []struct {
			T   string `json:"t"`
			Bad bool   `json:"bad"`
		} `json:"rows"`
		Chips []struct {
			N string `json:"n"`
			T string `json:"t"`
		} `json:"chips"`
		Panels []struct {
			Time  string `json:"time"`
			T     string `json:"t"`
			Quote string `json:"quote"`
			Dark  bool   `json:"dark"`
		} `json:"panels"`
	}
```

(b) หลังบล็อก `for _, ch := range raw.Chips {...}` เพิ่ม:

```go
	c.Stamp = agent.TruncateRunes(clean(raw.Stamp), 18)
	for _, pn := range raw.Panels {
		if len(c.Panels) >= 3 { // template จัดวางได้สูงสุด 3 ช่องโดยไม่ล้นเฟรม
			break
		}
		if t := agent.TruncateRunes(clean(pn.T), 36); t != "" {
			c.Panels = append(c.Panels, ContentPanel{
				Time:  agent.TruncateRunes(clean(pn.Time), 12),
				T:     t,
				Quote: agent.TruncateRunes(clean(pn.Quote), 44),
				Dark:  pn.Dark,
			})
		}
	}
```

(c) แก้เงื่อนไข hero fallback (บรรทัด `empty := ...`) ให้รวม field ใหม่:

```go
	empty := c.Title == "" && len(c.Rows) == 0 && c.Stat == "" && c.CTA == "" &&
		len(c.Chips) == 0 && c.Pill == "" && c.Sub == "" && c.StatLabel == "" &&
		c.Stamp == "" && len(c.Panels) == 0
```

(d) แก้ `speedForLayout` ให้รู้จัก layout ใหม่:

```go
func speedForLayout(layout string) string {
	switch layout {
	case "hook", "casefile":
		return "fast"
	case "hero", "stat", "evidence", "verdict":
		return "slow"
	default:
		return "normal"
	}
}
```

- [ ] **Step 5: รัน test ให้ผ่าน + test เดิมไม่พัง**

Run: `go test ./internal/producer/ -run "TestBuildSceneContent|TestBuildSceneSpecs" -v`
Expected: PASS ทั้งหมด (รวม test เดิมของ adapter)

- [ ] **Step 6: Commit**

```bash
git add internal/producer/composition_types.go internal/producer/scene_adapter.go internal/producer/scene_adapter_case_test.go
git commit -m "feat(case): SceneContent stamp/panels/caseNo + adapter parsing"
```

---

### Task 3: preset "case-file" + evidence image prompt + ตัวกรองภาพ

**Files:**
- Create: `internal/producer/case_format.go`
- Modify: `internal/producer/presets.go` (แก้ `PresetByKey` เท่านั้น)
- Modify: `internal/producer/brand_test.go` (เพิ่ม test — ไฟล์เดิม)
- Test: `internal/producer/case_format_test.go` (สร้างใหม่)

**Interfaces:**
- Consumes: `StylePreset`, `Brand`, `TypeTokens`, `MotionProfile`, `genericSceneSubject` (มีอยู่ใน producer package)
- Produces (Task 6, 7 ใช้):
  - `CaseFormatEnabled() bool`
  - `CaseFilePreset StylePreset` (Key = `"case-file"`) — **จงใจไม่ใส่ในสไลซ์ `Presets`** เพื่อไม่ให้ PickPreset/PickPresetWeighted สุ่มเจอ
  - `PresetByKey("case-file")` คืน CaseFilePreset (ทาง resume)
  - `buildEvidencePrompt(concept string, preset StylePreset, clipToken string) string`
  - `evidenceImageScenes(scenes []agent.GeneratedScene, caseEnabled bool) map[int]bool` — nil = ไม่จำกัด (โหมดเดิม)

- [ ] **Step 1: เขียน failing test**

สร้าง `internal/producer/case_format_test.go`:

```go
package producer

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

func TestCaseFormatEnabled(t *testing.T) {
	t.Setenv("CASE_FORMAT_ENABLED", "")
	if CaseFormatEnabled() {
		t.Error("must be off by default")
	}
	t.Setenv("CASE_FORMAT_ENABLED", "true")
	if !CaseFormatEnabled() {
		t.Error("must be on when env=true")
	}
}

func TestCaseFilePresetNotInRandomPool(t *testing.T) {
	for _, p := range Presets {
		if p.Key == "case-file" {
			t.Fatal("case-file must NOT be in Presets (random pool)")
		}
	}
	if PresetByKey("case-file").Key != "case-file" {
		t.Error("PresetByKey must resolve case-file (resume path)")
	}
	if PresetByKey("unknown-key").Key != "editorial-bold" {
		t.Error("unknown key must still fall back to editorial-bold")
	}
	if CaseFilePreset.BrandCSS() == "" {
		t.Error("case-file BrandCSS must render")
	}
}

func TestBuildEvidencePrompt(t *testing.T) {
	out := buildEvidencePrompt("a cream jar", CaseFilePreset, "clip-x")
	if !strings.Contains(out, "a cream jar") || !strings.Contains(out, "centered") {
		t.Errorf("evidence prompt missing subject/composition: %s", out)
	}
	if strings.Contains(out, "UPPER 55%") {
		t.Error("evidence prompt must not reserve lower frame (image sits in polaroid)")
	}
	if !strings.Contains(out, "clip-x") {
		t.Error("evidence prompt must keep the cohesion style-set token")
	}
}

func evScene(n int, layout, imgPrompt string) agent.GeneratedScene {
	return agent.GeneratedScene{SceneNumber: n, Layout: layout, ImagePrompt: imgPrompt,
		Content: json.RawMessage(`{}`)}
}

func TestEvidenceImageScenes(t *testing.T) {
	scenes := []agent.GeneratedScene{
		evScene(1, "casefile", "should be ignored"),
		evScene(2, "evidence", "a cream jar"),
		evScene(3, "hero", "should be ignored"),
		evScene(4, "evidence", "a phone"),
		evScene(5, "evidence", "a third thing - over cap"),
	}
	if evidenceImageScenes(scenes, false) != nil {
		t.Error("classic mode must return nil (no restriction)")
	}
	allowed := evidenceImageScenes(scenes, true)
	if len(allowed) != 2 || !allowed[2] || !allowed[4] || allowed[5] {
		t.Errorf("allowed = %v, want scenes 2 and 4 only (cap 2)", allowed)
	}
}
```

- [ ] **Step 2: รัน test ให้เห็นว่า fail**

Run: `go test ./internal/producer/ -run "TestCaseF|TestBuildEvidence|TestEvidenceImage" -v`
Expected: FAIL — compile error (undefined: CaseFormatEnabled ฯลฯ)

- [ ] **Step 3: สร้าง case_format.go**

```go
package producer

import (
	"os"
	"strings"

	"github.com/jaochai/video-fb/internal/agent"
)

// CaseFormatEnabled reports whether the case-file investigation format is on.
// Off => every code path behaves exactly as before (spec 2026-07-24 §4).
func CaseFormatEnabled() bool { return os.Getenv("CASE_FORMAT_ENABLED") == "true" }

// CaseFilePreset is the visual identity of the case-file format. It is
// deliberately NOT in Presets: the random/weighted pickers must never select
// it — the orchestrator chooses it explicitly when CaseFormatEnabled().
var CaseFilePreset = StylePreset{
	Key:         "case-file",
	DisplayName: "Case File",
	Palette:     Brand,
	ImageAnchor: "Evidence photograph, harsh direct camera flash, slightly desaturated muted tones, " +
		"plain neutral background, single centered subject, shallow shadows, " +
		"documentary forensic feel, photorealistic. No illustration, no 3D render, no text.",
	Font:        TypeTokens{Family: "Sarabun", HeadingFamily: "Kanit"},
	HeadingFont: TypeTokens{Family: "Sarabun", HeadingFamily: "Kanit"},
	Motion:      MotionProfile{EntranceDur: 0.42, EntranceEase: "power4.out", BGZoomTo: 1.05},
}

// buildEvidencePrompt renders the image prompt for a case-format "evidence"
// scene: one centered subject shot like a forensic photo. Unlike
// buildScenePrompt it does NOT reserve the lower 45% of the frame — the image
// sits inside a polaroid card, not under a text card.
func buildEvidencePrompt(concept string, preset StylePreset, clipToken string) string {
	subject := strings.TrimSpace(concept)
	if subject == "" {
		subject = genericSceneSubject
	}
	return preset.ImageAnchor + " " +
		"Subject: " + subject + ". " +
		"Composition: single subject centered, plain background, generous margins on all sides. " +
		"Maintain a cohesive style across the whole set: same lighting direction, " +
		"same color grade (style set: " + clipToken + "). " +
		"Avoid: oversaturated colors, warped hands or faces, generic stock-photo look, " +
		"watermarks, cluttered composition. " +
		"ABSOLUTELY NO text, letters, numbers, words, UI labels, or logos anywhere in the image."
}

// evidenceImageScenes returns the scene numbers eligible for AI image
// generation in case format: evidence-layout scenes only, capped at 2
// (spec §6). Returns nil in classic mode = no restriction.
func evidenceImageScenes(scenes []agent.GeneratedScene, caseEnabled bool) map[int]bool {
	if !caseEnabled {
		return nil
	}
	allowed := map[int]bool{}
	for _, s := range scenes {
		if len(allowed) >= 2 {
			break
		}
		if agent.ClampLayout(s.Layout) == "evidence" && strings.TrimSpace(s.ImagePrompt) != "" {
			allowed[s.SceneNumber] = true
		}
	}
	return allowed
}
```

- [ ] **Step 4: แก้ PresetByKey ใน presets.go**

แทนฟังก์ชันเดิม (บรรทัด 150-159):

```go
// PresetByKey returns the preset with key, or the editorial-bold preset (Presets[0])
// when key is unknown/empty. Resolves the out-of-pool case-file preset too, so a
// retried case clip keeps its visual identity. Never panics.
func PresetByKey(key string) StylePreset {
	if key == CaseFilePreset.Key {
		return CaseFilePreset
	}
	for _, p := range Presets {
		if p.Key == key {
			return p
		}
	}
	return Presets[0]
}
```

- [ ] **Step 5: รัน test ให้ผ่าน**

Run: `go test ./internal/producer/ -run "TestCaseF|TestBuildEvidence|TestEvidenceImage|TestPreset" -v`
Expected: PASS ทั้งหมด (รวม preset test เดิม)

- [ ] **Step 6: Commit**

```bash
git add internal/producer/case_format.go internal/producer/case_format_test.go internal/producer/presets.go
git commit -m "feat(case): case-file preset, evidence prompt, image scene filter"
```

---

### Task 4: Template — CSS + JS ของ 5 layout ใหม่ + Format plumbing ใน composition

**Files:**
- Modify: `internal/producer/composition_types.go` (ScenesParams: เพิ่ม `Format`, `CaseNumber`)
- Modify: `internal/producer/composition.go` (scenesTemplateData + inject CaseNo)
- Modify: `internal/producer/templates/layout_multi_scene.html.tmpl`
- Test: `internal/producer/composition_case_render_test.go` (สร้างใหม่)

**Interfaces:**
- Consumes: `SceneContent.CaseNo/Stamp/Panels` (Task 2)
- Produces: `ScenesParams{Format string; CaseNumber int}` — Task 6 เซ็ตจาก `CaseInfo`; HTML output มี `data-format="case"` + คลาส `cf-*`

- [ ] **Step 1: เขียน failing render test**

สร้าง `internal/producer/composition_case_render_test.go`:

```go
package producer

import (
	"strings"
	"testing"
)

func caseParams() ScenesParams {
	mk := func(n int, layout string, c SceneContent) SceneSpec {
		c.SceneNumber, c.Layout = n, layout
		c.Start, c.End = float64(n-1)*4, float64(n)*4
		return SceneSpec{SceneNumber: n, StartSec: c.Start, EndSec: c.End,
			LayoutVariant: "hook_big", CaptionStyle: "phrase_block", Content: c}
	}
	return ScenesParams{
		AspectRatio: "9:16", BrandName: "ADS VANCE", VoiceSrc: "assets/voice.wav",
		DurationSeconds: 20, Format: "case", CaseNumber: 91,
		ThemeKey: "case-file",
		Scenes: []SceneSpec{
			mk(1, "casefile", SceneContent{Title: "คดีบัญชีฟาร์มปลิว",
				Rows: []ContentRow{{Text: "ความเสียหาย: 30,000"}}, Stamp: "ด่วนที่สุด"}),
			mk(2, "comic", SceneContent{Panels: []ContentPanel{
				{Time: "วันที่ 1", T: "เปิดแอด"}, {Time: "คืนวันที่ 2", T: "ถูกปิด", Dark: true}}}),
			mk(3, "evidence", SceneContent{Kicker: "หลักฐานชิ้นที่ 1",
				Stamp: "REJECTED", Sub: "ครีเอทีฟชุดเดิม", BackgroundImage: "assets/bg-scene3.png"}),
			mk(4, "board", SceneContent{Kicker: "ผังสาเหตุ",
				Rows: []ContentRow{{Text: "บัญชีใหม่"}, {Text: "งบกระโดด"}}}),
			mk(5, "verdict", SceneContent{Title: "ออกแบบใหม่ + วอร์มบัญชี",
				Stamp: "ปิดคดี - รอดได้", CTA: "ส่งเคสมาเลย", Brand: "ADS VANCE"}),
		},
		Segments: []TranscriptSegment{{Text: "เปิดแฟ้มคดี", Start: 0, End: 2}},
	}
}

func TestRenderCaseFormat(t *testing.T) {
	out, err := RenderCompositionScenes(caseParams())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)
	for _, want := range []string{
		`data-format="case"`,
		"cf-folder", "cf-panel", "cf-polaroid", "cf-note", "cf-stamp-green",
		`คดีที่ 91`, // Go-injected case number
	} {
		if !strings.Contains(html, want) {
			t.Errorf("output missing %q", want)
		}
	}
	if strings.Contains(html, "-->") && strings.Contains(html[strings.Index(html, "<script>"):], "-->") {
		// "-->" หลัง <script> แรก = อันตราย (html/template ตัดบรรทัด)
		t.Error("inline script must never contain the sequence minus-minus-gt")
	}
}

func TestRenderClassicFormatUnchanged(t *testing.T) {
	p := caseParams()
	p.Format, p.CaseNumber = "", 0
	out, err := RenderCompositionScenes(p)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(string(out), `data-format="case"`) {
		t.Error("classic render must not carry data-format=case")
	}
	if strings.Contains(string(out), "คดีที่ 91") {
		t.Error("classic render must not inject a case number")
	}
}
```

- [ ] **Step 2: รัน test ให้เห็นว่า fail**

Run: `go test ./internal/producer/ -run TestRenderCa -v`
Expected: FAIL — compile error (`Format`, `CaseNumber` undefined ใน ScenesParams)

- [ ] **Step 3: เพิ่ม Format/CaseNumber ใน composition_types.go**

ท้าย struct `ScenesParams` (หลัง `Cover bool`):

```go
	// Case-file format (spec 2026-07-24). Format "case" switches the template's
	// data-format attribute; CaseNumber > 0 injects "คดีที่ N" into the
	// casefile/verdict scenes. Zero values = classic format, byte-identical output.
	Format     string
	CaseNumber int
```

- [ ] **Step 4: แก้ composition.go**

(a) struct `scenesTemplateData` เพิ่มหลัง `ThemeKey string`:

```go
	Format string
```

(b) ใน `RenderCompositionScenes` — ในลูป contents (หลังบรรทัด `contents[i].BackgroundImage = sc.BackgroundImage`) เพิ่ม:

```go
		// Case number is Go-injected — never trusted from the LLM (spec §5).
		if p.Format == "case" && p.CaseNumber > 0 &&
			(contents[i].Layout == "casefile" || contents[i].Layout == "verdict") {
			contents[i].CaseNo = fmt.Sprintf("คดีที่ %d", p.CaseNumber)
		}
```

(c) ใน `data := scenesTemplateData{...}` เพิ่ม `Format: p.Format,`

- [ ] **Step 5: แก้ template — attribute + CSS**

ใน `internal/producer/templates/layout_multi_scene.html.tmpl`:

(a) บรรทัด `<div id="root" ...` เพิ่ม `data-format="{{.Format}}"`:

```html
    <div id="root" data-theme="{{.ThemeKey}}" data-format="{{.Format}}" data-composition-id="main" data-start="0" data-duration="{{.DurationSeconds}}" data-width="{{.Width}}" data-height="{{.Height}}">
```

(b) เพิ่ม CSS block ก่อนบรรทัด `/* ── progress / badges / caption ── */`:

```css
      /* ── case-file format (data-format="case") ──
         Thai-safe ตามกติกาไฟล์นี้: letter-spacing >= 0, line-height >= 1.3,
         overflow-wrap:break-word เท่านั้น. Animation = GSAP timeline เดิม (ผ่าน
         kids-stagger ที่มีอยู่) — ไม่มี CSS transition/keyframes ใหม่ */
      .cf-folder{background:#E9DCB5;color:#3A2E12;border-radius:20px 34px 20px 20px;
        padding:52px 48px 44px;position:relative;box-shadow:0 30px 80px rgba(0,0,0,.5);
        display:flex;flex-direction:column;align-items:flex-start}
      .cf-folder::before{content:"";position:absolute;top:-26px;left:44px;width:300px;height:40px;
        background:#E9DCB5;border-radius:18px 18px 0 0}
      .cf-label{font-weight:800;font-size:30px;letter-spacing:.16em;color:#8A6D2F}
      .cf-name{font-weight:800;font-size:64px;line-height:1.32;padding-top:.06em;margin-top:8px;
        overflow-wrap:break-word;max-width:100%}
      .cf-meta{margin-top:18px;display:flex;flex-direction:column;gap:8px;max-width:100%}
      .cf-meta-line{font-weight:600;font-size:34px;line-height:1.4;color:#6B5622;overflow-wrap:break-word}
      .cf-stamp-red{display:inline-block;transform:rotate(-8deg);border:5px solid #C0392B;color:#C0392B;
        font-weight:800;font-size:40px;line-height:1.3;letter-spacing:.14em;padding:10px 30px;
        border-radius:14px;background:rgba(255,255,255,.55);margin-top:26px}
      .cf-panels{display:flex;flex-direction:column;gap:26px}
      .cf-panel{position:relative;background:#FFF6E6;color:#2A2113;border:6px solid #101C36;
        border-radius:14px;padding:32px 32px 28px;box-shadow:10px 10px 0 rgba(0,0,0,.55)}
      .cf-panel.dark{background:#16264A;color:#fff}
      .cf-panel-halftone{position:absolute;inset:0;border-radius:8px;opacity:.10;pointer-events:none;
        background-image:radial-gradient(#000 2px,transparent 2.4px);background-size:18px 18px}
      .cf-panel.dark .cf-panel-halftone{background-image:radial-gradient(#fff 2px,transparent 2.4px)}
      .cf-panel-time{position:absolute;top:-24px;left:26px;background:var(--amber);color:#fff;
        font-weight:800;font-size:26px;line-height:1.3;letter-spacing:.1em;padding:6px 20px;
        border-radius:10px;border:4px solid #101C36}
      .cf-panel-title{font-weight:800;font-size:44px;line-height:1.32;padding-top:.06em;overflow-wrap:break-word}
      .cf-panel-quote{font-weight:600;font-size:32px;line-height:1.4;margin-top:8px;color:#6B5622;overflow-wrap:break-word}
      .cf-panel.dark .cf-panel-quote{color:#BCD2FF}
      .cf-polaroid{align-self:center;background:#fff;padding:26px 26px 30px;border-radius:10px;
        box-shadow:0 30px 70px rgba(0,0,0,.55);transform:rotate(-3deg);position:relative;width:720px;max-width:100%}
      .cf-photo{display:block;width:100%;height:560px;object-fit:cover;border-radius:6px;background:#DDE4F0}
      .cf-photo-blank{background:linear-gradient(160deg,#E7ECF6,#C9D4E8)}
      .cf-stamp-onphoto{position:absolute;top:32%;left:10%;margin-top:0}
      .cf-polaroid-label{margin-top:18px;text-align:center;font-weight:700;font-size:30px;
        line-height:1.35;color:#43506B;overflow-wrap:break-word}
      .cf-notes{display:flex;flex-direction:column;gap:30px}
      .cf-note{position:relative;background:#FFF3C4;color:#4A3A08;border-radius:10px;
        padding:30px 36px;font-weight:700;font-size:40px;line-height:1.35;
        box-shadow:0 22px 50px rgba(0,0,0,.45);overflow-wrap:break-word}
      .cf-note-pin{position:absolute;top:-14px;left:50%;margin-left:-15px;width:30px;height:30px;
        border-radius:50%;background:var(--red);box-shadow:0 6px 12px rgba(0,0,0,.4)}
      .cf-stamp-green{align-self:center;transform:rotate(-7deg);border:6px solid var(--green);
        color:var(--green);font-weight:800;font-size:54px;line-height:1.3;letter-spacing:.14em;
        padding:14px 40px;border-radius:16px}
      .cf-folder .cf-label,.cf-name,.cf-panel-title,.cf-note,.cf-stamp-red,
      .cf-stamp-green{font-family:var(--font-heading,"Sarabun"),sans-serif}
      [data-format="case"] .scrim{background:
        linear-gradient(180deg, rgba(6,24,64,.35) 0%, rgba(6,24,64,.75) 45%, rgba(6,24,64,.96) 62%, var(--navy-deep) 100%)}
      [data-format="case"] .scene[data-layout="casefile"] .scene-content{bottom:480px}
      [data-format="case"] .scene[data-layout="verdict"] .scene-content{top:0;bottom:0;justify-content:center}
```

หมายเหตุ: `.cf-note-pin` ใช้ `margin-left:-15px` แทน `transform:translateX(-50%)` เพราะ `.cf-note` มี rotate จาก JS แล้ว — กัน transform ชนกัน

(c) ใน wrapper builder (`SCENES.forEach((sc, i) => {` บล็อกแรก) — แก้บรรทัด `w.innerHTML =` ให้ซีน evidence ไม่ใช้ภาพเป็นพื้นหลัง (ภาพไปอยู่ในโพลารอยด์แทน):

```js
        const evPhoto = sc.type === "evidence";
        w.innerHTML =
          (sc.bg && !evPhoto ? '<div class="scene-bg" style="background-image:url(\'' + sc.bg + '\')"></div>' : '<div class="scene-bg"></div>') +
          '<div class="scrim"></div><div class="scene-content" data-i="' + i + '"></div>';
```

(d) ใน content builder (`SCENES.forEach((sc,i)=>{ ... if(sc.type==="hook")...`) — เพิ่ม 5 branch ก่อน `else if(sc.type==="cta")`:

```js
        else if(sc.type==="casefile"){
          const f=el("div","cf-folder");
          f.appendChild(el("div","cf-label","CASE FILE"+(sc.caseNo?" · "+sc.caseNo:"")));
          f.appendChild(el("div","cf-name",sc.title));
          const meta=el("div","cf-meta");
          (sc.rows||[]).forEach(r=>meta.appendChild(el("div","cf-meta-line",r.t)));
          f.appendChild(meta);
          if(sc.stamp)f.appendChild(el("div","cf-stamp-red",sc.stamp));
          c.appendChild(f);
        }
        else if(sc.type==="comic"){
          const w2=el("div","cf-panels");
          (sc.panels||[]).forEach(pn=>{
            const p2=el("div","cf-panel"+(pn.dark?" dark":""));
            p2.appendChild(el("div","cf-panel-halftone"));
            if(pn.time)p2.appendChild(el("div","cf-panel-time",pn.time));
            p2.appendChild(el("div","cf-panel-title",pn.t));
            if(pn.quote)p2.appendChild(el("div","cf-panel-quote",pn.quote));
            w2.appendChild(p2);
          });
          c.appendChild(w2);
        }
        else if(sc.type==="evidence"){
          const po=el("div","cf-polaroid");
          if(sc.bg){
            const im=document.createElement("img");
            im.className="cf-photo"; im.src=sc.bg;
            po.appendChild(im);
          } else {
            po.appendChild(el("div","cf-photo cf-photo-blank"));
          }
          if(sc.stamp)po.appendChild(el("div","cf-stamp-red cf-stamp-onphoto",sc.stamp));
          if(sc.sub)po.appendChild(el("div","cf-polaroid-label",sc.sub));
          c.appendChild(po);
        }
        else if(sc.type==="board"){
          const w3=el("div","cf-notes");
          (sc.rows||[]).forEach((r,ri)=>{
            const n=el("div","cf-note",r.t);
            n.style.transform="rotate("+((ri%2===0)?-2:2)+"deg)";
            n.appendChild(el("div","cf-note-pin"));
            w3.appendChild(n);
          });
          c.appendChild(w3);
        }
        else if(sc.type==="verdict"){
          if(sc.caseNo)c.appendChild(el("div","kicker","สรุปสำนวน · "+sc.caseNo));
          c.appendChild(el("h1","title",sc.title));
          if(sc.stamp)c.appendChild(el("div","cf-stamp-green",sc.stamp));
          if(sc.cta)c.appendChild(el("div","cta",sc.cta));
          if(sc.brand)c.appendChild(el("div","brandbig",sc.brand));
          if(sc.sub)c.appendChild(el("div","sub",sc.sub));
        }
```

(ใช้ escape `·`/`ส...` ได้หรือจะพิมพ์ไทยตรงๆ ก็ได้ — ไฟล์เป็น UTF-8 พิมพ์ตรงๆ อ่านง่ายกว่า: `"สรุปสำนวน · "+sc.caseNo`)

**เช็คก่อน commit:** ห้ามมี `-->` ในโค้ดที่เพิ่ม (grep ดู), ทุก branch ใช้ `el()` เดิม, ไม่มี tween ใหม่ (ใช้ kids-stagger เดิมที่ไล่ opacity ลูกทีละตัวอยู่แล้ว — โน้ต/ช่อง comic จะโผล่ทีละใบเองผ่านกลไกนั้น)

- [ ] **Step 6: รัน test ให้ผ่าน + regression เดิม**

Run: `go test ./internal/producer/ -run "TestRenderCa|TestRenderComposition|Render" -v`
Expected: PASS ทั้งหมด — test render เดิมของ template ต้องไม่พังแม้แต่ตัวเดียว

- [ ] **Step 7: Commit**

```bash
git add internal/producer/composition_types.go internal/producer/composition.go internal/producer/templates/layout_multi_scene.html.tmpl internal/producer/composition_case_render_test.go
git commit -m "feat(case): template case-file layouts (casefile/comic/evidence/board/verdict) + Format plumbing"
```

---

### Task 5: Migration 059 — case_number + agent rows + QA/critic prompt

**Files:**
- Create: `migrations/059_case_file_format.sql`

**Interfaces:**
- Produces: คอลัมน์ `clips.case_number INT` (Task 6 repo ใช้), แถว `script_case`/`scene_case` (Task 7 orchestrator เรียกผ่าน GetByName)

- [ ] **Step 1: เขียนไฟล์ migration**

สร้าง `migrations/059_case_file_format.sql` (BEGIN/COMMIT เอง — RunMigrations ไม่หุ้ม):

```sql
-- 059: case-file format (spec docs/superpowers/specs/2026-07-24-case-file-format-design.md)
-- 1) clips.case_number  2) script_case + scene_case agent rows (แถวเดิมไม่แตะ)
-- 3) visual_qa/critic เติมเกณฑ์โหมดคดี. Rollback = ปิด CASE_FORMAT_ENABLED (ไม่ต้อง revert SQL)
BEGIN;

ALTER TABLE clips ADD COLUMN IF NOT EXISTS case_number INT;

-- script_case: copy contract การ output จากแถว script เดิมทั้งหมด (กัน regression แบบ 052
-- ที่เปลี่ยน output แล้วโค้ดอ่านไม่ได้) — เปลี่ยนเฉพาะวิธีเล่า ผ่าน prefix block + system_prompt
INSERT INTO agent_configs (agent_name, system_prompt, prompt_template, model, temperature, enabled)
SELECT 'script_case',
       system_prompt || E'\n\nโหมดนำเสนอ: สารคดีสืบสวน — ทุกคลิปคือ 1 คดี เล่าเหมือนนักสืบสรุปคดีให้รุ่นน้องฟัง ใช้คำว่า คดีนี้ / หลักฐาน / ผู้เสียหาย / ปิดคดี อย่างเป็นธรรมชาติ',
       $case_pfx$【โหมดแฟ้มคดี — โครงบังคับ 5 จังหวะ】
1) เปิดแฟ้มคดี (3 วินาทีแรกของบทพูด): บอกความเสียหาย/ปมช็อกทันที ห้ามทักทาย ห้ามเกริ่น เช่น "เปิดแฟ้มคดี: งบวันละหมื่นห้า ปลิวในสองวัน"
2) ลำดับเหตุการณ์: ไทม์ไลน์สั้นๆ วันไหนทำอะไร เสียเท่าไหร่
3) หลักฐาน + หักมุม: เฉลยสาเหตุจริงที่คนส่วนใหญ่เข้าใจผิด (open loop จากจังหวะ 1-2 ต้องมาเฉลยตรงนี้)
4) ทางรอด: ขั้นตอนแก้ที่ทำตามได้จริง 2-3 ขั้น
5) ปิดคดี: สรุปสำนวนหนึ่งประโยค + ชวนส่งเคสเข้ามาให้ทีมช่วยเช็ค
แทรก re-hook กลางคลิป เช่น "แต่หลักฐานชิ้นถัดไปต่างหากที่ชี้ตัวการจริง". ไม่ต้องใส่เลขคดี (ระบบใส่ให้เอง).

$case_pfx$ || prompt_template,
       model, temperature, TRUE
FROM agent_configs WHERE agent_name = 'script'
  AND NOT EXISTS (SELECT 1 FROM agent_configs WHERE agent_name = 'script_case');

-- scene_case: template ใหม่ทั้งฉบับ (โครง JSON output เดิมของ scene: scene_number/voice_text/
-- on_screen_text/emphasis_words/caption_style/image_prompt/layout/content)
INSERT INTO agent_configs (agent_name, system_prompt, prompt_template, model, temperature, enabled)
SELECT 'scene_case',
       'คุณคือ Director สายสารคดีสืบสวน แตกสคริปเป็นซีนสำหรับ explainer แนวตั้ง 9:16 ภาษาไทย โหมด "แฟ้มคดี". เป้าหมายสูงสุด: 3 วินาทีแรก (ปกแฟ้มคดี) ต้องหยุดนิ้วคนดู และคนดูต้องอยากรู้ว่าคดีปิดยังไง. ห้ามใส่ emoji เด็ดขาด ตอบเป็น JSON เท่านั้น.',
       $scene_case$แตกสคริปนี้ออกเป็น 6-9 ซีน สำหรับวิดีโอแนวตั้ง 9:16 ยาว {{.TargetDurationSec}} วินาที — โหมด "แฟ้มคดี" (สารคดีสืบสวน)

สคริป:
{{.Script}}

ธีมแบรนด์: {{.ThemeDescription}}

โครงคดีบังคับ:
- ซีนแรก layout "casefile" เสมอ: ปกแฟ้มคดี — title = ชื่อคดีสั้น คม ชวนเปิดดู (ไม่เกิน 40 ตัวอักษร), rows = 2-3 บรรทัดสรุปคดี (ผู้เสียหาย / ความเสียหาย / ปมชวนสงสัย) แต่ละบรรทัดไม่เกิน 36 ตัวอักษร, stamp = คำประทับสั้น เช่น "ด่วนที่สุด" (ไม่เกิน 12)
- ซีนสุดท้าย layout "verdict" เสมอ: title = สรุปสำนวนไม่เกิน 40, stamp = ตราปิดคดี เช่น "ปิดคดี - รอดได้" (ไม่เกิน 18), cta ไม่เกิน 14, brand = "ADS VANCE"
- ระหว่างทางเลือกใช้: "comic" (เล่าเหตุการณ์เป็นช่องการ์ตูน 2-3 ช่อง), "evidence" (โชว์หลักฐาน — ซีนประเภทเดียวที่มีภาพ), "board" (ผังสาเหตุ/โน้ตติดหมุด), "stat" (ตัวเลขช็อก — ช่อง stat ต้องเป็นตัวเลขนับได้จริงเท่านั้น เช่น "15,000" หรือ "0" ห้ามใส่คำ), "step" (ขั้นตอนทางรอด), "hero" (ประโยคหักมุม)
- แทรกซีน re-hook ราวกลางคลิป (board หรือ hero ที่โยนคำถามใหม่ให้อยากดูต่อ)
- อย่าใช้ layout เดียวกันเกิน 2 ซีนติดกัน หนึ่งซีนหนึ่งไอเดีย

กฎภาพ (สำคัญมาก): image_prompt ใส่ได้เฉพาะซีน layout "evidence" และรวมทั้งคลิปไม่เกิน 2 ซีน — บรรยายภาษาอังกฤษ เฉพาะ "วัตถุหลักฐานชิ้นเดียว วางกลางเฟรม" (เช่น a cream jar, a smartphone with a dark screen) ห้ามระบุสไตล์ศิลป์/สี/ตัวอักษร/โลโก้. ซีนอื่นทุกซีน image_prompt = ""

ตอบเป็น JSON array เท่านั้น แต่ละ object มี:
- "scene_number": ลำดับซีน (เริ่ม 1 ต่อเนื่อง)
- "voice_text": ประโยคพากย์ไทยของซีนนี้ (สั้น พูดลื่น ภาษาคดี)
- "on_screen_text": ข้อความบนจอสั้นๆ (ซีนแรกไม่เกิน 7 คำ)
- "emphasis_words": array 1-2 คำที่ต้องเน้น (ห้ามว่าง)
- "caption_style": "word_pop" (ซีนเปิด/พลังสูง) หรือ "phrase_block" (ซีนเนื้อหา)
- "image_prompt": ตามกฎภาพข้างบน
- "layout": หนึ่งใน "casefile" | "comic" | "evidence" | "board" | "stat" | "step" | "hero" | "verdict"
- "content": object ตาม layout (ด้านล่าง)

content แยกตาม layout:
- casefile: {"title":"ชื่อคดี","rows":[{"t":"ผู้เสียหาย: มือใหม่ยิงครีม"},{"t":"ความเสียหาย: 30,000 บาท"}],"stamp":"ด่วนที่สุด"}
- comic: {"panels":[{"time":"วันที่ 1","t":"เปิดแอด งบ 15,000","quote":"ใบแรกต้องรุ่งแน่"},{"time":"คืนวันที่ 2","t":"บัญชีถูกปิด","dark":true}]} — 2-3 ช่อง, ช่องดราม่าใส่ "dark":true, time ไม่เกิน 12 ตัวอักษร, t ไม่เกิน 36, quote ไม่เกิน 44
- evidence: {"kicker":"หลักฐานชิ้นที่ 1","stamp":"REJECTED","sub":"คำบรรยายใต้ภาพ ไม่เกิน 50"}
- board: {"kicker":"ผังสาเหตุ 3 ปัจจัย","rows":[{"t":"ปัจจัยที่หนึ่ง"},{"t":"ปัจจัยที่สอง"}]} — 2-3 ใบ
- stat: {"kicker":"หัวเรื่องสั้น","stat":"15,000","unit":"บาท","statLabel":"คำอธิบายไม่เกิน 28","chips":[{"n":"3x","t":"คำอธิบายสั้น"}]}
- step: {"num":"1","of":"ทางรอดข้อ 1 / 2","title":"ชื่อขั้นตอน","rows":[{"t":"รายละเอียด"}]}
- hero: {"title":"ข้อความใหญ่ ครอบคำเน้นด้วย <span class=\"acc\">คำ</span>","sub":"บรรทัดรอง"}
- verdict: {"title":"สรุปสำนวน","stamp":"ปิดคดี - รอดได้","cta":"ส่งเคสมาเลย","brand":"ADS VANCE","sub":"คำโปรยสั้น"}

กฎเหล็ก: ห้าม emoji หรือสัญลักษณ์ภาพในทุก field. ความยาว: cta ไม่เกิน 14, stamp ไม่เกิน 18, statLabel ไม่เกิน 28, sub ไม่เกิน 50, rows แต่ละแถวไม่เกิน 36, title ไม่เกิน 40. เขียนกระชับพอดีกรอบ.$scene_case$,
       model, temperature, TRUE
FROM agent_configs WHERE agent_name = 'scene'
  AND NOT EXISTS (SELECT 1 FROM agent_configs WHERE agent_name = 'scene_case');

-- Visual QA: องค์ประกอบดีไซน์โหมดคดี ไม่ใช่ defect (กัน false positive — บทเรียน PR#14/#17)
UPDATE agent_configs SET system_prompt = system_prompt || E'\n\nโหมด "แฟ้มคดี" (ถ้าเฟรมมีองค์ประกอบกระดาษ): แฟ้มสีกระดาษ, กรอบโพลารอยด์เอียง, ตราประทับหมุนเฉียง (แดง/เขียว), ช่องการ์ตูนขอบหนา+ลายจุด halftone, โน้ตกระดาษเหลืองติดหมุด — ทั้งหมดคือดีไซน์ที่ตั้งใจ ไม่ใช่ข้อบกพร่อง อย่าตั้ง ok=false เพราะความเอียง ลายจุด หรือสีกระดาษเหล่านี้'
WHERE agent_name = 'visual_qa'
  AND system_prompt NOT LIKE '%โหมด "แฟ้มคดี"%';

-- Critic: เกณฑ์ปกแฟ้ม + stat ต้องเป็นตัวเลข
UPDATE agent_configs SET system_prompt = system_prompt || E'\n\nโหมด "แฟ้มคดี": ซีนแรก layout casefile = ปกแฟ้มคดี — ชื่อคดี (content.title) ต้องสั้น คม ชวนเปิดดูเหมือนพาดหัวคดี ถ้าจืดให้เขียนใหม่ (คง layout และโครงสร้างเดิม). ช่อง stat ต้องเป็นตัวเลขนับได้จริง ถ้าพบคำในช่อง stat ให้ย้ายเนื้อหาซีนนั้นไปใช้ hero แทน'
WHERE agent_name = 'critic'
  AND system_prompt NOT LIKE '%โหมด "แฟ้มคดี"%';

COMMIT;
```

- [ ] **Step 2: ตรวจ syntax เบื้องต้น**

Run: `grep -c "\$case_pfx\$\|\$scene_case\$" migrations/059_case_file_format.sql`
Expected: `4` (dollar-quote เปิด-ปิดครบ 2 คู่)

Run: `grep -n "BEGIN;\|COMMIT;" migrations/059_case_file_format.sql`
Expected: BEGIN 1 ครั้งบรรทัดต้น, COMMIT 1 ครั้งบรรทัดท้าย

- [ ] **Step 3: Commit**

```bash
git add migrations/059_case_file_format.sql
git commit -m "migration: 059 case-file format (case_number + script_case/scene_case + QA/critic case criteria)"
```

---

### Task 6: models/repo + CaseInfo ผ่าน producer

**Files:**
- Modify: `internal/models/clip.go` (Clip struct)
- Modify: `internal/repository/clips.go` (clipColumns, scanClip, + 2 method ใหม่)
- Modify: `internal/producer/producer.go` (`ProduceHyperframes916`, `AssembleHyperframes916`, `generateSceneImagesParallel`, image loops)
- Test: `internal/producer/case_format_test.go` (เพิ่ม test)

**Interfaces:**
- Consumes: `evidenceImageScenes`, `buildEvidencePrompt`, `CaseFilePreset` (Task 3), `ScenesParams.Format/CaseNumber` (Task 4), คอลัมน์ `case_number` (Task 5)
- Produces (Task 7 ใช้):
  - `producer.CaseInfo{Enabled bool; CaseNumber int}`
  - `ProduceHyperframes916(ctx, clipID string, scenes []agent.GeneratedScene, preset StylePreset, caseInfo CaseInfo)` — **signature เปลี่ยน** (เพิ่ม caseInfo)
  - `AssembleHyperframes916(ctx, clipID, scenes, preset, caseInfo)` — signature เปลี่ยนเช่นกัน
  - `models.Clip.CaseNumber *int`
  - `ClipsRepo.NextCaseNumber(ctx) (int, error)` / `ClipsRepo.SetCaseNumber(ctx, id string, n int) error`

- [ ] **Step 1: models + repo**

`internal/models/clip.go` — เพิ่มใน struct Clip หลัง `ProductionStage`:

```go
	CaseNumber *int `json:"case_number,omitempty"` // case-file format running number (nil = classic clip)
```

`internal/repository/clips.go`:

(a) `clipColumns` เพิ่ม `case_number` ต่อท้าย:

```go
const clipColumns = `id, title, question, questioner_name, answer_script, voice_script,
	category, status, video_16_9_url, video_9_16_url, thumbnail_url,
	publish_date::text, created_at, updated_at, fail_reason, retry_count, style_preset, content_format,
	production_stage, review_retry_count, auto_review_held, case_number`
```

(b) `scanClip` เพิ่ม `&c.CaseNumber` ต่อท้าย (ลำดับต้องตรงกับ clipColumns):

```go
		&c.ProductionStage, &c.ReviewRetryCount, &c.AutoReviewHeld, &c.CaseNumber,
```

(c) เพิ่ม 2 method ท้ายไฟล์:

```go
// NextCaseNumber returns the next case-file running number: continues from
// MAX(case_number), seeded by the published-clip count so case #1 starts after
// the channel's real history (spec 2026-07-24 §5).
func (r *ClipsRepo) NextCaseNumber(ctx context.Context) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT GREATEST(
		COALESCE((SELECT MAX(case_number) FROM clips), 0),
		(SELECT COUNT(*) FROM clips WHERE status = 'published')) + 1`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("next case number: %w", err)
	}
	return n, nil
}

// SetCaseNumber pins a clip's case number (assigned once at production start).
func (r *ClipsRepo) SetCaseNumber(ctx context.Context, id string, n int) error {
	if _, err := r.pool.Exec(ctx,
		`UPDATE clips SET case_number = $1, updated_at = NOW() WHERE id = $2`, n, id); err != nil {
		return fmt.Errorf("set case number: %w", err)
	}
	return nil
}
```

- [ ] **Step 2: เขียน failing test สำหรับ CaseInfo**

เพิ่มใน `internal/producer/case_format_test.go`:

```go
func TestCaseInfoZeroValueIsClassic(t *testing.T) {
	var ci CaseInfo
	if ci.Enabled || ci.CaseNumber != 0 {
		t.Error("zero CaseInfo must mean classic format")
	}
}
```

Run: `go test ./internal/producer/ -run TestCaseInfo -v`
Expected: FAIL — undefined: CaseInfo

- [ ] **Step 3: เพิ่ม CaseInfo + ร้อยผ่าน producer.go**

(a) ใน `internal/producer/case_format.go` เพิ่ม:

```go
// CaseInfo carries the case-file production context down the producer path.
// Zero value = classic format (byte-identical to today's output).
type CaseInfo struct {
	Enabled    bool
	CaseNumber int // 0 = unknown; the template then omits the case number
}
```

(b) `internal/producer/producer.go`:

- แก้ signature บรรทัด 524: `func (p *Producer) ProduceHyperframes916(ctx context.Context, clipID string, scenes []agent.GeneratedScene, preset StylePreset, caseInfo CaseInfo) (*ProduceResult, error)` และส่งต่อไป `AssembleHyperframes916(ctx, clipID, scenes, preset, caseInfo)` (บรรทัด 526)
- แก้ signature บรรทัด 361: `func (p *Producer) AssembleHyperframes916(ctx context.Context, clipID string, scenes []agent.GeneratedScene, preset StylePreset, caseInfo CaseInfo) (*assembleOutput, error)`
- ใน `AssembleHyperframes916` ก่อนลูปเจนภาพ (ทั้ง path ปกติราวบรรทัด 390 และ path parallel ที่เรียก `generateSceneImagesParallel`) เพิ่ม:

```go
	allowedImg := evidenceImageScenes(scenes, caseInfo.Enabled)
```

- ในลูปเจนภาพปกติ (ราวบรรทัด 392) หลัง `if strings.TrimSpace(s.ImagePrompt) == "" { continue }` เพิ่ม:

```go
			if allowedImg != nil && !allowedImg[s.SceneNumber] {
				continue // case format: only evidence scenes get AI images (cap 2)
			}
```

- จุดสร้าง prompt (บรรทัด 397 และจุดเทียบเท่าใน `generateSceneImagesParallel`) แก้เป็น:

```go
				prompt := buildScenePrompt(s.ImagePrompt, "9:16", preset, clipID)
				if caseInfo.Enabled {
					prompt = buildEvidencePrompt(s.ImagePrompt, preset, clipID)
				}
```

- `generateSceneImagesParallel` (บรรทัด 313): เพิ่ม param `caseInfo CaseInfo` ท้ายสุด แล้วใช้ `allowedImg := evidenceImageScenes(scenes, caseInfo.Enabled)` + skip + evidence prompt แบบเดียวกันในลูปของมัน; อัปเดต call site ใน AssembleHyperframes916
- จุดสร้าง `params := ScenesParams{...}` (ราวบรรทัด 420) เพิ่ม 2 field:

```go
		Format:     map[bool]string{true: "case", false: ""}[caseInfo.Enabled],
		CaseNumber: caseInfo.CaseNumber,
```

(ถ้าอ่านยาก ใช้ if ธรรมดาก่อนสร้าง params: `format := ""; if caseInfo.Enabled { format = "case" }` แล้ว `Format: format,` — เลือกแบบ if ธรรมดา อ่านง่ายกว่า)

- [ ] **Step 4: แก้ compile error ที่ call site**

Run: `go build ./...`
Expected: error ที่ `internal/orchestrator/orchestrator.go:607` (จำนวน argument ไม่ครบ) — แก้ชั่วคราวให้ compile ผ่านก่อน (Task 7 จะใส่ logic จริง):

```go
	result, err := o.producer.ProduceHyperframes916(ctx, clipID, scenes, preset, producer.CaseInfo{})
```

Run: `go build ./...`
Expected: สำเร็จ ไม่มี error

- [ ] **Step 5: รัน test ทั้งหมดของ producer**

Run: `go test ./internal/producer/ ./internal/repository/ ./internal/models/ -count=1`
Expected: PASS ทั้งหมด

- [ ] **Step 6: Commit**

```bash
git add internal/models/clip.go internal/repository/clips.go internal/producer/producer.go internal/producer/case_format.go internal/producer/case_format_test.go internal/orchestrator/orchestrator.go
git commit -m "feat(case): CaseInfo through producer, evidence-only image gen, clips.case_number repo"
```

---

### Task 7: Orchestrator wiring — flag, agent rows, preset, เลขคดี

**Files:**
- Modify: `internal/orchestrator/orchestrator.go` (จุดเลือก script/scene agent, จุดเลือก preset, จุดเรียก ProduceHyperframes916)

**Interfaces:**
- Consumes: `producer.CaseFormatEnabled()`, `producer.CaseFilePreset`, `producer.CaseInfo`, `ClipsRepo.NextCaseNumber/SetCaseNumber` (Task 3, 6)
- Produces: พฤติกรรม runtime — flag เปิด: ใช้แถว `script_case`/`scene_case` (fallback แถวเดิมถ้าไม่มี), preset = case-file, คลิปได้เลขคดี

- [ ] **Step 1: เพิ่ม helper เลือก agent row**

เพิ่ม method ใน orchestrator.go (ใกล้กลุ่ม helper อื่น):

```go
// caseAgentConfig resolves the agent row for a role: when the case format is
// on it prefers the "<name>_case" row, failing open to the classic row so a
// missing/disabled case row never blocks production (spec §4).
func (o *Orchestrator) caseAgentConfig(ctx context.Context, name string) (*models.AgentConfig, error) {
	if producer.CaseFormatEnabled() {
		if cfg, err := o.agentsRepo.GetByName(ctx, name+"_case"); err == nil && cfg.Enabled {
			return cfg, nil
		}
		log.Printf("case format: %s_case row missing/disabled — falling back to %s", name, name)
	}
	return o.agentsRepo.GetByName(ctx, name)
}
```

- [ ] **Step 2: สลับจุดเรียก script/scene**

- บรรทัด 306: `scriptCfg, err := o.agentsRepo.GetByName(ctx, "script")` → `scriptCfg, err := o.caseAgentConfig(ctx, "script")`
- บรรทัด 828 (เส้น retry script): เปลี่ยนแบบเดียวกัน
- บรรทัด 473: `sceneCfg, err := o.agentsRepo.GetByName(ctx, "scene")` → `sceneCfg, err := o.caseAgentConfig(ctx, "scene")`

- [ ] **Step 3: บังคับ preset case-file**

จุดเลือก preset (บรรทัด 343-356) — ครอบด้วยเงื่อนไข flag:

```go
	preset := producer.PresetByKey("editorial-bold")
	if producer.CaseFormatEnabled() {
		preset = producer.CaseFilePreset // case format: fixed identity, skip random/weighted pickers
	} else if producer.StylePresetsEnabled() {
		// ... (โค้ดเดิมทั้งบล็อก PickPreset/PickPresetWeighted ไม่แตะ)
	}
```

จุด retry/resume (บรรทัด 796, 844) ใช้ `PresetByKey(clip.StylePreset)` อยู่แล้ว — Task 3 ทำให้ resolve "case-file" ได้แล้ว **ไม่ต้องแก้**

- [ ] **Step 4: เลขคดี + CaseInfo ที่จุดเรียก produce**

ที่บรรทัด 607 (แก้ placeholder จาก Task 6) — ก่อนเรียก ProduceHyperframes916:

```go
	caseInfo := producer.CaseInfo{Enabled: preset.Key == producer.CaseFilePreset.Key}
	if caseInfo.Enabled {
		if clip, cErr := o.clipsRepo.GetByID(ctx, clipID); cErr == nil &&
			clip.CaseNumber != nil && *clip.CaseNumber > 0 {
			caseInfo.CaseNumber = *clip.CaseNumber // resume keeps its number
		} else if n, nErr := o.clipsRepo.NextCaseNumber(ctx); nErr == nil {
			if sErr := o.clipsRepo.SetCaseNumber(ctx, clipID, n); sErr == nil {
				caseInfo.CaseNumber = n
			} else {
				log.Printf("case number: set failed (fail-open, clip renders without number): %v", sErr)
			}
		} else {
			log.Printf("case number: next failed (fail-open, clip renders without number): %v", nErr)
		}
	}
	result, err := o.producer.ProduceHyperframes916(ctx, clipID, scenes, preset, caseInfo)
```

หมายเหตุ implementer: ตัวแปร `preset` ในฟังก์ชันที่ครอบบรรทัด 607 คือ preset ของคลิปนั้น (fresh หรือ resume ก็ตาม) — `Enabled` ผูกกับ `preset.Key` จึงถูกทั้งสองเส้นทางโดยอัตโนมัติ ถ้าใน scope นั้นไม่มีตัวแปร preset ให้ไล่หาว่าฟังก์ชันรับมาจากไหน (grep `ProduceHyperframes916`) — ห้ามเดา

- [ ] **Step 5: Build + test ทั้งโปรเจกต์**

Run: `go build ./... && go test ./... -count=1`
Expected: build ผ่าน, test ผ่านทั้งหมด

- [ ] **Step 6: Commit**

```bash
git add internal/orchestrator/orchestrator.go
git commit -m "feat(case): orchestrator wiring — flag-gated agent rows, fixed preset, case number assignment"
```

---

### Task 8: Regression ครบวง + render จริง local

**Files:** ไม่มีไฟล์ใหม่ (verification task)

- [ ] **Step 1: Test suite เต็ม**

Run: `go test ./... -count=1`
Expected: PASS ทุก package — ถ้ามี fail แม้แต่ตัวเดียว หยุดและแก้ก่อน (ห้าม skip)

- [ ] **Step 2: ตรวจ flag-off regression**

Run: `grep -rn "CASE_FORMAT_ENABLED" internal/ | grep -v _test`
Expected: เจอเฉพาะใน `case_format.go` (definition) และผ่าน `CaseFormatEnabled()`/`caseAgentConfig` เท่านั้น — ไม่มีจุดไหนอ่าน env ตรง

Run: `go test ./internal/producer/ -run TestRenderClassicFormatUnchanged -v`
Expected: PASS (ยืนยัน classic path ไม่ปนเปื้อน)

- [ ] **Step 3: Render จริงด้วย hyperframes CLI (ถ้าเครื่องมี chromium)**

เขียน HTML ตัวอย่างจาก test fixture แล้วผ่าน lint+inspect:

```bash
go test ./internal/producer/ -run TestRenderCaseFormat -v
# ถ้ามี test ที่ dump ไฟล์ HTML ใน testdata อยู่แล้วให้ใช้ pattern เดิมของ repo;
# ไม่มีก็ข้าม step นี้ได้ (inspect gate จะรันจริงบน Railway ทุก produce อยู่แล้ว)
```

Expected: ผ่าน หรือบันทึกไว้ว่า inspect จะถูก gate ตอน produce จริง

- [ ] **Step 4: Commit (ถ้ามีแก้อะไรจาก regression)**

```bash
git status --short   # ถ้าว่าง = ไม่มีอะไรต้อง commit ใน task นี้
```

---

### Task 9: Simplify + PR

- [ ] **Step 1: รัน /simplify บน diff ทั้งหมด** (ตาม preference ของ user: simplify ก่อน commit ปิดงานเสมอ)

ใช้ skill `simplify` กับ diff ของ branch แล้ว apply การแก้ที่สมเหตุสมผล จากนั้น `go test ./... -count=1` ต้องผ่านเหมือนเดิม

- [ ] **Step 2: เปิด PR**

```bash
git push -u origin HEAD
gh pr create --title "feat: case-file format (แฟ้มคดีเสี่ยงแบน) — flag-gated" --body "$(cat <<'EOF'
## Summary
- Case-file investigation format: 5 new layouts (casefile/comic/evidence/board/verdict), flag `CASE_FORMAT_ENABLED` (default off)
- New agent rows `script_case`/`scene_case` (classic rows untouched); migration 059
- AI images now evidence-only (max 2/clip) in case mode; case number from `clips.case_number`
- Rollback = flag off; old themes intentionally kept (spec §9)

Spec: docs/superpowers/specs/2026-07-24-case-file-format-design.md

## Test plan
- [ ] go test ./... green
- [ ] flag-off regression: classic render byte-stable (TestRenderClassicFormatUnchanged)
- [ ] deploy flag-off -> migration 059 verified on Neon -> flip flag -> produce 1 clip -> user eyeball

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

---

### Task 10: Deploy runbook (หลัง merge — ทำร่วมกับ user)

- [ ] Deploy master (flag ยังปิด) → migration 059 รันอัตโนมัติ
- [ ] ตรวจบน Neon: `SELECT agent_name, enabled FROM agent_configs WHERE agent_name IN ('script_case','scene_case')` → 2 แถว enabled; `SELECT column_name FROM information_schema.columns WHERE table_name='clips' AND column_name='case_number'` → 1 แถว
- [ ] ตั้ง `CASE_FORMAT_ENABLED=true` บน Railway service `adsvance-v2` → redeploy
- [ ] ยิง produce 1 คลิป: `POST /orchestrator/produce` (count=1 — อย่าลืม ไม่ใส่ = 7 คลิป!)
- [ ] ตรวจคลิป: ปกแฟ้มมีเลขคดี, comic/board/verdict render ถูก, แคปชั่นไม่ทับ, เสียงเล่าเป็นคดี
- [ ] **user eyeball ก่อนปล่อย schedule อัตโนมัติ** — ถ้าไม่ผ่าน: ปิด flag (rollback สมบูรณ์ทันที)
- [ ] เฝ้า 1 สัปดาห์: needs_review/failed rate + retention → ผ่านแล้วค่อยเปิด PR ลบธีมเก่า (spec §9)

---

## Self-Review (ทำแล้ว)

- **Spec coverage:** §3 layouts → Task 1,2,4; §4 flag/prompt rows → Task 5,7; §5 เลขคดี → Task 5,6,7; §6 ภาพ → Task 3,6; §7 agent impacts → Task 5 (QA/critic), Task 7 (script/scene); §8 ไม่แตะ — ไม่มี task ไปยุ่ง; §9 ห้ามลบ — ระบุใน Global Constraints; §10 testing → Task 8,10 ✓
- **Placeholder scan:** ไม่มี TBD/TODO; ทุก code step มีโค้ดจริง ✓
- **Type consistency:** `CaseInfo{Enabled,CaseNumber}` ใช้ตรงกันใน Task 3(นิยาม)/6(ร้อย)/7(สร้าง); JSON keys `caseNo/stamp/panels` ตรงกันระหว่าง Task 2 (struct tags) กับ Task 4 (template JS `sc.caseNo/sc.stamp/sc.panels`); signature `ProduceHyperframes916(..., caseInfo)` ตรงกันใน Task 6/7 ✓
