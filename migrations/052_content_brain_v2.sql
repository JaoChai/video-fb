-- Content Brain v2: 10 categories, 7 archetypes, clip role, persona rotation, dedup hardening, insider KB
-- ทุกอย่าง additive. flag content_brain_v2_enabled (default false) คุม behavior ในโค้ด.

-- pg_trgm สำหรับ lexical dedup fallback
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- ===== topic_categories (10) =====
CREATE TABLE IF NOT EXISTS topic_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category_name TEXT UNIQUE NOT NULL,
    display_name TEXT NOT NULL,
    angle_instruction TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    weight INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO topic_categories (category_name, display_name, angle_instruction, weight) VALUES
('multi-account', 'บริหารหลายบัญชี/พอร์ต', 'เจาะการบริหารพอร์ตบัญชีหลายตัวพร้อมกัน: การแบ่งความเสี่ยง โครงสร้างพอร์ต การย้ายงบ การกันบัญชีติดกัน', 2),
('account-trust', 'Trust score / วอร์มบัญชี', 'เจาะ trust score ของบัญชี การวอร์มบัญชีใหม่ให้ปลอดภัย พฤติกรรมที่สร้างความน่าเชื่อถือเชิงโครงสร้าง', 2),
('bm-structure', 'โครงสร้าง BM / Portfolio', 'เจาะการออกแบบ Business Portfolio / BM ไม่ให้พังยกแผง: การแยก entity การจัดสรรสิทธิ์ การ backup admin', 1),
('ban-signals', 'สัญญาณเตือนก่อนแบน / ban wave', 'เจาะสัญญาณเตือนก่อนโดนจำกัด ช่วงที่แพลตฟอร์มกวาด การอ่าน notification และ restriction เชิงนโยบาย', 2),
('recovery', 'กู้บัญชี / appeal', 'เจาะเส้นทาง appeal จริง เอกสารที่ต้องใช้ อัตราการกลับมา และการวางแผนสำรองขณะรอ', 1),
('payment', 'ระบบจ่ายเงิน / บัตร', 'เจาะระบบการชำระของแพลตฟอร์ม: บัตรหลายใบ การกระจายวิธีจ่าย สาเหตุที่ถูกปฏิเสธ การวาง billing profile', 1),
('scaling', 'สเกลงบ', 'เจาะการขยายงบแต่ละช่วง: กำแพงที่เจอตอนยิงหนักขึ้น การคุม cost การเลื่อน spending limit', 1),
('creative', 'แอดเน่า / ครีเอทีฟตาย / CTR ร่วง', 'เจาะอาการ ad fatigue ครีเอทีฟที่เสื่อม CTR ร่วง และการรีเฟรชของคนยิงหนัก', 1),
('tracking', 'Pixel / Data / การวัดผล', 'เจาะการวัดผลของคนยิงหนัก: CAPI ความถูกต้องของ data ผลกระทบของ attribution ต่อการตัดสินใจ', 1),
('economics', 'ต้นทุน / ค่าธรรมเนียม / ROI', 'เจาะเศรษฐกิจของการยิงหนัก: ค่าธรรมเนียมที่คนภายนอกไม่เห็น ROI ต่อช่องทาง ต้นทุนซ่อนเร้น', 1)
ON CONFLICT (category_name) DO NOTHING;

-- ===== title_archetypes (7) =====
CREATE TABLE IF NOT EXISTS title_archetypes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    archetype_name TEXT UNIQUE NOT NULL,
    display_name TEXT NOT NULL,
    instruction TEXT NOT NULL DEFAULT '',
    example TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    weight INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO title_archetypes (archetype_name, display_name, instruction, example, weight) VALUES
