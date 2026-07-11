# Content Brain v2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** แก้ยอดวิวต่ำ + content วนซ้ำ โดยขยายหมวดเป็น 10, เพิ่มบทบาท reach/convert 70/30, 7 title archetypes, 4 personas, harden dedup, และเติม insider KB pack — ทั้งหมดหลัง flag `content_brain_v2_enabled` (default off)

**Architecture:** เป็น additive overhaul ของ content-picking layer. ทุกตาราง/คอลัมน์ใหม่เป็น additive. Picker logic ใหม่ reuse ท่า least-used/7d+weight ของ `formats.PickNext`. flag off = พฤติกรรมเดิมทุก path (rollback ทันที). pipeline wiring กระจุกอยู่ที่ `orchestrator.ProduceWeekly`.

**Tech Stack:** Go (net/http + pgx/v5 + pgvector), Neon Postgres, Railway auto-migrate on boot, ไม่มี ORM (raw SQL ใน repository layer)

## Global Constraints

- **Flag gate:** ทุก behavior ใหม่ต้องเช็ค `settings.content_brain_v2_enabled == "true"` ก่อน; false = เดิมทุก path. อ่าน flag ผ่าน `settingsRepo.Get(ctx, "content_brain_v2_enabled")` แล้ว string-compare `== "true"` (ค่า default ว่าง/ไม่มี = false = พฤติกรรมเดิม).
- **Migration numbering:** master ปัจจุบันล่าสุด = `050_retry_tick_5min.sql`. Migration ใหม่ใช้ `051`. **ถ้า PR #17 (migration 051 two-strike) merge เข้า master ก่อน implement** → bump เลข migration ใน plan นี้ทั้งหมด +1 (ใช้ `052` แทน). ตรวจ `ls migrations/ | tail -3` ก่อนสร้างไฟล์ทุกครั้ง.
- **Migration style:** plain `.sql`, idempotent (`IF NOT EXISTS`, `ON CONFLICT DO NOTHING`), ทุก statement ในไฟล์เดียวรันเป็น `pool.Exec` เดียว (multi-statement). ไม่มี down-migration.
- **SQL quoting:** Thai text ใน migration ที่อาจมี `'` ให้ใช้ dollar-quoting `$$...$$` สำหรับ UPDATE prompt_template.
- **Template engine:** เป็น custom replacer (`internal/agent/template.go`) ไม่ใช่ `text/template` — แทน `{{.FieldName}}` literal ตามชื่อ field ของ struct (reflection). เพิ่ม arg = เพิ่ม field ใน struct + ใส่ `{{.FieldName}}` ใน prompt template ใน DB.
- **Settings read pattern:** `settingsRepo.Get(ctx, key) (string, error)`. number = parse string ด้วย `strconv.ParseFloat`/`ParseInt` (fail → default). JSON = `json.Unmarshal`.
- **Dedup location:** fail-open logic อยู่ใน `question.go:140-146` (caller), ไม่ใช่ใน `dedup.go`.
- **Guardrail คงเดิม:** ห้ามสร้างเนื้อหาแนะนำการทำผิดนโยบาย Facebook / หลบระบบตรวจจับ — prompt template insider voice ต้องคำนึง.
- **No placeholders:** ทุก step มี code/SQL/command จริงครบ.

---

## File Structure

**Create:**
- `migrations/051_content_brain_v2.sql` — ตาราง/คอลัมน์/seed/settings/prompt template (Task 1)
- `internal/repository/topics.go` — repo สำหรับ topic_categories + title_archetypes (Task 2)
- `internal/repository/topics_test.go` — (Task 2)
- `scripts/ingest_insider_kb.sh` — KB ingest script (Task 8)
- `scripts/insider_kb_content/*.txt` — 10 insider KB sources (Task 8)

**Modify:**
- `internal/models/format.go` — เพิ่ม struct TopicCategory, TitleArchetype (Task 2)
- `internal/models/request.go` — CreateClipRequest เพิ่ม ClipRole/TitleArchetype/AudiencePersona (Task 7)
- `internal/orchestrator/topic_pick.go` — picker ใหม่ 4 ตัว (Task 3)
- `internal/orchestrator/topic_pick_test.go` — (Task 3)
- `internal/agent/question.go` — Generate signature + template data + persist pain_point + dedup fail-closed (Task 4, 6)
- `internal/agent/question_test.go` — (Task 4)
- `internal/agent/script.go` — Generate signature + template data (Task 5)
- `internal/agent/script_test.go` — (Task 5)
- `internal/agent/dedup.go` — threshold from setting + pain_point cooldown + lexical fallback (Task 6)
- `internal/agent/dedup_test.go` — (Task 6)
- `internal/orchestrator/orchestrator.go` — wire pickers + flag gate + news fallback + persist columns (Task 7)
- `internal/orchestrator/orchestrator_test.go` — (Task 7)
- `internal/repository/clips.go` — Create/Update INSERT/UPDATE columns ใหม่ (Task 7)

---

## Task 1: Migration — schema, seeds, settings, prompt templates

**Files:**
- Create: `migrations/051_content_brain_v2.sql`

**Interfaces:**
- Consumes: รูปแบบ migration เดิม (ดู `017_content_formats.sql`, `049_analytics_full_fetch.sql`)
- Produces: tables `topic_categories`, `title_archetypes`; columns `clips.clip_role/title_archetype/audience_persona`, `topic_history.pain_point`; extension `pg_trgm`; settings keys; updated `agent_configs.prompt_template` สำหรับ question/script

- [ ] **Step 1: ตรวจเลข migration ล่าสุด**

Run: `ls migrations/ | tail -3`
Expected: ลงท้ายด้วย `050_retry_tick_5min.sql`. ถ้าเห็น `051` อยู่แล้ว → ใช้ `052` แทนทุกที่ใน plan นี้.

- [ ] **Step 2: เขียนไฟล์ migration**

สร้าง `migrations/051_content_brain_v2.sql`:

