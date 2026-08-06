# เติมคลังหัวข้อพื้นฐาน 18 เรื่อง — ตำแหน่งโฆษณา แคมเปญ และการโฆษณาพื้นฐาน

วันที่: 2026-08-06
สถานะ: อนุมัติแล้ว รอ implement

## ปัญหา

คลัง `tutorial_features` ระดับ `basic` (ช่อง 15:00) มี 12 แถว ใช้วันละ 1 เรื่อง เหลือเรื่องที่ยัง
ไม่เคยออกอากาศแค่ 3 เรื่อง หลังจากนั้นตัวเลือกแบบ least-used จะเริ่มวนหัวข้อเดิม — เห็น
ตัวอย่างแล้วในฝั่ง advanced ที่ `audience_overlap_check` ถูกหยิบไป 3 รอบ

ช่องโหว่เนื้อหาที่ชัดที่สุดคือสามเรื่องที่คลังไม่มีเลย: **ตำแหน่งโฆษณา** (ไม่มีสักแถว),
**การสร้างและจัดการแคมเปญ** (มีแค่โครงสร้างชั้นกับงบ), และ **ชิ้นงานกับปลายทาง**
(ไม่มีเลยทั้งรูปแบบชิ้นงาน ช่องข้อความ ปุ่ม CTA และพิกเซล)

## แหล่งข้อมูลและวิธียืนยัน

เจ้าของเลือก: **เอกสาร Meta ทางการเท่านั้น** — Meta Business Help Center (`locale=en_US`)

WebFetch อ่านหน้าเหล่านี้ไม่ได้ (SPA คืนมาแต่ `<title>`) ต้องเปิดผ่าน Playwright แล้วดึง
`innerText` ของ `[role="main"]` บาง section เป็น accordion ต้องคลิกก่อนถึงเห็นขั้นตอน

ทุกแถวต้องบันทึก URL ของหน้า Help Center ที่ใช้ยืนยันไว้เป็นคอมเมนต์ในไฟล์ migration
เพื่อให้ตรวจย้อนได้ว่าชื่อเมนูมาจากไหน

### สิ่งที่ยืนยันแล้วในรอบแรก

- ขั้นตอน Placements จริง: `+ Create` → objective → `Continue` → ส่วน `Placements` ของ ad set
  → `Advantage+ placements` (ค่าเริ่มต้น) → hover → `Edit` → `Manual placements` →
  `Devices` / `Platforms` / `Placements` → `Allow limited spend to excluded placements` /
  `Manage excluded placements` → `Publish`
  (help/175741192481247)
- ตำแหน่งโฆษณาปี 2026 รวมของใหม่: **Threads feed**, **WhatsApp Status**;
  **ม.ค. 2026 Instagram Explore เลิกเป็น placement** (ไหลไป Instagram Reels แทน คงเหลือ
  `Instagram Explore home`); **มี.ค. 2026 Facebook Feed รวม Facebook Friends tab**
  (help/407108559393196)
- 6 วัตถุประสงค์ปัจจุบัน: `Awareness · Traffic · Engagement · Leads · App promotion · Sales`
  คู่กับ `Conversion location` และ `Performance goal` (help/1438417719786914)

## รายการหัวข้อ 18 เรื่อง

ทุกแถว `level='basic'`, `audience='beginner'`, `surface` = `ads_manager` เว้นที่ระบุ

### กลุ่ม A — ตำแหน่งโฆษณา (4)

| feature_key | เรื่อง | pain_point ใหม่ |
|---|---|---|
| `placement_advantage_vs_manual` | Advantage+ placements กับ Manual placements ต่างกันยังไง เปลี่ยนตรงไหน | `placement_mode_choice` |
| `placement_map_2026` | ตำแหน่งโฆษณามีที่ไหนบ้าง รวม Threads feed กับ WhatsApp Status | `placement_landscape` |
| `placement_breakdown_read` | ดูผลแยกตามตำแหน่ง ว่าเงินไปตกที่ตำแหน่งไหน | `placement_performance_read` |
| `audience_network_choice` | Audience Network คืออะไร ตัดทิ้งดีไหม กับ Allow limited spend | `audience_network_doubt` |

