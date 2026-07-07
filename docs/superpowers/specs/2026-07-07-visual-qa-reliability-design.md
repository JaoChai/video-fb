# Visual QA Reliability — Design Spec (2026-07-07)

## ปัญหา

ตั้งแต่ 2026-06-30 คลิป fail Visual QA พุ่งจาก ~10% → ~70% (6-7 ก.ค. fail 5/5)
ทุกคลิปที่ fail ถูก auto-review สั่ง hold แต่สุดท้ายถูก human กด Override publish ออกไปทั้งที่มีตำหนิ

## Root cause (จากการวิเคราะห์ 3 ทีม + ข้อมูล prod จริง)

**ตำหนิจริง (ตัวการหลัก) — Design Themes PR #14 (merge 1fb5398, 2026-07-02):**
- commit `b6ff577` สลับฟอนต์ heading ทั้งหมดจาก Sarabun → Kanit/Prompt (กลีฟไทยกว้างกว่า)
  แต่ font-size/ขนาดกล่องยังตั้งไว้สำหรับ Sarabun → ข้อความล้นกรอบ/ถูกครอปขอบจอ
- `.stat`(230px)/`.unit`(84px) ไม่มีทั้ง overflow protection และ length cap
  ซ้ำด้วย commit `2263109` บีบกล่อง stat แคบลง (left/right 56→110px) → "40,000 บาท" ตัดขอบ
- `overflow-wrap:anywhere` ตัดข้อความไทยกลางคำ ("ประ" โดดบรรทัด) แทนที่จะปล่อยให้
  Chromium ICU ตัดตามขอบเขตคำไทย

**QA ตรวจเกินจริง (false positive ~30-50% ของเคสข้อความ):**
- กติกา 1 ซีน fail = ทั้งคลิป fail — คลิป ~10 ซีน ขยาย per-scene FP เป็น clip fail แบบทวีคูณ
- แคปชั่นคาราโอเกะโชว์ทีละวลี (by design) → vision เห็น frame นิ่งตีความว่า "ข้อความถูกตัด"
- ซีนสั้น sample ที่ 60% ตกช่วง entrance animation → เห็น element กำลังเลื่อน = "ตัดขอบ/ซ้อนทับ"
- prompt บอกว่า on_screen_text "ควรจะเห็น" → โมเดล fail เพราะข้อความไม่ตรง ทั้งที่จอ render
  จาก `Content`/`VoiceText` คนละ field โดยดีไซน์
- (แก้ไปแล้ว: blank-frame จาก even-slicing เก่า + zero-duration scene — fix 07-03/07-07)

**ช่องโหว่ gate:**
- QA fail-OPEN: QA error/extract ไม่ได้ → คลิปผ่านเงียบๆ (`orchestrator.go:508`)
- `SetAutoReviewHeld` ไม่มี status guard → race เขียน hold ทับคลิปที่ published แล้ว
- PATCH `/clips/{id}` ตั้ง `status='published'` ตรงๆ ได้ → ข้าม publisher

## แนวทางที่เลือก (ทำทั้ง 3 track)

### Track A — แก้ตำหนิจริง (template)
1. **CSS** (`layout_multi_scene.html.tmpl`): ลบกฎบีบกล่อง stat (คืน default 56px),
   `.stat .unit` เป็น em-based (`.37em`) ให้ย่อตามตัวเลข, `.stat` เป็น `white-space:nowrap`,
   เปลี่ยน `overflow-wrap:anywhere`/`word-break:break-word` ทั้งหมด → `overflow-wrap:break-word`
   (ให้ Chromium ICU ตัดคำไทยตามพจนานุกรมก่อน break-word เป็นทางหนีสุดท้าย)
   และเพิ่ม element ที่ตกหล่น (.step-title,.step-of,.kicker,.sub,.brandbig,.chip .t) เข้าลิสต์
2. **Auto-fit JS**: หลัง build DOM วัด `scrollWidth` ของ `.stat`/`.chip .n` (element ที่ห้าม wrap)
   แล้วลด font-size จนพอดีกรอบ (ต่ำสุด 55%) — รองรับฟอนต์ Kanit/Prompt ที่กว้างขึ้นโดยไม่ต้อง
   revert ธีม; MOTION_V2 stat count-up วัดที่ค่าสุดท้าย (data-final) ก่อน fit

ตัดสินใจ: **ไม่ revert ฟอนต์ธีม** (เสีย variety ที่ตั้งใจทำ) — ใช้ auto-fit + wrap ที่ถูกต้องแทน

### Track B — แก้ QA ตรวจเกิน
1. **Two-strike confirm**: ซีนที่ pass แรก flag → สกัด frame ใหม่ที่ 85% ของซีน (พ้น entrance,
   แคปชั่นคนละวลี) แล้วตัดสินซ้ำ; ซีน fail จริงต่อเมื่อ**ทั้งสอง frame** ถูก flag
   (defect ที่ baked-in อยู่ทั้งซีนรอด 2 pass; FP จาก timing โดนเคลียร์) — แทนการผ่อน
   threshold 1-ซีน-fail ซึ่งจะปล่อยซีนพังจริงหลุด
2. **Sampling guard**: clamp timestamp ต่อซีนให้อยู่ใน [start+min(1.6s, 50% ของซีน), end−0.4s]
   — เลี่ยงทั้ง entrance และ transition
3. **Prompt (migration 051)**: สอน QA ว่า (ก) แคปชั่นล่างเป็นคาราโอเกะทีละวลี — วลีบางส่วน ≠
   ข้อความถูกตัด (ข) on_screen_text เป็น context ไม่ใช่ spec คำต่อคำ — ห้าม fail เพราะไม่ตรง
   (ค) element กลาง animation = ปกติ ตัดสินเฉพาะตำหนิที่นิ่งค้าง

### Track C — อุดช่องโหว่ gate
1. **Fail-closed flag** `QA_FAIL_CLOSED_ENABLED` (default off ตาม pattern RENDER_ERROR_GATE):
   เมื่อเปิด — QA enabled แต่ config fetch error หรือสกัด frame ไม่ได้เลย → `needs_review`
2. **`SetAutoReviewHeld`** เพิ่ม `AND status='needs_review'` กัน race
3. **PATCH guard**: handler ปฏิเสธ `status` = `published`/`producing` (pipeline เท่านั้นที่ตั้งได้)

## Out of scope
- Revert ฟอนต์/ธีม PR #14
- Thai word-segmentation library ฝั่ง Go (ใช้ Chromium ICU)
- เปลี่ยนพฤติกรรมปุ่ม Override ของ human (เป็น by-design; UI แสดงข้อมูลครบอยู่แล้ว)
- ปรับ auto-review agent

## Success criteria
- `go build ./... && go test ./...` ผ่าน
- Render ตัวอย่าง (eyeball หลัง deploy): ไม่มีข้อความล้นกรอบ/ตัดขอบจอ ในทุกธีม/ฟอนต์
- QA fail rate กลับสู่ระดับสะท้อนความจริง (คาด <20-30%) ภายใน ~1 สัปดาห์หลัง deploy
- คลิปที่ QA มองไม่เห็น (infra error) ไม่ publish เงียบๆ อีก (เมื่อเปิด flag)

## Rollback
- Track A/B โค้ด: revert commit (ไม่มี flag — เป็น bugfix ตรงๆ); migration 051 เป็น additive
  (append prompt) — ย้อนด้วย UPDATE ตัดข้อความที่เพิ่ม
- Track C1: ปิด flag `QA_FAIL_CLOSED_ENABLED`
