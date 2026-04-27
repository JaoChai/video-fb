-- Remove old daily publish (23:30) — replaced by noon + midnight produce_and_publish
DELETE FROM schedules WHERE action = 'publish_daily';

-- Add dual daily schedule: produce 1 clip + publish as YouTube private
INSERT INTO schedules (name, cron_expression, action, enabled) VALUES
    ('Noon Produce & Publish', '0 12 * * *', 'produce_and_publish', TRUE),
    ('Midnight Produce & Publish', '0 0 * * *', 'produce_and_publish', TRUE);
