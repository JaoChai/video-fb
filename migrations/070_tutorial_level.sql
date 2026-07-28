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

COMMIT;
