-- 081: โพสต์คลิปเข้าช่อง Telegram อัตโนมัติหลังขึ้น YouTube
--
-- ระบบโพสต์ผ่าน Zernio เหมือน YouTube/TikTok (แผน Accelerate ที่ใช้อยู่รองรับ platform
-- telegram แล้ว: uploads ไม่จำกัด, profiles 7/50 เมื่อ 2026-08-09) โพสต์ในช่องจะขึ้นเป็น
-- ชื่อและโลโก้ของช่องเอง ไม่ใช่ชื่อบอท
--
-- zernio_telegram_post_id: id ของโพสต์ Telegram ฝั่ง Zernio · NULL = ยังไม่ได้ส่ง
-- ใช้เป็นตัวกันส่งซ้ำและเป็นตัวชี้เป้าของรอบเก็บตก
--
-- ทำไมต้องปั๊ม 'skipped-backfill': รอบเก็บตกไล่คลิปที่ published ภายใน 24 ชม.ที่ยัง
-- ไม่มี id — ถ้าไม่ปั๊มค่าไว้ก่อน คลิป 2-4 ตัวที่เพิ่งขึ้นก่อน deploy จะถูกยิงเข้าช่อง
-- ทันทีที่ deploy ทั้งที่เจ้าของสั่งไว้ว่า "เริ่มนับจากคลิปใหม่เท่านั้น"
--
-- setting ค่าว่าง = ฟีเจอร์ปิดสนิท (โค้ดข้ามทั้งบล็อก ไม่ยิง HTTP เลย) เปิดใช้โดยกรอก
-- account id ที่ได้จาก Zernio หลังผูกช่อง ไม่ต้อง deploy ใหม่
-- ON CONFLICT DO NOTHING เพื่อไม่ให้การรัน migration ซ้ำทับค่าที่กรอกไว้แล้ว

BEGIN;

ALTER TABLE clip_metadata ADD COLUMN IF NOT EXISTS zernio_telegram_post_id TEXT;

UPDATE clip_metadata cm
SET zernio_telegram_post_id = 'skipped-backfill'
FROM clips c
WHERE c.id = cm.clip_id
  AND c.status = 'published'
  AND cm.zernio_telegram_post_id IS NULL;

INSERT INTO settings (key, value) VALUES ('zernio_telegram_account_id', '')
ON CONFLICT (key) DO NOTHING;

COMMIT;
