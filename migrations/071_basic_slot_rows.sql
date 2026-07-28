-- 071: แถวข้อมูลของช่วงคอนเทนต์พื้นฐาน 15:00
-- spec: docs/superpowers/specs/2026-07-28-basic-tutorial-slot-design.md
--
-- agent row ทั้ง 3 ตัวคัดลอกจากฝั่ง tutorial ด้วย INSERT..SELECT เพื่อให้ทุกอย่าง
-- ที่ไม่ได้ตั้งใจเปลี่ยน (โครงบังคับ 6 ข้อ, กฎ ui_vocab, รูปแบบ JSON ที่ต้องตอบ)
-- เหมือนกันแน่นอน แล้วเปลี่ยนเฉพาะบล็อก "เสียงคนวงใน" ของ script_basic
--
-- ห้าม UPDATE row ฝั่ง tutorial เด็ดขาด — 3 ช่วงที่วิ่งอยู่ต้องไม่กระทบ
-- schedules ไม่มี unique index บน action (มีแค่ PK บน id) → ON CONFLICT กันซ้ำไม่ได้
-- ต้องใช้ WHERE NOT EXISTS ไม่งั้นรันซ้ำจะได้ตารางเวลาซ้อนกันสองแถว
--
-- RunMigrations ไม่หุ้ม transaction ให้ ต้อง BEGIN/COMMIT เอง
-- Rollback: DELETE FROM agent_configs WHERE agent_name IN ('script_basic','scene_basic','critic_basic');
--           DELETE FROM content_formats WHERE format_name='basic';
--           DELETE FROM topic_categories WHERE category_name='beginner';
--           DELETE FROM schedules WHERE action='produce_basic';
BEGIN;

-- 1) content format (ปิดไว้ เพื่อไม่ให้ FormatsRepo.PickNext ของรอบปกติหยิบไปใช้)
INSERT INTO content_formats (format_name, display_name, question_instruction, script_instruction, enabled, weight)
VALUES ('basic', 'สอนพื้นฐานจากหน้าจอ',
        'หัวข้อมาจาก catalog tutorial_features (level = basic) เท่านั้น — agent ไม่ต้องคิดหัวข้อเอง',
        'เขียนสคริปต์แบบคู่มือสำหรับคนเพิ่งเริ่ม: เปิดด้วยความเสียหาย -> สัญญา+จำนวนขั้น -> เดินขั้นตอนตามข้อมูลฟีเจอร์ -> กับดัก -> สรุป โดยนิยามศัพท์อังกฤษทุกคำที่โผล่ครั้งแรก',
        FALSE, 1)
ON CONFLICT (format_name) DO NOTHING;

-- 2) scene_basic / critic_basic = สำเนาตรงของฝั่ง tutorial (หน้าตาคลิปต้องเหมือนกันเป๊ะ)
INSERT INTO agent_configs (agent_name, system_prompt, model, temperature, enabled, config, skills, prompt_template, insights)
SELECT 'scene_basic', system_prompt, model, temperature, enabled, config, skills, prompt_template, insights
FROM agent_configs WHERE agent_name = 'scene_tutorial'
ON CONFLICT (agent_name) DO NOTHING;

INSERT INTO agent_configs (agent_name, system_prompt, model, temperature, enabled, config, skills, prompt_template, insights)
SELECT 'critic_basic', system_prompt, model, temperature, enabled, config, skills, prompt_template, insights
FROM agent_configs WHERE agent_name = 'critic_tutorial'
ON CONFLICT (agent_name) DO NOTHING;

-- 3) script_basic = สำเนาของ script_tutorial ที่สลับบล็อกเสียงเป็นเสียงสำหรับมือใหม่
--    ใช้ DO block เพื่อ "ล้มให้ดัง" ถ้าหาข้อความต้นทางไม่เจอ — replace() ที่ไม่เจอ
--    จะเงียบ แล้วเราจะได้พรอมป์เสียงคนวงในไปสอนมือใหม่โดยไม่มีใครรู้
DO $mig$
DECLARE
  src TEXT;
  anchor TEXT := 'เสียงคนวงใน: พูดเหมือนคนที่บริหารบัญชีจำนวนมากมาเอง ใช้ศัพท์ที่คนวงในใช้ อ้างสถานการณ์ที่เฉพาะคนยิงหนักเจอ ห้ามเนื้อหาระดับพื้นฐาน';
  replacement TEXT := 'เสียงสำหรับคนเพิ่งเริ่ม: คนดูยังไม่เคยยิงแอดมาก่อน ศัพท์อังกฤษทุกคำที่โผล่ครั้งแรกต้องมีคำอธิบายไทยสั้นๆ ต่อท้ายทันที ห้ามสมมติว่าคนดูรู้อยู่แล้ว แต่ห้ามพูดจาดูถูก (ห้ามใช้ "ง่ายมาก" "ใครๆ ก็ทำได้" "แค่นี้เอง") ยกตัวอย่างด้วยงบหลักร้อยถึงหลักพันบาทต่อวันแบบคนเพิ่งเริ่มใช้จริง ไม่ใช่หลักหมื่น';
