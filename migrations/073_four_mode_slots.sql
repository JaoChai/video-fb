-- 073: 4 ช่วงเวลา 4 โหมด (spec 2026-07-28)
-- agent 3 แถวต่อโหมดใหม่ + ย้าย schedule 18:00 ไป action ของตัวเอง
--
-- prompt_template ของ scene_chat/scene_warroom เขียนใหม่ทั้งฉบับ ไม่ใช่ต่อท้ายของเดิม
-- เพราะ prompt ของ scene_case/scene_tutorial เป็นสัญญาฉบับสมบูรณ์ในตัวเอง ("ซีนแรก
-- layout casefile เสมอ") การต่อท้ายว่า "ใช้ได้เฉพาะ chat_in" จึงได้ prompt ที่ขัดกันเอง
-- แล้วโมเดลจะเลือกทำตามอันไหนก็ได้ — 059 กับ 063 ก็เขียนใหม่ทั้งฉบับด้วยเหตุผลเดียวกัน
-- JSON output contract (scene_number/voice_text/on_screen_text/emphasis_words/
-- caption_style/image_prompt/layout/content) คงเดิมเป๊ะทุกโหมด
--
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
       'คุณคือ Director ที่เล่าเรื่องผ่านหน้าจอแชท แตกสคริปเป็นซีนสำหรับ explainer แนวตั้ง 9:16 ภาษาไทย ' ||
       'โหมด "แชทลูกค้า". เป้าหมายสูงสุด: คนดูต้องรู้สึกว่ากำลังแอบส่องแชทจริงของคนที่เจอปัญหาเดียวกับตัวเอง ' ||
       'ทุกฟองข้อความต้องสั้นแบบที่คนพิมพ์จริง ห้ามใส่ emoji เด็ดขาด ตอบเป็น JSON เท่านั้น.',
       model, temperature, TRUE,
       E'แตกสคริปนี้ออกเป็น 5-8 ซีน สำหรับวิดีโอแนวตั้ง 9:16 ยาว {{.TargetDurationSec}} วินาที — โหมด "แชทลูกค้า"\n\n' ||
       E'สคริป:\n{{.Script}}\n\nธีมแบรนด์: {{.ThemeDescription}}\n\n' ||
       E'โครงบังคับ:\n' ||
       E'- ซีนแรก layout "hook" เสมอ: rows 1-2 บรรทัดบอกความเสียหาย (ไม่เกิน 36 ตัวอักษรต่อบรรทัด) — เป็นซีนเดียวที่มี image_prompt ได้\n' ||
       E'- ซีนที่สอง layout "chat_in" เสมอ: ลูกค้าเปิดเรื่องด้วยคำถามจริง\n' ||
       E'- สลับ "chat_out" (เราตอบ) กับ "chat_in" (ลูกค้าถามต่อ) ให้เป็นบทสนทนาที่เดินหน้า ไม่วน\n' ||
       E'- ซีนรองสุดท้าย layout "recap": สรุปสิ่งที่ต้องจำเป็นชิป 2-3 ตัว\n' ||
       E'- ซีนสุดท้าย layout "cta"\n' ||
       E'- อย่าใช้ layout เดียวกันเกิน 2 ซีนติดกัน\n\n' ||
       E'กฎภาพ (สำคัญมาก): image_prompt ใส่ได้เฉพาะซีนแรก (layout "hook") เท่านั้น หนึ่งใบต่อคลิป — ' ||
       E'บรรยายภาษาอังกฤษเป็นภาพบุคคลจริงกำลังใช้มือถือตอนกลางคืน สื่ออารมณ์กังวล ' ||
       E'ห้ามบรรยายหน้าจอ ห้ามมีตัวอักษรในภาพ. ทุกซีนที่เหลือ image_prompt = ""\n\n' ||
       E'ตอบเป็น JSON array เท่านั้น แต่ละ object มี:\n' ||
       E'- "scene_number": ลำดับซีน (เริ่ม 1 ต่อเนื่อง)\n' ||
       E'- "voice_text": ประโยคพากย์ไทยของซีนนี้ (สั้น พูดลื่น เหมือนเล่าให้เพื่อนฟัง)\n' ||
       E'- "on_screen_text": ข้อความบนจอสั้นๆ (ซีนแรกไม่เกิน 7 คำ)\n' ||
       E'- "emphasis_words": array 1-2 คำที่ต้องเน้น (ห้ามว่าง)\n' ||
       E'- "caption_style": "word_pop" (ซีนเปิด) หรือ "phrase_block" (ซีนเนื้อหา)\n' ||
       E'- "image_prompt": ตามกฎภาพข้างบน\n' ||
       E'- "layout": หนึ่งใน "hook" | "chat_in" | "chat_out" | "recap" | "cta"\n' ||
       E'- "content": object ตาม layout (ด้านล่าง)\n\n' ||
       E'content แยกตาม layout:\n' ||
       E'- hook: {"rows":[{"t":"บัญชีโดนแบนตอนตีสอง"},{"t":"งบยังวิ่งอยู่ 40,000"}]}\n' ||
       E'- chat_in: {"asker":"คุณเก่ง","stamp":"21:47 น.","msgs":[{"from":"them","t":"พี่ครับ บัญชีโดนแบน"},{"from":"them","t":"เพิ่งผูกบัตรใหม่เมื่อวาน","alert":true}]}\n' ||
       E'- chat_out: {"msgs":[{"from":"me","t":"อย่าเพิ่งยื่นอุทธรณ์ครับ"},{"from":"them","t":"แล้วทำไงดี"}],"verdict":"รอ 72 ชั่วโมงแล้วยื่นครั้งเดียว"}\n' ||
       E'- recap: {"title":"จำ 2 ข้อนี้","chips":[{"n":"72","t":"ชั่วโมงที่ต้องรอ"},{"n":"1","t":"ครั้งเท่านั้น"}]}\n' ||
       E'- cta: {"title":"ปิดท้ายสั้นๆ","cta":"ทักมาเช็ค","brand":"ADS VANCE","sub":"คำโปรยสั้น"}\n\n' ||
       E'กฎฟองข้อความ: "from" มีแค่ "them" (ลูกค้า) กับ "me" (เรา) · 1 ฟอง = 1 ประโยคสั้น ไม่เกิน 90 ตัวอักษร · ' ||
       E'ซีนหนึ่งมีได้ 2-4 ฟอง · "alert":true ใส่ได้ไม่เกิน 1 ฟองต่อซีน และใช้เฉพาะรายละเอียดที่เป็นสัญญาณอันตรายจริง ' ||
       E'ไม่ใช่ทุกประโยคที่เป็นข่าวร้าย · ใส่ asker กับ stamp เฉพาะซีน chat_in ซีนแรกของคลิป\n\n' ||
       E'กฎ chips ของ recap: ทุกชิปต้องมีทั้ง "n" และ "t" ห้ามขาดตัวใดตัวหนึ่ง · n สั้นไม่เกิน 6 ตัวอักษร\n\n' ||
       E'กฎเหล็ก: ห้าม emoji หรือสัญลักษณ์ภาพในทุก field. ความยาว: title ไม่เกิน 40, verdict ไม่เกิน 60, ' ||
       E'cta ไม่เกิน 14, sub ไม่เกิน 50, rows แต่ละแถวไม่เกิน 36, asker ไม่เกิน 16, stamp ไม่เกิน 12'
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
-- กติกาการสอน (ห้ามแต่งชื่อเมนู + จำนวน uistep ต้องเท่าจำนวนขั้นในคลัง) ยกมาทั้งดุ้น
-- จาก scene_tutorial เพราะตะแกรงฝั่ง Go ตรวจสองอย่างนี้ตรงๆ ถ้า prompt หลวมเมื่อไร
-- คลิปจะถูกตะแกรงตีตกทุกคืนแล้ว slot 21:00 เงียบไปเลย
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
       'คุณคือ Director ที่เล่าเรื่องผ่านจอมอนิเตอร์ห้องควบคุม แตกสคริปเป็นซีนสำหรับ explainer แนวตั้ง 9:16 ' ||
       'ภาษาไทย โหมด "ห้องควบคุม". ผู้ชมคือคนยิงแอดอยู่แล้ว ใช้ศัพท์เทคนิคตรงๆ ได้ ' ||
       'เป้าหมายสูงสุด: 3 วินาทีแรกต้องเป็นตัวเลขที่ทำให้คนดูรีบเช็คบัญชีตัวเอง ' ||
       'ห้ามใส่ emoji เด็ดขาด ตอบเป็น JSON เท่านั้น.',
       model, temperature, TRUE,
       E'แตกสคริปนี้ออกเป็นซีนสำหรับวิดีโอแนวตั้ง 9:16 ยาว {{.TargetDurationSec}} วินาที — โหมด "ห้องควบคุม"\n\n' ||
       E'สคริป:\n{{.Script}}\n\n{{.TutorialBrief}}\n\nธีมแบรนด์: {{.ThemeDescription}}\n\n' ||
       E'โครงบังคับ:\n' ||
       E'- ซีนแรก layout "dashboard" เสมอ: ตัวเลขที่ผิดปกติบนจอ — เป็นซีนเดียวที่มี image_prompt ได้\n' ||
       E'- ตรงกลาง layout "uistep" หนึ่งซีนต่อหนึ่งขั้น จำนวนต้องเท่ากับจำนวนขั้นในข้อมูลฟีเจอร์พอดี เรียงตามลำดับ n\n' ||
       E'- ก่อนซีน uistep ตัวสุดท้าย แทรก layout "alarm" หนึ่งซีน = กับดักที่คนพลาด\n' ||
       E'- ซีนรองสุดท้าย layout "alarm": สรุปสิ่งที่ต้องทำก่อนนอน ให้คนแคปเก็บ\n' ||
       E'- ซีนสุดท้าย layout "cta"\n\n' ||
       E'กฎภาพ (สำคัญมาก): image_prompt ใส่ได้เฉพาะซีนแรก (layout "dashboard") เท่านั้น หนึ่งใบต่อคลิป — ' ||
       E'บรรยายภาษาอังกฤษเป็นภาพห้องทำงานกลางคืนที่มีจอหลายจอเรืองแสง ' ||
       E'ห้ามบรรยายเนื้อหาบนจอ ห้ามมีตัวอักษรในภาพ. ทุกซีนที่เหลือ image_prompt = ""\n\n' ||
       E'กฎคำศัพท์ UI (เข้มที่สุด): ค่าใน panel.chrome, panel.breadcrumb, panel.items[].label และ panel.field.label ' ||
       E'ต้องเป็นคำที่ปรากฏในรายการคำศัพท์ UI ข้างบนแบบตรงตัวเท่านั้น ห้ามแปล ห้ามย่อ ห้ามแต่งใหม่ ' ||
       E'ระบบจะตีคลิปตกทันทีถ้าพบคำนอกรายการ\n\n' ||
       E'ตอบเป็น JSON array เท่านั้น แต่ละ object มี:\n' ||
       E'- "scene_number": ลำดับซีน (เริ่ม 1 ต่อเนื่อง)\n' ||
       E'- "voice_text": ประโยคพากย์ไทยของซีนนี้ (สั้น พูดลื่น เหมือนรุ่นพี่ที่ดูตัวเลขมาทั้งคืน)\n' ||
       E'- "on_screen_text": ข้อความบนจอสั้นๆ (ซีนแรกไม่เกิน 7 คำ)\n' ||
       E'- "emphasis_words": array 1-2 คำที่ต้องเน้น (ห้ามว่าง)\n' ||
       E'- "caption_style": "word_pop" (ซีนเปิด) หรือ "phrase_block" (ซีนเนื้อหา)\n' ||
       E'- "image_prompt": ตามกฎภาพข้างบน\n' ||
       E'- "layout": หนึ่งใน "dashboard" | "uistep" | "alarm" | "cta"\n' ||
       E'- "content": object ตาม layout (ด้านล่าง)\n\n' ||
       E'content แยกตาม layout:\n' ||
       E'- dashboard: {"statLabel":"CPM 7 วันล่าสุด","chips":[{"n":"+38%","t":"CPM","bad":true},{"n":"1.4x","t":"ROAS"}],"callout":"ค่าโฆษณาแพงขึ้น 38% ใน 3 วัน"}\n' ||
       E'- uistep: {"num":"2","of":"ขั้นที่ 2 / 3","title":"ชื่อขั้นภาษาไทย","panel":{"chrome":"Ads Manager","breadcrumb":"Rules > Create new rule","items":[{"label":"Campaigns","state":"normal"},{"label":"Automated Rules","state":"target"}],"field":{"label":"Greater than","value":"400 THB"}},"callout":"เลือก Automated Rules"}\n' ||
       E'- alarm: {"title":"ตั้งเพดานไว้ก่อนนอน","rows":[{"t":"CPM ทะลุเพดาน = หยุดแอดอัตโนมัติ"},{"t":"ไม่ตั้ง = จ่ายทั้งคืน","bad":true}]}\n' ||
       E'- cta: {"title":"ปิดท้ายสั้นๆ","cta":"ทักมาเช็ค","brand":"ADS VANCE","sub":"คำโปรยสั้น"}\n\n' ||
       E'กฎ dashboard: chips คือตัวเลขบนจอ 2-3 ตัว ทุกชิปต้องมีทั้ง "n" และ "t" · ' ||
       E'"bad":true ใส่กับตัวเลขที่วิ่งผิดทางเท่านั้น (ตัวเลขจะเป็นสีแดง) · n ไม่เกิน 6 ตัวอักษร · ' ||
       E'statLabel ไม่เกิน 28 · callout ไม่เกิน 60\n\n' ||
       E'กฎ panel.field: ใส่ field เฉพาะเมื่อมีค่าที่คนดูต้องกรอกหรือต้องอ่านจริง — ' ||
       E'ถ้าขั้นนั้นเป็นแค่การกดเมนู ห้ามใส่ field เลย\n\n' ||
       E'กฎ uistep: state มีแค่ "normal" | "target" | "done" — ขั้นที่ผ่านมาแล้วเป็น done ขั้นปัจจุบันเป็น target ' ||
       E'หนึ่งซีนมี target ได้แค่หนึ่งอัน. items ไม่เกิน 5 แถว. num เป็นตัวเลขขั้นปัจจุบันเท่านั้น\n\n' ||
       E'กฎเหล็ก: ห้าม emoji หรือสัญลักษณ์ภาพในทุก field. ความยาว: title ไม่เกิน 34, callout ไม่เกิน 60, ' ||
       E'cta ไม่เกิน 14, sub ไม่เกิน 50, rows แต่ละแถวไม่เกิน 36'
FROM agent_configs WHERE agent_name = 'scene_tutorial'
ON CONFLICT (agent_name) DO NOTHING;

INSERT INTO agent_configs (agent_name, system_prompt, model, temperature, enabled, prompt_template)
SELECT 'critic_warroom', system_prompt, model, temperature, TRUE, prompt_template
FROM agent_configs WHERE agent_name = 'critic_tutorial'
ON CONFLICT (agent_name) DO NOTHING;

-- ── บอก visual_qa ให้รู้จักดีไซน์ใหม่ กัน false positive แบบ PR#14/#17 ──
UPDATE agent_configs
SET system_prompt = system_prompt || E'\n\nดีไซน์ที่ถูกต้องของโหมดแชท (data-format="chat"): ' ||
    'ฟองข้อความชิดซ้าย=ลูกค้า ฟองสีส้มชิดขวา=เรา ฟองสีแดงจางคือข้อความที่เป็นสัญญาณอันตราย ' ||
    'การ์ดสีเขียวใต้บทสนทนาคือข้อสรุป — ทั้งหมดนี้คือดีไซน์ที่ตั้งใจ ไม่ใช่ข้อผิดพลาด ' ||
    E'\n\nดีไซน์ที่ถูกต้องของโหมดห้องควบคุม (data-format="warroom"): ' ||
    'ตารางเส้นสีฟ้าจางทับพื้นหลังคือลายจอมอนิเตอร์ กราฟรูปทรงเหลี่ยมคือ sparkline ' ||
    'ตัวเลข KPI สีแดงคือค่าที่วิ่งผิดทาง แถบสีแดงด้านซ้ายของกล่องคือแถบเตือน ' ||
    'และแผงจำลอง Ads Manager มีขอบหนาเรืองแสงฟ้าเพราะจำลองเป็นจอมอนิเตอร์ — ทั้งหมดคือดีไซน์ที่ตั้งใจ'
WHERE agent_name = 'visual_qa'
  AND system_prompt NOT LIKE '%data-format="warroom"%';

-- ── schedule 18:00 ไป action ของตัวเอง ──
-- scheduler โหลดตารางจาก DB ตอน start อยู่แล้ว การ deploy จึงทำให้แถวนี้มีผลทันที
UPDATE schedules SET action = 'produce_evening'
WHERE cron_expression = '0 18 * * *' AND action = 'produce_and_publish';

COMMIT;
