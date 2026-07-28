# 4 ช่วงเวลา 4 โหมด — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** เพิ่มโหมดหน้าตาใหม่ 2 ตัว (แชทลูกค้า 18:00, ห้องควบคุม 21:00) ให้คลิป 4 รอบต่อวันมีหน้าตาต่างกันครบทั้ง 4 แบบ

**Architecture:** โหมดหนึ่ง = preset (Go struct) + ชุด layout ที่ scene agent ใช้ได้ + CSS/JS branch ใน template เดียว + นโยบายภาพ AI + agent rows 3 แถวใน DB ทั้งหมดเสียบเข้าโครงที่มีอยู่แล้วของโหมด `case`/`tutorial` ไม่มีการรื้อกลไกเดิม โหมดใหม่ทั้งคู่ใช้ภาพ AI 1 ใบเท่าโหมดคู่มือ

**Tech Stack:** Go 1.25 · html/template + GSAP (hyperframes) · PostgreSQL (Neon) · kie.ai gpt-image-2

## Global Constraints

- **สเปกอ้างอิง:** `docs/superpowers/specs/2026-07-28-four-mode-slots-design.md`
- **ห้ามแตะตะแกรงคลิปสอน** — `tutorialGateFailure`, `UIVocabViolations`, `CountUIStepScenes` ต้องไม่ถูกแก้ ห้องควบคุม**ห่อ** layout `uistep` ด้วย CSS ไม่ใช่แทนที่
- **ห้ามใส่ feature flag ใหม่** — rollback คือ `git revert`
- **งบภาพ AI คงเดิม** — โหมดใหม่ใช้ 1 ภาพ (ซีนเปิด) เท่านั้น
- **กติกา Thai-safe ใน template:** `letter-spacing` ห้ามติดลบ, `line-height` ≥ 1.3, ใช้ `overflow-wrap:break-word` เท่านั้น
- **ห้ามเขียน `-->` ใน `<script>` ของ Go template** (เคยทำคลิปจอเปล่าขึ้น YouTube ทั้งคลิป)
- สีแบรนด์คงที่ทุกโหมด: navy `#0047AF` / amber `#F0A030` — โหมดต่างกันที่ media/font/motion เท่านั้น
- รันเทสต์ด้วย `go test ./internal/...` (ทั้ง repo ต้องเขียว 14/14 package ก่อน commit)

---

### Task 1: โครงข้อมูลโหมดแชท (`chat`)

**Files:**
- Create: `internal/producer/chat_format.go`
- Create: `internal/producer/chat_format_test.go`
- Modify: `internal/producer/case_format.go` (const block โหมด + `imageScenesForMode` + `promptForScene`)
- Modify: `internal/producer/composition_types.go` (ฟิลด์ใหม่)
- Modify: `internal/producer/presets.go` (`PresetByKey`)
- Modify: `internal/agent/scene_content.go` (`sceneLayouts`)

**Interfaces:**
- Produces: `producer.ModeChat = "chat"`, `producer.ChatPreset` (StylePreset, Key `"chat"`), `producer.ContentMessage{From, Text, Alert}`, ฟิลด์ `SceneContent.Msgs/Asker/Verdict`, layout `chat_in`/`chat_out`/`recap`
- Consumes: `producer.StylePreset`, `producer.Brand`, `producer.buildImagePromptCore` (มีอยู่แล้ว)

- [ ] **Step 1: เขียนเทสต์ที่ยังไม่ผ่าน**

สร้าง `internal/producer/chat_format_test.go`:

```go
package producer

import (
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

func TestChatPresetResolvesAndKeepsBrand(t *testing.T) {
	if PresetByKey("chat").Key != "chat" {
		t.Error("PresetByKey must resolve chat (resume path)")
	}
	if ChatPreset.Palette != Brand {
		t.Error("chat must keep the brand navy+orange palette")
	}
	if ChatPreset.BrandCSS() == "" {
		t.Error("chat BrandCSS must render")
	}
	if ChatPreset.HeadingFont.HeadingFamily == "" {
		t.Error("chat must set a heading font")
	}
}

func TestChatLayoutsAreAccepted(t *testing.T) {
	for _, l := range []string{"chat_in", "chat_out", "recap"} {
		if agent.ClampLayout(l) != l {
			t.Errorf("ClampLayout(%q) = %q, want it accepted", l, agent.ClampLayout(l))
		}
	}
}

func TestImageScenesForModeChatCapsAtOne(t *testing.T) {
	scenes := []agent.GeneratedScene{
		{SceneNumber: 1, Layout: "hook", ImagePrompt: "a marketer at night"},
		{SceneNumber: 2, Layout: "chat_in", ImagePrompt: "a phone screen"},
		{SceneNumber: 3, Layout: "chat_out", ImagePrompt: "another shot"},
	}
	got := imageScenesForMode(scenes, ModeChat)
	if len(got) != 1 || !got[1] {
		t.Errorf("chat mode must allow exactly scene 1, got %v", got)
	}
}

func TestChatCoverPromptIsPortraitNotForensic(t *testing.T) {
	out := promptForScene(
		agent.GeneratedScene{SceneNumber: 1, Layout: "hook", ImagePrompt: "a worried shop owner holding a phone"},
		ChatPreset, "clip-x", ModeChat)
	if !strings.Contains(out, "a worried shop owner holding a phone") {
		t.Errorf("cover prompt lost the subject: %s", out)
	}
	if !strings.Contains(out, "clip-x") {
		t.Error("cover prompt must keep the cohesion style-set token")
	}
	if strings.Contains(strings.ToLower(out), "forensic") {
		t.Error("chat cover must not inherit the case-file forensic anchor")
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่าไม่ผ่าน**

```bash
go test ./internal/producer/ -run 'TestChat|TestImageScenesForModeChat' 2>&1 | head -20
```

Expected: FAIL — `undefined: ChatPreset`, `undefined: ModeChat`

- [ ] **Step 3: เพิ่ม const โหมดกับนโยบายภาพ**

ใน `internal/producer/case_format.go` แก้ const block ให้เป็น:

```go
// Content mode of a clip. Derived per-clip (from clips.content_format), never
// from a process-wide flag alone — so clips of different modes can be produced
// by the same running server.
const (
	ModeClassic  = ""
	ModeCase     = "case"
	ModeTutorial = "tutorial"
	ModeChat     = "chat"
	ModeWarRoom  = "warroom"
)
```

ในฟังก์ชัน `imageScenesForMode` เพิ่ม case ใหม่ก่อน `default:` (ใช้กติกาเดียวกับ tutorial — ภาพเดียวที่ซีนแรกที่มี prompt):

```go
	case ModeTutorial, ModeChat, ModeWarRoom:
		for _, s := range scenes {
			if strings.TrimSpace(s.ImagePrompt) != "" {
				return map[int]bool{s.SceneNumber: true}
			}
		}
		return map[int]bool{}
```

(ลบ `case ModeTutorial:` เดิมออก — รวมเป็นบรรทัดเดียวข้างบน)

ในฟังก์ชัน `promptForScene` เพิ่ม case เดียว (warroom เพิ่มใน Task 2 พร้อมตัวฟังก์ชัน
— ใส่ตอนนี้จะคอมไพล์ไม่ผ่านเพราะ `buildWarRoomCoverPrompt` ยังไม่มี):

```go
	case ModeChat:
		return buildChatCoverPrompt(s.ImagePrompt, preset, clipToken)
```

- [ ] **Step 4: สร้าง preset กับ prompt builder ของโหมดแชท**

สร้าง `internal/producer/chat_format.go`:

```go
package producer

// ChatPreset is the visual identity of the chat format: a customer's message
// thread. Palette stays Brand (navy + orange) — only the mood changes. Motion is
// quick and light: a chat bubble should pop in, not glide cinematically.
var ChatPreset = StylePreset{
	Key:         "chat",
	DisplayName: "Customer Chat",
	Palette:     Brand,
	ImageAnchor: "Cinematic editorial PHOTOGRAPHY, shot on 50mm, a real person at night lit mainly by " +
		"the blue glow of a phone screen against a deep-navy #0047AF ambience, one warm amber #F0A030 " +
		"practical light in the background. Candid, worried, human. Photorealistic, premium, restrained. " +
		"NO illustration, NO 3D render, NO cartoon, no text, no user interface, no readable screen content. " +
		"Atmosphere: the message you send when the money already stopped moving.",
	Font:        TypeTokens{Family: "Sarabun", HeadingFamily: "Kanit"},
	HeadingFont: TypeTokens{Family: "Sarabun", HeadingFamily: "Kanit"},
	Motion:      MotionProfile{EntranceDur: 0.32, EntranceEase: "power2.out", BGZoomTo: 1.03},
}

