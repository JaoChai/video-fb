# Script Newsroom Debate — Design Spec

**วันที่:** 2026-07-18
**สถานะ:** อนุมัติ design แล้ว รอ implementation plan
**ที่มา:** แนวคิด "newsroom debate" จาก skill HearYourVOICE (github.com/killernay/HearYourVOICE) —
นักเขียน 3 คนเขียนแข่งกันแบบไม่เห็นกัน แล้วมี judge ตัดสิน ซึ่งจับ premise ผิดที่ single-pass มองข้ามได้

## เป้าหมาย

ยกระดับคุณภาพสคริปต์ (โดยเฉพาะ hook / retention ที่ลงทุนใน migration 056) ด้วยการเปลี่ยนขั้น script
จาก "เขียนรอบเดียว → critic แก้ทีหลัง" เป็น "เขียนแข่ง 3 มุมมองขนานกัน → judge เลือกผู้ชนะ + ดึงจุดเด่น"
โดยไม่กระทบความเสถียรของ pipeline (fail-open ทุกทาง) และ rollback ได้ด้วย flag ตัวเดียว

**ไม่อยู่ในขอบเขต:** debate ที่ขั้น question/hook แยก, มาตรฐาน research ใหม่, footage/Veo,
การเปลี่ยน TTS — วิเคราะห์แล้วเป็นโอกาสอนาคต ไม่ทำรอบนี้

## สถาปัตยกรรม (แบบ A ที่เลือก)

Debate อยู่ในระดับ orchestrator ใช้ script agent เดิมตัวเดียว ฉีด "lens instruction" ต่างกัน 3 แบบ
ต่อ call ผ่าน template field ใหม่ judge เป็น agent_config ใหม่ 1 ตัว

### การไหลของข้อมูล (เมื่อ flag เปิด)

```
question → script ×3 ขนาน (goroutine, เลนส์ต่างกันผ่าน {{.DebateLens}})
         → judge (`script_judge`) ให้คะแนน + เลือกผู้ชนะ + graft จุดเด่น
         → script ฉบับสุดท้าย (schema เดิมทุกประการ) → scene → critic → ... (เหมือนเดิม)
```

Judge คืน output ใน schema เดียวกับ script agent เดิม (answer_script / voice_script / metadata ฯลฯ)
บวก field คะแนน — ทุกอย่างหลังขั้น script ไม่รู้ความต่าง ไม่ต้องแก้ scene/critic

### ชิ้นส่วน

| ชิ้น | รายละเอียด |
|---|---|
| Flag | settings `script_debate_enabled` = `false` (default) — เปิด/ปิดจากหน้าเว็บ, rollback = ปิด |
| Lenses | settings `script_debate_lenses` = JSON array 3 ตัว `{key, name, instruction}` แก้ได้โดยไม่ deploy |
| Judge | agent_configs row ใหม่ `script_judge` (claude-sonnet-5, temperature ต่ำ ~0.3) |
| Template | เพิ่ม field `DebateLens` ใน `ScriptTemplateData` (internal/agent/script.go) + placeholder `{{.DebateLens}}` ใน prompt_template — flag ปิด/ไม่มีเลนส์ = แทนด้วยสตริงว่าง |
| Orchestrator | จุดเรียก script agent ใน internal/orchestrator/orchestrator.go แตกเป็น debate path เมื่อ flag เปิด |
| Audit | ตารางใหม่ `script_debates` (id, clip_id, candidates JSONB, verdict JSONB, created_at) |

### เลนส์ทั้ง 3 (seed เริ่มต้น — ปรับผ่าน setting ได้)

1. `hook_maximalist` — เขียนให้หยุดนิ้วแรงสุดใน 3 วิแรก กล้าตัดเนื้อหาที่ไม่เสริม hook
2. `skeptic_editor` — เข้มความแม่นของ claim ตัวเลข/นโยบาย Meta ห้าม oversell ห้ามเคลมที่พิสูจน์ไม่ได้
3. `target_viewer` — เขียนจากมุมคนดูจริงตาม audience persona ของรอบนั้น เน้นตรง pain ภาษาคนดูใช้จริง

