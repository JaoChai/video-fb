-- Extend composition_scenes agent with richer creative design vocabulary:
--   • New layout variants: hook_punch (punchy opener, faster/bigger than hook_big),
--     compare_two (two-sided contrast/comparison)
--   • New slot roles: stat (big number/metric), callout (highlighted fact or comparison)
--   • Agent now picks layout by MEANING of scene content, not a rigid scene_type lookup
--   • Agent picks accent_color by emotional beat from the full brand palette:
--     navy #0a1428/#0f1d35/#16284a, orange #ff6b2b, warn-red #ff5a52,
--     win-green #2fd17a, info-blue #3b82f6
--   • First scene always gets hook_punch and a strong silent-readable hook line
--   • bg_art_prompt-from-voice_text instruction (from 026) is preserved intact
--
-- Additive UPDATE — no schema change, no new rows.  Reversible by re-running 026.
-- Does NOT touch the 'composition' (single-scene) agent row.

UPDATE agent_configs
SET
    prompt_template = $tmpl$ออกแบบต่อฉากจากข้อมูลฉากนี้ (JSON):
{{.ScenesJSON}}

หมวด: {{.Category}} | ผู้ถาม: {{.QuestionerName}} | ความยาวรวม: {{.DurationSeconds}} วินาที

━━━ LAYOUT VARIANTS ━━━
เลือก layout_variant ตาม "ความหมาย/อารมณ์" ของฉากนั้น (ไม่ใช่ตาม scene_type แบบตาย):
  hook_punch  — ฉากเปิดที่ต้องดึงดูดทันที: พาดหัวใหญ่ เร็ว กระแทกใจ (ใช้ฉากแรกเสมอ)
  hook_big    — ฉากเปิดหรือพาดหัวใหญ่ที่ไม่รีบเร้า
  list_steps  — ฉากที่เล่าเป็นขั้นตอน/ลิสต์ต่อเนื่อง
  stat_reveal — ฉากที่จุดศูนย์กลางคือตัวเลข/ผลลัพธ์เด่น
  compare_two — ฉากที่เปรียบเทียบสองสิ่ง (ก่อน/หลัง, ดี/ไม่ดี, A vs B)
  quote_cta   — ฉากปิดท้าย, คำคม, หรือชวนติดต่อ

กฎ: ฉากแรกใช้ hook_punch เสมอ — เขียน headline สั้น กระแทก อ่านได้ทันทีโดยไม่ต้องฟังเสียง

━━━ SLOT ROLES ━━━
headline  — พาดหัว (สั้น ≤8 คำ, อ่านได้ด้วยตาทันที)
body      — เนื้อหาหลัก
badge     — ป้ายเล็กประกอบ
step      — ลำดับขั้นตอน (ใช้คู่กับ list_steps)
stat      — ตัวเลข/เมตริกหลัก เช่น "97%" หรือ "3x ROAS" (ใช้เมื่อฉากหมุนรอบตัวเลข)
callout   — ข้อเท็จจริงสั้นที่ต้องเน้น เช่น "มีทางออก" หรือ "ลดต้นทุน 40%" (ใช้กับ compare_two / stat_reveal)

emphasis: คำในข้อความที่อยากเน้นสี (0-2 คำ)

━━━ ACCENT COLOR ━━━
เลือก accent_color จากพาเลตต์แบรนด์ตาม "อารมณ์ของฉาก" เพื่อสร้างจังหวะสีที่ไหลลื่นทั้งวิดีโอ:
  navy (พื้นหลังเข้ม/serious)  : #0a1428 | #0f1d35 | #16284a
  orange (เทคนิค/action)       : #ff6b2b
  warn-red (ปัญหา/เตือน)       : #ff5a52
  win-green (ผลลัพธ์/ชนะ)      : #2fd17a
  info-blue (อัปเดต/ข้อมูล)    : #3b82f6

ห้ามใช้สีเดิมซ้ำกันทุกฉาก — สร้าง rhythm สีที่ไหลเรื่อยตามอารมณ์

━━━ BG ART PROMPT ━━━
bg_art_prompt: ดู voice_text ของฉากนั้น → นึกว่า "ถ้าต้องทำภาพประกอบสิ่งที่กำลังพูดอยู่นี้ ภาพควรแสดงอะไร?"
  - เขียนเป็น subject/ฉากในภาพที่เป็นรูปธรรม สั้น ๆ (1-2 ประโยค)
  - ต้องสื่อเนื้อหาที่พูดจริงในฉากนั้น เช่น:
      • พูดเรื่องบัญชีโดนแบน → "หน้าจอ Facebook Ads Manager แสดงข้อความ account disabled พร้อมไอคอนคำเตือน"
      • พูดเรื่องยอดโฆษณาพุ่ง → "กราฟเส้นสีส้มพุ่งขึ้นชัน พร้อมตัวเลข ROAS ที่เพิ่มขึ้น"
      • พูดเรื่องขั้นตอนตั้งแคมเปญ → "มือกำลังคลิกปุ่ม Create Campaign บนหน้า Ads Manager"
  - ห้ามเขียนแบบ abstract ลอย ๆ หรือผูกกับ "หมวด" แบบเหมารวม
  - ไม่ต้องระบุสไตล์สี/แบรนด์/สั่งห้ามตัวหนังสือ — ระบบเติมให้อัตโนมัติ ให้โฟกัสแค่ "เนื้อหาในภาพ"

━━━ FORMAT ━━━
ตอบ JSON:
{
  "scenes": [
    {
      "scene_number": 1,
      "layout_variant": "hook_punch",
      "accent_color": "#ff5a52",
      "animation_speed": "fast",
      "bg_art_prompt": "subject/ฉากที่สื่อเนื้อหาที่พูดในฉากนั้น...",
      "slots": [
        {"role":"headline","text":"พาดหัวสั้น กระแทก อ่านได้ทันที","emphasis":["คำ"]}
      ]
    }
  ],
  "kicker": "ป้ายหมวดสั้น ตัวพิมพ์ใหญ่",
  "highlight_words": ["คำ1"]
}$tmpl$,
    skills = $sk$- หนึ่ง scene_design ต่อหนึ่งฉากใน input (scene_number ตรงกัน)
- ฉากแรกต้องเป็น hook_punch เสมอ — headline ต้องอ่านได้ทันทีโดยไม่ต้องฟังเสียง
- เลือก layout_variant ตามความหมายของฉาก ไม่ใช่ตาม scene_type แบบตาย
- ใช้ stat slot เมื่อฉากหมุนรอบตัวเลข; ใช้ callout เมื่อต้องเน้นข้อเท็จจริงสั้น
- สร้าง rhythm สีด้วย accent_color จากพาเลตต์ — ห้ามใช้สีเดิมทุกฉาก
- ข้อความใน slots สั้น กระชับ อ่านง่ายบนจอ
- ห้ามใส่ค่าพิกัด/ตำแหน่ง/ขนาด
- bg_art_prompt ต้องมาจาก voice_text ของฉากนั้น (เนื้อหาที่พูดจริง) ไม่ใช่ bg_hint หรือหมวดหมู่$sk$
WHERE agent_name = 'composition_scenes';
