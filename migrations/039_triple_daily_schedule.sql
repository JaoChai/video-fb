-- 039_triple_daily_schedule.sql
-- Produce 3 clips/day instead of 1. The live DB had drifted from migration 007
-- (the Midnight 00:00 row was disabled by hand, leaving only Noon active), so
-- this migration resets ALL produce_and_publish rows to a clean trio spaced 6h
-- apart: 06:00, 12:00, 18:00 (Asia/Bangkok — the scheduler runs in that TZ).
--
-- Each tick produces exactly 1 clip (orchestrator.ProduceWeekly(ctx, 1)) then
-- publishes ready clips. 6h spacing >> the ~20m render budget, so ticks never
-- overlap; the production gate skips any tick that would, as a backstop.
-- Slots avoid every other job: TikTok publish 00:00, Analytics 04:00,
-- Learn/Self-Improve 03:00 Mon.
--
-- Rollback (restore single noon clip):
--   DELETE FROM schedules WHERE action = 'produce_and_publish';
--   INSERT INTO schedules (name, cron_expression, action, enabled)
--     VALUES ('Noon Produce & Publish', '0 12 * * *', 'produce_and_publish', TRUE);

DELETE FROM schedules WHERE action = 'produce_and_publish';

INSERT INTO schedules (name, cron_expression, action, enabled) VALUES
    ('Morning Produce & Publish',   '0 6 * * *',  'produce_and_publish', TRUE),
    ('Noon Produce & Publish',      '0 12 * * *', 'produce_and_publish', TRUE),
    ('Evening Produce & Publish',   '0 18 * * *', 'produce_and_publish', TRUE);
