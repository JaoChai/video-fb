# Content Brain v2 — แก้ยอดวิวต่ำ + content วนซ้ำ (Design Spec)

**วันที่:** 2026-07-11
**สถานะ:** อนุมัติแนวทางแล้ว (brainstorm 2026-07-11) — รอ implementation plan

## 1. ปัญหา (ตรวจแล้วจาก prod DB + โค้ดจริง)

ยอดวิวจริง ณ 2026-07-11: YouTube 104 คลิป median 12 วิว / TikTok 51 คลิป median 107 วิว
(TikTok วิวไปหยุดที่ ~100–130 เกือบทุกคลิป = อัลกอริทึมแจกรอบแรกแล้วไม่ไปต่อเพราะ retention ต่ำ)

ต้นตอความซ้ำ เรียงตามน้ำหนัก:

1. **หมวดหัวข้อแคบ** — settings.categories มี 7 หมวด (account, payment, campaign, pixel, recovery, multi-account, scaling) ล้วนเป็นปัญหา account-ops มุมแคบ
2. **หมวดถูกล็อกรายสัปดาห์** — `orchestrator.go:131,140` ใช้ `weekNum % len(categories)` เป็น fallback + `PickCategoryWeighted` (`topic_pick.go`) 50% เลือกหมวดวิวดีสุดซ้ำ → ~21 คลิป/สัปดาห์วนอยู่ ~2 หมวด
3. **สูตรชื่อคลิปตายตัว** — format `qa` weight 2 (เท่าตัวของ format อื่น) + `news` หาข่าวสดไม่เจอ → fallback เป็น qa (`orchestrator.go:192-201`) → เกิดสูตร "คุณXครับ รบกวนปรึกษา..." ครองช่อง แม้แต่ tips/case_story ก็ติดสูตรนี้
4. **Dedup อ่อน** — เทียบ embedding เพื่อนบ้านใกล้สุดตัวเดียว threshold 0.78 (`dedup.go:16,67-73`), หัวข้อเดิมมุมใหม่ (similarity 0.69–0.75) รอดหมด, เทียบแค่ระดับ title, embedding ล่ม = fail-open รับหมด (`question.go:142`)
5. **Persona เดียวตายตัว** — setting `audience_persona` string เดียวทุกคลิป → framing ซ้ำ
6. **KB จิ๋วและนิ่ง** — 21 sources / 30 chunks, RAG query เดิมดึง top-5 เดิมวน → วัตถุดิบเดิมตลอด
7. **Feedback หยาบ** — ระบบรู้ผลงานแค่ระดับหมวด ไม่รู้ระดับหัวข้อ/hook (learner ตายสนิท 0 revisions)

## 2. การตัดสินใจจาก brainstorm

| คำถาม | คำตอบ |
|---|---|
| นิยามความสำเร็จ | **ผสม**: ~70% คลิปดึงวิว (reach) / ~30% คลิปปิดการขาย (convert) |
| แพลตฟอร์ม | คลิปเดียวยิงทั้ง YouTube + TikTok เหมือนเดิม |
| ระดับภาษากับกลุ่มสายเทา | **ภาษาคนวงใน** — pain เฉพาะคนบริหารหลายบัญชี/ยิงหนัก โดยตัวคลิปไม่ผิดนโยบายแพลตฟอร์ม และ**คง guardrail เดิม**: ห้ามสร้างเนื้อหาแนะนำการทำผิดนโยบาย Facebook / ไม่มีเทคนิคหลบระบบตรวจจับ |
| Human involvement | **Full-auto** เหมือนเดิม (3 คลิป/วัน 06:00/12:00/18:00) |
| แนวทาง | แนวทาง 1 (Content Brain Overhaul) เต็มตัว + ยืมของแนวทาง 2 เฉพาะส่วนถูก (insider KB pack + ทำให้ format news ทำงานจริง) — performance loop ระดับหัวข้อเลื่อนเป็น phase ถัดไป |

## 3. Design

ทุกอย่างอยู่หลัง flag ใหม่ `content_brain_v2_enabled` (settings, default `false`) — ปิด flag = พฤติกรรมเดิมทุก path, rollback ทันทีโดยไม่ revert โค้ด

### 3.1 หมวดหัวข้อ 10 หมวด + picker แบบ per-clip

ตารางใหม่ `topic_categories` (ตามแบบ `content_formats`):

