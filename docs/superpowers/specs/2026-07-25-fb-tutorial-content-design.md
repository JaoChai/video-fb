# FB Ads Tutorial Content — Design Spec

วันที่: 2026-07-25
สถานะ: อนุมัติดีไซน์แล้ว รอทำ implementation plan

## 1. เป้าหมาย

เพิ่มเนื้อหาสายใหม่: **tutorial ฟีเจอร์จริงใน Facebook Ads** วันละ 1 คลิป เจาะกลุ่ม
**คนยิงโฆษณาสายเทา** (persona `grey-operator` ที่มีอยู่ในระบบ) โดยคนดูต้อง
**ทำตามได้จริง** — เห็นชื่อเมนู เห็นปุ่ม เห็นค่าที่ต้องกรอก

เกณฑ์ความสำเร็จ: `retention_rate` / `avg_view_percentage` ของคลิป tutorial
สูงกว่าคลิปแฟ้มคดีอย่างมีนัยหลังผลิตครบ 14 คลิป

### ขอบเขตนโยบาย (ไม่ต่อรอง)

สอน **ฟีเจอร์จริงของ Meta ที่กดได้จริง** เท่านั้น — โครงสร้าง BM / การแยก asset /
การสำรองข้อมูล / การอ่านสถานะบัญชี / การคุมงบอัตโนมัติ
**ห้ามสอนวิธีหลบ detection หรือทำผิดนโยบาย** ตรงกับ BOUNDARY ที่
`topic_categories.grey-operator.angle_instruction` กำหนดไว้แล้ว

## 2. สถานะปัจจุบัน (ตรวจจากโค้ด + prod DB 2026-07-25)

- ผลิต 3 คลิป/วัน 06:00 / 12:00 / 18:00 (Asia/Bangkok) action `produce_and_publish`
- `CASE_FORMAT_ENABLED=true` บน Railway → **ทุกคลิปเป็นแฟ้มคดี** ผ่าน agent rows
  `script_case` / `scene_case` (resolve ด้วย `caseAgentConfig()` ที่ fail-open กลับ row เดิม)
- layout ที่ template รองรับ: `hook, hero, stat, step, tip, cta` + case `casefile, comic,
  evidence, board, verdict` — clamp ที่ `agent.ClampLayout()`
- ภาพ AI: kie gpt-image-2, โหมดคดีจำกัด 2 ใบ/คลิป (`caseImageScenes`)
- TikTok ปิดทั้ง 3 schedule rows เหลือ YouTube Shorts อย่างเดียว
- ตัวเลข 90 วันล่าสุด: 06:00 median 12 วิว / 12:00 median 12 / 18:00 median 14,
  retention_rate 2–14%, likes/comments ~0
  → **ที่ระดับวิวนี้ ความต่างระหว่างสล็อตคือ noise** เลือกเวลาจากพฤติกรรมผู้ชม ไม่ใช่จากสถิติชุดนี้

## 3. ปัญหาแกน + แนวทางที่เลือก

`CASE_FORMAT_ENABLED` เป็น env var ระดับ process → สลับ agent rows + preset + CSS +
นโยบายภาพพร้อมกันทั้งเซิร์ฟเวอร์ คลิป tutorial จึงอยู่ร่วมกับคลิปแฟ้มคดีไม่ได้

**แนวทางที่เลือก: ยกระดับเป็น "โหมดต่อคลิป"**

- โหมด = `classic` | `case` | `tutorial` **derive จาก `clips.content_format` ที่มีคอลัมน์อยู่แล้ว**
  (migration 017) → ไม่ต้องเพิ่มคอลัมน์ใหม่ และ resume/retry ได้โหมดถูกอัตโนมัติ
  - `content_format == 'tutorial'` → mode tutorial
  - ไม่ใช่ และ `CASE_FORMAT_ENABLED` → mode case
  - นอกนั้น → classic
- `caseAgentConfig()` → `modeAgentConfig(ctx, name, mode)` หา `<name>_tutorial` /
  `<name>_case` fail-open กลับ row เดิม (pattern เดิมจาก migration 059 ที่พิสูจน์แล้ว)