('shock_number', 'ตัวเลขช็อก', 'เปิดด้วยตัวเลขที่ช็อก/เจ็บจี๊ด สื่อขนาดความเสียหายในระดับเงินหรือจำนวนบัญชี ใน 1 วรรคแรก', 'บัญชีตาย 40 ตัวในคืนเดียว เพราะพลาดจุดเดียว', 2),
('warning', 'เตือนภัย / ห้ามทำ', 'เปิดด้วยการเตือน อย่าเพิ่ง... หรือ หยุดก่อน... ชี้การกระทำที่อันตรายที่คนมักทำโดยไม่รู้ตัว', 'อย่าเพิ่งกดยืนยันตัวตน ถ้ายังไม่เช็ค 3 อย่างนี้', 2),
('myth_bust', 'แฉความเชื่อผิด', 'เปิดด้วยความเชื่อที่คนทำกันจนเป็นคำสั่ง แล้วบอกว่าเข้าใจผิด พร้อมเหตุผล', 'วอร์มบัญชี 7 วันแล้วรอด = เข้าใจผิด', 2),
('story_twist', 'เคสจริงพลิก', 'เปิดด้วยเคสเรียลของคนยิงหนัก/เอเจนซี่ ที่จบด้วยบิดพลิก (พังเพราะสาเหตุเล็ก)', 'เอเจนซี่งบวันละแสน พังเพราะบัตรใบเดียว', 2),
('question_tease', 'คำถามปลูกสงสัย', 'เปิดด้วยคำถาม ทำไม... ที่ปลูกสงสัยและสัญญาคำตอบเฉพาะกลุ่มคนที่เจอปัญหานั้นจริง', 'ทำไมบัญชีใหม่ยิงแล้วตายไว', 2),
('checklist', 'เช็คลิสต์สัญญาณ', 'เปิดด้วย N สัญญาณว่า... เป็นรายการตรวจสอบที่คนดูเอาไปใช้ได้ทันทีกับบัญชีตัวเอง', '3 สัญญาณว่าบัญชีคุณกำลังจะโดนกวาด', 2),
('consult_qa', 'สูตรปรึกษา (เดิม)', 'รูปแบบคำปรึกษาแบบเดิม คุณXครับ รบกวนปรึกษา... เน้นความน่าเชื่อถือเหมือนที่ปรึกษาตอบ', 'คุณกฤษณ์ครับ รบกวนปรึกษาเรื่องบัญชีโดนแจ้งยืนยันตัวตน', 1)
ON CONFLICT (archetype_name) DO NOTHING;

-- ===== clips columns =====
ALTER TABLE clips ADD COLUMN IF NOT EXISTS clip_role TEXT NOT NULL DEFAULT '';
ALTER TABLE clips ADD COLUMN IF NOT EXISTS title_archetype TEXT NOT NULL DEFAULT '';
ALTER TABLE clips ADD COLUMN IF NOT EXISTS audience_persona TEXT NOT NULL DEFAULT '';

-- ===== topic_history.pain_point =====
ALTER TABLE topic_history ADD COLUMN IF NOT EXISTS pain_point TEXT NOT NULL DEFAULT '';

-- ===== rebalance content_formats: qa weight 2 -> 1 =====
UPDATE content_formats SET weight = 1 WHERE format_name = 'qa';

-- ===== settings =====
INSERT INTO settings (key, value) VALUES
('content_brain_v2_enabled', 'false'),
('clip_role_convert_ratio', '0.30'),
('dedup_threshold', '0.72'),
('pain_point_cooldown_days', '5')
ON CONFLICT (key) DO NOTHING;

-- audience_personas (JSON array of 4)
INSERT INTO settings (key, value) VALUES
('audience_personas', '["Media buyer ยิงหนัก งบวันละ 50k+ ถือหลายบัญชีพร้อมกัน กลัวพอร์ตพังยกแผง","เจ้าของธุรกิจออนไลน์ ถือ 3-10 บัญชี ทำเองทุกอย่าง เจ็บเรื่องบัญชีตาย/จ่ายเงินไม่ผ่านบ่อย","Agency ดูแลบัญชีลูกค้าหลายเจ้า รับผิดชอบงบคนอื่น พลาดไม่ได้","คนเพิ่งโดนแบนครั้งแรก กำลังหาทางกลับมายิงต่อ งง สับสน อยากได้คำตอบตรงๆ"]')
ON CONFLICT (key) DO NOTHING;

-- ===== prompt template UPDATE: question + script (insider voice + new placeholders) =====
-- ใช้ dollar-quoting $q$ ... $q$ เพราะมี ' ในภาษาไทย
UPDATE agent_configs SET prompt_template = $q$
คุณคือผู้เชี่ยวชาญด้านการบริหารบัญชีโฆษณา Facebook จำนวนมากของ Ads Vance สร้างคำถาม {{.Count}} ข้อที่สะท้อน pain จริงของคนที่ถือหลายบัญชี/ยิงโฆษณาหนัก