| คอลัมน์ | ความหมาย |
|---|---|
| `category_name` | คีย์ เช่น `multi-account` |
| `display_name` | ชื่อไทย |
| `angle_instruction` | แนวมุมเล่าของหมวด (ป้อนเข้า question prompt) |
| `enabled`, `weight` | เปิด/ปิด + น้ำหนัก |

Seed 10 หมวด (map pain คนวงใน):

1. `multi-account` — บริหารหลายบัญชี/พอร์ต (มีเดิม)
2. `account-trust` — trust score, วอร์มบัญชี, พฤติกรรมบัญชีใหม่
3. `bm-structure` — โครงสร้าง BM/portfolio ไม่ให้พังยกแผง
4. `ban-signals` — สัญญาณเตือนก่อนโดนแบน, คลื่นกวาด (ban wave)
5. `recovery` — กู้บัญชี/appeal (มีเดิม)
6. `payment` — ระบบจ่ายเงิน/บัตร (มีเดิม)
7. `scaling` — สเกลงบ (มีเดิม)
8. `creative` — แอดเน่า/ครีเอทีฟตาย/CTR ร่วง (rebrand จาก campaign)
9. `tracking` — pixel/data/การวัดผล (rebrand จาก pixel)
10. `economics` — ต้นทุน ค่าธรรมเนียม ROI ของคนยิงหนัก (ใหม่)

Picker ใหม่ (flag on): least-used ภายใน 7 วันถ่วง weight (ท่าเดียวกับ `formats.PickNext`) + **ห้ามซ้ำหมวดภายในวันเดียวกัน** (3 คลิป/วัน = 3 หมวดต่างกัน) แทน weekly-lock + 50% exploit เดิม. `TopicPerformance` ยังแนบเป็นบล็อกสถิติใน prompt (ข้อมูลประกอบ) แต่ไม่บังคับทิศการเลือกอีก. **ข้อมูลเก่าไม่แตะ**: คลิป/หัวข้อเก่าคงค่าหมวดเดิม (campaign, pixel) ไว้ตามเดิม — หมวดใหม่ใช้เฉพาะคลิปใหม่ (บล็อกสถิติใน prompt เป็นข้อมูลประกอบ จึงมีชื่อเก่าปนได้ ไม่กระทบ logic).

### 3.2 บทบาทคลิป reach/convert (70/30)

- คอลัมน์ใหม่ `clips.clip_role` text (`reach`|`convert`), setting `clip_role_convert_ratio` default `0.30`
- Picker สุ่มต่อคลิปตามสัดส่วน (ไม่ต้อง exact ต่อวัน — ระยะยาวเฉลี่ยเอง)
- **reach**: question prompt สั่งหัวข้อ broad-appeal ในหมวดนั้น (ข่าวแรง/ตัวเลขช็อก/เตือนภัย/แฉความเชื่อผิด ที่คนยิงแอดทุกระดับหยุดดู); script CTA = ชวน follow/ดูคลิปต่อ
- **convert**: question prompt สั่งเจาะ pain ลึกของคนถือหลายบัญชี/คนกำลังโดนแบน; script CTA = ชวนทักแชท/ดูช่องทางใต้คลิปเรื่องบัญชีโฆษณา (soft sell — ไม่ใช่โฆษณาขายบัญชีโต้งๆ ทั้งคลิป)

### 3.3 คลังแม่แบบ hook/ชื่อคลิป (title archetypes)

ตารางใหม่ `title_archetypes` (`archetype_name, display_name, instruction, example, enabled, weight`) seed 7 แบบ:

| archetype | ตัวอย่างรูปหัวข้อ | weight |
|---|---|---|
| `shock_number` | "บัญชีตาย 40 ตัวในคืนเดียว เพราะพลาดจุดเดียว" | 2 |
| `warning` | "อย่าเพิ่งกดยืนยันตัวตน ถ้ายังไม่เช็ค 3 อย่างนี้" | 2 |
| `myth_bust` | "วอร์มบัญชี 7 วันแล้วรอด = เข้าใจผิด" | 2 |
| `story_twist` | "เอเจนซี่งบวันละแสน พังเพราะบัตรใบเดียว" | 2 |
| `question_tease` | "ทำไมบัญชีใหม่ยิงแล้วตายไว?" | 2 |
| `checklist` | "3 สัญญาณว่าบัญชีคุณกำลังจะโดนกวาด" | 2 |
| `consult_qa` | "คุณXครับ รบกวนปรึกษา..." (สูตรเดิม — คงไว้เป็นส่วนน้อย) | 1 |