- template เพิ่ม block `[data-format='tutorial']` — วิธีเดียวกับที่ case format ทำสำเร็จ
- **layout ใหม่ตัวเดียว: `uistep`** ที่เหลือใช้ `hook/hero/tip/step/cta` ที่มีอยู่

แนวทางที่พิจารณาแล้วไม่เอา:
- *เพิ่มแค่ row ใน `content_formats`* — ตกโจทย์ คลิปยังออกมาเป็นแฟ้มคดีสืบสวน ไม่มี UI mock
- *pipeline แยกไฟล์ใหม่ทั้งชุด* — ก๊อป render/audio/QA/publish/retry ทั้งหมด หนี้ maintenance ยาว

## 4. กลุ่มเป้าหมาย → หัวข้อ

คนยิงสายเทาไม่ได้ขาดความรู้ยิงแอด เขาขาด **ระบบที่ไม่พังยกแผง** หัวข้อจึงแบ่ง 3 กลุ่ม:

| กลุ่ม | pain ที่ตรง | ตัวอย่างหัวข้อ |
|---|---|---|
| กันเผางบตอนบัญชีเริ่มพัง | `scaling_velocity_ceiling`, `account_burn_economics` | Automated Rules ตัดงบเมื่อ CPM พุ่ง, Account spending limit, ตั้ง alert |
| กัน asset ลาม / สำรองก่อนตาย | `cross_account_linking`, `asset_backup_strategy`, `banned_asset_data_loss` | Business Portfolio แยก asset, export Custom Audience, แชร์ dataset ข้าม BM, admin สำรอง + 2FA, Domain Verification, Business Verification |
| อ่านระบบให้ออกก่อนโดน | `review_queue_purgatory`, `circumvention_flag_trigger`, `landing_page_flag` | Account Quality อ่าน restriction reason, Ad review "See details", Ads Library ส่อง vertical เดียวกัน, Audience overlap |

หัวข้อที่เข้าเกณฑ์ทันที ~20 หัวข้อ ขยายเป็น 50–60 ได้ → วนวันละ 1 คลิปได้ ~2 เดือนก่อนซ้ำ

## 5. Catalog: `tutorial_features`

1 แถว = 1 คลิป เป็น **แหล่งความจริงเดียว** ของขั้นตอนและชื่อเมนู

| คอลัมน์ | ชนิด | ความหมาย |
|---|---|---|
| `id` | uuid pk | |
| `feature_key` | text unique | `automated_rules_cpm_guard` |
| `display_name_th` | text | ตั้ง Automated Rules ตัดงบเมื่อ CPM พุ่ง |
| `surface` | text | `ads_manager` \| `business_settings` \| `events_manager` \| `account_quality` |
| `menu_path` | text[] | `{'Ads Manager','Rules','Create new rule'}` |
| `ui_vocab` | text[] | ชื่อเมนู/ปุ่ม/ฟิลด์ **ทุกคำที่อนุญาตให้ขึ้นจอ** |
| `steps` | jsonb | `[{n, title_th, action_th, ui_target, value_th}]` 3–5 ขั้น |
| `trap_th` | text | กับดักที่คนตั้งผิดแล้วฟีเจอร์ไม่ทำงาน |
| `pain_point` | text | ผูกกับเมนู pain ของ `grey-operator` |
| `why_matters_th` | text | ความเสียหายที่กันได้ (วัตถุดิบของ hook) |
| `last_verified_at` | timestamptz | วันที่ยืนยันเมนูล่าสุด |
| `needs_verify` | bool | ธงให้คนมาแก้ ตั้งโดย research guard |
| `weight` | int | น้ำหนักการสุ่ม |
| `used_count`, `last_used_at` | int, timestamptz | สำหรับ picker |
| `enabled` | bool | |

### การเลือกหัวข้อ

least-used + weight + ไม่ซ้ำใน 30 วัน + `enabled AND NOT needs_verify`
(pattern เดียวกับ `topic_categories.PickNextExclude`)

### Research เช็คความสด

ก่อนเขียนบท research agent (มีอยู่แล้ว) ค้นว่าเมนูของ feature นี้เปลี่ยนไหม
- เจอหลักฐานว่า UI เปลี่ยน → ตั้ง `needs_verify = true` + ข้ามไปหัวข้อถัดไป
- **retry ได้สูงสุด 2 รอบ แล้ว fail-open ไปหัวข้อที่ `last_verified_at` ใหม่สุด**
  (บทเรียนจาก `project_question_cooldown_deadlock`: ทุก filter ที่ drop ได้ ต้องมี
  retry/fail-open ห้ามคืน 0 → เคยทำให้ produce 0 คลิปเงียบๆ 2 รอบ)

