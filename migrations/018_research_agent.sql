-- Migration 018: Research agent replaces KB URL-crawl pipeline
-- Web knowledge now comes from live web search at generation time (OpenRouter :online).
-- Hand-written Thai business knowledge (text sources) stays in the KB unchanged.

-- 1. Seed research agent config (5th agent: question, script, image, analytics, research)
INSERT INTO agent_configs (agent_name, system_prompt, model, temperature, enabled)
SELECT 'research',
       'You are a research assistant for a Thai Facebook Ads content channel. You find recent, reliable information about Facebook Ads / Meta platform changes that affect Thai advertisers. You only cite trustworthy sources (Meta official announcements, major industry publications, Thai government agencies). You never fabricate facts — if you cannot find reliable information, you say so. You respond in Thai.',
       model, 0.3, TRUE
FROM agent_configs WHERE agent_name = 'question'
ON CONFLICT (agent_name) DO NOTHING;

-- 2. Remove URL-based knowledge sources (never worked: blocked sites, OOM bugs, 0 chunks produced)
--    Their chunks cascade-delete via FK. Hand-written text sources (url = '') are untouched.
DELETE FROM knowledge_sources WHERE COALESCE(url, '') != '';

-- 3. Remove the crawl schedule — no crawler anymore
DELETE FROM schedules WHERE action = 'crawl_knowledge';
