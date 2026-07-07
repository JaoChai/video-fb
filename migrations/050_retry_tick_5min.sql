-- 050_retry_tick_5min.sql
-- Fast pipeline (2026-07-07): a failed clip used to sit up to 15 minutes
-- waiting for the next retry_failed tick. Tighten the tick to every 5 minutes —
-- combined with the in-process retry nudge (failClip → RetryAllFailed after
-- 15s) this bounds the retry wait to seconds normally, ≤5 min worst case.
-- The tick is cheap when there is nothing to retry, and the production gate
-- already skips overlapping runs.
--
-- Rollback:
--   UPDATE schedules SET cron_expression = '*/15 * * * *' WHERE action = 'retry_failed';

UPDATE schedules SET cron_expression = '*/5 * * * *' WHERE action = 'retry_failed';
