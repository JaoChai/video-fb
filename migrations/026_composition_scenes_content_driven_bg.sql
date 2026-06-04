-- Fix: composition_scenes agent was using bg_hint (a scene-type category label) as the
-- bg_art_prompt source, producing abstract/content-irrelevant art.  This update makes
-- the agent derive bg_art_prompt from the scene's actual spoken content (voice_text)
-- so the background image illustrates what is *literally being said* in that scene.
--
-- Additive UPDATE — no schema change, no new rows.
-- Rollback: execute 023's UPDATE SQL directly against the DB (the runner will not
-- re-apply 023 automatically).
-- Does NOT touch the 'composition' (single-scene) agent row.

UPDATE agent_configs
SET
    prompt_template = $tmpl$ออกแบบต่อฉากจากข้อมูลฉากนี้ (JSON):
{{.ScenesJSON}}

หมวด: {{.Category}} | ผู้ถาม: {{.QuestionerName}} | ความยาวรวม: {{.DurationSeconds}} วินาที

เลือก layout_variant ต่อฉากจาก: hook_big (ฉากเปิด/พาดหัวใหญ่), list_steps (ขั้นตอน/ลิสต์), stat_reveal (ตัวเลข/ผลลัพธ์เด่น), quote_cta (คำคม/ปิดท้ายชวนติดต่อ)
slots: ใส่ข้อความลงช่องตาม role ที่ layout รองรับ — headline (พาดหัว), body (เนื้อหา), badge (ป้ายเล็ก), step (เลขขั้นตอน)
emphasis: คำในข้อความที่อยากเน้นสี (0-2 คำ)

bg_art_prompt: ดู voice_text ของฉากนั้น → นึกว่า "ถ้าต้องทำภาพประกอบสิ่งที่กำลังพูดอยู่นี้ ภาพควรแสดงอะไร?"
  - เขียนเป็น subject/ฉากในภาพที่เป็นรูปธรรม สั้น ๆ (1-2 ประโยค)
  - ต้องสื่อเนื้อหาที่พูดจริงในฉากนั้น เช่น:
      • พูดเรื่องบัญชีโดนแบน → "หน้าจอ Facebook Ads Manager แสดงข้อความ account disabled พร้อมไอคอนคำเตือน"
      • พูดเรื่องยอดโฆษณาพุ่ง → "กราฟเส้นสีส้มพุ่งขึ้นชัน พร้อมตัวเลข ROAS ที่เพิ่มขึ้น"
      • พูดเรื่องขั้นตอนตั้งแคมเปญ → "มือกำลังคลิกปุ่ม Create Campaign บนหน้า Ads Manager"
  - ห้ามเขียนแบบ abstract ลอย ๆ หรือผูกกับ "หมวด" แบบเหมารวม
  - ไม่ต้องระบุสไตล์สี/แบรนด์/สั่งห้ามตัวหนังสือ — ระบบเติมให้อัตโนมัติ ให้โฟกัสแค่ "เนื้อหาในภาพ"

ตอบ JSON:
{
  "scenes": [
    {"scene_number":1,"layout_variant":"hook_big","accent_color":"#ff6b2b","animation_speed":"normal","bg_art_prompt":"หน้าจอ Facebook Ads Manager ที่บัญชีถูกระงับ มีแถบแจ้งเตือนสีแดง","slots":[{"role":"headline","text":"...","emphasis":["คำ"]}]}
  ],
  "kicker":"ป้ายหมวดสั้น ตัวพิมพ์ใหญ่",
  "highlight_words":["คำ1"]
}

แนวทาง: accent_color ตามอารมณ์ (ปัญหา/เตือน=#ff5a52, เทคนิค=#ff6b2b, อัปเดต=#3b82f6); ฉาก hook ใช้ hook_big, ฉาก cta ใช้ quote_cta$tmpl$,
    skills = $sk$- หนึ่ง scene_design ต่อหนึ่งฉากใน input (scene_number ตรงกัน)
- ข้อความใน slots สั้น กระชับ อ่านง่ายบนจอ
- ห้ามใส่ค่าพิกัด/ตำแหน่ง/ขนาด
- bg_art_prompt ต้องมาจาก voice_text ของฉากนั้น (เนื้อหาที่พูดจริง) ไม่ใช่ bg_hint หรือหมวดหมู่$sk$
WHERE agent_name = 'composition_scenes';
