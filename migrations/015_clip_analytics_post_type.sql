-- 015_clip_analytics_post_type.sql
-- แยก analytics ของคลิปยาว (regular) กับ shorts
-- ก่อนหน้านี้ row regular + shorts ใช้คีย์ (clip_id, platform) เหมือนกัน → ทับกัน

ALTER TABLE clip_analytics
    ADD COLUMN IF NOT EXISTS post_type TEXT NOT NULL DEFAULT 'regular';

-- ลบของเก่าทิ้งเพื่อให้รอบ fetch ถัดไป backfill ใหม่ด้วย post_type ที่ถูก
-- (ของเก่ารวม regular+shorts ทับกันอยู่แล้ว — เก็บไว้ก็ตีความไม่ได้)
DELETE FROM clip_analytics WHERE post_type = 'regular';

CREATE INDEX IF NOT EXISTS idx_clip_analytics_lookup
    ON clip_analytics (clip_id, platform, post_type, fetched_at DESC);
