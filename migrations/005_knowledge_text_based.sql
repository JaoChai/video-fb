-- Make knowledge sources text-based instead of URL-based
ALTER TABLE knowledge_sources ALTER COLUMN url DROP NOT NULL;
ALTER TABLE knowledge_sources ALTER COLUMN url SET DEFAULT '';
ALTER TABLE knowledge_sources ADD COLUMN IF NOT EXISTS content TEXT NOT NULL DEFAULT '';
ALTER TABLE knowledge_sources ADD COLUMN IF NOT EXISTS category TEXT NOT NULL DEFAULT 'general';

-- Clear old URL-based data
DELETE FROM knowledge_chunks;
DELETE FROM knowledge_sources;
