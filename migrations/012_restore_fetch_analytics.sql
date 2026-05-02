INSERT INTO schedules (name, cron_expression, action, enabled)
SELECT 'Weekly Analytics', '0 4 * * 0', 'fetch_analytics', TRUE
WHERE NOT EXISTS (SELECT 1 FROM schedules WHERE action = 'fetch_analytics');
