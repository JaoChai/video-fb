CREATE TABLE IF NOT EXISTS agent_prompt_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_name TEXT NOT NULL,
    old_prompt TEXT NOT NULL,
    new_prompt TEXT NOT NULL,
    reason TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DELETE FROM schedules;
INSERT INTO schedules (name, cron_expression, action, enabled) VALUES
    ('Daily Publish', '30 23 * * *', 'publish_daily', TRUE),
    ('Weekly Self-Improve', '0 3 * * 1', 'analyze_and_improve', TRUE);
