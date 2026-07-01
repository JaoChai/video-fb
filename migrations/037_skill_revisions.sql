-- 037_skill_revisions.sql
-- Phase 3 learning loop. Append-only audit of every automatic skills change the
-- learner makes to an upstream agent. Each row stores the FULL old + new skills
-- text plus a rationale and the critique window, so any auto-applied change is
-- fully revertable by hand:
--   UPDATE agent_configs SET skills = (SELECT old_skills FROM skill_revisions
--     WHERE agent_name = '<name>' ORDER BY created_at DESC LIMIT 1)
--   WHERE agent_name = '<name>';
-- Kill switch for the whole loop:
--   UPDATE agent_configs SET enabled = FALSE WHERE agent_name = 'learner';
CREATE TABLE IF NOT EXISTS skill_revisions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_name     TEXT NOT NULL,
    old_skills     TEXT NOT NULL,
    new_skills     TEXT NOT NULL,
    rationale      TEXT NOT NULL DEFAULT '',
    critique_window INTEGER NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_skill_revisions_agent ON skill_revisions (agent_name, created_at DESC);

-- Seed the learner meta-agent. It reads recurring quality issues and rewrites an
-- upstream agent's skills guidelines. Output is the improved skills TEXT only.
INSERT INTO agent_configs (agent_name, system_prompt, prompt_template, model, temperature, enabled, skills)
SELECT
  'learner',
  'คุณคือ Learner ของ Ads Vance — โค้ชที่ปรับปรุง "skills guidelines" ของ agent ต้นทาง (เช่น scene, script) จากปัญหาคุณภาพที่เกิดซ้ำ ๆ ในงานจริง.

ระบบจะส่งให้คุณ:
- ชื่อ agent ที่กำลังปรับ
- skills guidelines ปัจจุบันของ agent นั้น
- สรุปปัญหาที่เกิดซ้ำ: คะแนนเฉลี่ยรายมิติ (hook/clarity/brand_fit/overall) + เหตุผลที่ critic แก้บ่อยที่สุด

หน้าที่ของคุณ:
- ออกแบบ skills guidelines ฉบับปรับปรุงที่ "แก้ที่ต้นเหตุ" ของปัญหาที่เกิดซ้ำ.
- เก็บของเดิมที่ยังดีไว้ เพิ่ม/แก้เฉพาะส่วนที่จำเป็น (ไม่รื้อทิ้งทั้งหมด).
- เขียนเป็น bullet สั้น กระชับ สั่งการได้จริง ภาษาไทยแบบที่ทีมใช้.

ข้อห้ามเด็ดขาด:
- ห้ามคืน skills ว่างหรือมีแต่ช่องว่าง.
- ถ้าของเดิมดีพออยู่แล้วและปัญหาไม่ชัด ให้ confident=false แล้วคืนของเดิม.
- ตอบเป็น JSON object เท่านั้น.',
  'agent ที่กำลังปรับ: {{.AgentName}}

skills ปัจจุบัน:
{{.CurrentSkills}}

สรุปปัญหาที่เกิดซ้ำ (จาก clip_critiques ช่วง {{.WindowDays}} วันล่าสุด):
{{.PatternSummary}}

จงคืน JSON object รูปแบบนี้เท่านั้น (ห้ามมีข้อความอื่นนอก JSON):
{
  "new_skills": "บทปรับปรุง skills แบบ bullet ภาษาไทย (ห้ามว่าง)",
  "rationale": "อธิบายสั้น ๆ ว่าแก้อะไรเพราะปัญหาอะไร",
  "confident": true
}',
  'claude-sonnet-5',
  0.3,
  TRUE,
  '- แก้ที่ต้นเหตุของปัญหาที่เกิดซ้ำ ไม่ใช่ปลายเหตุ.
- เก็บ guideline เดิมที่ยังได้ผลไว้ เพิ่มเฉพาะที่ขาด.
- เขียนสั้น สั่งการได้จริง วัดผลได้.'
WHERE NOT EXISTS (SELECT 1 FROM agent_configs WHERE agent_name = 'learner');

-- Weekly learning loop: every Monday 03:00 (DB server time). action = 'learn'.
INSERT INTO schedules (name, cron_expression, action, enabled)
SELECT 'Weekly Learn', '0 3 * * 1', 'learn', TRUE
WHERE NOT EXISTS (SELECT 1 FROM schedules WHERE action = 'learn');