```sql
-- Content Brain v2: 10 categories, 7 archetypes, clip role, persona rotation, dedup hardening, insider KB
-- ทุกอย่าง additive. flag content_brain_v2_enabled (default false) คุม behavior ในโค้ด.

-- pg_trgm สำหรับ lexical dedup fallback
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- ===== topic_categories (10) =====
CREATE TABLE IF NOT EXISTS topic_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category_name TEXT UNIQUE NOT NULL,
    display_name TEXT NOT NULL,
    angle_instruction TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    weight INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO topic_categories (category_name, display_name, angle_instruction, weight) VALUES
('multi-account', 'บริหารหลายบัญชี/พอร์ต', 'เจาะการบริหารพอร์ตบัญชีหลายตัวพร้อมกัน: การแบ่งความเสี่ยง โครงสร้างพอร์ต การย้ายงบ การกันบัญชีติดกัน', 2),
('account-trust', 'Trust score / วอร์มบัญชี', 'เจาะ trust score ของบัญชี การวอร์มบัญชีใหม่ให้ปลอดภัย พฤติกรรมที่สร้างความน่าเชื่อถือเชิงโครงสร้าง', 2),
('bm-structure', 'โครงสร้าง BM / Portfolio', 'เจาะการออกแบบ Business Portfolio / BM ไม่ให้พังยกแผง: การแยก entity การจัดสรรสิทธิ์ การ backup admin', 1),
('ban-signals', 'สัญญาณเตือนก่อนแบน / ban wave', 'เจาะสัญญาณเตือนก่อนโดน限制 ช่วงที่แพลตฟอร์มกวาด การอ่าน notification และ restriction เชิงนโยบาย', 2),
('recovery', 'กู้บัญชี / appeal', 'เจาะเส้นทาง appeal จริง เอกสารที่ต้องใช้ อัตราการกลับมา และการวางแผนสำรองขณะรอ', 1),
('payment', 'ระบบจ่ายเงิน / บัตร', 'เจาะระบบการชำระของแพลตฟอร์ม: บัตรหลายใบ การกระจายวิธีจ่าย สาเหตุที่ถูกปฏิเสธ การวาง billing profile', 1),
('scaling', 'สเกลงบ', 'เจาะการขยายงบแต่ละช่วง: กำแพงที่เจอตอนยิงหนักขึ้น การคุม cost การเลื่อน spending limit', 1),
('creative', 'แอดเน่า / ครีเอทีฟตาย / CTR ร่วง', 'เจาะอาการ ad fatigue ครีเอทีฟที่เสื่อม CTR ร่วง และการรีเฟรชของคนยิงหนัก', 1),
('tracking', 'Pixel / Data / การวัดผล', 'เจาะการวัดผลของคนยิงหนัก: CAPI ความถูกต้องของ data ผลกระทบของ attribution ต่อการตัดสินใจ', 1),
('economics', 'ต้นทุน / ค่าธรรมเนียม / ROI', 'เจาะเศรษฐกิจของการยิงหนัก: ค่าธรรมเนียมที่คนภายนอกไม่เห็น ROI ต่อช่องทาง ต้นทุนซ่อนเร้น', 1)
ON CONFLICT (category_name) DO NOTHING;

-- ===== title_archetypes (7) =====
CREATE TABLE IF NOT EXISTS title_archetypes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    archetype_name TEXT UNIQUE NOT NULL,
    display_name TEXT NOT NULL,
    instruction TEXT NOT NULL DEFAULT '',
    example TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    weight INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO title_archetypes (archetype_name, display_name, instruction, example, weight) VALUES
('shock_number', 'ตัวเลขช็อก', 'เปิดด้วยตัวเลขที่ช็อก/เจ็บจี๊ด สื่อขนาดความเสียหายในระดับเงินหรือจำนวนบัญชี ใน 1 วรรคแรก', 'บัญชีตาย 40 ตัวในคืนเดียว เพราะพลาดจุดเดียว', 2),
('warning', 'เตือนภัย / ห้ามทำ', 'เปิดด้วยการเตือน "อย่าเพิ่ง..." หรือ "หยุดก่อน..." ชี้การกระทำที่อันตรายที่คนมักทำโดยไม่รู้ตัว', 'อย่าเพิ่งกดยืนยันตัวตน ถ้ายังไม่เช็ค 3 อย่างนี้', 2),
('myth_bust', 'แฉความเชื่อผิด', 'เปิดด้วยความเชื่อที่คนทำกันจนเป็นคำสั่ง แล้วบอกว่าเข้าใจผิด พร้อมเหตุผล', 'วอร์มบัญชี 7 วันแล้วรอด = เข้าใจผิด', 2),
('story_twist', 'เคสจริงพลิก', 'เปิดด้วยเคสเรียลของคนยิงหนัก/เอเจนซี่ ที่จบด้บิดพลิก (พังเพราะสาเหตุเล็ก)', 'เอเจนซี่งบวันละแสน พังเพราะบัตรใบเดียว', 2),
('question_tease', 'คำถามปลูกสงสัย', 'เปิดด้วยคำถาม "ทำไม..." ที่ปลูกสงสัยและสัญญาคำตอบเฉพาะกลุ่มคนที่เจอปัญหานั้นจริง', 'ทำไมบัญชีใหม่ยิงแล้วตายไว?', 2),
('checklist', 'เช็คลิสต์สัญญาณ', 'เปิดด้วย "N สัญญาณว่า..." เป็นรายการตรวจสอบที่คนดูเอาไปใช้ได้ทันทีกับบัญชีตัวเอง', '3 สัญญาณว่าบัญชีคุณกำลังจะโดนกวาด', 2),
('consult_qa', 'สูตรปรึกษา (เดิม)', 'รูปแบบคำปรึกษาแบบเดิม "คุณXครับ รบกวนปรึกษา..." เน้นความน่าเชื่อถือเหมือนที่ปรึกษาตอบ', 'คุณกฤษณ์ครับ รบกวนปรึกษาเรื่องบัญชีโดนแจ้งยืนยันตัวตน', 1)
ON CONFLICT (archetype_name) DO NOTHING;

-- ===== clips columns =====
ALTER TABLE clips ADD COLUMN IF NOT EXISTS clip_role TEXT NOT NULL DEFAULT '';
ALTER TABLE clips ADD COLUMN IF NOT EXISTS title_archetype TEXT NOT NULL DEFAULT '';
ALTER TABLE clips ADD COLUMN IF NOT EXISTS audience_persona TEXT NOT NULL DEFAULT '';

-- ===== topic_history.pain_point =====
ALTER TABLE topic_history ADD COLUMN IF NOT EXISTS pain_point TEXT NOT NULL DEFAULT '';

-- ===== rebalance content_formats: qa weight 2 -> 1 =====
UPDATE content_formats SET weight = 1 WHERE format_name = 'qa';

-- ===== settings =====
INSERT INTO settings (key, value) VALUES
('content_brain_v2_enabled', 'false'),
('clip_role_convert_ratio', '0.30'),
('dedup_threshold', '0.72'),
('pain_point_cooldown_days', '5')
ON CONFLICT (key) DO NOTHING;

-- audience_personas (JSON array of 4)
INSERT INTO settings (key, value) VALUES
('audience_personas', '["Media buyer ยิงหนัก งบวันละ 50k+ ถือหลายบัญชีพร้อมกัน กลัวพอร์ตพังยกแผง","เจ้าของธุรกิจออนไลน์ ถือ 3-10 บัญชี ทำเองทุกอย่าง เจ็บเรื่องบัญชีตาย/จ่ายเงินไม่ผ่านบ่อย","Agency ดูแลบัญชีลูกค้าหลายเจ้า รับผิดชอบงบคนอื่น พลาดไม่ได้","คนเพิ่งโดนแบนครั้งแรก กำลังหาทางกลับมายิงต่อ งง สับสน อยากได้คำตอบตรงๆ"]')
ON CONFLICT (key) DO NOTHING;

-- ===== prompt template UPDATE: question + script (insider voice + new placeholders) =====
-- ใช้ dollar-quoting $$ เพราะมี ' ในภาษาไทย
UPDATE agent_configs SET prompt_template = $q$
คุณคือผู้เชี่ยวชาญด้านการบริหารบัญชีโฆษณา Facebook จำนวนมากของ Ads Vance สร้างคำถาม {{.Count}} ข้อที่สะท้อน pain จริงของคนที่ถือหลายบัญชี/ยิงโฆษณาหนัก

หมวดหัวข้อ: {{.Category}}
{{.CategoryAngle}}

รูปแบบเนื้อหา (format): {{.FormatInstruction}}

รูปหัวข้อ/ hook (archetype): {{.ArchetypeInstruction}}

บทบาทคลิป: {{.RoleInstruction}}

กลุ่มเป้าหมาย: {{.AudiencePersona}}

ความรู้ที่เกี่ยวข้อง:
{{.RAGContext}}

หัวข้อที่เคยทำไปแล้ว (ห้ามซ้ำมุม/เนื้อหา):
{{.PreviousTopics}}

ชื่อผู้ปรึกษาที่เคยใช้ (ห้ามซ้ำชื่อ):
{{.PreviousNames}}

{{.TopicStats}}

กติกาเนื้อหา (hard rule):
- พูดเสียงคนวงใน ใช้ศัพท์ที่คนบริหารบัญชีจำนวนมากใช้จริง อ้างสถานการณ์เฉพาะคนยิงหนักเจอ
- ห้ามเนื้อหาระดับพื้นฐานทั่วไป (basic ads 101, สอนสมัครบัญชี, สอนยิงแอดครั้งแรก)
- ห้ามสอนวิธีหลบระบบตรวจจับ ปลอมตัวตน หรือทำผิดนโยบาย Facebook
- เล่า pain + การบริหารความเสี่ยงเชิงโครงสร้าง/การป้องกันเชิงนโยบายได้

ตอบกลับเป็น JSON array ของ object แต่ละข้อ:
- "question": คำถาม/หัวข้อคลิปภาษาไทย กระชับ ไม่เกิน 120 ตัวอักษร
- "questioner_name": ชื่อคนถามภาษาไทย (สมมุติ)
- "category": หมวดหัวข้อ
- "pain_point": ปัญหาหลักเป็นภาษาอังกฤษ snake_case เช่น "account_banned" "payment_declined" "ad_fatigue"
$q$
WHERE agent_name = 'question';

UPDATE agent_configs SET prompt_template = $q$
คุณคือนักเขียนสคริปต์วิดีโอ Ads Vance เขียนสคริปต์ตอบคำถามต่อไปนี้

คำถาม: {{.Question}}
ชื่อผู้ถาม: {{.QuestionerName}}
หมวด: {{.Category}}

รูปแบบเนื้อหา (format): {{.FormatInstruction}}

รูปหัวข้อ/ hook (archetype) ที่ใช้กับคลิปนี้: {{.ArchetypeInstruction}}

บทบาทคลิป: {{.RoleInstruction}}

กลุ่มเป้าหมาย: {{.AudiencePersona}}

ความรู้ที่เกี่ยวข้อง:
{{.RAGContext}}

เสียงคนวงใน: พูดเหมือนคนที่บริหารบัญชีจำนวนมากมาเอง ใช้ศัพท์ที่คนวงในใช้ อ้างสถานการณ์ที่เฉพาะคนยิงหนักเจอ ห้ามเนื้อหาระดับพื้นฐาน

กติกาเนื้อหา: ห้ามสอนหลบระบบตรวจจับ/ปลอมตัวตน/ทำผิดนโยบาย Facebook

ตอบกลับเป็น JSON:
- "youtube_title": หัวข้อคลิปภาษาไทย ตาม archetype ที่กำหนด ต้องลงท้ายด้วย " | Ads Vance" เสมอ ไม่เกิน 70 ตัวอักษร ห้ามเด็ดขาด: ใส่ URL, line id, @handle ใน youtube_title
- "youtube_description": คำอธิบายภาษาไทย 2-3 บรรทัด
- "youtube_tags": array ของแท็กภาษาไทย/อังกฤษ 5-8 ตัว
- "answer_script": สคริปต์คำตอบภาษาไทยเต็ม 300-500 คำ เป็นธรรมชาติพูดได้ จบด้วย CTA ตามบทบาทคลิป (reach = ชวนติดตาม/ดูคลิปต่อ; convert = ชวนทักแชท/ดูช่องทางใต้คลิปเรื่องบัญชีโฆษณา แบบ soft sell ไม่ใช่โฆษณาขายบัญชีทั้งคลิป)
- "voice_script": สคริปต์สำหรับ voiceover ภาษาไทย สั้นกว่า answer_script 150-300 คำ
$q$
WHERE agent_name = 'script';
```

- [ ] **Step 3: build + รัน migration ตรวจ syntax**

Run: `go build ./...`
Expected: ผ่าน (migration ยังไม่กระทบ Go code)

สำหรับตรวจ SQL syntax — deploy จะ auto-migrate แต่ตรวจที่ local ก่อนได้ด้วยการ push ไป Railway staging หรือใช้ Neon temp branch. อย่างน้อยตรวจด้วยตาว่าทุก statement ปิดด้วย `;` และ dollar-quote `$$` นั้น balance (เปิด-ปิด).

- [ ] **Step 4: commit**

```bash
git add migrations/051_content_brain_v2.sql
git commit -m "feat(content-brain-v2): migration — topic_categories, title_archetypes, clip role/persona columns, dedup settings, insider prompt templates"
```

---

## Task 2: Models + topic_categories/title_archetypes repo

**Files:**
- Modify: `internal/models/format.go`
- Create: `internal/repository/topics.go`
- Test: `internal/repository/topics_test.go`

**Interfaces:**
- Consumes: ท่า query จาก `internal/repository/formats.go` (join vs 7-day clips usage), `*pgxpool.Pool`
- Produces:
  - `models.TopicCategory{ ID, CategoryName, DisplayName, AngleInstruction string; Enabled bool; Weight int }`
  - `models.TitleArchetype{ ID, ArchetypeName, DisplayName, Instruction, Example string; Enabled bool; Weight int }`
  - `TopicCategoriesRepo.GetAll(ctx) ([]models.TopicCategory, error)`
  - `TopicCategoriesRepo.PickNext(ctx) (*models.TopicCategory, error)`
  - `TitleArchetypesRepo.GetAll(ctx) ([]models.TitleArchetype, error)`
  - `TitleArchetypesRepo.PickNext(ctx) (*models.TitleArchetype, error)`

