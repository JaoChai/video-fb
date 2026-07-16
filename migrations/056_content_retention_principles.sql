-- 056 Content retention principles
-- Deep research (2026-07-16, primary sources TikTok/YouTube) established that
-- hook (first ~3s) + watch-to-completion is the strongest ranking lever.
-- Encode 3 principles into script/question/scene/critic prompts. Prompt-only,
-- no JSON output-shape change (avoids the 052 blank-narration regression).
-- Runs once via the schema_migrations gate. NOTE: the REPLACE is NOT self-
-- idempotent — each $new$ keeps the $old$ anchor as a prefix, so re-running this
-- file's raw SQL by hand would duplicate the appended rules. Do not re-run manually.

-- 1. script: 3-second spoken hook + open loop + clip arc
UPDATE agent_configs
SET prompt_template = REPLACE(
	prompt_template,
	$old$- "voice_script": สคริปต์สำหรับ voiceover ภาษาไทย สั้นกว่า answer_script 150-300 คำ จบด้วย CTA ชวนคอมเมนต์ใต้คลิปเหมือน answer_script$old$,
	$new$- "voice_script": สคริปต์สำหรับ voiceover ภาษาไทย สั้นกว่า answer_script 150-300 คำ จบด้วย CTA ชวนคอมเมนต์ใต้คลิปเหมือน answer_script

กติกา HOOK & ดูจนจบ (สำคัญสุดต่อการกระจายบน TikTok/Shorts):
- ประโยคแรกของ voice_script และ answer_script ต้องเป็น HOOK ที่ตรึงคนดูภายใน 3 วินาที (สั้น ไม่เกิน 15 คำ) เลือก 1 ใน 3 แบบ: (ก) ตั้งคำถามที่คลิปจะเฉลยทันที (ข) ตัวเลข/สถานะช็อก (ค) โยนผลลัพธ์หรือบทสรุปตอนจบขึ้นมาก่อน
- ห้ามเปิดด้วยการทวนคำถาม ทักทาย (เช่น "สวัสดีครับ") หรือเกริ่นยาว
- ใส่ OPEN LOOP ช่วงต้น (สัญญาว่าจะบอกวิธี/คำตอบท้ายคลิป) เพื่อดึงให้ดูจนจบ
- โครงคลิป: hook (ซีน 1 ราว 3 วิ) → ขยายเดิมพัน/ปัญหา → สเต็ปที่ทำตามได้จริง → payoff + CTA$new$)
WHERE agent_name = 'script';

-- 2. question: pick questions with a single-clip payoff + curiosity gap
UPDATE agent_configs
SET skills = REPLACE(
	skills,
	$old$- หมุนขั้ว hook อย่าให้ทุกข้อเป็น "เงินหาย": เงินหาย / เดดไลน์บีบเวลา / ป้ายสถานะถูกปฏิเสธ / เคลมที่สวนสามัญสำนึก$old$,
	$new$- หมุนขั้ว hook อย่าให้ทุกข้อเป็น "เงินหาย": เงินหาย / เดดไลน์บีบเวลา / ป้ายสถานะถูกปฏิเสธ / เคลมที่สวนสามัญสำนึก
- เลือกคำถามที่มี payoff ชัดเจน ตอบจบได้ใน 1 คลิป และเปิด curiosity gap (คนดูอยากรู้คำตอบ) — ไม่ใช่คำถามกว้างจนไม่มีบทสรุปในคลิปเดียว$new$)
WHERE agent_name = 'question';

-- 3. scene: tight fast-cut scenes + mid-clip re-hook (anti-skip)
UPDATE agent_configs
SET skills = REPLACE(
	skills,
	$old$- on_screen_text สั้น อ่านรู้เรื่องตอนปิดเสียง คุมความยาวตามลิมิตในกติกา$old$,
	$new$- on_screen_text สั้น อ่านรู้เรื่องตอนปิดเสียง คุมความยาวตามลิมิตในกติกา
- ทุกซีนสั้น ตัดไว หนึ่งซีนหนึ่งไอเดีย ห้ามซีนยาวหรือข้อมูลแน่นจนคนเบื่อ — ทุกซีนต้องทำให้คนดูอยากดูซีนถัดไป
- แทรก re-hook หรือจุดชวนสงสัยช่วงกลางคลิป (ราวซีนกลาง) เพื่อกันคนปัดหนีช่วงกลาง$new$)
WHERE agent_name = 'scene';

-- 4. critic: gate the spoken hook (scene-1 VoiceText) + open loop; fix, don't block
UPDATE agent_configs
SET skills = REPLACE(
	skills,
	$old$- Hook: scene แรก on_screen_text ต้อง <=7 คำ และช็อก/ชวนสงสัยใน 1 วิ ถ้ายาว/อืดให้ตัดให้สั้นคม$old$,
	$new$- Hook: scene แรก on_screen_text ต้อง <=7 คำ และช็อก/ชวนสงสัยใน 1 วิ ถ้ายาว/อืดให้ตัดให้สั้นคม
- Hook เสียงพูด: บทพูด (VoiceText) ของซีนแรกต้องเปิดด้วย hook ภายใน 3 วิ (คำถามที่จะเฉลย / ตัวเลข-สถานะช็อก / โยนผลลัพธ์ก่อน) ถ้าเปิดด้วยการทวนคำถาม ทักทาย หรือเกริ่นยาว ให้เขียนบทพูดซีนแรกใหม่ให้สั้นคม (แก้เฉพาะจุด ไม่ต้องรื้อทั้งบท)
- Open loop: ถ้าทั้งคลิปไม่มีจุดสัญญาว่าจะเฉลย/บอกวิธีท้ายคลิป ให้เติมประโยค open loop ในซีนต้นๆ$new$)
WHERE agent_name = 'critic';
