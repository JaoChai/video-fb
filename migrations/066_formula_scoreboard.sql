-- 066: กระดานคะแนนสูตร + audit การหมุนน้ำหนัก
--
-- formula_scores เก็บเป็น snapshot (ไม่ใช่ view) เพื่อให้ตอบได้เสมอว่า weight
-- ของสัปดาห์หนึ่งมาจากคะแนนชุดไหน weight_revisions เป็น append-only เหมือน
-- skill_revisions — ไม่มี UPDATE/DELETE เพื่อให้ rollback ได้ตลอดเวลา
--
-- weight ถูกปรับเป็นสเกลผลรวม 100 ต่อมิติ (1 หน่วย = 1%) FormatsRepo.PickNext
-- และ TopicCategoriesRepo.PickNextExclude เทียบ used/weight ภายในตารางเดียวกัน
-- การเปลี่ยนสเกลจึงไม่กระทบพฤติกรรมการเลือก
--
-- RunMigrations ไม่หุ้ม transaction ให้ ต้อง BEGIN/COMMIT เอง
--
-- Rollback:
--   UPDATE settings SET value='false' WHERE key='weight_tuner_enabled';
--   UPDATE schedules SET enabled=FALSE WHERE action='tune_weights';
--   (ตาราง formula_scores / weight_revisions ทิ้งไว้ได้ ไม่มีผลข้างเคียง)
BEGIN;

CREATE TABLE IF NOT EXISTS formula_scores (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    computed_at      TIMESTAMPTZ NOT NULL,
    dimension        TEXT NOT NULL,              -- content_format | category | style_preset
    value            TEXT NOT NULL,
    platform         TEXT NOT NULL,              -- youtube | tiktok
    n                INTEGER NOT NULL,
    median_pct       DOUBLE PRECISION NOT NULL,
    median_retention DOUBLE PRECISION NOT NULL DEFAULT 0,  -- 0 = ไม่มีข้อมูล ไม่ใช่แย่
    flop_rate        DOUBLE PRECISION NOT NULL,
    score_raw        DOUBLE PRECISION NOT NULL,
    score_final      DOUBLE PRECISION NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_formula_scores_snapshot
    ON formula_scores (computed_at DESC, dimension, platform);

CREATE TABLE IF NOT EXISTS weight_revisions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dimension   TEXT NOT NULL,
    value       TEXT NOT NULL,
    old_weight  INTEGER NOT NULL,
    new_weight  INTEGER NOT NULL,
    score_final DOUBLE PRECISION NOT NULL,
    n           INTEGER NOT NULL,
    computed_at TIMESTAMPTZ NOT NULL,            -- โยงกลับไป formula_scores.computed_at
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_weight_revisions_recent
    ON weight_revisions (created_at DESC);

-- ปรับ weight ของสูตรที่ enabled ให้เป็นสเกลผลรวม 100 แบบเท่าๆ กัน
-- (แถวที่ enabled = FALSE ไม่ต้องแตะ เพราะ PickNext กรองออกอยู่แล้ว)
-- ตรวจแล้วเมื่อ 2026-07-27: content_formats มี 4 แถว enabled (qa, news, tips,
-- case_story; tutorial disabled) และ topic_categories มี 3 แถว enabled — ถ้า
-- จำนวนนี้เปลี่ยนไปก่อนรัน ให้แก้ตัวเลขให้ผลรวม = 100 ก่อน
UPDATE content_formats SET weight = 25 WHERE enabled = TRUE;
UPDATE topic_categories SET weight = 33 WHERE enabled = TRUE;
UPDATE topic_categories SET weight = 34
WHERE category_name = (SELECT category_name FROM topic_categories WHERE enabled = TRUE ORDER BY category_name LIMIT 1);

INSERT INTO settings (key, value)
VALUES ('weight_tuner_enabled', 'false')
ON CONFLICT (key) DO NOTHING;

INSERT INTO schedules (name, action, cron_expression, enabled)
VALUES ('Weekly Weight Tune', 'tune_weights', '30 3 * * 1', FALSE)
ON CONFLICT DO NOTHING;

COMMIT;