- [ ] **Step 1: เพิ่ม struct ใน models/format.go**

Append ที่ท้าย `internal/models/format.go`:

```go
// TopicCategory — หมวดหัวข้อ v2 (map pain คนวงใน)
type TopicCategory struct {
	ID               string `json:"id"`
	CategoryName     string `json:"category_name"`
	DisplayName      string `json:"display_name"`
	AngleInstruction string `json:"angle_instruction"`
	Enabled          bool   `json:"enabled"`
	Weight           int    `json:"weight"`
}

// TitleArchetype — รูปหัวข้อ/hook
type TitleArchetype struct {
	ID             string `json:"id"`
	ArchetypeName  string `json:"archetype_name"`
	DisplayName    string `json:"display_name"`
	Instruction    string `json:"instruction"`
	Example        string `json:"example"`
	Enabled        bool   `json:"enabled"`
	Weight         int    `json:"weight"`
}
```

- [ ] **Step 2: เขียน failing test**

สร้าง `internal/repository/topics_test.go`:

```go
package repository

import (
	"context"
	"testing"
)

// PickNext ต้องคืน category ที่ใช้น้อยสุดใน 7 วัน (relative to weight)
func TestTopicCategoriesRepo_PickNext_LeastUsed(t *testing.T) {
	if testing.Short() {
		t.Skip("requires DB")
	}
	pool := testPool(t) // helper ที่มีอยู่ใน package repository test (ดู formats_test.go)
	repo := NewTopicCategoriesRepo(pool)
	ctx := context.Background()

	cat, err := repo.PickNext(ctx)
	if err != nil {
		t.Fatalf("PickNext: %v", err)
	}
	if cat == nil || cat.CategoryName == "" {
		t.Fatalf("expected non-empty category, got %+v", cat)
	}
	// enabled=false ต้องไม่ถูกเลือก
	if !cat.Enabled {
		t.Errorf("picked disabled category %s", cat.CategoryName)
	}
}

func TestTopicCategoriesRepo_GetAll(t *testing.T) {
	if testing.Short() {
		t.Skip("requires DB")
	}
	pool := testPool(t)
	repo := NewTopicCategoriesRepo(pool)
	ctx := context.Background()

	cats, err := repo.GetAll(ctx)
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(cats) < 10 {
		t.Errorf("expected >=10 seeded categories, got %d", len(cats))
	}
}

func TestTitleArchetypesRepo_PickNext(t *testing.T) {
	if testing.Short() {
		t.Skip("requires DB")
	}
	pool := testPool(t)
	repo := NewTitleArchetypesRepo(pool)
	ctx := context.Background()

	a, err := repo.PickNext(ctx)
	if err != nil {
		t.Fatalf("PickNext: %v", err)
	}
	if a == nil || a.ArchetypeName == "" {
		t.Fatalf("expected non-empty archetype, got %+v", a)
	}
}
```

หมายเหตุ: ถ้า package repository ไม่มี `testPool(t)` helper — ดูว่า `formats_test.go` ใช้ pattern อะไรยิง DB จริง แล้ว reuse (ถ้าไม่มี DB test เลย ให้สร้าง test แบบที่ mock pool ไม่ได้ ก็ skip แล้ว rely บน e2e ใน Task 9; แต่ก่อนสร้าง test ให้ `grep -l "testPool\|pgxpool.New" internal/repository/*_test.go` ก่อน)

- [ ] **Step 3: run test เพื่อ verify fail**

Run: `go test ./internal/repository/ -run TestTopicCategoriesRepo -v`
Expected: FAIL (compile error — `NewTopicCategoriesRepo` undefined)

- [ ] **Step 4: เขียน repo implementation**

สร้าง `internal/repository/topics.go`:

```go
package repository

import (
	"context"
	"sort"

	"github.com/jaochai/video-fb/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TopicCategoriesRepo struct {
	pool *pgxpool.Pool
}

func NewTopicCategoriesRepo(pool *pgxpool.Pool) *TopicCategoriesRepo {
	return &TopicCategoriesRepo{pool: pool}
}

// GetAll — คืนทุก category (เรียงตามชื่อ)
func (r *TopicCategoriesRepo) GetAll(ctx context.Context) ([]models.TopicCategory, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, category_name, display_name, angle_instruction, enabled, weight
		FROM topic_categories
		ORDER BY category_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.TopicCategory{}
	for rows.Next() {
		var c models.TopicCategory
		if err := rows.Scan(&c.ID, &c.CategoryName, &c.DisplayName, &c.AngleInstruction, &c.Enabled, &c.Weight); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// PickNext — least-used/7d + weight (reuse ท่า formats.PickNext)
func (r *TopicCategoriesRepo) PickNext(ctx context.Context) (*models.TopicCategory, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT tc.id, tc.category_name, tc.display_name, tc.angle_instruction, tc.enabled, tc.weight,
		       COALESCE(u.cnt, 0) AS used_count
		FROM topic_categories tc
		LEFT JOIN (
			SELECT category, COUNT(*) AS cnt
			FROM clips
			WHERE created_at > NOW() - INTERVAL '7 days'
			GROUP BY category
		) u ON u.category = tc.category_name
		WHERE tc.enabled = TRUE
		ORDER BY tc.category_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type usage struct {
		Cat       models.TopicCategory
		UsedCount int
	}
	usages := []usage{}
	for rows.Next() {
		var u usage
		if err := rows.Scan(&u.Cat.ID, &u.Cat.CategoryName, &u.Cat.DisplayName, &u.Cat.AngleInstruction, &u.Cat.Enabled, &u.Cat.Weight, &u.UsedCount); err != nil {
			return nil, err
		}
		usages = append(usages, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(usages) == 0 {
		return nil, nil
	}

	// least-used relative to weight (ท่าเดียวกับ formats.usageRatio)
	best := usages[0]
	bestRatio := ratio(best.UsedCount, best.Cat.Weight)
	for _, u := range usages[1:] {
		rr := ratio(u.UsedCount, u.Cat.Weight)
		if rr < bestRatio {
			best, bestRatio = u, rr
		}
	}
	return &best.Cat, nil
}

func ratio(used, weight int) float64 {
	w := weight
	if w < 1 {
		w = 1
	}
	return float64(used) / float64(w)
}

// sortedByCategory — helper (unused for now, kept for parity; remove if lint complains) -- จริงๆ ลบบรรทัดนี้ออก ไม่จำเป็น
var _ = sort.Strings

type TitleArchetypesRepo struct {
	pool *pgxpool.Pool
}

func NewTitleArchetypesRepo(pool *pgxpool.Pool) *TitleArchetypesRepo {
	return &TitleArchetypesRepo{pool: pool}
}

func (r *TitleArchetypesRepo) GetAll(ctx context.Context) ([]models.TitleArchetype, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, archetype_name, display_name, instruction, example, enabled, weight
		FROM title_archetypes
		ORDER BY archetype_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.TitleArchetype{}
	for rows.Next() {
		var a models.TitleArchetype
		if err := rows.Scan(&a.ID, &a.ArchetypeName, &a.DisplayName, &a.Instruction, &a.Example, &a.Enabled, &a.Weight); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// PickNext — least-used/7d + weight (นับจาก clips.title_archetype)
func (r *TitleArchetypesRepo) PickNext(ctx context.Context) (*models.TitleArchetype, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT ta.id, ta.archetype_name, ta.display_name, ta.instruction, ta.example, ta.enabled, ta.weight,
		       COALESCE(u.cnt, 0) AS used_count
		FROM title_archetypes ta
		LEFT JOIN (
			SELECT title_archetype, COUNT(*) AS cnt
			FROM clips
			WHERE created_at > NOW() - INTERVAL '7 days'
			  AND title_archetype <> ''
			GROUP BY title_archetype
		) u ON u.title_archetype = ta.archetype_name
		WHERE ta.enabled = TRUE
		ORDER BY ta.archetype_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type usage struct {
		Arch      models.TitleArchetype
		UsedCount int
	}
	usages := []usage{}
	for rows.Next() {
		var u usage
		if err := rows.Scan(&u.Arch.ID, &u.Arch.ArchetypeName, &u.Arch.DisplayName, &u.Arch.Instruction, &u.Arch.Example, &u.Arch.Enabled, &u.Arch.Weight, &u.UsedCount); err != nil {
			return nil, err
		}
		usages = append(usages, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(usages) == 0 {
		return nil, nil
	}
	best := usages[0]
	bestRatio := ratio(best.UsedCount, best.Arch.Weight)
	for _, u := range usages[1:] {
		rr := ratio(u.UsedCount, u.Arch.Weight)
		if rr < bestRatio {
			best, bestRatio = u, rr
		}
	}
	return &best.Arch, nil
}
```

ลบบรรทัด `var _ = sort.Strings` และ import `"sort"` ออก ถ้า Go lint บ่นว่าไม่ได้ใช้ (มันไม่จำเป็น — เก็บไว้ตอนแรกเผื่อ แต่ให้ลบออกจริง).

- [ ] **Step 5: run test verify pass**

Run: `go test ./internal/repository/ -run TestTopicCategoriesRepo -v`
Expected: PASS (ถ้า DB test skip ต้อง deploy/Neon verify แทน — ยอมรับได้)

Run: `go build ./...`
Expected: ผ่าน

- [ ] **Step 6: commit**

```bash
git add internal/models/format.go internal/repository/topics.go internal/repository/topics_test.go
git commit -m "feat(content-brain-v2): TopicCategory/TitleArchetype models + repos (least-used/7d+weight picker)"
```

---

## Task 3: Pickers ใน topic_pick.go (pure functions)

**Files:**
- Modify: `internal/orchestrator/topic_pick.go`
- Test: `internal/orchestrator/topic_pick_test.go`

**Interfaces:**
- Consumes: `models.TopicCategory`, `models.TitleArchetype` (Task 2)
- Produces (pure, testable):
  - `PickTopicCategory(all []models.TopicCategory, usedToday []string, rng *rand.Rand) models.TopicCategory`
  - `PickArchetype(all []models.TitleArchetype, rng *rand.Rand) models.TitleArchetype`
  - `PickClipRole(convertRatio float64, rng *rand.Rand) string` (คืน "reach" หรือ "convert")
  - `PickPersona(personas []string, rng *rand.Rand) string`

