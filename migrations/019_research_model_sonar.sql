-- Migration 019: Research agent must use a model with native web search.
-- deepseek (copied from question agent in 018) cannot search the web —
-- verified: OpenRouter :online/web plugin returns no results for it.
-- perplexity/sonar has built-in web search and works with Thai queries.
UPDATE agent_configs SET model = 'perplexity/sonar' WHERE agent_name = 'research';
