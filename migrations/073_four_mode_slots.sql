-- 073: 4 ช่วงเวลา 4 โหมด (spec 2026-07-28)
-- agent 3 แถวต่อโหมดใหม่ + ย้าย schedule 18:00 ไป action ของตัวเอง
-- idempotent: ON CONFLICT DO NOTHING ทุก INSERT (agent_name มี UNIQUE ตั้งแต่ 001)
-- และ UPDATE ที่ท้ายไฟล์มีเงื่อนไข action เดิม รันซ้ำครั้งที่สองจึงไม่แตะอะไร
-- RunMigrations ไม่หุ้ม transaction ให้ — ต้อง BEGIN/COMMIT เอง

BEGIN;

-- ── โหมดแชท (18:00) ──
INSERT INTO agent_configs (agent_name, system_prompt, model, temperature, enabled, prompt_template)
SELECT 'script_chat',
       system_prompt || E'\n\nโหมดแชท: เขียนบทเป็นบทสนทนาจริงระหว่างลูกค้ากับเรา ' ||
       'ลูกค้าเล่าปัญหาด้วยภาษาพูดสั้นๆ ทีละข้อความ (ไม่ใช่ย่อหน้ายาว) เราตอบทีละข้อความเช่นกัน ' ||
       'ห้ามบรรยายเหตุการณ์จากมุมผู้เล่าเรื่อง ให้เล่าผ่านสิ่งที่ทั้งสองฝ่ายพิมพ์คุยกัน ' ||
       'ปิดท้ายด้วยข้อสรุปที่ลูกค้าเอาไปทำต่อได้ทันที',
       model, temperature, TRUE, prompt_template
FROM agent_configs WHERE agent_name = 'script_case'
ON CONFLICT (agent_name) DO NOTHING;

INSERT INTO agent_configs (agent_name, system_prompt, model, temperature, enabled, prompt_template)
SELECT 'scene_chat',
       system_prompt || E'\n\nโหมดแชท: ใช้ layout ได้เฉพาะ hook, chat_in, chat_out, recap, cta ' ||
       'ห้ามใช้ layout ของโหมดอื่น (casefile, evidence, comic, board, verdict, uistep, dashboard, alarm)',
       model, temperature, TRUE,
       prompt_template || E'\n\nรูปแบบซีนของโหมดแชท:\n' ||
       '- chat_in = ลูกค้าถาม: {"type":"chat_in","asker":"ชื่อลูกค้า","stamp":"21:47 น.",' ||
       '"msgs":[{"from":"them","t":"ข้อความ"},{"from":"them","t":"ข้อความเสี่ยง","alert":true}]}' || E'\n' ||
       '- chat_out = เราตอบ: {"type":"chat_out","msgs":[{"from":"me","t":"คำตอบ"},' ||
       '{"from":"them","t":"ถามต่อ"}],"verdict":"ข้อสรุปสั้นๆ"}' || E'\n' ||
       '- recap = สรุปท้าย: {"type":"recap","title":"หัวข้อ","chips":[{"n":"72","t":"ชั่วโมง"}]}' || E'\n' ||
       'กติกา: 1 ฟองข้อความ = 1 ประโยคสั้น ห้ามเกิน 90 ตัวอักษร · alert ใช้ได้ไม่เกิน 1 ฟองต่อซีน · ' ||
       'ซีนแรกเป็น hook เท่านั้น (ซีนเดียวที่มีภาพ AI)'
FROM agent_configs WHERE agent_name = 'scene_case'
ON CONFLICT (agent_name) DO NOTHING;

INSERT INTO agent_configs (agent_name, system_prompt, model, temperature, enabled, prompt_template)
SELECT 'critic_chat',
       system_prompt || E'\n\nโหมดแชท: ตรวจเพิ่มว่าบทเป็นบทสนทนาจริง ไม่ใช่บรรยาย ' ||
       'ทุกฟองข้อความต้องสั้นแบบที่คนพิมพ์จริง และคำตอบของเราต้องมีสิ่งที่ทำต่อได้ ไม่ใช่แค่ปลอบใจ',
       model, temperature, TRUE, prompt_template
