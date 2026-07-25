-- 062: seed tutorial catalog (8 ฟีเจอร์ตั้งต้น = ผลิตได้ 8 วันโดยไม่ซ้ำ)
-- ทุก steps[].ui_target ต้องอยู่ใน ui_vocab ของแถวเดียวกัน (บังคับด้วย TestSeedStepsCoveredByUIVocab)
-- ⚠ menu_path/ui_vocab ต้องถูกตรวจกับหน้าจอ Ads Manager จริงก่อนเปิด schedule
-- RunMigrations ไม่หุ้ม transaction ให้ ต้อง BEGIN/COMMIT เอง
-- Rollback: DELETE FROM tutorial_features;
BEGIN;

INSERT INTO tutorial_features
    (feature_key, display_name_th, surface, menu_path, ui_vocab, steps, trap_th, pain_point, why_matters_th)
VALUES
('automated_rules_cpm_guard', 'ตั้ง Automated Rules ตัดงบเมื่อ CPM พุ่ง', 'ads_manager',
 ARRAY['Ads Manager','Rules','Create new rule'],
 string_to_array($vocab$Ads Manager|Campaigns|Ad sets|Ads|Rules|Create new rule|Rule name|Apply to|Action|Turn off ad set|Conditions|Cost per result|Greater than|Time range|Schedule|Continuously|Create$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เปิดหน้าสร้างกฎ","action_th":"ในเมนูซ้ายของ Ads Manager กด Rules แล้วกด Create new rule","ui_target":"Rules"},
  {"n":2,"title_th":"เลือกสิ่งที่จะให้ระบบปิด","action_th":"ที่ช่อง Action เลือก Turn off ad set","ui_target":"Turn off ad set"},
  {"n":3,"title_th":"ตั้งเงื่อนไขให้ตัดงบ","action_th":"ที่ Conditions เลือก Cost per result / Greater than แล้วใส่ตัวเลขที่รับไม่ได้","ui_target":"Cost per result","value_th":"400 บาท"},
  {"n":4,"title_th":"ให้กฎทำงานตลอดเวลา","action_th":"ที่ Schedule เลือก Continuously แล้วกด Create","ui_target":"Continuously"}
 ]$steps$,
 'คนส่วนใหญ่ตั้ง Time range เป็นช่วงยาว ทำให้กฎไม่ยิงตอนบัญชีเพิ่งเริ่มพัง ต้องตั้งช่วงสั้น เช่น วันนี้หรือ 3 วันล่าสุด',
 'scaling_velocity_ceiling',
 'บัญชีโดนปิดตอนตีสามแต่งบยังวิ่ง เพราะไม่มีอะไรกดปิดให้'),

('account_spending_limit', 'ตั้งเพดานงบระดับบัญชี กันงบวิ่งเกินตอนไม่ได้ดู', 'business_settings',
 ARRAY['Billing','Payment settings','Account spending limit'],
 string_to_array($vocab$Billing|Payment settings|Account spending limit|Set limit|Amount|Save|Reset spending limit|Remove limit$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เข้าหน้าตั้งค่าการชำระเงิน","action_th":"จากเมนู Billing กด Payment settings","ui_target":"Payment settings"},
  {"n":2,"title_th":"เปิดเพดานงบของบัญชี","action_th":"หา Account spending limit แล้วกด Set limit","ui_target":"Account spending limit"},
  {"n":3,"title_th":"ใส่เพดานที่ยอมเสียได้จริง","action_th":"ที่ช่อง Amount ใส่ยอดรวมสูงสุดที่ยอมให้บัญชีนี้ใช้ แล้วกด Save","ui_target":"Amount","value_th":"เท่ากับงบ 3 วัน"}
 ]$steps$,
 'เพดานนี้นับสะสมจากยอดใช้จ่ายเดิมของบัญชีด้วย ถ้าตั้งต่ำกว่ายอดที่ใช้ไปแล้ว แอดจะหยุดทันที ต้องกด Reset spending limit ก่อน',
 'account_burn_economics',
 'บัญชีเดียวเผางบเกินแผนได้ในคืนเดียวถ้าไม่มีเพดานกั้น'),

