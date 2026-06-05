-- Migration 028: retune script agent metadata for SEARCH intent.
-- Target audience searches for fixes when their ad accounts break, so:
--  - youtube_title: lead with the searched problem phrase; do NOT add brand
--    (orchestrator.validateScript appends " | Ads Vance" exactly once).
--  - youtube_description: first line = searchable problem+fix summary, THEN contact lines.
--  - youtube_tags: real search phrases.
-- Scene structure unchanged from 017 (single-scene).

UPDATE agent_configs
SET prompt_template = $tmpl$สร้าง voice script + ข้อมูล metadata สำหรับวิดีโอสั้น

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

- "youtube_title": พาดหัวสำหรับ "การค้นหา" — **ขึ้นต้นด้วยคำปัญหาที่กลุ่มเป้าหมายพิมพ์ค้นจริง** ตอนเจอปัญหา (เช่น "บัญชีโฆษณาโดนแบน", "BM โดนระงับ", "เพิ่มงบแล้วแอดโดนแบน") วางคำสำคัญไว้ต้นประโยค ตามด้วยวิธีแก้สั้นๆ เป็นภาษาที่คนพิมพ์ค้นจริง ไม่เกิน 55 ตัวอักษร
  **ห้ามเติมชื่อแบรนด์เองเด็ดขาด** (ห้ามมี " | Ads Vance", "{Ads Vance}", "(Ads Vance)", "Ads Vance" ใน youtube_title) — ระบบจะเติมต่อท้ายให้อัตโนมัติ
  ห้ามใส่ URL, line id, @handle ใน youtube_title
- "youtube_description": บรรทัดแรกเป็น **สรุปปัญหา + บอกว่ามีวิธีแก้** ด้วยภาษาที่คนค้นหา (1-2 ประโยค ใส่คำค้นสำคัญของหัวข้อนั้น) จากนั้นเว้นบรรทัด แล้วต่อด้วย 2 บรรทัดติดต่อนี้เป๊ะๆ ห้ามแก้:
  "ติดต่อทีมงาน line id : @adsvance\n\nเข้ากลุ่มเทเรแกรมเพื่อรับข่าวสาร : https://t.me/adsvancech"
- "youtube_tags": array ของ **วลีที่คนพิมพ์ค้นหาจริง** (ภาษาพูดไทย + ศัพท์หลัก เช่น "เฟสแบน", "บัญชีโฆษณาโดนระงับ", "แก้บัญชีโดนแบน", "facebook ads ban", "BM โดนแบน") 5-10 tag

ห้ามแนะนำการทำผิดนโยบาย Facebook$tmpl$
WHERE agent_name = 'script';
