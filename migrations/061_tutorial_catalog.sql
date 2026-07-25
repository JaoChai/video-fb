-- 061: tutorial catalog (spec docs/superpowers/specs/2026-07-25-fb-tutorial-content-design.md)
-- แหล่งความจริงเดียวของขั้นตอน + ชื่อเมนูที่อนุญาตให้ขึ้นจอ
-- RunMigrations ไม่หุ้ม transaction ให้ ต้อง BEGIN/COMMIT เอง
-- Rollback: DROP TABLE tutorial_features; ALTER TABLE clips DROP COLUMN tutorial_feature;
BEGIN;

CREATE TABLE IF NOT EXISTS tutorial_features (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    feature_key      TEXT UNIQUE NOT NULL,
    display_name_th  TEXT NOT NULL,
    surface          TEXT NOT NULL,
    menu_path        TEXT[] NOT NULL DEFAULT '{}',
    ui_vocab         TEXT[] NOT NULL DEFAULT '{}',
    steps            JSONB  NOT NULL DEFAULT '[]',
    trap_th          TEXT   NOT NULL DEFAULT '',
    pain_point       TEXT   NOT NULL DEFAULT '',
    why_matters_th   TEXT   NOT NULL DEFAULT '',
    needs_verify     BOOLEAN NOT NULL DEFAULT FALSE,
    verify_reason    TEXT   NOT NULL DEFAULT '',
    last_verified_at TIMESTAMPTZ,
    used_count       INTEGER NOT NULL DEFAULT 0,
    last_used_at     TIMESTAMPTZ,
    weight           INTEGER NOT NULL DEFAULT 1,
    enabled          BOOLEAN NOT NULL DEFAULT TRUE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ผูกคลิปกับฟีเจอร์ เพื่อให้ retry เอา ui_vocab เดิมมา validate ได้ และดู analytics รายฟีเจอร์ได้
ALTER TABLE clips ADD COLUMN IF NOT EXISTS tutorial_feature TEXT NOT NULL DEFAULT '';

COMMIT;
