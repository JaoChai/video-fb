-- 072: คลังหัวข้อพื้นฐาน 12 เรื่อง สำหรับช่วง 15:00 (level = 'basic')
-- spec: docs/superpowers/specs/2026-07-28-basic-tutorial-slot-design.md
--
-- ทุก steps[].ui_target ต้องอยู่ใน ui_vocab ของแถวเดียวกัน (บังคับด้วย TestSeedStepsCoveredByUIVocab)
-- pain_point ทุกค่าต้องอยู่ในเมนูของกลุ่ม beginner ใน migration 071 เท่านั้น
-- trap_th ต้องเป็นจุดที่คนเพิ่งเริ่มเข้าใจผิดจริง ไม่ใช่คำเตือนกว้างๆ
--
-- ⚠ menu_path/ui_vocab เขียนจากความรู้ ยังไม่ได้เทียบหน้าจอจริงทีละหน้า
--   ตัวกรอง needs_verify/two-strike + research agent เป็นตาข่ายรับ แต่ควร eyeball คลิปแรก
-- RunMigrations ไม่หุ้ม transaction ให้ ต้อง BEGIN/COMMIT เอง
-- Rollback: DELETE FROM tutorial_features WHERE level = 'basic';
BEGIN;

INSERT INTO tutorial_features
    (feature_key, display_name_th, surface, audience, level, menu_path, ui_vocab, steps, trap_th, pain_point, why_matters_th)
VALUES
('report_columns_basics', 'อ่านคอลัมน์ในรายงาน ว่าตัวเลขแต่ละตัวแปลว่าอะไร', 'ads_manager', 'beginner', 'basic',
 ARRAY['Ads Manager','Campaigns','Columns'],
 string_to_array($vocab$Ads Manager|Campaigns|Ad sets|Ads|Columns|Performance|Results|Reach|Impressions|Cost per result|Amount spent|Apply$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เปิดหน้ารายการแคมเปญ","action_th":"ใน Ads Manager กดแท็บ Campaigns เพื่อดูรายการแคมเปญทั้งหมด","ui_target":"Campaigns"},
  {"n":2,"title_th":"เลือกชุดคอลัมน์มาตรฐาน","action_th":"กด Columns แล้วเลือก Performance เพื่อให้เห็นตัวเลขชุดพื้นฐาน","ui_target":"Performance"},
  {"n":3,"title_th":"อ่านสามตัวที่ต้องดูก่อนเสมอ","action_th":"ดู Results คือจำนวนผลลัพธ์ที่ได้ Reach คือจำนวนคนที่เห็น Impressions คือจำนวนครั้งที่ถูกแสดง","ui_target":"Results"},
  {"n":4,"title_th":"ดูว่าผลลัพธ์หนึ่งครั้งจ่ายเท่าไหร่","action_th":"ดู Cost per result เทียบกับ Amount spent เพื่อรู้ว่าคุ้มไหม","ui_target":"Cost per result","value_th":"เทียบกับกำไรต่อออเดอร์"}
 ]$steps$,
 'คนใหม่มักดู Impressions แล้วดีใจว่าคนเห็นเยอะ ทั้งที่ Reach คือจำนวนคนจริง ส่วน Impressions นับซ้ำคนเดิมได้หลายครั้ง เห็นคนเดียวสิบรอบก็นับสิบ',
 'report_columns_meaning',
 'อ่านรายงานไม่ออก แปลว่าเผางบต่อไปเรื่อยๆ โดยไม่รู้ว่ากำลังได้หรือกำลังเสีย'),