ทั้ง 3 เลนส์ใช้ system_prompt + skills + insights ของ script agent เดิมร่วมกัน (learner ปรับที่เดียวมีผลทุกเลนส์)

## ความทนทาน — fail-open ทุกทาง

บทเรียนจาก cooldown deadlock (02778b3): ทุก path ที่ drop งานต้องมี fallback ห้ามคืน 0

| สถานการณ์ | พฤติกรรม |
|---|---|
| นักเขียนสำเร็จ ≥2 | ส่งเข้า judge ตามปกติ (2 candidate ก็ judge ได้) |
| สำเร็จแค่ 1 | ข้าม judge ใช้ฉบับนั้นเลย |
| สำเร็จ 0 | ถอยไปเรียก script single-pass แบบเดิม 1 ครั้ง |
| judge error / parse ไม่ได้ / output ไม่ผ่าน validate | ใช้ candidate ตัวแรกที่ validate ผ่าน (ไม่ retry judge) |
| flag ปิด | path เดิม 100% ไม่มี call เพิ่ม |

แย่สุดที่เป็นไปได้ = คุณภาพเท่าระบบปัจจุบัน ไม่มีทางที่คลิป fail เพราะ debate

Validate candidate ใช้เกณฑ์เดียวกับ output script ปกติ (struct + narration ต้องไม่ว่าง — บทเรียน regression 052:
เปลี่ยน prompt output ต้องแก้ struct + narration ด้วย; รอบนี้ schema output ไม่เปลี่ยน จึงไม่แตะ scriptNarration)

## ต้นทุน / เวลา

- ขั้น script: 1 → 4 LLM calls ต่อคลิป (3 ขนาน + 1 judge) ≈ ต้นทุนขั้น script ~4 เท่า
  (คิดเป็นส่วนน้อยของต้นทุนรวมต่อคลิปซึ่งมี scene/critic/image/visual_qa/auto_review เท่าเดิม)
- Latency เพิ่ม ~1 ช่วง call (3 ฉบับวิ่งขนาน) — ไม่กระทบ schedule 06:00/12:00/18:00 (3 คลิป/วัน)

## Migration 058

- หุ้ม `BEGIN; ... COMMIT;` เองทั้งไฟล์ (RunMigrations ไม่หุ้ม transaction — gotcha 057)
- Seed: agent_configs `script_judge` (system_prompt + prompt_template), settings `script_debate_enabled=false`,
  `script_debate_lenses` (JSON 3 เลนส์), ตาราง `script_debates`
- prompt_template ของ script เดิม: เพิ่ม `{{.DebateLens}}` ต่อท้าย (string-replace ธรรมดา ห้าม `{{if}}` —
  gotcha renderTemplate)

## การทดสอบ / verification

1. Unit tests: fallback ครบทุกแถวในตารางด้านบน, การฉีด lens ลง prompt, parse + validate judge output
2. หลัง deploy (flag ยังปิด): migration apply สำเร็จ, produce ปกติไม่เปลี่ยน
3. เปิด flag → `/produce` 1 คลิป → ตรวจ `script_debates` ว่ามี 3 candidates + verdict สมเหตุสมผล →
   eyeball คลิปจริง ก่อนปล่อยรัน schedule
4. Rollback: ปิด flag (ไม่ต้อง revert code)

## โอกาสอนาคตจาก HearYourVOICE (บันทึกไว้ ไม่ทำรอบนี้)

- มาตรฐาน research: 3 facts ขั้นต่ำ ≥2 แหล่งอิสระ + evidence log (ใช้กับเนื้อหาสายตัวเลข/นโยบาย)
- ขยาย debate ไปขั้น question (หลังประเมินผลรอบ script แล้ว)
- Verdict ใน `script_debates` เป็น input ให้ learner/analytics (เลนส์ไหนชนะบ่อย → ปรับ seed lens)
