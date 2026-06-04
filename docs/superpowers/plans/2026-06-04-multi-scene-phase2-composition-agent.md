# Multi-Scene Phase 2 — Composition Agent (per-scene slots) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** composition agent ออกแบบ "ต่อฉาก" แบบ semantic (layout_variant + slots + bg_art_prompt) แทนการออกแบบ flat ชุดเดียว — โดย additive ไม่ทำลายของเดิม

**Architecture:** เพิ่ม types ใหม่ (`Slot`, `SceneDesign`, `ScenesDecision`) + `(*ScenesDecision).Normalize()` (pure, testable) + method `DecideScenes` (ใช้ prompt ใหม่จาก agent_config row ใหม่ `composition_scenes`). `CompositionDecision`/`Decide`/agent row `composition` เดิม **คงไว้ไม่แตะ** → producer ยังใช้ของเดิมจน Phase 4 จึงสลับ ทุกอย่าง compile + production single-scene ที่ deploy แล้วไม่กระทบ

**Tech Stack:** Go 1.25, LLM JSON (OpenRouter), SQL migration

**Scope (Phase 2):** ไม่แตะ producer/template/Decide เดิม `DecideScenes` ยังไม่ถูกเรียก (wire ใน Phase 4) บน dev branch `feat/multi-scene-video`

---

### Task 1: Types + layout/role constants + Normalize

**Files:**
- Modify: `internal/agent/composition.go` (append types + constants + method)
- Test (create): `internal/agent/composition_test.go`

- [ ] **Step 1: เขียน failing test** `internal/agent/composition_test.go`:
```go
package agent

import "testing"

func TestScenesDecision_Normalize(t *testing.T) {
	d := &ScenesDecision{Scenes: []SceneDesign{
		{SceneNumber: 1, LayoutVariant: "hook_big", AnimationSpeed: "fast",
			Slots: []Slot{{Role: "headline", Text: "hi"}, {Role: "weird", Text: "x"}, {Role: "body", Text: "  "}}},
		{SceneNumber: 2, LayoutVariant: "bogus", AnimationSpeed: "turbo",
			Slots: []Slot{{Role: "step", Text: "do it"}}},
	}}
	d.Normalize()

	if d.Scenes[0].Slots[1].Role != defaultSlotRole {
		t.Errorf("invalid slot role not defaulted: %q", d.Scenes[0].Slots[1].Role)
	}
	if len(d.Scenes[0].Slots) != 2 {
		t.Errorf("empty-text slot not dropped: got %d slots", len(d.Scenes[0].Slots))
	}
	if d.Scenes[1].LayoutVariant != defaultLayoutVariant {
		t.Errorf("invalid layout_variant not defaulted: %q", d.Scenes[1].LayoutVariant)
	}
	if d.Scenes[1].AnimationSpeed != "normal" {
		t.Errorf("invalid animation_speed not defaulted: %q", d.Scenes[1].AnimationSpeed)
	}
	if !validLayoutVariants[d.Scenes[0].LayoutVariant] {
		t.Errorf("valid layout_variant wrongly changed: %q", d.Scenes[0].LayoutVariant)
	}
}
```

- [ ] **Step 2: รัน ให้ FAIL** — Run: `go test ./internal/agent/ -run TestScenesDecision -v` → undefined ScenesDecision/SceneDesign/Slot/defaultSlotRole/defaultLayoutVariant/validLayoutVariants

