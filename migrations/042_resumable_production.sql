-- 042_resumable_production.sql
-- Resumable production: track the last completed production checkpoint per clip so
-- a retry can skip the LLM stages and resume at rendering, and add a fast retry
-- schedule (every 15 min) so a failed clip recovers in minutes, not the next 6h
-- produce slot. production_stage values: '' (none), 'content_ready', 'rendered'.
--
-- Rollback:
--   ALTER TABLE clips DROP COLUMN IF EXISTS production_stage;
--   DELETE FROM schedules WHERE action = 'retry_failed';

ALTER TABLE clips ADD COLUMN IF NOT EXISTS production_stage TEXT NOT NULL DEFAULT '';

INSERT INTO schedules (name, cron_expression, action, enabled)
SELECT 'Retry Failed', '*/15 * * * *', 'retry_failed', TRUE
WHERE NOT EXISTS (SELECT 1 FROM schedules WHERE action = 'retry_failed');
