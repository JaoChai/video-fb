-- 033_clip_critiques.sql
-- Append-only log of Content Critic reviews. Phase 1 only writes this table;
-- a future Phase 3 (learning loop) reads it to find recurring low-score
-- patterns and tune upstream agents' skills. `applied` is FALSE when the
-- fail-safe kept the original content.
CREATE TABLE IF NOT EXISTS clip_critiques (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clip_id    UUID NOT NULL REFERENCES clips(id) ON DELETE CASCADE,
    score      JSONB NOT NULL DEFAULT '{}'::jsonb,
    changes    JSONB NOT NULL DEFAULT '[]'::jsonb,
    applied    BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_clip_critiques_clip_id ON clip_critiques (clip_id);