- [ ] **Step 1: เขียน failing test**

สร้าง `internal/orchestrator/topic_pick_test.go`:

```go
package orchestrator

import (
	"math/rand"
	"testing"

	"github.com/jaochai/video-fb/internal/models"
)

func TestPickTopicCategory_ExcludesUsedToday(t *testing.T) {
	all := []models.TopicCategory{
		{CategoryName: "multi-account", Enabled: true, Weight: 1},
		{CategoryName: "payment", Enabled: true, Weight: 1},
		{CategoryName: "scaling", Enabled: true, Weight: 1},
	}
	used := []string{"multi-account", "payment"}
	rng := rand.New(rand.NewSource(1))

	for i := 0; i < 20; i++ {
		got := PickTopicCategory(all, used, rng)
		if got.CategoryName == "multi-account" || got.CategoryName == "payment" {
			t.Errorf("picked used-today category %s", got.CategoryName)
		}
		if got.CategoryName == "" {
			t.Fatal("picked empty category")
		}
	}
}

func TestPickTopicCategory_LeastUsedPreferred(t *testing.T) {
	all := []models.TopicCategory{
		{CategoryName: "a", Enabled: true, Weight: 1},
		{CategoryName: "b", Enabled: true, Weight: 1},
	}
	rng := rand.New(rand.NewSource(1))
	// ทั้งคู่ใช้เท่ากัน (0) → สุ่มระหว่างสองตัว ไม่ panic
	got := PickTopicCategory(all, nil, rng)
	if got.CategoryName != "a" && got.CategoryName != "b" {
		t.Errorf("unexpected category %s", got.CategoryName)
	}
}

func TestPickArchetype_NonEmpty(t *testing.T) {
	all := []models.TitleArchetype{
		{ArchetypeName: "shock_number", Enabled: true, Weight: 2},
		{ArchetypeName: "warning", Enabled: true, Weight: 2},
	}
	rng := rand.New(rand.NewSource(2))
	got := PickArchetype(all, rng)
	if got.ArchetypeName == "" {
		t.Fatal("empty archetype")
	}
}

func TestPickClipRole_RatioDistribution(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	convert := 0
	for i := 0; i < 1000; i++ {
		if PickClipRole(0.30, rng) == "convert" {
			convert++
		}
	}
	// ~30% ± 5%
	if convert < 250 || convert > 350 {
		t.Errorf("convert ratio out of range: %d/1000", convert)
	}
}

func TestPickPersona_NonEmpty(t *testing.T) {
	personas := []string{"media buyer", "owner", "agency", "banned"}
	rng := rand.New(rand.NewSource(4))
	got := PickPersona(personas, rng)
	if got == "" {
		t.Fatal("empty persona")
	}
}
```

- [ ] **Step 2: run test verify fail**

Run: `go test ./internal/orchestrator/ -run TestPick -v`
Expected: FAIL (undefined: PickTopicCategory ฯลฯ)

- [ ] **Step 3: เขียน implementation**

Append ที่ท้าย `internal/orchestrator/topic_pick.go`:

```go
import (
	"math/rand"
	"sort"

	"github.com/jaochai/video-fb/internal/models"
)

// PickTopicCategory — least-used (relative to weight) บน enabled categories,
// และห้ามซ้ำ category ที่ใช้ในวันเดียวกัน (usedToday).
// ถ้าทุก category ถูกใช้ในวันนี้แล้ว → คืน least-used ทั้งหมด (ยกเลิกกฎวัน)
func PickTopicCategory(all []models.TopicCategory, usedToday []string, rng *rand.Rand) models.TopicCategory {
	used := map[string]bool{}
	for _, u := range usedToday {
		used[u] = true
	}
	available := []models.TopicCategory{}
	for _, c := range all {
		if !c.Enabled {
			continue
		}
		if used[c.CategoryName] {
			continue
		}
		available = append(available, c)
	}
	if len(available) == 0 {
		// fallback: enabled ทั้งหมด (ยกเลิกกฎห้ามซ้ำวัน)
		for _, c := range all {
			if c.Enabled {
				available = append(available, c)
			}
		}
	}
	if len(available) == 0 {
		return models.TopicCategory{}
	}
	// least-used relative to weight: ทุกตัวใช้น้อย = ratio เท่ากัน → สุ่ม
	// เพื่อให้ simple และ deterministic-testable: sort stable แล้วสุ่มในกลุ่ม min ratio
	// ที่นี่เราไม่มี usage count (repo จัดการแล้วใน PickNext) → สุ่มตาม weight
	return weightedPickCategory(available, rng)
}

func weightedPickCategory(cs []models.TopicCategory, rng *rand.Rand) models.TopicCategory {
	total := 0
	for _, c := range cs {
		w := c.Weight
		if w < 1 {
			w = 1
		}
		total += w
	}
	if total == 0 {
		return cs[0]
	}
	pick := rng.Intn(total)
	for _, c := range cs {
		w := c.Weight
		if w < 1 {
			w = 1
		}
		pick -= w
		if pick < 0 {
			return c
		}
	}
	return cs[len(cs)-1]
}

// PickArchetype — weighted random (repo PickNext จัด least-used แล้ว; ที่นี่ใช้เมื่อ repo ไม่ available)
func PickArchetype(all []models.TitleArchetype, rng *rand.Rand) models.TitleArchetype {
	enabled := []models.TitleArchetype{}
	for _, a := range all {
		if a.Enabled {
			enabled = append(enabled, a)
		}
	}
	if len(enabled) == 0 {
		return models.TitleArchetype{}
	}
	total := 0
	for _, a := range enabled {
		w := a.Weight
		if w < 1 {
			w = 1
		}
		total += w
	}
	pick := rng.Intn(total)
	for _, a := range enabled {
		w := a.Weight
		if w < 1 {
			w = 1
		}
		pick -= w
		if pick < 0 {
			return a
		}
	}
	return enabled[len(enabled)-1]
}

// PickClipRole — "reach" (1-ratio) / "convert" (ratio)
func PickClipRole(convertRatio float64, rng *rand.Rand) string {
	if convertRatio <= 0 {
		return "reach"
	}
	if convertRatio >= 1 {
		return "convert"
	}
	if rng.Float64() < convertRatio {
		return "convert"
	}
	return "reach"
}

// PickPersona — สุ่ม 1 จาก personas
func PickPersona(personas []string, rng *rand.Rand) string {
	if len(personas) == 0 {
		return ""
	}
	return personas[rng.Intn(len(personas))]
}

// sortStrings เก็บไว้กัน unused import (ใช้จริงเมื่อมี caller) -- ลบถ้าไม่จำเป็น
var _ = sort.Strings
```

ลบ `var _ = sort.Strings` กับ import `"sort"` ถ้าไม่ได้ใช้จริง (ออกแบบให้ repo PickNext ทำ least-used แล้ว เลยไม่ต้อง sort ที่นี่). **ตรวจให้แน่ใจว่าไม่มี unused import ตอน build.**

- [ ] **Step 4: run test verify pass**

Run: `go test ./internal/orchestrator/ -run TestPick -v`
Expected: PASS (ทั้ง 5 test)

Run: `go build ./...`
Expected: ผ่าน

- [ ] **Step 5: commit**

```bash
git add internal/orchestrator/topic_pick.go internal/orchestrator/topic_pick_test.go
git commit -m "feat(content-brain-v2): pure pickers — topic category (no-dup-per-day), archetype, clip role ratio, persona"
```

---

## Task 4: QuestionAgent — template args ใหม่ + persist pain_point

**Files:**
- Modify: `internal/agent/question.go`
- Test: `internal/agent/question_test.go`

**Interfaces:**
- Consumes: template engine `renderTemplate` (reflection over struct), `topic_history` insert (inline), `GeneratedQuestion.PainPoint` (มีแล้ว line 46)
- Produces:
  - `QuestionTemplateData` เพิ่ม fields: `CategoryAngle`, `ArchetypeInstruction`, `RoleInstruction`, `TopicStats`
  - `QuestionAgent.Generate` signature เพิ่ม params: `categoryAngle string`, `archetypeInstr string`, `roleInstr string`
  - `topic_history` insert เขียน `pain_point` column

- [ ] **Step 1: เขียน failing test**

Append `internal/agent/question_test.go` (ถ้าไฟล์มีอยู่แล้ว; ถ้าไม่มี สร้างใหม่ `package agent`):

```go
package agent

import "testing"

// renderTemplate ต้องแทน {{.ArchetypeInstruction}} {{.RoleInstruction}} {{.CategoryAngle}} {{.TopicStats}}
func TestQuestionTemplateData_NewFieldsRender(t *testing.T) {
	td := QuestionTemplateData{
		Count: 3, Category: "multi-account",
		ArchetypeInstruction: "ARCHX", RoleInstruction: "ROLEX",
		CategoryAngle: "ANGLEX", TopicStats: "STATSX",
	}
	out := renderTemplate("a {{.ArchetypeInstruction}} b {{.RoleInstruction}} c {{.CategoryAngle}} d {{.TopicStats}}", td)
	want := "a ARCHX b ROLEX c ANGLEX d STATSX"
	if out != want {
		t.Errorf("render mismatch:\n got: %s\nwant: %s", out, want)
	}
}
```

- [ ] **Step 2: run test verify fail**

Run: `go test ./internal/agent/ -run TestQuestionTemplateData_NewFieldsRender -v`
Expected: FAIL (compile error — unknown fields ArchetypeInstruction/RoleInstruction/CategoryAngle/TopicStats ใน QuestionTemplateData)

- [ ] **Step 3: แก้ QuestionTemplateData + Generate signature + topic_history insert**

ใน `internal/agent/question.go`:

แก้ struct `QuestionTemplateData` (ประมาณบรรทัด 20-28) ให้เป็น:

```go
type QuestionTemplateData struct {
	Count                int
	Category             string
	CategoryAngle        string
	ArchetypeInstruction string
	RoleInstruction      string
	TopicStats           string
	RAGContext           string
	PreviousTopics       string
	PreviousNames        string
	FormatInstruction    string
	AudiencePersona      string
}
```

