# Design: "แฟ้มคดีเสี่ยงแบน" — Case-File Format สำหรับ Hyperframes

**วันที่:** 2026-07-24
**สถานะ:** อนุมัติดีไซน์แล้ว (brainstorming session) — รอเขียน implementation plan
**อ้างอิง:** artifact เสนอคอนเซ็ปต์ (แบบที่ 3) + impact map ที่ตรวจจากโค้ด/prod จริง

## 1. เป้าหมาย

เปลี่ยนคลิปจาก "สไลด์ความรู้" เป็น "คดีสืบสวน" — ทุกคลิปคือ 1 คดี: เปิดแฟ้ม → ดูหลักฐาน
→ ผังสาเหตุ → ทางรอด → ปิดคดีด้วยตราประทับ ทั้งภาพและเสียงพากย์ไปทางเดียวกัน
เหตุผลเชิงกลยุทธ์ (จากรีเสิร์ชเทรนด์ 2026 + วิเคราะห์คลิปจริง 23 ก.ค.):

- โครง "คดี" คือ open loop ธรรมชาติ — เปิดแฟ้มแล้วคนดูรอตอนปิดแฟ้ม (retention)
- เลขคดีสร้างความเป็น series สะสมข้ามคลิป
- เนื้อหาเดิมของเราเป็น "เคสความเสียหาย" อยู่แล้วทุกคลิป แค่ภาษาภาพไม่เล่าตาม
- ภาพ AI เปลี่ยนหน้าที่จากพื้นหลังเต็มจอ (โดน scrim ทับ 90%) เป็น "รูปหลักฐานในกรอบ"
  → เจนแค่ 1-2 ใบ/คลิป ต้นทุนลด ~70-80% และภาพเพี้ยนเล็กน้อยไม่พังเพราะกรอบเล่าเรื่องแทน

## 2. การตัดสินใจหลัก (ยืนยันกับ user แล้ว)

| ประเด็น | การตัดสินใจ |
|---------|-------------|
| ขอบเขต | **แทนที่ทุกคลิปแบบมี flag** — flag เปิด = ทุกคลิปใหม่เป็นแฟ้มคดี; ธีมเก่า 4 ตัวคงอยู่ในโค้ดเป็นทางถอย จน eyeball + รัน ~1 สัปดาห์ retention ไม่แย่ลง ค่อยลบจริงใน PR แยก |
| เสียงพากย์ | **เล่าเป็นคดีเต็มตัว** — บทพูดใช้ภาษาคดี ("เปิดแฟ้มคดีที่ N", "หลักฐานชิ้นแรก", "ปิดคดี") |
| เลขคดี | **นับจริงจาก DB** โผล่เฉพาะในคลิป (ปกแฟ้ม + ตราปิดคดี) — ไม่ใส่ใน YouTube title |
| ภาพ AI | **เฉพาะซีน `evidence` สูงสุด 2 ใบ/คลิป** ซีนอื่นเป็น CSS ล้วน |
| สถาปัตยกรรม | **แนวทาง A: ต่อยอด template เดิม** ผ่าน `data-format="case"` — ไม่แยกไฟล์ใหม่ เพื่อให้กลไกร่วม (แคปชั่น/เสียง/auto-fit/cover) มีที่เดียว (บทเรียน blank-video regression) |
| Prompt | **เพิ่มแถวใหม่ `script_case` + `scene_case`** ใน agent_configs — แถวเดิมไม่แตะ, flag ปิด = เส้นทางเดิม 100% |

## 3. Layout ใหม่ 5 ตัว (เพิ่มใน layout_multi_scene.html.tmpl)

