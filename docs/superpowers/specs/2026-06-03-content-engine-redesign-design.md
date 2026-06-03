# Content Engine Redesign — แก้เนื้อหาซ้ำ + เพิ่มความสร้างสรรค์ + ตรงใจกลุ่มเป้าหมาย

**วันที่:** 2026-06-03
**สถานะ:** อนุมัติแล้ว (แนวทาง B — 2 เฟส)

## ปัญหา (จากการวิเคราะห์ด้วย agent team)

วิดีโอ 42 ตัว (27 เม.ย. – 2 มิ.ย.) มีเนื้อหาซ้ำเชิงสาระ ~65-75% และโครงสร้างสคริปต์ซ้ำ ~98% สาเหตุ:

1. **Auto-tune feedback loop** — weekly analyzer ใช้ LLM เขียนทับ `skills` ของ agent ตามยอดวิว
   รันแล้ว 3 ครั้ง (17/24/31 พ.ค.) ผลคือ skills ปัจจุบันสั่งให้ "เน้นหมวด Account/Payment,
   ห้ามสร้างคำถามเชิงเทคนิค" → ทำลายความหลากหลาย และจะแย่ลงทุกสัปดาห์
2. **KB นิ่ง** — knowledge_chunks มีแค่ 20 ก้อนค้างตั้งแต่ 29 เม.ย. เพราะ:
   - Schedule "Weekly Knowledge Crawl" ถูกเพิ่มใน DB หลัง server start → scheduler ไม่เคยรู้จัก
     (scheduler โหลด schedules แค่ตอน `Start()` ครั้งเดียว)
   - Crawler ออกแบบให้ดึงจาก URL (ผ่าน Jina Reader) แต่ KB ปัจจุบันเป็น text-based (url ว่างทุกแถว)
3. **กันซ้ำแบบ soft** — แค่ใส่รายการ 30 หัวข้อล่าสุดใน prompt บอกให้ LLM เลี่ยง
   ไม่มี semantic check → "Pixel ไม่นับ" กับ "No activity received" ถูกมองว่าไม่ซ้ำ
4. **Template ตายตัว** — รูปแบบเดียว (Q&A) + CTA เดียว + โครงเรื่องเดียว

## เป้าหมาย

- เนื้อหาไม่ซ้ำ (semantic dedup บังคับจริง ไม่ใช่แค่ขอร้อง LLM)
- เนื้อหาสร้างสรรค์หลากหลาย (4 รูปแบบ: Q&A / ข่าว Meta / ทิปส์ขั้นสูง / เคสจริง)
- ตรงใจกลุ่มเป้าหมาย (audience persona + เรียนรู้จากยอดวิวแบบมี guardrail)

---

## เฟส 1 — หยุดความซ้ำ (เร่งด่วน: auto-tune จะรันอีกครั้งวันจันทร์หน้า)

### 1.1 Auto-tune ใหม่: แยก insights ออกจาก skills

**หลักการ:** สิ่งที่มนุษย์/baseline กำหนด (skills) ต้องไม่ถูก LLM เขียนทับ
สิ่งที่ระบบเรียนรู้ (insights) ถูกจำกัดขอบเขตให้แนะนำได้แค่ "สไตล์การเล่า" ห้ามแตะ "การเลือกหัวข้อ"

**การเปลี่ยนแปลง:**

| ส่วน | เดิม | ใหม่ |
|------|------|------|
| `agent_configs.skills` | LLM analyzer เขียนทับทุกสัปดาห์ | Reset เป็น baseline ที่มนุษย์เขียน (เน้น diversity + persona) แล้วล็อกไว้ — analyzer แตะไม่ได้ |
| คอลัมน์ใหม่ `agent_configs.insights` | — | analyzer เขียนได้เฉพาะช่องนี้ จำกัดเฉพาะ style insights |
| `BuildSystemPrompt()` | system_prompt + skills | system_prompt + skills + insights |
| Analyzer prompt | "improve skills to produce engaging content" | เขียนใหม่: วิเคราะห์ได้เฉพาะ hook/จังหวะ/วิธีเปิดเรื่อง **ห้ามระบุหมวดหรือหัวข้อที่ต้องเน้น/เลี่ยงเด็ดขาด** |
| Code guardrail | ไม่มี | ตรวจ insights ก่อนบันทึก: ถ้ามี pattern สั่งเน้น/เลี่ยงหมวด (เช่น "เน้นหมวด", "หลีกเลี่ยงหมวด", "ห้ามสร้างคำถาม") → ปฏิเสธ เก็บค่าเดิม + log |
| ความยาว insights | — | จำกัด ≤ 1,000 ตัวอักษร |
| Prompt history | บันทึก skills เปลี่ยน | บันทึก insights เปลี่ยน (UI เดิมใช้ได้ต่อ) |

