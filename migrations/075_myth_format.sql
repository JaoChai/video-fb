-- 075: format "จับความเชื่อผิด" (spec 2026-07-30) — คลิปที่ 5 ของวัน ช่อง 09:00
--
-- โครงเดียวกับ tutorial/basic: content_format ที่ปิดไว้ (ไม่ให้ตัวสุ่ม format ของ
-- รอบ 12:00/18:00 หยิบไปใช้) + คลังหัวข้อใน DB + schedule/action ของตัวเอง
--
-- เหตุที่ต้องมีคลัง ไม่ปล่อยให้ agent คิดความเชื่อ+ข้อเท็จจริงเอง: คลิปแนวนี้ผิด
-- ไม่ได้ ถ้าโมเดลเดาข้อเท็จจริง คลิปจะกลายเป็นตัวสร้างความเชื่อผิดใหม่เสียเอง
--
-- 6 แถวที่ needs_verify=FALSE คือแถวที่มีแหล่งอ้างจริงแล้ว · อีก 6 แถวลง
-- needs_verify=TRUE เพราะยังหาเอกสารยืนยันไม่ได้ (เช่น trust tier ที่แหล่งเป็น
-- reseller ไม่ใช่ Meta) — ตัวเลือกใน repository ข้ามแถว needs_verify ทั้งหมด
--
-- RunMigrations ไม่หุ้ม transaction ให้ — ต้อง BEGIN/COMMIT เอง
-- idempotent: CREATE TABLE IF NOT EXISTS + ADD COLUMN IF NOT EXISTS +
-- ON CONFLICT DO NOTHING + WHERE NOT EXISTS ทุกคำสั่ง

BEGIN;

CREATE TABLE IF NOT EXISTS myth_beliefs (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    belief_key        TEXT NOT NULL UNIQUE,
    belief_th         TEXT NOT NULL,
    why_believed_th   TEXT NOT NULL,
    verdict           TEXT NOT NULL CHECK (verdict IN ('false','half_true','outdated')),
    fact_th           TEXT NOT NULL,
    source_label      TEXT NOT NULL,
    source_url        TEXT NOT NULL DEFAULT '',
    nuance_th         TEXT NOT NULL,
    cost_th           TEXT NOT NULL,
    audience          TEXT NOT NULL DEFAULT 'account-buyer',
    needs_verify      BOOLEAN NOT NULL DEFAULT FALSE,
    verify_reason     TEXT NOT NULL DEFAULT '',
    last_verified_at  TIMESTAMPTZ,
    used_count        INTEGER NOT NULL DEFAULT 0,
    last_used_at      TIMESTAMPTZ,
    parked_until      TIMESTAMPTZ,
    weight            INTEGER NOT NULL DEFAULT 1,
    enabled           BOOLEAN NOT NULL DEFAULT TRUE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- คลิปต้องจำแถวคลังของตัวเองไว้ ไม่งั้น retry เต็มรูปแบบจะโหลดข้อเท็จจริงกลับไม่ได้
-- แล้วตะแกรงจะปิดเงียบทั้งรอบ (บั๊กเดียวกับที่เคยเกิดกับคลิป basic ตอน retryFull)
ALTER TABLE clips ADD COLUMN IF NOT EXISTS myth_belief TEXT NOT NULL DEFAULT '';

-- question_instruction เป็น NOT NULL ไม่มี default — ลืมใส่แล้ว migration ล้มทั้งไฟล์
INSERT INTO content_formats (format_name, display_name, question_instruction, script_instruction, enabled, weight)
VALUES ('myth', 'จับความเชื่อผิด',
        'หัวข้อมาจาก catalog myth_beliefs เท่านั้น — agent ไม่ต้องคิดหัวข้อเอง และห้ามเพิ่มข้อเท็จจริงที่ไม่อยู่ในแถวคลัง',
        'เขียนสคริปต์แบบพิสูจน์ด้วยหลักฐาน: เปิดด้วยความเสียหายของการเชื่อผิด -> ยกคำเชื่อขึ้นมาตรงๆ พร้อมบอกว่าทำไมคนถึงเชื่อ -> ตัดสินด้วยข้อเท็จจริงจากแหล่งอ้าง -> บอกส่วนที่จริงของความเชื่อนั้น -> สรุปสิ่งที่ควรทำแทน',
        FALSE, 1)
ON CONFLICT (format_name) DO NOTHING;

COMMIT;
