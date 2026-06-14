-- 035_visual_qa.sql
-- Append-only log of Visual QA runs. One row per clip per QA pass. `passed`
-- FALSE is what drove the clip to status='needs_review' (publish blocked).
-- `issues` is the JSON array of per-scene verdicts (scene_number/ok/issues).
CREATE TABLE IF NOT EXISTS visual_qa (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clip_id    UUID NOT NULL REFERENCES clips(id) ON DELETE CASCADE,
    passed     BOOLEAN NOT NULL,
    issues     JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_visual_qa_clip_id ON visual_qa (clip_id);
