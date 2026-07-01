-- Migration 029: Move all chat agents onto kie.ai models.
-- KieLLMClient routes by model-name prefix: claude-* -> kie.ai Claude endpoint,
-- gemini-* -> kie.ai Gemini endpoint. Research uses Gemini googleSearch grounding
-- (replaces perplexity/sonar). Idempotent: re-running just re-sets the same values.

UPDATE agent_configs SET model = 'gemini-3-5-flash' WHERE agent_name = 'research';
UPDATE agent_configs SET model = 'claude-sonnet-5' WHERE agent_name = 'script';
UPDATE agent_configs SET model = 'gemini-3-5-flash' WHERE agent_name = 'image';
UPDATE agent_configs SET model = 'gemini-3-5-flash' WHERE agent_name = 'question';
UPDATE agent_configs SET model = 'gemini-3-5-flash' WHERE agent_name = 'analytics';
