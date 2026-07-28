-- 069: พักฟีเจอร์ tutorial ต้องโดนตัดสิน "เมนูย้าย" ซ้ำ 2 ครั้งถึงจะพักจริง
--
-- บั๊กที่แก้: คำตัดสิน "เมนูย้าย" จาก LLM แม่นไม่พอที่จะพักหัวข้อตั้งแต่ครั้งแรก
-- ข้อมูลจริง: พัก 7 ครั้งจาก ~5 รอบผลิต — verdict เดียวพักได้ทันที ทำให้คลังยุบเร็ว
-- เกินจริง (เห็นผลใน migration 067/068 ที่ต้องคอยเติมคลังตาม)
--
-- ทางแก้: flagged_at = เวลาที่โดนตัดสินครั้งแรก ต้องโดนธงซ้ำอีกครั้งภายในหน้าต่าง
-- เดิม (tutorialStrikeWindowDays = 30 วัน) ถึงจะพักจริง (tutorialParkDays = 14 วัน)
-- ตรรกะ 2 statement อยู่ใน Park (internal/repository/tutorial_features.go)
--
-- RunMigrations ไม่หุ้ม transaction ให้ ต้อง BEGIN/COMMIT เอง
-- Rollback: ALTER TABLE tutorial_features DROP COLUMN flagged_at;
BEGIN;

ALTER TABLE tutorial_features ADD COLUMN IF NOT EXISTS flagged_at TIMESTAMPTZ;

COMMIT;