('breakdown_age_gender', 'ดูผลแยกตามอายุและเพศ ว่าเงินไปตกที่กลุ่มไหน', 'ads_manager', 'beginner', 'basic',
 ARRAY['Ads Manager','Breakdown','By Delivery','Age','Gender'],
 string_to_array($vocab$Ads Manager|Campaigns|Breakdown|By Delivery|Age|Gender|Results|Amount spent|Cost per result$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เปิดเมนูแยกผล","action_th":"ที่แถบเครื่องมือของ Ads Manager กด Breakdown","ui_target":"Breakdown"},
  {"n":2,"title_th":"เลือกหมวดการแสดงผล","action_th":"เลือก By Delivery เพื่อดูว่าโฆษณาไปถึงใครบ้าง","ui_target":"By Delivery"},
  {"n":3,"title_th":"แยกตามอายุ","action_th":"กด Age แล้วดูว่าช่วงอายุไหนใช้ Amount spent ไปเท่าไหร่","ui_target":"Age"},
  {"n":4,"title_th":"เทียบราคาต่อผลลัพธ์ของแต่ละกลุ่ม","action_th":"ดู Cost per result ของแต่ละแถว กลุ่มที่แพงผิดปกติคือกลุ่มที่กินงบเปล่า","ui_target":"Cost per result"}
 ]$steps$,
 'เห็นกลุ่มไหนแพงแล้วรีบตัดทิ้งทันทีตั้งแต่วันแรก ทั้งที่ข้อมูลยังน้อยเกินจะสรุป ควรรอให้แต่ละกลุ่มมีผลลัพธ์หลายครั้งก่อนค่อยตัดสิน',
 'breakdown_basics',
 'ไม่รู้ว่าเงินไหลไปกลุ่มไหน แปลว่าปรับกลุ่มเป้าหมายด้วยการเดาล้วนๆ'),

('delivery_column_check', 'เช็กว่าโฆษณาวิ่งอยู่จริงไหม ดูที่คอลัมน์ Delivery', 'ads_manager', 'beginner', 'basic',
 ARRAY['Ads Manager','Ads','Delivery','Active','In review','Not delivering','Off','See details'],
 string_to_array($vocab$Ads Manager|Campaigns|Ad sets|Ads|Delivery|Active|In review|Not delivering|Off|See details$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เปิดที่ระดับโฆษณา","action_th":"ใน Ads Manager กดแท็บ Ads ไม่ใช่ระดับแคมเปญ เพราะสถานะจริงอยู่ชั้นนี้","ui_target":"Ads"},
  {"n":2,"title_th":"อ่านคอลัมน์สถานะ","action_th":"ดูคอลัมน์ Delivery ถ้าขึ้น Active แปลว่ากำลังวิ่งจริง","ui_target":"Delivery"},
  {"n":3,"title_th":"แยกให้ออกว่าค้างเพราะอะไร","action_th":"ถ้าขึ้น In review คือรอตรวจ ถ้าขึ้น Not delivering คือมีอะไรบล็อกอยู่","ui_target":"Not delivering"},
  {"n":4,"title_th":"เปิดดูเหตุผล","action_th":"ชี้ที่สถานะแล้วกด See details เพื่อดูว่าติดอะไร","ui_target":"See details"}
 ]$steps$,
 'สวิตช์เปิดอยู่ไม่ได้แปลว่าโฆษณาวิ่ง ถ้าชั้นบน (แคมเปญหรือ ad set) ถูกปิดไว้ ตัวโฆษณาจะไม่วิ่งทั้งที่ตัวมันเองเปิด ต้องเช็กให้ครบทั้งสามชั้น',
 'ad_not_delivering',
 'ตั้งโฆษณาเสร็จแล้วนั่งรอทั้งวัน โดยที่มันไม่เคยวิ่งเลยสักครั้ง'),

