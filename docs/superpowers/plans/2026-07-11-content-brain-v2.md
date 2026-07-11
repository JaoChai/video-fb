# Content Brain v2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** แก้ยอดวิวต่ำ + content วนซ้ำ โดยขยายหมวดเป็น 10, เพิ่มบทบาท reach/convert 70/30, 7 title archetypes, 4 personas, harden dedup, และเติม insider KB pack — ทั้งหมดหลัง flag `content_brain_v2_enabled` (default off)

**Architecture:** additive overhaul ของ content-picking layer. ทุกตาราง/คอลัมน์ใหม่เป็น additive. Picker logic ใหม่ reuse ท่า least-used/7d+weight ของ `formats.PickNext`. flag off = พฤติกรรมเดิมทุก path (rollback ทันที). pipeline wiring กระจุกอยู่ที่ `orchestrator.ProduceWeekly`.

**Tech Stack:** Go (net/http + pgx/v5 + pgvector), Neon Postgres, Railway auto-migrate on boot, ไม่มี ORM (raw SQL ใน repository layer)

## การตัดสินใจสถาปัตยกรรม (lock — เปลี่ยนไม่ได้ระหว่าง implement)

**A. Picks ทำ per-batch ครั้งเดียวใน `ProduceWeekly`, ส่งผ่าน params.**
scheduler จริงยิง `ProduceWeekly(ctx, 1)` = 1 clip ต่อครั้ง (`scheduler.go:157`) ดังนั้น per-batch **=** per-clip ในการรันจริง. การ manual produce `count>1` (handler/`-produce` flag) จะใช้ picks เดียวกันทั้ง batch — ยอมรับได้เพราะ "ห้ามซ้ำในวัน" ทำผ่าน SQL exclude อยู่แล้ว และ role/persona ระบบเฉลี่ยเองในระยะยาว. **ห้ามใช้ struct field** (`o.currentArchetype` ฯลฯ) เก็บ picks — ส่งผ่าน function params เท่านั้น.

**B. Picker ทางเดียว — repo `PickNextExclude(ctx, excludeToday []string)`** ทำ least-used/7d + weight + exclude หมวดที่ใช้วันนี้ ใน SQL ชุดเดียว (ท่าเดียวกับ `formats.PickNext`). ไม่มี pure function แยกสำหรับ category/archetype (จะแข่งกับ repo). Pure functions เฉพาะ `PickClipRole` + `PickPersona` (ไม่ใช้ DB).

**C. ไม่แตะ constructor signature.** threshold/cooldown ส่งผ่าน setter method (`Deduper.SetThreshold(float64)`, `QuestionAgent.SetPainCooldownDays(int)`) ที่ orchestrator เรียกใน `ProduceWeekly` ตอน flag on. `main.go` ไม่ต้องแก้เลย — ลด blast radius.

**D. `Generate` signature ของ QuestionAgent/ScriptAgent เปลี่ยน** → caller ใน orchestrator แก้ 2 จุด (บรรทัด 191 + 200 news fallback) + แก้ `produceClip`/`produceClipWithID` chain (ส่ง archetype+role ผ่าน).

## Global Constraints

- **Flag gate:** ทุก behavior ใหม่ต้องเช็ค `settings.content_brain_v2_enabled == "true"`; false = เดิมทุก path. อ่าน flag ผ่าน `settingsRepo.Get(ctx, "content_brain_v2_enabled")` แล้ว string-compare `== "true"` (ค่า default ว่าง/ไม่มี = false = พฤติกรรมเดิม).
- **Migration numbering:** master ปัจจุบันล่าสุด = `050_retry_tick_5min.sql`. Migration ใหม่ใช้ `051`. **ถ้า PR #17 (migration 051 two-strike) merge เข้า master ก่อน implement** → bump เลข migration ทั้งหมด +1. ตรวจ `ls migrations/ | tail -3` ก่อนสร้างไฟล์ทุกครั้ง.
- **Migration style:** plain `.sql`, idempotent (`IF NOT EXISTS`, `ON CONFLICT DO NOTHING`), ทุก statement ในไฟล์เดียวรันเป็น `pool.Exec` เดียว (multi-statement). ไม่มี down-migration.
- **SQL quoting:** Thai text ใน UPDATE prompt_template ใช้ dollar-quoting `$$...$$` (มี `'` ในภาษาไทยได้). ตรวจทุก `$$` เปิด-ปิด balance.
- **Template engine:** custom replacer (`internal/agent/template.go`) ไม่ใช่ `text/template` — แทน `{{.FieldName}}` literal ตามชื่อ field ของ struct (reflection). เพิ่ม arg = เพิ่ม field ใน struct + ใส่ `{{.FieldName}}` ใน prompt template ใน DB.
- **Settings read pattern:** `settingsRepo.Get(ctx, key) (string, error)`. number = parse string ด้วย `strconv.ParseFloat`/`ParseInt` (fail → default). JSON = `json.Unmarshal`.
- **filterBySimilarity มี param `threshold` อยู่แล้ว** (`dedup.go:30`) — ไม่ต้องเพิ่ม; แค่เปลี่ยนตัวที่ส่งเข้าจาก const เป็น field.
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
- `internal/orchestrator/topic_pick.go` — pure pickers PickClipRole + PickPersona (Task 3)
- `internal/orchestrator/topic_pick_test.go` — (Task 3)
- `internal/agent/question.go` — Generate signature + template data + persist pain_point + dedup fail-closed + SetPainCooldownDays (Task 4, 6)
- `internal/agent/question_test.go` — (Task 4)
- `internal/agent/script.go` — Generate signature + template data (Task 5)
- `internal/agent/script_test.go` — (Task 5)
- `internal/agent/dedup.go` — const→field threshold + SetThreshold + pain_point cooldown + lexical fallback (Task 6)
- `internal/agent/dedup_test.go` — (Task 6)
- `internal/orchestrator/orchestrator.go` — wire pickers + flag gate + news fallback + persist columns + thread params (Task 7)
- `internal/repository/clips.go` — Create INSERT columns ใหม่ + CategoriesUsedToday (Task 7)

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
('ban-signals', 'สัญญาณเตือนก่อนแบน / ban wave', 'เจาะสัญญาณเตือนก่อนโดนจำกัด ช่วงที่แพลตฟอร์มกวาด การอ่าน notification และ restriction เชิงนโยบาย', 2),
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
('warning', 'เตือนภัย / ห้ามทำ', 'เปิดด้วยการเตือน อย่าเพิ่ง... หรือ หยุดก่อน... ชี้การกระทำที่อันตรายที่คนมักทำโดยไม่รู้ตัว', 'อย่าเพิ่งกดยืนยันตัวตน ถ้ายังไม่เช็ค 3 อย่างนี้', 2),
('myth_bust', 'แฉความเชื่อผิด', 'เปิดด้วยความเชื่อที่คนทำกันจนเป็นคำสั่ง แล้วบอกว่าเข้าใจผิด พร้อมเหตุผล', 'วอร์มบัญชี 7 วันแล้วรอด = เข้าใจผิด', 2),
('story_twist', 'เคสจริงพลิก', 'เปิดด้วยเคสเรียลของคนยิงหนัก/เอเจนซี่ ที่จบด้วยบิดพลิก (พังเพราะสาเหตุเล็ก)', 'เอเจนซี่งบวันละแสน พังเพราะบัตรใบเดียว', 2),
('question_tease', 'คำถามปลูกสงสัย', 'เปิดด้วยคำถาม ทำไม... ที่ปลูกสงสัยและสัญญาคำตอบเฉพาะกลุ่มคนที่เจอปัญหานั้นจริง', 'ทำไมบัญชีใหม่ยิงแล้วตายไว', 2),
('checklist', 'เช็คลิสต์สัญญาณ', 'เปิดด้วย N สัญญาณว่า... เป็นรายการตรวจสอบที่คนดูเอาไปใช้ได้ทันทีกับบัญชีตัวเอง', '3 สัญญาณว่าบัญชีคุณกำลังจะโดนกวาด', 2),
('consult_qa', 'สูตรปรึกษา (เดิม)', 'รูปแบบคำปรึกษาแบบเดิม คุณXครับ รบกวนปรึกษา... เน้นความน่าเชื่อถือเหมือนที่ปรึกษาตอบ', 'คุณกฤษณ์ครับ รบกวนปรึกษาเรื่องบัญชีโดนแจ้งยืนยันตัวตน', 1)
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
-- ใช้ dollar-quoting $q$ ... $q$ เพราะมี ' ในภาษาไทย
UPDATE agent_configs SET prompt_template = $q$
คุณคือผู้เชี่ยวชาญด้านการบริหารบัญชีโฆษณา Facebook จำนวนมากของ Ads Vance สร้างคำถาม {{.Count}} ข้อที่สะท้อน pain จริงของคนที่ถือหลายบัญชี/ยิงโฆษณาหนัก

หมวดหัวข้อ: {{.Category}}
{{.CategoryAngle}}

รูปแบบเนื้อหา (format): {{.FormatInstruction}}

รูปหัวข้อ/hook (archetype): {{.ArchetypeInstruction}}

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
- "pain_point": ปัญหาหลักเป็นภาษาอังกฤษ snake_case เช่น account_banned payment_declined ad_fatigue
$q$
WHERE agent_name = 'question';