FROM agent_configs WHERE agent_name = 'critic'
ON CONFLICT (agent_name) DO NOTHING;

-- ── โหมดห้องควบคุม (21:00) ──
-- คัดลอกจากคู่สอนเดิมทั้งหมด เพราะกติกาการสอน (ห้ามแต่งชื่อเมนู ขั้นตอนต้องครบ)
-- ต้องเหมือนเดิมเป๊ะ เปลี่ยนแค่ layout ที่ห่อรอบ uistep
INSERT INTO agent_configs (agent_name, system_prompt, model, temperature, enabled, prompt_template)
SELECT 'script_warroom',
       system_prompt || E'\n\nโหมดห้องควบคุม: เปิดคลิปด้วยตัวเลขที่ผิดปกติจริงจากหน้าจอ ' ||
       '(เช่น CPM พุ่ง ROAS ตก) แล้วค่อยพาไปที่วิธีแก้ ผู้ชมคือคนยิงแอดอยู่แล้ว ' ||
       'ใช้ศัพท์เทคนิคได้ตรงๆ ไม่ต้องอธิบายพื้นฐาน',
       model, temperature, TRUE, prompt_template
FROM agent_configs WHERE agent_name = 'script_tutorial'
ON CONFLICT (agent_name) DO NOTHING;

INSERT INTO agent_configs (agent_name, system_prompt, model, temperature, enabled, prompt_template)
SELECT 'scene_warroom',
       system_prompt || E'\n\nโหมดห้องควบคุม: ใช้ layout ได้เฉพาะ dashboard, uistep, alarm, cta',
       model, temperature, TRUE,
       prompt_template || E'\n\nรูปแบบซีนของโหมดห้องควบคุม:\n' ||
       '- ซีนแรกเป็น dashboard เสมอ (ซีนเดียวที่มีภาพ AI): {"type":"dashboard",' ||
       '"statLabel":"CPM 7 วันล่าสุด","chips":[{"n":"+38%","t":"CPM","bad":true},{"n":"1.4x","t":"ROAS"}],' ||
       '"callout":"บรรทัดเตือนสั้นๆ"}' || E'\n' ||
       '- ซีนขั้นตอนใช้ uistep เหมือนเดิมทุกฟิลด์ ห้ามเปลี่ยนโครงสร้าง' || E'\n' ||
       '- alarm = สรุปสิ่งที่ต้องทำ: {"type":"alarm","title":"หัวข้อ",' ||
       '"rows":[{"t":"ทำอะไร"},{"t":"ถ้าไม่ทำจะเจออะไร","bad":true}]}' || E'\n' ||
       'ข้อบังคับ: จำนวนซีน uistep ต้องเท่ากับจำนวนขั้นในคลังเป๊ะ ห้ามขาดห้ามเกิน ' ||
       'และชื่อเมนูทุกคำต้องมาจาก ui_vocab ที่ให้มาเท่านั้น'
FROM agent_configs WHERE agent_name = 'scene_tutorial'
ON CONFLICT (agent_name) DO NOTHING;

INSERT INTO agent_configs (agent_name, system_prompt, model, temperature, enabled, prompt_template)
SELECT 'critic_warroom', system_prompt, model, temperature, TRUE, prompt_template
FROM agent_configs WHERE agent_name = 'critic_tutorial'
ON CONFLICT (agent_name) DO NOTHING;

-- ── schedule 18:00 ไป action ของตัวเอง ──
-- scheduler โหลดตารางจาก DB ตอน start อยู่แล้ว การ deploy จึงทำให้แถวนี้มีผลทันที
UPDATE schedules SET action = 'produce_evening'
WHERE cron_expression = '0 18 * * *' AND action = 'produce_and_publish';

COMMIT;