BEGIN
  IF EXISTS (SELECT 1 FROM agent_configs WHERE agent_name = 'script_basic') THEN
    RETURN; -- idempotent: มีแล้วไม่ต้องทำซ้ำ
  END IF;

  SELECT prompt_template INTO src FROM agent_configs WHERE agent_name = 'script_tutorial';
  IF src IS NULL THEN
    RAISE EXCEPTION 'script_tutorial row missing — cannot derive script_basic';
  END IF;
  IF position(anchor IN src) = 0 THEN
    RAISE EXCEPTION 'voice anchor not found in script_tutorial — update migration 071 before shipping';
  END IF;

  INSERT INTO agent_configs (agent_name, system_prompt, model, temperature, enabled, config, skills, prompt_template, insights)
  SELECT 'script_basic', system_prompt, model, temperature, enabled, config, skills,
         replace(prompt_template, anchor, replacement), insights
  FROM agent_configs WHERE agent_name = 'script_tutorial';
END $mig$;

-- 4) กลุ่มเป้าหมายใหม่ (ปิดไว้ เพื่อไม่ให้รอบ 12:00/18:00 หยิบไปใช้)
INSERT INTO topic_categories (category_name, display_name, angle_instruction, weight, enabled)
VALUES ('beginner', 'คนเพิ่งเริ่มยิงแอด',
$bg$Persona: The First-Timer — คนที่เพิ่งเปิดบัญชีโฆษณา หรือเคยกดโปรโมทโพสต์อย่างเดียว ยังไม่เคยเข้า Ads Manager เต็มรูปแบบ กลัวเสียเงินฟรี ไม่กล้ากดเพราะไม่รู้ว่าปุ่มไหนทำอะไร งบหลักร้อยถึงหลักพันต่อวัน

เมนู pain_point (เลือกค่า pain_point จากนี้เท่านั้น):
- report_columns_meaning: อ่านคอลัมน์ในรายงานไม่ออก ตัวเลขแต่ละตัวแปลว่าอะไร
- campaign_structure_confusion: Campaign กับ Ad set กับ Ad ต่างกันยังไง อะไรอยู่ชั้นไหน
- budget_type_choice: งบรายวันกับงบตลอดอายุแคมเปญ เลือกอันไหน
- objective_mismatch: เลือกวัตถุประสงค์แคมเปญผิด เลยไม่ได้ผลลัพธ์ที่อยากได้
- boost_vs_adsmanager: กดโปรโมทโพสต์กับยิงผ่าน Ads Manager ต่างกันตรงไหน
- ad_not_delivering: ไม่รู้ว่าแอดวิ่งอยู่จริงไหม ดูตรงไหน
- date_range_confusion: เปลี่ยนช่วงวันที่แล้วตัวเลขเปลี่ยน ไม่เข้าใจว่าทำไม
- basic_audience_setup: ตั้งกลุ่มเป้าหมายเบื้องต้นยังไงให้ไม่กว้างเกินไป
- page_account_link: ผูกเพจกับบัญชีโฆษณาไม่เป็น
- first_payment_setup: ตั้งวิธีชำระเงินครั้งแรก
- billing_receipt_reading: ดูใบเสร็จ/ยอดที่ถูกตัดจริงตรงไหน
- breakdown_basics: ดูผลแยกตามอายุ-เพศ ทำยังไง$bg$, 1, FALSE)
ON CONFLICT (category_name) DO NOTHING;

-- 5) ตารางเวลา 15:00 — ปิดไว้ก่อน เปิดหลัง eyeball คลิปแรก
--    GOTCHA: เปิด/ปิดต้อง PATCH ผ่าน API เท่านั้น scheduler อ่าน DB แค่ตอน Start()
INSERT INTO schedules (name, cron_expression, action, enabled)
SELECT 'Basic Produce & Publish', '0 15 * * *', 'produce_basic', FALSE
WHERE NOT EXISTS (SELECT 1 FROM schedules WHERE action = 'produce_basic');

COMMIT;
