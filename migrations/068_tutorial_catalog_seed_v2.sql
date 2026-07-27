-- 068: เติมคลัง tutorial อีก 10 ฟีเจอร์ ให้ครบทั้ง 3 กลุ่มเป้าหมายของ migration 057
--
-- ก่อนหน้านี้คลังมี 8 แถว เอียงไปทางบัญชี/สายเทา (grey 4, account-buyer 3,
-- performance 1) หลังไฟล์นี้เป็น 18 แถว = กลุ่มละ 6 → รันได้ 18 วันไม่ซ้ำ
-- pain_point ทุกค่าเลือกจากเมนูของกลุ่มนั้นใน migration 057 เท่านั้น
--
-- ทุก steps[].ui_target ต้องอยู่ใน ui_vocab ของแถวเดียวกัน (บังคับด้วย TestSeedStepsCoveredByUIVocab)
-- ⚠ menu_path/ui_vocab เขียนจากความรู้ ยังไม่ได้เทียบกับหน้าจอจริงทีละหน้า
--   ตัวกรอง needs_verify + research agent เป็นตาข่ายรับ แต่ควร eyeball คลิปแรกของแต่ละแถว
-- RunMigrations ไม่หุ้ม transaction ให้ ต้อง BEGIN/COMMIT เอง
-- Rollback: DELETE FROM tutorial_features WHERE created_at > (SELECT MAX(created_at) FROM tutorial_features WHERE feature_key = 'audience_overlap_check');
BEGIN;

INSERT INTO tutorial_features
    (feature_key, display_name_th, surface, audience, menu_path, ui_vocab, steps, trap_th, pain_point, why_matters_th)
VALUES
('breakdown_placement_cut', 'แยกผลรายตำแหน่งโฆษณา แล้วตัดตัวที่เผางบทิ้ง', 'ads_manager', 'performance-advertiser',
 ARRAY['Ads Manager','Breakdown','By Delivery','Placement'],
 string_to_array($vocab$Ads Manager|Campaigns|Ad sets|Ads|Breakdown|By Delivery|Placement|Columns|Cost per result|Impressions|Edit|Placements|Manual placements$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เปิดมุมมองแยกผล","action_th":"ที่แถบเครื่องมือของ Ads Manager กด Breakdown แล้วเลือก By Delivery","ui_target":"Breakdown"},
  {"n":2,"title_th":"แตกผลออกเป็นรายตำแหน่ง","action_th":"ใน By Delivery กด Placement","ui_target":"Placement"},
  {"n":3,"title_th":"หาตัวที่กินงบแต่ไม่ให้ผล","action_th":"ไล่ดู Cost per result ของแต่ละแถว เทียบกับค่าเฉลี่ยของแคมเปญ","ui_target":"Cost per result","value_th":"แพงกว่าค่าเฉลี่ย 2 เท่า"},
  {"n":4,"title_th":"ปิดตำแหน่งที่เผางบ","action_th":"กลับไป Edit ที่ ad set แล้วเลือก Manual placements เอาตำแหน่งนั้นออก","ui_target":"Manual placements"}
 ]$steps$,
 'ตัดตำแหน่งตั้งแต่ข้อมูลยังน้อย ทำให้เสีย inventory ราคาถูกไปฟรีๆ ต้องรอให้แต่ละตำแหน่งมี impression มากพอก่อนถึงตัดได้',
 'cpm_spike_no_change',
 'CPM พุ่งทั้งที่ครีเอทีฟไม่ได้เปลี่ยน เพราะงบไหลไปตำแหน่งที่ไม่มีวันปิดการขาย'),

