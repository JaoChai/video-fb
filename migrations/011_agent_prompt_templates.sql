-- Migration 011: Add prompt_template column to agent_configs and seed prompt templates + settings

-- 1. Add prompt_template column (idempotent)
ALTER TABLE agent_configs
    ADD COLUMN IF NOT EXISTS prompt_template TEXT NOT NULL DEFAULT '';

-- 2. UPDATE question agent prompt template
UPDATE agent_configs
SET prompt_template = $$สร้าง {{.Count}} คำถามจากลูกค้าเกี่ยวกับ Facebook Ads หมวด "{{.Category}}"

ข้อมูลอ้างอิงจาก knowledge base:
{{.RAGContext}}
{{.PreviousTopics}}

ตอบเป็น JSON array เท่านั้น แต่ละ object มี:
- "question": คำถามภาษาไทย สั้น กระชับ เหมือนลูกค้าถามจริง
- "questioner_name": ชื่อไทย เช่น "คุณ สมชาย" "คุณ มานี"
- "category": "{{.Category}}"
- "pain_point": ปัญหาหลักเป็นภาษาอังกฤษ เช่น "account_banned" "payment_failed"

ห้ามสร้างคำถามที่แนะนำการทำผิดนโยบาย Facebook$$
WHERE agent_name = 'question';

-- 3. UPDATE script agent prompt template
UPDATE agent_configs
SET prompt_template = $$สร้าง voice script + ข้อมูล metadata สำหรับวิดีโอ Q&A สั้น

โครงสร้างวิดีโอ: ใช้ "ภาพเดียว" คงที่ตลอดทั้งคลิป + "เสียงพากย์เดียว" เล่าจบในตัว (ไม่มีการตัดฉาก ไม่มี multi-scene)

คำถาม: "{{.Question}}"
ถามโดย: {{.QuestionerName}}
หมวด: {{.Category}}

ข้อมูลอ้างอิง:
{{.RAGContext}}

ตอบเป็น JSON object มี:
- "scenes": array ที่มี object **เพียง 1 ตัวเท่านั้น** (วิดีโอนี้ออกแบบเป็น single-scene):
  - "scene_number": 1
  - "scene_type": "main"
  - "text_content": ข้อความสั้นสำหรับแสดงบนภาพ (เน้นคำถาม)
  - "voice_text": บทพากย์ภาษาไทยแบบธรรมชาติ ไหลลื่นเป็นเรื่องเล่าเดียว ลำดับ: เกริ่นคำถาม → อธิบายคำตอบเป็นขั้นตอน → ปิดด้วย CTA
  - "duration_seconds": 30-55 (ให้พอดี YouTube Shorts)
  - "text_overlays": []
- "total_duration_seconds": 30-55

**กฎสำคัญสำหรับ voice_text** (ป้องกันเสียงตัด/อ่านผิด):
- **ห้ามมีอักขระ "@" และห้ามมี URL ใดๆ** ใน voice_text เด็ดขาด (TTS อ่านลิงก์ไม่ออก เสียงจะตัด)
- เรียกชื่อแบรนด์ว่า "**แอดส์แวนซ์**" สะกดเป็นเสียงไทย (ห้ามเขียน "Adsvance", "@adsvance", "Ads Vance" ใน voice_text)
- CTA ปิดท้ายให้พูดทำนองนี้: "ติดต่อทีมงานแอดส์แวนซ์ทางไลน์ ไอดีแอดส์แวนซ์ หรือเข้ากลุ่มเทเลแกรมแอดส์แวนซ์ได้เลยครับ"
- ใช้ "..." สำหรับจังหวะหายใจระหว่างประโยค

- "youtube_title": ชื่อวิดีโอที่ดึงดูดความสนใจ สั้นกระชับ และ**ต้องลงท้ายด้วย " | Ads Vance"** เสมอ ไม่เกิน 70 ตัวอักษรรวมทั้งหมด
  ตัวอย่างที่ถูกต้อง: "บัญชีโฆษณาถูกระงับ ทำอย่างไรดี? | Ads Vance"
  ตัวอย่างที่ถูกต้อง: "ทำไมโฆษณาไม่ผ่านการอนุมัติ | Ads Vance"
  **ห้ามเด็ดขาด**: ใส่ URL, line id, @handle, หรือข้อมูลติดต่อใดๆ ใน youtube_title (ข้อมูลติดต่ออยู่ใน youtube_description เท่านั้น)
- "youtube_description": ต้องมีแค่ 2 บรรทัดนี้เท่านั้น ห้ามเพิ่มเนื้อหาอื่น (URL/handle อยู่ตรงนี้ได้):
  "ติดต่อทีมงาน line id : @adsvance\n\nเข้ากลุ่มเทเรแกรมเพื่อรับข่าวสาร : https://t.me/adsvancech"
- "youtube_tags": array tags ไทย+อังกฤษ

ห้ามแนะนำการทำผิดนโยบาย Facebook$$
WHERE agent_name = 'script';

-- 4. UPDATE image agent prompt template
UPDATE agent_configs
SET prompt_template = $$สร้าง image prompt 1 ภาพ สำหรับวิดีโอ Facebook Ads Q&A

Brand Theme: {{.ThemeDescription}}
คนถาม: {{.QuestionerName}}
คำถาม: {{.QuestionText}}

สร้างภาพสไตล์ chat bubble / Facebook-like UI แสดงคำถามเด่นชัด พร้อม icon คำถาม
ภาพนี้จะใช้เป็นพื้นหลังตลอดทั้งคลิปในขณะที่เสียงพากย์อธิบายคำตอบ

ตอบเป็น JSON array ที่มี object เดียว:
- "scene_number": 1
- "image_prompt_16_9": prompt ภาษาอังกฤษ สำหรับ 16:9 landscape. ใส่ Thai text คำถามบนภาพ.
- "image_prompt_9_16": prompt เหมือนกันแต่สำหรับ 9:16 vertical format.

DO NOT include any logo, mascot, brand name, or brand text in the image.
ภาพต้องมี: dark gradient background ({{.PrimaryColor}} to darker), accent color {{.AccentColor}}, modern flat design, chat bubble with question text.$$
WHERE agent_name = 'image';

-- 5. INSERT categories and brand_aliases into settings (idempotent)
INSERT INTO settings (key, value)
VALUES
    ('categories', '["account","payment","campaign","pixel"]'),
    ('brand_aliases', '{"AdsVance":"แอดส์แวนซ์","Adsvance":"แอดส์แวนซ์","adsvance":"แอดส์แวนซ์","Ads Vance":"แอดส์แวนซ์","@adsvance":"แอดส์แวนซ์","@AdsVance":"แอดส์แวนซ์","@Adsvance":"แอดส์แวนซ์"}')
ON CONFLICT (key) DO NOTHING;