แก้ signature `Generate` (บรรทัด 49) จาก:

```go
func (a *QuestionAgent) Generate(ctx context.Context, count int, category string, format *models.ContentFormat, persona string, topicStats string, cfg *models.AgentConfig) ([]GeneratedQuestion, error)
```

เป็น:

```go
func (a *QuestionAgent) Generate(ctx context.Context, count int, category string, categoryAngle string, format *models.ContentFormat, persona string, archetypeInstr string, roleInstr string, topicStats string, cfg *models.AgentConfig) ([]GeneratedQuestion, error)
```

ใน body ของ Generate ตรงที่สร้าง `QuestionTemplateData` (ดูว่าประมาณบรรทัด 100-120) ให้เติม fields ใหม่:

```go
td := QuestionTemplateData{
	Count:                count,
	Category:             category,
	CategoryAngle:        categoryAngle,
	ArchetypeInstruction: archetypeInstr,
	RoleInstruction:      roleInstr,
	TopicStats:           topicStats,
	RAGContext:           ragCtx,
	PreviousTopics:       prevTopics,
	PreviousNames:        prevNames,
	FormatInstruction:    format.QuestionInstruction,
	AudiencePersona:      persona,
}
```

(ปรับชื่อตัวแปรท้องถิ่นให้ตรงกับที่มีจริงในไฟล์ — อ่าน context รอบๆ ก่อนแก้.)

**สำคัญ — เลิก append topicStats หลัง render:** ถ้าปัจจุบัน topicStats ถูก append หลัง render (บรรทัด ~127) ให้ลบบรรทัดนั้นทิ้ง เพราะตอนนี้มันเป็น field ใน template แล้ว (`{{.TopicStats}}`). ค้นหาสตริงที่ append topicStats แล้วลบ.

**แก้ topic_history insert** (บรรทัด ~186-193) — เพิ่ม pain_point. แก้ SQL insert ทั้งสอง variant (มี embedding / ไม่มี):

variant มี embedding:
```go
_, err = a.pool.Exec(ctx,
	`INSERT INTO topic_history (title, category, pain_point, embedding)
	 VALUES ($1, $2, $3, $4::vector)`,
	q.Question, q.Category, q.PainPoint, rag.FormatVector(emb))
```

variant ไม่มี embedding:
```go
_, err = a.pool.Exec(ctx,
	`INSERT INTO topic_history (title, category, pain_point)
	 VALUES ($1, $2, $3)`,
	q.Question, q.Category, q.PainPoint)
```

(อ่านบริบศรอบๆ insert จริงในไฟล์ก่อน — อาจมีใน loop แยก; เก็บโครงสร้างเดิมไว้ แค่เพิ่ม column + value.)

- [ ] **Step 4: run test verify pass**

Run: `go test ./internal/agent/ -run TestQuestionTemplateData_NewFieldsRender -v`
Expected: PASS

Run: `go build ./...`
Expected: FAIL ที่ orchestrator.go (caller Generate ยังส่ง args เดิม) — **คาดว่าจะ fail ตรงนี้, จะแก้ใน Task 7**. ถ้าแกะ caller อื่นที่เรียก Generate มีเฉพาะ orchestrator ก็ OK. ถ้ามี test เรียก Generate เก่า → แก้ test ด้วยหรือ mark skip.

- [ ] **Step 5: commit (ทั้งที่ orchestrator build break ชั่วคราว — จะแก้ Task 7)**

```bash
git add internal/agent/question.go internal/agent/question_test.go
git commit -m "feat(content-brain-v2): QuestionAgent — archetype/role/angle template args + persist pain_point to topic_history"
```

หมายเหตุ: ถ้า repo บังคับว่าทุก commit ต้อง build ผ่าน → อย่า commit ตอนนี้ รวม Task 4+5+7 ค่อย commit. แต่ปกติใช้แนวทางนี้ได้เพราะแต่ละ task มี context ของตัวเอง.

---

## Task 5: ScriptAgent — template args (archetype title + role CTA)

**Files:**
- Modify: `internal/agent/script.go`
- Test: `internal/agent/script_test.go`

**Interfaces:**
- Consumes: template engine, prompt template ใน DB (มี `{{.ArchetypeInstruction}}` `{{.RoleInstruction}}` แล้วจาก Task 1)
- Produces:
  - `ScriptTemplateData` เพิ่ม fields: `ArchetypeInstruction`, `RoleInstruction`
  - `ScriptAgent.Generate` signature เพิ่ม params: `archetypeInstr string`, `roleInstr string`

- [ ] **Step 1: เขียน failing test**

Append `internal/agent/script_test.go`:

```go
package agent

import "testing"

func TestScriptTemplateData_NewFieldsRender(t *testing.T) {
	td := ScriptTemplateData{
		Question: "Q", ArchetypeInstruction: "ARCH", RoleInstruction: "ROLE",
	}
	out := renderTemplate("h {{.ArchetypeInstruction}} i {{.RoleInstruction}}", td)
	want := "h ARCH i ROLE"
	if out != want {
		t.Errorf("render mismatch:\n got: %s\nwant: %s", out, want)
	}
}
```

- [ ] **Step 2: run test verify fail**

Run: `go test ./internal/agent/ -run TestScriptTemplateData_NewFieldsRender -v`
Expected: FAIL (unknown fields)

- [ ] **Step 3: แก้ ScriptTemplateData + Generate signature**

ใน `internal/agent/script.go`:

แก้ struct `ScriptTemplateData` (บรรทัด 14-21) ให้:

```go
type ScriptTemplateData struct {
	Question             string
	QuestionerName       string
	Category             string
	ArchetypeInstruction string
	RoleInstruction      string
	RAGContext           string
	FormatInstruction    string
	AudiencePersona      string
}
```

แก้ signature `Generate` (บรรทัด 61) จาก:

```go
func (a *ScriptAgent) Generate(ctx context.Context, question, questionerName, category string, format *models.ContentFormat, persona string, cfg *models.AgentConfig) (*GeneratedScript, error)
```

เป็น:

```go
func (a *ScriptAgent) Generate(ctx context.Context, question, questionerName, category string, format *models.ContentFormat, persona string, archetypeInstr string, roleInstr string, cfg *models.AgentConfig) (*GeneratedScript, error)
```

ใน body ตอนสร้าง `ScriptTemplateData` ให้เติม:

```go
td := ScriptTemplateData{
	Question:             question,
	QuestionerName:       questionerName,
	Category:             category,
	ArchetypeInstruction: archetypeInstr,
	RoleInstruction:      roleInstr,
	RAGContext:           ragCtx,
	FormatInstruction:    format.ScriptInstruction,
	AudiencePersona:      persona,
}
```

- [ ] **Step 4: run test verify pass**

Run: `go test ./internal/agent/ -run TestScriptTemplateData_NewFieldsRender -v`
Expected: PASS

- [ ] **Step 5: commit**

```bash
git add internal/agent/script.go internal/agent/script_test.go
git commit -m "feat(content-brain-v2): ScriptAgent — archetype/role template args (title + CTA control)"
```

---

## Task 6: Dedup hardening — threshold from setting + pain_point cooldown + lexical fallback

**Files:**
- Modify: `internal/agent/dedup.go`
- Modify: `internal/agent/question.go` (fail-open → retry → lexical)
- Test: `internal/agent/dedup_test.go`

**Interfaces:**
- Consumes: settings `dedup_threshold` (0.72), `pain_point_cooldown_days` (5); `topic_history.pain_point`; `pg_trgm similarity()`
- Produces:
  - `Deduper` มี `threshold float64` field (set ตอน construct หรือ per-call)
  - `Deduper.CheckQuestions` ใช้ threshold field นั้น
  - `Deduper.PainPointInCooldown(ctx, painPoint string, days int) (bool, error)` — query `topic_history WHERE pain_point = $1 AND created_at > NOW() - make_interval(days => $2)`
  - `Deduper.LexicalCheck(ctx, questions []GeneratedQuestion) (map[string]bool, error)` — `pg_trgm similarity()` เทียบ 30 title ล่าสุด, block ถ้า > 0.5
  - `question.go`: embed ล่ม → retry 1 → lexical guard → log warning

- [ ] **Step 1: เขียน failing test**

Append `internal/agent/dedup_test.go`:

```go
package agent

import "testing"

// filterBySimilarity ใช้ threshold ที่ส่งเข้า (ไม่ hardcode 0.78 แล้ว)
func TestFilterBySimilarity_CustomThreshold(t *testing.T) {
	questions := []GeneratedQuestion{
		{Question: "q1"},
		{Question: "q2"},
	}
	sims := map[string]SimilarityMatch{
		"q1": {Similarity: 0.75, MatchedTitle: "old"},
		"q2": {Similarity: 0.60, MatchedTitle: "old2"},
	}
	// threshold 0.72 → q1 (0.75) ต้องถูก reject, q2 (0.60) ผ่าน
	passed, rejected := filterBySimilarity(questions, sims, 0.72)
	if len(passed) != 1 || passed[0].Question != "q2" {
		t.Errorf("expected q2 to pass at threshold 0.72, got passed=%+v", passed)
	}
	if len(rejected) != 1 || rejected[0].Question.Question != "q1" {
		t.Errorf("expected q1 rejected at 0.72, got rejected=%+v", rejected)
	}
}
```

- [ ] **Step 2: run test verify fail**

Run: `go test ./internal/agent/ -run TestFilterBySimilarity_CustomThreshold -v`
Expected: FAIL (filterBySimilarity signature ยังไม่รับ threshold)

- [ ] **Step 3: แก้ dedup.go**

ใน `internal/agent/dedup.go`:

ลบ `const similarityThreshold = 0.78` (บรรทัด 16) แล้วใส่เป็น field ใน `Deduper` struct:

```go
type Deduper struct {
	llm    *KieLLMClient
	pool   *pgxpool.Pool
	rag    *rag.Engine
	threshold float64 // default 0.72; set จาก setting
}
```

(ดู `Deduper` struct จริงในไฟล์ก่อน — เพิ่ม field `threshold float64`.)

constructor `NewDeduper` — เพิ่ม param `threshold float64`:

```go
func NewDeduper(llm *KieLLMClient, pool *pgxpool.Pool, ragEngine *rag.Engine, threshold float64) *Deduper {
	if threshold <= 0 {
		threshold = 0.72
	}
	return &Deduper{llm: llm, pool: pool, rag: ragEngine, threshold: threshold}
}
```

(อ่าน constructor เดิมก่อน — เติม param โดยรักษา field เดิม.)

แก้ `filterBySimilarity` (บรรทัด 30-40) ให้รับ threshold:

```go
func filterBySimilarity(questions []GeneratedQuestion, sims map[string]SimilarityMatch, threshold float64) (passed []GeneratedQuestion, rejected []rejectedQuestion) {
	for _, q := range questions {
		m, ok := sims[q.Question]
		if ok && m.Similarity >= threshold {
			rejected = append(rejected, rejectedQuestion{Question: q, Match: m})
			continue
		}
		passed = append(passed, q)
	}
	return passed, rejected
}
```

ใน `CheckQuestions` (บรรทัด 54) เปลี่ยนการเรียก `filterBySimilarity(...)` ให้ส่ง `d.threshold`:

```go
passed, rejected := filterBySimilarity(questions, sims, d.threshold)
```

(อ่านจุดเรียกจริงใน CheckQuestions — ปรับให้ตรง.)

**เพิ่ม pain_point cooldown** — append ที่ท้าย dedup.go:

```go
// PainPointInCooldown — true ถ้า pain_point นี้เคยปรากฏใน N วันล่าสุด
func (d *Deduper) PainPointInCooldown(ctx context.Context, painPoint string, days int) (bool, error) {
	if painPoint == "" || days <= 0 {
		return false, nil
	}
	var n int
	err := d.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM topic_history
		 WHERE pain_point = $1 AND created_at > NOW() - make_interval(days => $2)`,
		painPoint, days).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// LexicalCheck — fallback เมื่อ embedding ล่ม ใช้ pg_trgm similarity() เทียบ 30 title ล่าสุด