### กลุ่ม B — แคมเปญ (6)

| feature_key | เรื่อง | pain_point ใหม่ |
|---|---|---|
| `campaign_create_end_to_end` | สร้างแคมเปญแรกตั้งแต่ + Create จนกด Publish | `first_campaign_walkthrough` |
| `conversion_location_choice` | Conversion location เลือกปลายทางให้ตรงกับวัตถุประสงค์ | `conversion_location_confusion` |
| `campaign_duplicate` | คัดลอกแคมเปญแทนสร้างใหม่ | `duplicate_vs_new` |
| `campaign_off_vs_delete` | ปิด กับ ลบ ต่างกันยังไง | `off_vs_delete` |
| `ad_schedule_dayparting` | ตั้งเวลาให้แอดวิ่งเฉพาะบางช่วง | `ad_scheduling_setup` |
| `learning_phase_basics` | Learning phase คืออะไร ทำไมห้ามแก้บ่อย | `learning_phase_meaning` |

### กลุ่ม C — ชิ้นงานกับปลายทาง (8)

| feature_key | เรื่อง | pain_point ใหม่ |
|---|---|---|
| `ad_format_choice` | รูปเดี่ยว วิดีโอ คาร์รูเซล คอลเลกชัน เลือกอันไหน | `ad_format_choice` |
| `aspect_ratio_specs` | สัดส่วนภาพกับขนาดขั้นต่ำที่แต่ละตำแหน่งกิน | `creative_size_specs` |
| `ad_copy_fields` | Primary text / Headline / Description ช่องไหนคืออะไร | `ad_copy_field_meaning` |
| `cta_button_choice` | เลือกปุ่ม Call to action | `cta_button_choice` |
| `ad_preview_before_publish` | พรีวิวชิ้นงานก่อนกด Publish | `preview_before_publish` |
| `destination_choice` | ปลายทาง เว็บ / Messenger / WhatsApp / Instant form | `destination_choice` |
| `pixel_basics` | พิกเซลคืออะไร คนเพิ่งเริ่มต้องมีไหม | `pixel_basics` |
| `edit_running_ad` | แก้แอดที่รันอยู่แล้วเกิดอะไรขึ้น | `edit_running_ad` |

หัวข้อทั้ง 18 เลี่ยงการทับกับ 12 แถวเดิมแล้ว จุดที่ใกล้ที่สุดคือ `conversion_location_choice`
กับแถวเดิม `campaign_objective_pick` (pain_point `objective_mismatch`) — ของเดิมอยู่ชั้น
"เลือกวัตถุประสงค์" ของใหม่อยู่ชั้น "เลือกปลายทางภายใต้วัตถุประสงค์นั้น" คนละชั้นกัน

`feature_key` เดิมทั้ง 12 ที่ห้ามชนกัน: `basic_audience_setup`, `billing_receipt_read`,
`boost_vs_ads_manager`, `breakdown_age_gender`, `campaign_objective_pick`,
`campaign_structure_tour`, `daily_vs_lifetime_budget`, `date_range_picker`,
`delivery_column_check`, `first_payment_method`, `page_ad_account_link`,
`report_columns_basics`

## สิ่งที่ต้องแก้

### 1. `migrations/080_basic_catalog_expansion.sql`

สองส่วนในไฟล์เดียว ครอบด้วย `BEGIN`/`COMMIT` เอง (RunMigrations ไม่หุ้ม transaction ให้)

