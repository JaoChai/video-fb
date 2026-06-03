-- Hyperframes pipeline, step 1: add the composition agent + per-clip design tracking.
-- The composition agent (LLM) designs each video's look (layout, accent colors,
-- animation speed, highlighted words, timed point cards). It uses the same model
-- as the script agent. Does NOT change the image agent yet — the Hyperframes path
-- starts with a CSS background, so image generation (and FFmpeg fallback) is
-- untouched. GPT-image-as-background-art comes in a later migration.

INSERT INTO agent_configs (agent_name, system_prompt, prompt_template, model, temperature, enabled, skills, insights)
SELECT
    'composition',
    $$คุณคือนักออกแบบวิดีโอสั้นแนวตั้ง (9:16) สำหรับช่อง "ADS VANCE" ที่ตอบคำถาม Facebook Ads ให้เจ้าของธุรกิจไทย
หน้าที่: ออกแบบ "หน้าตา" ของวิดีโอ Dynamic Karaoke จากบทพากย์ที่ให้มา
ตอบกลับเป็น JSON เท่านั้น ไม่มีคำอธิบายอื่น$$,
    $$หัวข้อ: {{.Question}}
หมวด: {{.Category}}
ผู้ถาม: {{.QuestionerName}}
ความยาว: {{.DurationSeconds}} วินาที

บทพากย์เต็ม:
{{.VoiceText}}

ช่วงเวลาคำพูด (วินาที) สำหรับ sync การ์ด:
{{.SegmentsContext}}

ออกแบบวิดีโอ ตอบ JSON:
{
  "accent_color": "#rrggbb",
  "secondary_accent": "#2fd17a",
  "animation_speed": "fast|normal|slow",
  "kicker": "ป้ายหมวดสั้น",
  "highlight_words": ["คำ1","คำ2"],
  "cards": [
    {"type":"cause","start":13.7,"end":24.6,"kicker":"สาเหตุ","body":"...","step":0},
    {"type":"step","start":27.7,"end":35.2,"kicker":"วิธีแก้","body":"...","step":1}
  ]
}$$,
    model,
    0.7,
    TRUE,
    $$แนวทางออกแบบ (design skills):
- accent_color เลือกตามอารมณ์เนื้อหา: ปัญหา/เตือน=#ff5a52, เทคนิค/ทั่วไป=#ff6b2b, อัปเดต/ข่าว=#3b82f6
- secondary_accent = สีการ์ดผลลัพธ์เชิงบวก ใช้ #2fd17a เสมอ
- highlight_words: เลือกคำสำคัญในหัวข้อ 1-2 คำ (คำที่สะดุดตาสุด)
- kicker: ป้ายหมวดสั้นๆ ตัวพิมพ์ใหญ่ เช่น "PIXEL & CAPI"
- cards: สร้าง 3-5 ใบไล่ประเด็นตามบทพากย์ — type "cause"(สาเหตุ/ปัญหา), "step"(ขั้นตอนแก้ ใส่ step 1,2,3), "win"(ผลลัพธ์)
  - แต่ละใบ start/end ต้อง sync กับช่วงเวลาในบทพากย์ที่พูดถึงเรื่องนั้น (ใช้ timestamp ที่ให้มา)
  - body สั้น ≤15 คำ
  - การ์ดห้ามเวลาทับกัน
- animation_speed: เนื้อหาเร่งด่วน=fast, เทคนิค=normal, เรื่องเล่า=slow$$,
    ''
FROM agent_configs
WHERE agent_name = 'script'
ON CONFLICT (agent_name) DO NOTHING;

-- Track which design each clip used, so the analyzer can later learn which
-- designs perform best (self-improvement loop extension).
ALTER TABLE clips ADD COLUMN IF NOT EXISTS composition_style TEXT NOT NULL DEFAULT '';