('date_range_picker', 'เลือกช่วงวันที่ให้ถูก ทำไมตัวเลขถึงเปลี่ยนไปมา', 'ads_manager', 'beginner', 'basic',
 ARRAY['Ads Manager','Campaigns','Today','Yesterday','Last 7 days','Maximum','Update','Amount spent'],
 string_to_array($vocab$Ads Manager|Campaigns|Today|Yesterday|Last 7 days|Maximum|Update|Amount spent|Results$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เปิดตัวเลือกช่วงวันที่","action_th":"ที่มุมขวาบนของ Ads Manager กดที่ช่วงวันที่ที่แสดงอยู่","ui_target":"Ads Manager"},
  {"n":2,"title_th":"เลือกช่วงที่ตอบคำถามของคุณ","action_th":"อยากรู้ผลรวมทั้งหมดเลือก Maximum อยากรู้แนวโน้มล่าสุดเลือก Last 7 days","ui_target":"Maximum"},
  {"n":3,"title_th":"กดยืนยันทุกครั้ง","action_th":"กด Update แล้วค่อยอ่าน Amount spent กับ Results ใหม่","ui_target":"Update"}
 ]$steps$,
 'ตัวเลขทุกตัวบนหน้าจอผูกกับช่วงวันที่ที่เลือกไว้เสมอ ไม่ใช่ยอดรวมทั้งหมด คนใหม่มักตกใจว่ายอดหาย ทั้งที่แค่ค้างอยู่ที่ Today',
 'date_range_confusion',
 'อ่านตัวเลขคนละช่วงเวลาแล้วสรุปว่าแอดพัง ทั้งที่มันไม่ได้พัง'),

('campaign_structure_tour', 'Campaign กับ Ad set กับ Ad อะไรอยู่ชั้นไหน', 'ads_manager', 'beginner', 'basic',
 ARRAY['Ads Manager','Campaigns','Ad sets','Ads','Budget','Audience','Placements'],
 string_to_array($vocab$Ads Manager|Campaigns|Ad sets|Ads|Budget|Audience|Placements|Delivery$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"ชั้นบนสุดคือเป้าหมาย","action_th":"กดแท็บ Campaigns ชั้นนี้ตอบว่าอยากได้อะไร และคุมงบรวมที่ Budget","ui_target":"Campaigns"},
  {"n":2,"title_th":"ชั้นกลางคือยิงใส่ใคร","action_th":"กดแท็บ Ad sets ชั้นนี้ตั้ง Audience และ Placements ว่าจะไปโผล่ตรงไหน","ui_target":"Ad sets"},
  {"n":3,"title_th":"ชั้นล่างคือหน้าตาโฆษณา","action_th":"กดแท็บ Ads ชั้นนี้คือรูป วิดีโอ และข้อความที่คนเห็นจริง","ui_target":"Ads"}
 ]$steps$,
 'คนใหม่มักสร้างแคมเปญใหม่ทุกครั้งที่อยากเปลี่ยนรูป ทั้งที่ควรเพิ่มโฆษณาในชั้น Ads ของ ad set เดิม การสร้างใหม่ทุกครั้งทำให้ระบบต้องเริ่มเรียนรู้ใหม่หมด',
 'campaign_structure_confusion',
 'ไม่รู้ว่าอะไรอยู่ชั้นไหน แปลว่าแก้อะไรก็ไปแก้ผิดที่ตลอด'),

('daily_vs_lifetime_budget', 'งบรายวัน กับ งบตลอดอายุแคมเปญ เลือกอันไหน', 'ads_manager', 'beginner', 'basic',
 ARRAY['Ads Manager','Ad sets','Edit','Budget','Daily budget','Lifetime budget','Schedule','Publish'],
 string_to_array($vocab$Ads Manager|Campaigns|Ad sets|Edit|Budget|Daily budget|Lifetime budget|Schedule|Publish$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เข้าไปแก้ที่ชั้นงบ","action_th":"กดแท็บ Ad sets เลือกตัวที่จะแก้ แล้วกด Edit","ui_target":"Ad sets"},
  {"n":2,"title_th":"เลือกแบบงบ","action_th":"ที่ Budget เลือก Daily budget ถ้าอยากให้ใช้เท่ากันทุกวัน","ui_target":"Daily budget"},
  {"n":3,"title_th":"ใช้แบบตลอดอายุเมื่อมีวันสิ้นสุด","action_th":"เลือก Lifetime budget เมื่อมีวันจบชัดเจน แล้วต้องตั้ง Schedule ให้มีวันสิ้นสุดด้วย","ui_target":"Lifetime budget"},
  {"n":4,"title_th":"บันทึก","action_th":"กด Publish แล้วรอสถานะอัปเดต","ui_target":"Publish"}
 ]$steps$,
 'งบรายวันไม่ได้ใช้เท่ากันเป๊ะทุกวัน ระบบใช้เกินได้ในวันที่ผลดีและไปหักคืนวันอื่น ดูยอดวันเดียวแล้วตกใจว่าใช้เกิน คือเข้าใจผิด',
 'budget_type_choice',
 'เลือกแบบงบผิด แปลว่าเงินหมดเร็วกว่าที่วางแผนไว้ หรือระบบไม่ยอมใช้งบเลย'),