**สิ่งที่ต้อง reset ใน DB:** skills ปัจจุบันของ question/script/image (ที่ LLM เขียนจนเพี้ยน)
→ แทนด้วย baseline ใหม่ผ่าน migration

### 1.2 KB มีชีวิต

**การเปลี่ยนแปลง:**

1. **Crawler รองรับ 2 แบบ:**
   - Source ที่มี URL → crawl ผ่าน Jina Reader (logic เดิม) → ลบ chunks เก่าของ source นั้น → chunk + embed ใหม่
   - Source ที่เป็น text (url ว่าง) → ข้าม crawl แต่ถ้ายังไม่มี chunks → auto-embed
2. **Seed แหล่งข่าว URL จริง** (migration ใหม่): Meta Newsroom, Meta Business Updates, Search Engine Land (Meta section), r/FacebookAds — เพื่อให้รูปแบบเนื้อหา "ข่าว" มีวัตถุดิบสด
3. **เปลี่ยน crawl schedule เป็นรายวัน** (ตี 2) — ข่าวต้องสด
4. **Scheduler reload schedules** — เพิ่ม method `Reload()` ให้ scheduler และให้ schedules handler เรียกหลัง create/update/toggle → แก้ปัญหา "เพิ่ม schedule แล้วไม่ทำงานจนกว่าจะ restart"
5. **เติมเองได้** — หน้า Knowledge เดิมใช้ได้อยู่แล้ว (create + embed) ไม่ต้องแก้

### 1.3 Semantic Dedup (กันซ้ำจริง)

**Flow ใหม่ใน QuestionAgent.Generate():**

```
LLM สร้างคำถาม N ข้อ
  → สำหรับแต่ละข้อ: สร้าง embedding
    → เทียบกับ embedding ของหัวข้อเก่าทั้งหมดใน topic_history (pgvector cosine)
      → ถ้า similarity > 0.85 → ตัดทิ้ง + log ว่าซ้ำกับหัวข้อไหน
  → ถ้าเหลือไม่พอ (น้อยกว่าที่ขอ) → ขอ LLM สร้างเพิ่มอีกรอบ
     พร้อมบอกว่า "หัวข้อ X ถูกตัดเพราะซ้ำกับ Y" (สูงสุด 2 รอบ)
  → คำถามที่ผ่าน → บันทึกลง topic_history พร้อม embedding
```

**Schema:** เพิ่มคอลัมน์ `embedding vector(1536)` ใน `topic_history` + ivfflat index

---

## เฟส 2 — สร้างสรรค์ + ตรงใจกลุ่มเป้าหมาย

### 2.1 รูปแบบเนื้อหา 4 แบบ (Content Formats)

**ตารางใหม่ `content_formats`:**

| คอลัมน์ | ความหมาย |
|---------|----------|
| `format_name` | qa / news / tips / case_story |
| `display_name` | ชื่อแสดงผล |
| `question_instruction` | วิธีสร้าง "โจทย์" ของ format นี้ (แทรกเข้า question template) |
| `script_instruction` | วิธีเขียนสคริปต์ของ format นี้ (แทรกเข้า script template) |
| `enabled` | เปิด/ปิด format |
| `weight` | น้ำหนักการสุ่ม (default 1) |

**4 formats เริ่มต้น:**

1. **qa** — ลูกค้าถามปัญหา → ตอบวิธีแก้ (แบบเดิม)
2. **news** — อัปเดต/ข่าวจาก Meta ที่กระทบคนยิงแอด → ดึงจาก chunks ที่ crawl ล่าสุด (ภายใน 7 วัน) แทน RAG search ปกติ
3. **tips** — เทคนิคขั้นสูง เช่น การ scale งบ, โครงสร้างแคมเปญ, การทำ creative
4. **case_story** — เล่าเคสจริง มีตัวละคร มีปม มีทางออก (storytelling)

