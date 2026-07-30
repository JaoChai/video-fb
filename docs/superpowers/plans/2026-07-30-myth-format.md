# แผน implement: คลิป "จับความเชื่อผิด" (format `myth`) ช่อง 09:00

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** เพิ่มคลิปชนิดที่ 5 ที่ยกความเชื่อผิดเรื่องยิงแอด/เพจ/BM มาตัดสินด้วยหลักฐานจากคลัง `myth_beliefs` โดยคำพิพากษาและแหล่งอ้างมาจาก DB ไม่ใช่จาก LLM

**Architecture:** เดินตามรอย format `tutorial`/`basic` ที่มีอยู่ — content_format ที่ `enabled=false` (ไม่ให้สุ่มเข้า pool) + schedule/action ของตัวเอง + คลังหัวข้อใน DB + preset ที่ล็อกตาม format ผ่าน `presetFor()`. ของใหม่ที่ไม่มีใน format เดิม: ฟิลด์ซีน 2 ตัวที่ Go เขียนทับค่าจาก LLM เสมอ (`Meter`, `Source`) และตะแกรงตัวเลขสองสไตรก์ที่ส่งคลิปเข้า `needs_review` แทนการทิ้ง

**Tech Stack:** Go 1.x (pgx/v5, chi), Postgres (Neon), Chromium+GSAP render ผ่าน hyperframes, Go html/template สำหรับ layout

**Spec:** `docs/superpowers/specs/2026-07-30-myth-format-design.md`

## Global Constraints

- ภาษาไทยในทุกข้อความบนจอและทุก prompt · ห้าม emoji ในทุกฟิลด์เนื้อหา (ฟอนต์ Sarabun เรนเดอร์ไม่ได้ → tofu)
- CSS ที่เพิ่มต้อง Thai-safe: `letter-spacing >= 0`, `line-height >= 1.3`, ตัดคำด้วย `overflow-wrap:break-word` เท่านั้น
- **ห้ามพิมพ์ `-->` หรือ Go template action (`{{...}}`) ใน JS comment ภายใน `.tmpl`** — ทำให้ไฟล์ template พังทั้งไฟล์
- ทุก selector CSS ใหม่ต้อง scope ด้วย `[data-format='myth']` (layout `verdict`/`hook`/`cta`/`tip` ใช้ร่วมกับ format อื่น)
- `RunMigrations` **ไม่หุ้ม transaction** — ไฟล์ migration ต้องมี `BEGIN;`/`COMMIT;` เอง และต้อง idempotent
- `schedules` ไม่มี unique index บน `action` → INSERT ต้องใช้ `WHERE NOT EXISTS` (ห้าม `ON CONFLICT`)
- `agent_configs.agent_name` มี UNIQUE → ใช้ `ON CONFLICT (agent_name) DO NOTHING`
- schedule ใหม่ต้อง `enabled = FALSE` ตอน migrate · เปิดใช้ผ่าน `PATCH /api/v1/schedules/{id}` เท่านั้น (scheduler reload จาก API ไม่ใช่จาก DB)
- ทุกตัวกรองที่ทำให้ผลิตไม่ได้ต้องมี fallback — ห้ามคืน 0 คลิปเงียบๆ
- คำสั่งทดสอบ: `go test ./...` (ทั้งชุด) หรือ `go test ./internal/<pkg>/ -run <TestName> -v`
- ห้ามรัน backend local ต่อ prod DB (scheduler จะยิง cron จริง)

---

## File Structure

| ไฟล์ | หน้าที่ |
|---|---|
| `internal/models/myth.go` (สร้าง) | struct `MythBelief` + ค่าคงที่ verdict 3 ค่า |
| `internal/repository/myth_beliefs.go` (สร้าง) | หยิบ/นับใช้/park แถวคลัง + พื้นกันคลังยุบ |
| `internal/repository/myth_beliefs_test.go` (สร้าง) | เทสต์ตัวเลือกแบบ pure + รูปร่าง SQL (ไม่ต้องมี DB) |
| `internal/agent/myth.go` (สร้าง) | `MythBrief` (prompt block) + `FactNumberViolations` + `DisallowedClaimViolations` |
| `internal/agent/myth_test.go` (สร้าง) | เทสต์ตะแกรงตัวเลข/คำอ้างลอย |
| `internal/producer/myth_format.go` (สร้าง) | `MythPreset`, `ModeMyth`, `buildMythCoverPrompt` |
| `internal/producer/myth_format_test.go` (สร้าง) | เทสต์ preset/mode wiring |
| `internal/producer/composition_types.go` (แก้) | ฟิลด์ `Meter`, `Source` ใน `SceneContent` + `MythVerdict`, `MythSource` ใน `ScenesParams` |
| `internal/producer/composition.go` (แก้) | Go-inject `Meter`/`Source`/`Stamp` ทับค่าจาก LLM |
| `internal/producer/case_format.go` (แก้) | เพิ่ม `ModeMyth` ใน `promptForScene` + `imageScenesForMode` |
| `internal/producer/templates/layout_multi_scene.html.tmpl` (แก้) | CSS block `[data-format='myth']` + สาขา builder `belief`/`proof`/`verdict` |
| `internal/producer/composition_myth_render_test.go` (สร้าง) | render test ของโหมด myth |
| `internal/agent/scene_content.go` (แก้) | เพิ่ม `belief`, `proof` ใน `sceneLayouts` |
| `internal/orchestrator/myth.go` (สร้าง) | `ProduceMyth` + `mythGateFailure` + regen รอบเดียว |
| `internal/orchestrator/myth_test.go` (สร้าง) | เทสต์ mode/preset/gate |
| `internal/orchestrator/tutorial.go` (แก้) | `clipMode("myth")` |
| `internal/orchestrator/orchestrator.go` (แก้) | ส่ง `myth` ลง produceClip/produceClipWithID + ตะแกรง 2 จุด + retry โหลดคลังกลับ |
| `internal/scheduler/scheduler.go` (แก้) | `produceMyth` + `handlerFor("produce_myth")` |
| `internal/scheduler/myth_action_test.go` (สร้าง) | เทสต์ handler ไม่เป็น nil |
| `internal/handler/orchestrator.go` (แก้) | `TriggerMyth` |
| `internal/router/router.go` (แก้) | route `POST /api/v1/orchestrator/produce-myth` |
| `internal/repository/clips.go` + `internal/models/clip.go` (แก้) | คอลัมน์ `myth_belief` |
| `migrations/075_myth_format.sql` (สร้าง) | schema + agent rows + schedule + คลัง 12 แถว |

---

### Task 1: schema + model ของคลัง

**Files:**
- Create: `internal/models/myth.go`
- Create: `migrations/075_myth_format.sql` (ส่วนที่ 1: schema — ส่วนที่เหลือเติมใน Task 8)
- Modify: `internal/models/clip.go:31` (เพิ่มฟิลด์ `MythBelief`)
- Modify: `internal/repository/clips.go:24,70` (คอลัมน์ `myth_belief`)
- Test: `internal/models/myth_test.go`

**Interfaces:**
- Produces: `models.MythBelief` (ทุกฟิลด์ตามด้านล่าง), `models.MythVerdictFalse/HalfTrue/Outdated`,
  `models.CreateClipRequest.MythBelief string`, `models.Clip.MythBelief string`

- [ ] **Step 1: เขียนเทสต์ที่ล้มก่อน**

`internal/models/myth_test.go`:
```go
package models

import "testing"

// ค่า verdict ทั้งสามต้องตรงกับ CHECK constraint ใน migration 075 เป๊ะ
// ถ้าไฟล์ใดไฟล์หนึ่งเปลี่ยนคำเดียว คลิปจะได้ Meter ว่างแล้วมิเตอร์หายไปเงียบๆ
func TestMythVerdictValues(t *testing.T) {
	for _, want := range []string{"false", "half_true", "outdated"} {
		if !ValidMythVerdict(want) {
			t.Errorf("ValidMythVerdict(%q) = false, ต้องเป็น true", want)
		}
	}
	if ValidMythVerdict("maybe") {
		t.Error(`ValidMythVerdict("maybe") = true, ต้องเป็น false`)
	}
}

// คลิปต้องจำได้ว่าตัวเองมาจากแถวคลังไหน ไม่งั้น retry เต็มรูปแบบจะโหลดคลังกลับไม่ได้
// แล้วตะแกรงข้อเท็จจริงจะปิดเงียบทั้งรอบ (บั๊กเดียวกับที่เคยเกิดกับคลิป basic)
func TestCreateClipRequestCarriesMythBelief(t *testing.T) {
	req := CreateClipRequest{MythBelief: "bm_stronger_than_personal"}
	if req.MythBelief != "bm_stronger_than_personal" {
		t.Errorf("MythBelief = %q", req.MythBelief)
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่าล้ม**

Run: `go test ./internal/models/ -run TestMyth -v`
Expected: FAIL — `undefined: ValidMythVerdict`

- [ ] **Step 3: เขียน model**

`internal/models/myth.go`:
```go
package models

// ค่า verdict ของแถวคลัง — ต้องตรงกับ CHECK constraint ของ myth_beliefs.verdict
// (migration 075) เป๊ะทุกตัวอักษร เพราะ Go เอาค่านี้ไปเป็น SceneContent.Meter ตรงๆ
const (
	MythVerdictFalse    = "false"     // ไม่จริง
	MythVerdictHalfTrue = "half_true" // จริงครึ่งเดียว
	MythVerdictOutdated = "outdated"  // เคยจริง วันนี้ไม่จริงแล้ว
)

// ValidMythVerdict บอกว่าค่านี้เป็น verdict ที่ระบบรู้จักไหม ใช้ตรวจแถวคลังที่
// อ่านมาจาก DB ก่อนส่งต่อให้ template — แถวที่ค่าเพี้ยนต้องหยุดที่นี่ ไม่ใช่ไป
// โผล่เป็นมิเตอร์เปล่าในคลิปที่เผยแพร่แล้ว
func ValidMythVerdict(v string) bool {
	switch v {
	case MythVerdictFalse, MythVerdictHalfTrue, MythVerdictOutdated:
		return true
	}
	return false
}

// MythBelief คือหนึ่งแถวของคลังความเชื่อผิด = หัวข้อของคลิปหนึ่งตัว
// ทุกข้อเท็จจริงที่คลิปพูดได้ต้องมาจากฟิลด์ในนี้เท่านั้น (ดูตะแกรงใน internal/agent/myth.go)
type MythBelief struct {
	ID            string `json:"id"`
	BeliefKey     string `json:"belief_key"`
	BeliefTH      string `json:"belief_th"`
	WhyBelievedTH string `json:"why_believed_th"`
	Verdict       string `json:"verdict"`
	FactTH        string `json:"fact_th"`
	SourceLabel   string `json:"source_label"`
	SourceURL     string `json:"source_url"`
	NuanceTH      string `json:"nuance_th"`
	CostTH        string `json:"cost_th"`
	Audience      string `json:"audience"`
	Weight        int    `json:"weight"`
	Enabled       bool   `json:"enabled"`
}
```

- [ ] **Step 4: เพิ่มฟิลด์ใน clip model**

`internal/models/clip.go` — เพิ่มใต้บรรทัด `TutorialFeature`:
```go
	MythBelief       string    `json:"myth_belief"`          // myth_beliefs.belief_key ("" = ไม่ใช่คลิป myth)
```

หา struct `CreateClipRequest` ในไฟล์เดียวกันแล้วเพิ่มฟิลด์เดียวกัน:
```go
	MythBelief      string
```

- [ ] **Step 5: รันเทสต์ให้ผ่าน**

Run: `go test ./internal/models/ -run TestMyth -v`
Expected: PASS ทั้งสองเทสต์

- [ ] **Step 6: เขียน migration ส่วน schema**

`migrations/075_myth_format.sql`:
```sql
-- 075: format "จับความเชื่อผิด" (spec 2026-07-30) — คลิปที่ 5 ของวัน ช่อง 09:00
--
-- โครงเดียวกับ tutorial/basic: content_format ที่ปิดไว้ (ไม่ให้ตัวสุ่ม format ของ
-- รอบ 12:00/18:00 หยิบไปใช้) + คลังหัวข้อใน DB + schedule/action ของตัวเอง
--
-- เหตุที่ต้องมีคลัง ไม่ปล่อยให้ agent คิดความเชื่อ+ข้อเท็จจริงเอง: คลิปแนวนี้ผิด
-- ไม่ได้ ถ้าโมเดลเดาข้อเท็จจริง คลิปจะกลายเป็นตัวสร้างความเชื่อผิดใหม่เสียเอง
--
-- 6 แถวที่ needs_verify=FALSE คือแถวที่มีแหล่งอ้างจริงแล้ว · อีก 6 แถวลง
-- needs_verify=TRUE เพราะยังหาเอกสารยืนยันไม่ได้ (เช่น trust tier ที่แหล่งเป็น
-- reseller ไม่ใช่ Meta) — ตัวเลือกใน repository ข้ามแถว needs_verify ทั้งหมด
--
-- RunMigrations ไม่หุ้ม transaction ให้ — ต้อง BEGIN/COMMIT เอง
-- idempotent: CREATE TABLE IF NOT EXISTS + ADD COLUMN IF NOT EXISTS +
-- ON CONFLICT DO NOTHING + WHERE NOT EXISTS ทุกคำสั่ง

BEGIN;