('campaign_objective_pick', 'เลือกวัตถุประสงค์แคมเปญให้ตรงกับสิ่งที่อยากได้', 'ads_manager', 'beginner', 'basic',
 ARRAY['Ads Manager','Create','Campaigns','Awareness','Traffic','Engagement','Leads','Sales','Next'],
 string_to_array($vocab$Ads Manager|Create|Campaigns|Awareness|Traffic|Engagement|Leads|Sales|Next$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เริ่มสร้างแคมเปญ","action_th":"ในหน้า Campaigns กด Create","ui_target":"Create"},
  {"n":2,"title_th":"เลือกสิ่งที่อยากได้จริงๆ","action_th":"อยากได้ยอดขายเลือก Sales อยากได้รายชื่อผู้สนใจเลือก Leads","ui_target":"Sales"},
  {"n":3,"title_th":"อย่าเลือกตามยอดที่ดูสวย","action_th":"Engagement จะได้ไลก์และคอมเมนต์เยอะแต่ไม่ได้แปลว่าขายได้","ui_target":"Engagement"},
  {"n":4,"title_th":"ไปต่อ","action_th":"กด Next เพื่อไปตั้งงบและกลุ่มเป้าหมาย","ui_target":"Next"}
 ]$steps$,
 'ระบบจะไปหาคนที่ทำสิ่งที่คุณเลือกไว้ ถ้าเลือก Traffic ระบบจะหาคนที่ชอบกดลิงก์ ซึ่งเป็นคนละกลุ่มกับคนที่ยอมจ่ายเงิน เปลี่ยนทีหลังไม่ได้ต้องสร้างแคมเปญใหม่',
 'objective_mismatch',
 'เลือกวัตถุประสงค์ผิดตั้งแต่ต้น แปลว่าระบบตั้งใจหาคนผิดกลุ่มให้คุณทั้งแคมเปญ'),

('boost_vs_ads_manager', 'กดโปรโมทโพสต์ กับ Ads Manager ต่างกันตรงไหน', 'ads_manager', 'beginner', 'basic',
 ARRAY['Ads Manager','Create','Campaigns','Ad sets','Audience','Placements','Sales','Existing post'],
 string_to_array($vocab$Ads Manager|Create|Campaigns|Ad sets|Ads|Audience|Placements|Sales|Existing post$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เปิดเครื่องมือตัวเต็ม","action_th":"เข้า Ads Manager แล้วกด Create แทนการกดปุ่มโปรโมทที่โพสต์","ui_target":"Ads Manager"},
  {"n":2,"title_th":"เลือกเป้าหมายที่โปรโมทโพสต์ให้ไม่ได้","action_th":"เลือก Sales ซึ่งการกดโปรโมทที่โพสต์ไม่มีให้เลือก","ui_target":"Sales"},
  {"n":3,"title_th":"คุมกลุ่มเป้าหมายและตำแหน่งเอง","action_th":"ที่ Ad sets ตั้ง Audience และ Placements ได้ละเอียดกว่า","ui_target":"Ad sets"},
  {"n":4,"title_th":"ใช้โพสต์เดิมที่มียอดอยู่แล้วได้","action_th":"ในชั้นโฆษณาเลือก Existing post เพื่อยืมไลก์และคอมเมนต์เดิมมาใช้","ui_target":"Existing post"}
 ]$steps$,
 'ปุ่มโปรโมทที่โพสต์ไม่ใช่ของปลอม แต่มันเลือกเป้าหมายได้จำกัดและคุมตำแหน่งไม่ได้ ถ้าเป้าหมายคือยอดขาย มันจะทำได้ไม่เท่า ไม่ใช่เพราะเงินน้อยกว่า',
 'boost_vs_adsmanager',
 'ใช้เครื่องมือที่คุมอะไรไม่ได้ แปลว่าจ่ายเท่ากันแต่ได้ผลน้อยกว่าที่ควร'),