('backup_payment_method', 'ผูกบัตรสำรอง กันแอดหยุดเพราะตัดเงินไม่ผ่าน', 'business_settings',
 ARRAY['Billing','Payment settings','Add payment method'],
 string_to_array($vocab$Billing|Payment settings|Add payment method|Credit or debit card|Card number|Save|Set as primary|Backup payment method$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เข้าหน้าวิธีชำระเงิน","action_th":"จากเมนู Billing กด Payment settings","ui_target":"Payment settings"},
  {"n":2,"title_th":"เพิ่มบัตรใบที่สอง","action_th":"กด Add payment method แล้วเลือก Credit or debit card","ui_target":"Add payment method"},
  {"n":3,"title_th":"ตั้งให้เป็นบัตรสำรอง","action_th":"บันทึกบัตรแล้วปล่อยไว้เป็น Backup payment method ไม่ต้องกด Set as primary","ui_target":"Backup payment method"}
 ]$steps$,
 'ถ้าใช้บัตรชื่อเดียวกันธนาคารเดียวกันกับใบหลัก พอโดนบล็อกจะโดนพร้อมกันทั้งคู่ บัตรสำรองต้องคนละธนาคาร',
 'payment_method_flag',
 'ตัดเงินไม่ผ่านรอบเดียว บัญชีหยุดวิ่งทั้งคืน'),

('export_custom_audience', 'สำรอง Custom Audience ก่อนบัญชีตาย', 'ads_manager',
 ARRAY['Audiences','Share audience'],
 string_to_array($vocab$Audiences|Custom Audiences|Share audience|Share to|Business|Audience name|Share|Permissions$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เปิดรายการกลุ่มเป้าหมาย","action_th":"จากเมนู Ads Manager เข้า Audiences แล้วดูที่ Custom Audiences","ui_target":"Custom Audiences"},
  {"n":2,"title_th":"เลือกกลุ่มที่ต้องเก็บไว้","action_th":"ติ๊กกลุ่มที่สร้างจากลูกค้าจริง แล้วกด Share audience","ui_target":"Share audience"},
  {"n":3,"title_th":"แชร์ไปยัง BM สำรอง","action_th":"ที่ Share to เลือก Business ปลายทางที่แยกไว้ แล้วกด Share","ui_target":"Share to"}
 ]$steps$,
 'แชร์ audience ไม่ได้ก๊อปข้อมูล ถ้า BM ต้นทางโดนปิด audience ที่แชร์ไปก็หายด้วย ต้องแชร์จาก BM ที่สะอาดที่สุดเป็นต้นทาง',
 'asset_backup_strategy',
 'บัญชีตายพร้อมกลุ่มลูกค้าที่สะสมมาเป็นปี'),

('domain_verification', 'ยืนยันโดเมน กันคนอื่นแย่งสิทธิ์และลด flag ที่ landing page', 'business_settings',
 ARRAY['Business settings','Brand safety','Domains'],
 string_to_array($vocab$Business settings|Brand safety|Domains|Add|Domain name|DNS Verification|Meta-tag Verification|HTML File Upload|Verify domain|Verified$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เปิดหน้าโดเมน","action_th":"ใน Business settings เข้า Brand safety แล้วกด Domains","ui_target":"Domains"},
  {"n":2,"title_th":"เพิ่มโดเมนที่ใช้ยิงจริง","action_th":"กด Add แล้วพิมพ์โดเมนที่ช่อง Domain name","ui_target":"Domain name"},
  {"n":3,"title_th":"เลือกวิธียืนยันที่ทำได้เร็วสุด","action_th":"เลือก DNS Verification แล้วเอา TXT record ไปใส่ที่ผู้ให้บริการโดเมน","ui_target":"DNS Verification"},
  {"n":4,"title_th":"กดยืนยันให้ขึ้น Verified","action_th":"กลับมากด Verify domain แล้วรอสถานะเปลี่ยนเป็น Verified","ui_target":"Verify domain"}
 ]$steps$,
 'ต้องยืนยันโดเมนหลักแบบไม่มี www และไม่มี path ถ้าใส่ทั้ง URL ระบบจะไม่ผ่านและวนอยู่แบบนั้น',
 'landing_page_flag',
 'โดเมนไม่ได้ยืนยัน แปลว่าใครก็อ้างสิทธิ์ได้ และ ad ผ่านแต่ landing โดนตี'),

