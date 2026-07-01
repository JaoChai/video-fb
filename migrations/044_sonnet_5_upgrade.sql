-- Migration 044: Upgrade all Claude Sonnet agents from sonnet-4-6 to sonnet-5.
-- kie.ai model id per https://docs.kie.ai/market/claude/cluade-sonnet-5 is
-- 'claude-sonnet-5'. KieLLMClient routes by the claude-* prefix, so the endpoint
-- is unchanged. Covers both the bare 'claude-sonnet-4-6' and the legacy
-- '...-20250514' variant seeded in migration 001. Idempotent: re-running is a no-op.

UPDATE agent_configs
SET model = 'claude-sonnet-5'
WHERE model IN ('claude-sonnet-4-6', 'claude-sonnet-4-6-20250514');
