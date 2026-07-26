# Visual QA — เกณฑ์ "คำไทยถูกตัดกลางคำ" Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** ให้ visual_qa จับตำหนิ "คำไทยถูกผ่ากลางคำคนละบรรทัด" ได้จริง และตำหนินั้นต้องรอดรอบยืนยัน (two-strike) ไปบล็อกการเผยแพร่จริง

**Architecture:** ตำหนิคลาสนี้เป็นข้อความบนจอล้วน — `hyperframes inspect` มองไม่เห็นเพราะกล่องไม่ล้น และ two-strike ปัจจุบันจะ**เคลียร์มันทิ้ง**เพราะเฟรมยืนยันที่ 85% ของซีนเป็นคนละวลีคาราโอเกะ (คอมเมนต์ `orchestrator.go:1064-1067` เขียนไว้ตรงๆ) แผนนี้จึงทำ 3 อย่าง: (1) ให้ verdict ของ visual_qa พก **รหัสประเภทตำหนิ** (`codes`) แล้วให้รหัส `wordbreak` เป็น "sticky" — รอบยืนยันเคลียร์ไม่ได้ และไม่ต้องเสีย credit ยิงซ้ำ, (2) migration 066 เติมเกณฑ์ + ข้อยกเว้นของกฎคาราโอเกะเข้า prompt ของ `visual_qa` และเข้า `auto_review` (ด่านสุดท้ายก่อน publish — ถ้ามันไม่รู้จักตำหนิคลาสนี้ มันจะ approve ทับ), (3) วัดผลจริงด้วยชุดเฟรมทองคำที่เรนเดอร์จาก CSS แคปชั่นตัวจริง

**Tech Stack:** Go 1.x (`github.com/jaochai/video-fb`), Postgres (Neon, project `snowy-grass-75448787`), kie.ai Claude vision (`claude-sonnet-5`), Chromium/Playwright MCP สำหรับสร้างเฟรมทดสอบ, Python 3 stdlib สำหรับ eval script

## Global Constraints

- **ภาษาไทยทั้งหมด** ในทุก prompt, ทุก `issues` string, ทุกคอมเมนต์โค้ดใหม่ — ตามสไตล์ไฟล์ที่แก้
- **migration ถัดไปคือ `066`** — ไฟล์ล่าสุดในรีโปคือ `migrations/065_tutorial_field_rule.sql`
- **`RunMigrations` ไม่หุ้ม transaction ให้** (`internal/database/migrations.go:41` ยิง `pool.Exec` ตรงๆ) → migration ต้องมี `BEGIN;` / `COMMIT;` เอง ตามแบบ `063_tutorial_agents.sql`
- **migration ต้อง idempotent** — ใช้ `WHERE ... NOT LIKE` หรือ `replace()` แบบที่รันซ้ำแล้วไม่สะสม ตามแบบ `051_visual_qa_caption_context.sql`
- **`renderTemplate` = string-replace ไม่ใช่ text/template** — ห้ามใส่ `{{if}}` / `{{range}}` ใน `prompt_template`
- **visual_qa ต้อง fail-open เรื่อง infra ต่อไป** — template error / vision error / ไม่มีเฟรม = `ok=true` เสมอ (`visualqa.go:117-148`) ห้ามแตะนโยบายนี้
- **ZWSP (U+200B) ห้ามหลุดเข้า TTS** — แผนนี้ไม่แตะ pipeline เสียง แต่ไฟล์ `.live-test` ที่สร้างต้องไม่ถูก import เข้าโค้ดจริง
- แบรนด์: navy + ส้ม, มาสคอตเสือดาว
- Branch: `feat/qa-thai-wordbreak-criterion` (ตัดจาก `master`)

## File Structure

| ไฟล์ | หน้าที่ |
|---|---|
| `internal/agent/visualqa.go` (modify) | เพิ่ม `Codes` ลง `SceneVerdict`/`visionVerdict`, เพิ่ม `stickyCodes` + `HasStickyCode`, ให้ `ConfirmMerge` ข้ามการเคลียร์ verdict ที่ sticky |
| `internal/agent/visualqa_confirm_test.go` (modify) | เทสต์ sticky: ตำหนิ `wordbreak` ต้องรอดรอบยืนยันที่บอกว่าผ่าน |
| `internal/agent/visualqa_wordbreak_test.go` (create) | เทสต์ `HasStickyCode` + เทสต์ parity ระหว่าง `stickyCodes` กับ enum ใน migration 066 |
| `internal/orchestrator/orchestrator.go` (modify, `~704-723`) | ไม่ยิงรอบยืนยันให้ซีนที่ตำหนิเป็น sticky ล้วน (ประหยัด credit + กันการเคลียร์) |
| `migrations/066_visual_qa_thai_wordbreak.sql` (create) | เติมเกณฑ์เข้า `visual_qa` (system_prompt + skills + prompt_template) และเข้า `auto_review` (system_prompt + skills) |
| `.live-test/qa-wordbreak/repro.html` (create) | เรนเดอร์กล่องแคปชั่นด้วย CSS/ฟอนต์/ความกว้างตัวจริง หาข้อความที่ Chromium ตัดกลางคำจริง แล้วสร้างคู่ bad/good |
| `.live-test/qa-wordbreak/eval.py` (create) | ยิง prompt ตัวใหม่ผ่าน kie.ai Claude vision ใส่เฟรมที่ label ไว้ แล้วรายงาน confusion matrix |

---

### Task 1: sticky verdict codes — ตำหนิที่รอบยืนยันเคลียร์ไม่ได้

**Files:**
- Modify: `internal/agent/visualqa.go:36-46` (structs), `:147` (verdict ที่คืน), `:167-192` (`ConfirmMerge`)
- Create: `internal/agent/visualqa_wordbreak_test.go`
- Modify: `internal/agent/visualqa_confirm_test.go`
- Modify: `internal/orchestrator/orchestrator.go:704-723`

**Interfaces:**
- Consumes: `VisualQAResult`, `SceneVerdict`, `ConfirmMerge` ที่มีอยู่แล้วใน `internal/agent/visualqa.go`
- Produces:
  - `SceneVerdict.Codes []string` (JSON tag `codes,omitempty`) — Task 2 ทำให้โมเดลเติมค่านี้
  - `func HasStickyCode(codes []string) bool` — orchestrator เรียก
  - `var stickyCodes map[string]bool` (package-private) — Task 2 เอาไปเทียบกับ enum ใน migration

**บริบทที่ต้องรู้ก่อนแก้ (อ่านก่อนเขียนโค้ด):**

`ConfirmMerge` ตอนนี้ทำงานแบบนี้: ซีนที่รอบแรกตก จะ "ตกจริง" ก็ต่อเมื่อรอบยืนยันตกด้วย ถ้ารอบยืนยันไม่เจอ = เคลียร์ทิ้ง. รอบยืนยันสุ่มเฟรมที่ `qaRecheckSceneFrac = 0.85` ส่วนรอบแรกที่ `qaSceneFrac = 0.6` — คอมเมนต์ที่ `orchestrator.go:1064-1067` ระบุเองว่าสองจุดนี้เป็น "**คนละวลีคาราโอเกะ**". นั่นถูกต้องสำหรับตำหนิที่เกิดจากจังหวะ (อนิเมชันยังไม่นิ่ง) แต่ผิดสำหรับตำหนิที่ผูกกับตัวข้อความ: คำที่ถูกผ่ากลางในวลีที่ 2 ย่อมไม่ปรากฏในวลีที่ 5 → รอบยืนยันเคลียร์ตำหนิจริงทิ้ง ทั้งที่คนดูเห็นเต็มตาตอนวินาทีนั้น

