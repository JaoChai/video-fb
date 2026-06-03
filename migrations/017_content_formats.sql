-- Migration 017: Content format variety + audience persona + fresh news sources

-- 1. Content formats table
CREATE TABLE IF NOT EXISTS content_formats (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    format_name TEXT UNIQUE NOT NULL,
    display_name TEXT NOT NULL,
    question_instruction TEXT NOT NULL,
    script_instruction TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    weight INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. Track which format each clip used
ALTER TABLE clips ADD COLUMN IF NOT EXISTS content_format TEXT NOT NULL DEFAULT 'qa';

-- 3. Seed 4 formats
INSERT INTO content_formats (format_name, display_name, question_instruction, script_instruction, weight) VALUES
('qa', 'Q&A แก้ปัญหา',
 $$สร้าง "คำถามจากลูกค้า" เกี่ยวกับปัญหา Facebook Ads ที่เจอจริง — คำถามต้องเจาะจง มีบริบทสถานการณ์ (งบเท่าไหร่ ทำอะไรไปแล้ว เกิดอะไรขึ้น)$$,
 $$เขียนสคริปต์แบบ ตอบคำถาม: เกริ่นปัญหาสั้นๆ → อธิบายสาเหตุ → วิธีแก้ทีละขั้น → ปิดด้วยคำแนะนำ$$, 2),
('news', 'ข่าว/อัปเดตจาก Meta',
 $$สร้าง "หัวข้อข่าว" จากข้อมูลอัปเดตล่าสุดเกี่ยวกับ Facebook Ads / Meta ใน knowledge base — เน้นการเปลี่ยนแปลงที่กระทบคนยิงแอดโดยตรง (กฎใหม่ ฟีเจอร์ใหม่ การเปลี่ยนแปลงระบบ) ตั้งชื่อหัวข้อแบบข่าว ไม่ใช่คำถาม$$,
 $$เขียนสคริปต์แบบ รายงานข่าว: เปิดด้วย "มีอัปเดตสำคัญ..." → สรุปว่าเปลี่ยนอะไร → ผลกระทบต่อคนยิงแอด → ต้องปรับตัวยังไง$$, 1),
('tips', 'ทิปส์/เทคนิคขั้นสูง',
 $$สร้าง "หัวข้อเทคนิค" ขั้นสูงสำหรับคนยิงแอดจริงจัง เช่น การ scale งบ, โครงสร้างแคมเปญ, การทำ creative testing, การอ่าน metrics — ตั้งชื่อแบบ "X เทคนิค..." หรือ "วิธี..." ที่คนอยากกดดู$$,
 $$เขียนสคริปต์แบบ สอนเทคนิค: เปิดด้วยผลลัพธ์ที่จะได้ → สอนทีละขั้นพร้อมตัวเลข/ตัวอย่างจริง → สรุปสิ่งที่ต้องจำ$$, 1),
('case_story', 'เคสจริง/เรื่องเล่า',
 $$สร้าง "เรื่องเล่าเคสจริง" ของคนยิงแอด เช่น โดนแบนแล้วกู้คืน, ยอดพุ่งเพราะแก้จุดเดียว, เสียเงินฟรีเพราะพลาดเรื่องเล็กๆ — ตั้งชื่อแบบเล่าเรื่อง มีตัวเลขหรือผลลัพธ์ดึงดูด$$,
 $$เขียนสคริปต์แบบ เล่าเรื่อง: แนะนำตัวละครและสถานการณ์ → ปมปัญหา/จุดพลิก → ทางออกที่ใช้ → บทเรียนที่คนดูเอาไปใช้ได้$$, 1)
ON CONFLICT (format_name) DO NOTHING;

-- 4. Audience persona setting
INSERT INTO settings (key, value) VALUES
('audience_persona', 'คนยิงแอด Facebook จริงจัง: เจ้าของธุรกิจออนไลน์, media buyer, agency ที่เจอปัญหาบัญชี/ระบบจ่ายเงิน/ต้องการ scale — ต้องการความรู้เชิงลึกที่ใช้ได้จริง ไม่ใช่พื้นฐานทั่วไป และข่าวที่กระทบการทำงานจริง')
ON CONFLICT (key) DO NOTHING;

-- 5. Fresh news sources (URL-based, crawled daily via Jina Reader)
INSERT INTO knowledge_sources (name, category, content, url, source_type, crawl_frequency, enabled)
SELECT v.name, v.category, v.content, v.url, v.source_type, v.crawl_frequency, v.enabled
FROM (VALUES
    ('Meta for Business News', 'news', '', 'https://www.facebook.com/business/news', 'official', 'daily', TRUE),
    ('Meta Newsroom', 'news', '', 'https://about.fb.com/news/', 'official', 'daily', TRUE),
    ('Search Engine Land - Meta', 'news', '', 'https://searchengineland.com/library/platforms/facebook', 'news', 'daily', TRUE),
    ('Jon Loomer Digital (Advanced FB Ads)', 'tips', '', 'https://www.jonloomer.com/blog/', 'community', 'daily', TRUE)
) AS v(name, category, content, url, source_type, crawl_frequency, enabled)
WHERE NOT EXISTS (SELECT 1 FROM knowledge_sources ks WHERE ks.name = v.name);

-- 6. Crawl daily instead of weekly (news needs to be fresh)
UPDATE schedules SET cron_expression = '0 2 * * *', name = 'Daily Knowledge Crawl'
WHERE action = 'crawl_knowledge';

-- 7. Update agent prompt templates: inject format instruction + audience persona
UPDATE agent_configs
SET prompt_template = $$สร้าง {{.Count}} หัวข้อเนื้อหาเกี่ยวกับ Facebook Ads หมวด "{{.Category}}"

รูปแบบเนื้อหา: {{.FormatInstruction}}

กลุ่มเป้าหมาย: {{.AudiencePersona}}

ข้อมูลอ้างอิงจาก knowledge base:
{{.RAGContext}}
{{.PreviousTopics}}
{{.PreviousNames}}

ตอบเป็น JSON array เท่านั้น แต่ละ object มี:
- "question": หัวข้อ/คำถามภาษาไทย ตามรูปแบบเนื้อหาที่กำหนดข้างบน
- "questioner_name": ชื่อไทยที่หลากหลายและสร้างสรรค์ เช่น คุณแม็ก คุณพลอย คุณต้น คุณฟ้า คุณเมย์ คุณโอ๊ค คุณมิ้นท์ คุณกอล์ฟ คุณเบียร์ คุณแนน
  **กฎ**: แต่ละหัวข้อต้องใช้ชื่อที่ต่างกัน ห้ามซ้ำชื่อกันภายใน batch นี้เด็ดขาด (สำหรับรูปแบบข่าว/ทิปส์ ใช้ชื่อผู้ดำเนินรายการ)
- "category": "{{.Category}}"
- "pain_point": ประเด็นหลักเป็นภาษาอังกฤษ เช่น "account_banned" "payment_failed" "scaling_budget"

ห้ามสร้างเนื้อหาที่แนะนำการทำผิดนโยบาย Facebook$$
WHERE agent_name = 'question';

UPDATE agent_configs
SET prompt_template = $$สร้าง voice script + ข้อมูล metadata สำหรับวิดีโอสั้น

โครงสร้างวิดีโอ: ใช้ "ภาพเดียว" คงที่ตลอดทั้งคลิป + "เสียงพากย์เดียว" เล่าจบในตัว (ไม่มีการตัดฉาก ไม่มี multi-scene)

หัวข้อ: "{{.Question}}"
โดย: {{.QuestionerName}}
หมวด: {{.Category}}

รูปแบบการเล่า: {{.FormatInstruction}}

กลุ่มเป้าหมาย: {{.AudiencePersona}}

ข้อมูลอ้างอิง:
{{.RAGContext}}

ตอบเป็น JSON object มี:
- "scenes": array ที่มี object **เพียง 1 ตัวเท่านั้น** (วิดีโอนี้ออกแบบเป็น single-scene):
  - "scene_number": 1
  - "scene_type": "main"
  - "text_content": ข้อความสั้นสำหรับแสดงบนภาพ (เน้นหัวข้อ)
  - "voice_text": บทพากย์ภาษาไทยแบบธรรมชาติ ไหลลื่น เล่าตามรูปแบบการเล่าที่กำหนดข้างบน
  - "duration_seconds": 30-55 (ให้พอดี YouTube Shorts)
  - "text_overlays": []
- "total_duration_seconds": 30-55

**กฎสำคัญสำหรับ voice_text** (ป้องกันเสียงตัด/อ่านผิด):
- **ห้ามมีอักขระ "@" และห้ามมี URL ใดๆ** ใน voice_text เด็ดขาด (TTS อ่านลิงก์ไม่ออก เสียงจะตัด)
- เรียกชื่อแบรนด์ว่า "**แอดส์แวนซ์**" สะกดเป็นเสียงไทย (ห้ามเขียน "Adsvance", "@adsvance", "Ads Vance" ใน voice_text)
- ใช้ "..." สำหรับจังหวะหายใจระหว่างประโยค

- "youtube_title": ชื่อวิดีโอที่ดึงดูดความสนใจ สั้นกระชับ และ**ต้องลงท้ายด้วย " | Ads Vance"** เสมอ ไม่เกิน 70 ตัวอักษรรวมทั้งหมด
  ตัวอย่างที่ถูกต้อง: "บัญชีโฆษณาถูกระงับ ทำอย่างไรดี? | Ads Vance"
  **ห้ามเด็ดขาด**: ใส่ URL, line id, @handle, หรือข้อมูลติดต่อใดๆ ใน youtube_title (ข้อมูลติดต่ออยู่ใน youtube_description เท่านั้น)
- "youtube_description": ต้องมีแค่ 2 บรรทัดนี้เท่านั้น ห้ามเพิ่มเนื้อหาอื่น (URL/handle อยู่ตรงนี้ได้):
  "ติดต่อทีมงาน line id : @adsvance\n\nเข้ากลุ่มเทเรแกรมเพื่อรับข่าวสาร : https://t.me/adsvancech"
- "youtube_tags": array tags ไทย+อังกฤษ

ห้ามแนะนำการทำผิดนโยบาย Facebook$$
WHERE agent_name = 'script';