('custom_columns_frequency', 'จัดคอลัมน์ให้เห็น Frequency คู่ CPM ก่อนครีเอทีฟจะตาย', 'ads_manager', 'performance-advertiser',
 ARRAY['Ads Manager','Columns','Customise columns'],
 string_to_array($vocab$Ads Manager|Columns|Customise columns|Performance|Frequency|Reach|CPM|Impressions|Apply|Save as preset$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เปิดหน้าจัดคอลัมน์","action_th":"ที่แถบขวาบนกด Columns แล้วเลือก Customise columns","ui_target":"Customise columns"},
  {"n":2,"title_th":"เพิ่มตัวเลขที่บอกอาการล้า","action_th":"ค้นหาแล้วติ๊ก Frequency และ Reach","ui_target":"Frequency"},
  {"n":3,"title_th":"เพิ่มราคาต่อการมองเห็น","action_th":"ติ๊ก CPM เพื่อให้อ่านคู่กับ Frequency ได้ในแถวเดียว","ui_target":"CPM"},
  {"n":4,"title_th":"เก็บชุดคอลัมน์ไว้ใช้ทุกวัน","action_th":"กด Save as preset ตั้งชื่อ แล้วกด Apply","ui_target":"Save as preset"}
 ]$steps$,
 'ดู Frequency ที่ระดับแคมเปญจะต่ำเสมอเพราะเฉลี่ยรวมทุก ad set ต้องดูที่ระดับ ad set ถึงจะเจอตัวที่ล้าจริง',
 'frequency_burnout_threshold',
 'รู้ว่าครีเอทีฟตายช้าไป 3 วัน เท่ากับจ่ายแพงขึ้นทุกวันโดยไม่มีใครเตือน'),

('video_hook_retention_columns', 'อ่านตัวเลข 3 วินาทีแรก ว่า hook รอดหรือไม่รอด', 'ads_manager', 'performance-advertiser',
 ARRAY['Ads Manager','Columns','Customise columns','Video engagement'],
 string_to_array($vocab$Ads Manager|Columns|Customise columns|Video engagement|3-second video plays|ThruPlay|Video average play time|Impressions|Apply$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เปิดหน้าจัดคอลัมน์","action_th":"กด Columns แล้วเลือก Customise columns","ui_target":"Customise columns"},
  {"n":2,"title_th":"เข้าหมวดตัวเลขวิดีโอ","action_th":"ในเมนูด้านซ้ายกด Video engagement","ui_target":"Video engagement"},
  {"n":3,"title_th":"ติ๊กตัวเลขที่บอกว่า hook รอด","action_th":"ติ๊ก 3-second video plays และ ThruPlay แล้วกด Apply","ui_target":"3-second video plays"},
  {"n":4,"title_th":"อ่านเป็นสัดส่วน ไม่ใช่ตัวเลขดิบ","action_th":"เอา 3-second video plays หารด้วย Impressions ของแถวเดียวกัน","ui_target":"Impressions","value_th":"ต่ำกว่า 20 เปอร์เซ็นต์ คือ hook ไม่ผ่าน"}
 ]$steps$,
 'ThruPlay สูงไม่ได้แปลว่า hook ดี ถ้าคลิปสั้นกว่า 15 วินาที ระบบนับจบคลิปเป็น ThruPlay ทั้งที่คนแค่ปล่อยผ่าน',
 'hook_dropoff_3s',
 'ครีเอทีฟตายตั้งแต่ 3 วิแรก แต่เลข ROAS รวมยังไม่ฟ้อง กว่าจะรู้ก็เผางบไปหลายวัน'),