- [ ] **Step 1: เขียนเทสต์ที่ต้องแดง — `HasStickyCode` + sticky รอดรอบยืนยัน**

สร้าง `internal/agent/visualqa_wordbreak_test.go`:

```go
package agent

import "testing"

func TestHasStickyCode(t *testing.T) {
	cases := []struct {
		name  string
		codes []string
		want  bool
	}{
		{"wordbreak ตรงตัว", []string{"wordbreak"}, true},
		{"wordbreak ปนกับรหัสอื่น", []string{"overflow", "wordbreak"}, true},
		{"ตัวพิมพ์ใหญ่/ช่องว่างต้องยังจับได้", []string{" WordBreak "}, true},
		{"รหัสที่ไม่ sticky", []string{"overflow", "ai_artifact"}, false},
		{"ไม่มีรหัสเลย", nil, false},
		{"รหัสว่าง", []string{""}, false},
	}
	for _, c := range cases {
		if got := HasStickyCode(c.codes); got != c.want {
			t.Errorf("%s: HasStickyCode(%v) = %v, want %v", c.name, c.codes, got, c.want)
		}
	}
}

// TestStickyCodesNonEmpty กันการลบรหัสทิ้งโดยไม่ตั้งใจ — ถ้า map ว่าง ConfirmMerge
// จะกลับไปเคลียร์ตำหนิ wordbreak ทิ้งเงียบๆ เหมือนก่อนแก้
func TestStickyCodesNonEmpty(t *testing.T) {
	if !stickyCodes["wordbreak"] {
		t.Fatal(`stickyCodes ต้องมี "wordbreak" — ไม่งั้นรอบยืนยันจะเคลียร์ตำหนิคำไทยถูกตัดกลางคำทิ้ง`)
	}
}
```

เพิ่มลงท้าย `internal/agent/visualqa_confirm_test.go`:

```go
// TestConfirmMerge_StickyCodeSurvivesCleanConfirm คือหัวใจของแผนนี้: เฟรมยืนยัน
// ที่ 85% เป็นคนละวลีคาราโอเกะ จึงไม่มีสิทธิ์ล้างตำหนิที่ผูกกับตัวข้อความ
func TestConfirmMerge_StickyCodeSurvivesCleanConfirm(t *testing.T) {
	first := VisualQAResult{Verdicts: []SceneVerdict{
		{SceneNumber: 1, OK: false, Issues: []string{`คำว่า "แอดมิน" ถูกผ่ากลางเป็น "แอด"/"มิน"`}, Codes: []string{"wordbreak"}},
	}}
	confirm := VisualQAResult{Verdicts: []SceneVerdict{{SceneNumber: 1, OK: true}}, Passed: true}

	got := ConfirmMerge(first, confirm)

	if got.Passed {
		t.Fatal("ตำหนิ sticky ต้องไม่ถูกรอบยืนยันเคลียร์")
	}
	if got.Verdicts[0].OK {
		t.Fatal("verdict ซีน 1 ต้องยังเป็น OK=false")
	}
	if len(got.Verdicts[0].Codes) != 1 || got.Verdicts[0].Codes[0] != "wordbreak" {
		t.Fatalf("codes ต้องถูกส่งต่อ ได้ %v", got.Verdicts[0].Codes)
	}
}

// TestConfirmMerge_NonStickyStillClears ยืนยันว่าพฤติกรรมเดิม (ลด false positive
// ของตำหนิที่เกิดจากจังหวะ) ยังอยู่ครบ
func TestConfirmMerge_NonStickyStillClears(t *testing.T) {
	first := VisualQAResult{Verdicts: []SceneVerdict{
		{SceneNumber: 1, OK: false, Issues: []string{"หัวข้อดูล้นกรอบ"}, Codes: []string{"overflow"}},
	}}
	confirm := VisualQAResult{Verdicts: []SceneVerdict{{SceneNumber: 1, OK: true}}, Passed: true}

	got := ConfirmMerge(first, confirm)

	if !got.Passed {
		t.Fatal("ตำหนิที่ไม่ sticky ต้องยังถูกรอบยืนยันเคลียร์ได้เหมือนเดิม")
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่าแดง**

Run: `go test ./internal/agent/ -run 'TestHasStickyCode|TestStickyCodesNonEmpty|TestConfirmMerge_' -v`
Expected: คอมไพล์ไม่ผ่าน — `undefined: HasStickyCode`, `undefined: stickyCodes`, `unknown field Codes in struct literal of type SceneVerdict`

- [ ] **Step 3: เพิ่ม `Codes` เข้า struct ทั้งสองตัว**

ใน `internal/agent/visualqa.go` แทนที่บล็อก `SceneVerdict` + `visionVerdict` (บรรทัด 36-46) ด้วย:

```go
// SceneVerdict คือคำตัดสินของโมเดลสำหรับหนึ่งซีน. OK=false คือตำหนิที่มั่นใจว่าจริง
// และต้องบล็อกการเผยแพร่อัตโนมัติ. Issues คือเหตุผลภาษาไทย (เขียนลง visual_qa ด้วย).
// Codes คือรหัสประเภทตำหนิจากชุดปิดที่ prompt กำหนด — ใช้แยกว่าตำหนิไหน "ขึ้นกับ
// จังหวะเวลา" (รอบยืนยันตัดสินซ้ำได้) กับไหน "ผูกกับตัวข้อความ" (รอบยืนยันตัดสินซ้ำไม่ได้).
type SceneVerdict struct {
	SceneNumber int      `json:"scene_number"`
	OK          bool     `json:"ok"`
	Issues      []string `json:"issues"`
	Codes       []string `json:"codes,omitempty"`
}

// visionVerdict คือ JSON ดิบที่โมเดลคืนต่อหนึ่งเฟรม
type visionVerdict struct {
	OK     bool     `json:"ok"`
	Issues []string `json:"issues"`
	Codes  []string `json:"codes"`
}
```

- [ ] **Step 4: ส่ง `Codes` ต่อจากโมเดลเข้า verdict**

ใน `internal/agent/visualqa.go` แทนบรรทัด 147:

```go
	return SceneVerdict{SceneNumber: f.SceneNumber, OK: out.OK, Issues: out.Issues}
```

ด้วย:

```go
	return SceneVerdict{SceneNumber: f.SceneNumber, OK: out.OK, Issues: out.Issues, Codes: out.Codes}
