-- 047_critic_visualqa_themes.sql
-- Design Themes: teach the critic to enforce the sharp hook + real emphasis, and
-- teach visual_qa that image MEDIA now varies by theme (photo / 3D / illustration
-- are all valid) — only navy+orange brand drift or true breakage should fail.

-- Critic: add hook<=7 words + emphasis presence to the review skills.
UPDATE agent_configs SET
  skills = skills || E'\n- Hook: scene แรก on_screen_text ต้อง <=7 คำ และช็อก/ชวนสงสัยใน 1 วิ. ถ้ายาว/อืด ให้ตัดให้สั้นคม.'
    || E'\n- emphasis_words: ทุกซีนต้องมีคำเน้น 1-2 คำที่ตรงประเด็น (ระบบใช้ไฮไลต์แคปชั่น). ถ้าว่าง/ผิด ให้เติม.'
    || E'\n- อย่าบังคับสไตล์ภาพใน image_prompt (สไตล์มาจากธีม) — ปรับได้แค่ "วัตถุ/ฉาก" ให้ตรงเนื้อหา.'
WHERE agent_name = 'critic' AND skills NOT LIKE '%emphasis_words: ทุกซีน%';

-- Visual QA: media varies by theme; do NOT fail a clip just because it is a photo
-- or a 3D render instead of flat illustration.
UPDATE agent_configs SET
  skills = skills || E'\n- สื่อภาพหลากหลายตามธีมได้ (ภาพถ่ายจริง / 3D เคลย์ / เวกเตอร์ / นีออน) — อย่าตั้ง ok=false เพราะ"ไม่ใช่เวกเตอร์แบน". ตัดสินที่แบรนด์ (navy+ส้ม) และความพังจริงเท่านั้น.'
WHERE agent_name = 'visual_qa' AND skills NOT LIKE '%สื่อภาพหลากหลายตามธีม%';