- Picker: least-used/7 วันถ่วง weight (reuse ท่า `PickNext`)
- Question agent รับ `{{.ArchetypeInstruction}}` คุมรูปหัวข้อ; Script agent คุม `youtube_title` ให้ตาม archetype เดียวกัน
- Archetype ตั้งฉากกับ format: format คุมโครงเรื่อง (qa/news/tips/case_story), archetype คุมหน้าตา hook/ชื่อ — ผสมกันได้
- Rebalance format: ลด weight `qa` 2 → 1 (เท่ากันทุก format); `news` fallback เมื่อไม่มีข่าวสด → เลือก format least-used ตัวถัดไป (ไม่ fix เป็น qa)
- บันทึก archetype ที่ใช้ลง `clips.title_archetype` (ไว้ให้ performance loop phase หน้า)

### 3.4 Persona หมุนเวียน 4 ตัว

Setting ใหม่ `audience_personas` (JSON array) — flag on สุ่ม 1 ตัวต่อคลิป, flag off ใช้ `audience_persona` เดิม:

1. Media buyer ยิงหนัก งบวันละ 50k+ ถือหลายบัญชีพร้อมกัน — กลัวพอร์ตพังยกแผง
2. เจ้าของธุรกิจออนไลน์ ถือ 3–10 บัญชี ทำเองทุกอย่าง — เจ็บเรื่องบัญชีตาย/จ่ายเงินไม่ผ่านบ่อย
3. Agency ดูแลบัญชีลูกค้าหลายเจ้า — รับผิดชอบงบคนอื่น พลาดไม่ได้
4. คนเพิ่งโดนแบนครั้งแรก กำลังหาทางกลับมายิงต่อ — งง สับสน อยากได้คำตอบตรงๆ

บันทึกตัวที่ใช้ลง `clips.audience_persona`

### 3.5 Dedup hardening

- Threshold 0.78 → **0.72** ผ่าน setting ใหม่ `dedup_threshold` (จูนได้ไม่ต้อง deploy)
- **Theme guard ผ่าน pain_point cooldown**: question agent คืน `pain_point` อยู่แล้วแต่ไม่เคยเก็บ → เพิ่มคอลัมน์ `topic_history.pain_point` แล้วบังคับ: pain_point เดิมห้ามซ้ำภายใน N วัน (setting `pain_point_cooldown_days` default 5) — กัน "หัวข้อเดิมเปลี่ยนมุม" ที่ embedding จับไม่ได้
- **Fail-open → fail-closed แบบมีทางหนี**: embedding ล่ม → retry 1 ครั้ง → ยังล่ม → ใช้ lexical guard แทน (pg_trgm `similarity()` เทียบ 30 title ล่าสุด, block ถ้า > 0.5) แล้ว log warning — production ไม่สะดุดแต่ไม่รับมั่ว (ต้องเปิด extension `pg_trgm` ใน migration)
- พฤติกรรม retry เดิม (LLM คิดหัวข้อใหม่ได้ 2 ครั้ง) คงไว้

### 3.6 Insider knowledge pack + insider voice

- เขียน knowledge sources ใหม่ ~10 sources (แตกเป็น ~60+ chunks) ingest ผ่าน API เดิม (`POST /knowledge` → reindex, `handler/knowledge.go` + `rag.StoreChunk`) — ไม่ต้องแก้โค้ด ingest
- เนื้อหาต่อ source: pain scenarios + ศัพท์ + สถานการณ์จริงของแต่ละหมวดใหม่ทั้ง 10 (เช่น อาการก่อนโดนกวาด, เหตุผลที่บัญชีตายยกแผง, โครงสร้างพอร์ตแบบต่างๆ พร้อมข้อดีข้อเสีย, เส้นทาง appeal จริงและอัตรารอด, กำแพงการสเกลแต่ละช่วงงบ)
- **กติกาเนื้อหา (hard rule)**: เล่า pain + การบริหารความเสี่ยงเชิงโครงสร้าง/การป้องกันเชิงนโยบายได้; **ห้าม** สอนหลบระบบตรวจจับ, ปลอมตัวตน, หรือเทคนิคทำผิดนโยบายแพลตฟอร์ม — สอดคล้อง guardrail ที่มีอยู่แล้วใน question prompt
- Question + script prompt เพิ่มย่อหน้า insider voice: "พูดเหมือนคนที่บริหารบัญชีจำนวนมากมาเอง ใช้ศัพท์ที่คนวงในใช้ อ้างสถานการณ์ที่เฉพาะคนยิงหนักเจอ — ห้ามเนื้อหาระดับพื้นฐานทั่วไป (basic ads 101)"