UPDATE agent_configs SET prompt_template = $q$
คุณคือนักเขียนสคริปต์วิดีโอ Ads Vance เขียนสคริปต์ตอบคำถามต่อไปนี้

คำถาม: {{.Question}}
ชื่อผู้ถาม: {{.QuestionerName}}
หมวด: {{.Category}}

รูปแบบเนื้อหา (format): {{.FormatInstruction}}

รูปหัวข้อ/hook (archetype) ที่ใช้กับคลิปนี้: {{.ArchetypeInstruction}}

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

- [ ] **Step 3: build + ตรวจ SQL syntax**

Run: `go build ./...`
Expected: ผ่าน (migration ยังไม่กระทบ Go code)

ตรวจด้วยตา: ทุก statement ปิดด้วย `;` และ dollar-quote `$q$` ทั้ง 2 บล็อก (question + script) เปิด-ปิดครบ (4 ตัว `$q$`).

- [ ] **Step 4: commit**

```bash
git add migrations/051_content_brain_v2.sql
git commit -m "feat(content-brain-v2): migration — topic_categories, title_archetypes, clip role/persona columns, dedup settings, insider prompt templates"
```

---

## Task 2: Models + topic_categories/title_archetypes repo (single picker path)

**Files:**
- Modify: `internal/models/format.go`
- Create: `internal/repository/topics.go`
- Test: `internal/repository/topics_test.go`