CREATE TABLE IF NOT EXISTS myth_beliefs (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    belief_key        TEXT NOT NULL UNIQUE,
    belief_th         TEXT NOT NULL,
    why_believed_th   TEXT NOT NULL,
    verdict           TEXT NOT NULL CHECK (verdict IN ('false','half_true','outdated')),
    fact_th           TEXT NOT NULL,
    source_label      TEXT NOT NULL,
    source_url        TEXT NOT NULL DEFAULT '',
    nuance_th         TEXT NOT NULL,
    cost_th           TEXT NOT NULL,
    audience          TEXT NOT NULL DEFAULT 'account-buyer',
    needs_verify      BOOLEAN NOT NULL DEFAULT FALSE,
    verify_reason     TEXT NOT NULL DEFAULT '',
    last_verified_at  TIMESTAMPTZ,
    used_count        INTEGER NOT NULL DEFAULT 0,
    last_used_at      TIMESTAMPTZ,
    parked_until      TIMESTAMPTZ,
    weight            INTEGER NOT NULL DEFAULT 1,
    enabled           BOOLEAN NOT NULL DEFAULT TRUE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- คลิปต้องจำแถวคลังของตัวเองไว้ ไม่งั้น retry เต็มรูปแบบจะโหลดข้อเท็จจริงกลับไม่ได้
-- แล้วตะแกรงจะปิดเงียบทั้งรอบ (บั๊กเดียวกับที่เคยเกิดกับคลิป basic ตอน retryFull)
ALTER TABLE clips ADD COLUMN IF NOT EXISTS myth_belief TEXT NOT NULL DEFAULT '';

-- question_instruction เป็น NOT NULL ไม่มี default — ลืมใส่แล้ว migration ล้มทั้งไฟล์
INSERT INTO content_formats (format_name, display_name, question_instruction, script_instruction, enabled, weight)
VALUES ('myth', 'จับความเชื่อผิด',
        'หัวข้อมาจาก catalog myth_beliefs เท่านั้น — agent ไม่ต้องคิดหัวข้อเอง และห้ามเพิ่มข้อเท็จจริงที่ไม่อยู่ในแถวคลัง',
        'เขียนสคริปต์แบบพิสูจน์ด้วยหลักฐาน: เปิดด้วยความเสียหายของการเชื่อผิด -> ยกคำเชื่อขึ้นมาตรงๆ พร้อมบอกว่าทำไมคนถึงเชื่อ -> ตัดสินด้วยข้อเท็จจริงจากแหล่งอ้าง -> บอกส่วนที่จริงของความเชื่อนั้น -> สรุปสิ่งที่ควรทำแทน',
        FALSE, 1)
ON CONFLICT (format_name) DO NOTHING;

COMMIT;
```

- [ ] **Step 7: เพิ่มคอลัมน์ใน clips repo**

`internal/repository/clips.go:24` — ต่อท้าย `clipCols` (ชื่อ const อาจต่างเล็กน้อย ใช้ const ที่มี `tutorial_feature` อยู่):
```go
	production_stage, review_retry_count, auto_review_held, case_number, tutorial_feature, myth_belief`
```

บรรทัด ~70 INSERT: เพิ่ม `myth_belief` เป็นคอลัมน์สุดท้ายและ `$11` (เลข placeholder ต่อจากที่มี) พร้อมส่ง `req.MythBelief`
ทุกจุดที่ `Scan(...)` อ่าน `clipCols` ต้องเพิ่ม `&c.MythBelief` ต่อท้าย `&c.TutorialFeature` ด้วย — คอมไพเลอร์จับไม่ได้ (pgx ตรวจ runtime) ให้รัน `go test ./internal/repository/ -v` เพื่อยืนยัน

- [ ] **Step 8: ตรวจว่าคอมไพล์ผ่านทั้ง repo**

Run: `go build ./... && go test ./internal/models/ ./internal/repository/ -v`
Expected: build ผ่าน, เทสต์เดิมทั้งหมดยังผ่าน

- [ ] **Step 9: commit**

```bash
git add internal/models/myth.go internal/models/myth_test.go internal/models/clip.go \
        internal/repository/clips.go migrations/075_myth_format.sql
git commit -m "feat(myth): schema คลังความเชื่อผิด + model + คอลัมน์ clips.myth_belief"
```

---

### Task 2: repository ของคลัง — ตัวเลือกหัวข้อ + พื้นกันคลังยุบ

**Files:**
- Create: `internal/repository/myth_beliefs.go`
- Test: `internal/repository/myth_beliefs_test.go`

**Interfaces:**
- Consumes: `models.MythBelief`, `models.ValidMythVerdict` (Task 1)
- Produces:
  - `repository.NewMythBeliefsRepo(pool *pgxpool.Pool) *MythBeliefsRepo`
  - `(*MythBeliefsRepo).PickNext(ctx context.Context) (*models.MythBelief, error)` — nil = คลังว่าง
  - `(*MythBeliefsRepo).GetByKey(ctx context.Context, key string) (*models.MythBelief, error)`
  - `(*MythBeliefsRepo).MarkUsed(ctx context.Context, id string) error` — บวก used_count + park 14 วัน
  - `repository.MythParkDays = 14`, `repository.pickMythLeastUsed([]mythUsage) models.MythBelief`

- [ ] **Step 1: เขียนเทสต์ที่ล้มก่อน**

`internal/repository/myth_beliefs_test.go`:
```go
package repository

import (
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/models"
)

// ตัวเลือกต้องหยิบแถวที่ใช้น้อยที่สุด ไม่ใช่แถวแรกตามตัวอักษร ไม่งั้นคลิปวนซ้ำ
// หัวข้อเดิมทั้งที่คลังมีของอีกเป็นสิบแถว
func TestPickMythLeastUsed(t *testing.T) {
	got := pickMythLeastUsed([]mythUsage{
		{Belief: models.MythBelief{BeliefKey: "a"}, UsedCount: 5},
		{Belief: models.MythBelief{BeliefKey: "b"}, UsedCount: 1},
		{Belief: models.MythBelief{BeliefKey: "c"}, UsedCount: 3},
	})
	if got.BeliefKey != "b" {
		t.Errorf("pickMythLeastUsed = %q, ต้องเป็น \"b\" (ใช้น้อยสุด)", got.BeliefKey)
	}
}

// weight สูงต้องมีโอกาสถูกหยิบมากกว่าเมื่อ used_count เท่ากัน — เทสต์ระดับที่จับได้
// จริงคือ "แถว weight 0 ต้องไม่ถูกหยิบก่อนแถว weight 3 ที่ใช้เท่ากัน"
func TestPickMythPrefersWeight(t *testing.T) {
	got := pickMythLeastUsed([]mythUsage{
		{Belief: models.MythBelief{BeliefKey: "low", Weight: 0}, UsedCount: 2},
		{Belief: models.MythBelief{BeliefKey: "high", Weight: 3}, UsedCount: 2},
	})
	if got.BeliefKey != "high" {
		t.Errorf("pickMythLeastUsed = %q, ต้องเป็น \"high\"", got.BeliefKey)
	}
}

// เงื่อนไข "แถวที่ใช้ได้ตอนนี้" ต้องกันแถวที่ยังไม่ได้ยืนยันข้อเท็จจริงออกไป —
// แถว needs_verify คือแถวที่ยังหาแหล่งอ้างไม่ได้ ปล่อยให้หลุดไปเท่ากับคลิปพูดสิ่งที่
// ไม่มีใครยืนยัน
func TestMythPickSQLExcludesUnverified(t *testing.T) {
	for _, want := range []string{"needs_verify = FALSE", "enabled = TRUE", "parked_until"} {
		if !strings.Contains(mythPickSQL, want) {
			t.Errorf("mythPickSQL ขาดเงื่อนไข %q", want)
		}
	}
}

// พื้นกันคลังยุบต้องอยู่ใน UPDATE เดียวกับการ park ไม่ใช่แยกคำสั่ง — ไม่งั้นสองรอบ
// ที่รันชนกันจะเห็น "เหลือแถวสุดท้าย" พร้อมกันแล้ว park ทิ้งทั้งคู่
func TestMythMarkUsedHasFloorInSameStatement(t *testing.T) {
	if !strings.Contains(mythMarkUsedSQL, "used_count = used_count + 1") {
		t.Error("mythMarkUsedSQL ต้องบวก used_count")
	}
	if !strings.Contains(mythMarkUsedSQL, "SELECT COUNT(*)") {
		t.Error("mythMarkUsedSQL ต้องนับคลังในคำสั่งเดียวกัน (พื้นกันคลังยุบ)")
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่าล้ม**

Run: `go test ./internal/repository/ -run TestMyth -v; go test ./internal/repository/ -run TestPickMyth -v`
Expected: FAIL — `undefined: pickMythLeastUsed`, `undefined: mythPickSQL`

- [ ] **Step 3: เขียน repository**

`internal/repository/myth_beliefs.go`:
```go
package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
)

type MythBeliefsRepo struct {
	pool *pgxpool.Pool
}

func NewMythBeliefsRepo(pool *pgxpool.Pool) *MythBeliefsRepo {
	return &MythBeliefsRepo{pool: pool}
}

const mythBeliefCols = `id, belief_key, belief_th, why_believed_th, verdict, fact_th,
	source_label, source_url, nuance_th, cost_th, audience, weight, enabled`

// MythParkDays คือระยะพักหลังใช้หัวข้อไปแล้ว — ระยะเดียวกับคลัง tutorial เพราะ
// เหตุผลเดียวกัน: คนดูจำได้ว่าเพิ่งเห็นเรื่องนี้ และการพักต้องหมดอายุเสมอ
const MythParkDays = 14

// MythMinPool คือจำนวนแถวที่ต้องเหลือใช้ได้เป็นอย่างน้อย ถ้า park แถวถัดไปจะทำให้
// เหลือน้อยกว่านี้ ให้ข้ามการ park (ยังบวก used_count) — คลิปซ้ำหัวข้อยังดีกว่า
// ไม่มีคลิปเลย และคลังตอนเริ่มมีแถวที่ยืนยันแล้วเพียง 6 แถว
const MythMinPool = 3

// mythAvailableWhere นิยาม "แถวที่หยิบมาทำคลิปได้ตอนนี้" ที่เดียว เพื่อให้ตัวเลือก
// กับตัวนับพื้นมองคลังชุดเดียวกันเสมอ
const mythAvailableWhere = `enabled = TRUE AND needs_verify = FALSE
	AND (parked_until IS NULL OR parked_until <= NOW())`

const mythPickSQL = `SELECT ` + mythBeliefCols + `, used_count
	FROM myth_beliefs
	WHERE ` + mythAvailableWhere + `
	ORDER BY belief_key`

// mythPickFallbackSQL ใช้เมื่อทุกแถวติด park — หยิบแถวที่พักนานที่สุด แทนการคืน nil
// ประตูทางเดียวเคยทำให้รอบผลิตคืน 0 คลิปเงียบๆ มาแล้วสองครั้งในระบบนี้
const mythPickFallbackSQL = `SELECT ` + mythBeliefCols + `, used_count
	FROM myth_beliefs
	WHERE enabled = TRUE AND needs_verify = FALSE
	ORDER BY parked_until NULLS FIRST, used_count
	LIMIT 1`

// mythMarkUsedSQL บวกตัวนับแล้ว park แถวนี้ 14 วัน — แต่ park เฉพาะเมื่อคลังยังเหลือ
// แถวใช้ได้มากกว่าพื้น การนับอยู่ใน statement เดียวกันโดยตั้งใจ: แยกนับแล้วค่อย
// UPDATE จะทำให้สองรอบที่ชนกันเห็น "เหลือแถวสุดท้าย" พร้อมกันแล้ว park ทิ้งทั้งคู่
const mythMarkUsedSQL = `
	UPDATE myth_beliefs
	SET used_count   = used_count + 1,
	    last_used_at = NOW(),
	    parked_until = CASE
	        WHEN (SELECT COUNT(*) FROM myth_beliefs m2
	              WHERE m2.enabled = TRUE AND m2.needs_verify = FALSE
	                AND (m2.parked_until IS NULL OR m2.parked_until <= NOW())
	                AND m2.id <> myth_beliefs.id) >= $2
	        THEN NOW() + make_interval(days => $3)
	        ELSE parked_until
	    END
	WHERE id = $1`

type mythUsage struct {
	Belief    models.MythBelief
	UsedCount int
}

// pickMythLeastUsed เลือกแถวที่ใช้น้อยสุด ตัดสินเสมอด้วย weight สูงกว่า แล้วจึงด้วย
// belief_key เพื่อให้ผลลัพธ์เสถียร (เทสต์ได้โดยไม่ต้องมีฐานข้อมูล)
func pickMythLeastUsed(usages []mythUsage) models.MythBelief {
	best := usages[0]
	for _, u := range usages[1:] {
		switch {
		case u.UsedCount < best.UsedCount:
			best = u
		case u.UsedCount == best.UsedCount && u.Belief.Weight > best.Belief.Weight:
			best = u
		}
	}
	return best.Belief
}

func scanMythUsage(scan func(dest ...any) error) (mythUsage, error) {
	var u mythUsage
	b := &u.Belief
	err := scan(&b.ID, &b.BeliefKey, &b.BeliefTH, &b.WhyBelievedTH, &b.Verdict, &b.FactTH,
		&b.SourceLabel, &b.SourceURL, &b.NuanceTH, &b.CostTH, &b.Audience, &b.Weight, &b.Enabled,
		&u.UsedCount)
	return u, err
}

