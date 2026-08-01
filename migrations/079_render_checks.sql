-- 079_render_checks.sql
-- บันทึกผลของด่านตรวจ Hyperframes ทุกคลิป (lint / inspect / render) แบบ append-only
-- เฟส 1 = สังเกตการณ์ล้วน ไม่มีด่านไหนเปลี่ยนพฤติกรรมการเผยแพร่
-- Idempotent; no goose syntax (RunMigrations ไม่หุ้ม transaction)

CREATE TABLE IF NOT EXISTS render_checks (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clip_id     UUID NOT NULL REFERENCES clips(id) ON DELETE CASCADE,
    stage       TEXT NOT NULL,               -- lint | inspect | render
    passed      BOOLEAN NOT NULL,
    duration_ms INT NOT NULL DEFAULT 0,      -- วัดว่า lint แพงแค่ไหน (ยังไม่เคยมีใครวัด)
    findings    JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_render_checks_clip_id ON render_checks (clip_id);
CREATE INDEX IF NOT EXISTS idx_render_checks_stage_created ON render_checks (stage, created_at DESC);
