-- 055 TikTok engagement fixes
-- Root cause of TikTok low distribution (analysis 2026-07-15):
--   (a) every clip's CTA drove viewers OFF-platform (ทักแชท/ดูช่องทาง) →
--       comments = 0 on all clips → TikTok has no engagement signal to rank on.
--   (b) TikTok posts carried no hashtags → no interest-cluster routing.
-- This migration:
--   1. rewrites the script agent's answer_script/voice_script CTA to drive
--      ON-platform comments (a question that demands a reply) — the single
--      strongest TikTok ranking lever — while keeping convert-role soft sell.
--   2. seeds a tunable tiktok_hashtags setting (read at publish time in
--      internal/publisher/publisher.go PublishTikTok).
-- Idempotent: REPLACE is a no-op once the old substring is gone;
-- INSERT ... ON CONFLICT DO NOTHING for the setting.

-- 1. answer_script CTA → comment-driving
UPDATE agent_configs
SET prompt_template = REPLACE(
	prompt_template,
	$old$- "answer_script": สคริปต์คำตอบภาษาไทยเต็ม 300-500 คำ เป็นธรรมชาติพูดได้ จบด้วย CTA ตามบทบาทคลิป (reach = ชวนติดตาม/ดูคลิปต่อ; convert = ชวนทักแชท/ดูช่องทางใต้คลิปเรื่องบัญชีโฆษณา แบบ soft sell ไม่ใช่โฆษณาขายบัญชีทั้งคลิป)$old$,
	$new$- "answer_script": สคริปต์คำตอบภาษาไทยเต็ม 300-500 คำ เป็นธรรมชาติพูดได้ จบด้วย CTA ที่กระตุ้นให้ "คอมเมนต์ใต้คลิป" เสมอ — ปิดท้ายด้วยคำถามสั้นๆ ที่ชวนคนดูตอบในคอมเมนต์ (เช่น "บัญชีคุณเคยเจอเคสนี้ไหม คอมเมนต์เล่าให้ฟังหน่อย" หรือ "คุณจะเลือกทางไหน 1 หรือ 2 พิมพ์บอกใต้คลิป") เพราะคอมเมนต์คือสัญญาณที่ทำให้คลิปถูกกระจายต่อบน TikTok. reach = เน้นชวนคอมเมนต์แล้วชวนติดตาม; convert = ชวนคอมเมนต์ก่อน แล้วค่อย soft sell สั้นๆ ว่ามีช่องทางปรึกษาเรื่องบัญชีโฆษณา "ดูช่องทางในโปรไฟล์" (ห้ามใส่ลิงก์/เบอร์/line id ในสคริปต์)$new$)
WHERE agent_name = 'script';

-- 2. voice_script inherits the comment CTA (it is the spoken/narrated track)
UPDATE agent_configs
SET prompt_template = REPLACE(
	prompt_template,
	$old$- "voice_script": สคริปต์สำหรับ voiceover ภาษาไทย สั้นกว่า answer_script 150-300 คำ$old$,
	$new$- "voice_script": สคริปต์สำหรับ voiceover ภาษาไทย สั้นกว่า answer_script 150-300 คำ จบด้วย CTA ชวนคอมเมนต์ใต้คลิปเหมือน answer_script$new$)
WHERE agent_name = 'script';

-- 3. TikTok hashtags (broad + niche, on-platform only — no @handles/links)
INSERT INTO settings (key, value) VALUES
('tiktok_hashtags', '#ยิงแอด #การตลาดออนไลน์ #คนทำแอด #บัญชีโฆษณาโดนแบน #เฟสแบน #mediabuyer')
ON CONFLICT (key) DO NOTHING;
