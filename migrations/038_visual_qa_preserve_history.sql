-- 038_visual_qa_preserve_history.sql
-- Keep Visual QA history even after its clip is deleted, so QA stats (esp. the
-- "blocked/rejected" count) stay accurate when a reviewer rejects+deletes a clip.
-- Switch the clip_id FK from ON DELETE CASCADE to ON DELETE SET NULL and allow
-- clip_id to be NULL. Idempotent: DROP IF EXISTS before re-adding the constraint.
ALTER TABLE visual_qa ALTER COLUMN clip_id DROP NOT NULL;

ALTER TABLE visual_qa DROP CONSTRAINT IF EXISTS visual_qa_clip_id_fkey;
ALTER TABLE visual_qa ADD CONSTRAINT visual_qa_clip_id_fkey
    FOREIGN KEY (clip_id) REFERENCES clips(id) ON DELETE SET NULL;