('basic_audience_setup', 'ตั้งกลุ่มเป้าหมายเบื้องต้น ไม่ให้กว้างหรือแคบเกินไป', 'ads_manager', 'beginner', 'basic',
 ARRAY['Ads Manager','Ad sets','Edit','Audience','Location','Age','Detailed targeting','Audience size','Publish'],
 string_to_array($vocab$Ads Manager|Ad sets|Edit|Audience|Location|Age|Detailed targeting|Audience size|Publish$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เข้าไปที่ชั้นกลุ่มเป้าหมาย","action_th":"กดแท็บ Ad sets เลือกตัวที่จะแก้ แล้วกด Edit","ui_target":"Ad sets"},
  {"n":2,"title_th":"จำกัดพื้นที่ก่อนเสมอ","action_th":"ที่ Audience ตั้ง Location ให้เหลือเฉพาะพื้นที่ที่ส่งของหรือให้บริการได้จริง","ui_target":"Location"},
  {"n":3,"title_th":"ตั้งช่วงอายุตามลูกค้าจริง","action_th":"ตั้ง Age ให้ตรงกับลูกค้าที่เคยซื้อจริง ไม่ใช่ช่วงกว้างสุด","ui_target":"Age"},
  {"n":4,"title_th":"ดูขนาดกลุ่มก่อนบันทึก","action_th":"ดู Audience size ทางขวาให้อยู่ในโซนที่ระบบแนะนำ แล้วกด Publish","ui_target":"Audience size"}
 ]$steps$,
 'ใส่ความสนใจใน Detailed targeting หลายอันพร้อมกันไม่ได้แปลว่าแคบลง ระบบนับแบบ "อย่างใดอย่างหนึ่ง" ยิ่งใส่เยอะกลุ่มยิ่งกว้างขึ้น',
 'basic_audience_setup',
 'กลุ่มกว้างเกินคือจ่ายให้คนที่ไม่มีวันซื้อ แคบเกินคือระบบหาคนไม่พอจนไม่ยอมใช้งบ'),

('page_ad_account_link', 'ผูกเพจกับบัญชีโฆษณาให้ยิงได้', 'business_settings', 'beginner', 'basic',
 ARRAY['Business settings','Accounts','Pages','Add','Ad accounts','Assign assets','Save changes'],
 string_to_array($vocab$Business settings|Accounts|Pages|Add|Ad accounts|Assign assets|Save changes$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เปิดหน้ารวมสินทรัพย์","action_th":"ใน Business settings กด Accounts แล้วเลือก Pages","ui_target":"Pages"},
  {"n":2,"title_th":"เพิ่มเพจเข้าระบบธุรกิจ","action_th":"กด Add แล้วเลือกเพจที่คุณเป็นแอดมิน","ui_target":"Add"},
  {"n":3,"title_th":"ผูกเพจเข้ากับบัญชีโฆษณา","action_th":"กด Assign assets แล้วเลือก Ad accounts ที่จะใช้ยิงเพจนี้","ui_target":"Assign assets"},
  {"n":4,"title_th":"บันทึก","action_th":"กด Save changes แล้วกลับไปเช็กว่าเพจโผล่ในตัวเลือกตอนสร้างโฆษณา","ui_target":"Save changes"}
 ]$steps$,
 'เป็นแอดมินเพจในฐานะบัญชีส่วนตัว ไม่เท่ากับเพจอยู่ในระบบธุรกิจ ถ้าไม่เพิ่มเข้ามาตรงนี้ ตอนสร้างโฆษณาจะหาเพจไม่เจอทั้งที่เห็นเพจอยู่ในเฟซบุ๊กปกติ',
 'page_account_link',
 'เพจไม่ผูกกับบัญชีโฆษณา แปลว่าสร้างโฆษณาไม่ได้เลยแม้จะมีเงินพร้อมจ่าย'),