## 6. โครงคลิป — 8 ซีน / 50–58 วินาที

| # | เวลา | layout | หน้าที่ |
|---|---|---|---|
| 1 | 0–3.5s | `hook` | **ความเสียหายที่ฟีเจอร์นี้กันได้ ไม่ใช่ชื่อฟีเจอร์** |
| 2 | 3.5–9s | `hero` | สัญญา + บอกจำนวนขั้น ("3 ขั้น ทำครั้งเดียวจบ") = open loop ที่มีจุดจบ |
| 3 | 9–17s | `uistep` | ขั้น 1/N |
| 4 | 17–25s | `uistep` | ขั้น 2/N |
| 5 | 25–31s | `tip` | **re-hook กลางคลิป + กับดัก** (จาก `trap_th`) |
| 6 | 31–40s | `uistep` | ขั้น 3/N — ค่าที่ถูก |
| 7 | 40–48s | `step` | **การ์ดสรุปทุกขั้นใน 1 เฟรม ให้แคปหน้าจอเก็บ** (สัญญาณ save/share) |
| 8 | 48–55s | `cta` | soft close — โยงบัญชีที่ยังไม่โดนจำกัด |

- แถบ **"ขั้น n/N" ค้างบนจอตลอดซีน 3–6** = ตัวขับ retention ที่ตรงที่สุดของ tutorial
- ตารางข้างบนคือรูปแบบมาตรฐานตอน `len(steps) == 3` (กรณีที่พบบ่อยที่สุด)
  **จำนวนซีน `uistep` ต้องเท่ากับ `len(steps)` ใน catalog เสมอ** (ดู guard ข้อ 10)
  - `len(steps) == 4` → 9 ซีน, ยาว ~58–66s
  - `len(steps) == 5` → 10 ซีน, ยาว ~66–75s
  - ซีน `tip` (re-hook) วางก่อนขั้นสุดท้ายเสมอ ไม่ว่ามีกี่ขั้น
  - **catalog ห้ามมี feature ที่เกิน 5 ขั้น** — เกินนั้นให้แตกเป็น 2 feature
- `TargetDurationSec` ของ `scene_tutorial` คำนวณจาก `len(steps)` ตามตารางข้างบน
  (เดิม 30–55) ต้องตรวจว่า TTS / QA gate ไม่ตีตกที่ความยาวใหม่

## 7. Layout ใหม่ `uistep`

```
┌────────────────────────────────┐
│  ขั้นที่ 2 / 3        ●●○      │  progress rail
│  ตั้งเงื่อนไขให้ตัดงบ            │  หัวข้อขั้น (ไทย)
│  ┌──────────────────────────┐  │
│  │ ● ● ●   Ads Manager      │  │  แถบ chrome = "นี่คือหน้าจอ"
│  ├──────────────────────────┤  │
│  │ Rules › Create new rule  │  │  breadcrumb (จาก menu_path)
│  │  Campaigns               │  │  state: normal
│  │  Ad sets            ✓    │  │  state: done
│  │ ┌──────────────────────┐ │  │
│  │ │ Cost per result      │ │  │  state: target
│  │ │ ▸ Greater than       │ │  │  (กรอบส้ม + ลูกศรเด้ง 1 ครั้ง ไม่ loop)
│  │ │ ▸ 400 THB            │ │  │
│  │ └──────────────────────┘ │  │
│  │  Audiences               │  │
│  └──────────────────────────┘  │
│  เลือก Cost per result         │  callout ไทย: กดอะไร ใส่ค่าอะไร
│  แล้วใส่ 400 บาท               │
└────────────────────────────────┘
```

`content` ที่ `scene_tutorial` ต้องส่ง:

```json
{ "num": "2", "of": "ขั้นที่ 2 / 3", "title": "ตั้งเงื่อนไขให้ตัดงบ",
  "panel": { "chrome": "Ads Manager", "breadcrumb": "Rules › Create new rule",
             "items": [{"label":"Campaigns","state":"normal"},
                       {"label":"Ad sets","state":"done"},
                       {"label":"Cost per result","state":"target"}],
             "field": {"label":"Greater than","value":"400 THB"} },
  "callout": "เลือก Cost per result แล้วใส่ 400 บาท" }
```

- `state` มี 3 ค่าเท่านั้น: `normal` / `target` / `done`
- `items` สูงสุด 5 รายการ (กันล้นเฟรม), `label` ≤ 28 ตัวอักษร
- `callout` ≤ 60 ตัวอักษร, `title` ≤ 34
- **`chrome` / `breadcrumb` / `items[].label` / `field.label` ต้องมาจาก `ui_vocab`**
  ข้อความไทย (`title`, `callout`, `of`) เขียนอิสระได้

## 8. หน้าตาโหมด tutorial (`TutorialPreset`)

| | แฟ้มคดี (ปัจจุบัน) | tutorial (ใหม่) |
|---|---|---|
| อารมณ์ | สืบสวน มืด ดราม่า | คู่มือ สะอาด อ่านง่าย |
| พื้นผิว | กระดาษ / halftone / ตราประทับเอียง | การ์ด "หน้าจอ" สีอ่อนบนพื้น navy |
| Motion | `power4.out` 0.42s | `power2.out` 0.28s คม เร็ว |
| ตัวเน้น | ตราแดง/เขียว | ส้มแบรนด์ + ลูกศรชี้ |
| ฟอนต์ | Kanit / Sarabun | **เหมือนเดิม** (bundle แล้ว ไม่เพิ่มไฟล์) |

สี navy + ส้มของแบรนด์คงเดิมทั้งหมด — คนดูรู้ว่าช่องเดียวกัน แต่รู้ทันทีว่าคลิปคนละประเภท

`TutorialPreset` วางแบบเดียวกับ `CaseFilePreset`: **ไม่อยู่ใน `Presets`** เพื่อไม่ให้ตัวสุ่ม
หยิบไปใช้ — orchestrator เลือกให้ตรงๆ เมื่อ mode == tutorial

## 9. ภาพ — AI แค่ 1 ใบทั้งคลิป

- **ซีน 1 เท่านั้น** = ภาพปก / เฟรมแรก / thumbnail สไตล์ editorial photo
  (สื่อความเสียหาย เช่น มือถือแจ้งเตือนกลางดึก, โต๊ะทำงานไฟติดตีสาม)
  **ไม่ใช่** forensic แบบแฟ้มคดี
- **ซีน 2–8 ไม่มีภาพ AI** — UI mock + ข้อความล้วน
- เหตุผล: ภาพ AI แย่งความสนใจจากขั้นตอน + ลดต้นทุน kie ~50% + ถ้า kie ล่ม
  `PIPELINE_FAST_ENABLED` fail-fast ที่ 150s → พื้น CSS ทำงานปกติ **คลิปไม่พัง**
- ต้องขยาย `caseImageScenes` → `imageScenesForMode(scenes, mode)` (tutorial = cap 1, ซีนแรก)

## 10. Guard — 3 ชั้น (แทนสายตาคน)

ผู้ใช้เลือก auto-publish ตั้งแต่คลิปแรก ความเข้มจึงย้ายมาไว้ที่โค้ด:

1. **Whitelist `ui_vocab` (Go, deterministic, fail-closed)** — ก่อนเรนเดอร์ เช็คทุก
   `chrome` / `breadcrumb` / `items[].label` / `field.label` ว่าอยู่ใน `ui_vocab` ของ
   feature นั้น (normalize case + ช่องว่าง) เจอคำที่ AI แต่งเอง →
   **คลิปไม่ publish ไปเข้า `needs_review` + log ว่าคำไหน** ชั้นนี้ไม่พึ่ง LLM
2. **นับขั้นต้องตรง** — จำนวนซีน `uistep` == `len(steps)` ใน catalog เป๊ะ
   ไม่ตรง → `needs_review`
3. **`critic_tutorial`** (agent row ใหม่) — hook ต้องเป็นความเสียหายไม่ใช่ชื่อฟีเจอร์ /
   การ์ดสรุปต้องมีครบทุกขั้น / ห้ามมีคำแนะนำหมิ่นนโยบาย