หมวดหัวข้อ: {{.Category}}
{{.CategoryAngle}}

รูปแบบเนื้อหา (format): {{.FormatInstruction}}

รูปหัวข้อ/hook (archetype): {{.ArchetypeInstruction}}

บทบาทคลิป: {{.RoleInstruction}}

กลุ่มเป้าหมาย: {{.AudiencePersona}}

ความรู้ที่เกี่ยวข้อง:
{{.RAGContext}}

หัวข้อที่เคยทำไปแล้ว (ห้ามซ้ำมุม/เนื้อหา):
{{.PreviousTopics}}

ชื่อผู้ปรึกษาที่เคยใช้ (ห้ามซ้ำชื่อ):
{{.PreviousNames}}

{{.TopicStats}}

กติกาเนื้อหา (hard rule):
- พูดเสียงคนวงใน ใช้ศัพท์ที่คนบริหารบัญชีจำนวนมากใช้จริง อ้างสถานการณ์เฉพาะคนยิงหนักเจอ
- ห้ามเนื้อหาระดับพื้นฐานทั่วไป (basic ads 101, สอนสมัครบัญชี, สอนยิงแอดครั้งแรก)
- ห้ามสอนวิธีหลบระบบตรวจจับ ปลอมตัวตน หรือทำผิดนโยบาย Facebook
- เล่า pain + การบริหารความเสี่ยงเชิงโครงสร้าง/การป้องกันเชิงนโยบายได้

ตอบกลับเป็น JSON array ของ object แต่ละข้อ:
- "question": คำถาม/หัวข้อคลิปภาษาไทย กระชับ ไม่เกิน 120 ตัวอักษร
- "questioner_name": ชื่อคนถามภาษาไทย (สมมุติ)
- "category": หมวดหัวข้อ
- "pain_point": ปัญหาหลักเป็นภาษาอังกฤษ snake_case เช่น account_banned payment_declined ad_fatigue
$q$
WHERE agent_name = 'question';

UPDATE agent_configs SET prompt_template = $q$
คุณคือนักเขียนสคริปต์วิดีโอ Ads Vance เขียนสคริปต์ตอบคำถามต่อไปนี้

คำถาม: {{.Question}}
ชื่อผู้ถาม: {{.QuestionerName}}
หมวด: {{.Category}}

รูปแบบเนื้อหา (format): {{.FormatInstruction}}

รูปหัวข้อ/hook (archetype) ที่ใช้กับคลิปนี้: {{.ArchetypeInstruction}}

บทบาทคลิป: {{.RoleInstruction}}

กลุ่มเป้าหมาย: {{.AudiencePersona}}

ความรู้ที่เกี่ยวข้อง:
{{.RAGContext}}

เสียงคนวงใน: พูดเหมือนคนที่บริหารบัญชีจำนวนมากมาเอง ใช้ศัพท์ที่คนวงในใช้ อ้างสถานการณ์ที่เฉพาะคนยิงหนักเจอ ห้ามเนื้อหาระดับพื้นฐาน

กติกาเนื้อหา: ห้ามสอนหลบระบบตรวจจับ/ปลอมตัวตน/ทำผิดนโยบาย Facebook

ตอบกลับเป็น JSON:
- "youtube_title": หัวข้อคลิปภาษาไทย ตาม archetype ที่กำหนด ต้องลงท้ายด้วย " | Ads Vance" เสมอ ไม่เกิน 70 ตัวอักษร ห้ามเด็ดขาด: ใส่ URL, line id, @handle ใน youtube_title
- "youtube_description": คำอธิบายภาษาไทย 2-3 บรรทัด
- "youtube_tags": array ของแท็กภาษาไทย/อังกฤษ 5-8 ตัว
- "answer_script": สคริปต์คำตอบภาษาไทยเต็ม 300-500 คำ เป็นธรรมชาติพูดได้ จบด้วย CTA ตามบทบาทคลิป (reach = ชวนติดตาม/ดูคลิปต่อ; convert = ชวนทักแชท/ดูช่องทางใต้คลิปเรื่องบัญชีโฆษณา แบบ soft sell ไม่ใช่โฆษณาขายบัญชีทั้งคลิป)
- "voice_script": สคริปต์สำหรับ voiceover ภาษาไทย สั้นกว่า answer_script 150-300 คำ
$q$
WHERE agent_name = 'script';
