-- Fix: migration 022 switched the script agent's `skills` to multi-scene but left
-- `prompt_template` hard-locked to single-scene ("scenes: array เพียง 1 ตัวเท่านั้น"),
-- so the model kept returning 1 scene. This updates prompt_template to instruct
-- 3-6 content-driven scenes with per-scene headline/voice/bg_hint (≤60s total),
-- matching agent.GeneratedScene (scene_number, scene_type, text_content, voice_text,
-- bg_hint, duration_seconds) and the Normalize()/orchestrator guard from Phase 1.

UPDATE agent_configs
SET prompt_template = $tmpl$สร้าง voice script แบบ multi-scene + metadata สำหรับวิดีโอสั้น

โครงสร้างวิดีโอ: แตกคำตอบเป็น "ฉาก" 3-6 ฉาก เล่าเรื่องต่อเนื่อง (ห้ามเกิน 6) — เสียงทุกฉากต่อกันเป็นคลิปเดียว รวมพูดจบใน 60 วินาที

หัวข้อ: "{{.Question}}"
โดย: {{.QuestionerName}}
หมวด: {{.Category}}

รูปแบบการเล่า: {{.FormatInstruction}}

กลุ่มเป้าหมาย: {{.AudiencePersona}}

ข้อมูลอ้างอิง:
{{.RAGContext}}

ตอบเป็น JSON object มี:
- "scenes": array 3-6 ฉาก เรียงเล่าเรื่อง แต่ละ object:
  - "scene_number": ลำดับเริ่มที่ 1
  - "scene_type": เลือกจาก "hook" | "problem" | "step" | "win" | "cta" (ฉากแรก=hook, ฉากท้าย=cta)
  - "text_content": พาดหัวสั้นมากขึ้นจอในฉากนั้น (≤8 คำ)
  - "voice_text": บทพากย์เฉพาะฉากนั้น (ภาษาพูดเป็นกันเอง, ใช้ ... สำหรับจังหวะหายใจ)
  - "bg_hint": บรรยายบรรยากาศพื้นหลังของฉากสั้นๆ (เช่น "แดชบอร์ดโฆษณาเรืองแสงสีส้ม") — ภาพห้ามมีตัวหนังสือ
  - "duration_seconds": ประมาณความยาวฉากนั้น (วินาที)
- "total_duration_seconds": รวมทุกฉาก ≤ 60
- "youtube_title": ดึงดูด สั้น ไม่เกิน 70 ตัวอักษร ลงท้ายด้วย {Ads Vance}
- "youtube_description": ใช้แค่ 2 บรรทัดนี้เท่านั้น:
  "ติดต่อทีมงาน line id : @adsvance"
  "เข้ากลุ่มเทเรแกรมเพื่อรับข่าวสาร : https://t.me/adsvancech"
- "youtube_tags": array ของ tag ภาษาไทย+อังกฤษ

ข้อบังคับ voice_text: ห้ามมีอักขระ "@" และห้ามมี URL ใดๆ (TTS อ่านไม่ออก เสียงจะตัด); เรียกแบรนด์ว่า "แอดส์แวนซ์"; ฉาก cta ปิดท้ายทำนอง "ติดต่อทีมงานแอดส์แวนซ์ทางไลน์ ไอดีแอดส์แวนซ์ หรือเข้ากลุ่มเทเลแกรมแอดส์แวนซ์ได้เลยครับ"$tmpl$
WHERE agent_name = 'script';