ชั้น 1–2 เป็นโค้ดตรวจตายตัว — **นี่คือเหตุผลที่ auto-publish ปลอดภัยพอ**
คลิปที่ผ่านคือคลิปที่ทุกชื่อเมนูตรงกับ catalog ที่คนคุมเอง

`visual_qa` ต้องเติมเกณฑ์โหมด tutorial (การ์ดหน้าจอสีอ่อน / กรอบส้ม / ลูกศรชี้ = ดีไซน์
ที่ตั้งใจ ไม่ใช่ defect) แบบเดียวกับที่ migration 059 ทำให้โหมดคดี — กัน false positive
ตามบทเรียน PR #14 / #17

## 11. Schedule + rollback + วัดผล

**Schedule** — แก้ 1 แถว:
`Morning Produce & Publish` `0 6 * * *` `produce_and_publish`
→ `Tutorial Produce & Publish` `0 21 * * *` `produce_tutorial`

- action ใหม่ `produce_tutorial` ใน `internal/scheduler/scheduler.go` switch
- ผลิตเสร็จ publish ~21:15–21:25 (ไม่ชนงานอื่น: TikTok ปิด, analytics 04:00,
  retry `*/5` ไม่กิน production lock ยาว)
- **Rollback = UPDATE กลับ 1 คำสั่ง ไม่ต้อง deploy**

**วัดผล** — เทียบได้เพราะ `clips.content_format = 'tutorial'` แยกชัด:

- **KPI หลัก: `retention_rate` + `avg_view_percentage`** (ไม่ใช่ยอดวิว — ที่ 10–30 วิว
  เป็น noise)
- KPI รอง: median วิว 14 วัน, likes + shares, `subscribers_gained`
- **จุดตัดสิน: หลัง 14 คลิป (2 สัปดาห์)**
  - retention สูงกว่าแฟ้มคดีชัด → พิจารณาเพิ่มสล็อต
  - ต่ำกว่า → แก้ hook ก่อน แล้วค่อยตัดสินใจ

**ก่อนเปิด schedule จริง:** เรนเดอร์คลิปตัวอย่าง 1 ตัวให้ผู้ใช้ eyeball ของจริง
(ไม่ใช่แค่ mockup)

## 12. รายการของที่ต้องแตะ

| ไฟล์ / ของ | ทำอะไร |
|---|---|
| migration ใหม่ | สร้าง `tutorial_features` + seed ~20 แถว, เพิ่ม agent rows `script_tutorial` / `scene_tutorial` / `critic_tutorial`, เติมเกณฑ์ `visual_qa`, ย้าย schedule row |
| `internal/producer/` (ไฟล์ใหม่ `tutorial_format.go`) | `TutorialPreset`, `imageScenesForMode`, whitelist validator |
| `internal/producer/case_format.go` | generalize `promptForScene` / `caseImageScenes` ให้รับ mode |
| `internal/agent/scene_content.go` | เพิ่ม `uistep` ใน `sceneLayouts` |
| `internal/producer/scene_adapter.go` | map `panel` / `callout` / `items` → `SceneContent`, clamp ความยาว |
| `internal/producer/templates/layout_multi_scene.html.tmpl` | block `[data-format='tutorial']` + renderer ของ `uistep` + progress rail |
| `internal/orchestrator/orchestrator.go` | `ProduceTutorial()`, `modeAgentConfig()`, resolve mode จาก `content_format` |
| `internal/scheduler/scheduler.go` | case `produce_tutorial` |
| `internal/repository/` | repo ของ `tutorial_features` (pick / mark used / mark needs_verify) |

## 13. ไม่ทำในรอบนี้ (YAGNI)

- ไม่ทำ screenshot library ของจริง (เก็บไว้เป็นทางเลือกเสริมหลังวัดผล)
- ไม่ทำ playlist / ซีรีส์
- **ไม่แตะคลิปแฟ้มคดี 12:00 / 18:00 เลย** — output ต้อง byte-identical
- ไม่เปิด TikTok กลับ (ผู้ใช้สั่งพักไม่มีกำหนด)
- ไม่ทำ UI หน้าจัดการ catalog ในรอบนี้ — แก้ผ่าน SQL/migration ไปก่อน
