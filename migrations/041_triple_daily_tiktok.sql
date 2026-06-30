-- 041_triple_daily_tiktok.sql
-- Match TikTok publishing to the 3-clips/day production cadence (migration 039).
-- Each publish_tiktok tick posts exactly ONE clip (newest-first drip,
-- publisher.PublishTikTok, LIMIT 1), so 1 slot/day fell behind 3 produced/day and
-- the TikTok backlog grew. Replace the single Midnight (00:00) slot with three
-- slots at 09:00 / 15:00 / 21:00 (Asia/Bangkok) — offset +3h from the 06/12/18
-- produce slots so a freshly produced clip is ready before its post, spaced 6h
-- apart, with 21:00 hitting Thai TikTok prime time. TikTok posts PUBLIC immediately.
--
-- Note: 3 posts/day matches 3 produced/day (backlog stops growing) but does not
-- drain the existing backlog faster than it accrues.
--
-- Rollback (restore single midnight slot):
--   DELETE FROM schedules WHERE action = 'publish_tiktok';
--   INSERT INTO schedules (name, cron_expression, action, enabled)
--     VALUES ('Midnight TikTok', '0 0 * * *', 'publish_tiktok', TRUE);

DELETE FROM schedules WHERE action = 'publish_tiktok';

INSERT INTO schedules (name, cron_expression, action, enabled) VALUES
    ('Morning TikTok',   '0 9 * * *',  'publish_tiktok', TRUE),
    ('Afternoon TikTok', '0 15 * * *', 'publish_tiktok', TRUE),
    ('Evening TikTok',   '0 21 * * *', 'publish_tiktok', TRUE);