**ส่วนที่ 1 — ขยายเมนู pain_point** ของ `topic_categories` แถว `beginner`
เมนูปัจจุบันมี 12 ค่าฝังอยู่ใน `angle_instruction` (migration 071) และแถวใหม่ทุกแถวต้องใช้
ค่าจากเมนูนี้เท่านั้น จึงต้องต่อ 18 บรรทัดใหม่เข้าไป ใช้ `UPDATE ... SET angle_instruction =
replace(...)` โดยมี guard: ถ้าหาข้อความยึดไม่เจอให้ `RAISE EXCEPTION` แบบเดียวกับที่ 071 ทำ
กับ `script_basic` — การ replace ที่ไม่เจอจะเงียบและทำให้ agent เห็นเมนูเก่าตลอดไป

**ส่วนที่ 2 — INSERT 18 แถว** รูปแบบเดียวกับ 072 ทุกประการ:
`string_to_array($vocab$...$vocab$, '|')` สำหรับ ui_vocab และ `$steps$[...]$steps$` สำหรับ steps
พร้อมคอมเมนต์ URL ของหน้า Help Center ที่ยืนยันแต่ละแถว

Rollback: `DELETE FROM tutorial_features WHERE feature_key IN (...18 คีย์...)` และคืนเมนูเดิม

### 2. `internal/agent/tutorial_seed_test.go`

เพิ่ม `{"../../migrations/080_basic_catalog_expansion.sql", 18}` เข้ารายการ seed ที่ตรวจ
เทสต์นี้บังคับสองข้อกับทุกแถว: `ui_target` ของทุกขั้นต้องอยู่ใน `ui_vocab` ของแถวเดียวกัน
(ถ้าหลุด gate ตอนรันจริงจะตีคลิปตกทุกใบโดยไม่มีใครรู้ว่าสาเหตุอยู่ที่ seed) และแต่ละแถว
ต้องมี 3-5 ขั้น

## ข้อจำกัดที่ต้องเคารพ

- **`needs_verify` ไม่ใช่ตาข่าย** — `PickNext` ของฝั่ง tutorial กรองด้วย
  `enabled = TRUE AND (parked_until IS NULL OR parked_until <= NOW())` เท่านั้น ไม่ได้ดู
  `needs_verify` (ต่างจากฝั่ง myth ที่กรอง) แถวที่ตั้งธงไว้ก็ยังถูกหยิบไปทำคลิปอยู่ดี
  จึงต้องยืนยันข้อมูลให้ครบ **ก่อน** insert
- **ห้ามแตะแถว level `advanced`** — ช่อง 21:00 ที่วิ่งอยู่ต้องไม่กระทบ
- `TutorialMinPool = 10` คือพื้นคลังที่ระบบ two-strike ห้ามพักเกิน — การเติม 18 แถวยกพื้นนี้
  ให้ปลอดภัยขึ้นมาก (12 → 30)

## ผลที่จะเกิดกับช่อง

ตัวเลือกใช้ least-used ดังนั้นหลัง deploy คลิป 15:00 จะเป็นเรื่องใหม่ล้วนติดกัน 18 วัน
แล้วจึงวนกลับไปเรื่องเดิม คลังรวมเป็น 30 เรื่อง = ครบรอบทุก 1 เดือน

## แผนตรวจ

1. `go test ./...` ผ่าน — เทสต์ seed จับ ui_target/ui_vocab ที่ไม่ตรงและจำนวนขั้นที่ผิด
2. รัน migration บน Neon branch แยกของ `snowy-grass-75448787` ไม่ใช่ default branch
3. บน branch นั้น ตรวจว่า `SELECT` ด้วยเงื่อนไขเดียวกับ `tutorialPickSQL` คืน 30 แถว และ
   แถวที่ least-used เป็นแถวใหม่
4. ตรวจว่า `angle_instruction` ของ `beginner` มีครบ 30 pain_point และไม่มีค่าซ้ำ
5. รายงานผลให้เจ้าของตัดสินใจ merge — ไม่ merge เอง