// คืน map[question]block (true = ซ้ำ > 0.5)
func (d *Deduper) LexicalCheck(ctx context.Context, questions []GeneratedQuestion) (map[string]bool, error) {
	rows, err := d.pool.Query(ctx,
		`SELECT title FROM topic_history ORDER BY created_at DESC LIMIT 30`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	past := []string{}
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		past = append(past, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := map[string]bool{}
	for _, q := range questions {
		for _, p := range past {
			var sim float64
			err := d.pool.QueryRow(ctx,
				`SELECT similarity($1, $2)`, q.Question, p).Scan(&sim)
			if err != nil {
				continue // ข้าม pair ที่ error ไม่ block มั่ว
			}
			if sim > 0.5 {
				out[q.Question] = true
				break
			}
		}
	}
	return out, nil
}
```

- [ ] **Step 4: แก้ question.go — fail-closed มีทางหนี**

ใน `internal/agent/question.go` ตรง dedup fail-open block (บรรทัด ~140-146) เปลี่ยนจาก:

```go
similarities, embeddings, err := a.deduper.CheckQuestions(ctx, questions)
if err != nil {
	log.Printf("QuestionAgent: dedup check failed, accepting without dedup: %v", err)
	accepted = append(accepted, questions...)
	break
}
```

เป็น (retry 1 ครั้ง → lexical guard):

```go
similarities, embeddings, err := a.deduper.CheckQuestions(ctx, questions)
if err != nil {
	// embedding ล่ม → retry 1 ครั้ง
	log.Printf("QuestionAgent: dedup embedding error, retrying: %v", err)
	time.Sleep(500 * time.Millisecond)
	similarities, embeddings, err = a.deduper.CheckQuestions(ctx, questions)
}
if err != nil {
	// ยังล่ม → lexical guard (pg_trgm) แทน ห้ามรับมั่ว
	log.Printf("QuestionAgent: dedup still failing, using lexical fallback: %v", err)
	blocked, lexErr := a.deduper.LexicalCheck(ctx, questions)
	if lexErr != nil {
		log.Printf("QuestionAgent: lexical fallback also failed, accepting all: %v", lexErr)
		accepted = append(accepted, questions...)
		break
	}
	for _, q := range questions {
		if !blocked[q.Question] {
			accepted = append(accepted, q)
		}
	}
	break
}
```

เพิ่ม import `"time"` ถ้ายังไม่มี.

**เพิ่ม pain_point cooldown** — หลังจากได้ `accepted` และก่อน insert topic_history ให้กรองต่อ:

```go
// pain_point cooldown (เฉพาะ flag on — caller ส่ง cooldownDays; 0 = skip)
if a.painCooldownDays > 0 {
	filtered := accepted[:0]
	for _, q := range accepted {
		inCD, err := a.deduper.PainPointInCooldown(ctx, q.PainPoint, a.painCooldownDays)
		if err != nil {
			log.Printf("QuestionAgent: pain_point cooldown check error: %v", err)
			filtered = append(filtered, q) // fail-open สำหรับ cooldown
			continue
		}
		if !inCD {
			filtered = append(filtered, q)
		}
	}
	accepted = filtered
}
```

เพิ่ม field `painCooldownDays int` ใน `QuestionAgent` struct + constructor param (ค่าจาก setting `pain_point_cooldown_days` — ส่งจาก orchestrator ใน Task 7).

- [ ] **Step 5: run tests verify pass**

Run: `go test ./internal/agent/ -run TestFilterBySimilarity_CustomThreshold -v`
Expected: PASS

Run: `go build ./...`
Expected: อาจยัง break ที่ orchestrator (caller ของ NewDeduper / NewQuestionAgent) — แก้ใน Task 7.

- [ ] **Step 6: commit**

```bash
git add internal/agent/dedup.go internal/agent/dedup_test.go internal/agent/question.go
git commit -m "feat(content-brain-v2): dedup hardening — threshold from setting + pain_point cooldown + lexical fallback (fail-closed w/ escape)"
```

---

## Task 7: Wire orchestrator (ProduceWeekly) + clip columns

**Files:**
- Modify: `internal/models/request.go` (CreateClipRequest)
- Modify: `internal/repository/clips.go` (Create/Update)
- Modify: `internal/orchestrator/orchestrator.go`
- Test: `internal/orchestrator/orchestrator_test.go`

**Interfaces:**
- Consumes: Task 2 (repos), Task 3 (pickers), Task 4/5/6 (agent signatures + Deduper threshold + QuestionAgent cooldown)
- Produces: flag-gated pipeline ที่ใช้ category 10, archetype, role 70/30, persona rotation, news fallback→least-used, persist clip_role/title_archetype/audience_persona

- [ ] **Step 1: แก้ CreateClipRequest + clips repo**

ใน `internal/models/request.go` เพิ่ม fields:

```go
type CreateClipRequest struct {
	Title          string  `json:"title"`
	Question       string  `json:"question"`
	QuestionerName string  `json:"questioner_name"`
	Category       string  `json:"category"`
	PublishDate    *string `json:"publish_date"`
	ContentFormat  string  `json:"content_format"`
	ClipRole       string  `json:"clip_role"`
	TitleArchetype string  `json:"title_archetype"`
	AudiencePersona string `json:"audience_persona"`
}
```

ใน `internal/repository/clips.go` — แก้ `Create` INSERT (บรรทัด ~67) ให้ส่ง columns ใหม่. อ่าน INSERT statement จริงก่อน แล้วเพิ่ม:

```sql
INSERT INTO clips (title, question, questioner_name, category, publish_date, content_format, clip_role, title_archetype, audience_persona, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'pending')
```

(ปรับตาม INSERT จริงในไฟล์ — เติม 3 columns + values ต่อท้าย; อย่าลืม RETURNING id หรือ Scan ตามเดิม.)

- [ ] **Step 2: เพิ่ม repos + fields ใน Orchestrator struct**

ใน `internal/orchestrator/orchestrator.go` แก้ `Orchestrator` struct เพิ่ม:

```go
topicCategoriesRepo *repository.TopicCategoriesRepo
titleArchetypesRepo *repository.TitleArchetypesRepo
```

ใน constructor `NewOrchestrator` เพิ่ม params + assignment (อ่าน constructor เดิมก่อน). เพิ่มอ่าน settings สำหรับ threshold/cooldown ตอน construct `Deduper` และ `QuestionAgent` — หรืออ่านตอน ProduceWeekly:

เพราะ settings เปลี่ยนได้แบบ hot (ไม่ deploy) → อ่านใน `ProduceWeekly` ดีกว่า construct-time. ดังนั้น:
- `Deduper` threshold: อ่านใน ProduceWeekly → สร้าง Deduper ใหม่ด้วย threshold จาก setting? ไม่ ideal. **ทางเลือก:** เพิ่ม method `Deduper.SetThreshold(float64)` แล้วเรียกใน ProduceWeekly เมื่อ flag on.

เพิ่มใน `Deduper`:
```go
func (d *Deduper) SetThreshold(t float64) {
	if t > 0 {
		d.threshold = t
	}
}
```

เพิ่มใน `QuestionAgent`:
```go
func (a *QuestionAgent) SetPainCooldownDays(days int) { a.painCooldownDays = days }
```

- [ ] **Step 3: แก้ ProduceWeekly — flag branch + new picks + caller signatures**

ใน `ProduceWeekly` (เริ่มบรรทัด 131 หลัง kie pre-flight) เพิ่มการอ่าน flag แล้วแยก branch:

```go
// content brain v2 flag
v2Raw, _ := o.settingsRepo.Get(ctx, "content_brain_v2_enabled")
v2 := v2Raw == "true"

var category string
var categoryAngle string
var topicStats string

if v2 {
	// ---- v2: topic_categories least-used + ห้ามซ้ำในวัน ----
	// หมวดที่ใช้ในวันนี้
	usedToday, _ := o.clipsRepo.CategoriesUsedToday(ctx) // เพิ่ม method (Step 4)
	tcat, err := o.topicCategoriesRepo.PickNext(ctx)
	if err != nil || tcat == nil {
		log.Printf("Orchestrator: topic_categories pick failed, falling back to legacy: %v", err)
		// fallback legacy
		cats, _ := o.settingsRepo.GetCategories(ctx)
		if len(cats) > 0 {
			category = cats[int(time.Now().Unix()/(7*24*3600))%len(cats)]
		}
	} else {
		allCats, _ := o.topicCategoriesRepo.GetAll(ctx)
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		picked := PickTopicCategory(allCats, usedToday, rng)
		if picked.CategoryName != "" {
			tcat = &picked
		}
		category = tcat.CategoryName
		categoryAngle = tcat.AngleInstruction
	}

	// archetype
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	archAll, _ := o.titleArchetypesRepo.GetAll(ctx)
	archetype := PickArchetype(archAll, rng)
	// (ถ้า repo.PickNext ต้องการ least-used จริง — ใช้ o.titleArchetypesRepo.PickNext(ctx) แทน)

	// role 70/30
	ratioStr, _ := o.settingsRepo.Get(ctx, "clip_role_convert_ratio")
	ratio, _ := strconv.ParseFloat(ratioStr, 64)
	if ratio == 0 {
		ratio = 0.30
	}
	role := PickClipRole(ratio, rng)

	// persona rotation
	personasJSON, _ := o.settingsRepo.Get(ctx, "audience_personas")
	personas := []string{}
	_ = json.Unmarshal([]byte(personasJSON), &personas)
	persona := ""
	if len(personas) > 0 {
		persona = PickPersona(personas, rng)
	} else {
		persona, _ = o.settingsRepo.Get(ctx, "audience_persona")
	}

	// dedup threshold + cooldown จาก setting
	threshStr, _ := o.settingsRepo.Get(ctx, "dedup_threshold")
	if t, err := strconv.ParseFloat(threshStr, 64); err == nil {
		o.questionAgent.SetPainCooldownDaysLookup(...) // หรือเก็บไว้ส่งต่อ
		o.deduper.SetThreshold(t)
	}
	cdStr, _ := o.settingsRepo.Get(ctx, "pain_point_cooldown_days")
	cdDays, _ := strconv.Atoi(cdStr)
	o.questionAgent.SetPainCooldownDays(cdDays)

	// topic stats ยังแนบ (ข้อมูลประกอบ) แม้ไม่บังคับทิศ
	scores, _ := o.analyticsRepo.TopicPerformance(ctx, 30, 3)
	topicStats = FormatTopicStats(scores)

	// เก็บ archetype/role ไว้ใช้ตอน produceClip
	o.currentArchetype = archetype
	o.currentRole = role
	o.currentAngle = categoryAngle
	o.currentPersona = persona
} else {
	// ---- legacy (เดิมทุกบรรทัด) ----
	weekNum := int(time.Now().Unix() / (7 * 24 * 3600))
	categories, _ := o.settingsRepo.GetCategories(ctx)
	category = categories[weekNum%len(categories)]
	... // (โค้ดเดิม PickCategoryWeighted + FormatTopicStats)
}
```

หมายเหตุ: เก็บ archetype/role/angle/persona "per-clip" ไม่ควรเป็น struct field เพราะ ProduceWeekly ผลิตหลาย clip. **ทางที่ดี:** ส่งค่าเหล่านี้ผ่าน `produceClip` params หรือ context. ปรับ signature `produceClip` ให้รับ `archetype models.TitleArchetype, role, angle, persona string`. แล้วใน per-question loop ส่งค่าเดียวกันทั้งหมด (เพราะ 1 ProduceWeekly = 1 batch ที่ใช้ picks เดียวกัน — ดู spec: "สุ่มต่อคลิป" แต่ในทางปฏิบัติ picks ทำครั้งเดียวต่อ batch; ถ้าต้องการ per-clip จริง ย้าย pick เข้าใน loop. **เลือก per-batch ง่ายกว่าและตรง "ห้ามซ้ำในวัน"** แต่ role/persona ควร per-clip — ย้าย role+persona pick เข้าใน per-question loop.)

**ปรับโครงให้แม่นยำ:** category/format/archetype = per-batch (once per ProduceWeekly); role/persona = per-clip (ใน per-question loop).

แก้ caller `questionAgent.Generate` (บรรทัด ~191) เป็น:

```go
questions, err := o.questionAgent.Generate(ctx, count, category, categoryAngle, format, persona, archetype.Instruction, role, topicStats, qaCfg)
```

สำหรับ role/persona per-clip: ย้าย role/persona pick เข้าใน per-question loop (แต่ Generate ทำครั้งเดียวสำหรับทั้ง batch → conflict). **วิธีแก้:** Generate ใช้ placeholder role/persona (ว่างหรือ "reach"/default); แล้วตอน produceClip แต่ละ clip สุ่ม role/persona ใหม่แล้ว override ใน question ก่อนเรียก ScriptAgent. แต่ ScriptAgent ใช้ question ที่ QuestionAgent สร้างไว้แล้ว → role/persona ส่งตรงที่ ScriptAgent.Generate.

**สรุปทางเลือกที่สะอาดที่สุด:**
- QuestionAgent.Generate: รับ archetype (per-batch), ใช้ role ที่ "representative" หรือว่าง สำหรับ framing คำถาม. จริงๆ role มีผลที่ CTA (ScriptAgent) มากกว่า. เลย **QuestionAgent รับ role แค่เพื่อ framing** (reach = คำถาม broad, convert = คำถามเจาะ).
- สุ่ม role/persona per-clip ใน per-question loop; แต่ละ question ใช้ role/persona ของตัวเอง.

อ่านโครงสร้าง per-question loop (228-244) จริง แล้วย้าย role/persona pick เข้าไป ส่งเข้า produceClip:

```go
for _, q := range questions {
	if ctx.Err() != nil { break }
	role := role // per-batch default
	persona := persona
	if v2 {
		role = PickClipRole(ratio, rng)
		personaBatch = PickPersona(personas, rng)
	}
	if err := o.produceClip(ctx, q, format, archetype, role, angle, persona, v2); err != nil {
		log.Printf("produceClip error: %v", err)
		anyFailed = true
	}
}
```

(ปรับ `produceClip` signature ให้รับค่าเหล่านี้ — อ่าน `produceClip` เดิมก่อน.)

ใน `produceClip`/`produceClipWithID`:
- แก้ `clipsRepo.Create` ให้ส่ง ClipRole/TitleArchetype/AudiencePersona
- แก้ `scriptAgent.Generate` (บรรทัด 353) ให้ส่ง archetype.Instruction + role:

```go
script, err := o.scriptAgent.Generate(ctx, q.Question, q.QuestionerName, q.Category, format, persona, archetype.Instruction, role, scriptCfg)
```

- [ ] **Step 4: เพิ่ม clipsRepo.CategoriesUsedToday + update news fallback**

ใน `internal/repository/clips.go` เพิ่ม:

```go
// CategoriesUsedToday — หมวดที่สร้างคลิปในวันนี้ (UTC) เพื่อกันซ้ำในวัน
func (r *ClipsRepo) CategoriesUsedToday(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT category FROM clips
		 WHERE created_at > NOW() - INTERVAL '24 hours' AND category <> ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
```

**แก้ news → qa fallback** (บรรทัด 192-201): เปลี่ยนจาก `GetByName(ctx, "qa")` เป็น least-used format:

```go
if errors.Is(err, agent.ErrNoFreshNews) {
	// v2: fallback เป็น least-used format (ไม่ fix qa); legacy: qa เดิม
	if v2 {
		format, err = o.formatsRepo.PickNext(ctx)
		if err != nil {
			format, _ = o.formatsRepo.GetByName(ctx, "qa") // last resort
		}
	} else {
		format, err = o.formatsRepo.GetByName(ctx, "qa")
	}
	questions, err = o.questionAgent.Generate(ctx, count, category, categoryAngle, format, persona, archetype.Instruction, role, topicStats, qaCfg)
}
```

- [ ] **Step 5: เขียน/อัปเดต orchestrator test**

ใน `internal/orchestrator/orchestrator_test.go` เพิ่ม smoke test ว่า flag off = legacy path ไม่พัง (ถ้ามี test setup อยู่แล้ว reuse; ถ้าไม่มี DB test ใน package orchestrator ให้ skip + rely บน e2e Task 9):

```go
func TestProduceWeekly_FlagOff_LegacyPath(t *testing.T) {
	if testing.Short() {
		t.Skip("requires DB + LLM")
	}
	// verify flag off ไม่ panic + ใช้ settings.categories (legacy)
	// (detail depends on existing test harness — อย่างน้อยตรวจว่า flag gate compile ผ่าน)
}
```

- [ ] **Step 6: build + run tests**

Run: `go build ./...`
Expected: ผ่าน (ทุก caller แก้แล้ว)

Run: `go test ./internal/orchestrator/ ./internal/agent/ ./internal/repository/ -short -v`
Expected: PASS (unit/pure tests; DB tests skip)

- [ ] **Step 7: commit**

```bash
git add internal/models/request.go internal/repository/clips.go internal/orchestrator/orchestrator.go internal/orchestrator/topic_pick.go internal/agent/*.go
git commit -m "feat(content-brain-v2): wire flag-gated pipeline in ProduceWeekly — 10 categories, archetype, role 70/30, persona rotation, news fallback→least-used, persist clip role/archetype/persona"
```

---

## Task 8: Insider KB pack ingest

**Files:**
- Create: `scripts/insider_kb_content/01_multi_account.txt` ... `10_economics.txt` (10 ไฟล์)
- Create: `scripts/ingest_insider_kb.sh`

**Interfaces:**
- Consumes: KB API `POST /api/v1/knowledge/sources` (body `{name, category, content}`) + `POST /api/v1/knowledge/sources/{id}/embed`
- Produces: ~10 sources ใหม่ใน `knowledge_sources` + embedded chunks ใน `knowledge_chunks`

- [ ] **Step 1: เขียน content 10 sources**

สร้างโฟลเดอร์ `scripts/insider_kb_content/` และ 10 ไฟล์ `.txt` (1 ต่อ category). แต่ละไฟล์ ~400-800 คำ เนื้อหา: pain scenarios + ศัพท์ + สถานการณ์จริงของหมวดนั้น. **กติกา hard rule (จาก spec §3.6):** เล่า pain + การบริหารความเสี่ยงเชิงโครงสร้างได้; **ห้าม** สอนหลบระบบตรวจจับ/ปลอมตัวตน/ทำผิดนโยบาย.

ตัวอย่างโครง `scripts/insider_kb_content/01_multi_account.txt`:

```
หมวด: บริหารหลายบัญชี/พอร์ต

Pain scenarios ของคนถือหลายบัญชี:
- พอร์ตพังยกแผงเพราะบัญชีติดกัน (shared signal: บัตรใบเดียวกัน, IP, device, payment profile)
- บัญชีใหม่ในพอร์ตตายไวเพราะไม่ได้วอร์มก่อนยิงหนัก
- ย้ายงบระหว่างบัญชีแล้ว trigger การตรวจสอบ

โครงสร้างพอร์ตที่กระจายความเสี่ยง (เชิงนโยบาย):
- แยก entity ทางธุรกิจ (portfolio per entity) ไม่ใช่บัญชีเป๊ะ แต่การแยกข้อมูลตัวตน/การเงิน
- กระจายวิธีชำระ (billing profile ต่างกัน) ไม่ใช้บัตร/ธนาคารซ้ำข้ามพอร์ต
- backup admin access แยกคน ไม่รวมคนเดียวถือทั้งพอร์ต
- การวาง ad account ตามขนาดงบช่วง (spending limit tier) เพื่อกระจาย exposure

ศัพท์ที่คนวงในใช้:
พอร์ต, ad account, spending limit, billing profile, BM (Business Portfolio), entity, warm-up, trust score

ข้อควรระวัง (เชิงการป้องกัน):
- อย่าย้ายงบกะทันหันข้ามบัญชี — เพิ่มทีละน้อย (ramp)
- บัญชีใหม่ต้องสร้างพฤติกรรมปกติก่อน (วอร์ม) ก่อนยิงงบเต็ม
```

(เขียนครบ 10 ไฟล์ตาม category_name ใน migration — multi-account, account-trust, bm-structure, ban-signals, recovery, payment, scaling, creative, tracking, economics. ทุกไฟล์ตามโครง: pain scenarios + โครงสร้าง/การบริหาร + ศัพท์ + ข้อควรระวัง. ห้ามละเมิด guardrail.)

- [ ] **Step 2: เขียน ingest script**

สร้าง `scripts/ingest_insider_kb.sh`:

```bash
#!/usr/bin/env bash
# Ingest insider KB pack เข้า Ads Vance ผ่าน KB API
# ใช้: BASE_URL=... API_TOKEN=... ./scripts/ingest_insider_kb.sh
set -euo pipefail

: "${BASE_URL:?need BASE_URL e.g. https://adsvance-v2.up.railway.app}"
: "${API_TOKEN:?need API_TOKEN}"

DIR="$(dirname "$0")/insider_kb_content"
shopt -s nullglob
files=("$DIR"/*.txt)

for f in "${files[@]}"; do
  name="insider-$(basename "$f" .txt)"
  category=$(basename "$f" .txt | sed 's/^[0-9]*_//')
  content=$(cat "$f")
  echo "==> ingesting $name (category=$category)"

  resp=$(curl -sS -X POST "$BASE_URL/api/v1/knowledge/sources" \
    -H "Authorization: Bearer $API_TOKEN" \
    -H "Content-Type: application/json" \
    -d "$(jq -n --arg n "$name" --arg c "$category" --arg ct "$content" '{name:$n, category:$c, content:$ct}')")

  id=$(echo "$resp" | jq -r '.id // empty')
  if [ -z "$id" ]; then
    echo "  FAIL: no id in response: $resp" >&2
    continue
  fi
  echo "  created source $id, embedding..."
  curl -sS -X POST "$BASE_URL/api/v1/knowledge/sources/$id/embed" \
    -H "Authorization: Bearer $API_TOKEN" | jq -r '.chunks // "embedded"'
done

echo "done. rollback: ลบ sources ที่ name LIKE '"'"'insider-%'"'"' ผ่าน DELETE /api/v1/knowledge/sources/{id}"
```

chmod: `chmod +x scripts/ingest_insider_kb.sh`

หมายเหตุ: ตรวจว่า KB API ต้องการ auth header อะไรจริง (`grep -n "Authorization\|middleware" internal/router/router.go internal/handler/knowledge.go`). ถ้าไม่มี auth ก็ลบบรรทัด Authorization.

- [ ] **Step 3: build + verify script syntax**

Run: `go build ./... && bash -n scripts/ingest_insider_kb.sh`
Expected: ผ่าน (syntax OK; ยังไม่รันจริง — รันหลัง deploy ใน Task 9)

- [ ] **Step 4: commit**

```bash
git add scripts/ingest_insider_kb.sh scripts/insider_kb_content/
git commit -m "feat(content-brain-v2): insider KB pack (10 sources) + ingest script"
```

---

## Task 9: Deploy + end-to-end verify on flag

**Files:** none (ops + manual eyeball)

- [ ] **Step 1: deploy ขึ้น prod (push master)**

push master → Railway auto-deploy + auto-migrate (migration 051 รันตอน boot). ตรวจ Railway logs ว่า migration applied + server start OK.

- [ ] **Step 2: ingest insider KB**

```bash
BASE_URL=<prod-url> API_TOKEN=<token> ./scripts/ingest_insider_kb.sh
```

ตรวจ: `GET /api/v1/knowledge/sources` เห็น sources ที่ name เริ่มด้วย `insider-` และ chunks count > 0.

- [ ] **Step 3: flip flag + trigger 1 clip**

flip flag (Neon `run_sql`):
```sql
UPDATE settings SET value='true' WHERE key='content_brain_v2_enabled';
```

trigger produce (1 clip เพื่อทดสอบ — ใช้ endpoint /orchestrator/produce หรือรอ schedule):
```bash
curl -X POST <prod>/orchestrator/produce?count=1 -H "Authorization: Bearer <token>"
```

- [ ] **Step 4: eyeball clip ผลลัพธ์**

ตรวจคลิปที่ผลิตได้:
- title ไม่ใช่สูตร "คุณXครับ" (เว้น archetype = consult_qa)
- CTA ตรง role (reach = ชวนติดตาม; convert = ชวนทักแชท)
- clip_role / title_archetype / audience_persona ถูกบันทึกใน clips (query Neon)
- render ผ่าน (สถานะ ready ไม่ใช่ failed/needs_review)

Run query verify:
```sql
SELECT title, clip_role, title_archetype, audience_persona, content_format, status
FROM clips ORDER BY created_at DESC LIMIT 3;
```

- [ ] **Step 5: เกณฑ์วัดหลัง 3 วัน (9 คลิป)**

(เก็บไว้ดูทีหลัง — ไม่ใช่ gate ตอน implement):
- ชื่อคลิปขึ้นต้น "คุณXครับ" ≤ 2/9
- ครอบคลุม ≥ 5 หมวด, ≥ 4 archetypes
- role split 60/40–80/20
- ไม่มีคู่หัวข้อที่ "เรื่องเดียวกัน" (eyeball)

ถ้าไม่ผ่าน → tune setting (ratio/threshold/cooldown) โดยไม่ deploy; ถ้าพัง → `content_brain_v2_enabled=false` rollback ทันที.

- [ ] **Step 6: อัปเดต memory**

หลัง verify ผ่าน → อัปเดต/สร้าง memory file `project_content_brain_v2.md` บันทึก: flag, migration 051, success criteria, gotchas.

---

## Self-Review (plan author)

**Spec coverage check (ทุกส่วนของ spec §3):**
- §3.1 หมวด 10 + picker per-clip ห้ามซ้ำในวัน → Task 1 (table+seed) + Task 2 (repo) + Task 3 (PickTopicCategory) + Task 7 (wire) ✓
- §3.2 role reach/convert 70/30 → Task 1 (settings) + Task 3 (PickClipRole) + Task 4/5 (prompt) + Task 7 (wire per-clip) ✓
- §3.3 title archetypes 7 + rebalance qa weight + news fallback → Task 1 (table+seed+weight UPDATE) + Task 2 (repo) + Task 3 (PickArchetype) + Task 4/5 (prompt) + Task 7 (news fallback) ✓
- §3.4 persona 4 → Task 1 (settings JSON) + Task 3 (PickPersona) + Task 4/5 (prompt) + Task 7 (wire) ✓
- §3.5 dedup hardening (threshold 0.72 + pain_point cooldown + lexical fallback) → Task 1 (pg_trgm + settings + column) + Task 6 ✓
- §3.6 insider KB pack + insider voice → Task 1 (prompt template UPDATE) + Task 8 (KB ingest) ✓
- §3.7 out of scope (performance loop, UI queue, visual pipeline, auto-ingest news) → ไม่มี task = ✓

**Placeholder scan:** ไม่มี TBD/TODO. บาง step บอก "อ่านไฟล์จริงก่อนแก้" เพราะตำแหน่งบรรทัดอาจเลื่อน — นี่คือคำสั่งตรวจสอบ ไม่ใช่ placeholder เพราะมี code ตัวอย่างครบ.

**Type consistency:** `models.TopicCategory` / `models.TitleArchetype` ใช้ชื่อเดียวกันทุก task. `PickTopicCategory` / `PickArchetype` / `PickClipRole` / `PickPersona` ชื่อตรงทั้งหมด. `Deduper.SetThreshold` / `QuestionAgent.SetPainCooldownDays` ชื่อตรง.

**จุดที่ต้องระวังตอน implement:**
1. Generate signature ของ QuestionAgent/ScriptAgent เปลี่ยน → caller ทุกตัว (orchestrator + test เก่า) ต้องแก้. Task 4/5 อาจ break build ชั่วคราว — commit รวม Task 7 ถ้า repo บังคับ green build.
2. `Deduper` constructor เพิ่ม param threshold → caller ของ NewDeduper (cmd/server/main.go) ต้องแก้ — เพิ่มใน Task 6 หรือ 7.
3. `QuestionAgent` constructor เพิ่ม param painCooldownDays → caller (main.go) ต้องแก้ — Task 6/7.
4. migration prompt template ใช้ `$$` dollar-quoting — ตรวจ balance ทุกครั้ง.
5. KB API auth header — ตรวจ router.go จริงใน Task 8.
