-- 077: ข้อความคอมเมนต์ปักหมุดใต้คลิป YouTube
--
-- ระบบส่งคลิปผ่าน Zernio ซึ่งรับ platformSpecificData.firstComment แล้วโพสต์คอมเมนต์
-- ใต้คลิปพร้อมปักหมุดให้เอง (เอกสาร: docs.zernio.com/platforms/youtube) เราจึงไม่ต้อง
-- มี OAuth ของ YouTube เอง — และ YouTube Data API เองก็ปักหมุดคอมเมนต์ไม่ได้อยู่แล้ว
--
-- ทำไมต้องอยู่ใน settings ไม่ใช่ค่าคงที่ในโค้ด: ข้อความติดต่อเปลี่ยนบ่อยกว่าโค้ด และ
-- ตั้งค่าว่างคือสวิตช์ปิดฟีเจอร์ทันทีโดยไม่ต้อง deploy (publisher.go ไม่แนบ
-- platformSpecificData เลยเมื่อค่าว่าง = คำขอเหมือนก่อนมีฟีเจอร์นี้)
--
-- ON CONFLICT DO NOTHING เพราะถ้า user แก้ข้อความเองจากหน้าเว็บแล้ว การรัน migration
-- ซ้ำ (deploy ใหม่) ต้องไม่ทับค่าที่แก้ไว้
--
-- ข้อความนี้ต่างจากที่อยู่ใน youtube_description โดยตั้งใจ: บน Shorts คำอธิบายถูกซ่อน
-- ต้องกดถึงเห็น ส่วนคอมเมนต์อยู่บนหน้าจอ

BEGIN;

INSERT INTO settings (key, value) VALUES (
  'youtube_first_comment',
  $txt$สนใจบัญชีโฆษณา / สอบถามเพิ่มเติม
ติดต่อทีมงานได้ที่ LINE id : @adsvance

กลุ่มข่าวสาร Telegram : https://t.me/adsvancech$txt$
)
ON CONFLICT (key) DO NOTHING;

COMMIT;
