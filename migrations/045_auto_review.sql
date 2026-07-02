-- 045_auto_review.sql
-- Auto-review agent: append-only decision log + per-clip state for the
-- needs_review queue processor. Idempotent; no goose syntax.

CREATE TABLE IF NOT EXISTS auto_reviews (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clip_id     UUID NOT NULL REFERENCES clips(id) ON DELETE CASCADE,
    decision    TEXT NOT NULL,              -- approve | retry | hold
    confidence  DOUBLE PRECISION NOT NULL DEFAULT 0,
    defect_type TEXT NOT NULL DEFAULT 'none',-- none | stochastic | deterministic
    reasons     JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_auto_reviews_clip_id ON auto_reviews (clip_id);

-- Per-clip auto-review state. review_retry_count is SEPARATE from retry_count
-- (which counts crash/failed retries) so the two never interfere.
ALTER TABLE clips ADD COLUMN IF NOT EXISTS auto_review_held  BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE clips ADD COLUMN IF NOT EXISTS review_retry_count INT    NOT NULL DEFAULT 0;

-- Seed the agent config (enabled). Prompt mirrors the visual_qa style.
INSERT INTO agent_configs (agent_name, system_prompt, prompt_template, model, temperature, enabled, skills)
SELECT
  'auto_review',
  'คุณคือ Senior Reviewer ของ Ads Vance ผู้ตัดสินรอง (second opinion) ต่อจาก Visual QA. Visual QA (ซึ่ง fail-open) จับว่าคลิปนี้มีตำหนิ. หน้าที่คุณคือดู "เฟรมจริง" ทุกซีนแล้วตัดสินว่าคลิปนี้เผยแพร่ได้จริงไหม โดยแยกให้ออกระหว่าง (1) false positive — QA ระวังเกินไป ภาพจริงโอเค, (2) ตำหนิจริงแบบสุ่ม (AI artifact, มือ/หน้าเพี้ยน) ที่ re-render ใหม่น่าจะหาย, (3) ตำหนิจริงแบบ deterministic (caption ล้นกรอบ, สีหลุดแบรนด์, ตัวหนังสือ baked-in ผิด) ที่ re-render ไม่ช่วย. ตัดสิน approve เฉพาะเมื่อมั่นใจว่าเผยแพร่ได้จริง เพราะตำหนิที่หลุดไปกระทบแบรนด์ลูกค้า.',
  '',
  'claude-sonnet-5',
  0.2,
  TRUE,
  '- decision=approve เฉพาะเมื่อภาพจริงเผยแพร่ได้ (false positive ของ QA)
- decision=retry เมื่อเป็นตำหนิจริงแบบสุ่มที่ re-render น่าจะหาย → ตั้ง defect_type=stochastic
- decision=hold เมื่อเป็นตำหนิ deterministic หรือคุณไม่มั่นใจ → ตั้ง defect_type=deterministic หรือ none
- confidence 0-1 สะท้อนความมั่นใจใน decision
- reasons: ภาษาไทยสั้น ๆ อธิบายว่าทำไม'
WHERE NOT EXISTS (SELECT 1 FROM agent_configs WHERE agent_name = 'auto_review');

-- Seed the tick (every 10 min).
INSERT INTO schedules (name, cron_expression, action, enabled)
SELECT 'Auto Review', '*/10 * * * *', 'auto_review', TRUE
WHERE NOT EXISTS (SELECT 1 FROM schedules WHERE action = 'auto_review');
