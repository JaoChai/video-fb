-- 067: ปลดประตูทางเดียวของ needs_verify + ผูกฟีเจอร์กับกลุ่มเป้าหมาย
--
-- บั๊กที่แก้: needs_verify ถูกตั้ง TRUE ได้อย่างเดียว ไม่มีโค้ด/API/หน้าจอไหนตั้งกลับ
-- เป็น FALSE เลย ภายใน 2 วันหลังเปิด schedule 21:00 คลังโดนพัก 7 จาก 8 แถว เหลือ
-- ผ่านตัวกรองตัวเดียว (audience_overlap_check) → คลิปสอนซ้ำหัวข้อเดิมทุกวัน
-- และถ้าแถวสุดท้ายโดนพักด้วยจะไม่มีคลิป 21:00 เลย
--
-- ทางแก้: parked_until = การพักมีวันหมดอายุ (14 วัน ตั้งใน tutorialParkDays)
-- และ Park นับคลังในคำสั่งเดียวกัน ไม่ยอมพักถ้าเหลือไม่ถึง TutorialMinPool
--
-- คอลัมน์ needs_verify กลายเป็นของเหลือ: โค้ดไม่อ่านและไม่เขียนอีกแล้ว ไฟล์นี้เซ็ต
-- ให้เป็น FALSE ทั้งตารางเป็นครั้งสุดท้าย แล้วปล่อยคอลัมน์ไว้เฉยๆ เผื่อ rollback
-- (verify_reason ยังใช้อยู่ = บันทึกว่าถูกพักเพราะอะไร)
--
-- RunMigrations ไม่หุ้ม transaction ให้ ต้อง BEGIN/COMMIT เอง
-- Rollback: ALTER TABLE tutorial_features DROP COLUMN parked_until, DROP COLUMN audience;
BEGIN;

ALTER TABLE tutorial_features ADD COLUMN IF NOT EXISTS parked_until TIMESTAMPTZ;

-- กลุ่มเป้าหมายของฟีเจอร์ = ค่าเดียวกับ topic_categories.category_name (migration 057)
-- ก่อนหน้านี้คลิป tutorial ถูกฮาร์ดโค้ดเป็น grey-operator ทุกใบ ทำให้ปกคลิปกับ
-- metadata ถูกเขียนด้วยมุมผิดกลุ่มเวลาหัวข้อเป็นสาย performance
ALTER TABLE tutorial_features ADD COLUMN IF NOT EXISTS audience TEXT NOT NULL DEFAULT 'grey-operator';

UPDATE tutorial_features SET audience = 'account-buyer'
WHERE feature_key IN ('backup_payment_method', 'backup_admin_2fa', 'account_quality_check');

UPDATE tutorial_features SET audience = 'performance-advertiser'
WHERE feature_key = 'audience_overlap_check';

-- ปลด 7 แถวที่ค้างอยู่ตอนนี้ให้กลับเข้าคิวทันที เก็บ verify_reason ไว้เป็นบันทึก
UPDATE tutorial_features SET needs_verify = FALSE, parked_until = NULL WHERE needs_verify;

COMMIT;