('ab_test_creative', 'สร้าง A/B Test ให้ผลตัดสินได้จริง ไม่ใช่เดาเอา', 'ads_manager', 'performance-advertiser',
 ARRAY['Ads Manager','A/B Test'],
 string_to_array($vocab$Ads Manager|Campaigns|A/B Test|Variable|Creative|Audience|Placement|Key metric|Cost per result|Test name|Duration|Create test$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เลือกตัวตั้งต้นที่จะใช้เทียบ","action_th":"ติ๊กแคมเปญที่รันอยู่แล้วกด A/B Test","ui_target":"A/B Test"},
  {"n":2,"title_th":"ล็อกว่าจะเทียบอะไรตัวเดียว","action_th":"ที่ Variable เลือก Creative เพื่อให้ต่างกันแค่ครีเอทีฟ","ui_target":"Creative"},
  {"n":3,"title_th":"ตั้งตัวชี้วัดที่ใช้ตัดสิน","action_th":"ที่ Key metric เลือก Cost per result ไม่ใช่ยอดคลิก","ui_target":"Key metric"},
  {"n":4,"title_th":"ให้เวลานานพอที่ผลจะนิ่ง","action_th":"ตั้ง Duration แล้วกด Create test","ui_target":"Duration","value_th":"4-7 วัน"}
 ]$steps$,
 'ถ้าเปลี่ยนมากกว่าหนึ่งอย่างในตัวที่เอามาเทียบ ผลที่ออกมาบอกไม่ได้ว่าชนะเพราะอะไร ต้องต่างกันแค่ครีเอทีฟจริงๆ',
 'creative_test_structure',
 'เทสแบบเดาเอาแล้วเชื่อผลผิด แปลว่าสเกลตัวที่ไม่ได้ชนะจริงด้วยงบก้อนใหญ่'),

('attribution_window_compare', 'เทียบหน้าต่างการวัดผล ให้เลขคุยกับหลังบ้านรู้เรื่อง', 'ads_manager', 'performance-advertiser',
 ARRAY['Ads Manager','Columns','Comparing attribution settings'],
 string_to_array($vocab$Ads Manager|Columns|Comparing attribution settings|7-day click|1-day click|1-day view|Results|Apply$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เปิดตัวเทียบหน้าต่างการวัดผล","action_th":"กด Columns แล้วเลือก Comparing attribution settings","ui_target":"Comparing attribution settings"},
  {"n":2,"title_th":"ติ๊กหน้าต่างที่จะเอามาเทียบ","action_th":"ติ๊ก 7-day click และ 1-day click พร้อมกัน","ui_target":"7-day click"},
  {"n":3,"title_th":"อ่านสองคอลัมน์ในแถวเดียวกัน","action_th":"กด Apply แล้วดู Results ของทั้งสองหน้าต่างเทียบกันทีละแถว","ui_target":"Results"}
 ]$steps$,
 'เลข 7-day click สูงกว่าเสมอ ไม่ได้แปลว่าดีกว่า ถ้าหลังบ้านนับแบบวันเดียว ต้องเอา 1-day click ไปเทียบเท่านั้นเลขถึงจะตรงกัน',
 'attribution_number_mismatch',
 'เถียงกับหลังบ้านว่าใครผิด ทั้งที่แค่คนละหน้าต่างการวัดผล เสียเวลาและตัดงบผิดตัว'),

('payment_threshold_check', 'อ่าน Payment threshold ให้ออก ว่าวงเงินไม่ขยับเพราะอะไร', 'business_settings', 'account-buyer',
 ARRAY['Billing & payments','Payment settings','Payment threshold'],
 string_to_array($vocab$Billing & payments|Payment settings|Payment threshold|Amount|Transactions|Account spending limit|Save$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เปิดหน้าการเงินของบัญชี","action_th":"จาก Billing & payments กด Payment settings","ui_target":"Payment settings"},
  {"n":2,"title_th":"ดูยอดที่ระบบใช้ตัดเงินเป็นรอบ","action_th":"หา Payment threshold แล้วจดตัวเลขปัจจุบันไว้","ui_target":"Payment threshold"},
  {"n":3,"title_th":"เทียบกับรายการที่ตัดจริง","action_th":"เปิด Transactions ดูว่ารอบที่ผ่านมาระบบตัดที่ยอดเท่าไหร่","ui_target":"Transactions"},
  {"n":4,"title_th":"แยกให้ออกว่าติดเพดานไหน","action_th":"ถ้ายอดตัดชนเลขเดิมทุกรอบคือติด threshold ไม่ใช่ Account spending limit","ui_target":"Account spending limit"}
 ]$steps$,
 'threshold ขยับเองเมื่อจ่ายตรงเวลาติดกันหลายรอบ กดขอเพิ่มถี่ๆ ไม่ได้ทำให้เร็วขึ้น และบัญชีใหม่จะเริ่มที่ยอดต่ำเสมอ',
 'spending_limit_frozen',
 'เข้าใจผิดว่าบัญชีโดนจำกัด ทั้งที่แค่ยังไม่ถึงรอบตัดเงิน เลยไปกดอุทธรณ์ทิ้งประวัติไว้ฟรีๆ'),