| layout | ใช้ตอนไหน | โครง content JSON |
|--------|-----------|-------------------|
| `casefile` | ซีน 1 เสมอ (= poster frame ของ Cover W1) | `{caseNo, caseTitle, victim, damage, urgency}` — ปกแฟ้มสีกระดาษ + ตราแดง "ด่วนที่สุด" |
| `comic` | เล่าเหตุการณ์ 1-2 ซีน | `{panels:[{time, t, quote, dark}]}` 2-3 ช่อง, ช่อง `dark:true` = จังหวะดราม่าพื้นมืด, ลาย halftone |
| `evidence` | ซีนหลักฐาน (ซีนเดียวที่มีภาพ AI) | `{label, stamp, caption}` — โพลารอยด์ครอบภาพ AI + ตราแดงเฉียง (เช่น REJECTED) |
| `board` | ผังสาเหตุ / re-hook กลางคลิป | `{kicker, notes:[{t}]}` โน้ตกระดาษเหลืองติดหมุดแดง 2-3 ใบ โผล่ทีละใบตามจังหวะเสียง |
| `verdict` | ซีนปิดท้าย (แทน cta ในโหมดคดี) | `{caseNo, summary, stamp, cta, brand}` — ตราเขียว "ปิดคดี — รอดได้" + ปุ่ม CTA |

Layout เดิมที่ยังใช้ในโหมดคดี: `stat` (บังคับตัวเลขนับได้จริงเท่านั้น — ห้ามคำ), `step`, `hero`
Layout เดิมที่โหมดคดีไม่ใช้ (แต่โค้ดคงอยู่จนกว่าจะลบธีมเก่า): `hook`, `tip`, `cta`

กติกาที่สืบทอดจาก template เดิม (ต้องคงไว้):

- ห้ามเขียน `-->` ใน inline script (html/template ตัดบรรทัด → blank video)
- Thai-safe: letter-spacing ≥ 0, line-height ≥ 1.3, ไม่ใช้ word-break แบบ anywhere
- GSAP-only animation (seek-safe), ห้าม CSS keyframes ใน timeline
- ทุก element เข้าฉากผ่าน timeline เดียว ไม่มี tween ชนกัน

## 4. Flag + เส้นทางการทำงาน

- Flag ใหม่: env `CASE_FORMAT_ENABLED` (default false — ตามแนว flag อื่นของโปรเจกต์)
- orchestrator: flag เปิด → ใช้ agent rows `script_case`/`scene_case` + ส่ง format="case" ให้ producer
- producer → composition builder: ส่ง `Format` ลง template data → `data-format="case"` บน #root
- flag ปิด → ทุกอย่างเดินเส้นทางเดิมเป๊ะ (แถว prompt เดิม, layout เดิม, preset สุ่มเดิม)
- **Rollback = ปิด flag** ไม่ต้อง revert โค้ด/migration

## 5. เลขคดี

- migration: `ALTER TABLE clips ADD COLUMN case_number INT` (nullable, ไม่มี default)
- producer ตอนเริ่มผลิตคลิปโหมดคดี:
  `COALESCE(MAX(case_number), (SELECT COUNT(*) FROM clips WHERE status='published'))+1`
  → คดีแรกต่อจากจำนวนคลิปที่เผยแพร่จริง (~90+) — ไม่โกหกคนดู และดูมีประวัติ series ทันที
- ส่งเข้า template ผ่าน SceneContent (casefile + verdict scenes)
- คลิปโหมดเดิม: case_number = NULL (ไม่กระทบ)

## 6. ภาพ AI — บทบาทใหม่

- `scene_case` prompt สั่ง: image_prompt ใส่ได้เฉพาะซีน layout `evidence` สูงสุด 2 ซีน/คลิป
  ซีนอื่นต้องเป็น `""` — และ image_prompt ต้องบรรยาย "วัตถุหลักฐาน" (เช่น กล่องครีม, หน้าจอมือถือ)
- `presets.go`: เพิ่ม `StylePreset` key `case-file`:
  - ImageAnchor: "evidence photograph, harsh direct camera flash, slightly desaturated muted tones,
    plain neutral background, single centered subject, documentary forensic feel, no text"
  - Font: Sarabun + Kanit (โทนเอกสารราชการ)
  - Motion: entrance เร็วคม (สไตล์ตัดต่อสารคดี)
- `buildScenePrompt` รับ mode เพิ่ม: โหมด evidence **ไม่ใช้**กฎ "วัตถุครึ่งบน เว้นครึ่งล่าง"
  (ภาพอยู่ในกรอบโพลารอยด์ ไม่ใช่พื้นหลังใต้ text card) — ใช้ centered subject แทน
- flag เปิด → producer เลือก preset `case-file` ตายตัว (ข้ามระบบสุ่ม/epsilon-greedy)

## 7. ผลกระทบต่อ agent อื่น

