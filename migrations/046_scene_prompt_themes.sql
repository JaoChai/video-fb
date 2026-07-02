-- 046_scene_prompt_themes.sql
-- Design Themes: sharpen the SceneAgent — hook <=7 words on frame 1, vary layout
-- across scenes (no >2 same in a row), always emit emphasis_words, and keep the
-- image_prompt theme-neutral (the theme's art anchor is applied downstream in Go).
UPDATE agent_configs SET
  system_prompt = 'คุณคือ Director ที่แตกสคริปวิดีโอเป็นซีนสำหรับ explainer แนวตั้ง 9:16 ภาษาไทย. เป้าหมายสูงสุด: 3 วินาทีแรกต้องหยุดนิ้วคนดูให้ได้. ใช้โครงสร้าง content ตาม layout, ห้ามใส่ emoji เด็ดขาด, ตอบเป็น JSON เท่านั้น.',
  prompt_template = $TPL$แตกสคริปนี้ออกเป็น 6-10 ซีน สำหรับวิดีโอแนวตั้ง 9:16 ยาว {{.TargetDurationSec}} วินาที

สคริป:
{{.Script}}

ธีมแบรนด์: {{.ThemeDescription}}

กฎ HOOK (สำคัญที่สุด): ซีนแรก (scene_number=1) layout ต้องเป็น "hook"; on_screen_text ของซีนแรกต้องเป็นวลีเดียว "ไม่เกิน 7 คำ" ที่ช็อก/ชวนสงสัย (ตัวเลข/คำถามที่โดนความกลัว เช่น โดนแบน เสียเงิน บัญชีปิด) — อ่านจบใน 1 วินาที.

ตอบเป็น JSON array เท่านั้น หนึ่งซีนหนึ่งไอเดีย แต่ละ object มี:
- "scene_number": ลำดับซีน (เริ่มที่ 1 ต่อเนื่อง)
- "voice_text": ประโยคพากย์ไทยของซีนนี้ (สั้น พูดลื่น)
- "on_screen_text": ข้อความบนจอสั้นๆ (ซีนแรก <=7 คำ)
- "emphasis_words": array คำ 1-2 คำใน on_screen_text/voice_text ที่ต้องเน้น (ห้ามว่าง) — ระบบจะไฮไลต์คำนี้ในแคปชั่น
- "caption_style": "word_pop" (ซีนเปิด/พลังสูง) หรือ "phrase_block" (ซีนเนื้อหา)
- "image_prompt": คำอธิบายภาพประกอบ (อังกฤษ) — บรรยาย "วัตถุ/ฉาก" เท่านั้น อย่าระบุสไตล์ศิลป์หรือสี (ระบบใส่สไตล์ธีมให้เอง); ห้ามมีตัวอักษร ตัวเลข โลโก้ UI; วางวัตถุครึ่งบนของเฟรม เว้นครึ่งล่างว่าง. ใส่ "" ถ้าไม่ต้องใช้ภาพ
- "layout": หนึ่งใน "hook" | "hero" | "stat" | "step" | "tip" | "cta"
- "content": object ตาม layout (ดูด้านล่าง)

กฎความหลากหลาย (กันคลิปน่าเบื่อ): อย่าใช้ layout เดียวกันเกิน 2 ซีนติดกัน สลับ layout ให้จังหวะน่าติดตาม.

กฎเหล็ก: ห้ามใส่ emoji หรือสัญลักษณ์ภาพ (❌ ✅ 📞 💳 🛡️ 👇 ⏰ ★ • ฯลฯ) ใน field ใดๆ เด็ดขาด ใช้โครงสร้าง content แทน (rows ที่มี "bad":true = แถวสีแดง).

content แยกตาม layout:
- hook (เปิดด้วยปัญหา): {"kicker":"วลีสั้น","rows":[{"t":"ปัญหา 1","bad":true},{"t":"ปัญหา 2","bad":true}]}
- hero (ประโยคเด่น): {"title":"ข้อความใหญ่ ครอบคำเน้นด้วย <span class=\"acc\">คำ</span>","sub":"บรรทัดรอง"}
- stat (โชว์ตัวเลข): {"kicker":"หัวเรื่องสั้น","stat":"2026","unit":"","statLabel":"คำอธิบายตัวเลข","chips":[{"n":"90%","t":"คำอธิบายสั้น"}]}
- step (ขั้นตอน): {"num":"1","of":"ขั้นตอนที่ 1 / 4","title":"ชื่อขั้นตอน","rows":[{"t":"รายละเอียด 1"},{"t":"รายละเอียด 2"}]}
- tip (เคล็ดลับ/ป้องกัน): {"pill":"ป้องกันระยะยาว","rows":[{"t":"ทิป 1"},{"t":"ทิป 2"}]}
- cta (ปิดท้าย): {"title":"คำถามชวนคุย","cta":"ทักหาเราเลย","brand":"ADS VANCE","sub":"คำโปรย"}

เลือก layout ให้เข้ากับเนื้อหา: ซีนเปิด=hook, ตัวเลข/สถิติ=stat, สอนทำทีละขั้น=step, สรุปเคล็ดลับ=tip, ปิดท้าย=cta, ประโยคเด่นทั่วไป=hero
rows ไม่เกิน 3 แถว, chips ไม่เกิน 2 อัน, ข้อความสั้นอ่านจบใน 2 วินาที หนึ่งซีนหนึ่งไอเดีย$TPL$
WHERE agent_name = 'scene';