('business_verification_submit', 'ยื่นยืนยันธุรกิจให้ผ่านรอบเดียว', 'business_settings', 'account-buyer',
 ARRAY['Business settings','Security Centre','Business verification'],
 string_to_array($vocab$Business settings|Security Centre|Business verification|Start verification|Legal business name|Business address|Phone number|Website|Submit|Pending$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เปิดหน้ายืนยันธุรกิจ","action_th":"ใน Business settings กด Security Centre แล้วหา Business verification","ui_target":"Business verification"},
  {"n":2,"title_th":"กรอกชื่อให้ตรงเอกสารทุกตัวอักษร","action_th":"กด Start verification แล้วใส่ Legal business name ตามหนังสือรับรอง","ui_target":"Legal business name"},
  {"n":3,"title_th":"ใส่ที่อยู่และเบอร์ที่ตรวจย้อนได้","action_th":"กรอก Business address และ Phone number ชุดเดียวกับที่จดทะเบียน","ui_target":"Business address"},
  {"n":4,"title_th":"ส่งแล้วรอสถานะ","action_th":"กด Submit แล้วสถานะจะขึ้น Pending อย่ายื่นซ้ำระหว่างรอ","ui_target":"Submit"}
 ]$steps$,
 'ชื่อบริษัทที่พิมพ์ต่างจากหนังสือรับรองแม้แต่คำว่า จำกัด หรือจุด จะโดนตีตกทุกรอบโดยไม่บอกเหตุผล ต้องคัดลอกมาทั้งบรรทัด',
 'business_verification_stuck',
 'BM ที่ verify ไม่ผ่าน แปลว่าเพดานงบและสิทธิ์หลายอย่างถูกล็อกไว้ตั้งแต่วันแรก'),

('partner_access_grant', 'ให้สิทธิ์แบบ Partner แทนการส่งรหัสผ่าน', 'business_settings', 'account-buyer',
 ARRAY['Business settings','Partners','Add'],
 string_to_array($vocab$Business settings|Partners|Add|Give a partner access to your assets|Partner business ID|Assign assets|Ad accounts|Manage campaigns|Save changes$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เปิดหน้าพาร์ทเนอร์","action_th":"ใน Business settings กด Partners","ui_target":"Partners"},
  {"n":2,"title_th":"เลือกให้สิทธิ์แบบพาร์ทเนอร์","action_th":"กด Add แล้วเลือก Give a partner access to your assets","ui_target":"Give a partner access to your assets"},
  {"n":3,"title_th":"ใส่เลขธุรกิจของอีกฝ่าย","action_th":"กรอก Partner business ID ไม่ใช่อีเมลและไม่ใช่รหัสผ่าน","ui_target":"Partner business ID"},
  {"n":4,"title_th":"ให้สิทธิ์เท่าที่ต้องใช้","action_th":"ที่ Assign assets เลือก Ad accounts แล้วให้ระดับ Manage campaigns แล้วกด Save changes","ui_target":"Manage campaigns"}
 ]$steps$,
 'ถ้าให้สิทธิ์ด้วยการส่งรหัสผ่าน หรือยอมให้เขาเพิ่มเราเป็นคนใน BM ของเขา เจ้าของ BM ดึงทุกอย่างคืนได้ทันที การเป็น Partner คือฝั่งเราถือ asset เอง',
 'seller_clawback_risk',
 'จ่ายเงินไปแล้วแต่ asset ยังอยู่ในมือคนขาย เท่ากับเช่าอยู่โดยไม่รู้ตัว'),

