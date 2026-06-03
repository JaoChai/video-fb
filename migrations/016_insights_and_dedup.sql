-- Migration 016: Separate auto-tune insights from human-controlled skills + semantic dedup support

-- 1. New insights column — the ONLY field the weekly analyzer may write to
ALTER TABLE agent_configs ADD COLUMN IF NOT EXISTS insights TEXT NOT NULL DEFAULT '';

-- 2. Embedding column on topic_history for semantic dedup
ALTER TABLE topic_history ADD COLUMN IF NOT EXISTS embedding vector(1536);
CREATE INDEX IF NOT EXISTS topic_history_embedding_idx ON topic_history
    USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

-- 3. Archive current (LLM-drifted) skills into history, then reset to human baseline
INSERT INTO agent_prompt_history (agent_name, old_prompt, new_prompt, reason)
SELECT agent_name, skills, '', '[reset] Reset LLM-drifted skills to human baseline (migration 016)'
FROM agent_configs
WHERE agent_name IN ('question', 'script', 'image') AND skills != '';

-- 4. Human baseline skills — diversity-first, persona-aware
UPDATE agent_configs SET skills = $$สร้างคำถามที่หลากหลายจริงๆ ทั้งมุมปัญหา ระดับความลึก และสถานการณ์
- กลุ่มเป้าหมายคือคนยิงแอดจริงจัง (เจ้าของธุรกิจออนไลน์, media buyer, agency) ไม่ใช่มือใหม่หัดยิงแอด
- คำถามต้องเจาะจง มีรายละเอียดสถานการณ์จริง (ตัวเลขงบ, ระยะเวลา, สิ่งที่ลองแล้ว)
- ห้ามตั้งคำถามที่ความหมายซ้ำหรือใกล้เคียงกับหัวข้อที่เคยทำแล้ว แม้จะใช้คำต่างกัน
- กระจายความหลากหลาย: ปัญหาเร่งด่วน / เทคนิคขั้นสูง / ความเข้าใจผิดที่พบบ่อย / การตัดสินใจเชิงกลยุทธ์$$,
    insights = ''
WHERE agent_name = 'question';

UPDATE agent_configs SET skills = $$เขียนสคริปต์ให้น่าฟังและหลากหลาย
- กลุ่มเป้าหมายคือคนยิงแอดจริงจัง ใช้ภาษาที่คนในวงการเข้าใจ ไม่ต้องอธิบายพื้นฐานเกิน
- หมุนเวียนวิธีเปิดเรื่อง อย่าใช้รูปแบบเดิมติดกัน: (1) เปิดด้วยคำตอบ/ตัวเลขทันที (2) เปิดด้วยสถานการณ์เร้าใจ (3) เปิดด้วยคำถามกระแทกใจ
- หมุนเวียน CTA ปิดท้าย 3 แบบ: (1) "ติดต่อทีมงานแอดส์แวนซ์ทางไลน์ ไอดีแอดส์แวนซ์ได้เลยครับ" (2) "เข้ากลุ่มเทเลแกรมแอดส์แวนซ์ มีเทคนิคแบบนี้ทุกวันครับ" (3) "ถ้าเจอปัญหาแบบนี้อยู่ ทักทีมงานแอดส์แวนซ์ได้เลยครับ"
- คำตอบต้อง actionable ทำตามได้จริง ไม่ใช่คำแนะนำลอยๆ$$,
    insights = ''
WHERE agent_name = 'script';

UPDATE agent_configs SET skills = $$สร้างภาพที่หลากหลาย ไม่ใช้องค์ประกอบเดิมซ้ำทุกคลิป
- หมุนเวียนสไตล์: chat bubble / dashboard mockup / notification screen / split comparison
- สีและ mood ตาม brand theme แต่ composition ต้องต่างกันในแต่ละคลิป$$,
    insights = ''
WHERE agent_name = 'image';