**การเลือก format:** orchestrator เลือกแบบ weighted rotation —
ดู format ที่ใช้ไป 7 วันล่าสุดจาก clips แล้วเลือก format ที่ถูกใช้น้อยที่สุด (ถ่วงด้วย weight)
→ การันตีว่าทุก format ได้ออกอากาศสม่ำเสมอ ไม่มี format ไหนหายไป

**Schema clips:** เพิ่มคอลัมน์ `content_format TEXT DEFAULT 'qa'`

**โครงสร้างเดิมที่คงไว้:** ยังใช้ QuestionAgent → ScriptAgent → ImageAgent → Producer เหมือนเดิม
แค่ template ที่ส่งให้ agent ถูกประกอบจาก format instruction + base template

### 2.2 Audience Persona

- Setting ใหม่ `audience_persona` (แก้ได้ในหน้า Settings):
  > "คนยิงแอด Facebook จริงจัง: เจ้าของธุรกิจออนไลน์, media buyer, agency
  > ที่เจอปัญหาบัญชี/ระบบจ่ายเงิน/ต้องการ scale — ต้องการความรู้เชิงลึก ไม่ใช่พื้นฐาน
  > และข่าวที่กระทบการทำงานจริง"
- แทรกใน question + script template: `กลุ่มเป้าหมาย: {{.AudiencePersona}}`
- Baseline skills ใหม่ (ข้อ 1.1) อ้างอิง persona นี้ด้วย

### 2.3 ความหลากหลายของสคริปต์

- Baseline skills ของ script agent กำหนด: หมุนเวียนวิธีเปิดเรื่อง (คำถามกระแทก / ตัวเลข / สถานการณ์),
  CTA มี 3 แบบให้สลับ
- ลบ CTA ตายตัวออกจาก template แล้วให้เป็นรายการใน skills แทน

---

## สิ่งที่ไม่ทำ (ตัดสินใจแล้ว)

- ไม่สร้าง Content Planner Agent ใหม่ (แนวทาง C) — ซับซ้อนเกินจำเป็น
- ไม่แตะ video rendering pipeline (producer/ffmpeg/kieai)
- ไม่เปลี่ยนจำนวนคลิปต่อวัน (ยังคง 1 คลิป/วัน ตาม Noon schedule)
- ไม่แก้ระบบ category rotation (weekNum % 4) — ความซ้ำไม่ได้มาจากตรงนี้

## ผลกระทบต่อไฟล์

**Backend (Go):**
- `internal/analyzer/analyzer.go` — เขียน prompt ใหม่ + เขียนลง insights + guardrail
- `internal/models/agent.go` — เพิ่ม Insights field + BuildSystemPrompt รวม insights
- `internal/repository/agents.go` — UpdateInsightsByName
- `internal/agent/question.go` — semantic dedup + format instruction + persona
- `internal/agent/script.go` — format instruction + persona
- `internal/rag/rag.go` — เพิ่ม SearchRecent (สำหรับ news format)
- `internal/crawler/crawler.go` — รองรับ text/URL sources + auto-embed
- `internal/orchestrator/orchestrator.go` — เลือก content format + ส่งให้ agents
- `internal/scheduler/scheduler.go` — Reload() method
- `internal/handler/schedules.go` — เรียก Reload หลัง CRUD
- `internal/repository/` — content_formats repo, settings (persona)

**Migrations:**
- `016_insights_and_dedup.sql` — insights column, topic_history embedding, reset skills เป็น baseline
- `017_content_formats.sql` — content_formats table + seed 4 formats, clips.content_format, audience_persona setting, news sources (URL), crawl schedule รายวัน

**Frontend (React):**
- Settings page — แสดง audience_persona (ถ้าเป็น generic key-value อยู่แล้ว ไม่ต้องแก้)
- หน้า Agents — แสดง insights field (read-only) แยกจาก skills

## เกณฑ์ความสำเร็จ

1. Build ผ่าน (`go build ./...` + `go test ./...` + frontend build)
2. Analyzer ใหม่ไม่สามารถเขียนคำสั่งเน้น/เลี่ยงหมวดได้ (มี unit test ยืนยัน guardrail)
3. คำถามที่ semantic similarity > 0.85 กับของเก่าถูกตัดทิ้ง (มี unit test)
4. Format rotation เลือก format ที่ใช้น้อยสุดใน 7 วัน (มี unit test)
5. Crawler ทำงานได้ทั้ง text source และ URL source