// buildChatCoverPrompt renders the image prompt for the ONE AI image a chat clip
// gets: the opening portrait. Composition reserves the lower half for the hook
// copy; every later scene is a CSS message thread and gets no image.
func buildChatCoverPrompt(concept string, preset StylePreset, clipToken string) string {
	return buildImagePromptCore(concept,
		"cinematic portrait, the subject placed in the UPPER half of the frame, lower half dark and uncluttered",
		preset, clipToken)
}
```

- [ ] **Step 5: เพิ่ม layout ใหม่กับฟิลด์เนื้อหา**

ใน `internal/agent/scene_content.go` แก้ `sceneLayouts`:

```go
var sceneLayouts = map[string]bool{
	"hook": true, "hero": true, "stat": true, "step": true, "tip": true, "cta": true,
	// case-file format (spec 2026-07-24): investigation storytelling layouts
	"casefile": true, "comic": true, "evidence": true, "board": true, "verdict": true,
	// tutorial format (spec 2026-07-25): simulated Ads Manager UI walkthrough
	"uistep": true,
	// chat format (spec 2026-07-28): a customer's message thread
	"chat_in": true, "chat_out": true, "recap": true,
}
```

ใน `internal/producer/composition_types.go` เพิ่มท้าย struct `SceneContent` (ก่อนปีกกาปิด):

```go
	// chat format (spec 2026-07-28). Msgs drives chat_in/chat_out; Asker names
	// the customer in the thread header; Verdict is the green summary chip.
	Msgs    []ContentMessage `json:"msgs,omitempty"`
	Asker   string           `json:"asker,omitempty"`
	Verdict string           `json:"verdict,omitempty"`
```

และเพิ่ม type ใหม่ถัดจาก `ContentChip`:

```go
// ContentMessage is one bubble in a chat-format thread. From is "them" (the
// customer, left) or "me" (us, right). Alert=true tints the bubble red — used
// for the detail that signals danger, not for every negative sentence.
type ContentMessage struct {
	From  string `json:"from"`
	Text  string `json:"t"`
	Alert bool   `json:"alert,omitempty"`
}
```

ใน `internal/producer/presets.go` แก้ `PresetByKey` เป็นเวอร์ชันนี้ (ยังไม่มี warroom — Task 2 จะเติม case ให้เอง):

```go
func PresetByKey(key string) StylePreset {
	switch key {
	case TutorialPreset.Key:
		return TutorialPreset
	case ChatPreset.Key:
		return ChatPreset
	}
	return CaseFilePreset
}
```

- [ ] **Step 6: รันเทสต์ให้ผ่าน**

```bash
go test ./internal/producer/ ./internal/agent/ 2>&1 | grep -E '^(ok|FAIL|---)'
```

Expected: `ok` ทั้งสอง package

- [ ] **Step 7: commit**

```bash
git add internal/producer/chat_format.go internal/producer/chat_format_test.go \
  internal/producer/case_format.go internal/producer/composition_types.go \
  internal/producer/presets.go internal/agent/scene_content.go
git commit -m "feat(chat): โครงข้อมูลโหมดแชท — preset, layout, นโยบายภาพ 1 ใบ"
```

---

### Task 2: โครงข้อมูลโหมดห้องควบคุม (`warroom`)

**Files:**
- Create: `internal/producer/warroom_format.go`
- Create: `internal/producer/warroom_format_test.go`
- Modify: `internal/producer/composition_types.go` (`ContentChip.Bad`)
- Modify: `internal/producer/presets.go` (เติม case `WarRoomPreset`)
- Modify: `internal/agent/scene_content.go` (`sceneLayouts`)

**Interfaces:**
- Consumes: `ModeWarRoom` (สร้างแล้วใน Task 1), `promptForScene` case `ModeWarRoom` ที่เรียก `buildWarRoomCoverPrompt`
- Produces: `producer.WarRoomPreset` (Key `"warroom"`), `buildWarRoomCoverPrompt`, layout `dashboard`/`alarm`, ฟิลด์ `ContentChip.Bad`

- [ ] **Step 1: เขียนเทสต์ที่ยังไม่ผ่าน**

สร้าง `internal/producer/warroom_format_test.go`:

```go
package producer