// PickNext คืนแถวที่ควรทำคลิปรอบนี้ · nil = คลังไม่มีแถวที่ยืนยันแล้วเลย
// แถวที่ verdict เพี้ยน (ไม่อยู่ใน 3 ค่าที่รู้จัก) ถูกข้าม เพราะมันจะเรนเดอร์เป็น
// มิเตอร์เปล่าในคลิปที่เผยแพร่แล้ว
func (r *MythBeliefsRepo) PickNext(ctx context.Context) (*models.MythBelief, error) {
	rows, err := r.pool.Query(ctx, mythPickSQL)
	if err != nil {
		return nil, fmt.Errorf("query myth beliefs: %w", err)
	}
	defer rows.Close()

	usages := []mythUsage{}
	for rows.Next() {
		u, sErr := scanMythUsage(rows.Scan)
		if sErr != nil {
			return nil, fmt.Errorf("scan myth belief: %w", sErr)
		}
		if !models.ValidMythVerdict(u.Belief.Verdict) {
			continue
		}
		usages = append(usages, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(usages) > 0 {
		picked := pickMythLeastUsed(usages)
		return &picked, nil
	}

	// ทุกแถวติด park — หยิบแถวที่พักนานสุดแทนการคืน nil
	u, err := scanMythUsage(r.pool.QueryRow(ctx, mythPickFallbackSQL).Scan)
	if err != nil {
		return nil, nil // คลังว่างจริง ให้ผู้เรียกรายงานเป็น error ที่อ่านออก
	}
	if !models.ValidMythVerdict(u.Belief.Verdict) {
		return nil, nil
	}
	return &u.Belief, nil
}

// GetByKey ใช้โดยเส้นทาง retry ที่มีแต่ clips.myth_belief ให้ไปต่อ
func (r *MythBeliefsRepo) GetByKey(ctx context.Context, key string) (*models.MythBelief, error) {
	var b models.MythBelief
	err := r.pool.QueryRow(ctx,
		`SELECT `+mythBeliefCols+` FROM myth_beliefs WHERE belief_key = $1`, key).
		Scan(&b.ID, &b.BeliefKey, &b.BeliefTH, &b.WhyBelievedTH, &b.Verdict, &b.FactTH,
			&b.SourceLabel, &b.SourceURL, &b.NuanceTH, &b.CostTH, &b.Audience, &b.Weight, &b.Enabled)
	if err != nil {
		return nil, fmt.Errorf("get myth belief %s: %w", key, err)
	}
	return &b, nil
}

// MarkUsed บวกตัวนับและพักแถวนี้ เรียกหลังคลิปผลิตสำเร็จเท่านั้น
func (r *MythBeliefsRepo) MarkUsed(ctx context.Context, id string) error {
	if _, err := r.pool.Exec(ctx, mythMarkUsedSQL, id, MythMinPool, MythParkDays); err != nil {
		return fmt.Errorf("mark myth belief used: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: รันเทสต์ให้ผ่าน**

Run: `go test ./internal/repository/ -run "TestMyth|TestPickMyth" -v`
Expected: PASS ทั้ง 4 เทสต์

- [ ] **Step 5: commit**

```bash
git add internal/repository/myth_beliefs.go internal/repository/myth_beliefs_test.go
git commit -m "feat(myth): repository คลังความเชื่อผิด + พื้นกันคลังยุบใน UPDATE เดียว"
```

---

### Task 3: prompt block + ตะแกรงข้อเท็จจริง (pure functions)

**Files:**
- Create: `internal/agent/myth.go`
- Test: `internal/agent/myth_test.go`

**Interfaces:**
- Consumes: `models.MythBelief` (Task 1), `agent.GeneratedScene`, `agent.ClampLayout`
- Produces:
  - `agent.MythBrief(b *models.MythBelief) string` — คืน "" เมื่อ nil
  - `agent.FactNumberViolations(scenes []GeneratedScene, b *models.MythBelief) []string`
  - `agent.DisallowedClaimViolations(scenes []GeneratedScene, b *models.MythBelief) []string`

- [ ] **Step 1: เขียนเทสต์ที่ล้มก่อน**

`internal/agent/myth_test.go`:
```go
package agent

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/models"
)

func mythFixture() *models.MythBelief {
	return &models.MythBelief{
		BeliefKey:     "bm_stronger_than_personal",
		BeliefTH:      "เปิด BM แล้วบัญชีแข็งกว่าใช้บัญชีส่วนตัว",
		WhyBelievedTH: "เพราะเอเจนซีทุกที่ใช้ BM",
		Verdict:       models.MythVerdictHalfTrue,
		FactTH:        "BM ให้ความเป็นเจ้าของสินทรัพย์และสิทธิ์ทีม ไม่ได้ลดโอกาสถูกแบน",
		SourceLabel:   "Jon Loomer",
		NuanceTH:      "ที่จริงคือเรื่องความเป็นเจ้าของเพจและ pixel",
		CostTH:        "ย้ายบัญชีเสียเวลา 3 วันโดยไม่ได้อะไรเพิ่ม",
	}
}

func sceneWithVoice(n int, layout, voice, onScreen string) GeneratedScene {
	return GeneratedScene{SceneNumber: n, Layout: layout, VoiceText: voice, OnScreenText: onScreen,
		Content: json.RawMessage(`{}`)}
}

// ตัวเลขข้อเท็จจริงที่ไม่มีในแถวคลังคือสิ่งที่โมเดลแต่งขึ้น — ต้องจับได้ ไม่งั้นคลิป
// จะเผยแพร่ตัวเลขที่ไม่มีใครยืนยัน
func TestFactNumberViolationsCatchesInventedNumber(t *testing.T) {
	scenes := []GeneratedScene{
		sceneWithVoice(1, "hook", "เสียเงินฟรี 25000 บาททุกเดือน", "เสียเงินฟรี"),
	}
	v := FactNumberViolations(scenes, mythFixture())
	if len(v) == 0 {
		t.Fatal("FactNumberViolations = ว่าง แต่ 25000 ไม่มีในแถวคลัง")
	}
	if !strings.Contains(v[0], "25000") {
		t.Errorf("ข้อความตำหนิควรบอกตัวเลขที่ผิด ได้ %q", v[0])
	}
}

// ตัวเลขที่มาจากแถวคลังต้องผ่าน ไม่งั้นตะแกรงจะตีคลิปที่ถูกต้องตกทุกคืน
func TestFactNumberViolationsAllowsCatalogNumber(t *testing.T) {
	scenes := []GeneratedScene{
		sceneWithVoice(1, "hook", "ย้ายบัญชีเสียเวลา 3 วันโดยเปล่าประโยชน์", "เสียเวลา 3 วัน"),
	}
	if v := FactNumberViolations(scenes, mythFixture()); len(v) > 0 {
		t.Errorf("FactNumberViolations = %v แต่ 3 มาจาก cost_th", v)
	}
}

// เลขลำดับขั้น/ข้อ ไม่ใช่ตัวเลขข้อเท็จจริง ตะแกรงต้องไม่จับ
func TestFactNumberViolationsIgnoresOrdinals(t *testing.T) {
	scenes := []GeneratedScene{
		sceneWithVoice(1, "proof", "ข้อ 2 ที่ต้องจำ", "ข้อ 2"),
	}
	if v := FactNumberViolations(scenes, mythFixture()); len(v) > 0 {
		t.Errorf("FactNumberViolations = %v แต่ \"ข้อ 2\" เป็นเลขลำดับ", v)
	}
}

// คลิป myth ที่ไม่มีแถวคลัง (nil) = คลิปโหมดอื่น ตะแกรงต้องไม่ทำงาน
func TestFactNumberViolationsNilBelief(t *testing.T) {
	if v := FactNumberViolations([]GeneratedScene{sceneWithVoice(1, "hook", "999", "")}, nil); v != nil {
		t.Errorf("FactNumberViolations(nil) = %v ต้องเป็น nil", v)
	}
}

// คำอ้างลอย (trust score/tier/HiVA) ใช้ได้เฉพาะแถวที่มี source_url จริง
func TestDisallowedClaimViolations(t *testing.T) {
	b := mythFixture() // SourceURL ว่าง
	scenes := []GeneratedScene{sceneWithVoice(1, "proof", "บัญชีคุณอยู่ tier 2", "tier 2")}
	if v := DisallowedClaimViolations(scenes, b); len(v) == 0 {
		t.Fatal("DisallowedClaimViolations = ว่าง แต่พูด tier โดยไม่มี source_url")
	}
	b.SourceURL = "https://www.facebook.com/business/help/xxxx"
	if v := DisallowedClaimViolations(scenes, b); len(v) > 0 {
		t.Errorf("DisallowedClaimViolations = %v แต่แถวนี้มี source_url แล้ว", v)
	}
}

// brief ต้องขนข้อมูลครบทุกฟิลด์ที่ซีนต้องใช้ ขาดฟิลด์ใดฟิลด์หนึ่ง = ซีนนั้นว่าง
func TestMythBriefCarriesEveryField(t *testing.T) {
	got := MythBrief(mythFixture())
	for _, want := range []string{
		"เปิด BM แล้วบัญชีแข็งกว่าใช้บัญชีส่วนตัว",
		"เพราะเอเจนซีทุกที่ใช้ BM",
		"BM ให้ความเป็นเจ้าของสินทรัพย์",
		"Jon Loomer",
		"ที่จริงคือเรื่องความเป็นเจ้าของเพจ",
		"ย้ายบัญชีเสียเวลา 3 วัน",
		"จริงครึ่งเดียว",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("MythBrief ขาด %q", want)
		}
	}
	if MythBrief(nil) != "" {
		t.Error("MythBrief(nil) ต้องเป็น \"\"")
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่าล้ม**

Run: `go test ./internal/agent/ -run "TestFactNumber|TestDisallowed|TestMythBrief" -v`
Expected: FAIL — `undefined: FactNumberViolations`

- [ ] **Step 3: เขียน implementation**

`internal/agent/myth.go`:
```go
package agent

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/jaochai/video-fb/internal/models"
)

// mythVerdictLabelTH แปลง verdict ของแถวคลังเป็นคำไทยที่ทั้ง prompt และตรายางบนจอ
// ใช้ร่วมกัน — ที่เดียว เพื่อให้สิ่งที่ agent อ่านตรงกับสิ่งที่คนดูเห็น
func mythVerdictLabelTH(v string) string {
	switch v {
	case models.MythVerdictFalse:
		return "ไม่จริง"
	case models.MythVerdictHalfTrue:
		return "จริงครึ่งเดียว"
	case models.MythVerdictOutdated:
		return "เคยจริง วันนี้ไม่จริงแล้ว"
	}
	return ""
}

// MythVerdictLabelTH เปิดคำแปลให้แพ็กเกจอื่นใช้ (producer เอาไปเป็นข้อความตรายาง)
func MythVerdictLabelTH(v string) string { return mythVerdictLabelTH(v) }

// MythBrief คือบล็อกข้อมูลที่ script/scene agent ได้เห็น — ข้อเท็จจริงทุกตัวที่คลิป
// พูดได้ต้องอยู่ในบล็อกนี้ ไม่มีทางอื่น เพราะตะแกรงฝั่ง Go ตรวจตรงนี้ตรงๆ
func MythBrief(b *models.MythBelief) string {
	if b == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## ข้อมูลความเชื่อที่ต้องหักล้าง (ใช้ได้เฉพาะข้อมูลในบล็อกนี้ ห้ามเพิ่มตัวเลขหรือข้อเท็จจริงเอง)\n")
	sb.WriteString("ความเชื่อที่ยกมา: " + b.BeliefTH + "\n")
	sb.WriteString("ทำไมคนถึงเชื่อ (ใช้เล่าให้คนดูไม่รู้สึกถูกดูถูก): " + b.WhyBelievedTH + "\n")
	sb.WriteString("คำตัดสิน: " + mythVerdictLabelTH(b.Verdict) + "\n")
	sb.WriteString("ข้อเท็จจริงที่หักล้าง: " + b.FactTH + "\n")
	sb.WriteString("แหล่งอ้าง (แสดงบนจอ ห้ามแต่งชื่อแหล่งเอง): " + b.SourceLabel + "\n")
	sb.WriteString("ส่วนที่จริงของความเชื่อนี้ (ต้องพูดถึงเสมอ): " + b.NuanceTH + "\n")
	sb.WriteString("ความเสียหายถ้าเชื่อผิด (ใช้เป็นวัตถุดิบของ hook): " + b.CostTH + "\n")
	sb.WriteString("\nกฎเหล็ก: ตัวเลขทุกตัวที่พูดหรือขึ้นจอต้องปรากฏในบล็อกนี้ " +
		"ห้ามอ้างคะแนนความน่าเชื่อถือ ระดับบัญชี หรือเพดานงบที่บล็อกนี้ไม่ได้ระบุ\n")
	return sb.String()
}

// factNumberRe จับตัวเลข (มีจุดทศนิยม/คอมมาได้) ที่เป็นตัวเลขข้อเท็จจริง
var factNumberRe = regexp.MustCompile(`\d[\d,\.]*`)

// ordinalPrefixes คือคำที่อยู่หน้าตัวเลขแล้วทำให้เลขนั้นเป็นลำดับ ไม่ใช่ข้อเท็จจริง
// (สเปก §7.2) — "ข้อ 2" / "ขั้นที่ 3" ไม่ต้องมีในแถวคลัง
var ordinalPrefixes = []string{"ข้อ", "ขั้นที่", "ขั้น", "อย่างที่", "ที่", "แบบที่", "ข้อที่"}

// normalizeNumber ตัดคอมมาและศูนย์ท้ายทศนิยมออก เพื่อให้ "40,000" กับ "40000"
// นับเป็นตัวเลขเดียวกัน
func normalizeNumber(s string) string {
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSuffix(s, ".")
	return s
}

// catalogNumbers รวมตัวเลขทุกตัวที่แถวคลังอนุญาต
func catalogNumbers(b *models.MythBelief) map[string]bool {
	allowed := map[string]bool{}
	for _, field := range []string{b.FactTH, b.NuanceTH, b.CostTH, b.BeliefTH, b.WhyBelievedTH} {
		for _, m := range factNumberRe.FindAllString(field, -1) {
			allowed[normalizeNumber(m)] = true
		}
	}
	return allowed
}

// isOrdinalAt บอกว่าตัวเลขที่ตำแหน่ง idx ถูกนำหน้าด้วยคำบอกลำดับหรือไม่
func isOrdinalAt(text string, idx int) bool {
	head := strings.TrimSpace(text[:idx])
	for _, p := range ordinalPrefixes {
		if strings.HasSuffix(head, p) {
			return true
		}
	}
	return false
}

// FactNumberViolations คือครึ่ง deterministic ของตะแกรงข้อเท็จจริง: คืนรายการตำหนิ
// เมื่อซีนพูดตัวเลขที่แถวคลังไม่ได้ให้มา คืน nil เมื่อ b == nil (คลิปโหมดอื่น)
//
// ตรวจทั้ง voice_text (สิ่งที่คนดูได้ยิน) และ on_screen_text (สิ่งที่คนดูเห็น)
// เพราะตัวเลขที่แต่งขึ้นเสียหายเท่ากันทั้งสองทาง
func FactNumberViolations(scenes []GeneratedScene, b *models.MythBelief) []string {
	if b == nil {
		return nil
	}
	allowed := catalogNumbers(b)

	var out []string
	for _, s := range scenes {
		for field, text := range map[string]string{"voice_text": s.VoiceText, "on_screen_text": s.OnScreenText} {
			for _, loc := range factNumberRe.FindAllStringIndex(text, -1) {
				num := text[loc[0]:loc[1]]
				if isOrdinalAt(text, loc[0]) || allowed[normalizeNumber(num)] {
					continue
				}
				out = append(out, fmt.Sprintf("scene %d %s: ตัวเลข %q ไม่มีในแถวคลัง %s",
					s.SceneNumber, field, num, b.BeliefKey))
			}
		}
	}
	return out
}

// unsourcedClaimTerms คือคำที่อ้างระบบคะแนน/ระดับบัญชีของ Meta ซึ่งไม่มีเอกสาร
// ทางการรองรับ — พูดได้เฉพาะแถวที่มี source_url จริง
var unsourcedClaimTerms = []string{"trust score", "trustscore", "tier", "เทียร์", "hiva", "คะแนนความน่าเชื่อถือ"}

// DisallowedClaimViolations คืนรายการตำหนิเมื่อซีนอ้างระบบคะแนน/ระดับบัญชีขณะที่
// แถวคลังไม่มี source_url — วันนี้ยังไม่มีแถวไหนมี จึงเท่ากับห้ามทั้งหมด
func DisallowedClaimViolations(scenes []GeneratedScene, b *models.MythBelief) []string {
	if b == nil || strings.TrimSpace(b.SourceURL) != "" {
		return nil
	}
	var out []string
	for _, s := range scenes {
		hay := strings.ToLower(s.VoiceText + " " + s.OnScreenText)
		for _, term := range unsourcedClaimTerms {
			if strings.Contains(hay, term) {
				out = append(out, fmt.Sprintf("scene %d: อ้าง %q แต่แถวคลัง %s ไม่มี source_url",
					s.SceneNumber, term, b.BeliefKey))
			}
		}
	}
	return out
}
```

- [ ] **Step 4: รันเทสต์ให้ผ่าน**

Run: `go test ./internal/agent/ -run "TestFactNumber|TestDisallowed|TestMythBrief" -v`
Expected: PASS ทั้ง 6 เทสต์

- [ ] **Step 5: เพิ่ม layout ใหม่ในรายการที่อนุญาต**

`internal/agent/scene_content.go` — ใน `sceneLayouts` เพิ่มบรรทัด:
```go
	// myth format (spec 2026-07-30): การ์ดคำเชื่อ + การ์ดหลักฐาน
	"belief": true, "proof": true,
```

- [ ] **Step 6: รันเทสต์ทั้งแพ็กเกจ**

Run: `go test ./internal/agent/ -v`
Expected: PASS ทั้งหมด (เทสต์เดิมไม่พัง)

- [ ] **Step 7: commit**

```bash
git add internal/agent/myth.go internal/agent/myth_test.go internal/agent/scene_content.go
git commit -m "feat(myth): prompt brief + ตะแกรงตัวเลข/คำอ้างลอย + layout belief/proof"
```

---

### Task 4: preset + mode ของโหมด myth

**Files:**
- Create: `internal/producer/myth_format.go`
- Modify: `internal/producer/case_format.go:47-61` (`promptForScene`), `:98-120` (`imageScenesForMode`), `:66-73` (const block)
- Test: `internal/producer/myth_format_test.go`

**Interfaces:**
- Consumes: `producer.StylePreset`, `producer.Brand`, `producer.buildImagePromptCore`
- Produces: `producer.MythPreset` (Key `"factcheck"`), `producer.ModeMyth = "myth"`, `producer.buildMythCoverPrompt`

- [ ] **Step 1: เขียนเทสต์ที่ล้มก่อน**

`internal/producer/myth_format_test.go`:
```go
package producer

import (
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

// preset ของ myth ต้องหาได้จาก key ที่เก็บในคลิป ไม่งั้นคลิปเก่าที่ retry จะกลับมา
// เป็นแฟ้มคดี (ค่า fallback) แล้ว CSS ทั้งชุดไม่ทำงาน
func TestPresetByKeyResolvesMyth(t *testing.T) {
	if got := PresetByKey(MythPreset.Key); got.Key != "factcheck" {
		t.Errorf("PresetByKey(%q).Key = %q", MythPreset.Key, got.Key)
	}
}

// ภาพปกของ myth ต้องเป็นโต๊ะตรวจเอกสารกลางวัน ไม่ใช่โต๊ะนักสืบกลางคืนของแฟ้มคดี —
// ถ้าซ้ำกัน ช่องจะมีคลิปสองแบบที่หน้าตาเหมือนกันคนละชื่อ
func TestMythImageAnchorIsDaylightDesk(t *testing.T) {
	a := strings.ToLower(MythPreset.ImageAnchor)
	for _, want := range []string{"daylight", "magnifier"} {
		if !strings.Contains(a, want) {
			t.Errorf("MythPreset.ImageAnchor ขาดคำ %q", want)
		}
	}
	if strings.Contains(a, "at night") {
		t.Error("MythPreset.ImageAnchor ไม่ควรเป็นภาพกลางคืน (ชนกับ case-file/warroom)")
	}
}

// โหมด myth ได้ภาพ AI ใบเดียวเท่านั้น (ซีนปก) ซีนที่เหลือเป็น CSS — ภาพในซีน
// การ์ดจะแย่งความสนใจจากข้อความที่คนดูต้องอ่าน
func TestMythAllowsOneImageScene(t *testing.T) {
	scenes := []agent.GeneratedScene{
		{SceneNumber: 1, Layout: "hook", ImagePrompt: "a desk"},
		{SceneNumber: 2, Layout: "belief", ImagePrompt: "another"},
		{SceneNumber: 3, Layout: "proof", ImagePrompt: "third"},
	}
	allowed := imageScenesForMode(scenes, ModeMyth)
	if len(allowed) != 1 || !allowed[1] {
		t.Errorf("imageScenesForMode(myth) = %v ต้องอนุญาตแค่ซีน 1", allowed)
	}
}

// prompt ของซีนปก myth ต้องกันพื้นที่ครึ่งล่างให้การ์ด ไม่งั้นการ์ดทับหน้าคนในภาพ
func TestMythCoverPromptReservesLowerHalf(t *testing.T) {
	got := buildMythCoverPrompt("a desk with documents", MythPreset, "tok")
	if !strings.Contains(strings.ToLower(got), "upper half") {
		t.Errorf("buildMythCoverPrompt ไม่ได้สั่งวางวัตถุครึ่งบน: %q", got)
	}
}

// promptForScene ต้องรู้จักโหมด myth ไม่งั้นซีนปกจะได้ prompt แบบ classic
// ที่ไม่กันพื้นที่ให้การ์ดเลย
func TestPromptForSceneRoutesMyth(t *testing.T) {
	s := agent.GeneratedScene{SceneNumber: 1, Layout: "hook", ImagePrompt: "a desk"}
	got := promptForScene(s, MythPreset, "tok", ModeMyth)
	if !strings.Contains(strings.ToLower(got), "upper half") {
		t.Errorf("promptForScene(myth) ไม่ได้ใช้ cover prompt: %q", got)
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่าล้ม**

Run: `go test ./internal/producer/ -run TestMyth -v`
Expected: FAIL — `undefined: MythPreset`, `undefined: ModeMyth`

- [ ] **Step 3: เขียน preset**

`internal/producer/myth_format.go`:
```go
package producer

// MythPreset คือหน้าตาของโหมดจับความเชื่อผิด: โต๊ะตรวจเอกสารกลางวัน ไม่ใช่โต๊ะ
// นักสืบกลางคืนของแฟ้มคดีและไม่ใช่จอมอนิเตอร์ของห้องควบคุม — สามโหมดนี้ต้องแยกออก
// จากกันด้วยสายตาภายในวินาทีแรก palette ยังเป็น Brand (navy+orange) เหมือนทุกโหมด
// เพราะช่องต้องอ่านเป็นแบรนด์เดียว
var MythPreset = StylePreset{
	Key:         "factcheck",
	DisplayName: "Fact-Check Lab",
	Palette:     Brand,
	ImageAnchor: "Cinematic editorial PHOTOGRAPHY, shot on 50mm, a clean review desk in DAYLIGHT: " +
		"documents and a printed report under a bright desk lamp, a magnifier and a pen resting on top, " +
		"deep-navy #0047AF accents in the room, one warm amber #F0A030 highlight. " +
		"Photorealistic, calm, methodical, premium. NO illustration, NO 3D render, NO cartoon, " +
		"no text, no logos, no readable document content. " +
		"Atmosphere: someone is about to check whether what everyone repeats is actually true.",
	Font:        TypeTokens{Family: "Sarabun", HeadingFamily: "Kanit"},
	HeadingFont: TypeTokens{Family: "Sarabun", HeadingFamily: "Kanit"},
	// ตรายางกระแทกลงการ์ด = entrance สั้นและแข็ง ไม่ใช่การไถลแบบภาพยนตร์
	Motion: MotionProfile{EntranceDur: 0.24, EntranceEase: "power4.out", BGZoomTo: 1.02},
}

// buildMythCoverPrompt เขียน prompt ของภาพ AI ใบเดียวที่คลิป myth ได้: ซีนเปิด
// วัตถุหลักอยู่ครึ่งบน เพราะการ์ดคำเชื่อกินครึ่งล่างของเฟรม
func buildMythCoverPrompt(concept string, preset StylePreset, clipToken string) string {
	return buildImagePromptCore(concept,
		"cinematic high-angle desk scene in daylight, the key subject placed in the UPPER half of the frame, lower half clean and uncluttered",
		preset, clipToken)
}
```

- [ ] **Step 4: ต่อสายโหมดใน case_format.go**

`internal/producer/case_format.go` — ใน const block เพิ่ม:
```go
	ModeMyth     = "myth"
```

ใน `promptForScene` เพิ่ม case ก่อน `default`:
```go
	case ModeMyth:
		return buildMythCoverPrompt(s.ImagePrompt, preset, clipToken)
```

ใน `imageScenesForMode` เปลี่ยน case ที่รวมโหมดภาพเดียวให้รวม myth ด้วย:
```go
	case ModeTutorial, ModeChat, ModeWarRoom, ModeMyth:
```

- [ ] **Step 5: ต่อ PresetByKey**

`internal/producer/presets.go` — ใน `PresetByKey` เพิ่ม case:
```go
	case MythPreset.Key:
		return MythPreset
```

- [ ] **Step 6: รันเทสต์ให้ผ่าน**

Run: `go test ./internal/producer/ -run TestMyth -v && go test ./internal/producer/ -run TestPromptForScene -v`
Expected: PASS ทั้งหมด

- [ ] **Step 7: commit**

```bash
git add internal/producer/myth_format.go internal/producer/myth_format_test.go \
        internal/producer/case_format.go internal/producer/presets.go
git commit -m "feat(myth): MythPreset (Fact-Check Lab) + ModeMyth + นโยบายภาพใบเดียว"
```

---

### Task 5: ซีนบนจอ — ฟิลด์ Go-injected + CSS + builder + render test

**Files:**
- Modify: `internal/producer/composition_types.go:40-83` (`SceneContent`), `:184-189` (`ScenesParams`)
- Modify: `internal/producer/composition.go:100-112` (จุด inject)
- Modify: `internal/producer/templates/layout_multi_scene.html.tmpl` (CSS block ~บรรทัด 246 ก่อน war-room + builder ~บรรทัด 640 + `FORMAT_MYTH` ~บรรทัด 403 + reuseCover ~บรรทัด 427)
- Test: `internal/producer/composition_myth_render_test.go`

**Interfaces:**
- Consumes: `MythPreset`, `ModeMyth` (Task 4), `agent.MythVerdictLabelTH` (Task 3)
- Produces: `SceneContent.Meter`, `SceneContent.Source`, `ScenesParams.MythVerdict`, `ScenesParams.MythSource`

- [ ] **Step 1: เขียนเทสต์ที่ล้มก่อน**

`internal/producer/composition_myth_render_test.go`:
```go
package producer

import (
	"strings"
	"testing"
)

func mythParams() ScenesParams {
	mk := func(n int, layout string, c SceneContent) SceneSpec {
		c.SceneNumber, c.Layout = n, layout
		c.Start, c.End = float64(n-1)*5, float64(n)*5
		return SceneSpec{SceneNumber: n, StartSec: c.Start, EndSec: c.End,
			LayoutVariant: "hook_big", CaptionStyle: "phrase_block", Content: c}
	}
	return ScenesParams{
		AspectRatio: "9:16", BrandName: "ADS VANCE", VoiceSrc: "assets/voice.wav",
		DurationSeconds: 30, Format: "myth", ThemeKey: "factcheck",
		MythVerdict: "half_true", MythSource: "Jon Loomer",
		Scenes: []SceneSpec{
			mk(1, "hook", SceneContent{Rows: []ContentRow{{Text: "ย้าย BM เสียเวลา 3 วัน"}},
				BackgroundImage: "assets/bg-scene1.png"}),
			mk(2, "belief", SceneContent{Kicker: "ที่เชื่อกัน",
				Title: "เปิด BM แล้วบัญชีแข็งกว่าบัญชีส่วนตัว",
				Sub:   "เพราะเอเจนซีทุกที่ใช้ BM"}),
			mk(3, "verdict", SceneContent{Title: "ไม่ได้แข็งกว่า",
				Stamp: "โมเดลแต่งเอง", Meter: "โมเดลแต่งเอง"}),
			mk(4, "proof", SceneContent{Title: "BM ให้ความเป็นเจ้าของ ไม่ใช่ภูมิคุ้มกัน",
				Rows:   []ContentRow{{Text: "สิทธิ์ทีมและ pixel อยู่กับธุรกิจ"}},
				Source: "โมเดลแต่งเอง"}),
			mk(5, "tip", SceneContent{Title: "ส่วนที่จริงคือ", Rows: []ContentRow{{Text: "ความเป็นเจ้าของสินทรัพย์"}}}),
			mk(6, "cta", SceneContent{Title: "ปิดท้าย", CTA: "ทักมาเช็ค", Brand: "ADS VANCE"}),
		},
		Segments: []TranscriptSegment{{Text: "เปิด BM", Start: 0, End: 2}},
	}
}

func TestRenderMythFormat(t *testing.T) {
	out, err := RenderCompositionScenes(mythParams())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)
	for _, want := range []string{
		`data-format="myth"`,
		"const FORMAT_MYTH = true",
		`data-layout="belief"`,
		`data-layout="proof"`,
		"ที่เชื่อกัน",
		"เปิด BM แล้วบัญชีแข็งกว่าบัญชีส่วนตัว",
		"เพราะเอเจนซีทุกที่ใช้ BM",
		"BM ให้ความเป็นเจ้าของ ไม่ใช่ภูมิคุ้มกัน",
		".mb-card",   // CSS ของการ์ดต้องอยู่ในไฟล์ที่ส่งออก
		".mb-meter",  // มิเตอร์ 3 ระดับ
	} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML โหมด myth ขาด %q", want)
		}
	}
}

// หัวใจของสเปก: คำตัดสินและแหล่งอ้างต้องมาจาก DB ไม่ใช่จาก LLM ค่าที่โมเดลส่งมา
// ต้องถูกเขียนทับทุกครั้ง — ถ้าข้อนี้พัง คลิปจะตัดสินความเชื่อเองได้
func TestMythVerdictAndSourceAreGoInjected(t *testing.T) {
	out, err := RenderCompositionScenes(mythParams())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)
	if strings.Contains(html, "โมเดลแต่งเอง") {
		t.Error("ค่าที่ LLM ส่งมายังอยู่ในผลลัพธ์ — Go ต้องเขียนทับ Meter/Source/Stamp")
	}
	for _, want := range []string{`"meter":"half_true"`, "จริงครึ่งเดียว", "Jon Loomer"} {
		if !strings.Contains(html, want) {
			t.Errorf("ผลลัพธ์ขาดค่าที่ Go ต้องฉีดเข้าไป: %q", want)
		}
	}
}

// CSS ของ myth ต้องไม่ไปแตะ verdict ของแฟ้มคดี — สอง format ใช้ layout ชื่อเดียวกัน
func TestMythCSSIsScoped(t *testing.T) {
	out, _ := RenderCompositionScenes(mythParams())
	html := string(out)
	idx := strings.Index(html, ".mb-card")
	if idx < 0 {
		t.Fatal("ไม่พบ .mb-card")
	}
	block := html[max0(idx-400):idx]
	if !strings.Contains(block, "[data-format='myth']") {
		t.Error("บล็อก CSS ของ myth ต้อง scope ด้วย [data-format='myth']")
	}
}

func max0(i int) int {
	if i < 0 {
		return 0
	}
	return i
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่าล้ม**

Run: `go test ./internal/producer/ -run "TestRenderMyth|TestMythVerdict|TestMythCSS" -v`
Expected: FAIL — `unknown field Meter in struct literal`

- [ ] **Step 3: เพิ่มฟิลด์ใน types**

`internal/producer/composition_types.go` — ใน `SceneContent` ต่อท้ายบล็อก chat format:
```go
	// myth format (spec 2026-07-30). ทั้งสองค่าเป็น Go-injected เหมือน CaseNo:
	// Meter คือ verdict จากแถวคลัง (false|half_true|outdated) และ Source คือชื่อ
	// แหล่งอ้าง — LLM ตัดสินความเชื่อหรือแต่งชื่อแหล่งเองไม่ได้
	Meter  string `json:"meter,omitempty"`
	Source string `json:"source,omitempty"`
```

ใน `ScenesParams` ต่อท้าย `CaseNumber`:
```go
	// myth format (spec 2026-07-30). MythVerdict/MythSource มาจากแถวคลังโดยตรง
	// และถูกฉีดทับค่าที่ LLM ส่งมาใน RenderCompositionScenes
	MythVerdict string
	MythSource  string
```

`isEmptyContent` (composition_types.go หรือ scene_adapter.go บรรทัด ~283) ไม่ต้องแก้ — ซีน myth ทุกซีนมี Title/Rows อยู่แล้ว

- [ ] **Step 4: ฉีดค่าใน composition.go**

`internal/producer/composition.go` — ต่อท้ายบล็อก `if p.Format == "case"` ในลูป `for i, sc := range p.Scenes`:
```go
		// โหมด myth: คำตัดสิน + แหล่งอ้างเป็น Go-injected (สเปก §7.1) — เขียนทับค่า
		// ที่ LLM ส่งมาเสมอ ไม่ใช่เติมเมื่อว่าง ตรายางที่โมเดลเขียนเองเท่ากับให้โมเดล
		// ตัดสินความเชื่อแทนคลัง
		if p.Format == "myth" {
			switch contents[i].Layout {
			case "verdict":
				contents[i].Meter = p.MythVerdict
				contents[i].Stamp = agent.MythVerdictLabelTH(p.MythVerdict)
			case "proof":
				contents[i].Source = p.MythSource
			default:
				contents[i].Meter, contents[i].Source = "", ""
			}
		}
```

เพิ่ม import `"github.com/jaochai/video-fb/internal/agent"` ถ้ายังไม่มีในไฟล์ (ตรวจด้วย `head -20 internal/producer/composition.go`) — ถ้าการ import ทำให้เกิด import cycle ให้ย้ายคำแปลมาเป็น map ใน `myth_format.go` แทนแล้วเรียกตัวนั้น

- [ ] **Step 5: เพิ่ม CSS block ใน template**

`internal/producer/templates/layout_multi_scene.html.tmpl` — แทรก **ก่อน** บรรทัดคอมเมนต์ `/* ── war-room format (data-format='warroom') ── */`:
```css
      /* ── myth format (data-format='myth') ──
         การ์ดกระดาษบนโต๊ะตรวจ: พื้นสว่าง ตัวหนังสือเข้ม ตรงข้ามกับซีนอื่นที่เป็น
         ตัวขาวบนน้ำเงิน — คนดูต้องรู้ทันทีว่า "นี่คือของที่กำลังถูกตรวจ"
         Thai-safe: letter-spacing >= 0, line-height >= 1.3, break-word เท่านั้น */
      [data-format='myth'] .scene[data-layout="belief"] .scene-content,
      [data-format='myth'] .scene[data-layout="proof"] .scene-content,
      [data-format='myth'] .scene[data-layout="verdict"] .scene-content{
        top:170px;bottom:410px;justify-content:center}
      [data-format='myth'] .scene[data-layout="belief"] .scene-bg,
      [data-format='myth'] .scene[data-layout="proof"] .scene-bg,
      [data-format='myth'] .scene[data-layout="verdict"] .scene-bg,
      [data-format='myth'] .scene[data-layout="tip"] .scene-bg,
      [data-format='myth'] .scene[data-layout="cta"] .scene-bg{opacity:.26;filter:saturate(.62)}
      [data-format='myth'] .mb-card{position:relative;background:#FBF7EC;color:#141d33;
        border-radius:26px;padding:40px 38px;transform:rotate(-1.1deg);
        box-shadow:0 30px 72px rgba(0,0,0,.52)}
      [data-format='myth'] .mb-kicker{font-weight:800;font-size:30px;line-height:1.34;
        color:#8a6d28;margin-bottom:16px}
      [data-format='myth'] .mb-belief{font-weight:800;font-size:60px;line-height:1.3;
        overflow-wrap:break-word}
      [data-format='myth'] .mb-why{margin-top:22px;font-weight:600;font-size:36px;
        line-height:1.4;color:#41506b;overflow-wrap:break-word}
      [data-format='myth'] .mb-stamp{display:inline-block;margin-top:30px;padding:16px 28px;
        border:6px solid var(--red);border-radius:14px;transform:rotate(-4deg);
        font-weight:800;font-size:44px;line-height:1.3;color:var(--red);
        text-transform:none;opacity:.94}
      [data-format='myth'] .mb-meter{display:flex;gap:12px;margin-top:30px}
      [data-format='myth'] .mb-meter .lv{flex:1;height:20px;border-radius:10px;
        background:rgba(188,210,255,.24)}
      [data-format='myth'] .mb-meter .lv.on{background:var(--red)}
      [data-format='myth'] .mb-meter.half .lv.on{background:var(--amber-bright)}
      [data-format='myth'] .mb-meter-l{display:flex;gap:12px;margin-top:14px;
        font-weight:700;font-size:26px;line-height:1.34;color:var(--muted)}
      [data-format='myth'] .mb-meter-l span{flex:1;text-align:center}
      [data-format='myth'] .mb-fact{font-weight:800;font-size:52px;line-height:1.3;
        color:#141d33;overflow-wrap:break-word}
      [data-format='myth'] .mb-src{margin-top:26px;padding-top:20px;
        border-top:2px solid rgba(20,29,51,.18);font-weight:700;font-size:28px;
        line-height:1.34;color:#5b6b8a}
```

- [ ] **Step 6: เพิ่มค่าคงที่ format ใน JS**

หาบรรทัด `const FORMAT_WARROOM = ...` แล้วเพิ่มใต้มัน:
```
      const FORMAT_MYTH = {{if eq .Format "myth"}}true{{else}}false{{end}};
```

ในนิพจน์ `reuseCover` เพิ่มเงื่อนไข (ซีน myth ที่ไม่มีภาพของตัวเองใช้ภาพปกเป็นฉากหลังจาง):
```
          (FORMAT_MYTH && (sc.type === "belief" || sc.type === "proof" || sc.type === "verdict" || sc.type === "tip" || sc.type === "cta")) ||
```

- [ ] **Step 7: เพิ่มสาขา builder**

หาสาขา `else if(sc.type==="alarm")` แล้วแทรก **ก่อน** สาขา `else if(sc.type==="cta")`:
```js
        else if(sc.type==="belief"){
          const card=el("div","mb-card");
          if(sc.kicker) card.appendChild(el("div","mb-kicker",sc.kicker));
          if(sc.title) card.appendChild(el("h1","mb-belief",sc.title));
          if(sc.sub) card.appendChild(el("div","mb-why",sc.sub));
          c.appendChild(card);
        }
        else if(sc.type==="proof"){
          const card=el("div","mb-card");
          if(sc.title) card.appendChild(el("h1","mb-fact",sc.title));
          if(sc.rows&&sc.rows.length) card.appendChild(rowsBlock(sc.rows));
          if(sc.source) card.appendChild(el("div","mb-src","แหล่งอ้าง: "+sc.source));
          c.appendChild(card);
        }
        else if(FORMAT_MYTH && sc.type==="verdict"){
          const card=el("div","mb-card");
          if(sc.title) card.appendChild(el("h1","mb-fact",sc.title));
          if(sc.stamp) card.appendChild(el("div","mb-stamp",sc.stamp));
          const half=(sc.meter==="half_true");
          const on=(sc.meter==="false")?3:(half?2:1);
          const m=el("div","mb-meter"+(half?" half":""));
          for(let k=1;k<=3;k++){m.appendChild(el("div","lv"+(k<=on?" on":"")));}
          card.appendChild(m);
          const lbl=el("div","mb-meter-l");
          ["เคยจริง","ครึ่งเดียว","ไม่จริง"].forEach(function(t){lbl.appendChild(el("span",null,t));});
          card.appendChild(lbl);
          c.appendChild(card);
        }
```

**ห้ามเขียน `-->` หรือ `{{` ใน JS comment ในบล็อกนี้** — ทำให้ไฟล์ template พังทั้งไฟล์
สาขา `verdict` ของ myth ต้องอยู่ **ก่อน** สาขา `verdict` เดิมของแฟ้มคดี (ถ้ามี) เพื่อไม่ให้แฟ้มคดีเปลี่ยนหน้าตา — ตรวจด้วย `grep -n 'verdict' internal/producer/templates/layout_multi_scene.html.tmpl`

- [ ] **Step 8: รันเทสต์ให้ผ่าน**

Run: `go test ./internal/producer/ -run "TestRenderMyth|TestMythVerdict|TestMythCSS" -v`
Expected: PASS ทั้ง 3 เทสต์

- [ ] **Step 9: ยืนยันว่าโหมดเดิมไม่พัง**

Run: `go test ./internal/producer/ -v`
Expected: เทสต์ render ของ case/chat/warroom/tutorial/scenes ทั้งหมดยังผ่าน

- [ ] **Step 10: commit**

```bash
git add internal/producer/composition_types.go internal/producer/composition.go \
        internal/producer/templates/layout_multi_scene.html.tmpl \
        internal/producer/composition_myth_render_test.go
git commit -m "feat(myth): การ์ดคำเชื่อ/หลักฐาน + ตรายาง + มิเตอร์ 3 ระดับ (Meter/Source เป็น Go-injected)"
```

---

### Task 6: orchestrator — ProduceMyth + ตะแกรงสองสไตรก์ + retry

**Files:**
- Create: `internal/orchestrator/myth.go`
- Modify: `internal/orchestrator/tutorial.go:37-48` (`clipMode`)
- Modify: `internal/orchestrator/orchestrator.go:391,418,493,499,517,933` (ส่ง `myth` ลงไป) + `:989-999` (`presetFor`) + `:1004-1015` (`resolveFormatInfo`) + `:911-922` (retry โหลดคลัง)
- Test: `internal/orchestrator/myth_test.go`

**Interfaces:**
- Consumes: `repository.MythBeliefsRepo` (Task 2), `agent.MythBrief`/`FactNumberViolations`/`DisallowedClaimViolations` (Task 3), `producer.MythPreset`/`ModeMyth` (Task 4)
- Produces:
  - `(*Orchestrator).ProduceMyth(ctx context.Context) error`
  - `orchestrator.mythFormatName = "myth"`
  - `orchestrator.mythGateFailure(scenes []agent.GeneratedScene, b *models.MythBelief) string`
  - `orchestrator.needsMythBelief(contentFormat string) bool`
  - ฟิลด์ `mythBeliefsRepo *repository.MythBeliefsRepo` ใน struct `Orchestrator` + พารามิเตอร์ใน `New...`

- [ ] **Step 1: เขียนเทสต์ที่ล้มก่อน**

`internal/orchestrator/myth_test.go`:
```go
package orchestrator

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/producer"
)

// content_format "myth" ต้องแมปไปโหมดและ preset ของตัวเอง ไม่ใช่ตกไปที่ค่า default
// (แฟ้มคดี) — ถ้าพลาดตรงนี้ คลิปจะเรนเดอร์เป็นแฟ้มคดี กินเลขคดี และ CSS ใหม่
// ไม่ทำงานเลย (บั๊กเดียวกับที่เกือบเกิดกับคลิป basic)
func TestMythFormatMapsToMythPreset(t *testing.T) {
	if got := clipMode(mythFormatName); got != producer.ModeMyth {
		t.Errorf("clipMode(myth) = %q ต้องเป็น %q", got, producer.ModeMyth)
	}
	if got := presetFor(mythFormatName); got.Key != producer.MythPreset.Key {
		t.Errorf("presetFor(myth).Key = %q ต้องเป็น %q", got.Key, producer.MythPreset.Key)
	}
}

// คลิป myth ต้องโหลดแถวคลังกลับมาตอน retry ไม่งั้นตะแกรงข้อเท็จจริงปิดเงียบ
// แล้วคลิปที่แต่งตัวเลขเองจะ auto-publish (retryFull วิ่งทุก 15 นาที)
func TestNeedsMythBelief(t *testing.T) {
	if !needsMythBelief(mythFormatName) {
		t.Error("needsMythBelief(myth) = false ต้องเป็น true")
	}
	for _, f := range []string{"qa", "tips", "tutorial", "basic", "case_story", "news"} {
		if needsMythBelief(f) {
			t.Errorf("needsMythBelief(%q) = true ต้องเป็น false", f)
		}
	}
}

func gateBelief() *models.MythBelief {
	return &models.MythBelief{
		BeliefKey: "k", BeliefTH: "b", WhyBelievedTH: "w",
		Verdict: models.MythVerdictHalfTrue, FactTH: "f", SourceLabel: "s",
		NuanceTH: "n", CostTH: "เสียเวลา 3 วัน",
	}
}

// ตะแกรงต้องจับตัวเลขที่แต่งขึ้นและคำอ้างลอย และต้องปล่อยผ่านคลิปที่สะอาด
func TestMythGateFailure(t *testing.T) {
	clean := []agent.GeneratedScene{{SceneNumber: 1, Layout: "hook",
		VoiceText: "เสียเวลา 3 วันโดยเปล่าประโยชน์", Content: json.RawMessage(`{}`)}}
	if msg := mythGateFailure(clean, gateBelief()); msg != "" {
		t.Errorf("mythGateFailure(สะอาด) = %q ต้องเป็น \"\"", msg)
	}

	invented := []agent.GeneratedScene{{SceneNumber: 1, Layout: "hook",
		VoiceText: "เสียเงิน 25000 บาท", Content: json.RawMessage(`{}`)}}
	if msg := mythGateFailure(invented, gateBelief()); !strings.Contains(msg, "25000") {
		t.Errorf("mythGateFailure(ตัวเลขแต่ง) = %q ต้องบอกตัวเลขที่ผิด", msg)
	}

	claim := []agent.GeneratedScene{{SceneNumber: 2, Layout: "proof",
		VoiceText: "บัญชีคุณอยู่ tier 2", Content: json.RawMessage(`{}`)}}
	if msg := mythGateFailure(claim, gateBelief()); msg == "" {
		t.Error("mythGateFailure(คำอ้างลอย) = \"\" ต้องจับได้")
	}

	if msg := mythGateFailure(invented, nil); msg != "" {
		t.Errorf("mythGateFailure(nil belief) = %q ต้องเป็น \"\" (คลิปโหมดอื่น)", msg)
	}
}

// จำนวนซีนของคลิป myth ต้องคงที่ 6 ซีน ตามโครงในสเปก §6.2 — ไม่ผูกกับจำนวนขั้น
// อย่างคลิปสอน
func TestMythSceneShape(t *testing.T) {
	n, dur := mythSceneShape()
	if n != 6 {
		t.Errorf("mythSceneShape sceneCount = %d ต้องเป็น 6", n)
	}
	if dur < 45 || dur > 90 {
		t.Errorf("mythSceneShape duration = %d วินาที ควรอยู่ในช่วง 45-90", dur)
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่าล้ม**

Run: `go test ./internal/orchestrator/ -run TestMyth -v; go test ./internal/orchestrator/ -run TestNeedsMyth -v`
Expected: FAIL — `undefined: mythFormatName`

- [ ] **Step 3: เขียน orchestrator/myth.go**

```go
package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/jaochai/video-fb/internal/agent"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/producer"
)

// mythFormatName คือแถว content_formats ของคลิปจับความเชื่อผิด (seed ไว้แบบปิด
// เพื่อไม่ให้ตัวสุ่ม format ของรอบ 12:00/18:00 หยิบไปใช้)
const mythFormatName = "myth"

// mythSceneShape คือรูปร่างคลิปตามสเปก §6.2: hook + belief + verdict + proof +
// tip(ส่วนที่จริง) + cta = 6 ซีน คงที่ ไม่ผูกกับข้อมูลในคลังอย่างคลิปสอน
func mythSceneShape() (sceneCount, durationSec int) {
	return 6, 62
}

// needsMythBelief บอกว่าการสร้างคลิปนี้ใหม่ทั้งตัวต้องโหลดแถวคลัง myth ก่อนไหม
//
// คลิป myth auto-publish โดยไม่มีคนตรวจ ถ้า retry ไม่โหลดแถวคลังมาด้วย ตะแกรง
// ข้อเท็จจริงจะไม่มีอะไรเทียบ = ปิดเงียบ แล้วคลิปที่แต่งตัวเลขเองจะขึ้น YouTube
// (บั๊กแบบเดียวกับที่เคยเกิดกับคลิป basic ตอน retryFull)
func needsMythBelief(contentFormat string) bool {
	return clipMode(contentFormat) == producer.ModeMyth
}

// mythGateFailure คือครึ่ง deterministic ของตะแกรงข้อเท็จจริง คืนเหตุผลที่อ่านออก
// เมื่อซีนพูดสิ่งที่แถวคลังไม่ได้ให้มา และคืน "" เมื่อผ่าน (หรือ b == nil = โหมดอื่น)
//
// ตัวนี้ทำงานแทนคนตรวจ: คลิปเผยแพร่เองเมื่อผ่าน จึงเข้มกับสองสิ่งที่คลิปแนวนี้
// พลาดแล้วเสียหายที่สุด — ตัวเลขที่โมเดลแต่งขึ้น และการอ้างระบบคะแนนที่ไม่มีเอกสาร
func mythGateFailure(scenes []agent.GeneratedScene, b *models.MythBelief) string {
	if b == nil {
		return ""
	}
	if v := agent.FactNumberViolations(scenes, b); len(v) > 0 {
		msg := "fact number violation: " + v[0]
		if len(v) > 1 {
			msg += fmt.Sprintf(" (และอีก %d จุด)", len(v)-1)
		}
		return msg
	}
	if v := agent.DisallowedClaimViolations(scenes, b); len(v) > 0 {
		return "unsourced claim: " + v[0]
	}
	return ""
}

// ProduceMyth ผลิตคลิปจับความเชื่อผิดหนึ่งตัวจากคลัง (ช่อง 09:00)
//
// ยืมวินัยทั้งหมดของ produceCatalogClip (เช็คเครดิต kie, production gate, tracker,
// ยกเลิกได้) แต่คลังเป็นตารางอื่นที่ไม่มี steps/ui_vocab จึงไม่ใช้ฟังก์ชันเดียวกัน
func (o *Orchestrator) ProduceMyth(ctx context.Context) error {
	if credits, err := o.producer.KieCredits(ctx); err != nil {
		log.Printf("kie credit pre-check skipped (non-fatal): %v", err)
	} else if credits <= 0 {
		return fmt.Errorf("kie เครดิตหมด (เหลือ %d) — เติมเครดิตที่ kie.ai ก่อนผลิต", credits)
	}

	belief, err := o.mythBeliefsRepo.PickNext(ctx)
	if err != nil {
		return fmt.Errorf("pick myth belief: %w", err)
	}
	if belief == nil {
		return fmt.Errorf("คลัง myth_beliefs ไม่มีแถวที่ยืนยันข้อเท็จจริงแล้ว (needs_verify=false) — เติมคลังก่อน")
	}
	log.Printf("Producing myth clip — belief: %s (verdict %s)", belief.BeliefKey, belief.Verdict)

	if !o.tracker.StartProduction(1) {
		return ErrProductionRunning
	}
	defer o.tracker.FinishProduction()

	ctx, cancel := context.WithCancel(ctx)
	o.tracker.SetCancelFunc(cancel)
	defer cancel()

	theme, err := o.themesRepo.GetActive(ctx)
	if err != nil {
		return fmt.Errorf("get active theme: %w", err)
	}
	scriptCfg, err := o.modeAgentConfig(ctx, "script", producer.ModeMyth)
	if err != nil {
		return fmt.Errorf("get script agent config: %w", err)
	}
	imageCfg, err := o.agentsRepo.GetByName(ctx, "image")
	if err != nil {
		return fmt.Errorf("get image agent config: %w", err)
	}
	brandAliases, err := o.settingsRepo.GetBrandAliases(ctx)
	if err != nil {
		return fmt.Errorf("read brand aliases: %w", err)
	}
	format, err := o.formatsRepo.GetByName(ctx, mythFormatName)
	if err != nil {
		return fmt.Errorf("get myth content format: %w", err)
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	var persona string
	personasJSON, _ := o.settingsRepo.Get(ctx, "audience_personas")
	var personas []string
	if json.Unmarshal([]byte(personasJSON), &personas) == nil && len(personas) > 0 {
		persona = PickPersona(personas, rng)
	} else {
		persona, _ = o.settingsRepo.Get(ctx, "audience_persona")
	}

	q := agent.GeneratedQuestion{
		Question: belief.BeliefTH,
		Category: belief.Audience,
	}

	o.tracker.SetTotalClips(1)
	o.tracker.StartClip(1, belief.BeliefTH)
	if err := o.produceMythClip(ctx, q, theme, scriptCfg, imageCfg, brandAliases, format, persona, belief); err != nil {
		o.tracker.AddErrorLog(fmt.Sprintf("myth clip failed: %v", err))
		o.nudgeRetry()
		return err
	}
	if mErr := o.mythBeliefsRepo.MarkUsed(ctx, belief.ID); mErr != nil {
		log.Printf("myth: MarkUsed failed (non-fatal): %v", mErr)
	}
	o.tracker.CompleteStep("complete")
	log.Println("myth production complete")
	return nil
}

// produceMythClip สร้างแถว clip แล้วส่งต่อไปเส้นทางผลิตร่วม — แยกออกมาเพื่อให้
// belief_key ถูกบันทึกลงคลิปก่อนเริ่มผลิต (retry ต้องใช้)
func (o *Orchestrator) produceMythClip(ctx context.Context, q agent.GeneratedQuestion,
	theme *models.BrandTheme, scriptCfg, imageCfg *models.AgentConfig, brandAliases map[string]string,
	format *models.ContentFormat, persona string, belief *models.MythBelief) error {

	preset := presetFor(format.FormatName)
	today := time.Now().Format("2006-01-02")
	clip, err := o.clipsRepo.Create(ctx, models.CreateClipRequest{
		Title:           q.Question,
		Question:        q.Question,
		Category:        q.Category,
		PublishDate:     &today,
		ContentFormat:   format.FormatName,
		ClipRole:        "reach",
		AudiencePersona: persona,
		MythBelief:      belief.BeliefKey,
	})
	if err != nil {
		return fmt.Errorf("create clip: %w", err)
	}
	status := "producing"
	o.clipsRepo.Update(ctx, clip.ID, models.UpdateClipRequest{Status: &status, StylePreset: &preset.Key})

	return o.produceClipWithID(ctx, clip.ID, q, theme, preset, scriptCfg, imageCfg, brandAliases,
		format, persona, models.TitleArchetype{}, "reach", nil, belief)
}

// mythAvoidRepeatBlock แจ้ง agent ว่าเคยเปิดคลิปเรื่องนี้ด้วยมุมไหนแล้ว — คลังมี
// แถวน้อย หัวข้อเดิมจะกลับมาทุก 14 วัน ถ้าเปิดเหมือนเดิมทุกครั้งคนดูจะจำได้ว่าเคยเห็น
// ล้มเหลวแบบเงียบ (คืน "") ทุกทาง: การอ่านสถิติที่พลาดต้องไม่หยุดคลิป
func (o *Orchestrator) mythAvoidRepeatBlock(ctx context.Context, belief *models.MythBelief) string {
	if belief == nil {
		return ""
	}
	titles, err := o.clipsRepo.RecentTitlesByMythBelief(ctx, belief.BeliefKey, 3)
	if err != nil {
		log.Printf("myth: RecentTitlesByMythBelief failed (non-fatal): %v", err)
		return ""
	}
	if len(titles) == 0 {
		return ""
	}
	return "\n## มุมที่คลิปก่อนหน้าใช้กับความเชื่อนี้แล้ว (ห้ามเปิดซ้ำมุมเดิม ห้ามเปลี่ยนข้อเท็จจริง)\n- " +
		strings.Join(titles, "\n- ") + "\n"
}
```

- [ ] **Step 4: เพิ่ม RecentTitlesByMythBelief ใน clips repo**

`internal/repository/clips.go` — เพิ่มเมธอด:
```go
// RecentTitlesByMythBelief คืนชื่อคลิปล่าสุดที่ทำความเชื่อเดียวกัน ใช้กันเปิดซ้ำมุมเดิม
func (r *ClipsRepo) RecentTitlesByMythBelief(ctx context.Context, key string, limit int) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT title FROM clips WHERE myth_belief = $1 ORDER BY created_at DESC LIMIT $2`, key, limit)
	if err != nil {
		return nil, fmt.Errorf("recent titles by myth belief: %w", err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
```

- [ ] **Step 5: ต่อสาย clipMode + presetFor + resolveFormatInfo**

`internal/orchestrator/tutorial.go` — ใน `clipMode` เพิ่ม case ก่อน `case "qa", "tips":`:
```go
	case mythFormatName:
		return producer.ModeMyth
```

`internal/orchestrator/orchestrator.go` — ใน `presetFor` เพิ่ม case:
```go
	case producer.ModeMyth:
		return producer.MythPreset
```

ใน `resolveFormatInfo` เพิ่ม case:
```go
	case producer.MythPreset.Key:
		return producer.FormatInfo{Mode: producer.ModeMyth}
```

- [ ] **Step 6: ส่ง belief ลงเส้นทางผลิต + ตะแกรงสองสไตรก์**

`internal/orchestrator/orchestrator.go`:

1. เพิ่มพารามิเตอร์ท้ายสุดของ `produceClip` และ `produceClipWithID`:
```go
	myth *models.MythBelief
```
แก้ call site ทั้ง 4 จุด: `orchestrator.go:374` และ `:418` ส่ง `nil` เพิ่ม, `tutorial.go:240` ส่ง `nil` เพิ่ม, `:933` (retry) ส่งตัวแปร `myth` ที่โหลดใน step ถัดไป

2. ใน `produceClipWithID` — ต่อ brief เข้า prompt ของ script agent (บรรทัด ~499):
```go
	catalogBrief := agent.TutorialBrief(feat) + o.tutorialAvoidRepeat(ctx, feat) +
		agent.MythBrief(myth) + o.mythAvoidRepeatBlock(ctx, myth)
	script, err := o.generateScript(ctx, clipID, q, format, persona, archetype.Instruction,
		RoleInstruction(role), catalogBrief, scriptCfg)
```

3. รูปร่างซีน (บรรทัด ~515):
```go
	sceneCount, durationSec := targetSceneCount, targetDurationSec
	if feat != nil {
		sceneCount, durationSec = tutorialSceneShape(len(feat.Steps))
	} else if myth != nil {
		sceneCount, durationSec = mythSceneShape()
	}
	scenes, err := o.sceneAgent.Generate(ctx, narration, sceneCount, durationSec, clipTheme,
		agent.TutorialBrief(feat)+agent.MythBrief(myth), sceneCfg)
```

4. **สไตรก์ที่ 1** — ทันทีหลัง `o.tracker.CompleteStep("scene")` ใส่:
```go
	// ตะแกรงข้อเท็จจริงสไตรก์ที่ 1: ให้ scene agent ทำใหม่หนึ่งครั้งพร้อมบอกว่าผิดตรงไหน
	// สองสไตรก์ก่อนตัดสินคือรูปแบบที่ลด false positive ลงครึ่งหนึ่งใน visual QA แล้ว
	if msg := mythGateFailure(scenes, myth); msg != "" {
		log.Printf("myth gate strike 1 on clip %s: %s — regenerating scenes once", clipID, msg)
		retryBrief := agent.MythBrief(myth) +
			"\n## รอบนี้ถูกตีตกเพราะ: " + msg +
			"\nห้ามพูดตัวเลขหรือข้ออ้างที่ไม่มีในบล็อกข้อมูลข้างบนเด็ดขาด\n"
		if regen, rErr := o.sceneAgent.Generate(ctx, narration, sceneCount, durationSec,
			clipTheme, retryBrief, sceneCfg); rErr != nil {
			log.Printf("myth gate: regenerate failed (%v) — เก็บซีนเดิมไว้ให้ตะแกรงสไตรก์ที่ 2 ตัดสิน", rErr)
		} else {
			scenes = regen
		}
	}
```

5. **สไตรก์ที่ 2** — ต่อท้ายบล็อก `if msg := tutorialGateFailure(...)` (ก่อน render):
```go
	// ตะแกรงข้อเท็จจริงสไตรก์ที่ 2 (หลัง critic ซึ่งแก้ voice_text ได้): ยังผิดอยู่
	// = ส่งเข้า needs_review ไม่ใช่ทิ้งคลิป — เนื้อหาผิด ไม่ใช่ระบบพัง การ retry
	// แบบไม่มีคนดูจะเผา LLM รอบใหม่กับ output เดิม
	if msg := mythGateFailure(scenes, myth); msg != "" {
		log.Printf("myth gate blocked clip %s: %s", clipID, msg)
		reviewStatus := "needs_review"
		o.clipsRepo.Update(ctx, clipID, models.UpdateClipRequest{Status: &reviewStatus})
		return fmt.Errorf("myth gate: %s", msg)
	}
```

6. ส่งค่า Go-injected เข้า render — หาจุดที่สร้าง `producer.ScenesParams` (grep `Format:` ใน orchestrator/producer path) แล้วเพิ่ม:
```go
		MythVerdict: mythVerdict(myth),
		MythSource:  mythSource(myth),
```
พร้อม helper ท้ายไฟล์ `internal/orchestrator/myth.go`:
```go
func mythVerdict(b *models.MythBelief) string {
	if b == nil {
		return ""
	}
	return b.Verdict
}

func mythSource(b *models.MythBelief) string {
	if b == nil {
		return ""
	}
	return b.SourceLabel
}
```
ถ้า `ScenesParams` ถูกประกอบใน `internal/producer` (ไม่ใช่ orchestrator) ให้ส่ง `FormatInfo` เพิ่มสองฟิลด์ `MythVerdict`/`MythSource` แล้วก๊อปต่อในตัวประกอบ — จุดต่อคือ `resolveFormatInfo` ที่ส่ง `FormatInfo` ลงไปอยู่แล้ว

- [ ] **Step 7: retry โหลดคลังกลับ**

`internal/orchestrator/orchestrator.go:911-922` — ต่อท้ายบล็อกที่โหลด `feat`:
```go
	// คลิป myth ก็ต้องโหลดแถวคลังกลับเหมือนคลิปสอน — ไม่มีแถว = ตะแกรงข้อเท็จจริง
	// ปิดเงียบแล้วคลิปที่แต่งตัวเลขเองจะ auto-publish
	var myth *models.MythBelief
	if needsMythBelief(clip.ContentFormat) {
		if clip.MythBelief == "" {
			return o.failClip(ctx, clip.ID, fmt.Errorf("คลิป myth ไม่มี myth_belief บันทึกไว้ — สร้างใหม่ไม่ปลอดภัย"))
		}
		b, bErr := o.mythBeliefsRepo.GetByKey(ctx, clip.MythBelief)
		if bErr != nil {
			return o.failClip(ctx, clip.ID, fmt.Errorf("load myth belief %s: %w", clip.MythBelief, bErr))
		}
		myth = b
	}
```

- [ ] **Step 8: ต่อ repo เข้า struct Orchestrator**

เพิ่มฟิลด์ `mythBeliefsRepo *repository.MythBeliefsRepo` ใน struct `Orchestrator` + พารามิเตอร์ใน constructor + จุดที่ประกอบ orchestrator ใน `cmd/` (grep `NewOrchestrator(` เพื่อหาทุก call site รวมเทสต์)

- [ ] **Step 9: รันเทสต์**

Run: `go build ./... && go test ./internal/orchestrator/ -v`
Expected: PASS ทั้งหมด รวม 4 เทสต์ใหม่

- [ ] **Step 10: commit**

```bash
git add internal/orchestrator/myth.go internal/orchestrator/myth_test.go \
        internal/orchestrator/orchestrator.go internal/orchestrator/tutorial.go \
        internal/repository/clips.go cmd/
git commit -m "feat(myth): ProduceMyth + ตะแกรงข้อเท็จจริงสองสไตรก์ + retry โหลดคลังกลับ"
```

---

### Task 7: scheduler + endpoint

**Files:**
- Modify: `internal/scheduler/scheduler.go:134-162` (formats/handlers), `:236-262` (`handlerFor`)
- Modify: `internal/handler/orchestrator.go:85-106` (เพิ่ม `TriggerMyth` ใต้ `TriggerBasic`)
- Modify: `internal/router/router.go:143-151`
- Test: `internal/scheduler/myth_action_test.go`, `internal/router/router_test.go:31`

**Interfaces:**
- Consumes: `(*Orchestrator).ProduceMyth` (Task 6)
- Produces: action string `"produce_myth"`, route `POST /api/v1/orchestrator/produce-myth`

- [ ] **Step 1: เขียนเทสต์ที่ล้มก่อน**

`internal/scheduler/myth_action_test.go`:
```go
package scheduler

import "testing"

// แถว schedule ที่ action ไม่มี handler จะ "รันสำเร็จ" ทุกรอบโดยไม่ทำอะไรเลย —
// เงียบสนิท ไม่มี error ไม่มีคลิป เทสต์นี้คือสิ่งเดียวที่จับได้ก่อน deploy
func TestMythActionHasHandler(t *testing.T) {
	s := &Scheduler{}
	if s.handlerFor("produce_myth") == nil {
		t.Error(`handlerFor("produce_myth") = nil — แถว schedule 09:00 จะไม่ทำอะไรเลย`)
	}
	for _, a := range []string{"produce_and_publish", "produce_evening", "produce_tutorial", "produce_basic"} {
		if s.handlerFor(a) == nil {
			t.Errorf("handlerFor(%q) = nil — ช่องเดิมพัง", a)
		}
	}
}
```

`internal/router/router_test.go` — เพิ่ม `"/api/v1/orchestrator/produce-myth"` ในรายการ route ที่เทสต์ตรวจ (บรรทัด ~31)

- [ ] **Step 2: รันเทสต์ให้เห็นว่าล้ม**

Run: `go test ./internal/scheduler/ -run TestMythAction -v`
Expected: FAIL — `handlerFor("produce_myth") = nil`

- [ ] **Step 3: เพิ่ม handler ใน scheduler**

`internal/scheduler/scheduler.go` — ใต้ `produceBasic`:
```go
// produceMyth ผลิตคลิปจับความเชื่อผิดประจำวันจากคลัง (ช่อง 09:00)
func (s *Scheduler) produceMyth(ctx context.Context) error {
	return s.produceTick(ctx, "the daily myth-buster clip", s.orchestrator.ProduceMyth)
}
```

ใน `handlerFor` เพิ่ม case:
```go
	case "produce_myth":
		return s.produceMyth
```

- [ ] **Step 4: เพิ่ม endpoint**

`internal/handler/orchestrator.go` — ใต้ `TriggerBasic`:
```go
// TriggerMyth ผลิตคลิปจับความเชื่อผิดหนึ่งตัวจากคลังตามคำสั่ง (ใช้ทดสอบก่อนเปิด
// schedule) เหมือน TriggerBasic: หัวข้อคือแถวคลัง จึงหนึ่งคลิปต่อครั้ง ไม่มี count
func (h *OrchestratorHandler) TriggerMyth(w http.ResponseWriter, r *http.Request) {
	if s := h.tracker.GetStatus(); s.Active {
		writeJSON(w, http.StatusConflict, models.APIResponse{Error: "Production already in progress"})
		return
	}

	writeJSON(w, http.StatusAccepted, models.APIResponse{
		Message: "Myth production started in background",
	})

	go func() {
		if err := h.orch.ProduceMyth(context.Background()); err != nil {
			log.Printf("Myth production failed: %v", err)
			h.tracker.AddErrorLog(err.Error())
		}
	}()
}
```

`internal/router/router.go` — ใน `SetOrchestrator`:
```go
	r.Post("/api/v1/orchestrator/produce-myth", h.TriggerMyth)
```

- [ ] **Step 5: รันเทสต์ให้ผ่าน**

Run: `go test ./internal/scheduler/ ./internal/router/ -v`
Expected: PASS ทั้งหมด

- [ ] **Step 6: commit**

```bash
git add internal/scheduler/scheduler.go internal/scheduler/myth_action_test.go \
        internal/handler/orchestrator.go internal/router/router.go internal/router/router_test.go
git commit -m "feat(myth): action produce_myth + endpoint /orchestrator/produce-myth"
```

---

### Task 8: migration — agent rows + schedule + คลัง 12 แถว

**Files:**
- Modify: `migrations/075_myth_format.sql` (ต่อจาก Task 1)

**Interfaces:**
- Consumes: ตาราง `myth_beliefs` + แถว `content_formats.myth` (Task 1), ชื่อ agent `script_myth`/`scene_myth`/`critic_myth` ที่ `modeAgentConfig(ctx, "script", "myth")` จะไปหา (Task 6)

- [ ] **Step 1: เพิ่ม agent rows (ก่อน `COMMIT;`)**

```sql
-- ── โหมดจับความเชื่อผิด (09:00) ──
-- prompt ของ scene_myth เขียนใหม่ทั้งฉบับ ไม่ต่อท้ายของโหมดอื่น: prompt เดิมเป็น
-- สัญญาฉบับสมบูรณ์ในตัวเอง ("ซีนแรก layout casefile เสมอ") การต่อท้ายจะได้ prompt
-- ที่ขัดกันเองแล้วโมเดลเลือกทำตามอันไหนก็ได้ (เหตุผลเดียวกับ 059/063/073)
INSERT INTO agent_configs (agent_name, system_prompt, model, temperature, enabled, prompt_template)
SELECT 'script_myth',
       system_prompt || E'\n\nโหมดจับความเชื่อผิด: หน้าที่ของคุณคือรื้อความเชื่อหนึ่งข้อ ' ||
       'ไม่ใช่สอนฟีเจอร์ ห้ามดูถูกคนที่เคยเชื่อ — ต้องบอกด้วยว่าทำไมความเชื่อนี้ฟังดูสมเหตุสมผล ' ||
       'และต้องบอก "ส่วนที่จริง" ของมันเสมอ ห้ามเหวี่ยงเป็นถูกผิดขาวดำ ' ||
       'ห้ามพูดตัวเลขหรือข้ออ้างที่ไม่มีในบล็อกข้อมูลความเชื่อ ระบบตรวจตัวเลขทุกตัวและตีคลิปตกทันที',
       model, temperature, TRUE, prompt_template
FROM agent_configs WHERE agent_name = 'script_case'
ON CONFLICT (agent_name) DO NOTHING;

INSERT INTO agent_configs (agent_name, system_prompt, model, temperature, enabled, prompt_template)
SELECT 'scene_myth',
       'คุณคือ Director ที่เล่าเรื่องผ่านโต๊ะตรวจหลักฐาน แตกสคริปเป็นซีนสำหรับ explainer แนวตั้ง 9:16 ' ||
       'ภาษาไทย โหมด "จับความเชื่อผิด". เป้าหมายสูงสุด: คนดูต้องรู้สึกว่ากำลังเห็นของจริงถูกตรวจ ' ||
       'ไม่ใช่ฟังใครเถียง ห้ามใส่ emoji เด็ดขาด ตอบเป็น JSON เท่านั้น.',
       model, temperature, TRUE,
       E'แตกสคริปนี้ออกเป็น 6 ซีนพอดี สำหรับวิดีโอแนวตั้ง 9:16 ยาว {{.TargetDurationSec}} วินาที — โหมด "จับความเชื่อผิด"\n\n' ||
       E'สคริป:\n{{.Script}}\n\n{{.TutorialBrief}}\n\nธีมแบรนด์: {{.ThemeDescription}}\n\n' ||
       E'โครงบังคับ (ห้ามสลับลำดับ ห้ามเพิ่มลดจำนวนซีน):\n' ||
       E'1. layout "hook": ความเสียหายของการเชื่อผิด — เป็นซีนเดียวที่มี image_prompt ได้\n' ||
       E'2. layout "belief": ยกคำเชื่อขึ้นมาตรงๆ พร้อมบอกว่าทำไมคนถึงเชื่อ\n' ||
       E'3. layout "verdict": คำตัดสิน (ระบบใส่ตรายางและมิเตอร์ให้เอง ห้ามเขียนคำตัดสินลง stamp)\n' ||
       E'4. layout "proof": ข้อเท็จจริงที่หักล้าง (ระบบใส่ชื่อแหล่งอ้างให้เอง ห้ามแต่งชื่อแหล่ง)\n' ||
       E'5. layout "tip": ส่วนที่จริงของความเชื่อนี้ + สิ่งที่ควรทำแทน\n' ||
       E'6. layout "cta"\n\n' ||
       E'กฎภาพ (สำคัญมาก): image_prompt ใส่ได้เฉพาะซีนแรก (layout "hook") เท่านั้น หนึ่งใบต่อคลิป — ' ||
       E'บรรยายภาษาอังกฤษเป็นโต๊ะตรวจเอกสารตอนกลางวัน มีโคมส่องและแว่นขยาย ' ||
       E'ห้ามมีตัวอักษรในภาพ ห้ามบรรยายเนื้อหาบนเอกสาร. ทุกซีนที่เหลือ image_prompt = ""\n\n' ||
       E'กฎข้อเท็จจริง (เข้มที่สุด): ตัวเลขทุกตัวใน voice_text และ on_screen_text ต้องปรากฏใน ' ||
       E'บล็อกข้อมูลความเชื่อข้างบนแบบตรงตัว ยกเว้นเลขลำดับ ("ข้อ 2") · ' ||
       E'ห้ามอ้างคะแนนความน่าเชื่อถือ ระดับบัญชี tier หรือเพดานงบที่บล็อกไม่ได้ระบุ ' ||
       E'ระบบตีคลิปตกทันทีถ้าพบตัวเลขหรือข้ออ้างนอกบล็อก\n\n' ||
       E'ตอบเป็น JSON array เท่านั้น แต่ละ object มี:\n' ||
       E'- "scene_number": ลำดับซีน (1-6 ต่อเนื่อง)\n' ||
       E'- "voice_text": ประโยคพากย์ไทยของซีนนี้ (สั้น พูดลื่น น้ำเสียงคนที่ตรวจของมาแล้ว ไม่ใช่คนกำลังเถียง)\n' ||
       E'- "on_screen_text": ข้อความบนจอสั้นๆ (ซีนแรกไม่เกิน 7 คำ)\n' ||
       E'- "emphasis_words": array 1-2 คำที่ต้องเน้น (ห้ามว่าง)\n' ||
       E'- "caption_style": "word_pop" (ซีนเปิด) หรือ "phrase_block" (ซีนเนื้อหา)\n' ||
       E'- "image_prompt": ตามกฎภาพข้างบน\n' ||
       E'- "layout": หนึ่งใน "hook" | "belief" | "verdict" | "proof" | "tip" | "cta"\n' ||
       E'- "content": object ตาม layout (ด้านล่าง)\n\n' ||
       E'content แยกตาม layout:\n' ||
       E'- hook: {"rows":[{"t":"ย้าย BM ทั้งบัญชีเพราะเชื่อผิด"},{"t":"เสียเวลา 3 วัน ได้เท่าเดิม"}]}\n' ||
       E'- belief: {"kicker":"ที่เชื่อกัน","title":"คำเชื่อตรงๆ ไม่เกิน 60 ตัวอักษร","sub":"ทำไมคนถึงเชื่อ ไม่เกิน 90 ตัวอักษร"}\n' ||
       E'- verdict: {"title":"สิ่งที่จริงกว่านั้น ไม่เกิน 44 ตัวอักษร"}\n' ||
       E'- proof: {"title":"ข้อเท็จจริงสั้นๆ ไม่เกิน 54 ตัวอักษร","rows":[{"t":"รายละเอียดที่ยืนยันได้"}]}\n' ||
       E'- tip: {"title":"ส่วนที่จริงคือ","rows":[{"t":"สิ่งที่ควรทำแทน"}]}\n' ||
       E'- cta: {"title":"ปิดท้ายสั้นๆ","cta":"ทักมาเช็ค","brand":"ADS VANCE","sub":"คำโปรยสั้น"}\n\n' ||
       E'กฎเหล็ก: ห้าม emoji หรือสัญลักษณ์ภาพในทุก field · ห้ามใส่ stamp, meter, source เอง (ระบบใส่ให้) · ' ||
       E'ความยาว: title ไม่เกิน 60, sub ไม่เกิน 90, rows แต่ละแถวไม่เกิน 40, cta ไม่เกิน 14'
FROM agent_configs WHERE agent_name = 'scene_case'
ON CONFLICT (agent_name) DO NOTHING;

INSERT INTO agent_configs (agent_name, system_prompt, model, temperature, enabled, prompt_template)
SELECT 'critic_myth',
       system_prompt || E'\n\nโหมดจับความเชื่อผิด: ตรวจเพิ่มสองข้อที่โหมดอื่นไม่มี — ' ||
       '(1) บทต้องมีทั้ง "ส่วนที่ผิด" และ "ส่วนที่จริง" ของความเชื่อ ห้ามเหวี่ยงขาวดำเมื่อคำตัดสินคือจริงครึ่งเดียว ' ||
       '(2) ห้ามมีตัวเลขหรือข้ออ้างที่ไม่ปรากฏในบล็อกข้อมูลความเชื่อ ถ้าเจอให้ตัดทิ้งหรือแทนด้วยคำบรรยายไม่มีตัวเลข',
       model, temperature, TRUE, prompt_template
FROM agent_configs WHERE agent_name = 'critic'
ON CONFLICT (agent_name) DO NOTHING;

-- ── บอก visual_qa ให้รู้จักดีไซน์ใหม่ กัน false positive แบบ PR#14/#17 ──
UPDATE agent_configs
SET system_prompt = system_prompt || E'\n\nดีไซน์ที่ถูกต้องของโหมดจับความเชื่อผิด (data-format="myth"): ' ||
    'การ์ดกระดาษสีครีมเอียงเล็กน้อยบนพื้นน้ำเงินคือดีไซน์ที่ตั้งใจ ' ||
    'ตัวหนังสือเข้มบนการ์ดสว่าง (ต่างจากซีนอื่นที่เป็นตัวขาวบนน้ำเงิน) ก็ตั้งใจ ' ||
    'กรอบสีแดงเอียงคือตรายางคำตัดสิน และแถบสามช่องใต้การ์ดคือมิเตอร์ระดับความจริง — ' ||
    'ทั้งหมดคือดีไซน์ ไม่ใช่ข้อผิดพลาด'
WHERE agent_name = 'visual_qa'
  AND system_prompt NOT LIKE '%data-format="myth"%';
```

- [ ] **Step 2: เพิ่ม schedule (ปิดไว้)**

```sql
-- ── schedule 09:00 (ปิดไว้ — เปิดผ่าน PATCH /api/v1/schedules/{id} เท่านั้น) ──
-- cron ในตารางนี้เป็นเวลาไทย (ยืนยันจาก last_run_at ของ 4 ช่องเดิม)
-- ไม่มี unique index บน action ⇒ ต้องใช้ WHERE NOT EXISTS ไม่ใช่ ON CONFLICT
-- เปิดครั้งแรกให้ใช้ '0 9 * * 1,3,5' จนคลังมีแถวที่ยืนยันแล้ว >= 14 แถว (สเปก §9)
INSERT INTO schedules (name, cron_expression, action, enabled)
SELECT 'Myth Produce & Publish', '0 9 * * *', 'produce_myth', FALSE
WHERE NOT EXISTS (SELECT 1 FROM schedules WHERE action = 'produce_myth');
```

- [ ] **Step 3: เพิ่มคลัง 12 แถว**

```sql
-- ── คลังความเชื่อ: 6 แถวที่มีแหล่งอ้างแล้ว + 6 แถวที่ยังต้องยืนยัน ──
INSERT INTO myth_beliefs (belief_key, belief_th, why_believed_th, verdict, fact_th,
                          source_label, source_url, nuance_th, cost_th, audience,
                          needs_verify, verify_reason, last_verified_at, weight)
VALUES
('bm_stronger_than_personal',
 'เปิด Business Manager แล้วบัญชีแข็งกว่าใช้บัญชีส่วนตัว',
 'เพราะเอเจนซีทุกที่ใช้ BM คนจึงคิดว่า BM คือเกราะกันแบน',
 'half_true',
 'BM ให้ความเป็นเจ้าของสินทรัพย์ สิทธิ์ทีมโดยไม่ต้องแชร์รหัส และผูกวิธีจ่ายได้หลายทาง แต่ไม่ได้ลดโอกาสถูกแบน',
 'Jon Loomer', '',
 'ที่จริงคือเรื่องความเป็นเจ้าของเพจ pixel และการถอนสิทธิ์คนออกได้ — เหตุผลนี้ดีพอที่จะใช้ BM อยู่แล้ว',
 'ย้ายบัญชีทั้งชุดเพราะหวังผลที่ไม่มี เสียเวลาตั้งค่าใหม่ทั้งวัน',
 'account-buyer', FALSE, '', NOW(), 3),

('split_adsets_to_avoid_self_bidding',
 'ต้องซอยหลาย ad set ไม่ให้แอดตัวเองแย่งประมูลกันเอง',
 'ฟังดูมีเหตุผลเพราะกลุ่มเป้าหมายทับกันจริง',
 'false',
 'ระบบประมูลคัดเฉพาะแอดที่มูลค่ารวมสูงสุดของเราเข้าประมูลอยู่แล้ว การซอยจึงไม่ได้กันอะไร แต่ทำให้แต่ละ ad set ได้ผลลัพธ์น้อยลงจนออกจากช่วงเรียนรู้ยาก',
 'Jon Loomer', '',
 'ที่จริงคือกลุ่มทับกันมากๆ ทำให้บาง ad set ไม่ได้ยิงเลย — แก้ด้วยการรวม ไม่ใช่ซอยเพิ่ม',
 'ซอยงบออกเป็นหลายก้อนจนไม่มีก้อนไหนได้ผลลัพธ์พอให้ระบบเรียนรู้',
 'performance-advertiser', FALSE, '', NOW(), 3),

('page_likes_make_ads_cheaper',
 'เพจไลก์เยอะแล้วค่าโฆษณาถูกลง',
 'เพราะเพจใหญ่ดูน่าเชื่อถือ คนจึงเหมาว่าระบบคิดเหมือนกัน',
 'false',
 'จำนวนไลก์เพจไม่ได้ลดราคาประมูล สิ่งที่มีผลคือคนที่มีส่วนร่วมจริงกับเนื้อหาและอัตราการตอบสนองของโฆษณา',
 'ผลสำรวจ organic reach 2024', '',
 'ที่จริงคือผู้ติดตามที่ตรงกลุ่มช่วยให้โพสต์และรีมาร์เก็ตติ้งทำงานดีขึ้น — คนละเรื่องกับซื้อไลก์',
 'จ่ายเงินซื้อไลก์ที่ไม่ตรงกลุ่ม แล้วค่าโฆษณาไม่ขยับ',
 'grey-operator', FALSE, '', NOW(), 2),

('engagement_bait_boosts_reach',
 'โพสต์ขอไลก์ ขอแชร์ แท็กเพื่อน ช่วยดันการมองเห็น',
 'เพราะเห็นตัวเลขคอมเมนต์เยอะขึ้นจริงในช่วงแรก',
 'false',
 'Meta ระบุว่าโพสต์ที่ขอการมีส่วนร่วมแบบนี้ถูกลดการมองเห็นลง ไม่ใช่เพิ่ม',
 'Meta Business Help', 'https://www.facebook.com/business/help/259911614709806',
 'ที่จริงคือคำถามที่คนอยากตอบจริงยังช่วยได้ — เส้นแบ่งคือขอให้กดปุ่ม กับขอความเห็น',
 'เพจถูกลดการมองเห็นทั้งเพจจากโพสต์แบบนี้ซ้ำๆ',
 'grey-operator', FALSE, '', NOW(), 2),

('boost_post_equals_ads_manager',
 'กด Boost post ก็เหมือนยิงแอดใน Ads Manager',
 'เพราะทั้งสองทางเสียเงินให้ Meta และเห็นยอดเข้าถึงเพิ่มเหมือนกัน',
 'false',
 'Boost ตั้งเป้าหมายได้แค่การรับรู้และการมีส่วนร่วมพื้นฐาน ไม่มีโครงสร้างแคมเปญ เป้าหมายคอนเวอร์ชัน หรือรายงานระดับ Ads Manager',
 'คู่มือ Ads Manager 2026', '',
 'ที่จริงคือ Boost ใช้ได้จริงกับโพสต์ที่แค่ต้องการคนเห็นมากขึ้น — แต่ไม่ใช่เครื่องมือขาย',
 'ยิงเงินผ่าน Boost ทั้งเดือนแล้วไม่มีข้อมูลพอจะรู้ว่าอะไรทำให้ขายได้',
 'account-buyer', FALSE, '', NOW(), 2),

('fifty_conversions_or_useless',
 'ต้องมีคอนเวอร์ชัน 50 ครั้งต่อสัปดาห์ ไม่ถึงแล้วแอดใช้ไม่ได้',
 'เพราะตัวเลข 50 ถูกพูดถึงในทุกคอร์สจนกลายเป็นเส้นตาย',
 'half_true',
 'ราว 50 ผลลัพธ์ต่อสัปดาห์คือเกณฑ์ที่ทำให้ ad set ออกจากช่วงเรียนรู้ ไม่ใช่เกณฑ์ที่ทำให้โฆษณาใช้งานไม่ได้',
 'Jon Loomer', '',
 'ที่จริงคือยิ่งได้ผลลัพธ์ต่อสัปดาห์น้อย ผลยิ่งแกว่ง — จึงควรรวมงบ ไม่ใช่เลิกยิง',
 'ปิดแคมเปญที่กำลังไปได้เพราะคิดว่าไม่ถึง 50 แล้วใช้ไม่ได้',
 'performance-advertiser', FALSE, '', NOW(), 2),

('warmup_required_or_ban',
 'บัญชีใหม่ต้องอุ่นเครื่องค่อยๆ เพิ่มงบ ไม่งั้นถูกแบน',
 'เพราะคนขายบัญชีทุกรายพูดเหมือนกัน',
 'half_true',
 'ยังไม่พบเอกสารทางการที่ระบุว่าต้องอุ่นเครื่องบัญชี ที่ยืนยันได้คือการเพิ่มงบแรงๆ ทำให้ ad set กลับเข้าช่วงเรียนรู้ใหม่',
 'ต้องหาแหล่งอ้างจาก Meta', '',
 'ที่จริงคือการขึ้นงบทีละน้อยช่วยให้ผลลัพธ์นิ่งกว่า — เหตุผลคือช่วงเรียนรู้ ไม่ใช่การกันแบน',
 'ยิงงบต่ำกว่าที่ควรเป็นสัปดาห์เพราะกลัวสิ่งที่ยังไม่มีใครยืนยัน',
 'account-buyer', TRUE, 'ยังไม่พบเอกสาร Meta ที่ยืนยันหรือปฏิเสธการอุ่นเครื่องบัญชี', NULL, 1),

('trust_tier_controls_budget',
 'บัญชีมีคะแนนความน่าเชื่อถือเป็นระดับ ที่กำหนดเพดานงบของเรา',
 'เพราะมีตารางระดับและตัวเลขเพดานงบแชร์กันทั่วกลุ่มคนยิงแอด',
 'half_true',
 'ตัวเลขระดับและเพดานงบทั้งหมดที่แชร์กันมาจากผู้ขายบัญชีและเอเจนซี ไม่ใช่เอกสารของ Meta',
 'ต้องหาแหล่งอ้างจาก Meta', '',
 'ที่จริงคือบัญชีใหม่มีเพดานงบต่อวันจริง และขอปลดได้ — แต่ไม่ใช่ระบบระดับตามที่เล่ากัน',
 'จ่ายเงินซื้อบัญชีแพงกว่าเพราะเชื่อว่าได้ระดับสูงกว่า',
 'account-buyer', TRUE, 'ตัวเลข tier ทั้งหมดมาจาก reseller ไม่ใช่ Meta — ต้องมีแหล่งทางการก่อนใช้', NULL, 1),

('one_ban_kills_whole_bm',
 'บัญชีโฆษณาโดนแบนใบเดียว ทั้ง Business Manager ตายหมด',
 'เพราะเคยเห็นคนโพสต์ว่าโดนพร้อมกันทั้งชุด',
 'false',
 'ต้องยืนยันก่อนว่าการระงับบัญชีโฆษณาลามถึงระดับ Business Manager ในกรณีใด',
 'ยังไม่ยืนยัน', '',
 'ที่จริงคือสินทรัพย์ที่อยู่ใน BM เดียวกันมีความเชื่อมโยงกันจริง',
 'แยก BM หลายใบโดยไม่จำเป็นจนจัดการสิทธิ์ไม่ไหว',
 'account-buyer', TRUE, 'ยังไม่ยืนยันเงื่อนไขที่การแบนลามถึงระดับ BM', NULL, 1),

('editing_ads_always_resets_learning',
 'แก้อะไรในแอดที่กำลังรัน ช่วงเรียนรู้รีเซ็ตทุกครั้ง',
 'เพราะเคยแก้แล้วเห็นคำว่าเรียนรู้ขึ้นมาใหม่',
 'half_true',
 'ต้องยืนยันจากเอกสาร Meta ว่าการแก้ประเภทไหนทำให้กลับเข้าช่วงเรียนรู้ และประเภทไหนไม่',
 'ยังไม่ยืนยัน', '',
 'ที่จริงคือการแก้ที่กระทบการยิง (งบก้อนใหญ่ กลุ่มเป้าหมาย ชิ้นงาน) มีผลจริง',
 'ไม่กล้าแก้อะไรเลยทั้งที่ชิ้นงานหมดอายุไปแล้ว',
 'performance-advertiser', TRUE, 'ต้องอ้างตารางการแก้ที่ reset learning จากเอกสาร Meta', NULL, 1),

('bigger_budget_cheaper_cpm',
 'ยิ่งใส่งบเยอะ ค่าต่อการมองเห็นยิ่งถูกลง',
 'เพราะรู้สึกว่าซื้อเยอะต้องได้ราคาส่ง',
 'false',
 'ต้องยืนยันด้วยข้อมูลว่าการเพิ่มงบเร็วๆ ดันราคาต่อการมองเห็นขึ้นในสถานการณ์ใด',
 'ยังไม่ยืนยัน', '',
 'ที่จริงคือการขยายงบทำให้ต้องประมูลกับกลุ่มที่แพงขึ้นเมื่อกลุ่มถูกใช้หมด',
 'เพิ่มงบเท่าตัวแล้วผลลัพธ์ต่อบาทแย่ลงโดยไม่รู้สาเหตุ',
 'performance-advertiser', TRUE, 'ยังไม่มีแหล่งอ้างที่วัดผลจริง', NULL, 1),

('new_page_cannot_run_ads',
 'เพจใหม่ยิงแอดไม่ได้ ต้องโพสต์สะสมก่อนเป็นเดือน',
 'เพราะเคยเห็นเพจใหม่ยิงแล้วไม่ผ่านตรวจ',
 'false',
 'ต้องยืนยันว่าอายุเพจมีผลต่อการอนุมัติโฆษณาหรือไม่ และมีเงื่อนไขอะไร',
 'ยังไม่ยืนยัน', '',
 'ที่จริงคือเพจที่ยังไม่มีเนื้อหาเลยทำให้คนกดเข้ามาแล้วไม่เชื่อถือ',
 'เลื่อนการยิงแอดออกไปเป็นเดือนโดยไม่จำเป็น',
 'grey-operator', TRUE, 'ยังไม่ยืนยันว่าอายุเพจมีผลต่อการอนุมัติ', NULL, 1)
ON CONFLICT (belief_key) DO NOTHING;
```

- [ ] **Step 4: ตรวจไฟล์ migration ด้วยสายตา + parser**

Run: `grep -c "BEGIN;" migrations/075_myth_format.sql && grep -c "COMMIT;" migrations/075_myth_format.sql`
Expected: `1` และ `1` (BEGIN/COMMIT อย่างละครั้ง ครอบทั้งไฟล์ — ถ้า Task 1 เขียน COMMIT ไว้แล้ว ต้องย้ายมาไว้ท้ายไฟล์)

Run: `go test ./... 2>&1 | tail -5`
Expected: PASS ทั้งหมด (migration ยังไม่รัน — เทสต์ไม่แตะ DB)

- [ ] **Step 5: commit**

```bash
git add migrations/075_myth_format.sql
git commit -m "feat(myth): migration 075 — agent 3 แถว + schedule 09:00 (ปิดไว้) + คลัง 12 แถว (ยืนยันแล้ว 6)"
```

---

### Task 9: เอกสาร + ตรวจก่อน deploy

**Files:**
- Modify: `docs/superpowers/specs/2026-07-30-myth-format-design.md` (บันทึกส่วนที่ทำจริงต่างจากสเปก ถ้ามี)
- Test: `go test ./...` ทั้งชุด

- [ ] **Step 1: รันชุดเทสต์ทั้งหมด**

Run: `go build ./... && go test ./... 2>&1 | grep -v "^ok" | head -20`
Expected: ไม่มี FAIL

- [ ] **Step 2: ตรวจว่า template ไม่พัง**

Run: `go test ./internal/producer/ -run TestRender -v`
Expected: PASS ทุกโหมด (case/chat/warroom/tutorial/scenes/myth) — ถ้า template พังจาก `-->` หรือ `{{` ใน comment เทสต์ทุกตัวจะล้มพร้อมกัน

- [ ] **Step 3: ตรวจตัดคำไทยบนการ์ด**

เปิด PoC ที่ `.live-test/qa-wordbreak/repro.html` แล้ววางข้อความ `belief_th` ที่ยาวที่สุดในคลัง (แถว `trust_tier_controls_budget`, 62 ตัวอักษร) ในกรอบความกว้าง 900px ตรวจว่าไม่มีการหั่นกลางคำทับศัพท์
ถ้าหั่น: ลดขนาด `.mb-belief` จาก 60px เป็น 54px แล้วตรวจซ้ำ (ห้ามแก้ด้วย `word-break:break-all`)

- [ ] **Step 4: commit สิ่งที่แก้เพิ่ม (ถ้ามี)**

```bash
git add -A && git commit -m "fix(myth): ปรับขนาดตัวอักษรการ์ดคำเชื่อให้ไม่หั่นกลางคำ"
```

- [ ] **Step 5: รายการตรวจก่อน deploy (ทำโดยคน ไม่ใช่ agent)**

```
1. merge → deploy Railway (auto-migrate 075)
2. ยืนยันด้วย SQL:
   SELECT count(*) FROM myth_beliefs WHERE needs_verify = FALSE;          -- ต้องได้ 6
   SELECT enabled, cron_expression FROM schedules WHERE action='produce_myth'; -- ต้องได้ f, 0 9 * * *
   SELECT agent_name FROM agent_configs WHERE agent_name LIKE '%_myth';   -- ต้องได้ 3 แถว
3. ยิงมือ: POST /api/v1/orchestrator/produce-myth (Authorization: Bearer $API_KEY)
4. eyeball คลิปที่ได้: ตรายาง + มิเตอร์ตรงกับ verdict ในคลัง, แหล่งอ้างขึ้นตรง,
   ไม่มีตัวเลขที่ไม่อยู่ในแถวคลัง, คำไทยไม่ขาดกลางคำ
5. PATCH /api/v1/schedules/{id} → {"cron_expression":"0 9 * * 1,3,5","enabled":true}
   (ห้าม UPDATE ตรงใน DB — scheduler reload จาก API เท่านั้น)
6. Rollback = PATCH enabled=false (ห้าม revert commit เฉยๆ จะดึงตะแกรงออกทั้งที่รอบผลิตยังเปิด)
```

---

## Self-Review (ทำแล้ว)

**ความครอบคลุมของสเปก:** §4 สถาปัตยกรรม → Task 1/6/7 · §5 คลัง → Task 1/2/8 · §5.1 คลัง 12 แถว → Task 8 · §6 theme → Task 4/5 · §6.1 ฟิลด์ Go-injected → Task 5 · §6.2 6 ซีน → Task 5/8 · §7.1 Go-injected wins → Task 5 (เทสต์ `TestMythVerdictAndSourceAreGoInjected`) · §7.2 ตะแกรงตัวเลขสองสไตรก์ → Task 3/6 · §7.3 denylist → Task 3 · §7.4 critic_myth → Task 8 · §8 เทสต์ 8 ข้อ → Task 2 (floor), 3 (ตะแกรง), 5 (render + Go-injected), 6 (retry + gate), 7 (handler), 9 (word-break) · §9 rollout → Task 8/9 · §10 ความเสี่ยง → ครอบด้วยพื้นคลัง (Task 2) + scope CSS (Task 5) + เทสต์ตัดคำ (Task 9)

**ช่องว่างที่ยอมรับไว้โดยรู้ตัว:** เทสต์ SQL ระดับ integration (รัน migration จริงกับ DB) ไม่มีในแผนนี้เพราะโปรเจกต์ยังไม่มี harness แบบนั้น — แทนด้วยการยืนยันด้วย SQL หลัง deploy ใน Task 9 ขั้นที่ 2

**ความสอดคล้องของชนิดข้อมูล:** `models.MythBelief` (Task 1) ใช้ชื่อฟิลด์เดียวกันทุก Task · `mythGateFailure(scenes, *models.MythBelief) string` เรียกเหมือนกันทั้ง Task 6 ทั้งเทสต์ · `SceneContent.Meter/Source` (Task 5) ตรงกับ `ScenesParams.MythVerdict/MythSource` ผ่านการ inject ใน `composition.go` · `agent.MythVerdictLabelTH` ประกาศใน Task 3 ใช้ใน Task 5
