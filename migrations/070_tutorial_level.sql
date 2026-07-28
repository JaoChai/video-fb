-- 070: แยกคลังหัวข้อเป็น 2 ระดับ — คลิป 21:00 (advanced) กับคลิป 15:00 (basic)
-- spec: docs/superpowers/specs/2026-07-28-basic-tutorial-slot-design.md
--
-- ถ้าใช้คลังเดียวกัน คลิป 21:00 ที่ทำให้คนยิงหนักจะสุ่มได้ "CPM คืออะไร"
-- ซึ่งทำลายจุดขายของช่วงนั้น การแยกด้วย level ทำให้แต่ละช่วงมีคิวหมุนของตัวเอง
-- และมีพื้นกันคลังยุบของตัวเอง โดยไม่ต้องแก้ตรรกะการเลือกหัวข้อเลย
--
-- แถวเดิมทั้งหมดเป็น advanced ด้วย DEFAULT — ไม่ต้อง backfill แยก
-- RunMigrations ไม่หุ้ม transaction ให้ ต้อง BEGIN/COMMIT เอง
-- Rollback: ALTER TABLE tutorial_features DROP COLUMN level;
BEGIN;

ALTER TABLE tutorial_features
  ADD COLUMN IF NOT EXISTS level TEXT NOT NULL DEFAULT 'advanced';

-- level ที่สะกดผิด ('Basic'/'beginner') ไม่ทำให้อะไรพัง แต่แถวนั้นจะหายไปจากทั้ง
-- สองคิว (PickNext กรองด้วย level = $1 เป๊ะๆ) แบบเงียบสนิท — CHECK ทำให้คนที่
-- แก้แถวด้วยมือเจอ error ทันทีแทนที่จะมารู้ตอนคลังยุบ
-- drop-then-add เพราะ Postgres ไม่มี ADD CONSTRAINT IF NOT EXISTS และทั้งคู่อยู่ใน
-- transaction เดียวกัน จึงรันซ้ำได้โดยไม่มีจังหวะที่ตารางไม่มี constraint
ALTER TABLE tutorial_features
  DROP CONSTRAINT IF EXISTS tutorial_features_level_check;

ALTER TABLE tutorial_features
  ADD CONSTRAINT tutorial_features_level_check
  CHECK (level IN ('advanced', 'basic'));

COMMIT;
