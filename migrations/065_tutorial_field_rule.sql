-- 065: scene_tutorial — กฎเรื่อง panel.field (เจอจากคลิปจริงตัวแรกบน prod 2026-07-25)
--
-- คลิปแรก (audience_overlap_check) ขั้นที่ 1 ใส่ field.label = "Selected audience"
-- โดยไม่มี value → เรนเดอร์เป็นกล่องส้มเปล่าที่ซ้ำชื่อกับแถว target ด้านบน สอนอะไรไม่ได้
-- โค้ดกันไว้แล้ว (scene_adapter ทิ้ง field ที่ value ว่าง) migration นี้แก้ที่ต้นทาง
-- เพื่อไม่ให้โมเดลเสีย token กับ field ที่จะถูกทิ้งอยู่ดี
--
-- RunMigrations ไม่หุ้ม transaction ให้ ต้อง BEGIN/COMMIT เอง
-- Rollback: ไม่ต้อง — เป็นการเติมข้อความในกฎ uistep ที่มีอยู่
BEGIN;

UPDATE agent_configs
SET prompt_template = REPLACE(prompt_template,
  'กฎ uistep: state มีแค่ "normal" | "target" | "done"',
  'กฎ panel.field: ใส่ field **เฉพาะเมื่อมีค่าที่คนดูต้องกรอกหรือต้องอ่านจริง** เช่น {"label":"Greater than","value":"400 THB"} — ถ้าขั้นนั้นเป็นแค่การกดเมนู ห้ามใส่ field เลย (ใส่ label โดย value ว่าง = กล่องเปล่าซ้ำชื่อแถวด้านบน ระบบจะทิ้งทั้ง field)

กฎ uistep: state มีแค่ "normal" | "target" | "done"')
WHERE agent_name = 'scene_tutorial'
  AND prompt_template NOT LIKE '%กฎ panel.field%';

COMMIT;