**Interfaces:**
- Consumes: ท่า query จาก `internal/repository/formats.go` (LEFT JOIN vs 7-day clips usage), `*pgxpool.Pool`, `usageRatio` pattern (inline ไม่ reuse ข้าม file)
- Produces:
  - `models.TopicCategory{ ID, CategoryName, DisplayName, AngleInstruction string; Enabled bool; Weight int }`
  - `models.TitleArchetype{ ID, ArchetypeName, DisplayName, Instruction, Example string; Enabled bool; Weight int }`
  - `TopicCategoriesRepo.GetAll(ctx) ([]models.TopicCategory, error)`
  - `TopicCategoriesRepo.PickNextExclude(ctx, excludeToday []string) (*models.TopicCategory, error)` — least-used/7d + weight + exclude category ที่ใช้ใน 24h ช่วงล่าสุด
  - `TitleArchetypesRepo.GetAll(ctx) ([]models.TitleArchetype, error)`
  - `TitleArchetypesRepo.PickNext(ctx) (*models.TitleArchetype, error)` — least-used/7d + weight จาก `clips.title_archetype`

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
	ID            string `json:"id"`
	ArchetypeName string `json:"archetype_name"`
	DisplayName   string `json:"display_name"`
	Instruction   string `json:"instruction"`
	Example       string `json:"example"`
	Enabled       bool   `json:"enabled"`
	Weight        int    `json:"weight"`
}
```

- [ ] **Step 2: ตรวจ test harness ที่มีใน package repository**

Run: `grep -l "pgxpool.New\|testPool\|TestMain" internal/repository/*_test.go`
ถ้ามี helper สร้าง pool — จดชื่อฟังก์ชันไว้ใช้ใน Step 3. ถ้าไม่มีเลย → test ใน Step 3 ใช้ `t.Skip` + rely บน e2e (Task 9).

- [ ] **Step 3: เขียน failing test**

สร้าง `internal/repository/topics_test.go` (ถ้า package repository test ใช้ build tag `//go:build integration` หรือ `testing.Short()` ให้ทำตาม pattern เดิม — ดู formats_test.go ก่อน):

```go
package repository

import (
	"context"
	"testing"
)

func TestTopicCategoriesRepo_GetAll_Seeded(t *testing.T) {
	pool := testPool(t) // ใช้ helper เดียวกับ formats_test.go; ถ้าไม่มี → t.Skip("requires DB")
	if pool == nil {
		t.Skip("requires DB")
	}
	repo := NewTopicCategoriesRepo(pool)
	cats, err := repo.GetAll(context.Background())
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(cats) < 10 {
		t.Errorf("expected >=10 seeded categories, got %d", len(cats))
	}
}

func TestTopicCategoriesRepo_PickNextExclude_ExcludesToday(t *testing.T) {
	pool := testPool(t)
	if pool == nil {
		t.Skip("requires DB")
	}
	repo := NewTopicCategoriesRepo(pool)
	// exclude ทุกหมวดยกเว้น 1 → ต้องคืนตัวนั้น
	all, _ := repo.GetAll(context.Background())
	if len(all) == 0 {
		t.Skip("no categories seeded")
	}
	exclude := []string{}
	keep := all[0].CategoryName
	for _, c := range all {
		if c.CategoryName != keep {
			exclude = append(exclude, c.CategoryName)
		}
	}
	got, err := repo.PickNextExclude(context.Background(), exclude)
	if err != nil {
		t.Fatalf("PickNextExclude: %v", err)
	}
	if got == nil || got.CategoryName != keep {
		t.Errorf("expected %s, got %+v", keep, got)
	}
}

func TestTitleArchetypesRepo_PickNext(t *testing.T) {
	pool := testPool(t)
	if pool == nil {
		t.Skip("requires DB")
	}
	repo := NewTitleArchetypesRepo(pool)
	got, err := repo.PickNext(context.Background())
	if err != nil {
		t.Fatalf("PickNext: %v", err)
	}
	if got == nil || got.ArchetypeName == "" {
		t.Fatalf("expected non-empty archetype, got %+v", got)
	}
}
```

หมายเหตุ: ถ้า `testPool` ไม่มีจริง → เปลี่ยนบรรทัดแรกของแต่ละ test เป็น `t.Skip("requires DB")` แล้วลบ `pool := testPool(t)`. unit test จริงๆ ของ feature นี้คือ pure logic ที่ Test Task 3.

- [ ] **Step 4: run test verify fail**

Run: `go test ./internal/repository/ -run TestTopicCategories -v`
Expected: FAIL (compile error — `NewTopicCategoriesRepo` undefined, `testPool` อาจ undefined ด้วย → แก้ตามจริงใน Step 5)

- [ ] **Step 5: เขียน repo implementation**

สร้าง `internal/repository/topics.go`:

```go
package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
)

type TopicCategoriesRepo struct {
	pool *pgxpool.Pool
}

func NewTopicCategoriesRepo(pool *pgxpool.Pool) *TopicCategoriesRepo {
	return &TopicCategoriesRepo{pool: pool}
}

// GetAll — ทุก category เรียงตามชื่อ
func (r *TopicCategoriesRepo) GetAll(ctx context.Context) ([]models.TopicCategory, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, category_name, display_name, angle_instruction, enabled, weight
		FROM topic_categories
		ORDER BY category_name`)
	if err != nil {
		return nil, fmt.Errorf("query topic_categories: %w", err)
	}
	defer rows.Close()
	out := []models.TopicCategory{}
	for rows.Next() {
		var c models.TopicCategory
		if err := rows.Scan(&c.ID, &c.CategoryName, &c.DisplayName, &c.AngleInstruction, &c.Enabled, &c.Weight); err != nil {
			return nil, fmt.Errorf("scan topic_category: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// PickNextExclude — least-used/7d + weight (ท่า formats.PickNext) และ exclude category ที่ใช้ใน 24h ล่าสุด
// (กันซ้ำในวันเดียวกัน). excludeToday empty = ไม่ exclude.
func (r *TopicCategoriesRepo) PickNextExclude(ctx context.Context, excludeToday []string) (*models.TopicCategory, error) {
	// กรอง exclude ฝั่ง Go (ง่าย + param-safe) — ดึง usage ทั้งหมดก่อน
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
		return nil, fmt.Errorf("query topic category usage: %w", err)
	}
	defer rows.Close()

	type usage struct {
		Cat       models.TopicCategory
		UsedCount int
	}
	exclude := map[string]bool{}
	for _, e := range excludeToday {
		exclude[e] = true
	}
	usages := []usage{}
	usagesNoExclude := []usage{}
	for rows.Next() {
		var u usage
		if err := rows.Scan(&u.Cat.ID, &u.Cat.CategoryName, &u.Cat.DisplayName, &u.Cat.AngleInstruction, &u.Cat.Enabled, &u.Cat.Weight, &u.UsedCount); err != nil {
			return nil, fmt.Errorf("scan topic category usage: %w", err)
		}
		usagesNoExclude = append(usagesNoExclude, u)
		if exclude[u.Cat.CategoryName] {
			continue
		}
		usages = append(usages, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// ถ้า exclude หมด → ยกเลิกกฎวัน (fallback ใช้ทั้งหมด)
	pool := usages
	if len(pool) == 0 {
		pool = usagesNoExclude
	}
	if len(pool) == 0 {
		return nil, nil
	}

	best := pool[0]
	bestRatio := catUsageRatio(best.UsedCount, best.Cat.Weight)
	for _, u := range pool[1:] {
		if r2 := catUsageRatio(u.UsedCount, u.Cat.Weight); r2 < bestRatio {
			best, bestRatio = u, r2
		}
	}
	return &best.Cat, nil
}

// catUsageRatio — private helper คำนวณ used/weight (เหมือน formats.usageRatio แต่ scope เฉพาะ file นี้ ชื่อไม่ชน)
func catUsageRatio(used, weight int) float64 {
	w := weight
	if w < 1 {
		w = 1
	}
	return float64(used) / float64(w)
}

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
		return nil, fmt.Errorf("query title_archetypes: %w", err)
	}
	defer rows.Close()
	out := []models.TitleArchetype{}
	for rows.Next() {
		var a models.TitleArchetype
		if err := rows.Scan(&a.ID, &a.ArchetypeName, &a.DisplayName, &a.Instruction, &a.Example, &a.Enabled, &a.Weight); err != nil {
			return nil, fmt.Errorf("scan title_archetype: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// PickNext — least-used/7d + weight นับจาก clips.title_archetype
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
		return nil, fmt.Errorf("query title archetype usage: %w", err)
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
			return nil, fmt.Errorf("scan title archetype usage: %w", err)
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
	bestRatio := catUsageRatio(best.UsedCount, best.Arch.Weight)
	for _, u := range usages[1:] {
		if r2 := catUsageRatio(u.UsedCount, u.Arch.Weight); r2 < bestRatio {
			best, bestRatio = u, r2
		}
	}
	return &best.Arch, nil
}
```

- [ ] **Step 6: run test verify pass + build**

Run: `go test ./internal/repository/ -run TestTopicCategories -v`
Expected: PASS หรือ SKIP (ถ้าไม่มี DB helper)

Run: `go build ./...`
Expected: ผ่าน

- [ ] **Step 7: commit**

```bash
git add internal/models/format.go internal/repository/topics.go internal/repository/topics_test.go
git commit -m "feat(content-brain-v2): TopicCategory/TitleArchetype models + repos (PickNextExclude single picker path)"
```

---

## Task 3: Pure pickers — PickClipRole + PickPersona (no DB)

**Files:**
- Modify: `internal/orchestrator/topic_pick.go`
- Test: `internal/orchestrator/topic_pick_test.go`

**Interfaces:**
- Consumes: nothing (pure)
- Produces (pure, testable):
  - `PickClipRole(convertRatio float64, rng *rand.Rand) string` (คืน "reach" หรือ "convert")
  - `PickPersona(personas []string, rng *rand.Rand) string`

หมายเหตุ: ไม่มี pure `PickTopicCategory`/`PickArchetype` — ใช้ repo `PickNextExclude`/`PickNext` (Task 2) ที่มี DB และ least-used จริง.

- [ ] **Step 1: เขียน failing test**

สร้าง `internal/orchestrator/topic_pick_test.go`:

```go
package orchestrator

import (
	"math/rand"
	"testing"
)

func TestPickClipRole_RatioDistribution(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	convert := 0
	n := 1000
	for i := 0; i < n; i++ {
		if PickClipRole(0.30, rng) == "convert" {
			convert++
		}
	}
	// ~30% ± 5%
	if convert < n*25/100 || convert > n*35/100 {
		t.Errorf("convert ratio out of range: %d/%d", convert, n)
	}
}

func TestPickClipRole_Boundaries(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	if got := PickClipRole(0, rng); got != "reach" {
		t.Errorf("ratio=0 → want reach, got %s", got)
	}
	if got := PickClipRole(1, rng); got != "convert" {
		t.Errorf("ratio=1 → want convert, got %s", got)
	}
}

func TestPickPersona_NonEmpty(t *testing.T) {
	personas := []string{"media buyer", "owner", "agency", "banned"}
	rng := rand.New(rand.NewSource(4))
	got := PickPersona(personas, rng)
	if got == "" {
		t.Fatal("empty persona")
	}
	// ต้องเป็นหนึ่งใน list
	found := false
	for _, p := range personas {
		if p == got {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("persona %q not in list", got)
	}
}

func TestPickPersona_EmptyList(t *testing.T) {
	rng := rand.New(rand.NewSource(4))
	if got := PickPersona(nil, rng); got != "" {
		t.Errorf("empty list → want empty string, got %q", got)
	}
}
```

- [ ] **Step 2: run test verify fail**

Run: `go test ./internal/orchestrator/ -run TestPickClipRole -v`
Expected: FAIL (undefined: PickClipRole, PickPersona)

- [ ] **Step 3: เขียน implementation**

Append ที่ท้าย `internal/orchestrator/topic_pick.go` (ไฟล์มี import `math/rand` อยู่แล้วจาก PickCategoryWeighted — ตรวจก่อน ถ้าไม่มีให้เพิ่ม):

```go
// PickClipRole — "reach" (prob 1-convertRatio) / "convert" (prob convertRatio).
// Pure: caller ส่ง *rand.Rand เพื่อให้ทดสอบได้.
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

// PickPersona — สุ่ม 1 จาก personas (empty list → "")
func PickPersona(personas []string, rng *rand.Rand) string {
	if len(personas) == 0 {
		return ""
	}
	return personas[rng.Intn(len(personas))]
}
```

- [ ] **Step 4: run test verify pass + build**

Run: `go test ./internal/orchestrator/ -run "TestPickClipRole|TestPickPersona" -v`
Expected: PASS (ทั้ง 4 test)

Run: `go build ./...`
Expected: ผ่าน

- [ ] **Step 5: commit**

```bash
git add internal/orchestrator/topic_pick.go internal/orchestrator/topic_pick_test.go
git commit -m "feat(content-brain-v2): pure pickers — PickClipRole (ratio) + PickPersona"
```

---

## Task 4: QuestionAgent — template args ใหม่ + persist pain_point

**Files:**
- Modify: `internal/agent/question.go`
- Test: `internal/agent/question_test.go`

**Interfaces:**
- Consumes: template engine `renderTemplate` (reflection), `GeneratedQuestion.PainPoint` (มีแล้ว บรรทัด 46), topic_history INSERT (inline บรรทัด 186-193)
- Produces:
  - `QuestionTemplateData` เพิ่ม fields: `CategoryAngle`, `ArchetypeInstruction`, `RoleInstruction`, `TopicStats`
  - `QuestionAgent.Generate` signature เพิ่ม params: `categoryAngle string`, `archetypeInstr string`, `roleInstr string` (แทรกระหว่าง category กับ format)
  - topic_history INSERT เขียน `pain_point` column

- [ ] **Step 1: เขียน failing test**

ถ้าไม่มี `internal/agent/question_test.go` → สร้างใหม่ `package agent`. แล้ว append:

```go
package agent

import "testing"

// renderTemplate ต้องแทน {{.CategoryAngle}} {{.ArchetypeInstruction}} {{.RoleInstruction}} {{.TopicStats}}
func TestQuestionTemplateData_NewFieldsRender(t *testing.T) {
	td := QuestionTemplateData{
		Count: 3, Category: "multi-account",
		CategoryAngle:        "ANGLEX",
		ArchetypeInstruction: "ARCHX",
		RoleInstruction:      "ROLEX",
		TopicStats:           "STATSX",
	}
	out, err := renderTemplate("a {{.CategoryAngle}} b {{.ArchetypeInstruction}} c {{.RoleInstruction}} d {{.TopicStats}}", td)
	if err != nil {
		t.Fatalf("renderTemplate: %v", err)
	}
	want := "a ANGLEX b ARCHX c ROLEX d STATSX"
	if out != want {
		t.Errorf("render mismatch:\n got: %s\nwant: %s", out, want)
	}
}
```

- [ ] **Step 2: run test verify fail**

Run: `go test ./internal/agent/ -run TestQuestionTemplateData_NewFieldsRender -v`
Expected: FAIL (compile error — unknown fields CategoryAngle/ArchetypeInstruction/RoleInstruction/TopicStats ใน QuestionTemplateData)

- [ ] **Step 3: แก้ QuestionTemplateData (บรรทัด 20-28)**

เปลี่ยนจาก:
```go
type QuestionTemplateData struct {
	Count             int
	Category          string
	RAGContext        string
	PreviousTopics    string
	PreviousNames     string
	FormatInstruction string
	AudiencePersona   string
}
```
เป็น:
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

- [ ] **Step 4: แก้ Generate signature (บรรทัด 49)**

เปลี่ยนจาก:
```go
func (a *QuestionAgent) Generate(ctx context.Context, count int, category string, format *models.ContentFormat, persona string, topicStats string, cfg *models.AgentConfig) ([]GeneratedQuestion, error) {
```
เป็น:
```go
func (a *QuestionAgent) Generate(ctx context.Context, count int, category string, categoryAngle string, format *models.ContentFormat, persona string, archetypeInstr string, roleInstr string, topicStats string, cfg *models.AgentConfig) ([]GeneratedQuestion, error) {
```

- [ ] **Step 5: แก้ template data construction (บรรทัด 113-121)**

เปลี่ยนจาก:
```go
	userPrompt, err := renderTemplate(cfg.PromptTemplate, QuestionTemplateData{
		Count:             count,
		Category:          category,
		RAGContext:        ragContext.String(),
		PreviousTopics:    previousList,
		PreviousNames:     previousNames,
		FormatInstruction: format.QuestionInstruction,
		AudiencePersona:   persona,
	})
```
เป็น:
```go
	userPrompt, err := renderTemplate(cfg.PromptTemplate, QuestionTemplateData{
		Count:                count,
		Category:             category,
		CategoryAngle:        categoryAngle,
		ArchetypeInstruction: archetypeInstr,
		RoleInstruction:      roleInstr,
		TopicStats:           topicStats,
		RAGContext:           ragContext.String(),
		PreviousTopics:       previousList,
		PreviousNames:        previousNames,
		FormatInstruction:    format.QuestionInstruction,
		AudiencePersona:      persona,
	})
```

- [ ] **Step 6: ลบ append topicStats หลัง render (บรรทัด 127)**

เพราะตอนนี้ topicStats เป็น field ใน template แล้ว (`{{.TopicStats}}`). ลบบรรทัดนี้ทิ้ง:
```go
	// Real-performance context (empty when the topic_stats kill switch is off).
	userPrompt += topicStats
```

- [ ] **Step 7: แก้ topic_history INSERT (บรรทัด 184-194) เพิ่ม pain_point**

เปลี่ยนจาก:
```go
	for _, q := range accepted {
		if emb, ok := allEmbeddings[q.Question]; ok {
			a.pool.Exec(ctx,
				`INSERT INTO topic_history (title, category, embedding) VALUES ($1, $2, $3::vector)`,
				q.Question, q.Category, rag.FormatVector(emb))
		} else {
			a.pool.Exec(ctx,
				`INSERT INTO topic_history (title, category) VALUES ($1, $2)`,
				q.Question, q.Category)
		}
	}
```
เป็น:
```go
	for _, q := range accepted {
		if emb, ok := allEmbeddings[q.Question]; ok {
			a.pool.Exec(ctx,
				`INSERT INTO topic_history (title, category, pain_point, embedding) VALUES ($1, $2, $3, $4::vector)`,
				q.Question, q.Category, q.PainPoint, rag.FormatVector(emb))
		} else {
			a.pool.Exec(ctx,
				`INSERT INTO topic_history (title, category, pain_point) VALUES ($1, $2, $3)`,
				q.Question, q.Category, q.PainPoint)
		}
	}
```

- [ ] **Step 8: run test verify pass**

Run: `go test ./internal/agent/ -run TestQuestionTemplateData_NewFieldsRender -v`
Expected: PASS

Run: `go build ./...`
Expected: FAIL ที่ orchestrator.go (caller Generate ยังส่ง args เดิม) — **คาดว่าจะ fail ตรงนี้, จะแก้ใน Task 7**.

- [ ] **Step 9: commit (build break ชั่วคราวระหว่าง task — จะแก้ Task 7)**

```bash
git add internal/agent/question.go internal/agent/question_test.go
git commit -m "feat(content-brain-v2): QuestionAgent — category/angle/archetype/role template args + persist pain_point

build breaks at orchestrator caller; fixed in Task 7"
```

---

## Task 5: ScriptAgent — template args (archetype title + role CTA)

**Files:**
- Modify: `internal/agent/script.go`
- Test: `internal/agent/script_test.go`

**Interfaces:**
- Consumes: template engine, prompt template ใน DB (มี `{{.ArchetypeInstruction}}` `{{.RoleInstruction}}` แล้วจาก Task 1)
- Produces:
  - `ScriptTemplateData` เพิ่ม fields: `ArchetypeInstruction`, `RoleInstruction`
  - `ScriptAgent.Generate` signature เพิ่ม params: `archetypeInstr string`, `roleInstr string` (แทรกระหว่าง persona กับ cfg)

- [ ] **Step 1: ตรวจ ScriptTemplateData + Generate signature ปัจจุบัน**

Run: `sed -n '14,21p;61p' internal/agent/script.go` — จด struct fields จริง + signature จริงไว้.

- [ ] **Step 2: เขียน failing test**

ถ้าไม่มี `internal/agent/script_test.go` → สร้าง `package agent`. append:

```go
package agent

import "testing"

func TestScriptTemplateData_NewFieldsRender(t *testing.T) {
	td := ScriptTemplateData{
		Question:             "Q",
		ArchetypeInstruction: "ARCH",
		RoleInstruction:      "ROLE",
	}
	out, err := renderTemplate("h {{.ArchetypeInstruction}} i {{.RoleInstruction}}", td)
	if err != nil {
		t.Fatalf("renderTemplate: %v", err)
	}
	want := "h ARCH i ROLE"
	if out != want {
		t.Errorf("render mismatch:\n got: %s\nwant: %s", out, want)
	}
}
```

- [ ] **Step 3: run test verify fail**

Run: `go test ./internal/agent/ -run TestScriptTemplateData_NewFieldsRender -v`
Expected: FAIL (unknown fields)

- [ ] **Step 4: แก้ ScriptTemplateData (บรรทัด 14-21)**

เพิ่ม 2 fields (อ่าน struct จริงก่อน — เติมเข้าไปเป็น fields ใหม่ คง fields เดิมไว้):
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

- [ ] **Step 5: แก้ Generate signature (บรรทัด 61)**

เปลี่ยนจาก:
```go
func (a *ScriptAgent) Generate(ctx context.Context, question, questionerName, category string, format *models.ContentFormat, persona string, cfg *models.AgentConfig) (*GeneratedScript, error) {
```
เป็น:
```go
func (a *ScriptAgent) Generate(ctx context.Context, question, questionerName, category string, format *models.ContentFormat, persona string, archetypeInstr string, roleInstr string, cfg *models.AgentConfig) (*GeneratedScript, error) {
```

- [ ] **Step 6: แก้ template data construction ใน body**

หาจุดที่สร้าง `ScriptTemplateData{...}` ใน Generate (อ่านรอบๆ signature) เติม 2 fields:
```go
	td := ScriptTemplateData{
		Question:             question,
		QuestionerName:       questionerName,
		Category:             category,
		ArchetypeInstruction: archetypeInstr,
		RoleInstruction:      roleInstr,
		RAGContext:           ragCtx, // ชื่อตัวแปรท้องถิ่นจริงในไฟล์
		FormatInstruction:    format.ScriptInstruction,
		AudiencePersona:      persona,
	}
```
(ปรับชื่อตัวแปรท้องถิ่นให้ตรงกับที่มีจริง — อ่าน context รอบๆ ก่อนแก้.)

- [ ] **Step 7: run test verify pass**

Run: `go test ./internal/agent/ -run TestScriptTemplateData_NewFieldsRender -v`
Expected: PASS

- [ ] **Step 8: commit**

```bash
git add internal/agent/script.go internal/agent/script_test.go
git commit -m "feat(content-brain-v2): ScriptAgent — archetype/role template args (title + CTA control)"
```

---

## Task 6: Dedup hardening — const→field threshold + cooldown + lexical fallback

**Files:**
- Modify: `internal/agent/dedup.go`
- Modify: `internal/agent/question.go` (fail-open → retry → lexical + pain_point cooldown)
- Test: `internal/agent/dedup_test.go`

**Interfaces:**
- Consumes: settings `dedup_threshold` (0.72), `pain_point_cooldown_days` (5); `topic_history.pain_point`; `pg_trgm similarity()`
- Produces:
  - `Deduper` เพิ่ม field `threshold float64` + `SetThreshold(t float64)` method
  - `Deduper.PainPointInCooldown(ctx, painPoint string, days int) (bool, error)`
  - `Deduper.LexicalCheck(ctx, questions []GeneratedQuestion) (map[string]bool, error)` — `pg_trgm similarity()` เทียบ 30 title ล่าสุด, block ถ้า > 0.5
  - `question.go`: embed ล่ม → retry 1 → lexical guard → log
  - `question.go`: pain_point cooldown filter ก่อน insert
  - `QuestionAgent` เพิ่ม field `painCooldownDays int` + `SetPainCooldownDays(days int)` method

หมายเหตุ: `filterBySimilarity` (บรรทัด 30) **มี param `threshold` อยู่แล้ว** — ไม่ต้องแก้ signature แค่เปลี่ยนตัวส่งจาก `similarityThreshold` const เป็น `a.deduper.threshold`.

- [ ] **Step 1: เขียน failing test**

ถ้าไม่มี `internal/agent/dedup_test.go` → สร้าง `package agent`. append:

```go
package agent

import "testing"

// threshold ที่ส่งเข้า filterBySimilarity ควบคุม cutoff (ไม่ใช่ const ตายตัว)
func TestFilterBySimilarity_CustomThreshold(t *testing.T) {
	questions := []GeneratedQuestion{{Question: "q1"}, {Question: "q2"}}
	sims := map[string]SimilarityMatch{
		"q1": {Similarity: 0.75, MatchedTitle: "old"},
		"q2": {Similarity: 0.60, MatchedTitle: "old2"},
	}
	// threshold 0.72 → q1 (0.75>=0.72) reject, q2 (0.60<0.72) pass
	passed, rejected := filterBySimilarity(questions, sims, 0.72)
	if len(passed) != 1 || passed[0].Question != "q2" {
		t.Errorf("expected q2 to pass at threshold 0.72, got passed=%+v", passed)
	}
	if len(rejected) != 1 || rejected[0].Question.Question != "q1" {
		t.Errorf("expected q1 rejected at 0.72, got rejected=%+v", rejected)
	}
}

// SetThreshold เปลี่ยนค่าที่ Deduper ใช้
func TestDeduper_SetThreshold(t *testing.T) {
	d := &Deduper{}
	d.SetThreshold(0.72)
	if d.threshold != 0.72 {
		t.Errorf("expected threshold 0.72, got %v", d.threshold)
	}
	d.SetThreshold(0) // ค่าไร้สาระ → ไม่เปลี่ยน
	if d.threshold != 0.72 {
		t.Errorf("zero threshold should not overwrite, got %v", d.threshold)
	}
}
```

- [ ] **Step 2: run test verify fail**

Run: `go test ./internal/agent/ -run "TestFilterBySimilarity_CustomThreshold|TestDeduper_SetThreshold" -v`
Expected: FAIL (TestFilterBySimilarity อาจผ่านเพราะ func มีอยู่ — แต่ TestDeduper_SetThreshold FAIL เพราะไม่มี field/SetThreshold)

- [ ] **Step 3: แก้ dedup.go — const → field + SetThreshold**

ลบบรรทัด 14-17:
```go
// similarityThreshold: questions with >= this cosine similarity to any past
// topic are considered semantic duplicates and rejected.
// Calibrated against real data: known duplicate pairs in production scored
// 0.81-0.82, while genuinely different angles scored 0.69-0.75.
const similarityThreshold = 0.78
```

แก้ `Deduper` struct + constructor + เพิ่ม setter (บรรทัด 43-49):
```go
// Deduper checks generated questions against past topics using pgvector.
type Deduper struct {
	pool      *pgxpool.Pool
	rag       *rag.Engine
	threshold float64 // default 0.78 (legacy); orchestrator set 0.72 เมื่อ flag on
}

func NewDeduper(pool *pgxpool.Pool, ragEngine *rag.Engine) *Deduper {
	return &Deduper{pool: pool, rag: ragEngine, threshold: 0.78}
}

// SetThreshold — orchestrator เรียกเมื่อ flag on (ค่าจาก setting dedup_threshold).
// ค่า <= 0 ไม่มีผล (กันเขียนทับด้วยค่าไร้สาระ).
func (d *Deduper) SetThreshold(t float64) {
	if t > 0 {
		d.threshold = t
	}
}
```

หมายเหตุ: `NewDeduper` signature **ไม่เปลี่ยน** → `main.go` และ `NewQuestionAgent` (ที่เรียก NewDeduper) ไม่ต้องแก้.

- [ ] **Step 4: เพิ่ม PainPointInCooldown + LexicalCheck ท้าย dedup.go**

```go
// PainPointInCooldown — true ถ้า pain_point นี้เคยปรากฏใน topic_history ใน N วันล่าสุด.
// กัน "หัวข้อเดิมเปลี่ยนมุม" ที่ embedding จับไม่ได้. painPoint ว่าง/days<=0 → false.
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
		return false, fmt.Errorf("pain_point cooldown query: %w", err)
	}
	return n > 0, nil
}

// LexicalCheck — fallback เมื่อ embedding ล่ม. ใช้ pg_trgm similarity() เทียบ 30 title ล่าสุด.
// คืน map[question]true สำหรับ question ที่มี similarity > 0.5 กับ title เก่าอย่างน้อย 1 ตัว.
func (d *Deduper) LexicalCheck(ctx context.Context, questions []GeneratedQuestion) (map[string]bool, error) {
	rows, err := d.pool.Query(ctx,
		`SELECT title FROM topic_history ORDER BY created_at DESC LIMIT 30`)
	if err != nil {
		return nil, fmt.Errorf("query recent titles for lexical check: %w", err)
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
			if err := d.pool.QueryRow(ctx, `SELECT similarity($1, $2)`, q.Question, p).Scan(&sim); err != nil {
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

- [ ] **Step 5: แก้ question.go — Deduper.CheckQuestions error → retry → lexical**

ใน `question.go` แก้บรรทัด 140-146. เปลี่ยนจาก:
```go
		similarities, embeddings, err := a.deduper.CheckQuestions(ctx, questions)
		if err != nil {
			// Embedding service down — accept as-is rather than block production.
			log.Printf("QuestionAgent: dedup check failed, accepting without dedup: %v", err)
			accepted = append(accepted, questions...)
			break
		}
```
เป็น:
```go
		similarities, embeddings, err := a.deduper.CheckQuestions(ctx, questions)
		if err != nil {
			// Embedding ล่ม → retry 1 ครั้ง ก่อน fallback lexical
			log.Printf("QuestionAgent: dedup embedding error, retrying once: %v", err)
			time.Sleep(500 * time.Millisecond)
			similarities, embeddings, err = a.deduper.CheckQuestions(ctx, questions)
		}
		if err != nil {
			// ยังล่ม → lexical guard (pg_trgm) แทน ห้ามรับมั่ว
			log.Printf("QuestionAgent: dedup still failing, using lexical fallback: %v", err)
			blocked, lexErr := a.deduper.LexicalCheck(ctx, questions)
			if lexErr != nil {
				log.Printf("QuestionAgent: lexical fallback also failed, accepting all (last resort): %v", lexErr)
				accepted = append(accepted, questions...)
			} else {
				for _, q := range questions {
					if !blocked[q.Question] {
						accepted = append(accepted, q)
					}
				}
			}
			break
		}
```

เพิ่ม import `"time"` ที่ block import (บรรทัด 3-13) ถ้ายังไม่มี.

- [ ] **Step 6: แก้ question.go — เปลี่ยน const threshold → field (บรรทัด 151)**

เปลี่ยน:
```go
		passed, rejected := filterBySimilarity(questions, similarities, similarityThreshold)
```
เป็น:
```go
		passed, rejected := filterBySimilarity(questions, similarities, a.deduper.threshold)
```

- [ ] **Step 7: เพิ่ม painCooldownDays field + setter ใน QuestionAgent**

ใน `question.go` แก้ struct (บรรทัด 30-36) เพิ่ม 1 field:
```go
type QuestionAgent struct {
	llm             *KieLLMClient
	rag             *rag.Engine
	pool            *pgxpool.Pool
	deduper         *Deduper
	research        *ResearchAgent
	painCooldownDays int // 0 = skip cooldown (legacy); set จาก setting pain_point_cooldown_days
}
```

เพิ่ม setter method (หลัง constructor บรรทัด 40):
```go
// SetPainCooldownDays — orchestrator เรียกเมื่อ flag on (ค่าจาก setting pain_point_cooldown_days).
func (a *QuestionAgent) SetPainCooldownDays(days int) { a.painCooldownDays = days }
```

- [ ] **Step 8: เพิ่ม pain_point cooldown filter ก่อน insert (หลังบรรทัด 181 ก่อน insert loop)**

หลัง `if len(accepted) > count { accepted = accepted[:count] }` และก่อน `for _, q := range accepted` insert loop ให้แทรก:
```go
	// pain_point cooldown: กันหัวข้อเดิมเปลี่ยนมุม (flag on เท่านั้น — painCooldownDays > 0)
	if a.painCooldownDays > 0 && len(accepted) > 0 {
		filtered := accepted[:0]
		for _, q := range accepted {
			inCD, err := a.deduper.PainPointInCooldown(ctx, q.PainPoint, a.painCooldownDays)
			if err != nil {
				log.Printf("QuestionAgent: pain_point cooldown check error (fail-open): %v", err)
				filtered = append(filtered, q) // fail-open สำหรับ cooldown
				continue
			}
			if !inCD {
				filtered = append(filtered, q)
			} else {
				log.Printf("QuestionAgent: pain_point %q in cooldown, dropped", q.PainPoint)
			}
		}
		accepted = filtered
	}
```

- [ ] **Step 9: run tests verify pass**

Run: `go test ./internal/agent/ -run "TestFilterBySimilarity_CustomThreshold|TestDeduper_SetThreshold" -v`
Expected: PASS

Run: `go build ./...`
Expected: FAIL ที่ orchestrator.go (caller) — แก้ใน Task 7.

- [ ] **Step 10: commit**

```bash
git add internal/agent/dedup.go internal/agent/dedup_test.go internal/agent/question.go
git commit -m "feat(content-brain-v2): dedup hardening — threshold via SetThreshold + pain_point cooldown + lexical fallback (fail-closed w/ escape)"
```

---

## Task 7: Wire orchestrator (ProduceWeekly) + clip columns — thread params

**Files:**
- Modify: `internal/models/request.go` (CreateClipRequest)
- Modify: `internal/repository/clips.go` (Create INSERT + CategoriesUsedToday)
- Modify: `internal/orchestrator/orchestrator.go` (struct fields, constructor, ProduceWeekly, produceClip, produceClipWithID)
- Test: `internal/orchestrator/orchestrator_test.go`

**Interfaces:**
- Consumes: Task 2 (repos), Task 3 (pure pickers), Task 4/5/6 (agent signatures + SetThreshold + SetPainCooldownDays)
- Produces: flag-gated pipeline ใช้ category 10 + exclude-per-day, archetype, role 70/30, persona rotation, news fallback→least-used, persist clip_role/title_archetype/audience_persona

- [ ] **Step 1: แก้ CreateClipRequest (internal/models/request.go บรรทัด 5-12)**

เปลี่ยนจาก:
```go
type CreateClipRequest struct {
	Title          string  `json:"title"`
	Question       string  `json:"question"`
	QuestionerName string  `json:"questioner_name"`
	Category       string  `json:"category"`
	PublishDate    *string `json:"publish_date"`
	ContentFormat  string  `json:"content_format"`
}
```
เป็น:
```go
type CreateClipRequest struct {
	Title           string  `json:"title"`
	Question        string  `json:"question"`
	QuestionerName  string  `json:"questioner_name"`
	Category        string  `json:"category"`
	PublishDate     *string `json:"publish_date"`
	ContentFormat   string  `json:"content_format"`
	ClipRole        string  `json:"clip_role"`
	TitleArchetype  string  `json:"title_archetype"`
	AudiencePersona string  `json:"audience_persona"`
}
```

- [ ] **Step 2: แก้ clips.go Create INSERT**

อ่าน `Create` INSERT จริงก่อน: `sed -n '67,100p' internal/repository/clips.go`. ใน INSERT statement เพิ่ม 3 columns + values. ตัวอย่าง (ปรับตามจริง — ดูว่ามี RETURNING หรือ Scan อะไร):

```sql
INSERT INTO clips (title, question, questioner_name, category, publish_date, content_format, clip_role, title_archetype, audience_persona, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'pending')
RETURNING id, ...
```
และเพิ่ม args `$7 req.ClipRole, $8 req.TitleArchetype, $9 req.AudiencePersona` ต่อท้าย args list (เลื่อน $ ที่เหลือถ้ามี — เช่น status อาจเป็น $10).

- [ ] **Step 3: เพิ่ม clipsRepo.CategoriesUsedToday**

ใน `internal/repository/clips.go` append:
```go
// CategoriesUsedToday — หมวดที่สร้างคลิปใน 24h ล่าสุด (กันซ้ำในวันเดียวกัน)
func (r *ClipsRepo) CategoriesUsedToday(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT category FROM clips
		 WHERE created_at > NOW() - INTERVAL '24 hours' AND category <> ''`)
	if err != nil {
		return nil, fmt.Errorf("query categories used today: %w", err)
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

- [ ] **Step 4: เพิ่ม repo fields ใน Orchestrator struct + constructor**

อ่าน `Orchestrator` struct จริง: `grep -n "type Orchestrator struct" -A 25 internal/orchestrator/orchestrator.go`. เพิ่ม 2 fields:
```go
	topicCategoriesRepo *repository.TopicCategoriesRepo
	titleArchetypesRepo *repository.TitleArchetypesRepo
```

อ่าน constructor `NewOrchestrator` จริง: `grep -n "func NewOrchestrator" -A 40 internal/orchestrator/orchestrator.go`. เพิ่ม 2 params + 2 assignment (ปรับตามรูปแบบ params เดิม):
```go
func NewOrchestrator(..., topicCategoriesRepo *repository.TopicCategoriesRepo, titleArchetypesRepo *repository.TitleArchetypesRepo, ...) *Orchestrator {
	return &Orchestrator{
		...
		topicCategoriesRepo: topicCategoriesRepo,
		titleArchetypesRepo: titleArchetypesRepo,
		...
	}
}
```

อัปเดต caller ใน `cmd/server/main.go` (หาจุดที่เรียก `orchestrator.NewOrchestrator(...)`): เพิ่ม 2 args `repository.NewTopicCategoriesRepo(pool), repository.NewTitleArchetypesRepo(pool)`.

- [ ] **Step 5: แก้ ProduceWeekly — flag branch + picks (บรรทัด 131-166)**

แทนที่ block บรรทัด 131-166 (จาก `weekNum := ...` ถึง persona block `}`):
```go
	// ===== content brain v2 flag =====
	v2Raw, _ := o.settingsRepo.Get(ctx, "content_brain_v2_enabled")
	v2 := v2Raw == "true"

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	var category, categoryAngle string
	var archetype models.TitleArchetype
	var role, persona string

	if v2 {
		// ---- category: least-used/7d+weight, exclude หมวดที่ใช้ในวันนี้ ----
		usedToday, _ := o.clipsRepo.CategoriesUsedToday(ctx)
		tcat, err := o.topicCategoriesRepo.PickNextExclude(ctx, usedToday)
		if err != nil || tcat == nil {
			log.Printf("Orchestrator: topic_categories pick failed, legacy fallback: %v", err)
			v2 = false // ลดระดับกลับ legacy
		} else {
			category = tcat.CategoryName
			categoryAngle = tcat.AngleInstruction
		}

		// ---- archetype: least-used/7d+weight ----
		if v2 {
			if a, err := o.titleArchetypesRepo.PickNext(ctx); err != nil || a == nil {
				log.Printf("Orchestrator: archetype pick failed, using empty: %v", err)
			} else {
				archetype = *a
			}
		}

		// ---- role 70/30 ----
		ratioStr, _ := o.settingsRepo.Get(ctx, "clip_role_convert_ratio")
		ratio, err := strconv.ParseFloat(ratioStr, 64)
		if err != nil || ratio <= 0 || ratio >= 1 {
			ratio = 0.30
		}
		role = PickClipRole(ratio, rng)

		// ---- persona rotation (fallback ใช้ audience_persona เดิม) ----
		personasJSON, _ := o.settingsRepo.Get(ctx, "audience_personas")
		var personas []string
		if json.Unmarshal([]byte(personasJSON), &personas) == nil && len(personas) > 0 {
			persona = PickPersona(personas, rng)
		} else {
			persona, _ = o.settingsRepo.Get(ctx, "audience_persona")
		}

		// ---- dedup threshold + cooldown จาก setting ----
		if tStr, _ := o.settingsRepo.Get(ctx, "dedup_threshold"); tStr != "" {
			if t, err := strconv.ParseFloat(tStr, 64); err == nil {
				o.questionAgent.Deduper().SetThreshold(t)
			}
		}
		if cdStr, _ := o.settingsRepo.Get(ctx, "pain_point_cooldown_days"); cdStr != "" {
			if cd, err := strconv.Atoi(cdStr); err == nil {
				o.questionAgent.SetPainCooldownDays(cd)
			}
		}
	}

	var topicStats string
	if !v2 {
		// ---- legacy: weekNum round-robin + PickCategoryWeighted ----
		weekNum := int(time.Now().Unix() / (7 * 24 * 3600))
		categories, err := o.settingsRepo.GetCategories(ctx)
		if err != nil {
			return fmt.Errorf("read categories: %w", err)
		}
		if len(categories) == 0 {
			return fmt.Errorf("no categories configured")
		}
		category = categories[weekNum%len(categories)]
		if v, err := o.settingsRepo.Get(ctx, "topic_stats_enabled"); err != nil || v != "false" {
			if scores, err := o.analyticsRepo.TopicPerformance(ctx, 30, 3); err != nil {
				log.Printf("topic performance unavailable, using round-robin category: %v", err)
			} else {
				category = PickCategoryWeighted(categories, scores, weekNum, rand.Intn)
				topicStats = FormatTopicStats(scores)
			}
		}
		persona, _ = o.settingsRepo.Get(ctx, "audience_persona")
	} else {
		// v2: topicStats เป็นข้อมูลประกอบ (แนบใน prompt แต่ไม่บังคับทิศ)
		if scores, err := o.analyticsRepo.TopicPerformance(ctx, 30, 3); err == nil {
			topicStats = FormatTopicStats(scores)
		}
	}
```

เพิ่ม imports ถ้ายังไม่มี: `"encoding/json"`, `"strconv"`.

- [ ] **Step 6: เพิ่ม Deduper() accessor ใน QuestionAgent (internal/agent/question.go)**

เพราะ orchestrator ต้องเรียก `o.questionAgent.Deduper().SetThreshold(t)`:
```go
// Deduper — expose ให้ orchestrator set threshold ตอน flag on
func (a *QuestionAgent) Deduper() *Deduper { return a.deduper }
```

- [ ] **Step 7: แก้ caller Generate 2 จุด (บรรทัด 191 + 200)**

บรรทัด 191 เปลี่ยนจาก:
```go
	questions, err := o.questionAgent.Generate(ctx, count, category, format, persona, topicStats, qaCfg)
```
เป็น:
```go
	questions, err := o.questionAgent.Generate(ctx, count, category, categoryAngle, format, persona, archetype.Instruction, role, topicStats, qaCfg)
```

block news fallback (บรรทัด 192-201) เปลี่ยนจาก:
```go
	if errors.Is(err, agent.ErrNoFreshNews) {
		log.Println("No fresh news available, falling back to Q&A format")
		format, err = o.formatsRepo.GetByName(ctx, "qa")
		if err != nil {
			o.tracker.FailStep("question", err)
			return fmt.Errorf("fallback to qa format: %w", err)
		}
		questions, err = o.questionAgent.Generate(ctx, count, category, format, persona, topicStats, qaCfg)
	}
```
เป็น:
```go
	if errors.Is(err, agent.ErrNoFreshNews) {
		if v2 {
			// v2: fallback เป็น least-used format (ไม่ fix qa)
			log.Println("No fresh news available, falling back to least-used format")
			if f, ferr := o.formatsRepo.PickNext(ctx); ferr == nil && f != nil {
				format = f
			} else {
				format, _ = o.formatsRepo.GetByName(ctx, "qa") // last resort
			}
		} else {
			log.Println("No fresh news available, falling back to Q&A format")
			format, err = o.formatsRepo.GetByName(ctx, "qa")
			if err != nil {
				o.tracker.FailStep("question", err)
				return fmt.Errorf("fallback to qa format: %w", err)
			}
		}
		questions, err = o.questionAgent.Generate(ctx, count, category, categoryAngle, format, persona, archetype.Instruction, role, topicStats, qaCfg)
	}
```

- [ ] **Step 8: แก้ produceClip signature + ส่ง args (บรรทัด 253 + caller บรรทัด 236)**

เปลี่ยน signature (บรรทัด 253) จาก:
```go
func (o *Orchestrator) produceClip(ctx context.Context, q agent.GeneratedQuestion, theme *models.BrandTheme, scriptCfg, imageCfg *models.AgentConfig, brandAliases map[string]string, format *models.ContentFormat, persona string) error {
```
เป็น:
```go
func (o *Orchestrator) produceClip(ctx context.Context, q agent.GeneratedQuestion, theme *models.BrandTheme, scriptCfg, imageCfg *models.AgentConfig, brandAliases map[string]string, format *models.ContentFormat, persona, archetypeInstr, role string) error {
```

caller (บรรทัด 236) เปลี่ยนจาก:
```go
		if err := o.produceClip(ctx, q, theme, scriptCfg, imageCfg, brandAliases, format, persona); err != nil {
```
เป็น:
```go
		if err := o.produceClip(ctx, q, theme, scriptCfg, imageCfg, brandAliases, format, persona, archetype.Instruction, role); err != nil {
```

- [ ] **Step 9: แก้ produceClip ส่ง clip_role/archetype/persona ใน clipsRepo.Create**

ใน `produceClip` (อ่านรอบๆ บรรทัด 272-282 ที่ clipsRepo.Create) เพิ่ม fields ใน CreateClipRequest:
```go
	clip, err := o.clipsRepo.Create(ctx, models.CreateClipRequest{
		Title:           q.Question,
		Question:        q.Question,
		QuestionerName:  q.QuestionerName,
		Category:        q.Category,
		PublishDate:     &today,
		ContentFormat:   format.FormatName,
		ClipRole:        role,
		TitleArchetype:  archetypeName, // ดู Step 10
		AudiencePersona: persona,
	})
```
หมายเหตุ: `archetypeName` — ใน produceClip เราส่ง `archetypeInstr` string เข้ามา (ไม่ใช่ struct). สำหรับบันทึก `clips.title_archetype` เราต้องการชื่อ archetype ด้วย. **ทางเลือก:** ส่ง `archetype models.TitleArchetype` เข้า produceClip ทั้ง struct แทนแค่ instruction — เปลี่ยน Step 8 signature เป็น `archetype models.TitleArchetype` แล้วใช้ `archetype.ArchetypeName` + `archetype.Instruction`. **แนะนำทางนี้** (ง่ายกว่า).

แก้ Step 8 signature จริงเป็น:
```go
func (o *Orchestrator) produceClip(ctx context.Context, q agent.GeneratedQuestion, theme *models.BrandTheme, scriptCfg, imageCfg *models.AgentConfig, brandAliases map[string]string, format *models.ContentFormat, persona string, archetype models.TitleArchetype, role string) error {
```
caller:
```go
		if err := o.produceClip(ctx, q, theme, scriptCfg, imageCfg, brandAliases, format, persona, archetype, role); err != nil {
```
แล้ว Create ใช้ `TitleArchetype: archetype.ArchetypeName`.

- [ ] **Step 10: แก้ produceClipWithID signature + scriptAgent.Generate caller (บรรทัด 344 + 353)**

อ่าน `produceClipWithID` (บรรทัด 344) + ดูว่า produceClip เรียก produceClipWithID ยังไง (อ่านรอบๆ บรรทัด 285+). เพิ่ม `archetype models.TitleArchetype, role string` ทั้ง signature ของ produceClipWithID และตอน caller ใน produceClip.

แก้ scriptAgent.Generate caller (บรรทัด 353) จาก:
```go
	script, err := o.scriptAgent.Generate(ctx, q.Question, q.QuestionerName, q.Category, format, persona, scriptCfg)
```
เป็น:
```go
	script, err := o.scriptAgent.Generate(ctx, q.Question, q.QuestionerName, q.Category, format, persona, archetype.Instruction, role, scriptCfg)
```

- [ ] **Step 11: แก้ / ย้าย Update clips ถ้ามี Update แยก**

ถ้า produceClip/produceClipWithID มี `clipsRepo.Update(...)` ที่ set style_preset/status ภายหลัง → clip_role/archetype/persona set ตอน Create แล้ว (Step 9) ไม่ต้อง Update ซ้ำ. (อ่านเพื่อยืนยันว่าไม่มี UPDATE เขียนทับค่าว่าง.)

- [ ] **Step 12: build + run tests**

Run: `go build ./...`
Expected: ผ่าน (ทุก caller signature ตรงแล้ว)

Run: `go test ./internal/orchestrator/ ./internal/agent/ ./internal/repository/ -short -v`
Expected: PASS (pure/unit tests ผ่าน; DB tests skip)

Run: `go vet ./...`
Expected: ผ่าน

- [ ] **Step 13: commit**

```bash
git add internal/models/request.go internal/repository/clips.go internal/orchestrator/orchestrator.go internal/agent/question.go cmd/server/main.go
git commit -m "feat(content-brain-v2): wire flag-gated pipeline in ProduceWeekly — 10 categories w/ exclude-per-day, archetype, role 70/30, persona rotation, news fallback→least-used, persist clip role/archetype/persona; setter-based threshold/cooldown (no constructor change)"
```

---

## Task 8: Insider KB pack ingest

**Files:**
- Create: `scripts/insider_kb_content/01_multi_account.txt` ... `10_economics.txt` (10 ไฟล์)
- Create: `scripts/ingest_insider_kb.sh`

**Interfaces:**
- Consumes: KB API `POST /api/v1/knowledge/sources` (body `{name, category, content}`) + `POST /api/v1/knowledge/sources/{id}/embed`
- Produces: ~10 sources ใหม่ใน `knowledge_sources` + embedded chunks ใน `knowledge_chunks`

- [ ] **Step 1: ตรวจ KB API auth จริง**

Run: `grep -n "Authorization\|router.Group\|middleware\|knowledge" internal/router/router.go internal/handler/knowledge.go | head -20`
จดว่า endpoint `/api/v1/knowledge/sources` ต้องการ auth header อะไรจริง (ถ้ามี). ถ้าไม่มี auth → ลบบรรทัด Authorization ใน script (Step 3).

- [ ] **Step 2: เขียน content 10 sources**

สร้างโฟลเดอร์ `scripts/insider_kb_content/` และ 10 ไฟล์ `.txt` (1 ต่อ category). แต่ละไฟล์ ~400-800 คำ เนื้อหา: pain scenarios + ศัพท์ + สถานการณ์จริงของหมวดนั้น. **กติกา hard rule (spec §3.6):** เล่า pain + การบริหารความเสี่ยงเชิงโครงสร้างได้; **ห้าม** สอนหลบระบบตรวจจับ/ปลอมตัวตน/ทำผิดนโยบาย.

ตัวอย่างโครง `scripts/insider_kb_content/01_multi_account.txt`:

```
หมวด: บริหารหลายบัญชี/พอร์ต

Pain scenarios ของคนถือหลายบัญชี:
- พอร์ตพังยกแผงเพราะบัญชีติดกัน (shared signal: บัตรใบเดียวกัน, payment profile, device)
- บัญชีใหม่ในพอร์ตตายไวเพราะไม่ได้วอร์มก่อนยิงหนัก
- ย้ายงบระหว่างบัญชีแล้ว trigger การตรวจสอบ

โครงสร้างพอร์ตที่กระจายความเสี่ยง (เชิงนโยบาย):
- แยก entity ทางธุรกิจ (portfolio per entity) เป็นการแยกข้อมูลตัวตน/การเงิน
- กระจายวิธีชำระ (billing profile ต่างกัน) ไม่ใช้บัตร/ธนาคารซ้ำข้ามพอร์ต
- backup admin access แยกคน ไม่รวมคนเดียวถือทั้งพอร์ต
- วาง ad account ตามขนาดงบช่วง (spending limit tier) เพื่อกระจาย exposure

ศัพท์ที่คนวงในใช้:
พอร์ต, ad account, spending limit, billing profile, Business Portfolio, entity, warm-up, trust score

ข้อควรระวัง (เชิงการป้องกัน):
- อย่าย้ายงบกะทันหันข้ามบัญชี — เพิ่มทีละน้อย (ramp)
- บัญชีใหม่ต้องสร้างพฤติกรรมปกติก่อน (วอร์ม) ก่อนยิงงบเต็ม
```

เขียนครบ 10 ไฟล์ตาม category_name ใน migration (multi-account, account-trust, bm-structure, ban-signals, recovery, payment, scaling, creative, tracking, economics). ทุกไฟล์ตามโครงเดียวกัน: pain scenarios + โครงสร้าง/การบริหาร + ศัพท์ + ข้อควรระวัง. **ห้ามละเมิด guardrail.**

- [ ] **Step 3: เขียน ingest script**

สร้าง `scripts/ingest_insider_kb.sh` (ปรับ AUTH ตาม Step 1 — ถ้าไม่มี auth ละบรรทัด `-H "Authorization: ..."`):

```bash
#!/usr/bin/env bash
# Ingest insider KB pack เข้า Ads Vance ผ่าน KB API
# ใช้: BASE_URL=... ./scripts/ingest_insider_kb.sh   (เพิ่ม API_TOKEN=... ถ้า endpoint มี auth)
set -euo pipefail

: "${BASE_URL:?need BASE_URL e.g. https://adsvance-v2.up.railway.app}"
API_TOKEN="${API_TOKEN:-}"

DIR="$(dirname "$0")/insider_kb_content"
AUTH=()
if [ -n "$API_TOKEN" ]; then
	AUTH=(-H "Authorization: Bearer $API_TOKEN")
fi

shopt -s nullglob
files=("$DIR"/*.txt)

for f in "${files[@]}"; do
	name="insider-$(basename "$f" .txt)"
	category=$(basename "$f" .txt | sed 's/^[0-9]*_//')
	content=$(cat "$f")
	echo "==> ingesting $name (category=$category)"

	resp=$(curl -sS -X POST "$BASE_URL/api/v1/knowledge/sources" \
		"${AUTH[@]}" \
		-H "Content-Type: application/json" \
		-d "$(jq -n --arg n "$name" --arg c "$category" --arg ct "$content" '{name:$n, category:$c, content:$ct}')")

	id=$(echo "$resp" | jq -r '.id // empty')
	if [ -z "$id" ]; then
		echo "  FAIL: no id in response: $resp" >&2
		continue
	fi
	echo "  created source $id, embedding..."
	curl -sS -X POST "$BASE_URL/api/v1/knowledge/sources/$id/embed" "${AUTH[@]}" | jq -r '.chunks // "embedded"'
done

echo "done. rollback: ลบ sources ที่ name LIKE 'insider-%' ผ่าน DELETE /api/v1/knowledge/sources/{id}"
```

chmod:
```bash
chmod +x scripts/ingest_insider_kb.sh
```

- [ ] **Step 4: build + verify script syntax**

Run: `go build ./... && bash -n scripts/ingest_insider_kb.sh`
Expected: ผ่าน

- [ ] **Step 5: commit**

```bash
git add scripts/ingest_insider_kb.sh scripts/insider_kb_content/
git commit -m "feat(content-brain-v2): insider KB pack (10 sources) + ingest script"
```

---

## Task 9: Deploy + end-to-end verify on flag

**Files:** none (ops + manual eyeball)

- [ ] **Step 1: deploy ขึ้น prod (push master)**

push master → Railway auto-deploy + auto-migrate (migration 051 รันตอน boot). ตรวจ Railway logs ว่า migration applied + server start OK (ไม่มี panic).

- [ ] **Step 2: ingest insider KB**

```bash
BASE_URL=<prod-url> ./scripts/ingest_insider_kb.sh
```
(เพิ่ม `API_TOKEN=...` ถ้า endpoint มี auth.) ตรวจ: `GET /api/v1/knowledge/sources` เห็น sources ที่ name เริ่มด้วย `insider-` และ chunks count > 0.

- [ ] **Step 3: flip flag + trigger 1 clip**

flip flag (Neon `run_sql` — adsvance-v2 = snowy-grass-75448787):
```sql
UPDATE settings SET value='true' WHERE key='content_brain_v2_enabled';
```

trigger produce (1 clip):
```bash
curl -X POST <prod>/api/v1/orchestrator/produce -H "Content-Type: application/json" -d '{"count":1}'
```
(ปรับ endpoint ตาม router จริง — ตรวจ `grep -n "produce\|orchestrator" internal/router/router.go`.)

- [ ] **Step 4: eyeball clip ผลลัพธ์**

ตรวจคลิปที่ผลิตได้:
- title ไม่ใช่สูตร "คุณXครับ" (เว้น archetype = consult_qa)
- CTA ตรง role (reach = ชวนติดตาม; convert = ชวนทักแชท)
- clip_role / title_archetype / audience_persona ถูกบันทึกใน clips (query Neon)
- render ผ่าน (status=ready ไม่ใช่ failed/needs_review)

Run query verify:
```sql
SELECT title, clip_role, title_archetype, audience_persona, content_format, status
FROM clips ORDER BY created_at DESC LIMIT 3;
```

- [ ] **Step 5: เกณฑ์วัดหลัง 3 วัน (9 คลิป — ไม่ใช่ gate ตอน implement)**

- ชื่อคลิปขึ้นต้น "คุณXครับ" ≤ 2/9
- ครอบคลุม ≥ 5 หมวด, ≥ 4 archetypes
- role split อยู่ช่วง 60/40–80/20
- ไม่มีคู่หัวข้อที่คนอ่านแล้วรู้สึก "เรื่องเดียวกัน" (eyeball)

ถ้าไม่ผ่าน → tune setting (ratio/threshold/cooldown) โดยไม่ deploy; ถ้าพัง → `content_brain_v2_enabled=false` rollback ทันที.

- [ ] **Step 6: อัปเดต memory**

หลัง verify ผ่าน → สร้าง memory file `project_content_brain_v2.md` บันทึก: flag, migration 051, success criteria, gotchas.

---

## Self-Review (plan author — v2)

**Spec coverage (ทุกส่วนของ spec §3):**
- §3.1 หมวด 10 + picker per-clip ห้ามซ้ำในวัน → Task 1 + Task 2 (`PickNextExclude`) + Task 7 (CategoriesUsedToday + wire) ✓
- §3.2 role reach/convert 70/30 → Task 1 (settings) + Task 3 (PickClipRole) + Task 4/5 (prompt) + Task 7 (wire) ✓
- §3.3 title archetypes 7 + rebalance qa weight + news fallback → Task 1 (table+seed+weight UPDATE) + Task 2 (repo PickNext) + Task 4/5 (prompt) + Task 7 (news fallback→least-used) ✓
- §3.4 persona 4 → Task 1 (settings JSON) + Task 3 (PickPersona) + Task 4/5 (prompt) + Task 7 (wire) ✓
- §3.5 dedup hardening → Task 1 (pg_trgm + settings + column) + Task 6 ✓
- §3.6 insider KB pack + insider voice → Task 1 (prompt template UPDATE) + Task 8 (KB ingest) ✓
- §3.7 out of scope (performance loop, UI queue, visual pipeline, auto-ingest news) → ไม่มี task = ✓

**Placeholder scan:** ไม่มี TBD/TODO/`SetPainCooldownDaysLookup(...)`. ทุก code step มี code จริง. บาง step บอก "อ่านไฟล์จริงก่อนแก้ (ระบุบรรทัด)" เพราะตำแหน่งอาจเลื่อน — มี code ตัวอย่างครบ.

**Type consistency:** `models.TopicCategory`/`TitleArchetype` ชื่อตรงทุก task. `PickNextExclude`/`PickNext`/`PickClipRole`/`PickPersona` ชื่อตรง. `Deduper.SetThreshold`/`QuestionAgent.SetPainCooldownDays`/`Deduper()` accessor ชื่อตรง. produceClip/produceClipWithID รับ `archetype models.TitleArchetype, role string`.

**จุดที่ต้องระวังตอน implement:**
1. Task 4/5/6 เปลี่ยน Generate signature → build break ระหว่าง task (commit note บอก) → Task 7 ปิด. ถ้า repo บังคับ green build ทุก commit → รวม commit Task 4+5+6+7.
2. `NewOrchestrator` เพิ่ม params → `cmd/server/main.go` caller ต้องแก้ (Task 7 Step 4).
3. `NewDeduper`/`NewQuestionAgent` **ไม่เปลี่ยน signature** (ใช้ setter) → `main.go` constructor อื่นไม่ต้องแตะ.
4. migration prompt template ใช้ `$q$` dollar-quoting — ตรวจ balance (4 ตัว `$q$`).
5. KB API auth — ตรวจ router.go จริงใน Task 8 Step 1.
6. ลำดับ commit: Task 1 → 2 → 3 (build green) → 4 → 5 → 6 (build break) → 7 (build green อีกครั้ง) → 8 → 9.
```

---

แก้ plan v2 เสร็จ ครอบทุกประเด็นที่ review พบ. มา commit + เสนอ execute choice ใหม่
</thinking> |
<details>
<summary>Thought process</summary>

แก้ไขปัญหา plan เสร็จแล้ว ตอนนี้ commit และสรุปการเปลี่ยนแปลงให้ผู้ใช้ เพื่อให้เห็นว่าแก้ประเด็นไหนบ้าง

ประเด็นที่แก้:
1. Task 3 picker logic — ลบ `PickTopicCategory`/`PickArchetype` pure ที่แข่งกับ repo; เหลือ pure เฉพาะ `PickClipRole`/`PickPersona`. ใช้ repo `PickNextExclude` ทางเดียว
2. Task 7 placeholder — ลบ `SetPainCooldownDaysLookup(...)` และ struct field `o.currentArchetype`. ใช้ params ผ่าน produceClip chain หมด
3. Task 2 ratio collision — ตั้งชื่อ `catUsageRatio` private ไม่ชน `usageRatio`
4. Constructor — ใช้ setter (`SetThreshold`/`SetPainCooldownDays`/`Deduper()`) ไม่แตะ constructor signature, main.go ไม่ต้องแก้ constructor (แค่ NewOrchestrator)
5. produceClip signature — แสดงเต็มพร้อม theme/scriptCfg/imageCfg/brandAliases + เพิ่ม archetype/role

ความเปลี่ยนแปลงสถาปัตยกรรมที่ lock:
- per-batch picks (เพราะ scheduler count=1)
- setter-based threshold/cooldown (no constructor change)
- single picker path (repo PickNextExclude)

มา commit แล้วเสนอ execute

</details> |

คอมมิต plan v2 + สรุปการเปลี่ยนแปลง
</thinking> |
<details>
<summary>Thought process</summary>

ขออภัย มี tag ที่ไม่จำเป็นค้างอยู่ เนื้อหา plan ถูกเขียนเรียบร้อยแล้ว ตอนนี้แค่ commit + สรุปให้ user

</details> |</invoke>