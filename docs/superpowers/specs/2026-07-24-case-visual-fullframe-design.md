# Design: Case-File Visual Full-Frame — ภาพปก + จัดเต็มเฟรมทุกซีน

**วันที่:** 2026-07-24 (ต่อยอด spec case-file-format เดิมวันเดียวกัน)
**สถานะ:** อนุมัติดีไซน์แล้ว (brainstorming รอบ 2)
**ที่มา:** eyeball คดีที่ 146 — ครึ่งบนของเฟรมว่างเปล่า (พื้นที่ที่ template เดิมกันไว้ให้ภาพ AI แต่โหมดคดีตัดภาพออก) + user ต้องการภาพประกอบแบบไม่ต้องทุกซีน

## การตัดสินใจ (ยืนยันกับ user แล้ว)

| ประเด็น | การตัดสินใจ |
|---------|-------------|
| ขอบเขต | ภาพ AI เฉพาะซีนที่คุ้ม + จัดเต็มเฟรมทุกซีน — **ไม่แตะ motion เพิ่ม** |
| ภาพปก | สไตล์ "โต๊ะนักสืบ + ของกลางของเคส" (LLM เลือกของกลางตามเคสจริง) — แนวทาง A เจนตามเคส ไม่ใช่ baked asset |
| งบภาพ | **คงเดิม 2 ใบ/คลิป**: ปกแฟ้ม 1 + หลักฐาน 1 |
| Reuse | hero/verdict ใช้ไฟล์ภาพปกซ้ำเป็นพื้นจาง (JS ฝั่ง template — ฟรี ไม่เจนเพิ่ม) |

## ผังภาพต่อคลิป (9 ซีนตัวอย่าง)

casefile ✅เจนใบ 1 (โต๊ะนักสืบ) · comic ❌ · hero ♻️reuse ปก · evidence ✅เจนใบ 2 (โพลารอยด์) ·
board ❌ · stat ❌ · step ❌ · verdict ♻️reuse ปก → คนดูเห็นภาพ 4/9 ซีน จ่าย 2 ใบ

## องค์ประกอบใหม่

1. **ซีนปก (casefile) ผังใหม่ บน→ล่าง:** พาดหัว hook ใหญ่ (`cf-hook`, จาก on_screen_text +
   emphasis highlight — ข้อมูลมีอยู่แล้ว ไม่แตะ prompt) → ภาพโต๊ะนักสืบ (scene-bg, scrim อ่อนกว่าปกติ
   ให้ภาพอ่านออก) → แฟ้มคดี + ตรา (เดิม) — `justify-content:space-between` เต็มเฟรม
2. **ซีนไร้ภาพ (comic/board/step/stat):** ยกเนื้อหากึ่งกลางแนวตั้ง (top:150px;bottom:430px;center)
   + ลายน้ำเลขคดียักษ์จางมุมบนขวา (`cf-wm`, opacity ~0.07) + texture จุด halftone จางบน scene-bg
3. **hero/verdict:** ถ้าซีนไม่มี bg ของตัวเอง JS ใช้ `SCENES[0].bg` (ภาพปก) เป็นพื้น จาง+ลด
   saturation ผ่าน CSS — ให้ความรู้สึก "กลับมาที่โต๊ะคดี"
4. **comic ได้หัวซีน:** เพิ่ม `kicker` เข้า schema comic ใน prompt (template render kicker อยู่แล้ว)

## ส่วนที่ต้องแตะ

| ที่ | อะไร |
|----|------|
| migration 060 | UPDATE `scene_case` แถวเดียว: กฎภาพใหม่ (casefile ต้องมี image_prompt ฉากโต๊ะ+ของกลาง, evidence 1 ซีน, อื่น "") + comic schema เพิ่ม kicker — ใช้ REPLACE แบบ 056, idempotent |
| `case_format.go` | `evidenceImageScenes` → รับ casefile ด้วย (cap 2 เดิม); เพิ่ม `buildCoverPrompt` (composition: ฉากโต๊ะมุมสูง วัตถุครึ่งบน ล่างมืดเรียบ — ใช้ `buildImagePromptCore` ร่วม); `promptForScene` แยก 3 ทาง (cover/evidence/classic) |
| `scene_adapter.go` | casefile: set `SceneContent.Hook` (json `hook`) จาก on_screen_text + emphasis highlight |
| `composition.go`/types | `scenesTemplateData.CaseNumber` (ส่งเลขคดีให้ JS ทำลายน้ำ) |
| template | CSS `.cf-hook`/`.cf-wm`/texture/ตำแหน่งซีน + JS: hook ในซีนปก, ลายน้ำ, reuse bg hero/verdict |
| **ไม่แตะ** | script prompt, pipeline, QA/critic, publish, flag (`CASE_FORMAT_ENABLED` เดิมคุม) |

## กติกาที่สืบทอด (เหมือน spec แรกทุกข้อ)

ห้าม `-->` ใน script / Thai-safe CSS / GSAP-only / classic path byte-stable / fail-open:
ภาพปกเจนพลาด → circuit breaker เดิม → ปกกราฟิก CSS (แบบคดี 146) คลิปไม่ fail เพราะภาพ

## การทดสอบ

1. Unit: filter รับ casefile (อัปเดต test เดิมที่คาด evidence-only), buildCoverPrompt, adapter Hook field
2. Render test: `cf-hook`/`cf-wm`/CASE_NO ปรากฏในโหมด case; classic ไม่มี (test เดิมยืนยัน)
3. Prod: produce 1 คลิป เทียบคดีที่ 146 → user eyeball