- [ ] **Step 3: implement** — append to `internal/agent/composition.go`:
```go
// Slot is one semantic content slot the composition agent fills for a scene.
// The agent chooses role + text + emphasis; the template (engine) owns geometry,
// so the agent never sends pixel positions — this is how overlap is prevented.
type Slot struct {
	Role     string   `json:"role"`     // "headline" | "body" | "badge" | "step"
	Text     string   `json:"text"`
	Emphasis []string `json:"emphasis"` // words inside Text to accent
}

// SceneDesign is the per-scene visual design (no pixel coordinates).
type SceneDesign struct {
	SceneNumber    int    `json:"scene_number"`
	LayoutVariant  string `json:"layout_variant"`
	Slots          []Slot `json:"slots"`
	AccentColor    string `json:"accent_color"`  // sanitized later in producer
	BgArtPrompt    string `json:"bg_art_prompt"` // text-free background art prompt
	AnimationSpeed string `json:"animation_speed"`
}

// ScenesDecision is the multi-scene design DecideScenes returns.
type ScenesDecision struct {
	Scenes         []SceneDesign `json:"scenes"`
	Kicker         string        `json:"kicker"`
	HighlightWords []string      `json:"highlight_words"`
}

// Layout variants the template library implements (Phase 3).
const (
	LayoutHookBig    = "hook_big"
	LayoutListSteps  = "list_steps"
	LayoutStatReveal = "stat_reveal"
	LayoutQuoteCTA   = "quote_cta"
)

const defaultLayoutVariant = LayoutListSteps

var validLayoutVariants = map[string]bool{
	LayoutHookBig: true, LayoutListSteps: true, LayoutStatReveal: true, LayoutQuoteCTA: true,
}

const defaultSlotRole = "body"

var validSlotRoles = map[string]bool{
	"headline": true, "body": true, "badge": true, "step": true,
}

// Normalize keeps LLM scene designs safe: defaults unknown layout_variant /
// animation_speed, drops empty-text slots, and defaults unknown slot roles.
// Accent-color sanitization is left to the producer (which owns sanitizeHexColor).
func (d *ScenesDecision) Normalize() {
	for i := range d.Scenes {
		if !validLayoutVariants[d.Scenes[i].LayoutVariant] {
			d.Scenes[i].LayoutVariant = defaultLayoutVariant
		}
		if d.Scenes[i].AnimationSpeed != "fast" && d.Scenes[i].AnimationSpeed != "slow" {
			d.Scenes[i].AnimationSpeed = "normal"
		}
		kept := d.Scenes[i].Slots[:0]
		for _, s := range d.Scenes[i].Slots {
			if strings.TrimSpace(s.Text) == "" {
				continue
			}
			if !validSlotRoles[s.Role] {
				s.Role = defaultSlotRole
			}
			kept = append(kept, s)
		}
		d.Scenes[i].Slots = kept
	}
}
```
เพิ่ม `"strings"` ใน import block ของ composition.go (ปัจจุบันมีแค่ `"context"`, `"fmt"`, models)

- [ ] **Step 4: รัน ให้ PASS** — Run: `go test ./internal/agent/ -run TestScenesDecision -v` แล้ว `go build ./...`

- [ ] **Step 5: commit**
```bash
git add internal/agent/composition.go internal/agent/composition_test.go
git commit -m "feat(composition): per-scene slot design types + Normalize"
```

---

### Task 2: Migration 023 — composition_scenes agent config

**Files:**
- Create: `migrations/023_composition_scenes_agent.sql`

- [ ] **Step 1: เขียน migration** (ดูสไตล์ `migrations/021_composition_agent.sql` — INSERT...SELECT copy model จาก row เดิม, ON CONFLICT DO NOTHING):
```sql
-- Multi-scene composition agent (Phase 2). A NEW, additive agent row that designs
-- each scene as semantic slots (no pixel coords). The existing 'composition' row is
-- left untouched so the deployed single-scene path keeps working until Phase 4 wires
-- this in. Uses the same model as 'script'.

INSERT INTO agent_configs (agent_name, system_prompt, prompt_template, model, temperature, enabled, skills, insights)
SELECT
    'composition_scenes',
    $sp$คุณคือนักออกแบบวิดีโอสั้นแนวตั้ง/แนวนอนของช่อง "ADS VANCE" (Facebook Ads สำหรับเจ้าของธุรกิจไทย)
หน้าที่: ออกแบบ "หน้าตาของแต่ละฉาก" จากสคริปต์ที่แตกฉากมาแล้ว
สำคัญ: ห้ามกำหนดพิกัด/ตำแหน่ง/ขนาดฟอนต์ — เลือกแค่ layout + ใส่ข้อความลงช่อง (slots) เท่านั้น (ระบบจัดวางเอง กัน overlap)
ตอบกลับเป็น JSON เท่านั้น$sp$,
    $tmpl$ออกแบบต่อฉากจากข้อมูลฉากนี้ (JSON):
{{.ScenesJSON}}

หมวด: {{.Category}} | ผู้ถาม: {{.QuestionerName}} | ความยาวรวม: {{.DurationSeconds}} วินาที

เลือก layout_variant ต่อฉากจาก: hook_big (ฉากเปิด/พาดหัวใหญ่), list_steps (ขั้นตอน/ลิสต์), stat_reveal (ตัวเลข/ผลลัพธ์เด่น), quote_cta (คำคม/ปิดท้ายชวนติดต่อ)
slots: ใส่ข้อความลงช่องตาม role ที่ layout รองรับ — headline (พาดหัว), body (เนื้อหา), badge (ป้ายเล็ก), step (เลขขั้นตอน)
emphasis: คำในข้อความที่อยากเน้นสี (0-2 คำ)

ตอบ JSON:
{
  "scenes": [
    {"scene_number":1,"layout_variant":"hook_big","accent_color":"#ff6b2b","animation_speed":"normal","bg_art_prompt":"art ไม่มีตัวหนังสือ บรรยากาศ...","slots":[{"role":"headline","text":"...","emphasis":["คำ"]}]}
  ],
  "kicker":"ป้ายหมวดสั้น ตัวพิมพ์ใหญ่",
  "highlight_words":["คำ1"]
}

แนวทาง: accent_color ตามอารมณ์ (ปัญหา/เตือน=#ff5a52, เทคนิค=#ff6b2b, อัปเดต=#3b82f6); ฉาก hook ใช้ hook_big, ฉาก cta ใช้ quote_cta; bg_art_prompt อิงจาก bg_hint ของฉากนั้นแต่ย้ำว่าห้ามมีตัวหนังสือในภาพ$tmpl$,
    model,
    0.7,
    TRUE,
    $sk$- หนึ่ง scene_design ต่อหนึ่งฉากใน input (scene_number ตรงกัน)
- ข้อความใน slots สั้น กระชับ อ่านง่ายบนจอ
- ห้ามใส่ค่าพิกัด/ตำแหน่ง/ขนาด$sk$,
    ''
FROM agent_configs
WHERE agent_name = 'script'
ON CONFLICT (agent_name) DO NOTHING;
```