### 3.7 สิ่งที่**ไม่ทำ**รอบนี้ (out of scope)

- Performance loop ระดับหัวข้อ/hook (รอ traffic โต — ตอนนี้สถิติเป็น noise)
- UI queue ให้คนรีวิวหัวข้อ (user เลือก full-auto)
- แตะ visual pipeline / rendering / จำนวนคลิปต่อวัน
- Auto-ingest ข่าวจากแหล่งภายนอกใหม่ (ใช้ research agent เดิมของ format news ต่อ)

## 4. Data flow (หลัง flag on)

```
ticker (3/วัน) → ProduceWeekly
  → pick category   (topic_categories: least-used 7d + ห้ามซ้ำในวัน)
  → pick format     (content_formats: least-used 7d, qa weight=1, news fallback→least-used)
  → pick archetype  (title_archetypes: least-used 7d)
  → pick role       (สุ่ม 70/30 reach/convert)
  → pick persona    (สุ่มจาก audience_personas)
  → QuestionAgent(category.angle + format + archetype + role + persona + RAG)
  → dedup (embedding 0.72 → pain_point cooldown 5d → [ถ้า embed ล่ม: pg_trgm guard])
  → ScriptAgent(role CTA + archetype title + insider voice) → SceneAgent → ... (เดิม)
  → บันทึก clips.clip_role / title_archetype / audience_persona + topic_history.pain_point
```

## 5. Error handling

- ตารางใหม่ว่าง/อ่านไม่ได้ → fallback พฤติกรรมเดิม (settings.categories, no archetype, persona เดิม) + log — ห้ามทำ production สะดุด
- Dedup: ตามข้อ 3.5 (fail-closed มีทางหนี lexical)
- News ไม่มีข่าวสด → fallback format least-used (ไม่ใช่ qa)

## 6. Testing & success criteria

**Unit tests:** picker หมวด (ห้ามซ้ำในวัน, least-used), picker archetype, role ratio, dedup threshold + pain_point cooldown + lexical fallback, news fallback ไม่เป็น qa

**End-to-end:** ผลิต 1 คลิปจริงบน flag on → render ผ่าน + eyeball ว่า title ไม่ใช่สูตร "คุณXครับ" (เมื่อ archetype ≠ consult_qa) + CTA ตรง role

**เกณฑ์วัดหลังปล่อย 3 วัน (9 คลิป):**
- ชื่อคลิปขึ้นต้น "คุณXครับ" ≤ 2/9
- ครอบคลุม ≥ 5 หมวด, ≥ 4 archetypes
- role split อยู่ช่วง 60/40–80/20
- ไม่มีคู่หัวข้อที่คนอ่านแล้วรู้สึก "เรื่องเดียวกัน" (eyeball)

**เกณฑ์เชิงทิศทาง (2 สัปดาห์ ไม่ใช่ gate):** TikTok median > 130 (หลุด initial pool), YouTube median > 25

## 7. Rollback

- `content_brain_v2_enabled=false` → กลับพฤติกรรมเดิมทุกอย่างทันที (picker เดิม, persona เดิม, dedup threshold เดิม)
- ตาราง/คอลัมน์ใหม่เป็น additive ทั้งหมด — ไม่มี destructive migration
- Insider KB pack: ลบได้ผ่าน API เดิม (sources ระบุชื่อชัด prefix `insider-`)

## 8. ไฟล์/ตารางที่แตะ (โดยประมาณ)

- Migrations: `topic_categories` (สร้าง+seed), `title_archetypes` (สร้าง+seed), `clips.clip_role/title_archetype/audience_persona`, `topic_history.pain_point`, extension `pg_trgm`, settings ใหม่ (flag, ratio, threshold, cooldown, personas), update `content_formats.weight`
- Code: `internal/orchestrator/orchestrator.go` (pipeline wiring), `internal/orchestrator/topic_pick.go` (picker ใหม่), `internal/repository/formats.go` (generic PickNext หรือ repo ใหม่), `internal/agent/question.go` + `internal/agent/script.go` (template args ใหม่), `internal/agent/dedup.go` (threshold/cooldown/lexical), prompt templates ใน `agent_configs` (migration)
- KB: ingest ผ่าน API หลัง deploy (script/คำสั่งแนบใน plan)
