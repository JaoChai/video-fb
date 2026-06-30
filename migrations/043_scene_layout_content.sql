-- 043_scene_layout_content.sql
-- Persist the Style-B structured content (layout + content) so a resumed clip
-- re-renders identical visuals instead of a blank CSS-only video. Without these,
-- scenesToGenerated could not reconstruct the renderer's primary input.
-- Rollback: ALTER TABLE scenes DROP COLUMN IF EXISTS layout; ALTER TABLE scenes DROP COLUMN IF EXISTS content;
ALTER TABLE scenes ADD COLUMN IF NOT EXISTS layout TEXT NOT NULL DEFAULT '';
ALTER TABLE scenes ADD COLUMN IF NOT EXISTS content JSONB;