```

- [ ] **Step 5: เพิ่ม `stickyCodes` + `HasStickyCode`**

แทรกใน `internal/agent/visualqa.go` เหนือ `func ConfirmMerge` (คือเหนือบรรทัด 160 เดิม):

```go
// stickyCodes คือรหัสตำหนิที่ "ผูกกับตัวข้อความ ไม่ใช่จังหวะเวลา" — รอบยืนยันเคลียร์
// ไม่ได้. เหตุผล: เฟรมยืนยันถูกสุ่มที่ 85% ของซีน ซึ่งเป็นคนละวลีคาราโอเกะกับเฟรมรอบแรก
// ที่ 60% (ดู qaRecheckSceneFrac ใน orchestrator). คำที่ถูกผ่ากลางในวลีหนึ่งย่อมไม่โผล่
// ในอีกวลีหนึ่ง ถ้าปล่อยให้เคลียร์ได้ ตำหนิจริงที่คนดูเห็นเต็มตาตอนวินาทีนั้นจะหลุดขึ้น
// YouTube เงียบๆ ทุกครั้ง. two-strike มีไว้ฆ่า false positive จากอนิเมชันที่ยังไม่นิ่ง
// เท่านั้น — ตำหนิคลาสนี้ไม่ได้เกิดจากอนิเมชัน จึงไม่อยู่ในขอบเขตของมัน.
var stickyCodes = map[string]bool{
	"wordbreak": true,
}

// HasStickyCode บอกว่า verdict นี้มีรหัสที่รอบยืนยันแตะไม่ได้ไหม. ทน whitespace และ
// ตัวพิมพ์ใหญ่เพราะค่านี้มาจาก LLM ไม่ใช่จาก enum ในโค้ด.
func HasStickyCode(codes []string) bool {
	for _, c := range codes {
		if stickyCodes[strings.ToLower(strings.TrimSpace(c))] {
			return true
		}
	}
	return false
}
```

เพิ่ม `"strings"` เข้า import block ที่หัวไฟล์ (`internal/agent/visualqa.go:3-11`) ให้เป็น:

```go
import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/jaochai/video-fb/internal/models"
	"golang.org/x/sync/errgroup"
)
```

- [ ] **Step 6: ให้ `ConfirmMerge` เคารพ sticky**

ใน `internal/agent/visualqa.go` ใน loop ของ `ConfirmMerge` แทรกบล็อก sticky ก่อนการเช็ค `confirmFailed` — ผลลัพธ์ของ loop ต้องเป็นแบบนี้:

```go
	out := make([]SceneVerdict, len(first.Verdicts))
	for i, v := range first.Verdicts {
		if v.OK {
			out[i] = v
			continue
		}
		if HasStickyCode(v.Codes) {
			// ตำหนิผูกกับตัวข้อความ — เฟรมยืนยันเป็นคนละวลี ไม่มีสิทธิ์ล้างผลรอบแรก
			out[i] = v
			continue
		}
		if issues, still := confirmFailed[v.SceneNumber]; still {
			merged := append(append([]string{}, v.Issues...), issues...)
			out[i] = SceneVerdict{SceneNumber: v.SceneNumber, OK: false, Issues: merged, Codes: v.Codes}
			continue
		}
		note := append([]string{"เฟรมยืนยัน (recheck) ไม่พบปัญหา — เคลียร์ผลรอบแรก"}, v.Issues...)
		out[i] = SceneVerdict{SceneNumber: v.SceneNumber, OK: true, Issues: note}
	}
```

และแก้คอมเมนต์หัวฟังก์ชัน `ConfirmMerge` (บรรทัด 160-166 เดิม) โดยเติมประโยคท้าย:

```go
// ...fail-open, matching reviewFrame's infra policy. ข้อยกเว้นเดียวคือ verdict ที่พก
// รหัสใน stickyCodes — รอบยืนยันแตะไม่ได้ (ดูเหตุผลที่ stickyCodes).
```

- [ ] **Step 7: รันเทสต์ให้ผ่าน**

Run: `go test ./internal/agent/ -run 'TestHasStickyCode|TestStickyCodesNonEmpty|TestConfirmMerge_' -v`
Expected: PASS ทุกตัว รวมเทสต์ `TestConfirmMerge_*` เดิม 5 ตัวที่ต้องไม่พัง

- [ ] **Step 8: ไม่ยิงรอบยืนยันให้ซีนที่ sticky ล้วน**

ใน `internal/orchestrator/orchestrator.go` แทนบล็อก `if !qaRes.Passed { ... }` (บรรทัด 704-723) ด้วย:

```go
		if !qaRes.Passed {
			// Two-strike confirm: re-sample every flagged scene later in the scene
			// (past the entrance animation, on a different caption phrase) and
			// re-judge. A scene only stays failed when BOTH frames show the defect.
			// ยกเว้นตำหนิที่พกรหัส sticky (เช่น คำไทยถูกตัดกลางคำ) — เฟรมยืนยันเป็น
			// คนละวลีคาราโอเกะ จึงตัดสินซ้ำไม่ได้ ไม่ต้องเสีย credit ยิงเลย
			flagged := make(map[int]bool)
			for _, v := range qaRes.Verdicts {
				if !v.OK && !agent.HasStickyCode(v.Codes) {
					flagged[v.SceneNumber] = true
				}
			}
			if len(flagged) > 0 {
				confirmFrames := o.extractQAFramesAt(clipID, result.LocalVideo916Path, scenes, qaRecheckSceneFrac, probedDur, flagged)
				confirmRes := o.visualQAAgent.Review(ctx, agent.VisualQAInput{
					Question: q.Question,
					Frames:   confirmFrames,
					Fast:     producer.PipelineFastEnabled(),
				}, qaCfg)
				qaRes = agent.ConfirmMerge(qaRes, confirmRes)
				log.Printf("visualqa: clip %s confirm pass done — %d scene(s) rechecked, passed=%v",
					clipID, len(confirmFrames), qaRes.Passed)
			} else {
				log.Printf("visualqa: clip %s — ตำหนิทุกซีนเป็นชนิดที่รอบยืนยันตัดสินซ้ำไม่ได้ ข้ามรอบยืนยัน", clipID)
			}
		}
```

- [ ] **Step 9: build + รันเทสต์ทั้งรีโป**

Run: `go build ./... && go test ./internal/agent/ ./internal/orchestrator/ ./internal/producer/ ./internal/models/`
Expected: build ผ่าน, เทสต์ทุกแพ็กเกจ `ok`

- [ ] **Step 10: commit**

```bash
git add internal/agent/visualqa.go internal/agent/visualqa_confirm_test.go internal/agent/visualqa_wordbreak_test.go internal/orchestrator/orchestrator.go
git commit -m "feat(qa): รหัสประเภทตำหนิ + sticky code ที่รอบยืนยันเคลียร์ไม่ได้