('ad_review_status_check', 'อ่านสถานะ review ให้ออก ว่าค้างปกติหรือค้างผิดปกติ', 'ads_manager', 'grey-operator',
 ARRAY['Ads Manager','Ads','Delivery'],
 string_to_array($vocab$Ads Manager|Campaigns|Ad sets|Ads|Delivery|In review|Active|Rejected|See details|Edit|Account Quality$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เปิดที่ระดับโฆษณา","action_th":"ใน Ads Manager กดแท็บ Ads ไม่ใช่ระดับแคมเปญ","ui_target":"Ads"},
  {"n":2,"title_th":"อ่านคอลัมน์สถานะจริง","action_th":"ดูคอลัมน์ Delivery ว่าขึ้น In review หรือ Active","ui_target":"Delivery"},
  {"n":3,"title_th":"เปิดเหตุผลของตัวที่ค้าง","action_th":"ชี้ที่ In review แล้วกด See details","ui_target":"See details"},
  {"n":4,"title_th":"แยกว่าเป็นที่ ad หรือที่บัญชี","action_th":"ถ้าค้างพร้อมกันหลายตัว ให้ไปดู Account Quality ว่าบัญชีติดอะไรอยู่","ui_target":"Account Quality","value_th":"ค้างเกิน 24 ชั่วโมง"}
 ]$steps$,
 'กด Edit แก้โฆษณาที่ค้าง review จะรีเซ็ตคิวใหม่ทั้งตัว ยิ่งแก้ยิ่งค้าง ถ้ารีบให้ปิดตัวนั้นแล้วสร้างใหม่แทนการแก้',
 'review_queue_purgatory',
 'แยกไม่ออกว่าค้างเพราะคิวหรือเพราะบัญชีโดนจับตา ทำให้แก้ผิดทางจนลามหนักกว่าเดิม'),

('pixel_partner_separation', 'แยกพิกเซล กันบัญชีคนละชุดเชื่อมถึงกัน', 'business_settings', 'grey-operator',
 ARRAY['Business settings','Data sources','Pixels'],
 string_to_array($vocab$Business settings|Data sources|Pixels|Add|Assign partners|Assign assets|Ad accounts|Partner business ID|Save changes$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เปิดรายการแหล่งข้อมูล","action_th":"ใน Business settings กด Data sources แล้วเลือก Pixels","ui_target":"Pixels"},
  {"n":2,"title_th":"ดูว่าพิกเซลตัวนี้ผูกกับใครบ้าง","action_th":"กด Assign partners เพื่อดูรายชื่อธุรกิจที่ใช้พิกเซลตัวเดียวกัน","ui_target":"Assign partners"},
  {"n":3,"title_th":"สร้างพิกเซลแยกให้แต่ละชุด","action_th":"ถ้าเจอบัญชีคนละชุดใช้ตัวเดียวกัน ให้กด Add สร้างพิกเซลใหม่ของชุดนั้น","ui_target":"Add"},
  {"n":4,"title_th":"ผูกเฉพาะบัญชีของชุดนั้น","action_th":"ที่ Assign assets เลือกเฉพาะ Ad accounts ของชุดนั้นแล้วกด Save changes","ui_target":"Save changes"}
 ]$steps$,
 'พิกเซลตัวเดียวที่ผูกข้ามหลายชุดบัญชี คือเส้นเชื่อมที่ชัดที่สุดเส้นหนึ่ง แยกบัตร แยกเครื่อง แยกเพจแล้วแต่ยังใช้พิกเซลร่วมกัน เท่ากับบอกเองว่าเจ้าของเดียวกัน',
 'cross_account_linking',
 'ทำทุกอย่างแยกกันหมดแล้วบัญชียังลามพร้อมกัน เพราะลืมเส้นที่มองไม่เห็นเส้นนี้')
ON CONFLICT (feature_key) DO NOTHING;

COMMIT;
