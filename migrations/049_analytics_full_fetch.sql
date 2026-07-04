-- 049: store everything Zernio actually provides + publish status.
-- Additive only. Rollback: revert the code; these columns/tables are inert without it.

ALTER TABLE clip_analytics ADD COLUMN IF NOT EXISTS engagement_rate FLOAT NOT NULL DEFAULT 0;
ALTER TABLE clip_analytics ADD COLUMN IF NOT EXISTS avg_view_percentage FLOAT NOT NULL DEFAULT 0;
ALTER TABLE clip_analytics ADD COLUMN IF NOT EXISTS subscribers_gained INT NOT NULL DEFAULT 0;
ALTER TABLE clip_analytics ADD COLUMN IF NOT EXISTS subscribers_lost INT NOT NULL DEFAULT 0;

-- Per-day YouTube analytics (Zernio daily-views endpoint). TikTok has no daily
-- endpoint; its trend is derived from successive clip_analytics snapshots instead.
CREATE TABLE IF NOT EXISTS clip_analytics_daily (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clip_id UUID NOT NULL REFERENCES clips(id) ON DELETE CASCADE,
    platform TEXT NOT NULL,
    post_type TEXT NOT NULL DEFAULT 'shorts',
    date DATE NOT NULL,
    views INT NOT NULL DEFAULT 0,
    estimated_minutes_watched FLOAT NOT NULL DEFAULT 0,
    average_view_duration FLOAT NOT NULL DEFAULT 0,
    avg_view_percentage FLOAT NOT NULL DEFAULT 0, -- fraction: 0.83 = 83%
    subscribers_gained INT NOT NULL DEFAULT 0,
    subscribers_lost INT NOT NULL DEFAULT 0,
    likes INT NOT NULL DEFAULT 0,
    comments INT NOT NULL DEFAULT 0,
    shares INT NOT NULL DEFAULT 0,
    UNIQUE (clip_id, platform, post_type, date)
);
CREATE INDEX IF NOT EXISTS idx_clip_analytics_daily_clip ON clip_analytics_daily (clip_id, date DESC);

-- Publish outcome per Zernio post, refreshed on every analytics fetch.
-- status='failed' rows are excluded from all learning queries.
CREATE TABLE IF NOT EXISTS clip_publish_status (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clip_id UUID NOT NULL REFERENCES clips(id) ON DELETE CASCADE,
    platform TEXT NOT NULL,
    post_type TEXT NOT NULL DEFAULT 'regular',
    zernio_post_id TEXT NOT NULL,
    status TEXT NOT NULL, -- published | failed | scheduled | unknown
    error_message TEXT,
    checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (clip_id, platform, post_type)
);
CREATE INDEX IF NOT EXISTS idx_clip_publish_status_failed ON clip_publish_status (status) WHERE status = 'failed';

-- Kill switch for feeding topic performance into category pick + question prompt.
INSERT INTO settings (key, value) VALUES ('topic_stats_enabled', 'true')
ON CONFLICT (key) DO NOTHING;