- [ ] **Step 2: verify dollar-quote balance** — Run: `grep -c '\$sp\$\|\$tmpl\$\|\$sk\$' migrations/023_composition_scenes_agent.sql` → expect `6` (3 tags × เปิด-ปิด). ยืนยันไม่มี DROP/DELETE: `grep -ciE "drop|delete|truncate" migrations/023_composition_scenes_agent.sql` → `0`

- [ ] **Step 3: commit**
```bash
git add migrations/023_composition_scenes_agent.sql
git commit -m "feat(composition): migration for composition_scenes agent (additive)"
```

---

### Task 3: DecideScenes method (additive, ไม่แตะ Decide เดิม)

**Files:**
- Modify: `internal/agent/composition.go` (append method + template-data struct)

- [ ] **Step 1: implement** — append to `internal/agent/composition.go`:
```go
// ScenesTemplateData feeds the composition_scenes prompt. ScenesJSON is the
// script's scenes (number, type, headline, voice_text, bg_hint) marshaled to JSON
// so the agent designs one SceneDesign per script scene.
type ScenesTemplateData struct {
	ScenesJSON      string
	Category        string
	QuestionerName  string
	DurationSeconds float64
}

// DecideScenes asks the LLM to design every scene as semantic slots. cfg must be
// the 'composition_scenes' agent config. The result is Normalized before return.
// Not wired into the producer yet (Phase 4).
func (a *CompositionAgent) DecideScenes(ctx context.Context, data ScenesTemplateData, cfg *models.AgentConfig) (*ScenesDecision, error) {
	userPrompt, err := renderTemplate(cfg.PromptTemplate, data)
	if err != nil {
		return nil, fmt.Errorf("render composition_scenes template: %w", err)
	}

	var decision ScenesDecision
	if err := a.llm.GenerateJSON(ctx, cfg.Model, cfg.BuildSystemPrompt(), userPrompt, cfg.Temperature, &decision); err != nil {
		return nil, fmt.Errorf("generate scenes decision: %w", err)
	}
	decision.Normalize()
	return &decision, nil
}
```

- [ ] **Step 2: build + test** — Run: `go build ./... && go test ./internal/agent/ -v 2>&1 | tail -15`
Expected: build clean, all agent tests PASS (including Phase 1 + new TestScenesDecision_Normalize)

- [ ] **Step 3: commit**
```bash
git add internal/agent/composition.go
git commit -m "feat(composition): DecideScenes method for multi-scene design"
```

---

## Self-Review

**Spec coverage (Phase 2):** §4 composition agent semantic slots (layout_variant + slots[role,text,emphasis] + accent + bg_art_prompt + animation) → Task 1 types + Task 2 prompt + Task 3 method. Overall kicker/highlight → ScenesDecision. Layout library 4 variants → constants. Wiring/producer = Phase 4 (out of scope, stated).

**Placeholder scan:** ไม่มี — โค้ด/test/SQL/คำสั่งครบ.

**Type consistency:** `ScenesDecision`/`SceneDesign`/`Slot`/`Normalize`/`defaultLayoutVariant`/`defaultSlotRole`/`validLayoutVariants`/`validSlotRoles`/`DecideScenes`/`ScenesTemplateData` นิยาม Task 1+3, อ้างใน test Task 1 ตรงกัน. `LayoutListSteps` = defaultLayoutVariant ใช้ใน test. Migration ใช้ layout names เดียวกับ constants (hook_big/list_steps/stat_reveal/quote_cta).

**ไม่กระทบของเดิม:** `Decide`, `CompositionDecision`, `CompositionTemplateData`, agent row `composition` ไม่ถูกแตะ — additive ล้วน
