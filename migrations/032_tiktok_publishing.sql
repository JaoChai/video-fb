-- 032_tiktok_publishing.sql
-- Add TikTok as a publish target: track a per-clip TikTok post id, store the
-- Zernio TikTok accountId, and add a midnight (Asia/Bangkok) TikTok-only publish
-- schedule. The noon job still does produce + YouTube publish unchanged.

ALTER TABLE clip_metadata ADD COLUMN IF NOT EXISTS zernio_tiktok_post_id TEXT;

INSERT INTO settings (key, value)
SELECT 'zernio_tiktok_account_id', '6a2aafab5f7d1751ab805933'
WHERE NOT EXISTS (SELECT 1 FROM settings WHERE key = 'zernio_tiktok_account_id');

INSERT INTO schedules (name, cron_expression, action, enabled)
SELECT 'Midnight TikTok', '0 0 * * *', 'publish_tiktok', TRUE
WHERE NOT EXISTS (SELECT 1 FROM schedules WHERE action = 'publish_tiktok');