('backup_admin_2fa', 'ตั้งแอดมินสำรอง และบังคับ 2FA กันโดนล็อกออกทั้งทีม', 'business_settings',
 ARRAY['Business settings','People','Add people'],
 string_to_array($vocab$Business settings|People|Add people|Email address|Admin access|Assign assets|Security Center|Two-factor authentication|Required for everyone|Save changes$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เพิ่มคนที่สองเป็นแอดมิน","action_th":"ใน Business settings เข้า People กด Add people ใส่ Email address แล้วเลือก Admin access","ui_target":"Admin access"},
  {"n":2,"title_th":"ให้สิทธิ์เข้าถึง asset","action_th":"กด Assign assets แล้วเลือกบัญชีโฆษณาและเพจที่ต้องเข้าถึงได้","ui_target":"Assign assets"},
  {"n":3,"title_th":"บังคับ 2FA ทั้งทีม","action_th":"เข้า Security Center ที่ Two-factor authentication เลือก Required for everyone แล้วกด Save changes","ui_target":"Two-factor authentication"}
 ]$steps$,
 'ถ้าแอดมินสำรองใช้เบอร์และอีเมลชุดเดียวกับคนแรก พอโดน checkpoint จะเข้าไม่ได้ทั้งคู่ ต้องคนละเบอร์คนละอีเมล',
 'checkpoint_lock_2fa',
 'แอดมินคนเดียวติด checkpoint แปลว่าทั้งพอร์ตเข้าไม่ได้'),

('account_quality_check', 'อ่าน Account Quality ให้ออกว่าโดนอะไรและอุทธรณ์ตรงไหน', 'account_quality',
 ARRAY['Account Quality','Account status'],
 string_to_array($vocab$Account Quality|Account status|Issues|See details|Policy|Request review|Appeal|Restricted|Ad accounts|Pages$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เปิดหน้าสถานะบัญชี","action_th":"เข้า Account Quality แล้วดูที่ Account status","ui_target":"Account status"},
  {"n":2,"title_th":"หาสาเหตุจริงไม่ใช่ข้อความสรุป","action_th":"ที่รายการใน Issues กด See details เพื่อดูว่าชนนโยบายข้อไหน","ui_target":"See details"},
  {"n":3,"title_th":"ยื่นอุทธรณ์จากหน้านี้เท่านั้น","action_th":"กด Request review ในรายการนั้นโดยตรง อย่าไปยื่นจากหน้าอื่น","ui_target":"Request review"}
 ]$steps$,
 'ปุ่ม Request review เป็นสีเทาเมื่อมียอดค้างชำระ ต้องเคลียร์ยอดก่อนถึงจะกดได้ ไม่ใช่ระบบพัง',
 'appeal_bot_rejection',
 'อุทธรณ์ผิดที่ผิดเวลา เท่ากับโดนปัดตกใน 10 นาทีโดยไม่มีคนอ่าน'),

('audience_overlap_check', 'เช็ก Audience Overlap กันแอดตัวเองแย่ง auction กันเอง', 'ads_manager',
 ARRAY['Audiences','Show audience overlap'],
 string_to_array($vocab$Audiences|Custom Audiences|Show audience overlap|Compare|Overlap|Selected audience|Ad sets$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เลือกกลุ่มตั้งต้น","action_th":"ใน Audiences ติ๊กกลุ่มที่ใช้เป็นตัวหลักที่ Custom Audiences","ui_target":"Custom Audiences"},
  {"n":2,"title_th":"เปิดเครื่องมือเทียบ","action_th":"กด Show audience overlap แล้วเลือกกลุ่มอื่นที่ใช้ยิงพร้อมกันมา Compare","ui_target":"Show audience overlap"},
  {"n":3,"title_th":"อ่านตัวเลขให้เป็น","action_th":"ดูค่า Overlap ถ้าเกิน 30 เปอร์เซ็นต์ ให้ยุบเหลือ ad set เดียว","ui_target":"Overlap","value_th":"เกิน 30 เปอร์เซ็นต์"}
 ]$steps$,
 'เครื่องมือนี้เทียบได้เฉพาะ Custom Audience ไม่ครอบคลุมกลุ่ม interest ที่ซ้อนกัน ซ้อนกันจริงอาจมากกว่าที่เห็น',
 'audience_overlap_self_bid',
 'ยิงหลาย ad set ทับกลุ่มเดียวกัน เท่ากับจ่าย CPM แพงขึ้นเพราะสู้กับตัวเอง')
ON CONFLICT (feature_key) DO NOTHING;

COMMIT;