เฟรมยืนยันที่ 85% ของซีนเป็นคนละวลีคาราโอเกะกับเฟรมรอบแรกที่ 60% ตำหนิที่ผูกกับ
ตัวข้อความ (คำไทยถูกผ่ากลางคำ) จึงถูก ConfirmMerge เคลียร์ทิ้งทุกครั้งทั้งที่คนดูเห็นจริง
two-strike มีไว้ฆ่า false positive จากอนิเมชันที่ยังไม่นิ่งเท่านั้น ไม่ใช่ตำหนิคลาสนี้"
```

---

### Task 2: migration 066 — เกณฑ์ "คำไทยถูกตัดกลางคำ" เข้า visual_qa + auto_review

**Files:**
- Create: `migrations/066_visual_qa_thai_wordbreak.sql`
- Modify: `internal/agent/visualqa_wordbreak_test.go` (เพิ่มเทสต์ parity)

**Interfaces:**
- Consumes: `stickyCodes` จาก Task 1 (เทสต์เทียบว่ารหัสใน migration สะกดตรงกัน)
- Produces: แถว `agent_configs` ของ `visual_qa` และ `auto_review` ที่ prompt รู้จักตำหนิคลาสนี้ และ `prompt_template` ของ `visual_qa` ขอ field `codes`

**บริบทที่ต้องรู้ก่อนเขียน migration (อ่านก่อน):**

1. `visual_qa.system_prompt` ปัจจุบันมีบรรทัดนี้อยู่ ซึ่ง**กดทับ**เกณฑ์ใหม่ถ้าไม่เขียนข้อยกเว้นให้ชัด:
   > กล่องแคปชั่นล่างสุด (กรอบขอบส้ม) คือซับไตเติลคาราโอเกะ: แสดงบทพากย์ทีละ "วลีสั้นๆ" ไม่ใช่ประโยคเต็ม — วลีสั้น/ขึ้นต้นกลางประโยค = ปกติ **ห้ามตีความว่า "ข้อความถูกตัด/อ่านไม่ครบ"**
   
   เกณฑ์ใหม่ต้องแยกให้ขาดระหว่าง "วลีไม่เต็มประโยค" (ปกติ, กฎเดิมยังใช้) กับ "คำเดียวถูกผ่ากลางสองบรรทัด" (พัง, กฎเดิมไม่คุ้มครอง)

2. `auto_review` คือด่านสุดท้าย: คลิปที่ visual_qa ตี needs_review จะถูก tick ทุก 10 นาทีเอาไป judge ต่อ ถ้ามัน `approve` ด้วย confidence ≥ threshold คลิปจะ publish ทันที `auto_review` สุ่มเฟรมที่ `autoReviewSceneFrac = 0.45` — **คนละจังหวะคาราโอเกะอีกจุดหนึ่ง** ถ้าไม่บอกมันไว้ มันจะเห็นเฟรมสะอาดแล้วสรุปว่า QA ตีตกผิด → approve ทับ → งานทั้งหมดใน Task 1 สูญเปล่า

3. `auto_review.prompt_template` เป็นสตริงว่าง — user prompt ประกอบใน Go (`internal/agent/autoreview.go:78-94`) migration จึงต้องแก้ที่ `system_prompt` และ `skills` เท่านั้น

- [ ] **Step 1: เขียนเทสต์ parity ให้แดงก่อน**

ใน `internal/agent/visualqa_wordbreak_test.go` แทน import เดิม (`import "testing"`) ด้วย:

```go
import (
	"os"
	"strings"
	"testing"
)
```

แล้วเพิ่มฟังก์ชันนี้ลงท้ายไฟล์:

```go
// TestMigration066DeclaresStickyCodes กันความพังที่เงียบที่สุดของฟีเจอร์นี้: ถ้า prompt
// สะกดรหัสไม่ตรงกับ stickyCodes ใน Go (เช่น "word_break" กับ "wordbreak") ตำหนิจะไหลผ่าน
// รอบยืนยันไปเผยแพร่โดยไม่มีใครรู้ว่าสาเหตุอยู่ที่การสะกด ไม่ใช่ที่โมเดล
func TestMigration066DeclaresStickyCodes(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/066_visual_qa_thai_wordbreak.sql")
	if err != nil {
		t.Fatalf("อ่าน migration 066 ไม่ได้: %v", err)
	}
	sql := string(raw)

	for code := range stickyCodes {
		if !strings.Contains(sql, `"`+code+`"`) {
			t.Errorf("migration 066 ไม่ได้ประกาศรหัส %q ให้โมเดล — ConfirmMerge จะไม่มีวันเห็นมัน", code)
		}
	}
	if !strings.Contains(sql, "BEGIN;") || !strings.Contains(sql, "COMMIT;") {
		t.Error("migration ต้องหุ้ม BEGIN/COMMIT เอง — RunMigrations ไม่หุ้มให้")
	}
	if !strings.Contains(sql, "auto_review") {
		t.Error("migration ต้องแก้ auto_review ด้วย ไม่งั้นมัน approve ทับตำหนิที่ visual_qa จับได้")
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่าแดง**

Run: `go test ./internal/agent/ -run TestMigration066 -v`
Expected: FAIL — `อ่าน migration 066 ไม่ได้: open ../../migrations/066_visual_qa_thai_wordbreak.sql: no such file or directory`

- [ ] **Step 3: เขียน migration**

สร้าง `migrations/066_visual_qa_thai_wordbreak.sql`:

```sql
-- 066: เกณฑ์ "คำไทยถูกตัดกลางคำ" สำหรับ visual_qa + auto_review
--
-- ทำไมต้องมี: ตำหนิคลาสนี้มองไม่เห็นจากทุกด่านอัตโนมัติที่มีอยู่ — hyperframes inspect
-- ผ่านเพราะกล่องไม่ล้น (คำถูกตัดสวยงามอยู่ในกรอบ) และ visual_qa ไม่เคยถูกสอนให้มองหา
-- ซ้ำร้าย system_prompt เดิมสั่งไว้ว่า "วลีคาราโอเกะไม่เต็มประโยค = ปกติ ห้ามตีความว่า
-- ข้อความถูกตัด" ซึ่งกดทับเกณฑ์นี้พอดี ต้องเขียนข้อยกเว้นให้ชัด
--
-- คู่กับ commit ที่เพิ่ม stickyCodes ใน internal/agent/visualqa.go — รหัส "wordbreak"
-- ต้องสะกดตรงกันเป๊ะ (TestMigration066DeclaresStickyCodes บังคับไว้)
--
-- RunMigrations ไม่หุ้ม transaction ให้ ต้อง BEGIN/COMMIT เอง
--
-- Rollback:
--   UPDATE agent_configs SET system_prompt = split_part(system_prompt, E'\n\nเกณฑ์เพิ่ม — "คำไทยถูกตัดกลางคำ"', 1)
--    WHERE agent_name IN ('visual_qa','auto_review');
--   (ส่วน prompt_template ปล่อยไว้ได้ — field codes เกินมาไม่ทำให้อะไรพัง)

BEGIN;

-- ── visual_qa: เกณฑ์ + ข้อยกเว้นของกฎคาราโอเกะ + ตัวกันตีตกผิด ──
UPDATE agent_configs SET
  system_prompt = system_prompt || E'\n\nเกณฑ์เพิ่ม — "คำไทยถูกตัดกลางคำ" (รหัส wordbreak):\nภาษาไทยไม่มีช่องว่างระหว่างคำ Chromium จึงขึ้นบรรทัดใหม่กลางคำได้ — เป็นบั๊กจริงที่คนดูเห็น ไม่ใช่ผลของจังหวะคาราโอเกะ. ตรวจข้อความทุกที่บนจอ ทั้งกล่องแคปชั่นล่างและหัวข้อ/ข้อความในการ์ด.\n- พัง (ok=false และใส่ "wordbreak" ใน codes): ชิ้นท้ายบรรทัดบนกับชิ้นต้นบรรทัดล่างต่อกันเป็น "คำเดียว" และแยกกันแล้วแต่ละชิ้นไม่เป็นคำ เช่น "แอด"/"มิน", "คอนเวอร์"/"ชัน", "แดช"/"บอร์ด", "อัลกอ"/"ริทึม", "เฟซบุ๊"/"ก".\n- สัญญาณที่ชัดที่สุด: บรรทัดล่างขึ้นต้นด้วยสระ วรรณยุกต์ หรือตัวการันต์ที่ยืนเดี่ยวไม่ได้ — เห็นแบบนี้คือถูกตัดกลางคำแน่นอน.\n- ไม่พัง (ok=true): ขึ้นบรรทัดใหม่ "ระหว่างคำ" เช่น บรรทัดบนจบด้วย "ยิงแอด" บรรทัดล่างขึ้นด้วย "ให้ปัง" — ภาษาไทยตัดบรรทัดแบบนี้เป็นเรื่องปกติ ห้ามตั้ง ok=false.\n- ข้อยกเว้นของกฎคาราโอเกะข้างบน: "วลีไม่เต็มประโยค / ขึ้นต้นกลางประโยค" ยังถือว่าปกติเหมือนเดิมทุกประการ. แต่ "คำเดียวถูกผ่ากลางคนละบรรทัด" พังเสมอ — กฎคาราโอเกะไม่คุ้มครองกรณีนี้.\n- ถ้าอ่านตัวอักษรไม่ชัด หรือไม่แน่ใจว่าเป็นการตัดกลางคำหรือระหว่างคำ ให้ ok=true (fail-open เหมือนเดิม).',
  skills = skills || E'\n- คำไทยถูกผ่ากลางคำคนละบรรทัด = พังเสมอ (codes: "wordbreak") — กฎ "วลีคาราโอเกะไม่เต็มประโยค = ปกติ" ไม่คุ้มครองกรณีนี้.\n- ตัดบรรทัดระหว่างคำ = ปกติ; อ่านไม่ชัด/ไม่แน่ใจ = ok=true.'
WHERE agent_name = 'visual_qa'
  AND system_prompt NOT LIKE '%wordbreak%';

-- prompt_template: ขอ field codes เพิ่มจาก JSON เดิม. replace() ทำงานครั้งเดียวเพราะ
-- สตริงเป้าหมายหายไปหลังแทนที่ และ WHERE กันการต่อท้ายซ้ำ
UPDATE agent_configs SET
  prompt_template = replace(prompt_template,
      E'{\n  "ok": true,\n  "issues": []\n}',
      E'{\n  "ok": true,\n  "issues": [],\n  "codes": []\n}')
    || E'\n\ncodes คือรหัสประเภทของตำหนิ เลือกจากชุดนี้เท่านั้น: "wordbreak" (คำไทยถูกตัดกลางคำ), "overflow" (ล้นกรอบหรือถูกครอปจนอ่านไม่ครบ), "brand_color" (สีหลุดแบรนด์), "baked_text" (ตัวหนังสือมั่วอบอยู่ในภาพ AI), "ai_artifact" (ภาพ AI เพี้ยน), "other" (อื่นๆ). ถ้า ok=true ให้ codes เป็น array ว่าง.'
WHERE agent_name = 'visual_qa'
  AND prompt_template NOT LIKE '%codes%';

-- ── auto_review: ด่านสุดท้ายก่อน publish ──
-- ถ้ามันไม่รู้จักตำหนิคลาสนี้ มันจะเห็นเฟรมของตัวเอง (สุ่มที่ 45% ของซีน = คนละจังหวะ
-- คาราโอเกะอีกจุด) แล้วสรุปว่า QA ตีตกผิด → approve ทับ → การบล็อกทั้งหมดสูญเปล่า
UPDATE agent_configs SET
  system_prompt = system_prompt || E'\n\nเกณฑ์เพิ่ม — "คำไทยถูกตัดกลางคำ": ถ้า Visual QA แจ้งว่ามีคำเดียวถูกผ่ากลางคนละบรรทัด (เช่น "แอด"/"มิน") ให้ถือเป็นตำหนิ deterministic — เรนเดอร์ใหม่ด้วยข้อความชุดเดิมจะตัดที่เดิม ห้ามตั้ง decision=retry และห้าม approve. ให้ decision=hold, defect_type=deterministic.\nสำคัญมาก: เฟรมที่คุณเห็นถูกจับคนละวินาทีกับเฟรมที่ Visual QA เห็น และแคปชั่นคาราโอเกะเปลี่ยนวลีไปแล้ว — "ผมไม่เจอในเฟรมของผม" ไม่ได้แปลว่า "ไม่มี". จะ approve ได้ต้องมีหลักฐานว่า Visual QA อ่านผิดจริง (เช่น สองชิ้นที่มันอ้างเป็นคนละคำที่ต่อกันได้ตามปกติอยู่แล้ว) เท่านั้น มิฉะนั้นให้ hold.',
  skills = skills || E'\n- QA แจ้งคำไทยถูกผ่ากลางบรรทัด → hold + defect_type=deterministic เสมอ ห้าม approve/retry เว้นแต่พิสูจน์ได้ว่า QA อ่านผิด'
WHERE agent_name = 'auto_review'
  AND system_prompt NOT LIKE '%คำไทยถูกตัดกลางคำ%';

COMMIT;
```

- [ ] **Step 4: รันเทสต์ให้ผ่าน**

Run: `go test ./internal/agent/ -run TestMigration066 -v`
Expected: PASS

- [ ] **Step 5: ตรวจว่า SQL รันได้จริงและ idempotent — บน Neon branch ชั่วคราวเท่านั้น**

ห้ามรันกับ default branch. สร้าง branch ทดสอบก่อน:

```
ใช้ MCP: mcp__plugin_neon_neon__create_branch  projectId=snowy-grass-75448787
```

จากนั้นรันเนื้อ migration ทั้งไฟล์ผ่าน `mcp__plugin_neon_neon__run_sql` โดยส่ง `branchId` ของ branch ที่เพิ่งสร้าง **สองรอบติดกัน** แล้วตรวจว่ารอบสองไม่สะสมข้อความ:

```sql
SELECT agent_name,
       (length(system_prompt) - length(replace(system_prompt, 'wordbreak', ''))) / length('wordbreak') AS wordbreak_hits,
       prompt_template LIKE '%"codes": []%' AS has_codes_json
FROM agent_configs WHERE agent_name IN ('visual_qa','auto_review') ORDER BY agent_name;
```

Expected: `visual_qa` → `wordbreak_hits = 2` (หนึ่งในเกณฑ์ หนึ่งใน skills), `has_codes_json = true`; `auto_review` → `wordbreak_hits = 0`, `has_codes_json = false`. ค่าต้อง**เท่าเดิมทั้งรอบแรกและรอบสอง**

เสร็จแล้วลบ branch ทิ้ง: `mcp__plugin_neon_neon__delete_branch`

- [ ] **Step 6: commit**

```bash
git add migrations/066_visual_qa_thai_wordbreak.sql internal/agent/visualqa_wordbreak_test.go
git commit -m "feat(qa): migration 066 — เกณฑ์คำไทยถูกตัดกลางคำเข้า visual_qa + auto_review

hyperframes inspect มองตำหนิคลาสนี้ไม่เห็นเพราะคำถูกตัดอยู่ในกรอบสวยงาม และ prompt เดิม
สั่งไว้ว่าวลีคาราโอเกะไม่เต็มประโยค = ปกติ ซึ่งกดทับเกณฑ์นี้พอดี จึงต้องเขียนข้อยกเว้นชัดๆ
auto_review ต้องรู้ด้วย ไม่งั้นมัน approve ทับสิ่งที่ visual_qa เพิ่งบล็อก"
```

---

### Task 3: ชุดเฟรมทองคำ + วัดผลจริงก่อนขึ้น prod

**Files:**
- Create: `.live-test/qa-wordbreak/repro.html`
- Create: `.live-test/qa-wordbreak/eval.py`
- Create: `.live-test/qa-wordbreak/RESULT.md` (ผลที่วัดได้)

**Interfaces:**
- Consumes: `visual_qa.system_prompt` + `prompt_template` หลัง migration 066 (ดึงมาเป็น `prompts.json`), kie.ai Claude vision endpoint `https://api.kie.ai/claude/v1/messages`
- Produces: ตัวเลข TP/FP/TN/FN ที่ตัดสินว่า prompt นี้ควรขึ้น prod ไหม

**บริบท:**

ประเด็นทั้งหมดของงานนี้คือ "ถ้าบั๊กแบบนี้กลับมาอีกจะไม่มีใครเห็นจนกว่าจะมีคนดูคลิป" — แผนที่แก้ prompt แล้วจบโดยไม่วัด ก็คือการหวังว่ามันจะได้ผล ซึ่งเป็นความพังแบบเดียวกัน. Task นี้จึงสร้างเฟรมที่ **รู้คำตอบล่วงหน้า** แล้ววัด

เฟรมต้องเรนเดอร์ด้วย geometry เดียวกับของจริง มิฉะนั้นวัดคนละอย่าง — จาก `internal/producer/templates/layout_multi_scene.html.tmpl:291-294` กล่อง `.cap-phrase` คือ: `width:920px; font-weight:700; font-size:48px; line-height:1.34; padding:26px 40px; text-align:center; background:rgba(8,24,64,.86); border:2px solid #FFB454; border-radius:24px` ฟอนต์ `Sarabun` และ `.cap-word{display:inline;white-space:normal;overflow-wrap:break-word}`

- [ ] **Step 1: เขียน repro.html ที่หาข้อความซึ่ง Chromium ตัดกลางคำจริง**

สร้าง `.live-test/qa-wordbreak/repro.html`:

```html
<!doctype html>
<meta charset="utf-8">
<title>qa-wordbreak repro</title>
<style>
  @font-face{font-family:"Sarabun";src:url("../../internal/producer/assets/fonts/Sarabun-Bold.ttf") format("truetype");font-weight:700;font-display:block}
  body{margin:0;background:#0A1633;font-family:"Sarabun",sans-serif}
  /* คัดลอกจาก .cap-phrase ใน layout_multi_scene.html.tmpl แบบตัวต่อตัว —
     ถ้า template เปลี่ยน ต้องมาแก้ที่นี่ด้วย ไม่งั้นวัดคนละกล่องกับของจริง */
  .cap-phrase{width:920px;text-align:center;font-weight:700;font-size:48px;line-height:1.34;
    color:#fff;background:rgba(8,24,64,.86);border:2px solid #FFB454;border-radius:24px;
    padding:26px 40px;box-shadow:0 18px 50px rgba(0,0,0,.5);margin:40px}
  .cap-word{display:inline;white-space:normal;overflow-wrap:break-word}
</style>
<div id="out"></div>
<script>
const ZWSP = "​";

// คำทับศัพท์ที่ ICU ของ Chromium แบ่งผิด — คัดจาก thaiLoanWords ใน internal/producer/thaiwrap.go
const LOAN = ["คอนเวอร์ชัน", "อัลกอริทึม", "แดชบอร์ด", "เฟซบุ๊ก", "แอดมิน"];

// ประโยคจริงแนวเนื้อหา Ads Vance หนึ่งประโยคต่อหนึ่งคำ
const SENTENCES = [
  "ตัวเลขคอนเวอร์ชันตกลงครึ่งหนึ่งภายในสามวันแบบไม่มีสัญญาณเตือน",
  "อัลกอริทึมของเมต้าเปลี่ยนรอบการเรียนรู้ใหม่ทุกครั้งที่คุณแก้งบ",
  "เปิดแดชบอร์ดแล้วดูตรงคอลัมน์ขวาสุดก่อนจะกดปิดแคมเปญทิ้ง",
  "บัญชีเฟซบุ๊กที่เพิ่งสร้างใหม่ยังไม่มีประวัติพอให้ระบบไว้ใจ",
  "ให้แอดมินอีกคนช่วยยืนยันตัวตนก่อนแล้วค่อยเพิ่มวิธีจ่ายเงิน",
];

// คำเติมหน้าที่ใช้ขยับจุดตัดบรรทัด — ทุกตัวเป็นคำไทยปกติ ไม่บิดความหมายประโยค
const FILLERS = ["", "จริงๆ ", "วันนี้", "ที่ผ่านมา", "หลายคนเจอว่า", "เท่าที่ดูมา", "บอกเลยว่า", "ล่าสุด"];

// วัดว่าข้อความในกล่องขึ้นบรรทัดใหม่ตรงไหน แล้วจุดนั้นอยู่กลางคำเป้าหมายหรือไม่
// วิธี: ห่อทีละอักขระใน <span> แล้วดู offsetTop ที่กระโดด = จุดขึ้นบรรทัด
function breakIndexes(box, text) {
  box.innerHTML = "";
  const spans = [...text].map(ch => {
    const s = document.createElement("span");
    s.textContent = ch;
    box.appendChild(s);
    return s;
  });
  const idx = [];
  let prevTop = spans.length ? spans[0].getBoundingClientRect().top : 0;
  for (let i = 1; i < spans.length; i++) {
    const top = spans[i].getBoundingClientRect().top;
    if (top > prevTop + 1) idx.push(i);
    prevTop = top;
  }
  return idx;
}

// จุดตัดอยู่ "กลางคำ" เมื่อมันตกในช่วง (start, end) ของคำเป้าหมายแบบไม่รวมขอบ
function cutsInsideWord(text, word, idx) {
  for (let at = text.indexOf(word); at !== -1; at = text.indexOf(word, at + 1)) {
    for (const i of idx) if (i > at && i < at + word.length) return { at, i };
  }
  return null;
}

const out = document.getElementById("out");
const probe = document.createElement("div");
probe.className = "cap-phrase";
probe.style.position = "absolute";
probe.style.left = "-9999px";
document.body.appendChild(probe);

const report = [];
document.fonts.ready.then(() => {
  SENTENCES.forEach((base, n) => {
    const word = LOAN[n];
    let found = null;
    for (const f of FILLERS) {
      const text = f + base;
      const hit = cutsInsideWord(text, word, breakIndexes(probe, text));
      if (hit) { found = { text, word, ...hit }; break; }
    }
    if (!found) { report.push({ n: n + 1, word, status: "ไม่พบจุดตัดกลางคำ" }); return; }

    // bad = ข้อความดิบ (Chromium ตัดกลางคำ) / good = ประกบ zwsp เหมือน GuardLoanWords
    for (const kind of ["bad", "good"]) {
      const el = document.createElement("div");
      el.className = "cap-phrase";
      el.id = `${kind}${n + 1}`;
      const inner = document.createElement("span");
      inner.className = "cap-word";
      inner.textContent = kind === "bad"
        ? found.text
        : found.text.split(word).join(ZWSP + word + ZWSP);
      el.appendChild(inner);
      out.appendChild(el);
    }
    report.push({ n: n + 1, word, status: "ok", text: found.text });
  });
  probe.remove();
  window.__report = report;
  window.__ready = true;
});
</script>
```

- [ ] **Step 2: เปิดใน Chromium แล้วยืนยันว่าหาจุดตัดกลางคำได้ครบ**

ใช้ Playwright MCP (ต้องเป็น Chromium — ICU ของ Node ให้ผลคนละชุด ตามที่ `.live-test/thai-wordbreak/scan.js` เตือนไว้):

```
mcp__plugin_playwright_playwright__browser_navigate  url=file:///Users/jaochai/Code/video-fb/.live-test/qa-wordbreak/repro.html
mcp__plugin_playwright_playwright__browser_evaluate  function=() => (window.__ready ? window.__report : "ยังไม่พร้อม")
```

Expected: array 5 รายการ ทุกรายการ `status: "ok"`
ถ้ามีรายการไหนขึ้น `"ไม่พบจุดตัดกลางคำ"` → เติมคำใน `FILLERS` หรือเปลี่ยนประโยคใน `SENTENCES` ให้ยาวขึ้นแล้วรันซ้ำ

- [ ] **Step 3: ถ่ายเฟรมทั้ง 10 ใบ**

ยิงทีละ element ด้วย `mcp__plugin_playwright_playwright__browser_take_screenshot` โดยระบุ `element` + `ref` ของ `#bad1..#bad5` และ `#good1..#good5` เซฟลง `.live-test/qa-wordbreak/frames/` ชื่อ `bad_1.png … bad_5.png`, `good_1.png … good_5.png`

ตรวจด้วยตาเอง 1 ใบ (`bad_1.png`) ว่าคำถูกผ่ากลางจริงตามที่ script บอก — ถ้า script ถูกแต่ภาพไม่ตรง แสดงว่าฟอนต์ไม่โหลด (จะ fallback ไป sans-serif แล้ว metric เพี้ยน)

- [ ] **Step 4: ดึง prompt ตัวจริงจาก prod DB ลงไฟล์**

ต้องดึงจาก DB ไม่ใช่ก๊อปจาก migration — เพราะ prompt จริงคือ prompt เดิมที่ต่อกับส่วนใหม่ และ `BuildSystemPrompt` ยังเอา `skills` + `insights` มาต่อท้ายอีก (`internal/models/agent.go:18-27`)

รัน `mcp__plugin_neon_neon__run_sql` (projectId `snowy-grass-75448787`, branch ทดสอบที่รัน 066 แล้ว):

```sql
SELECT json_build_object(
  'system', system_prompt
    || CASE WHEN skills   <> '' THEN E'\n\n## Skills & Guidelines\n'  || skills   ELSE '' END
    || CASE WHEN insights <> '' THEN E'\n\n## Performance Insights\n' || insights ELSE '' END,
  'template', prompt_template,
  'model', model,
  'temperature', temperature
)::text AS payload
FROM agent_configs WHERE agent_name = 'visual_qa';
```

เซฟผลลงเป็น `.live-test/qa-wordbreak/prompts.json`

- [ ] **Step 5: เขียน eval.py**

สร้าง `.live-test/qa-wordbreak/eval.py`:

```python
#!/usr/bin/env python3
"""วัดว่าเกณฑ์ wordbreak ใน prompt ของ visual_qa จับตำหนิได้จริงและไม่ตีตกภาพดี

  KIE_API_KEY=... python3 eval.py            # รันทุกเฟรมใน frames/
  KIE_API_KEY=... python3 eval.py --repeat 3 # ยิงซ้ำวัดความเสถียรของโมเดล

ไฟล์ใน frames/ ต้องชื่อ bad_*.png (คาดว่าโมเดลต้องจับได้) หรือ good_*.png (ต้องปล่อยผ่าน)
prompts.json ดึงมาจาก agent_configs ตัวจริง — อย่าก๊อปจาก migration เพราะของจริงคือ
prompt เดิมต่อกับส่วนใหม่ ต่อกับ skills ต่อกับ insights
"""
import base64
import json
import os
import sys
import urllib.request
from collections import Counter

URL = "https://api.kie.ai/claude/v1/messages"          # kieClaudeAPI ใน internal/agent/kiellm.go
MAX_TOKENS = 8000                                       # kieLLMMaxTokens
HERE = os.path.dirname(os.path.abspath(__file__))

KEY = os.environ.get("KIE_API_KEY") or ""
if not KEY:
    raise SystemExit("ตั้ง KIE_API_KEY ก่อนรัน — ดึงจาก Neon: SELECT value FROM settings WHERE key='kie_api_key'")


def render(tpl, scene_number, on_screen, voice):
    """เลียน renderTemplate ของ Go — string replace ล้วน ไม่ใช่ text/template"""
    return (tpl.replace("{{.Question}}", "วิธีดูแลบัญชีโฆษณาให้รอด")
               .replace("{{.SceneNumber}}", str(scene_number))
               .replace("{{.OnScreenText}}", on_screen)
               .replace("{{.VoiceText}}", voice))


def judge(cfg, png_path):
    img = base64.b64encode(open(png_path, "rb").read()).decode()
    user = render(cfg["template"], 1, "เตือนเรื่องบัญชีโฆษณา", "ระวังบัญชีโดนปิดโดยไม่รู้ตัว")
    body = {
        "model": cfg["model"],
        "system": cfg["system"],
        "max_tokens": MAX_TOKENS,
        "stream": False,
        "temperature": cfg["temperature"],
        "messages": [{"role": "user", "content": [
            {"type": "text", "text": user},
            {"type": "image", "source": {"type": "base64", "media_type": "image/png", "data": img}},
        ]}],
    }
    req = urllib.request.Request(URL, data=json.dumps(body).encode(),
                                 headers={"Authorization": "Bearer " + KEY,
                                          "Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=300) as r:
        resp = json.load(r)
    text = "".join(p.get("text", "") for p in resp.get("content", []))
    if "```" in text:
        text = text.split("```")[1]
        if text.startswith("json"):
            text = text[4:]
    v = json.loads(text[text.index("{"):text.rindex("}") + 1])
    return bool(v.get("ok")), [str(x) for x in v.get("issues", [])], [str(c) for c in v.get("codes", [])]


def main():
    repeat = 1
    if "--repeat" in sys.argv:
        repeat = int(sys.argv[sys.argv.index("--repeat") + 1])
    cfg = json.load(open(os.path.join(HERE, "prompts.json")))
    frames_dir = os.path.join(HERE, "frames")
    tally, rows = Counter(), []

    for name in sorted(os.listdir(frames_dir)):
        if not name.endswith(".png"):
            continue
        expect_bad = name.startswith("bad_")
        for k in range(repeat):
            ok, issues, codes = judge(cfg, os.path.join(frames_dir, name))
            flagged_wordbreak = (not ok) and any(c.strip().lower() == "wordbreak" for c in codes)
            if expect_bad:
                tally["TP" if flagged_wordbreak else "FN"] += 1
            else:
                tally["FP" if not ok else "TN"] += 1
            rows.append({"frame": name, "run": k + 1, "ok": ok, "codes": codes, "issues": issues})
            print(f"{name} #{k+1}: ok={ok} codes={codes} issues={issues}", flush=True)

    tp, fn, fp, tn = tally["TP"], tally["FN"], tally["FP"], tally["TN"]
    print(f"\nจับตำหนิได้ (recall)  : {tp}/{tp+fn}")
    print(f"ตีตกภาพดี (FP)       : {fp}/{fp+tn}")
    json.dump(rows, open(os.path.join(HERE, "eval_raw.json"), "w"), ensure_ascii=False, indent=1)


if __name__ == "__main__":
    main()
```

- [ ] **Step 6: รัน eval**

Run: `KIE_API_KEY=<คีย์จาก settings table> python3 .live-test/qa-wordbreak/eval.py --repeat 2`
Expected: พิมพ์ผลทีละเฟรม แล้วสรุป recall กับ FP

**เกณฑ์ตัดสินว่าผ่าน:**
- `bad_*` ต้องถูกจับพร้อมรหัส `wordbreak` **อย่างน้อย 4 จาก 5** (recall ≥ 80%)
- `good_*` ต้องถูกตีตก **ไม่เกิน 1 จาก 5** (FP ≤ 20%)

ถ้าไม่ผ่าน: อย่าดันขึ้น prod — กลับไปแก้ถ้อยคำใน migration 066 (ตัวที่มีผลมากที่สุดคือบรรทัด "สัญญาณที่ชัดที่สุด" กับตัวอย่างคู่คำ) แล้วรัน Step 4-6 ซ้ำ ห้ามแก้ `stickyCodes` เพื่อให้ตัวเลขสวย

- [ ] **Step 7: บันทึกผลลง RESULT.md**

สร้าง `.live-test/qa-wordbreak/RESULT.md` เขียนวันที่ที่รัน, จำนวนเฟรม, recall, FP, และรหัสที่โมเดลคืนมาจริงในแต่ละเฟรม. เอาไว้เทียบตอนแก้ prompt รอบหน้า — ไม่งั้นรอบหน้าจะไม่มีฐานเปรียบเทียบ

- [ ] **Step 8: /simplify แล้ว commit**

รัน `/simplify` กับ diff ทั้งหมดของ branch นี้ (ตามที่เคยตกลงกันไว้ว่าให้ทำก่อนขั้น commit) แล้ว:

```bash
git add .live-test/qa-wordbreak/
git commit -m "test(qa): ชุดเฟรมทองคำวัดเกณฑ์ wordbreak ก่อนขึ้น prod

เรนเดอร์กล่องแคปชั่นด้วย CSS ฟอนต์ และความกว้าง 920px ตัวเดียวกับ layout_multi_scene
แล้วหาจุดที่ Chromium ตัดกลางคำจริงด้วย Intl/getBoundingClientRect คู่ bad/good ต่างกัน
แค่ ZWSP ประกบคำเดียว จึงวัดเฉพาะเกณฑ์นี้ไม่ปนตัวแปรอื่น"
```

- [ ] **Step 9: ขึ้น prod**

รัน skill `pre-deploy-checklist` ก่อน แล้วเปิด PR เข้า `master`. หลัง merge Railway จะ deploy เองและรัน migration 066 อัตโนมัติ

หลัง deploy ตรวจ 2 อย่าง:

1. migration ลงจริง —
   ```sql
   SELECT filename, applied_at FROM schema_migrations WHERE filename LIKE '066%';
   ```
2. ดูคลิปรอบถัดไป (produce 12:00 / 18:00 / 21:00 ไทย) แล้วเช็คว่ามี verdict ที่พก `codes` แล้ว —
   ```sql
   SELECT clip_id, passed, issues FROM visual_qa
   WHERE created_at > now() - interval '1 day' AND issues::text LIKE '%codes%'
   ORDER BY created_at DESC LIMIT 5;
   ```
   ถ้าผ่านไป 3 คลิปแล้วยังไม่มี `codes` โผล่เลย = โมเดลไม่ทำตาม schema ใหม่ ต้องกลับไปแก้ `prompt_template` (ไม่ใช่แก้โค้ด — โค้ดฝั่ง Go ไม่แคร์ว่า `codes` ว่างหรือไม่มี)

---

## หมายเหตุสำหรับผู้อนุมัติแผน

**สิ่งที่แผนนี้ไม่ทำ (จงใจ):**

- **ไม่ทำข้อ 1 ของ brief** — two-strike กับ Claude เปิดอยู่แล้ว ไม่มี flag ให้เปิด: `orchestrator.go:704-723` เรียก `agent.ConfirmMerge` แบบไม่มีเงื่อนไข เข้ามาตั้งแต่ commit `f345544` และ prod DB ยืนยันว่ามันทำงานจริง (15 จาก 64 แถวใน `visual_qa` ช่วง 20 วันหลัง มีโน้ต "เฟรมยืนยัน (recheck) ไม่พบปัญหา") ตัวเลข 12.6% → 4.9% คือผลที่ backtest วัดกับ **Gemini** ไม่ใช่ส่วนต่างที่ Claude ยังไม่ได้รับ
- **ไม่ทำตัวตรวจ wordbreak แบบ deterministic ฝั่ง Go** — เป็นทางที่ถูกกว่าและแม่นกว่าในระยะยาว (รัน `Intl.Segmenter` ตอน `hyperframes inspect` แล้วเทียบกับจุดขึ้นบรรทัดจริง) แต่ต้องแก้ inspector ในฝั่ง hyperframes ซึ่งอยู่นอกขอบเขตที่สั่งมา ถ้าสนใจเป็นแผนแยกได้

**ต้นทุนที่เพิ่ม:** ศูนย์ credit ต่อคลิปในกรณีปกติ (`codes` เป็น field ในคำตอบเดิม ไม่ใช่คอลใหม่) และ **ลด** credit ลงในกรณีที่ตำหนิทุกซีนเป็น sticky — Step 8 ของ Task 1 ข้ามรอบยืนยันทิ้งไปเลย

**ความเสี่ยงที่รู้ตัว:** ตำหนิ `wordbreak` ที่เป็น false positive จะเคลียร์ไม่ได้อีกต่อไป → คลิปไป `needs_review` แล้วตกถึง `auto_review` ซึ่ง migration 066 สั่งให้ `hold` แทบทุกกรณี = ต้องมีคนกดเอง คิว needs_review จะยาวขึ้นถ้า FP สูง — นี่คือเหตุผลที่ Task 3 มีเกณฑ์ FP ≤ 20% เป็นด่านก่อนขึ้น prod