('first_payment_method', 'ตั้งวิธีชำระเงินครั้งแรก', 'business_settings', 'beginner', 'basic',
 ARRAY['Billing & payments','Payment settings','Add payment method','Credit or debit card','Card number','Save','Primary'],
 string_to_array($vocab$Billing & payments|Payment settings|Add payment method|Credit or debit card|Card number|Save|Primary$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เปิดหน้าการเงิน","action_th":"จากเมนู Billing & payments กด Payment settings","ui_target":"Payment settings"},
  {"n":2,"title_th":"เพิ่มวิธีจ่ายเงิน","action_th":"กด Add payment method แล้วเลือก Credit or debit card","ui_target":"Add payment method"},
  {"n":3,"title_th":"กรอกข้อมูลบัตร","action_th":"ใส่ Card number ให้ตรงกับบัตรที่เปิดใช้จ่ายออนไลน์ไว้แล้ว","ui_target":"Card number"},
  {"n":4,"title_th":"ตั้งเป็นตัวหลักแล้วบันทึก","action_th":"ตั้งให้เป็น Primary แล้วกด Save","ui_target":"Primary"}
 ]$steps$,
 'เฟซบุ๊กไม่ได้ตัดเงินทันทีที่เริ่มยิง แต่ตัดเป็นรอบเมื่อยอดสะสมถึงเกณฑ์ หรือตามรอบบิล คนใหม่มักคิดว่าบัตรมีปัญหาเพราะยังไม่เห็นยอดตัด',
 'first_payment_setup',
 'ไม่มีวิธีจ่ายเงินที่ใช้ได้ แปลว่าโฆษณาหยุดกลางคันโดยไม่มีใครบอก'),

('billing_receipt_read', 'ดูใบเสร็จและยอดที่ถูกตัดจริง', 'business_settings', 'beginner', 'basic',
 ARRAY['Billing & payments','Transactions','Amount','Date','Download','Payment settings'],
 string_to_array($vocab$Billing & payments|Transactions|Amount|Date|Download|Payment settings|Ads Manager$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เปิดหน้ารายการเรียกเก็บเงิน","action_th":"จากเมนู Billing & payments กด Transactions","ui_target":"Transactions"},
  {"n":2,"title_th":"อ่านยอดที่ตัดจริงแต่ละรอบ","action_th":"ดู Amount คู่กับ Date เพื่อรู้ว่าถูกตัดไปเมื่อไหร่ครั้งละเท่าไหร่","ui_target":"Amount"},
  {"n":3,"title_th":"เก็บใบเสร็จไว้ทำบัญชี","action_th":"กด Download เพื่อโหลดใบเสร็จของรอบนั้น","ui_target":"Download"}
 ]$steps$,
 'ยอดในหน้านี้กับยอดใน Ads Manager ไม่ตรงกันเป็นเรื่องปกติ เพราะหน้านี้คือเงินที่ถูกตัดไปแล้วตามรอบ ส่วนอีกหน้าคือยอดใช้จ่ายสะสมของช่วงวันที่ที่เลือกอยู่',
 'billing_receipt_reading',
 'อ่านยอดตัดเงินไม่เป็น แปลว่าทำบัญชีไม่ตรงและเถียงกับธนาคารไม่ได้')
ON CONFLICT (feature_key) DO NOTHING;

COMMIT;
