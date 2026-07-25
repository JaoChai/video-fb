-- 064: tutorial content format row + ย้ายสล็อตผลิต 06:00 -> 21:00
--
-- content_formats.tutorial ต้อง enabled = FALSE เพื่อไม่ให้ FormatsRepo.PickNext
-- (WHERE cf.enabled = TRUE) หยิบไปใช้ในรอบผลิตปกติ 12:00/18:00 — ProduceTutorial
-- ดึงแถวนี้ด้วย GetByName ซึ่งไม่กรอง enabled
--
-- RunMigrations ไม่หุ้ม transaction ให้ ต้อง BEGIN/COMMIT เอง
--
-- Rollback (คืนสล็อตเช้า):
--   UPDATE schedules SET name='Morning Produce & Publish', cron_expression='0 6 * * *',
--          action='produce_and_publish' WHERE action='produce_tutorial';
BEGIN;

INSERT INTO content_formats (format_name, display_name, question_instruction, script_instruction, enabled, weight)
VALUES ('tutorial', 'สอนใช้ฟีเจอร์จริง',
 $$หัวข้อมาจาก catalog tutorial_features เท่านั้น — agent ไม่ต้องคิดหัวข้อเอง$$,
 $$เขียนสคริปต์แบบคู่มือ: เปิดด้วยความเสียหาย -> สัญญา+จำนวนขั้น -> เดินขั้นตอนตามข้อมูลฟีเจอร์ -> กับดัก -> การ์ดสรุป -> ปิดแบบชวนคุย$$,
 FALSE, 1)
ON CONFLICT (format_name) DO NOTHING;

UPDATE schedules
SET name = 'Tutorial Produce & Publish',
    cron_expression = '0 21 * * *',
    action = 'produce_tutorial'
WHERE action = 'produce_and_publish' AND cron_expression = '0 6 * * *';

COMMIT;
