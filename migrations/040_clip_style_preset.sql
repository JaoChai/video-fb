-- 040_clip_style_preset.sql
-- Records which style preset (see internal/producer/presets.go) produced a clip:
-- used for trace/debug and to let the scheduler avoid repeating the previous
-- clip's look. NULL/'' = legacy clips produced before presets existed.
ALTER TABLE clips ADD COLUMN IF NOT EXISTS style_preset TEXT NOT NULL DEFAULT '';