import (
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

func TestWarRoomPresetResolvesAndKeepsBrand(t *testing.T) {
	if PresetByKey("warroom").Key != "warroom" {
		t.Error("PresetByKey must resolve warroom (resume path)")
	}
	if WarRoomPreset.Palette != Brand {
		t.Error("warroom must keep the brand navy+orange palette")
	}
	if WarRoomPreset.BrandCSS() == "" {
		t.Error("warroom BrandCSS must render")
	}
}

func TestWarRoomLayoutsAreAccepted(t *testing.T) {
	for _, l := range []string{"dashboard", "alarm"} {
		if agent.ClampLayout(l) != l {
			t.Errorf("ClampLayout(%q) = %q, want it accepted", l, agent.ClampLayout(l))
		}
	}
}

// ห้องควบคุมเป็นคลิปสอน — uistep ต้องยังใช้ได้ ไม่งั้นตะแกรง ui_vocab เปิดค้าง
func TestWarRoomKeepsUIStepLayout(t *testing.T) {
	if agent.ClampLayout("uistep") != "uistep" {
		t.Fatal("uistep must remain a valid layout — the tutorial gate counts it")
	}
}

func TestImageScenesForModeWarRoomCapsAtOne(t *testing.T) {
	scenes := []agent.GeneratedScene{
		{SceneNumber: 1, Layout: "dashboard", ImagePrompt: "a night control room"},
		{SceneNumber: 2, Layout: "uistep", ImagePrompt: "a menu"},
	}
	got := imageScenesForMode(scenes, ModeWarRoom)
	if len(got) != 1 || !got[1] {
		t.Errorf("warroom mode must allow exactly scene 1, got %v", got)
	}
}

func TestWarRoomCoverPromptHasNoReadableScreens(t *testing.T) {
	out := promptForScene(
		agent.GeneratedScene{SceneNumber: 1, Layout: "dashboard", ImagePrompt: "a desk with three glowing monitors"},
		WarRoomPreset, "clip-y", ModeWarRoom)
	if !strings.Contains(out, "a desk with three glowing monitors") {
		t.Errorf("cover prompt lost the subject: %s", out)
	}
	if !strings.Contains(strings.ToLower(out), "no readable") {
		t.Error("warroom cover must forbid readable screen content (AI renders fake Thai as garbage)")
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่าไม่ผ่าน**

```bash
go test ./internal/producer/ -run TestWarRoom 2>&1 | head -20
```

Expected: FAIL — `undefined: WarRoomPreset`

- [ ] **Step 3: สร้าง preset กับ prompt builder**

สร้าง `internal/producer/warroom_format.go`:

```go
package producer

// WarRoomPreset is the visual identity of the war-room format: monitors, live
// numbers, warning lights. It is the advanced tutorial's skin — the uistep
// layout still carries every teaching step, only the frame around it changes,
// so the ui_vocab gate keeps working untouched.
var WarRoomPreset = StylePreset{
	Key:         "warroom",
	DisplayName: "War Room",
	Palette:     Brand,
	ImageAnchor: "Cinematic editorial PHOTOGRAPHY, shot on 35mm, a dim night workspace with several " +
		"monitors glowing deep-navy #0047AF blue, one warm amber #F0A030 desk lamp, cables and a " +
		"coffee cup. Photorealistic, technical, premium. NO illustration, NO 3D render, NO cartoon, " +
		"no text, no logos, no readable screen content — screens are pure abstract glow. " +
		"Atmosphere: someone is watching the numbers move at 2am.",
	Font:        TypeTokens{Family: "Prompt", HeadingFamily: "Kanit"},
	HeadingFont: TypeTokens{Family: "Prompt", HeadingFamily: "Kanit"},
	Motion:      MotionProfile{EntranceDur: 0.26, EntranceEase: "power4.out", BGZoomTo: 1.03},
}

// buildWarRoomCoverPrompt renders the image prompt for the ONE AI image a
// war-room clip gets: the opening establishing shot. Every later scene is a CSS
// monitor and gets no image — a photo there would fight the menu the viewer has
// to read.
func buildWarRoomCoverPrompt(concept string, preset StylePreset, clipToken string) string {
	return buildImagePromptCore(concept,
		"cinematic wide shot, the key subject placed in the UPPER half of the frame, lower half dark and uncluttered",
		preset, clipToken)
}
```

- [ ] **Step 4: เพิ่ม layout + ฟิลด์ KPI สีแดง + เติม PresetByKey**

ใน `internal/agent/scene_content.go` เติมท้าย `sceneLayouts`:

```go
	// war-room format (spec 2026-07-28): live dashboard wrapping the uistep walkthrough
	"dashboard": true, "alarm": true,
```

ใน `internal/producer/composition_types.go` แก้ `ContentChip`:

```go
// ContentChip is one small stat chip beneath a stat card, and one KPI tile in
// the war-room dashboard. Bad=true renders the number in red (a metric moving
// the wrong way).
type ContentChip struct {
	N   string `json:"n"`
	T   string `json:"t"`
	Bad bool   `json:"bad,omitempty"`
}
```

ใน `internal/producer/presets.go` เติม case ที่ Task 1 เว้นไว้:

```go
	case WarRoomPreset.Key:
		return WarRoomPreset
```

ใน `internal/producer/case_format.go` ฟังก์ชัน `promptForScene` เติม case ที่ Task 1
เว้นไว้ (ตอนนี้ `buildWarRoomCoverPrompt` มีแล้วจาก Step 3):

```go
	case ModeWarRoom:
		return buildWarRoomCoverPrompt(s.ImagePrompt, preset, clipToken)
```

- [ ] **Step 5: รันเทสต์ให้ผ่าน**

```bash
go test ./internal/producer/ ./internal/agent/ 2>&1 | grep -E '^(ok|FAIL|---)'
```

Expected: `ok` ทั้งสอง package

- [ ] **Step 6: commit**

```bash
git add internal/producer/warroom_format.go internal/producer/warroom_format_test.go \
  internal/producer/composition_types.go internal/producer/presets.go internal/agent/scene_content.go
git commit -m "feat(warroom): โครงข้อมูลโหมดห้องควบคุม — preset, dashboard/alarm, uistep คงเดิม"
```

---

### Task 3: หน้าตาโหมดแชทใน template

**Files:**
- Modify: `internal/producer/templates/layout_multi_scene.html.tmpl` (CSS block + JS const + JS renderer)
- Create: `internal/producer/composition_chat_render_test.go`

**Interfaces:**
- Consumes: `SceneContent.Msgs/Asker/Verdict/Stamp/Chips`, `ScenesParams.Format` = `"chat"`
- Produces: HTML ที่มี `data-format="chat"`, `const FORMAT_CHAT = true`, และ markup class `.ch-bub`, `.ch-head`, `.ch-verdict`

- [ ] **Step 1: เขียนเทสต์เรนเดอร์ที่ยังไม่ผ่าน**

สร้าง `internal/producer/composition_chat_render_test.go`:

```go
package producer

import (
	"strings"
	"testing"
)

func chatParams() ScenesParams {
	mk := func(n int, layout string, c SceneContent) SceneSpec {
		c.SceneNumber, c.Layout = n, layout
		c.Start, c.End = float64(n-1)*5, float64(n)*5
		return SceneSpec{SceneNumber: n, StartSec: c.Start, EndSec: c.End,
			LayoutVariant: "hook_big", CaptionStyle: "phrase_block", Content: c}
	}
	return ScenesParams{
		AspectRatio: "9:16", BrandName: "ADS VANCE", VoiceSrc: "assets/voice.wav",
		DurationSeconds: 25, Format: "chat", ThemeKey: "chat",
		Scenes: []SceneSpec{
			mk(1, "hook", SceneContent{Rows: []ContentRow{{Text: "บัญชีโดนแบนตอนตีสอง"}},
				BackgroundImage: "assets/bg-scene1.png"}),
			mk(2, "chat_in", SceneContent{
				Asker: "คุณเก่ง", Stamp: "21:47 น.",
				Msgs: []ContentMessage{
					{From: "them", Text: "พี่ครับ บัญชีโดนแบน"},
					{From: "them", Text: "เพิ่งผูกบัตรใหม่เมื่อวาน", Alert: true},
				}}),
			mk(3, "chat_out", SceneContent{
				Msgs: []ContentMessage{
					{From: "me", Text: "อย่าเพิ่งยื่นอุทธรณ์ครับ"},
					{From: "them", Text: "แล้วทำไงดี"},
				},
				Verdict: "รอ 72 ชั่วโมงแล้วยื่นครั้งเดียว"}),
			mk(4, "recap", SceneContent{Title: "จำ 2 ข้อนี้",
				Chips: []ContentChip{{N: "72", T: "ชั่วโมงที่ต้องรอ"}, {N: "1", T: "ครั้งเท่านั้น"}}}),
		},
		Segments: []TranscriptSegment{{Text: "บัญชีโดนแบน", Start: 0, End: 2}},
	}
}

func TestRenderChatFormat(t *testing.T) {
	out, err := RenderCompositionScenes(chatParams())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)
	for _, want := range []string{
		`data-format="chat"`,
		"const FORMAT_CHAT = true",
		`data-layout="chat_in"`,
		`data-layout="chat_out"`,
		"คุณเก่ง",
		"21:47 น.",
		"พี่ครับ บัญชีโดนแบน",
		"รอ 72 ชั่วโมงแล้วยื่นครั้งเดียว",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered chat HTML missing %q", want)
		}
	}
}

// ฟองข้อความของเรากับของลูกค้าต้องแยกกันได้ในผลลัพธ์ ไม่งั้นคลิปอ่านไม่รู้ว่าใครพูด
func TestChatBubblesCarrySide(t *testing.T) {
	out, _ := RenderCompositionScenes(chatParams())
	html := string(out)
	if !strings.Contains(html, "ch-bub") {
		t.Fatal("chat bubbles must render with the ch-bub class")
	}
	for _, want := range []string{`"from":"them"`, `"from":"me"`} {
		if !strings.Contains(html, want) {
			t.Errorf("SCENES JSON missing %q — the renderer cannot tell the sides apart", want)
		}
	}
}

// โหมดอื่นต้องไม่ติดธง FORMAT_CHAT
func TestChatFlagOffForOtherModes(t *testing.T) {
	p := chatParams()
	p.Format, p.ThemeKey = "case", "case-file"
	out, _ := RenderCompositionScenes(p)
	if !strings.Contains(string(out), "const FORMAT_CHAT = false") {
		t.Error("FORMAT_CHAT must be false when the clip is not a chat clip")
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่าไม่ผ่าน**

```bash
go test ./internal/producer/ -run TestRenderChatFormat 2>&1 | head -15
```

Expected: FAIL — ไม่พบ `const FORMAT_CHAT = true`

- [ ] **Step 3: เพิ่ม CSS ของโหมดแชท**

ใน `internal/producer/templates/layout_multi_scene.html.tmpl` แทรกบล็อกนี้**ต่อท้าย CSS ของ tutorial** (หลังบรรทัดที่ขึ้นต้นด้วย `[data-format='tutorial'] .scene[data-layout="cta"] .scene-bg`) และ**ก่อน** `*{margin:0;padding:0;box-sizing:border-box}`:

```css
      /* ── chat format (data-format='chat') ──
         Thai-safe: letter-spacing >= 0, line-height >= 1.3, break-word เท่านั้น */
      [data-format='chat'] .scene[data-layout="chat_in"] .scene-content,
      [data-format='chat'] .scene[data-layout="chat_out"] .scene-content{
        top:150px;bottom:400px;justify-content:flex-start}
      [data-format='chat'] .scene[data-layout="chat_in"] .scene-bg,
      [data-format='chat'] .scene[data-layout="chat_out"] .scene-bg,
      [data-format='chat'] .scene[data-layout="recap"] .scene-bg{opacity:.28;filter:saturate(.6)}
      [data-format='chat'] .scene[data-layout="recap"] .scene-content{
        top:150px;bottom:430px;justify-content:center}
      .ch-head{display:flex;align-items:center;gap:20px;padding-bottom:26px;
        border-bottom:2px solid rgba(255,255,255,.12);margin-bottom:26px}
      .ch-ava{flex:0 0 auto;width:74px;height:74px;border-radius:50%;
        background:linear-gradient(135deg,var(--amber),var(--amber-soft));
        display:flex;align-items:center;justify-content:center;
        font-weight:800;font-size:38px;color:#11182b}
      .ch-name{font-weight:800;font-size:40px;line-height:1.32;color:#fff}
      .ch-stamp{font-weight:600;font-size:28px;line-height:1.34;color:var(--muted)}
      .ch-thread{display:flex;flex-direction:column;gap:22px}
      .ch-bub{max-width:82%;padding:26px 30px;border-radius:34px;
        font-weight:700;font-size:38px;line-height:1.38;overflow-wrap:break-word}
      .ch-bub.them{align-self:flex-start;background:rgba(28,43,79,.94);
        border:1px solid rgba(255,255,255,.10);border-bottom-left-radius:10px;color:#dbe5fb}
      .ch-bub.them.alert{background:rgba(255,107,98,.16);
        border-color:rgba(255,107,98,.55);color:#ffd9d6}
      .ch-bub.me{align-self:flex-end;color:#11182b;border-bottom-right-radius:10px;
        background:linear-gradient(135deg,var(--amber),var(--amber-soft))}
      .ch-verdict{margin-top:28px;background:rgba(74,222,128,.14);
        border:2px solid rgba(74,222,128,.55);border-radius:26px;padding:26px 30px;
        font-weight:800;font-size:38px;line-height:1.38;color:#c9f7d8;overflow-wrap:break-word}
```

- [ ] **Step 4: เพิ่มธง FORMAT_CHAT กับ renderer**

หาบรรทัด `const FORMAT_TUTORIAL = ...` (ประมาณบรรทัด 326) แล้วเพิ่มถัดลงมา:

```javascript
      const FORMAT_CHAT = {{if eq .Format "chat"}}true{{else}}false{{end}};
      const FORMAT_WARROOM = {{if eq .Format "warroom"}}true{{else}}false{{end}};
```

ในนิพจน์ `reuseCover` (ประมาณบรรทัด 347-349) เพิ่มเงื่อนไขของสองโหมดใหม่ ให้ทั้งก้อนเป็น:

```javascript
        const reuseCover =
          (FORMAT_CASE && (sc.type === "hero" || sc.type === "verdict")) ||
          (FORMAT_TUTORIAL && (sc.type === "hero" || sc.type === "tip" || sc.type === "cta")) ||
          (FORMAT_CHAT && (sc.type === "chat_in" || sc.type === "chat_out" || sc.type === "recap")) ||
          (FORMAT_WARROOM && (sc.type === "dashboard" || sc.type === "alarm" || sc.type === "cta"));
```

ในลูก `SCENES.forEach((sc,i)=>{...})` ตัวที่สร้างเนื้อหา เพิ่ม branch ต่อจาก `else if(sc.type==="uistep"){...}` (ก่อน branch ถัดไป):

```javascript
        else if(sc.type==="chat_in" || sc.type==="chat_out"){
          if(sc.asker){
            const head=el("div","ch-head");
            head.appendChild(el("div","ch-ava",(sc.asker||"?").trim().charAt(0)));
            const nw=el("div");
            nw.appendChild(el("div","ch-name",sc.asker));
            if(sc.stamp) nw.appendChild(el("div","ch-stamp",sc.stamp));
            head.appendChild(nw);
            c.appendChild(head);
          }
          const th=el("div","ch-thread");
          (sc.msgs||[]).forEach(m=>{
            const side=(m.from==="me")?"me":"them";
            th.appendChild(el("div","ch-bub "+side+(m.alert?" alert":""),m.t));
          });
          c.appendChild(th);
          if(sc.verdict) c.appendChild(el("div","ch-verdict",sc.verdict));
        }
        else if(sc.type==="recap"){
          if(sc.title) c.appendChild(el("h1","title",sc.title));
          if(sc.chips) c.appendChild(chipsBlock(sc.chips));
        }
```

- [ ] **Step 5: รันเทสต์ให้ผ่าน**

```bash
go test ./internal/producer/ -run 'TestRenderChatFormat|TestChatBubbles|TestChatFlagOff' -v 2>&1 | grep -E '^(=== RUN|--- (PASS|FAIL)|ok|FAIL)'
```

Expected: PASS ทั้ง 3 ตัว

- [ ] **Step 6: รันเทสต์ทั้ง package กันของเดิมพัง**

```bash
go test ./internal/... 2>&1 | grep -E '^(FAIL|---)' ; echo "ok count: $(go test ./internal/... 2>&1 | grep -c '^ok')"
```

Expected: ไม่มี FAIL

- [ ] **Step 7: commit**

```bash
git add internal/producer/templates/layout_multi_scene.html.tmpl \
  internal/producer/composition_chat_render_test.go
git commit -m "feat(chat): หน้าตาโหมดแชทใน template — ฟองข้อความ หัวแชท สรุปเขียว"
```

---

### Task 4: หน้าตาโหมดห้องควบคุมใน template (ห่อ uistep)

**Files:**
- Modify: `internal/producer/templates/layout_multi_scene.html.tmpl` (CSS block + JS renderer)
- Create: `internal/producer/composition_warroom_render_test.go`

**Interfaces:**
- Consumes: `SceneContent.StatLabel/Chips/Callout/Title/Rows/Panel`, `ScenesParams.Format` = `"warroom"`, ธง `FORMAT_WARROOM` (เพิ่มแล้วใน Task 3)
- Produces: HTML ที่มี `data-format="warroom"` + markup class `.wr-gauge`, `.wr-kpi`, `.wr-alarm` และ `uistep` เดิมที่ถูกห่อ

- [ ] **Step 1: เขียนเทสต์เรนเดอร์ที่ยังไม่ผ่าน**

สร้าง `internal/producer/composition_warroom_render_test.go`:

```go
package producer

import (
	"strings"
	"testing"
)

func warroomParams() ScenesParams {
	mk := func(n int, layout string, c SceneContent) SceneSpec {
		c.SceneNumber, c.Layout = n, layout
		c.Start, c.End = float64(n-1)*5, float64(n)*5
		return SceneSpec{SceneNumber: n, StartSec: c.Start, EndSec: c.End,
			LayoutVariant: "hook_big", CaptionStyle: "phrase_block", Content: c}
	}
	return ScenesParams{
		AspectRatio: "9:16", BrandName: "ADS VANCE", VoiceSrc: "assets/voice.wav",
		DurationSeconds: 30, Format: "warroom", ThemeKey: "warroom",
		Scenes: []SceneSpec{
			mk(1, "dashboard", SceneContent{
				StatLabel: "CPM 7 วันล่าสุด",
				Chips:     []ContentChip{{N: "+38%", T: "CPM", Bad: true}, {N: "1.4x", T: "ROAS"}},
				Callout:   "ค่าโฆษณาแพงขึ้น 38% ใน 3 วัน",
				BackgroundImage: "assets/bg-scene1.png"}),
			mk(2, "uistep", SceneContent{
				Num: "1", Of: "ขั้นที่ 1 / 2", StepTotal: 2, Title: "เปิดกฎอัตโนมัติ",
				Panel: &ContentUIPanel{
					Chrome: "Ads Manager", Breadcrumb: "Rules › Create new rule",
					Items: []ContentUIItem{
						{Label: "Campaigns", State: "normal"},
						{Label: "Automated Rules", State: "target"},
					},
					Field: &ContentUIField{Label: "Greater than", Value: "400 THB"},
				},
				Callout: "เลือก Automated Rules"}),
			mk(3, "alarm", SceneContent{Title: "ตั้งเพดานไว้ก่อนนอน",
				Rows: []ContentRow{{Text: "CPM ทะลุเพดาน = หยุดแอดอัตโนมัติ"},
					{Text: "ไม่ตั้ง = จ่ายทั้งคืน", Bad: true}}}),
		},
		Segments: []TranscriptSegment{{Text: "ค่าโฆษณาแพงขึ้น", Start: 0, End: 2}},
	}
}

func TestRenderWarRoomFormat(t *testing.T) {
	out, err := RenderCompositionScenes(warroomParams())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)
	for _, want := range []string{
		`data-format="warroom"`,
		"const FORMAT_WARROOM = true",
		`data-layout="dashboard"`,
		`data-layout="alarm"`,
		"CPM 7 วันล่าสุด",
		"ค่าโฆษณาแพงขึ้น 38% ใน 3 วัน",
		"wr-kpi",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered warroom HTML missing %q", want)
		}
	}
}

// หัวใจของสเปก: ห้องควบคุมต้องยังเรนเดอร์ uistep ด้วย markup เดิม เพราะตะแกรง
// ui_vocab กับตัวนับขั้นตอนนับเฉพาะซีน uistep — ถ้าหายไปคลิปสอนผิดจะหลุดขึ้นช่อง
func TestWarRoomStillRendersUIStep(t *testing.T) {
	out, _ := RenderCompositionScenes(warroomParams())
	html := string(out)
	for _, want := range []string{`data-layout="uistep"`, "ui-panel", "Automated Rules", "ui-rail"} {
		if !strings.Contains(html, want) {
			t.Errorf("warroom must keep the uistep walkthrough intact — missing %q", want)
		}
	}
}

// KPI ที่แย่ต้องมีธง bad ติดไปถึง JSON ไม่งั้นเลขแดงหายทั้งโหมด
func TestWarRoomKPICarriesBadFlag(t *testing.T) {
	out, _ := RenderCompositionScenes(warroomParams())
	if !strings.Contains(string(out), `"bad":true`) {
		t.Error("SCENES JSON must carry the bad flag for a failing KPI")
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่าไม่ผ่าน**

```bash
go test ./internal/producer/ -run TestRenderWarRoomFormat 2>&1 | head -15
```

Expected: FAIL — ไม่พบ `wr-kpi`

- [ ] **Step 3: เพิ่ม CSS ของห้องควบคุม**

แทรกต่อท้าย CSS ของโหมดแชท (ก่อน `*{margin:0;padding:0;box-sizing:border-box}`):

```css
      /* ── war-room format (data-format='warroom') ──
         uistep ใช้ markup เดิมทุกอย่าง เปลี่ยนแค่กรอบที่ห่อมัน เพื่อให้ตะแกรง
         ui_vocab + ตัวนับขั้นตอนทำงานเหมือนเดิมโดยไม่ต้องแก้โค้ดตะแกรง */
      [data-format='warroom'] .scene-bg::after{content:"";position:absolute;inset:0;
        background-image:linear-gradient(rgba(46,139,255,.09) 1px,transparent 1px),
          linear-gradient(90deg,rgba(46,139,255,.09) 1px,transparent 1px);background-size:96px 96px}
      [data-format='warroom'] .scene[data-layout="dashboard"] .scene-content{
        top:170px;bottom:420px;justify-content:center}
      [data-format='warroom'] .scene[data-layout="alarm"] .scene-content,
      [data-format='warroom'] .scene[data-layout="uistep"] .scene-content{
        top:160px;bottom:410px;justify-content:center}
      [data-format='warroom'] .scene[data-layout="alarm"] .scene-bg,
      [data-format='warroom'] .scene[data-layout="cta"] .scene-bg{opacity:.30;filter:saturate(.6)}
      [data-format='warroom'] .ui-panel{background:#050d22;border-color:rgba(88,146,255,.42)}
      .wr-live{display:flex;align-items:center;gap:14px;font-weight:800;font-size:28px;
        letter-spacing:.10em;color:var(--muted);margin-bottom:20px}
      .wr-live i{width:18px;height:18px;border-radius:50%;background:var(--red);
        box-shadow:0 0 18px rgba(255,107,98,.85)}
      .wr-gauge{background:rgba(9,22,52,.88);border:2px solid rgba(88,146,255,.34);
        border-radius:26px;padding:28px 30px}
      .wr-gauge-l{font-weight:700;font-size:30px;line-height:1.34;color:var(--muted)}
      .wr-spark{height:150px;margin-top:18px;
        background:linear-gradient(180deg,rgba(255,180,84,.34),transparent);
        clip-path:polygon(0 76%,13% 60%,26% 68%,39% 40%,52% 52%,65% 22%,78% 34%,91% 10%,100% 16%,100% 100%,0 100%)}
      .wr-kpis{display:flex;gap:18px;margin-top:22px}
      .wr-kpi{flex:1;background:rgba(9,22,52,.88);border:2px solid rgba(88,146,255,.28);
        border-radius:22px;padding:24px 20px}
      .wr-kpi .n{font-weight:800;font-size:64px;line-height:1.3;color:var(--amber-bright)}
      .wr-kpi .n.bad{color:var(--red)}
      .wr-kpi .t{font-weight:700;font-size:26px;line-height:1.34;color:var(--muted);margin-top:6px}
      .wr-alarm{margin-top:24px;background:rgba(255,107,98,.16);border-left:10px solid var(--red);
        border-radius:0 22px 22px 0;padding:26px 30px;font-weight:800;font-size:38px;
        line-height:1.38;color:#ffd9d6;overflow-wrap:break-word}
```

- [ ] **Step 4: เพิ่ม renderer ของ dashboard กับ alarm**

เพิ่ม branch ต่อจาก branch `recap` ที่เพิ่มไว้ใน Task 3:

```javascript
        else if(sc.type==="dashboard"){
          const live=el("div","wr-live");live.appendChild(el("i"));
          live.appendChild(el("span",null,"LIVE"));c.appendChild(live);
          const g=el("div","wr-gauge");
          if(sc.statLabel) g.appendChild(el("div","wr-gauge-l",sc.statLabel));
          g.appendChild(el("div","wr-spark"));
          c.appendChild(g);
          if(sc.chips&&sc.chips.length){
            const k=el("div","wr-kpis");
            sc.chips.forEach(ch=>{
              const t=el("div","wr-kpi");
              t.appendChild(el("div","n"+(ch.bad?" bad":""),ch.n));
              t.appendChild(el("div","t",ch.t));
              k.appendChild(t);
            });
            c.appendChild(k);
          }
          if(sc.callout) c.appendChild(el("div","wr-alarm",sc.callout));
        }
        else if(sc.type==="alarm"){
          if(sc.title) c.appendChild(el("h1","title",sc.title));
          c.appendChild(rowsBlock(sc.rows));
        }
```

- [ ] **Step 5: รันเทสต์ให้ผ่าน**

```bash
go test ./internal/producer/ -run 'TestRenderWarRoom|TestWarRoomStill|TestWarRoomKPI' -v 2>&1 | grep -E '^(--- (PASS|FAIL)|ok|FAIL)'
```

Expected: PASS ทั้ง 3 ตัว

- [ ] **Step 6: รันเทสต์ทั้ง repo**

```bash
go test ./internal/... 2>&1 | grep -E '^(FAIL|---)'
```

Expected: ไม่มีผลลัพธ์

- [ ] **Step 7: commit**

```bash
git add internal/producer/templates/layout_multi_scene.html.tmpl \
  internal/producer/composition_warroom_render_test.go
git commit -m "feat(warroom): หน้าตาห้องควบคุม — จอ กราฟ KPI ไฟเตือน โดย uistep คงเดิม"
```

---

### Task 5: ล็อก content_format ต่อ slot

**Files:**
- Modify: `internal/repository/formats.go` (`PickNextIn`)
- Create: `internal/repository/formats_allowed_test.go`
- Modify: `internal/orchestrator/orchestrator.go` (`ProduceWeekly` รับ allowed + fallback ภายใน allowed)
- Modify: `internal/scheduler/scheduler.go` (ส่ง allowed ต่อ slot)
- Modify: `internal/handler/orchestrator.go:53`, `cmd/server/main.go:140` (call site เดิม)

**Interfaces:**
- Produces: `(*FormatsRepo).PickNextIn(ctx context.Context, allowed []string) (*models.ContentFormat, error)`, `(*Orchestrator).ProduceWeekly(ctx context.Context, count int, allowed []string) error`
- Consumes: `pickLeastUsed` (มีอยู่แล้ว), `models.ContentFormat`

- [ ] **Step 1: เขียนเทสต์ที่ยังไม่ผ่าน**

สร้าง `internal/repository/formats_allowed_test.go`:

```go
package repository

import (
	"testing"

	"github.com/jaochai/video-fb/internal/models"
)

// filterAllowed เป็นตัวกรองบริสุทธิ์ เทสต์ได้โดยไม่ต้องมี DB — กติกาที่ห้ามพลาดคือ
// "ห้ามคืน format นอกชุดที่ slot อนุญาต" เพราะ format คือตัวกำหนดโหมดของคลิป
func TestFilterAllowedKeepsOnlyListed(t *testing.T) {
	usages := []models.FormatUsage{
		{Format: models.ContentFormat{FormatName: "qa", Weight: 1}, UsedCount: 5},
		{Format: models.ContentFormat{FormatName: "tips", Weight: 1}, UsedCount: 1},
		{Format: models.ContentFormat{FormatName: "case_story", Weight: 1}, UsedCount: 0},
	}
	got := filterAllowed(usages, []string{"qa", "tips"})
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	for _, u := range got {
		if u.Format.FormatName == "case_story" {
			t.Error("case_story is not in the allowed list but survived the filter")
		}
	}
}

func TestFilterAllowedEmptyListMeansNoRestriction(t *testing.T) {
	usages := []models.FormatUsage{
		{Format: models.ContentFormat{FormatName: "qa", Weight: 1}},
	}
	if len(filterAllowed(usages, nil)) != 1 {
		t.Error("nil allowed list must mean no restriction (manual /produce keeps working)")
	}
}

// ถ้ากรองแล้วไม่เหลืออะไร ต้องไม่เงียบๆ คืนตัวแรกของคลังทั้งหมด
func TestFilterAllowedNoMatchReturnsEmpty(t *testing.T) {
	usages := []models.FormatUsage{
		{Format: models.ContentFormat{FormatName: "qa", Weight: 1}},
	}
	if len(filterAllowed(usages, []string{"news"})) != 0 {
		t.Error("no match must return empty so the caller can fail loudly")
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่าไม่ผ่าน**

```bash
go test ./internal/repository/ -run TestFilterAllowed 2>&1 | head -10
```

Expected: FAIL — `undefined: filterAllowed`

- [ ] **Step 3: เพิ่ม filterAllowed + PickNextIn**

ใน `internal/repository/formats.go` เพิ่มหลังฟังก์ชัน `PickNext`:

```go
// filterAllowed keeps only the usages whose format is in allowed. An empty or
// nil allowed list means "no restriction" — the manual /produce path and the
// unit tests rely on that. Pure, so the slot-locking rule is testable without a DB.
func filterAllowed(usages []models.FormatUsage, allowed []string) []models.FormatUsage {
	if len(allowed) == 0 {
		return usages
	}
	ok := make(map[string]bool, len(allowed))
	for _, a := range allowed {
		ok[a] = true
	}
	out := make([]models.FormatUsage, 0, len(usages))
	for _, u := range usages {
		if ok[u.Format.FormatName] {
			out = append(out, u)
		}
	}
	return out
}

// PickNextIn is PickNext restricted to one slot's formats. The slot decides the
// clip's visual mode (see clipMode), so letting an unlisted format through would
// render, say, an evening Q&A clip as a case file. Returns an error when nothing
// in allowed is enabled — producing the wrong mode is worse than producing nothing.
func (r *FormatsRepo) PickNextIn(ctx context.Context, allowed []string) (*models.ContentFormat, error) {
	usages, err := r.formatUsages(ctx)
	if err != nil {
		return nil, err
	}
	usages = filterAllowed(usages, allowed)
	if len(usages) == 0 {
		return nil, fmt.Errorf("no enabled content_format in %v", allowed)
	}
	picked := pickLeastUsed(usages)
	return &picked, nil
}
```

แล้วแยกคิวรีออกจาก `PickNext` เพื่อให้สองตัวใช้ร่วมกัน — แทนที่ body ของ `PickNext` เดิมด้วย:

```go
// PickNext returns the enabled format that has been used least (relative to its
// weight) in the last 7 days — guarantees every format gets airtime.
func (r *FormatsRepo) PickNext(ctx context.Context) (*models.ContentFormat, error) {
	usages, err := r.formatUsages(ctx)
	if err != nil {
		return nil, err
	}
	picked := pickLeastUsed(usages)
	return &picked, nil
}

// formatUsages loads every enabled format with its 7-day usage count.
func (r *FormatsRepo) formatUsages(ctx context.Context) ([]models.FormatUsage, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT cf.id, cf.format_name, cf.display_name, cf.question_instruction,
		       cf.script_instruction, cf.enabled, cf.weight,
		       COALESCE(u.cnt, 0) AS used_count
		FROM content_formats cf
		LEFT JOIN (
			SELECT content_format, COUNT(*) AS cnt
			FROM clips
			WHERE created_at > NOW() - INTERVAL '7 days'
			GROUP BY content_format
		) u ON u.content_format = cf.format_name
		WHERE cf.enabled = TRUE
		ORDER BY cf.format_name`)
	if err != nil {
		return nil, fmt.Errorf("query format usage: %w", err)
	}
	defer rows.Close()

	var usages []models.FormatUsage
	for rows.Next() {
		var u models.FormatUsage
		if err := rows.Scan(&u.Format.ID, &u.Format.FormatName, &u.Format.DisplayName,
			&u.Format.QuestionInstruction, &u.Format.ScriptInstruction,
			&u.Format.Enabled, &u.Format.Weight, &u.UsedCount); err != nil {
			return nil, fmt.Errorf("scan format usage: %w", err)
		}
		usages = append(usages, u)
	}
	return usages, nil
}
```

- [ ] **Step 4: รันเทสต์ให้ผ่าน**

```bash
go test ./internal/repository/ 2>&1 | grep -E '^(ok|FAIL|---)'
```

Expected: `ok`

- [ ] **Step 5: ให้ ProduceWeekly รับชุด format ของ slot**

ใน `internal/orchestrator/orchestrator.go` แก้ signature:

```go
func (o *Orchestrator) ProduceWeekly(ctx context.Context, count int, allowed []string) error {
```

ในตัว body หาบรรทัด `format, err := o.formatsRepo.PickNext(ctx)` (ประมาณบรรทัด 253) เปลี่ยนเป็น:

```go
	format, err := o.formatsRepo.PickNextIn(ctx, allowed)
```

แล้วแก้เส้นทาง fallback ตอนไม่มีข่าวสด (ประมาณบรรทัด 285-296) ให้ fallback อยู่ในชุดของ slot ทั้งก้อน:

```go
	if errors.Is(err, agent.ErrNoFreshNews) {
		// No reliable news found — never fabricate news; produce another format
		// from THIS SLOT's set. Falling back outside the set would hand the clip
		// to a different visual mode (see clipMode), which is worse than skipping.
		log.Println("No fresh news available, falling back within the slot's formats")
		remaining := make([]string, 0, len(allowed))
		for _, a := range allowed {
			if a != "news" {
				remaining = append(remaining, a)
			}
		}
		if len(remaining) == 0 {
			o.tracker.FailStep("question", err)
			return fmt.Errorf("no fresh news and no other format allowed for this slot: %w", err)
		}
		f, ferr := o.formatsRepo.PickNextIn(ctx, remaining)
		if ferr != nil {
			o.tracker.FailStep("question", ferr)
			return fmt.Errorf("news fallback: %w", ferr)
		}
		format = f
```

**สำคัญ:** เก็บโค้ดที่ตามหลังบล็อกนี้ (การเรียก `questionAgent.Generate` ซ้ำด้วย format ใหม่) ไว้เหมือนเดิม อ่านบริบทรอบๆ ก่อนแก้ เพราะโครงเดิมมีสาขา v2/legacy ที่ต้องยุบให้เหลือทางเดียว

- [ ] **Step 6: แก้ call site ทั้ง 3 จุด**

`internal/scheduler/scheduler.go` — แทนที่ `produceAndPublish` ด้วยสองตัวแยกตาม slot:

```go
// noonFormats / eveningFormats ล็อกชนิดเนื้อหาไว้กับ slot เพราะ content_format
// เป็นตัวกำหนดโหมดหน้าตาของคลิป (ดู clipMode) — ปล่อยให้สุ่มข้ามชุดเมื่อไร
// คลิปเย็นจะกลายเป็นแฟ้มคดีทันที
var (
	noonFormats    = []string{"case_story", "news"}
	eveningFormats = []string{"qa", "tips"}
)

func (s *Scheduler) produceNoon(ctx context.Context) error {
	return s.produceTick(ctx, "1 new case-file clip", func(c context.Context) error {
		return s.orchestrator.ProduceWeekly(c, 1, noonFormats)
	})
}

func (s *Scheduler) produceEvening(ctx context.Context) error {
	return s.produceTick(ctx, "1 new chat clip", func(c context.Context) error {
		return s.orchestrator.ProduceWeekly(c, 1, eveningFormats)
	})
}
```

ใน `handlerFor` แทน `case "produce_and_publish": return s.produceAndPublish` ด้วย:

```go
	case "produce_and_publish":
		return s.produceNoon
	case "produce_evening":
		return s.produceEvening
```

`internal/handler/orchestrator.go:53` (manual /produce — ไม่ล็อก slot):

```go
		if err := h.orch.ProduceWeekly(context.Background(), count, nil); err != nil {
```

`cmd/server/main.go:140`:

```go
		if err := orch.ProduceWeekly(ctx, *produceFlag, nil); err != nil {
```

- [ ] **Step 7: build + เทสต์ทั้ง repo**

```bash
go build ./... && go test ./internal/... 2>&1 | grep -E '^(FAIL|---)'
```

Expected: build ผ่าน ไม่มี FAIL

- [ ] **Step 8: commit**

```bash
git add internal/repository/formats.go internal/repository/formats_allowed_test.go \
  internal/orchestrator/orchestrator.go internal/scheduler/scheduler.go \
  internal/handler/orchestrator.go cmd/server/main.go
git commit -m "feat(slots): ล็อกชนิดเนื้อหาต่อช่วงเวลา — 12:00 เคส/ข่าว, 18:00 ถาม-ตอบ/ทิปส์"
```

---

### Task 6: ผูก 4 โหมดเข้ากับ content_format

**Files:**
- Modify: `internal/orchestrator/tutorial.go` (`clipMode`, `presetFor`)
- Modify: `internal/orchestrator/tutorial_test.go` (เทสต์ mapping ใหม่)
- Modify: `internal/orchestrator/orchestrator.go` (`resolveFormatInfo` รองรับ 2 โหมดใหม่)

**Interfaces:**
- Consumes: `producer.ModeChat`, `producer.ModeWarRoom`, `producer.ChatPreset`, `producer.WarRoomPreset` (Task 1-2)
- Produces: `clipMode(contentFormat) string` คืน 4 โหมด, `presetFor(contentFormat) producer.StylePreset` คืน 4 preset

- [ ] **Step 1: เขียนเทสต์ที่ยังไม่ผ่าน**

แทนที่ `TestClipModeFromContentFormat` ใน `internal/orchestrator/tutorial_test.go` ด้วย:

```go
func TestClipModeFromContentFormat(t *testing.T) {
	for _, tc := range []struct{ format, want string }{
		{"basic", producer.ModeTutorial},
		{"tutorial", producer.ModeWarRoom},
		{"qa", producer.ModeChat},
		{"tips", producer.ModeChat},
		{"case_story", producer.ModeCase},
		{"news", producer.ModeCase},
		{"ไม่รู้จัก", producer.ModeCase},
	} {
		if got := clipMode(tc.format); got != tc.want {
			t.Errorf("clipMode(%q) = %q, want %q", tc.format, got, tc.want)
		}
	}
}

// preset เป็นตัวตัดสินหน้าตาคลิปจริง ทุก content_format ต้องได้ preset ของโหมดตัวเอง
func TestPresetForCoversAllFourModes(t *testing.T) {
	for _, tc := range []struct{ format, wantKey string }{
		{"basic", "tutorial"},
		{"tutorial", "warroom"},
		{"qa", "chat"},
		{"tips", "chat"},
		{"case_story", "case-file"},
		{"news", "case-file"},
	} {
		if got := presetFor(tc.format); got.Key != tc.wantKey {
			t.Errorf("presetFor(%q) = %q, want %q", tc.format, got.Key, tc.wantKey)
		}
	}
}

// คลิปสอนทั้งสอง slot ต้องผ่านคลังหัวข้อเสมอ ไม่งั้นตะแกรง ui_vocab ไม่มีคำอนุญาตให้เทียบ
func TestNeedsCatalogFeatureCoversBothTeachingSlots(t *testing.T) {
	for _, format := range []string{tutorialFormatName, basicFormatName} {
		if !needsCatalogFeature(format) {
			t.Errorf("needsCatalogFeature(%q) = false, want true", format)
		}
	}
	for _, format := range []string{"qa", "tips", "news", "case_story"} {
		if needsCatalogFeature(format) {
			t.Errorf("needsCatalogFeature(%q) = true, want false", format)
		}
	}
}
```

**ลบ** `TestPresetForCoversBasicAndTutorial` เดิมทิ้ง (ถูกแทนด้วย `TestPresetForCoversAllFourModes`) และลบ `TestNeedsCatalogFeatureCoversBasic` เดิม (ถูกแทนด้วยตัวใหม่ข้างบน)

- [ ] **Step 2: รันเทสต์ให้เห็นว่าไม่ผ่าน**

```bash
go test ./internal/orchestrator/ -run 'TestClipMode|TestPresetFor' 2>&1 | head -12
```

Expected: FAIL — `clipMode("tutorial")` คืน `tutorial` ไม่ใช่ `warroom`

- [ ] **Step 3: เขียน mapping ใหม่**

ใน `internal/orchestrator/tutorial.go` แทนที่ `clipMode` ด้วย:

```go
// clipMode derives the RENDER mode from the clip's persisted content_format.
//
// ชื่อ content_format กับชื่อโหมดไม่ตรงกันโดยตั้งใจ: "tutorial" คือคลิปสอนขั้นสูง
// (21:00) ที่ตอนนี้แต่งหน้าเป็นห้องควบคุม ส่วน "basic" คือคลิปสอนมือใหม่ (15:00)
// ที่ใช้หน้าตาคู่มือ — content_format ตอบว่า "ใครเขียนบท" ส่วนโหมดตอบว่า
// "หน้าตาเป็นแบบไหน" สองคำถามนี้แยกกันมาตั้งแต่ต้น (ดู agentModeFor)
func clipMode(contentFormat string) string {
	switch contentFormat {
	case basicFormatName:
		return producer.ModeTutorial
	case tutorialFormatName:
		return producer.ModeWarRoom
	case "qa", "tips":
		return producer.ModeChat
	default:
		return producer.ModeCase
	}
}
```

แทนที่ `presetFor` ใน `internal/orchestrator/orchestrator.go` ด้วย:

```go
// presetFor returns the preset a clip is pinned to. Every clip is pinned: its
// content_format decides which of the four formats it belongs to.
//
// preset เป็นตัวตัดสินหน้าตาคลิปจริง (resolveFormatInfo แปลง preset เป็น
// FormatInfo.Mode ที่ไปเป็น data-format ของ template และนโยบายภาพ) การถามผ่าน
// clipMode ที่เดียวจึงกันสอง slot สอนแยกหน้าตากันโดยไม่มีใครรู้ตัว
func presetFor(contentFormat string) producer.StylePreset {
	switch clipMode(contentFormat) {
	case producer.ModeTutorial:
		return producer.TutorialPreset
	case producer.ModeWarRoom:
		return producer.WarRoomPreset
	case producer.ModeChat:
		return producer.ChatPreset
	default:
		return producer.CaseFilePreset
	}
}
```

- [ ] **Step 4: ให้ resolveFormatInfo รู้จักโหมดใหม่**

ใน `internal/orchestrator/orchestrator.go` แก้หัวของ `resolveFormatInfo` (ส่วนที่เช็ค TutorialPreset) ให้เป็น:

```go
	switch preset.Key {
	case producer.TutorialPreset.Key:
		return producer.FormatInfo{Mode: producer.ModeTutorial}
	case producer.ChatPreset.Key:
		return producer.FormatInfo{Mode: producer.ModeChat}
	case producer.WarRoomPreset.Key:
		return producer.FormatInfo{Mode: producer.ModeWarRoom}
	}
	if preset.Key != producer.CaseFilePreset.Key {
		return producer.FormatInfo{}
	}
```

(ส่วนที่เหลือของฟังก์ชัน — การจองเลขคดี — คงเดิมทั้งหมด)

- [ ] **Step 5: รันเทสต์ให้ผ่าน**

```bash
go test ./internal/orchestrator/ -v -run 'TestClipMode|TestPresetFor|TestNeedsCatalog' 2>&1 | grep -E '^(--- (PASS|FAIL)|ok|FAIL)'
```

Expected: PASS ทั้งหมด

- [ ] **Step 6: เทสต์ทั้ง repo + build**

```bash
go build ./... && go test ./internal/... 2>&1 | grep -E '^(FAIL|---)'
```

Expected: ไม่มีผลลัพธ์

- [ ] **Step 7: commit**

```bash
git add internal/orchestrator/tutorial.go internal/orchestrator/orchestrator.go \
  internal/orchestrator/tutorial_test.go
git commit -m "feat(modes): ผูก 4 โหมดเข้ากับ content_format — qa/tips=แชท, tutorial=ห้องควบคุม"
```

---

### Task 7: agent rows ของสองโหมดใหม่ (migration)

**Files:**
- Create: `migrations/073_four_mode_slots.sql`
- Create: `internal/scheduler/evening_action_test.go`

**Interfaces:**
- Consumes: `modeAgentConfig(ctx, name, mode)` ที่มองหาแถวชื่อ `<name>_<mode>` (มีอยู่แล้ว)
- Produces: แถว `script_chat`, `scene_chat`, `critic_chat`, `script_warroom`, `scene_warroom`, `critic_warroom` และ schedule row ของ 18:00

- [ ] **Step 1: เขียนเทสต์ที่ยังไม่ผ่าน**

สร้าง `internal/scheduler/evening_action_test.go`:

```go
package scheduler

import "testing"

// schedule row ของ 18:00 ชี้ไป action ใหม่ ถ้า handlerFor ไม่รู้จัก แถวนั้นจะเงียบ
// ไม่ทำอะไรเลย และไม่มีคลิปเย็นทั้งวันโดยไม่มี error ให้เห็น
func TestEveningActionHasHandler(t *testing.T) {
	s := &Scheduler{}
	if s.handlerFor("produce_evening") == nil {
		t.Error(`handlerFor("produce_evening") = nil — the 18:00 schedule row would silently do nothing`)
	}
	if s.handlerFor("produce_and_publish") == nil {
		t.Error(`handlerFor("produce_and_publish") = nil — the 12:00 row would break`)
	}
}
```

- [ ] **Step 2: รันเทสต์**

```bash
go test ./internal/scheduler/ -run TestEveningAction 2>&1 | head -10
```

Expected: PASS (Task 6 เพิ่ม case ไปแล้ว) — ถ้า FAIL ให้กลับไปเติม `case "produce_evening"` ใน `handlerFor`

- [ ] **Step 3: เขียน migration**

สร้าง `migrations/073_four_mode_slots.sql`:

```sql
-- 073: 4 ช่วงเวลา 4 โหมด (spec 2026-07-28)
-- agent 3 แถวต่อโหมดใหม่ + ย้าย schedule 18:00 ไป action ของตัวเอง
-- idempotent: รันซ้ำได้ ไม่ทับค่าที่คนแก้ไว้ใน UI

BEGIN;

-- ── โหมดแชท (18:00) ──
INSERT INTO agent_configs (agent_name, system_prompt, model, temperature, enabled, prompt_template)
SELECT 'script_chat',
       system_prompt || E'\n\nโหมดแชท: เขียนบทเป็นบทสนทนาจริงระหว่างลูกค้ากับเรา ' ||
       'ลูกค้าเล่าปัญหาด้วยภาษาพูดสั้นๆ ทีละข้อความ (ไม่ใช่ย่อหน้ายาว) เราตอบทีละข้อความเช่นกัน ' ||
       'ห้ามบรรยายเหตุการณ์จากมุมผู้เล่าเรื่อง ให้เล่าผ่านสิ่งที่ทั้งสองฝ่ายพิมพ์คุยกัน ' ||
       'ปิดท้ายด้วยข้อสรุปที่ลูกค้าเอาไปทำต่อได้ทันที',
       model, temperature, TRUE, prompt_template
FROM agent_configs WHERE agent_name = 'script_case'
ON CONFLICT (agent_name) DO NOTHING;

INSERT INTO agent_configs (agent_name, system_prompt, model, temperature, enabled, prompt_template)
SELECT 'scene_chat',
       system_prompt || E'\n\nโหมดแชท: ใช้ layout ได้เฉพาะ hook, chat_in, chat_out, recap, cta ' ||
       'ห้ามใช้ layout ของโหมดอื่น (casefile, evidence, comic, board, verdict, uistep, dashboard, alarm)',
       model, temperature, TRUE,
       prompt_template || E'\n\nรูปแบบซีนของโหมดแชท:\n' ||
       '- chat_in = ลูกค้าถาม: {"type":"chat_in","asker":"ชื่อลูกค้า","stamp":"21:47 น.",' ||
       '"msgs":[{"from":"them","t":"ข้อความ"},{"from":"them","t":"ข้อความเสี่ยง","alert":true}]}\n' ||
       '- chat_out = เราตอบ: {"type":"chat_out","msgs":[{"from":"me","t":"คำตอบ"},' ||
       '{"from":"them","t":"ถามต่อ"}],"verdict":"ข้อสรุปสั้นๆ"}\n' ||
       '- recap = สรุปท้าย: {"type":"recap","title":"หัวข้อ","chips":[{"n":"72","t":"ชั่วโมง"}]}\n' ||
       'กติกา: 1 ฟองข้อความ = 1 ประโยคสั้น ห้ามเกิน 90 ตัวอักษร · alert ใช้ได้ไม่เกิน 1 ฟองต่อซีน · ' ||
       'ซีนแรกเป็น hook เท่านั้น (ซีนเดียวที่มีภาพ AI)',
       prompt_template
FROM agent_configs WHERE agent_name = 'scene_case'
ON CONFLICT (agent_name) DO NOTHING;

INSERT INTO agent_configs (agent_name, system_prompt, model, temperature, enabled, prompt_template)
SELECT 'critic_chat',
       system_prompt || E'\n\nโหมดแชท: ตรวจเพิ่มว่าบทเป็นบทสนทนาจริง ไม่ใช่บรรยาย ' ||
       'ทุกฟองข้อความต้องสั้นแบบที่คนพิมพ์จริง และคำตอบของเราต้องมีสิ่งที่ทำต่อได้ ไม่ใช่แค่ปลอบใจ',
       model, temperature, TRUE, prompt_template
FROM agent_configs WHERE agent_name = 'critic'
ON CONFLICT (agent_name) DO NOTHING;

-- ── โหมดห้องควบคุม (21:00) ──
-- คัดลอกจากคู่สอนเดิมทั้งหมด เพราะกติกาการสอน (ห้ามแต่งชื่อเมนู ขั้นตอนต้องครบ)
-- ต้องเหมือนเดิมเป๊ะ เปลี่ยนแค่ layout ที่ห่อรอบ uistep
INSERT INTO agent_configs (agent_name, system_prompt, model, temperature, enabled, prompt_template)
SELECT 'script_warroom',
       system_prompt || E'\n\nโหมดห้องควบคุม: เปิดคลิปด้วยตัวเลขที่ผิดปกติจริงจากหน้าจอ ' ||
       '(เช่น CPM พุ่ง ROAS ตก) แล้วค่อยพาไปที่วิธีแก้ ผู้ชมคือคนยิงแอดอยู่แล้ว ' ||
       'ใช้ศัพท์เทคนิคได้ตรงๆ ไม่ต้องอธิบายพื้นฐาน',
       model, temperature, TRUE, prompt_template
FROM agent_configs WHERE agent_name = 'script_tutorial'
ON CONFLICT (agent_name) DO NOTHING;

INSERT INTO agent_configs (agent_name, system_prompt, model, temperature, enabled, prompt_template)
SELECT 'scene_warroom',
       system_prompt || E'\n\nโหมดห้องควบคุม: ใช้ layout ได้เฉพาะ dashboard, uistep, alarm, cta',
       model, temperature, TRUE,
       prompt_template || E'\n\nรูปแบบซีนของโหมดห้องควบคุม:\n' ||
       '- ซีนแรกเป็น dashboard เสมอ (ซีนเดียวที่มีภาพ AI): {"type":"dashboard",' ||
       '"statLabel":"CPM 7 วันล่าสุด","chips":[{"n":"+38%","t":"CPM","bad":true},{"n":"1.4x","t":"ROAS"}],' ||
       '"callout":"บรรทัดเตือนสั้นๆ"}\n' ||
       '- ซีนขั้นตอนใช้ uistep เหมือนเดิมทุกฟิลด์ ห้ามเปลี่ยนโครงสร้าง\n' ||
       '- alarm = สรุปสิ่งที่ต้องทำ: {"type":"alarm","title":"หัวข้อ",' ||
       '"rows":[{"t":"ทำอะไร"},{"t":"ถ้าไม่ทำจะเจออะไร","bad":true}]}\n' ||
       'ข้อบังคับ: จำนวนซีน uistep ต้องเท่ากับจำนวนขั้นในคลังเป๊ะ ห้ามขาดห้ามเกิน ' ||
       'และชื่อเมนูทุกคำต้องมาจาก ui_vocab ที่ให้มาเท่านั้น',
       prompt_template
FROM agent_configs WHERE agent_name = 'scene_tutorial'
ON CONFLICT (agent_name) DO NOTHING;

INSERT INTO agent_configs (agent_name, system_prompt, model, temperature, enabled, prompt_template)
SELECT 'critic_warroom', system_prompt, model, temperature, TRUE, prompt_template
FROM agent_configs WHERE agent_name = 'critic_tutorial'
ON CONFLICT (agent_name) DO NOTHING;

-- ── schedule 18:00 ไป action ของตัวเอง ──
UPDATE schedules SET action = 'produce_evening'
WHERE cron_expression = '0 18 * * *' AND action = 'produce_and_publish';

COMMIT;
```

- [ ] **Step 4: ตรวจว่า migration idempotent**

```bash
grep -c "ON CONFLICT (agent_name) DO NOTHING" migrations/073_four_mode_slots.sql
```

Expected: `6` — ทุก INSERT ต้องมี ถ้าน้อยกว่านี้ให้เติมให้ครบ ไม่งั้นรันซ้ำจะพัง

- [ ] **Step 5: ตรวจว่ามี unique constraint บน agent_name จริง**

```bash
grep -rn "agent_name" migrations/*.sql | grep -i "unique\|primary key" | head -3
```

ถ้าไม่พบ ให้แก้ทุก INSERT เป็นรูปแบบนี้แทน (กันแถวซ้ำโดยไม่พึ่ง constraint):

```sql
INSERT INTO agent_configs (...)
SELECT ... FROM agent_configs WHERE agent_name = 'script_case'
  AND NOT EXISTS (SELECT 1 FROM agent_configs WHERE agent_name = 'script_chat');
```

- [ ] **Step 6: build + เทสต์**

```bash
go build ./... && go test ./internal/... 2>&1 | grep -E '^(FAIL|---)'
```

Expected: ไม่มีผลลัพธ์

- [ ] **Step 7: commit**

```bash
git add migrations/073_four_mode_slots.sql internal/scheduler/evening_action_test.go
git commit -m "feat(agents): prompt ของโหมดแชทกับห้องควบคุม + ย้าย schedule 18:00"
```

---

### Task 8: เรนเดอร์จริงดูด้วยตา ก่อนขึ้น prod

**Files:**
- ไม่แก้ไฟล์ใด — เป็นด่านตรวจด้วยตา

**Interfaces:**
- Consumes: ทุกอย่างจาก Task 1-7

- [ ] **Step 1: เพิ่มตัวทิ้งไฟล์ HTML ไว้เปิดดูในเบราว์เซอร์**

โปรเจกต์นี้ยังไม่มีตัวเรนเดอร์ตัวอย่างของโหมดใหม่ (`render_sample_test.go` รับแค่ `HF_SAMPLE=1` ของคลิปตัวอย่างเดิม) เพิ่มตัวทิ้งไฟล์เข้าไปในเทสต์เรนเดอร์ที่เขียนไว้แล้ว

เพิ่มท้าย `internal/producer/composition_chat_render_test.go`:

```go
// TestDumpChatHTML เขียนผลเรนเดอร์ลงไฟล์ให้เปิดดูด้วยตา ปกติข้าม — ตั้ง
// HF_KEEP_DIR=<dir> เพื่อสั่งให้ทิ้งไฟล์ (ตั้งใจไม่ assert อะไร มันคือด่านสายตา)
func TestDumpChatHTML(t *testing.T) {
	dir := os.Getenv("HF_KEEP_DIR")
	if dir == "" {
		t.Skip("set HF_KEEP_DIR=<dir> to dump the chat HTML for eyeballing")
	}
	out, err := RenderCompositionScenes(chatParams())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "chat.html"), out, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Logf("wrote %s/chat.html", dir)
}
```

เพิ่ม import `"os"` และ `"path/filepath"` ที่หัวไฟล์

ทำแบบเดียวกันใน `internal/producer/composition_warroom_render_test.go` โดยเปลี่ยนชื่อฟังก์ชันเป็น `TestDumpWarRoomHTML`, ใช้ `warroomParams()` และเขียนเป็น `warroom.html`

แล้วรัน:

```bash
mkdir -p /tmp/claude/modes
HF_KEEP_DIR=/tmp/claude/modes go test ./internal/producer/ -run 'TestDump(Chat|WarRoom)HTML' -v 2>&1 | grep -E 'wrote|PASS|FAIL'
open /tmp/claude/modes/chat.html /tmp/claude/modes/warroom.html
```

Expected: ได้ 2 ไฟล์ เปิดในเบราว์เซอร์เห็นซีนทับกัน (ปกติ — GSAP ยังไม่รัน timeline ตอนเปิดนิ่งๆ) ให้ดูทีละซีนด้วย DevTools โดยตั้ง `opacity:1` ที่ `.scene` ที่สนใจ

- [ ] **Step 2: ตรวจ 5 ข้อนี้ด้วยตาในเฟรมที่ได้**

| ตรวจ | ผ่านเมื่อ |
|---|---|
| ฟองข้อความ | ของลูกค้าอยู่ซ้าย ของเราอยู่ขวาสีส้ม แยกออกชัดเจน |
| ข้อความไทย | ไม่มีสระ/วรรณยุกต์ลอย ไม่มีคำถูกตัดกลางคำ |
| ล้นเฟรม | ไม่มีข้อความล้นขอบล่างหรือถูกครอบตัด |
| ห้องควบคุม uistep | แถวเมนูเป้าหมายยังเห็นชัด ป้าย "กดตรงนี้" กับวงแตะไม่ทับตัวหนังสือ |
| KPI สีแดง | ตัวเลขที่ `bad:true` เป็นสีแดงจริง |

- [ ] **Step 3: ตรวจว่าตะแกรงยังทำงาน (ด่านที่ห้ามพลาด)**

```bash
go test ./internal/orchestrator/ -run 'TestTutorialGate|TestUIVocab' -v 2>&1 | grep -E '^(--- (PASS|FAIL))'
```

Expected: PASS ทุกตัว — ถ้ามีตัวไหน FAIL แปลว่าการห่อ `uistep` ไปแตะตะแกรงเข้า **ห้ามขึ้น prod**

- [ ] **Step 4: ตรวจครบทั้งระบบก่อนส่งมอบ**

```bash
go build ./... && go vet ./... && go test ./... 2>&1 | grep -E '^(FAIL|---)'
echo "package ที่ผ่าน: $(go test ./... 2>&1 | grep -c '^ok')"
cd frontend && npm run build 2>&1 | tail -3
```

Expected: build/vet ผ่าน, ไม่มี FAIL, frontend build ผ่าน

- [ ] **Step 5: commit ผลการตรวจ (ถ้ามีการแก้ระหว่างทาง)**

```bash
git add -A internal/ && git commit -m "fix: ปรับหน้าตาโหมดใหม่หลังดูเฟรมจริง" || echo "ไม่มีอะไรต้องแก้"
```

---

## หลังทำครบ

**อย่าเพิ่ง deploy ทันที** — สองเรื่องที่ต้องทำตามลำดับ:

1. **deploy โค้ดก่อน** แล้วดูว่า migration 073 รันสำเร็จ (`railway logs` หา `073`)
2. รอคลิปแรกของแต่ละโหมดใหม่ (18:00 กับ 21:00) แล้ว **eyeball ก่อนปล่อยยาว** — ทั้งสองโหมดยังไม่เคยผลิตคลิปจริงเลย

**ถ้าคลิปโหมดใหม่ออกมาพัง:** `git revert` ของ PR นี้แล้ว deploy — แถว agent ที่ migration เพิ่มไว้จะค้างใน DB แต่ไม่มีใครเรียก ไม่ต้อง down-migration

## สิ่งที่แผนนี้ไม่ทำ

- ไม่แตะโหมดแฟ้มคดีกับคู่มือที่รันอยู่บน prod
- ไม่เพิ่ม feature flag (rollback = revert)
- ไม่เพิ่มงบภาพ AI — โหมดใหม่ใช้ 1 ภาพเท่าโหมดคู่มือ
- ไม่แตะโค้ดตะแกรง `ui_vocab` / `CountUIStepScenes`
