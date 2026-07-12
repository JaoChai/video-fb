-- 051_visual_qa_caption_context.sql
-- Teach visual_qa three runtime facts that were causing false positives:
-- 1) the bottom caption box is a karaoke subtitle: it reveals the narration one
--    SHORT PHRASE at a time — a partial phrase is normal, not truncated text;
-- 2) on_screen_text is scene context (what the scene should convey), NOT a
--    verbatim spec — the screen renders from structured Content/VoiceText;
-- 3) a frame may be captured mid entrance/exit animation — judge only defects
--    that are static (e.g. a fully-settled headline overflowing its box).

UPDATE agent_configs SET
  system_prompt = system_prompt || E'\n\nข้อเท็จจริงของระบบ render (สำคัญมาก):\n- กล่องแคปชั่นล่างสุด (กรอบขอบส้ม) คือซับไตเติลคาราโอเกะ: แสดงบทพากย์ทีละ "วลีสั้นๆ" ไม่ใช่ประโยคเต็ม — วลีสั้น/ขึ้นต้นกลางประโยค = ปกติ ห้ามตีความว่า "ข้อความถูกตัด/อ่านไม่ครบ".\n- on_screen_text คือ "สาระที่ซีนควรสื่อ" ไม่ใช่ข้อความที่ต้องปรากฏคำต่อคำ — ห้ามตั้ง ok=false เพียงเพราะข้อความบนจอไม่ตรง on_screen_text.\n- เฟรมอาจถูกจับระหว่างอนิเมชันเข้า/ออก: องค์ประกอบที่กำลังเลื่อน/จาง/ยังไม่นิ่ง = ปกติ. ตั้ง ok=false เฉพาะตำหนิที่ "นิ่งค้าง" เช่น หัวข้อหลักล้นกรอบ/ถูกครอปทั้งที่แสดงเต็มที่แล้ว.',
  skills = skills || E'\n- แคปชั่นคาราโอเกะล่างจอขึ้นทีละวลี — วลีบางส่วน ≠ ข้อความถูกตัด.\n- on_screen_text = context ไม่ใช่ spec คำต่อคำ — ไม่ตรง ≠ พัง.'
WHERE agent_name = 'visual_qa' AND system_prompt NOT LIKE '%คาราโอเกะ%';

-- Reword the per-frame prompt so the model stops treating on_screen_text as a
-- verbatim requirement. replace() is idempotent (no-op once rewritten).
UPDATE agent_configs SET
  prompt_template = replace(prompt_template,
    E'ข้อความบนจอที่ "ควร" จะเห็น (on_screen_text): ',
    'สาระที่ซีนนี้ควรสื่อ (context — ไม่จำเป็นต้องตรงคำต่อคำ): ')
WHERE agent_name = 'visual_qa';
