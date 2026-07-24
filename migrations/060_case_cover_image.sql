-- 060: case-file visual full-frame (spec docs/superpowers/specs/2026-07-24-case-visual-fullframe-design.md)
-- แก้กฎภาพของ scene_case: เพิ่มภาพปกซีน casefile (ฉากโต๊ะนักสืบ + ของกลางของเคส) — งบภาพรวมยัง 2 ใบ/คลิป
-- และเพิ่ม kicker เข้า schema ของ comic. REPLACE ทั้งคู่ idempotent: apply ซ้ำแล้วหาข้อความเดิมไม่เจอ = no-op
BEGIN;

UPDATE agent_configs SET prompt_template = REPLACE(prompt_template,
$old$กฎภาพ (สำคัญมาก): image_prompt ใส่ได้เฉพาะซีน layout "evidence" และรวมทั้งคลิปไม่เกิน 2 ซีน — บรรยายภาษาอังกฤษ เฉพาะ "วัตถุหลักฐานชิ้นเดียว วางกลางเฟรม" (เช่น a cream jar, a smartphone with a dark screen) ห้ามระบุสไตล์ศิลป์/สี/ตัวอักษร/โลโก้. ซีนอื่นทุกซีน image_prompt = ""$old$,
$new$กฎภาพ (สำคัญมาก): ใส่ image_prompt ได้เฉพาะ 2 ซีนเท่านั้น
(1) ซีนแรก layout "casefile" ต้องมี image_prompt เสมอ: บรรยายภาษาอังกฤษเป็นฉากโต๊ะทำงานตอนกลางคืนถ่ายมุมสูง มี "ของกลางของเคสนี้" วางเด่น 1 ชิ้น (เช่น a designer handbag on a dark desk under a lamp, a smartphone glowing among scattered papers) — เลือกวัตถุให้ตรงกับสินค้า/สถานการณ์ในเคสจริง
(2) ซีน layout "evidence" 1 ซีน: วัตถุหลักฐานชิ้นเดียววางกลางเฟรม (เช่น a cream jar)
ห้ามระบุสไตล์ศิลป์/สี/ตัวอักษร/โลโก้ในทุก image_prompt. ซีนอื่นนอกจากสองซีนนี้ image_prompt = ""$new$)
WHERE agent_name = 'scene_case'
  AND prompt_template LIKE '%ใส่ได้เฉพาะซีน layout "evidence"%';

UPDATE agent_configs SET prompt_template = REPLACE(prompt_template,
$oldc$- comic: {"panels":[{"time":"วันที่ 1","t":"เปิดแอด งบ 15,000","quote":"ใบแรกต้องรุ่งแน่"},{"time":"คืนวันที่ 2","t":"บัญชีถูกปิด","dark":true}]}$oldc$,
$newc$- comic: {"kicker":"หัวเรื่องสั้นของเหตุการณ์","panels":[{"time":"วันที่ 1","t":"เปิดแอด งบ 15,000","quote":"ใบแรกต้องรุ่งแน่"},{"time":"คืนวันที่ 2","t":"บัญชีถูกปิด","dark":true}]}$newc$)
WHERE agent_name = 'scene_case'
  AND prompt_template NOT LIKE '%"kicker":"หัวเรื่องสั้นของเหตุการณ์"%';

COMMIT;
