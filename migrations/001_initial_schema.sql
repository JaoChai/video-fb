CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "vector";

CREATE TABLE IF NOT EXISTS clips (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title TEXT NOT NULL,
    question TEXT NOT NULL,
    questioner_name TEXT NOT NULL,
    answer_script TEXT NOT NULL DEFAULT '',
    voice_script TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL DEFAULT 'account',
    status TEXT NOT NULL DEFAULT 'draft',
    video_16_9_url TEXT,
    video_9_16_url TEXT,
    thumbnail_url TEXT,
    publish_date DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS scenes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clip_id UUID NOT NULL REFERENCES clips(id) ON DELETE CASCADE,
    scene_number INT NOT NULL,
    scene_type TEXT NOT NULL,
    text_content TEXT NOT NULL,
    image_prompt TEXT NOT NULL DEFAULT '',
    image_16_9_url TEXT,
    image_9_16_url TEXT,
    voice_text TEXT NOT NULL DEFAULT '',
    duration_seconds FLOAT NOT NULL DEFAULT 10,
    text_overlays JSONB NOT NULL DEFAULT '[]'
);

CREATE TABLE IF NOT EXISTS clip_metadata (
    clip_id UUID PRIMARY KEY REFERENCES clips(id) ON DELETE CASCADE,
    youtube_title TEXT,
    youtube_description TEXT,
    youtube_tags TEXT[],
    zernio_post_id TEXT,
    youtube_video_id TEXT,
    tiktok_post_id TEXT,
    ig_post_id TEXT,
    fb_post_id TEXT
);

CREATE TABLE IF NOT EXISTS clip_analytics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clip_id UUID NOT NULL REFERENCES clips(id) ON DELETE CASCADE,
    platform TEXT NOT NULL,
    views INT NOT NULL DEFAULT 0,
    likes INT NOT NULL DEFAULT 0,
    comments INT NOT NULL DEFAULT 0,
    shares INT NOT NULL DEFAULT 0,
    watch_time_seconds FLOAT NOT NULL DEFAULT 0,
    retention_rate FLOAT NOT NULL DEFAULT 0,
    fetched_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS knowledge_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    source_type TEXT NOT NULL,
    crawl_frequency TEXT NOT NULL DEFAULT 'weekly',
    last_crawled_at TIMESTAMPTZ,
    enabled BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS knowledge_chunks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id UUID NOT NULL REFERENCES knowledge_sources(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    embedding VECTOR(1536),
    metadata JSONB NOT NULL DEFAULT '{}',
    url TEXT,
    crawled_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_embedding
    ON knowledge_chunks USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

CREATE TABLE IF NOT EXISTS topic_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title TEXT NOT NULL,
    category TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS agent_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_name TEXT UNIQUE NOT NULL,
    system_prompt TEXT NOT NULL,
    model TEXT NOT NULL DEFAULT 'claude-sonnet-4-6-20250514',
    temperature FLOAT NOT NULL DEFAULT 0.7,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    config JSONB NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    cron_expression TEXT NOT NULL,
    action TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    last_run_at TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS brand_themes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL DEFAULT 'default',
    primary_color TEXT NOT NULL DEFAULT '#1a3a8f',
    secondary_color TEXT NOT NULL DEFAULT '#ffffff',
    accent_color TEXT NOT NULL DEFAULT '#f5851f',
    font_name TEXT NOT NULL DEFAULT 'Noto Sans Thai',
    logo_url TEXT,
    mascot_description TEXT DEFAULT 'Leopard astronaut riding a rocket, holding phone with Facebook icon',
    image_style TEXT DEFAULT 'Modern flat design, dark gradient background, energetic, tech-savvy',
    active BOOLEAN NOT NULL DEFAULT TRUE
);

INSERT INTO agent_configs (agent_name, system_prompt, model) VALUES
    ('question', 'You generate realistic customer questions about Facebook Ads problems in Thai.', 'claude-sonnet-4-6-20250514'),
    ('script', 'You write Q&A video scripts answering Facebook Ads questions in Thai.', 'claude-sonnet-4-6-20250514'),
    ('image', 'You generate image prompts for video scenes matching brand theme.', 'claude-sonnet-4-6-20250514'),
    ('analytics', 'You analyze video performance metrics and recommend improvements.', 'claude-sonnet-4-6-20250514')
ON CONFLICT (agent_name) DO NOTHING;

INSERT INTO brand_themes (name, primary_color, secondary_color, accent_color, active)
SELECT 'Ads Vance Default', '#1a3a8f', '#ffffff', '#f5851f', TRUE
WHERE NOT EXISTS (SELECT 1 FROM brand_themes WHERE name = 'Ads Vance Default');

INSERT INTO schedules (name, cron_expression, action)
SELECT * FROM (VALUES
    ('Weekly Production', '0 3 * * 1', 'produce_weekly'),
    ('Daily Publish', '0 17 * * *', 'publish_daily'),
    ('Weekly Knowledge Crawl', '0 2 * * 0', 'crawl_knowledge'),
    ('Weekly Analytics', '0 4 * * 0', 'fetch_analytics')
) AS v(name, cron_expression, action)
WHERE NOT EXISTS (SELECT 1 FROM schedules WHERE name = v.name);

INSERT INTO knowledge_sources (name, url, source_type, crawl_frequency)
SELECT * FROM (VALUES
    ('Meta Business Help Center', 'https://business.facebook.com/help', 'official', 'weekly'),
    ('Facebook Ads Policies', 'https://facebook.com/policies/ads', 'official', 'weekly'),
    ('Jon Loomer Digital', 'https://jonloomer.com', 'practitioner', 'weekly'),
    ('AdEspresso Blog', 'https://adespresso.com/blog', 'practitioner', 'weekly'),
    ('Social Media Examiner', 'https://socialmediaexaminer.com', 'practitioner', 'weekly'),
    ('r/FacebookAds', 'https://reddit.com/r/FacebookAds', 'community', 'daily'),
    ('r/PPC', 'https://reddit.com/r/PPC', 'community', 'daily')
) AS v(name, url, source_type, crawl_frequency)
WHERE NOT EXISTS (SELECT 1 FROM knowledge_sources WHERE name = v.name);
