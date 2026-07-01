-- Migration 030: topic-driven pipeline schema.
-- Additive columns for the SceneAgent output + research brief, and seed rows
-- for the new `scene` (Claude) and `metadata` (Gemini) agents.

ALTER TABLE scenes ADD COLUMN IF NOT EXISTS layout_variant TEXT NOT NULL DEFAULT '';
ALTER TABLE scenes ADD COLUMN IF NOT EXISTS on_screen_text TEXT NOT NULL DEFAULT '';
ALTER TABLE scenes ADD COLUMN IF NOT EXISTS emphasis_words JSONB NOT NULL DEFAULT '[]';
ALTER TABLE scenes ADD COLUMN IF NOT EXISTS beat TEXT NOT NULL DEFAULT '';
ALTER TABLE scenes ADD COLUMN IF NOT EXISTS caption_style TEXT NOT NULL DEFAULT '';

ALTER TABLE clips ADD COLUMN IF NOT EXISTS core_message TEXT NOT NULL DEFAULT '';
ALTER TABLE clips ADD COLUMN IF NOT EXISTS narrative_angle TEXT NOT NULL DEFAULT '';
ALTER TABLE clips ADD COLUMN IF NOT EXISTS research_brief JSONB NOT NULL DEFAULT '{}';

-- Seed the SceneAgent (Claude): script -> 6-10 scene JSON.
INSERT INTO agent_configs (agent_name, system_prompt, prompt_template, model, temperature, enabled, skills)
VALUES (
  'scene',
  'คุณคือ Director ที่แตกสคริปวิดีโอเป็นซีนสำหรับ explainer แนวตั้ง 9:16 ภาษาไทย ให้คนดูเข้าใจง่ายและน่าสนใจ ตอบเป็น JSON เท่านั้น',
  $$แตกสคริปนี้ออกเป็น 6-10 ซีน สำหรับวิดีโอแนวตั้ง 9:16 ยาว {{.TargetDurationSec}} วินาที

สคริป:
{{.Script}}

ธีมแบรนด์: {{.ThemeDescription}}

ตอบเป็น JSON array เท่านั้น แต่ละ object มี:
- "scene_number": ลำดับซีน (เริ่มที่ 1)
- "beat": บทบาทในเรื่อง — หนึ่งใน "hook" | "problem" | "payoff" | "cta"
- "voice_text": ประโยคพากย์ไทยของซีนนี้ (สั้น พูดได้ลื่น)
- "on_screen_text": ข้อความบนจอ สั้น อ่านรู้เรื่องตอนปิดเสียง
- "emphasis_words": array คำที่ต้องไฮไลต์ (1-3 คำ)
- "layout_variant": หนึ่งใน "hook_big" | "hook_punch" | "phrase_block" | "stat_reveal" | "quote_cta" | "word_pop" | "static" | "intro" | "outro"
- "caption_style": "phrase_block" หรือ "word_pop"
- "duration_seconds": ความยาวซีนโดยประมาณ (วินาที)
- "image_prompt": คำอธิบายภาพประกอบซีนนี้แบบสั้น (อังกฤษ) หรือ "" ถ้าซีนนี้ไม่ต้องใช้ภาพ

หนึ่งซีนหนึ่งไอเดีย ห้ามยัดสองความคิดในซีนเดียว$$,
  'claude-sonnet-5',
  0.6,
  TRUE,
  'แตกเป็น 6-10 ซีน หนึ่งซีนหนึ่งไอเดีย, เลือก layout_variant ให้เข้าจังหวะ, กำหนด emphasis_words, on_screen_text สั้นอ่านรู้เรื่องตอนปิดเสียง, output JSON ตาม schema'
)
ON CONFLICT (agent_name) DO UPDATE SET
  system_prompt = EXCLUDED.system_prompt,
  prompt_template = EXCLUDED.prompt_template,
  model = EXCLUDED.model,
  temperature = EXCLUDED.temperature,
  skills = EXCLUDED.skills;

-- Seed the MetadataAgent (Gemini): script -> youtube title/desc/tags.
INSERT INTO agent_configs (agent_name, system_prompt, prompt_template, model, temperature, enabled, skills)
VALUES (
  'metadata',
  'คุณสร้าง metadata ยูทูบภาษาไทยแบบ search-intent ตอบเป็น JSON เท่านั้น',
  $$สร้าง metadata สำหรับวิดีโอเรื่อง "{{.Topic}}" หมวด "{{.Category}}"

สคริป:
{{.Script}}

กลุ่มผู้ชม: {{.AudiencePersona}}

ตอบเป็น JSON object เท่านั้น:
- "youtube_title": หัวข้อแบบ search-intent ภาษาไทย ไม่เกิน 55 ตัวอักษร (ระบบจะต่อท้าย " | Ads Vance" ให้เอง ห้ามใส่ชื่อแบรนด์เอง)
- "youtube_description": คำอธิบายกระชับ 2-3 ประโยค
- "youtube_tags": array แท็กภาษาไทย 5-8 คำ$$,
  'gemini-3-5-flash',
  0.6,
  TRUE,
  'title แบบ search-intent ภาษาไทย, ไม่ใส่ชื่อแบรนด์เอง, desc กระชับ, tags ตรงหัวข้อ'
)
ON CONFLICT (agent_name) DO UPDATE SET
  system_prompt = EXCLUDED.system_prompt,
  prompt_template = EXCLUDED.prompt_template,
  model = EXCLUDED.model,
  temperature = EXCLUDED.temperature,
  skills = EXCLUDED.skills;