| Agent | เปลี่ยนอะไร |
|-------|-------------|
| `script_case` (ใหม่) | โครงบท 5 จังหวะ: เปิดแฟ้ม → เดิมพัน/เหตุการณ์ → หักมุม/หลักฐาน → ทางรอด → ปิดคดี; คงกติกา retention เดิมจาก 056 (hook 3 วิ, open loop, re-hook กลางคลิป) แต่แปลงเป็นภาษาคดี |
| `scene_case` (ใหม่) | enum layout ใหม่ + โครง content ตามตาราง §3 + กติกา: ซีน 1 = casefile เสมอ, ซีนจบ = verdict เสมอ, stat ต้องเป็นตัวเลขนับได้, image_prompt เฉพาะ evidence ≤ 2 |
| `critic` (แก้แถวเดิม) | เพิ่มเกณฑ์: ซีน casefile ต้องมีชื่อคดีชวนเปิดดู — คง layout/โครงสร้างเดิมตามกติกาที่มีอยู่ |
| `visual_qa` (แก้แถวเดิม) | เพิ่มย่อหน้า: กรอบกระดาษ/โพลารอยด์เอียง/ตราประทับเฉียง/ลาย halftone = ดีไซน์ตั้งใจ ไม่ใช่ defect |
| Cover W1 | ไม่แก้ — ซีน 1 (casefile) เป็น poster frame ผ่านกลไกเดิม |

## 8. ส่วนที่ไม่เกี่ยวข้อง (ตรวจแล้ว ไม่แตะ)

TTS/เสียงพากย์, แคปชั่นคาราโอเกะ, ffmpeg/render pipeline, R2 storage, publish YouTube,
schedules, Question agent, dedup, script debate, analytics, retry/resume, auto_review flow

## 9. ส่วนที่จะลบ (หลังผ่านเกณฑ์ §10 — ทำเป็น PR แยกทีหลัง)

- Presets 4 ตัวเดิม (editorial-bold, cinematic-photo, neon-techno, soft-3d-clay)
- ระบบสุ่ม preset + epsilon-greedy (`PickPreset`, `PickPresetWeighted`) + flag `STYLE_PRESETS_*`
- Layout `hook`/`tip`/`cta` + CSS per-theme texture ใน template
- แถว agent `script`/`scene` เดิม (เมื่อ `_case` เป็นเส้นทางเดียว)

**ห้ามลบใน PR แรกเด็ดขาด** — ทั้งหมดนี้คือทางถอยระหว่างช่วงพิสูจน์

## 10. การทดสอบ + เกณฑ์สำเร็จ

1. Unit: ClampLayout รับ 5 layout ใหม่, buildSceneContent parse content ใหม่ครบ field,
   เลขคดี increment ถูก (รวม edge: คดีแรก, คลิปโหมดเดิมไม่ได้เลข)
2. Regression: flag ปิด → output identical กับปัจจุบัน (ทดสอบ template render ทั้ง 2 โหมด)
3. Local render จริง 1 คลิป: `hyperframes lint` + `inspect` (overflow audit) + `render` ผ่าน
4. Prod: deploy flag ปิด → เปิด flag → produce 1 คลิป → **eyeball โดย user** ก่อนปล่อยรันอัตโนมัติ
5. เกณฑ์ลบของเก่า: รัน ≥ 1 สัปดาห์ + retention เฉลี่ยไม่ต่ำกว่าช่วงก่อนเปิด + ไม่มี QA fail พุ่ง

## 11. ความเสี่ยงที่รู้ล่วงหน้า

- **Visual QA false positive**: ดีไซน์กระดาษ/ตราเอียงผิดจากธีมเดิมมาก → แก้ prompt ตาม §7 แล้ว
  ต้องดู fail rate หลังเปิด flag ใกล้ชิด (บทเรียนรอบ PR #14/#17)
- **GLM/LLM วนลูป wiring ข้ามไฟล์**: งานนี้แตะหลายไฟล์ประสานกัน — แผน implementation ต้องแบ่ง
  task ให้แต่ละชิ้นจบในไฟล์เดียวเท่าที่ทำได้ (บทเรียนจาก script debate)
- **ช่อง comic ข้อความล้น**: ต้องกำหนด max ตัวอักษรต่อช่องใน prompt + ผ่าน inspect gate เสมอ
