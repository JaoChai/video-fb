-- 080: เติมคลังหัวข้อพื้นฐาน 18 เรื่อง (ตำแหน่งโฆษณา / แคมเปญ / ชิ้นงานกับปลายทาง)
-- spec: docs/superpowers/specs/2026-08-06-basic-catalog-expansion-design.md
--
-- คลัง level='basic' เดิม 12 แถว เหลือเรื่องที่ยังไม่เคยออกอากาศ 3 เรื่อง หลังจากนั้น
-- ตัวเลือกแบบ least-used จะเริ่มวนหัวข้อเดิม migration นี้ยกคลังเป็น 30 แถว
--
-- ทุก steps[].ui_target ต้องอยู่ใน ui_vocab ของแถวเดียวกัน (บังคับด้วย TestSeedStepsCoveredByUIVocab)
-- ui_vocab = "คำศัพท์ UI ที่อนุญาตให้ปรากฏบนจอเท่านั้น" (agent.UIVocabViolations ตีคลิปตกถ้าหลุด)
--
-- pain_point ของแถว: produceCatalogClip ส่งค่านี้เข้า GeneratedQuestion → ลง topic_history
-- ซึ่ง Deduper.PainPointInCooldown ใช้กันหัวข้อซ้ำของรอบ 12:00/18:00 จึงต้องไม่ชนกับค่าเดิม
-- เมนู pain_point ใน topic_categories.beginner (ส่วนที่ 1): QuestionAgent เป็นผู้อ่าน แต่ช่อง
-- 15:00 ข้าม QuestionAgent ทั้งหมด (หัวข้อคือแถวในคลัง) และ category beginner ยัง enabled=false
-- อยู่ ⇒ ตอนนี้ยังไม่มีใครอ่านเมนูนี้จริง เติมไว้เพื่อรักษากฎที่ migration 071/072 ตั้งไว้ว่า
-- เมนูต้องสะท้อนคลัง — วันที่เปิด category นี้จะได้ไม่มีหัวข้อตกหล่น
--
-- ชื่อเมนู/ปุ่ม/ลำดับขั้นทุกแถวยืนยันจาก Meta Business Help Center (locale=en_US) เมื่อ 2026-08-06
-- URL อ้างอิงกำกับไว้เหนือแต่ละแถว — WebFetch อ่านหน้าเหล่านี้ไม่ได้ (SPA) ต้องเปิดผ่านเบราว์เซอร์
--
-- RunMigrations ไม่หุ้ม transaction ให้ ต้อง BEGIN/COMMIT เอง
-- Rollback:
--   DELETE FROM tutorial_features WHERE feature_key IN ('placement_advantage_vs_manual',
--     'placement_map_2026','placement_breakdown_read','audience_network_choice',
--     'campaign_create_end_to_end','conversion_location_choice','campaign_duplicate',
--     'campaign_off_vs_delete','ad_schedule_dayparting','learning_phase_basics',
--     'ad_format_choice','aspect_ratio_specs','ad_copy_fields','cta_button_choice',
--     'ad_preview_before_publish','destination_choice','pixel_basics','edit_running_ad');
--   แล้วตัดบล็อกเมนูที่ขึ้นต้นด้วย '- placement_mode_choice:' ถึง '- edit_running_ad:' ออกจาก
--   topic_categories.angle_instruction ของแถว beginner
BEGIN;

-- 1) ขยายเมนู pain_point ของกลุ่ม beginner ให้ครอบคลุม 18 หัวข้อใหม่
--    ใช้ DO block เพื่อ "ล้มให้ดัง" ถ้าหาข้อความยึดไม่เจอ — replace() ที่ไม่เจอจะเงียบ
--    แล้ว agent จะเห็นเมนูเก่าตลอดไป ส่วนแถวใหม่จะมี pain_point ที่ไม่มีในเมนู
DO $mig$
DECLARE
  anchor TEXT := '- breakdown_basics: ดูผลแยกตามอายุ-เพศ ทำยังไง';
  addition TEXT := '- breakdown_basics: ดูผลแยกตามอายุ-เพศ ทำยังไง
- placement_mode_choice: Advantage+ placements กับ Manual placements ต่างกันยังไง
- placement_landscape: ไม่รู้ว่าแอดไปโผล่ที่ไหนได้บ้าง
- placement_performance_read: ดูไม่เป็นว่าเงินไปตกที่ตำแหน่งไหน
- audience_network_doubt: Audience Network คืออะไร ควรตัดทิ้งไหม
- first_campaign_walkthrough: สร้างแคมเปญแรกไม่เป็น ไม่รู้ว่าต้องกดอะไรตามลำดับไหน
- conversion_location_confusion: เลือก Conversion location ไม่ถูก ผลลัพธ์เลยไปเกิดผิดที่
- duplicate_vs_new: อยากได้แอดอีกตัว ควรคัดลอกหรือสร้างใหม่
- off_vs_delete: ปิดกับลบต่างกันยังไง กดอันไหนดี
- ad_scheduling_setup: อยากให้แอดวิ่งเฉพาะบางช่วงเวลา ตั้งตรงไหน
- learning_phase_meaning: เห็นคำว่า Learning ในคอลัมน์ Delivery แล้วไม่รู้ว่าคืออะไร
- ad_format_choice: รูปเดี่ยว วิดีโอ หรือคาร์รูเซล เลือกอันไหน
- creative_size_specs: ไม่รู้ว่าต้องเตรียมภาพสัดส่วนไหน ขนาดเท่าไหร่
- ad_copy_field_meaning: Primary text, Headline, Description ช่องไหนคืออะไร
- cta_button_choice: เลือกปุ่ม Call to action ไม่ถูก
- preview_before_publish: กด Publish ไปโดยไม่เคยเห็นว่าแอดหน้าตาเป็นยังไง
- destination_choice: ส่งคนไปเว็บ ไปแชท หรือไปไหนดี
- pixel_basics: พิกเซลคืออะไร คนเพิ่งเริ่มต้องมีไหม
- edit_running_ad: แก้แอดที่กำลังวิ่งอยู่แล้วจะเกิดอะไรขึ้น';
  cur TEXT;
BEGIN
  SELECT angle_instruction INTO cur FROM topic_categories WHERE category_name = 'beginner';
  IF cur IS NULL THEN
    RAISE EXCEPTION 'topic_categories row beginner missing — run migration 071 first';
  END IF;
  IF position('- placement_mode_choice:' IN cur) > 0 THEN
    RETURN; -- idempotent: เติมไปแล้ว
  END IF;
  IF position(anchor IN cur) = 0 THEN
    RAISE EXCEPTION 'pain_point menu anchor not found in beginner angle_instruction — update migration 080 before shipping';
  END IF;
  UPDATE topic_categories
  SET angle_instruction = replace(angle_instruction, anchor, addition)
  WHERE category_name = 'beginner';
END $mig$;

-- 2) หัวข้อใหม่ 18 แถว
INSERT INTO tutorial_features
    (feature_key, display_name_th, surface, audience, level, menu_path, ui_vocab, steps, trap_th, pain_point, why_matters_th)
VALUES

-- กลุ่ม A: ตำแหน่งโฆษณา
-- ที่มา: help/175741192481247 (Choose ad placements in Meta Ads Manager)
('placement_advantage_vs_manual', 'Advantage+ placements กับ Manual placements ต่างกันยังไง', 'ads_manager', 'beginner', 'basic',
 ARRAY['Ads Manager','+ Create','Placements','Advantage+ placements','Edit','Manual placements'],
 string_to_array($vocab$Ads Manager|+ Create|Continue|Ad sets|Placements|Advantage+ placements|Edit|Manual placements|Devices|All devices|Platforms|Publish$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เปิดหน้าสร้างโฆษณา","action_th":"ใน Ads Manager กด + Create เลือกวัตถุประสงค์แล้วกด Continue","ui_target":"+ Create"},
  {"n":2,"title_th":"หาส่วนตำแหน่งในชั้นกลาง","action_th":"ทำตามขั้นตอนจนถึงส่วน Placements ของชั้น Ad sets ค่าเริ่มต้นคือ Advantage+ placements","ui_target":"Placements"},
  {"n":3,"title_th":"สลับมาเลือกเอง","action_th":"ชี้ที่ Advantage+ placements แล้วกด Edit จากนั้นเลือก Manual placements","ui_target":"Manual placements"},
  {"n":4,"title_th":"ติ๊กออกเฉพาะที่ไม่เอา","action_th":"ใต้ Platforms ติ๊กออกที่ไม่ต้องการ ส่วน Devices เอกสารแนะนำให้คง All devices ไว้","ui_target":"Platforms"}
 ]$steps$,
 'คนเพิ่งเริ่มมักสลับเป็น Manual placements แล้วติ๊กเหลือ Facebook Feed ที่เดียวเพราะคิดว่าคุมงบได้ แต่การตัดตำแหน่งทิ้งคือการตัดโอกาสที่ระบบจะไปเจอราคาถูกกว่าให้ ยิ่งเหลือที่ให้ระบบเลือกน้อย ต้นทุนต่อผลลัพธ์ยิ่งขยับขึ้น',
 'placement_mode_choice',
 'ตั้งตำแหน่งผิดตั้งแต่วันแรก แปลว่าจ่ายแพงกว่าที่ควรทุกวันโดยไม่มีอะไรเตือน'),

-- ที่มา: help/407108559393196 (About ad placements across Meta technologies)
('placement_map_2026', 'แอดเราไปโผล่ที่ไหนได้บ้าง แผนที่ตำแหน่งโฆษณา', 'ads_manager', 'beginner', 'basic',
 ARRAY['Ads Manager','Placements','Manual placements','Platforms'],
 string_to_array($vocab$Ads Manager|Placements|Manual placements|Platforms|Facebook Feed|Instagram Feed|Threads feed|Facebook Reels|Instagram Reels|Facebook Stories|Instagram Stories|WhatsApp Status|Facebook Marketplace|Facebook search results|Audience Network$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เปิดรายการตำแหน่งทั้งหมด","action_th":"ใน Ads Manager เข้าส่วน Placements แล้วเลือก Manual placements เพื่อให้เห็นรายการเต็ม","ui_target":"Manual placements"},
  {"n":2,"title_th":"กลุ่มแรก ที่ที่คนเลื่อนดู","action_th":"ดูกลุ่มฟีด ได้แก่ Facebook Feed, Instagram Feed และ Threads feed","ui_target":"Facebook Feed"},
  {"n":3,"title_th":"กลุ่มจอเต็มแนวตั้ง","action_th":"Facebook Reels, Instagram Reels, Facebook Stories, Instagram Stories และ WhatsApp Status ทั้งหมดเป็นจอเต็มแนวตั้ง","ui_target":"Instagram Reels"},
  {"n":4,"title_th":"กลุ่มค้นหาและแอปนอก","action_th":"Facebook Marketplace กับ Facebook search results อยู่ในเครือ ส่วน Audience Network คือแอปนอกเครือ Meta","ui_target":"Audience Network"}
 ]$steps$,
 'เห็นชื่อ Threads feed กับ WhatsApp Status แล้วคิดว่าเป็นของใหม่ที่ยังไม่ต้องสน ทั้งที่สองที่นี้เป็นตำแหน่งที่แอดวิ่งได้จริงแล้ว และรูปแนวนอนที่เตรียมไว้สำหรับฟีดจะถูกครอบตัดจนอ่านไม่ออกทันทีเมื่อไปโผล่ในจอเต็มแนวตั้ง',
 'placement_landscape',
 'ไม่รู้ว่าแอดตัวเองไปโผล่ที่ไหนได้บ้าง แปลว่าเตรียมชิ้นงานผิดสัดส่วนมาตั้งแต่ต้น'),

-- ที่มา: help/1098535543548363 (View Meta ad results by platform, device and placement)
('placement_breakdown_read', 'ดูผลแยกตามตำแหน่ง ว่าเงินไปตกที่ไหน', 'ads_manager', 'beginner', 'basic',
 ARRAY['Ads Manager','Breakdown','By Delivery','Placement'],
 string_to_array($vocab$Ads Manager|Campaigns|Columns|Reports|Breakdown|By Delivery|Placement|Platform|Placement and device|Amount spent|Cost per result$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"หาเมนูแยกผล","action_th":"ใน Ads Manager หาปุ่ม Breakdown ซึ่งอยู่ระหว่าง Columns กับ Reports","ui_target":"Breakdown"},
  {"n":2,"title_th":"เลือกหมวดการแสดงผล","action_th":"กด By Delivery เพื่อดูว่าโฆษณาไปแสดงที่ไหน","ui_target":"By Delivery"},
  {"n":3,"title_th":"แยกตามตำแหน่ง","action_th":"เลือก Placement ตารางจะแตกออกเป็นแถวละตำแหน่ง","ui_target":"Placement"},
  {"n":4,"title_th":"อ่านว่าที่ไหนกินงบเปล่า","action_th":"เทียบ Amount spent กับ Cost per result ของแต่ละแถว แถวที่แพงผิดปกติคือจุดที่ต้องดู","ui_target":"Cost per result"}
 ]$steps$,
 'ตัวเลขที่แยกตามตำแหน่งเป็นค่าประมาณ ตามที่ Meta ระบุไว้เองว่า estimated จึงใช้ดูแนวโน้มว่าที่ไหนแพงผิดปกติได้ แต่คนเพิ่งเริ่มมักเห็นตัวเลขวันเดียวแล้วรีบตัดตำแหน่งนั้นทิ้ง ทั้งที่ข้อมูลยังน้อยเกินจะสรุป',
 'placement_performance_read',
 'ไม่รู้ว่างบไหลไปตำแหน่งไหน แปลว่าปรับอะไรก็เดาล้วนๆ'),

-- ที่มา: help/175741192481247 + help/407108559393196
('audience_network_choice', 'Audience Network คืออะไร ตัดทิ้งดีไหม', 'ads_manager', 'beginner', 'basic',
 ARRAY['Ads Manager','Placements','Manual placements','Audience Network','Allow limited spend to excluded placements'],
 string_to_array($vocab$Ads Manager|Placements|Manual placements|Platforms|Audience Network|Allow limited spend to excluded placements|Manage excluded placements|Publish$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"หาตำแหน่งกลุ่มแอปนอก","action_th":"ใน Placements เลือก Manual placements แล้วเลื่อนหากลุ่มที่ชื่อ Audience Network","ui_target":"Audience Network"},
  {"n":2,"title_th":"รู้ก่อนว่ามันคือที่ไหน","action_th":"Audience Network คือแอปและเว็บนอกเครือ Meta ที่รับโฆษณาของเราไปแสดงต่อ","ui_target":"Manual placements"},
  {"n":3,"title_th":"ถ้าจะตัดออก","action_th":"ติ๊กออกใต้ Platforms แล้วดูตัวเลือก Allow limited spend to excluded placements ที่โผล่ขึ้นมา","ui_target":"Allow limited spend to excluded placements"},
  {"n":4,"title_th":"คุมเป็นรายตำแหน่ง","action_th":"กด Manage excluded placements ถ้าอยากเลือกทีละตัวว่าตำแหน่งไหนยอมให้ใช้งบได้","ui_target":"Manage excluded placements"}
 ]$steps$,
 'ติ๊กตัด Audience Network ออกแล้วเปิด Allow limited spend to excluded placements ทิ้งไว้ แล้วเข้าใจว่าตัดขาดแล้ว ทั้งที่ตัวเลือกนี้ยอมให้ใช้งบได้ถึง 5 เปอร์เซ็นต์ต่อตำแหน่งที่ตัดทิ้ง เมื่อระบบคิดว่าจะช่วยให้ผลดีขึ้น งบจึงยังไหลไปที่นั่นอยู่',
 'audience_network_doubt',
 'เข้าใจผิดว่าตัดตำแหน่งออกแล้ว ทำให้อ่านรายงานผิดตามไปด้วยทั้งเดือน'),

-- กลุ่ม B: แคมเปญ
-- ที่มา: help/621956575422138 (advertising levels) + help/175741192481247 (ลำดับการสร้าง)
('campaign_create_end_to_end', 'สร้างแคมเปญแรก ตั้งแต่กดสร้างจนกด Publish', 'ads_manager', 'beginner', 'basic',
 ARRAY['Ads Manager','+ Create','Continue','Ad sets','Ads','Publish'],
 string_to_array($vocab$Ads Manager|+ Create|Continue|Campaigns|Ad sets|Ads|Awareness|Traffic|Engagement|Leads|Sales|Audience|Placements|Budget|Schedule|Next|Publish$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เริ่มที่ปุ่มเดียว","action_th":"ใน Ads Manager กด + Create เพื่อเปิดการสร้างแคมเปญใหม่","ui_target":"+ Create"},
  {"n":2,"title_th":"ชั้นบน บอกว่าอยากได้อะไร","action_th":"เลือกวัตถุประสงค์เช่น Awareness, Traffic, Engagement, Leads หรือ Sales แล้วกด Continue","ui_target":"Continue"},
  {"n":3,"title_th":"ชั้นกลาง ยิงใส่ใครที่ไหนงบเท่าไหร่","action_th":"ที่ชั้น Ad sets ตั้ง Audience, Placements, Budget และ Schedule แล้วกด Next","ui_target":"Ad sets"},
  {"n":4,"title_th":"ชั้นล่าง หน้าตาที่คนเห็น","action_th":"ที่ชั้น Ads ใส่รูปหรือวิดีโอและข้อความที่จะแสดงจริง","ui_target":"Ads"},
  {"n":5,"title_th":"ส่งเข้าตรวจ","action_th":"กด Publish แล้วแอดจะเข้าคิวตรวจก่อนเริ่มวิ่ง","ui_target":"Publish"}
 ]$steps$,
 'คนเพิ่งเริ่มมักคิดว่ากด Publish แล้วแอดวิ่งทันที แต่ทุกแอดต้องผ่านการตรวจก่อน ช่วงนั้นสถานะจะยังไม่ใช่กำลังวิ่ง การรีบไปกดแก้ซ้ำๆ ระหว่างรอ คือการส่งแอดกลับเข้าคิวตรวจใหม่ทุกครั้ง',
 'first_campaign_walkthrough',
 'กดผิดลำดับตั้งแต่แคมเปญแรก แล้วไปแก้ผิดชั้นไปตลอด'),

-- ที่มา: help/2035196646663270 (conversion location) + help/1438417719786914 (objectives)
('conversion_location_choice', 'Conversion location เลือกให้ผลลัพธ์ไปเกิดถูกที่', 'ads_manager', 'beginner', 'basic',
 ARRAY['Ads Manager','+ Create','Conversion location','Ad sets','Next'],
 string_to_array($vocab$Ads Manager|+ Create|Continue|Conversion location|Website|App|Messenger|WhatsApp|On your ad|Ad sets|Traffic|Engagement|Leads|Sales|Next$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เลือกเป้าหมายก่อน","action_th":"กด + Create แล้วเลือกวัตถุประสงค์เช่น Traffic, Engagement, Leads หรือ Sales","ui_target":"+ Create"},
  {"n":2,"title_th":"หาช่องเลือกที่เกิดผล","action_th":"ที่ชั้น Ad sets จะมี Conversion location ให้เลือกว่าผลลัพธ์จะไปเกิดที่ไหน","ui_target":"Conversion location"},
  {"n":3,"title_th":"เลือกปลายทางจริง","action_th":"เลือก Website ถ้าอยากให้ไปเว็บ เลือก Messenger หรือ WhatsApp ถ้าอยากให้ทัก เลือก On your ad ถ้าอยากได้ยอดบนตัวโฆษณาเอง","ui_target":"Website"},
  {"n":4,"title_th":"ทำต่อจนจบ","action_th":"ตั้งค่าที่เหลือของ Ad sets แล้วกด Next","ui_target":"Next"}
 ]$steps$,
 'เลือกวัตถุประสงค์ถูกแล้วแต่ลืมดู Conversion location เช่นอยากให้ทักแชทแต่ปล่อยปลายทางเป็น Website ระบบก็จะไปหาคนที่ชอบกดลิงก์แทนคนที่ชอบทัก ผลที่ได้จึงผิดประเภทตั้งแต่วันแรกทั้งที่ตั้งวัตถุประสงค์ถูก',
 'conversion_location_confusion',
 'ปลายทางผิด แปลว่าระบบไปตามหาคนผิดกลุ่มให้เราทุกบาทที่จ่าย'),

-- ที่มา: help/209669919072999 (Duplicate ad campaigns)
('campaign_duplicate', 'คัดลอกแคมเปญ แทนการสร้างใหม่ทุกครั้ง', 'ads_manager', 'beginner', 'basic',
 ARRAY['Ads Manager','Campaigns','Duplicate','Number of copies','Publish'],
 string_to_array($vocab$Ads Manager|Campaigns|Ad sets|Ads|Duplicate|Original campaign|Existing campaign|New campaign|Number of copies|Show existing reactions, comments and shares on new ads|Publish$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เลือกสิ่งที่จะคัดลอก","action_th":"ใน Ads Manager กดแท็บ Campaigns, Ad sets หรือ Ads แล้วติ๊กช่องหน้าตัวที่ต้องการ","ui_target":"Campaigns"},
  {"n":2,"title_th":"สั่งคัดลอก","action_th":"กด Duplicate ด้านบนตาราง","ui_target":"Duplicate"},
  {"n":3,"title_th":"เลือกว่าจะไปอยู่ที่ไหน","action_th":"ถ้าคัดลอก ad set หรือ ad ให้เลือก Original campaign, Existing campaign หรือ New campaign","ui_target":"Original campaign"},
  {"n":4,"title_th":"เก็บยอดมีส่วนร่วมเดิมไว้","action_th":"ตั้ง Number of copies แล้วติ๊ก Show existing reactions, comments and shares on new ads เพื่อยกไลก์และคอมเมนต์เดิมมาด้วย","ui_target":"Show existing reactions, comments and shares on new ads"},
  {"n":5,"title_th":"ตรวจแล้วส่ง","action_th":"แก้สิ่งที่ต้องการแล้วกด Publish","ui_target":"Publish"}
 ]$steps$,
 'คัดลอกแอดโดยไม่ติ๊ก Show existing reactions, comments and shares on new ads แล้วยอดไลก์กับคอมเมนต์ที่สะสมมาหายเกลี้ยง กลายเป็นแอดหน้าใหม่ที่ไม่มีใครเคยคุยด้วย ทั้งที่ของเดิมสะสมมาเป็นเดือน',
 'duplicate_vs_new',
 'สร้างใหม่ทุกครั้งแทนการคัดลอก แปลว่าทิ้งทั้งการตั้งค่าและยอดมีส่วนร่วมที่สะสมมา'),

-- ที่มา: help/172764286113080 (Turn an ad on or off) + help/316478108955072 (significant edits)
('campaign_off_vs_delete', 'ปิด กับ ลบ ต่างกันตรงไหน', 'ads_manager', 'beginner', 'basic',
 ARRAY['Ads Manager','Campaigns','Ad sets','Ads','Delivery'],
 string_to_array($vocab$Ads Manager|Campaigns|Ad sets|Ads|Delivery|Off|Delete$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เปิดชั้นที่จะจัดการ","action_th":"ใน Ads Manager เลือกแท็บ Campaigns, Ad sets หรือ Ads ที่ด้านบนจอ","ui_target":"Campaigns"},
  {"n":2,"title_th":"ปิดด้วยสวิตช์","action_th":"กดสวิตช์หน้าแถวที่ต้องการ ระบบจะหยุดส่งโฆษณาภายในไม่กี่นาที และคอลัมน์ Delivery จะขึ้นว่า Off","ui_target":"Delivery"},
  {"n":3,"title_th":"รู้ว่าปิดชั้นบนกระทบชั้นล่าง","action_th":"ปิดที่ชั้น Campaigns จะปิด Ad sets และ Ads ทุกตัวข้างในตามไปด้วย","ui_target":"Ad sets"},
  {"n":4,"title_th":"ลบคือคนละเรื่อง","action_th":"Delete ใช้เก็บกวาดของที่ไม่ใช้แล้ว ต่างจากการปิดที่ยังเก็บทุกอย่างไว้ให้กลับมาเปิดได้","ui_target":"Delete"}
 ]$steps$,
 'ปิด ad set ทิ้งไว้ยาวๆ แล้วค่อยกลับมาเปิด โดยไม่รู้ว่าการหยุดตั้งแต่ 7 วันขึ้นไปทำให้ ad set กลับเข้าสู่ช่วงเรียนรู้ใหม่ตอนเปิด ผลที่เคยนิ่งจึงเหวี่ยงอีกรอบทั้งที่ไม่ได้แก้อะไรเลย',
 'off_vs_delete',
 'กดปิดกับกดลบสลับกัน แล้วเสียข้อมูลที่ควรเก็บ หรือหยุดยาวจนต้องเรียนรู้ใหม่'),

-- ที่มา: help/1381935425400769 (Schedule an ad set in Meta Ads Manager)
('ad_schedule_dayparting', 'ตั้งให้แอดวิ่งเฉพาะช่วงเวลาที่ต้องการ', 'ads_manager', 'beginner', 'basic',
 ARRAY['Ads Manager','Ad sets','Edit','Delivery scheduling','Scheduled'],
 string_to_array($vocab$Ads Manager|Ad sets|Edit|Delivery scheduling|Scheduled|Insert ad|Review|Publish$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เลือกชุดโฆษณาที่จะตั้งเวลา","action_th":"ใน Ads Manager เปิดแท็บ Ad sets แล้วเลือกชุดที่ต้องการ","ui_target":"Ad sets"},
  {"n":2,"title_th":"เข้าโหมดแก้ไข","action_th":"กดไอคอน Edit ของชุดนั้น","ui_target":"Edit"},
  {"n":3,"title_th":"เปิดตารางเวลา","action_th":"ใต้หัวข้อ Delivery scheduling เลือก Scheduled แล้วเลือกช่วงเวลาที่อยากให้วิ่ง","ui_target":"Delivery scheduling"},
  {"n":4,"title_th":"เลือกว่าช่องไหนใช้แอดตัวไหน","action_th":"ใช้เมนู Insert ad เลือกโฆษณาที่จะแสดงในแต่ละช่วง","ui_target":"Insert ad"},
  {"n":5,"title_th":"ตรวจแล้วส่ง","action_th":"กด Review แล้วกด Publish","ui_target":"Publish"}
 ]$steps$,
 'ตั้งเวลาตามนาฬิกาของตัวเอง แล้วคิดว่าแอดจะวิ่งช่วงนั้นเป๊ะ ทั้งที่เอกสารระบุว่าโฆษณาแสดงตามเขตเวลาของคนดู ถ้ายิงข้ามประเทศ ช่วงเวลาที่ตั้งไว้จะไม่ตรงกับที่คิด และแต่ละช่วงต้องยาวอย่างน้อยหนึ่งชั่วโมง',
 'ad_scheduling_setup',
 'ปล่อยแอดวิ่ง 24 ชั่วโมงทั้งที่ลูกค้าซื้อแค่บางช่วง คือการจ่ายค่าโฆษณาให้ช่วงที่ไม่มีใครซื้อ'),

-- ที่มา: help/112167992830700 (About the learning phase) + help/316478108955072
('learning_phase_basics', 'คำว่า Learning ในคอลัมน์ Delivery แปลว่าอะไร', 'ads_manager', 'beginner', 'basic',
 ARRAY['Ads Manager','Ad sets','Delivery','Columns','Last significant edit'],
 string_to_array($vocab$Ads Manager|Ad sets|Delivery|Learning|Learning limited|Columns|Last significant edit|Results$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"ดูสถานะที่ชั้นชุดโฆษณา","action_th":"ใน Ads Manager เปิดแท็บ Ad sets แล้วดูคอลัมน์ Delivery","ui_target":"Delivery"},
  {"n":2,"title_th":"อ่านคำว่ากำลังเรียนรู้","action_th":"ถ้าขึ้นว่า Learning แปลว่าระบบยังทดลองหาวิธีส่งโฆษณาที่ดีที่สุดอยู่ ผลช่วงนี้จึงยังไม่นิ่ง","ui_target":"Learning"},
  {"n":3,"title_th":"เช็กว่าเพิ่งไปแก้อะไรไว้","action_th":"กด Columns เพิ่มคอลัมน์ Last significant edit เพื่อดูว่าแก้ครั้งใหญ่ล่าสุดเมื่อไหร่","ui_target":"Last significant edit"},
  {"n":4,"title_th":"แยกให้ออกจากอาการติดขัด","action_th":"ถ้าขึ้นว่า Learning limited แปลว่าผลลัพธ์น้อยเกินกว่าจะเรียนรู้จบ ไม่ใช่แค่กำลังเรียนรู้ตามปกติ","ui_target":"Learning limited"}
 ]$steps$,
 'เห็นผลไม่ดีในช่วง Learning แล้วรีบเข้าไปแก้กลุ่มเป้าหมายหรือเปลี่ยนรูป ซึ่งเอกสารระบุว่าเป็นการแก้ครั้งใหญ่ที่ทำให้ระบบเริ่มเรียนรู้ใหม่ตั้งแต่ต้น ยิ่งแก้บ่อยยิ่งไม่มีวันออกจากช่วงนี้ และต้นทุนต่อผลลัพธ์จะสูงค้างอยู่แบบนั้น',
 'learning_phase_meaning',
 'ไม่รู้จักช่วงเรียนรู้ แปลว่าเข้าไปแก้ผิดจังหวะจนแอดไม่มีวันเข้าที่'),

-- กลุ่ม C: ชิ้นงานกับปลายทาง
-- ที่มา: help/1393077930949041 (Create an image ad) + help/1375829326076396 (Create a carousel ad)
('ad_format_choice', 'รูปเดี่ยว วิดีโอ หรือคาร์รูเซล เลือกอันไหน', 'ads_manager', 'beginner', 'basic',
 ARRAY['Ads Manager','Ads','Ad setup','Ad creative','Carousel'],
 string_to_array($vocab$Ads Manager|Ads|Ad setup|Ad creative|Single image or video|Carousel|Set up creative|+ Add cards|Apply to all cards|Publish$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"ไปที่ชั้นล่างสุด","action_th":"ทำตามขั้นตอนจนถึงส่วน Ad setup ที่ชั้น Ads","ui_target":"Ad setup"},
  {"n":2,"title_th":"เลือกแบบรูปเดียวหรือวิดีโอเดียว","action_th":"เลือก Single image or video แล้วไปที่ Ad creative กด Set up creative เพื่อใส่ไฟล์","ui_target":"Single image or video"},
  {"n":3,"title_th":"หรือเลือกแบบเลื่อนได้","action_th":"เลือก Carousel ถ้าอยากใส่ตั้งแต่ 2 ชิ้นขึ้นไปให้คนเลื่อนดู แต่ละใบมีพาดหัวและลิงก์ของตัวเอง","ui_target":"Carousel"},
  {"n":4,"title_th":"ใส่ใบทีละใบ","action_th":"ใน Ad creative กด + Add cards เพื่อเพิ่มใบ แล้วเลือก Apply to all cards ถ้าอยากใช้ข้อความเดียวกันทุกใบ","ui_target":"+ Add cards"}
 ]$steps$,
 'เลือก Carousel เพราะคิดว่าใส่ได้เยอะกว่าย่อมดีกว่า แล้วใส่รูปสินค้าซ้ำกันหลายใบโดยไม่เปลี่ยนพาดหัว ทั้งที่จุดแข็งของแบบนี้คือแต่ละใบมีพาดหัวและลิงก์ของตัวเอง ถ้าไม่ใช้ก็เท่ากับทำแอดรูปเดียวที่คนต้องออกแรงเลื่อนเพิ่ม',
 'ad_format_choice',
 'เลือกรูปแบบชิ้นงานผิด แปลว่าเสียโอกาสเล่าของที่มีอยู่แล้วให้ครบ'),

-- ที่มา: help/682655495435254 (Aspect ratios) + help/469767027114079 (minimum image pixels)
('aspect_ratio_specs', 'ต้องเตรียมภาพสัดส่วนไหน ขนาดเท่าไหร่', 'ads_manager', 'beginner', 'basic',
 ARRAY['Ads Manager','Ads','Ad creative','Ads Guide'],
 string_to_array($vocab$Ads Manager|Ads|Ad creative|Ads Guide|1:1|4:5|9:16|1.91:1|Instagram Feed|Instagram Stories|Facebook Reels$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"รู้ว่ามีกี่สัดส่วนให้เลือก","action_th":"เอกสารของ Meta รองรับสัดส่วน 1.91:1, 1:1, 4:5 และ 9:16 เป็นหลัก แล้วแต่ตำแหน่งที่จะไปโผล่","ui_target":"Ads Guide"},
  {"n":2,"title_th":"จอเต็มแนวตั้งใช้ 9:16","action_th":"Instagram Stories และ Facebook Reels แนะนำ 9:16 เพราะเต็มจอมือถือพอดี","ui_target":"9:16"},
  {"n":3,"title_th":"ฟีดใช้แนวตั้งเตี้ยกว่า","action_th":"Instagram Feed แนะนำ 4:5 สำหรับภาพนิ่ง ซึ่งกินพื้นที่ฟีดมากกว่าแบบจัตุรัส","ui_target":"4:5"},
  {"n":4,"title_th":"อย่าให้ภาพเล็กเกินขั้นต่ำ","action_th":"ขนาดขั้นต่ำที่แนะนำคือ 1080 x 1080 พิกเซลสำหรับ 1:1 และ 1440 x 1800 พิกเซลสำหรับ 4:5","ui_target":"1:1"}
 ]$steps$,
 'เตรียมภาพแนวนอนไฟล์เดียวแล้วยิงทุกตำแหน่ง พอไปโผล่ใน Instagram Stories หรือ Facebook Reels ที่เป็นจอเต็มแนวตั้ง ภาพจะถูกครอบตัดจนข้อความบนภาพหายไปครึ่งหนึ่ง โดยที่รายงานไม่เคยบอกว่าคนเห็นภาพแหว่ง',
 'creative_size_specs',
 'ภาพผิดสัดส่วน แปลว่าจ่ายเงินให้คนเห็นงานที่ถูกครอบตัดจนอ่านไม่รู้เรื่อง'),

-- ที่มา: help/223409425500940 (Creative best practices for text in ads) + help/1375829326076396
('ad_copy_fields', 'Primary text, Headline, Description ช่องไหนคืออะไร', 'ads_manager', 'beginner', 'basic',
 ARRAY['Ads Manager','Ads','Ad creative','Primary text','Headline','Description'],
 string_to_array($vocab$Ads Manager|Ads|Ad creative|Primary text|Headline|Description|Website URL|Publish$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เปิดส่วนใส่ข้อความ","action_th":"ที่ชั้น Ads เข้าส่วน Ad creative ซึ่งรวมช่องข้อความทั้งหมดไว้","ui_target":"Ad creative"},
  {"n":2,"title_th":"ช่องข้อความหลัก","action_th":"Primary text คือข้อความที่คนอ่านก่อน เอกสารแนะนำให้ยาวไม่เกิน 1 ถึง 3 บรรทัด","ui_target":"Primary text"},
  {"n":3,"title_th":"ช่องพาดหัว","action_th":"Headline คือบรรทัดหนาใต้ภาพ ใช้บอกสิ่งที่จะได้แบบสั้นที่สุด","ui_target":"Headline"},
  {"n":4,"title_th":"ช่องเสริมที่อาจไม่ขึ้น","action_th":"Description อาจแสดงหรือไม่แสดงก็ได้แล้วแต่ตำแหน่ง จึงห้ามใส่ข้อมูลที่ขาดไม่ได้ไว้ตรงนี้","ui_target":"Description"},
  {"n":5,"title_th":"ใส่ปลายทางให้ครบ","action_th":"ใส่ Website URL แล้วกด Publish","ui_target":"Website URL"}
 ]$steps$,
 'เว้นช่องข้อความว่างไว้เพราะคิดว่าเดี๋ยวค่อยมาแก้ ทั้งที่เอกสารระบุว่าระบบอาจดึงข้อความจากเว็บที่ใส่เป็นปลายทางมาใส่ให้เอง แล้วเราแก้ไม่ได้ และถ้าเอาโพสต์เดิมจากเพจมาทำเป็นแอด ก็จะแก้ข้อความในชิ้นงานนั้นไม่ได้เลย',
 'ad_copy_field_meaning',
 'ใส่ข้อความผิดช่อง แปลว่าเรื่องสำคัญไปอยู่ในที่ที่คนอาจไม่เห็นเลย'),

-- ที่มา: help/450688792208401 (About CTA buttons) + help/621765098354751 (How to add a CTA)
('cta_button_choice', 'เลือกปุ่ม Call to action ให้ตรงกับสิ่งที่อยากให้ทำ', 'ads_manager', 'beginner', 'basic',
 ARRAY['Ads Manager','Ads','Ad creative','Call to action'],
 string_to_array($vocab$Ads Manager|Ads|Ad creative|Call to action|Learn more|Shop now|Contact us|See details|Add a destination|Website|Publish$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"หาช่องเลือกปุ่ม","action_th":"ที่ชั้น Ads ในส่วน Ad creative หาเมนู Call to action","ui_target":"Call to action"},
  {"n":2,"title_th":"เลือกให้ตรงกับเป้าหมาย","action_th":"เลือกปุ่มอย่าง Learn more, Shop now, Contact us หรือ See details ให้ตรงกับสิ่งที่อยากให้คนทำ","ui_target":"Shop now"},
  {"n":3,"title_th":"ถ้าเมนูไม่โผล่","action_th":"ถ้าใช้วัตถุประสงค์แบบสร้างการรับรู้ ต้องกด Add a destination แล้วเลือก Website ก่อน เมนูปุ่มจึงจะขึ้น","ui_target":"Add a destination"},
  {"n":4,"title_th":"ส่งงาน","action_th":"ตรวจปุ่มที่เลือกอีกครั้งแล้วกด Publish","ui_target":"Publish"}
 ]$steps$,
 'รายการปุ่มที่เลือกได้ไม่เหมือนกันในทุกวัตถุประสงค์ คนเพิ่งเริ่มจึงงงว่าทำไมปุ่มที่เห็นคนอื่นใช้ถึงไม่มีให้เลือก ทั้งที่สาเหตุคือวัตถุประสงค์คนละตัว ไม่ใช่บัญชีมีปัญหา',
 'cta_button_choice',
 'ปุ่มไม่ตรงกับสิ่งที่อยากให้ทำ แปลว่าคนกดแล้วไปเจอสิ่งที่ไม่ได้คาดไว้'),

-- ที่มา: help/1625047404403494 (Preview an ad during ad creation)
('ad_preview_before_publish', 'ดูตัวอย่างชิ้นงานก่อนกด Publish', 'ads_manager', 'beginner', 'basic',
 ARRAY['Ads Manager','+ Create','Ad creative','Ad preview','Preview on device'],
 string_to_array($vocab$Ads Manager|+ Create|Ads|Ad creative|Ad preview|Preview on device|Publish$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"สร้างให้มีของก่อน","action_th":"กด + Create แล้วใส่ข้อความกับไฟล์ในส่วน Ad creative ให้ครบก่อน","ui_target":"Ad creative"},
  {"n":2,"title_th":"เปิดหน้าต่างตัวอย่าง","action_th":"ดูที่ Ad preview ซึ่งเปิดดูได้ทุกจังหวะระหว่างสร้าง ไม่ต้องรอจนเสร็จ","ui_target":"Ad preview"},
  {"n":3,"title_th":"ไล่ดูทีละตำแหน่ง","action_th":"สลับดูตัวอย่างของแต่ละตำแหน่ง เพื่อเช็กว่าภาพไม่ถูกครอบตัดจนข้อความหาย","ui_target":"Ad preview"},
  {"n":4,"title_th":"ส่งเข้าเครื่องตัวเอง","action_th":"กด Preview on device เพื่อส่งการแจ้งเตือนไปดูบนมือถือของตัวเอง","ui_target":"Preview on device"}
 ]$steps$,
 'กด Preview on device แล้วนั่งรอการแจ้งเตือนบนมือถือ Android จนคิดว่าระบบพัง ทั้งที่เอกสารระบุว่าฟีเจอร์ส่งการแจ้งเตือนนี้ใช้ได้เฉพาะ iOS ไม่รองรับ Android และไม่รองรับบน Instagram',
 'preview_before_publish',
 'ปล่อยแอดโดยไม่เคยเห็นของจริง คือการจ่ายเงินให้คนเห็นงานที่เราเองยังไม่เคยดู'),

-- ที่มา: help/621765098354751 (CTA + conversion location) + help/1438417719786914
('destination_choice', 'ส่งคนไปเว็บ ไปแชท หรือไปไหนดี', 'ads_manager', 'beginner', 'basic',
 ARRAY['Ads Manager','Ad sets','Conversion location','Ads','Ad creative'],
 string_to_array($vocab$Ads Manager|Ad sets|Conversion location|Website|App|Messenger|WhatsApp|On your ad|Ads|Ad creative|Website URL|Publish$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"ตัดสินใจที่ชั้นกลางก่อน","action_th":"ที่ชั้น Ad sets เลือก Conversion location ว่าผลลัพธ์จะไปเกิดที่ไหน","ui_target":"Conversion location"},
  {"n":2,"title_th":"ถ้ามีเว็บของตัวเอง","action_th":"เลือก Website แล้วไปใส่ Website URL ที่ส่วน Ad creative ของชั้น Ads","ui_target":"Website"},
  {"n":3,"title_th":"ถ้าขายผ่านการทัก","action_th":"เลือก Messenger หรือ WhatsApp เพื่อให้คนกดแล้วเปิดห้องแชทกับเราทันที","ui_target":"Messenger"},
  {"n":4,"title_th":"ถ้าอยากได้ยอดบนตัวโฆษณาเอง","action_th":"เลือก On your ad เมื่อสิ่งที่ต้องการคือยอดมีส่วนร่วมหรือยอดดูวิดีโอบนโพสต์","ui_target":"On your ad"}
 ]$steps$,
 'ส่งคนไปเว็บทั้งที่หลังบ้านรับออเดอร์ทางแชทอย่างเดียว คนกดเข้าไปแล้วไม่รู้จะสั่งยังไงก็กดออก เงินที่จ่ายไปกลายเป็นค่าพาคนไปยืนหน้าประตูที่ล็อกอยู่',
 'destination_choice',
 'ปลายทางไม่ตรงกับวิธีขายจริง แปลว่าคนสนใจแล้วแต่ซื้อไม่ได้'),

-- ที่มา: help/742478679120153 (Learn about Meta Pixel)
('pixel_basics', 'พิกเซลคืออะไร คนเพิ่งเริ่มต้องมีไหม', 'events_manager', 'beginner', 'basic',
 ARRAY['Events Manager','Meta Pixel','Ads Manager','Custom Audiences'],
 string_to_array($vocab$Events Manager|Meta Pixel|Ads Manager|Custom Audiences|Website$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"รู้ก่อนว่ามันคืออะไร","action_th":"Meta Pixel คือโค้ดสั้นๆ ที่เอาไปวางบนเว็บของเรา ไม่ใช่ปุ่มใน Ads Manager","ui_target":"Meta Pixel"},
  {"n":2,"title_th":"มันบันทึกอะไร","action_th":"เมื่อมีคนทำสิ่งใดบนเว็บเช่นหยิบใส่ตะกร้าหรือสั่งซื้อ พิกเซลจะบันทึกไว้เป็นเหตุการณ์","ui_target":"Website"},
  {"n":3,"title_th":"ไปดูผลที่ไหน","action_th":"เปิด Events Manager เพื่อดูว่ามีเหตุการณ์อะไรส่งเข้ามาบ้าง","ui_target":"Events Manager"},
  {"n":4,"title_th":"เอาไปใช้ต่อยังไง","action_th":"ใน Ads Manager เอาเหตุการณ์เหล่านี้ไปสร้าง Custom Audiences เพื่อยิงซ้ำกับคนที่เคยสนใจ","ui_target":"Custom Audiences"}
 ]$steps$,
 'รับจ้างยิงแอดให้ลูกค้าแล้วเอาพิกเซลของบัญชีตัวเองไปฝังบนเว็บของลูกค้า ซึ่งเงื่อนไขเครื่องมือธุรกิจของ Meta ระบุชัดว่าห้ามวางพิกเซลบนเว็บที่เราไม่ได้เป็นเจ้าของ นี่คือความเสี่ยงระดับบัญชี ไม่ใช่แค่เรื่องมารยาท',
 'pixel_basics',
 'ไม่มีพิกเซล แปลว่ายิงแอดโดยไม่รู้เลยว่าคนที่กดเข้าไปแล้วทำอะไรต่อ'),

-- ที่มา: help/2169779963333459 (Edit an ad) + help/112167992830700 (learning phase)
('edit_running_ad', 'แก้แอดที่กำลังวิ่งอยู่ แล้วเกิดอะไรขึ้น', 'ads_manager', 'beginner', 'basic',
 ARRAY['Ads Manager','Campaigns','Edit','Publish','Close'],
 string_to_array($vocab$Ads Manager|Campaigns|Ad sets|Ads|Edit|Publish|Close|Placements|Show more settings|Learning$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เปิดชั้นที่จะแก้","action_th":"ใน Ads Manager กด Campaigns ทางซ้าย แล้วเลือกแท็บ Ad sets หรือ Ads ตามสิ่งที่จะแก้","ui_target":"Campaigns"},
  {"n":2,"title_th":"เข้าโหมดแก้ไข","action_th":"กด Edit เหนือตารางรายงานเพื่อเปิดหน้าต่างแก้ไข","ui_target":"Edit"},
  {"n":3,"title_th":"แก้ตำแหน่งต้องกดขยายก่อน","action_th":"ถ้าจะแก้ Placements ต้องกด Show more settings ก่อน แล้วชี้ที่กลุ่มตำแหน่งจึงจะกด Edit ได้","ui_target":"Show more settings"},
  {"n":4,"title_th":"เลือกว่าจะส่งเลยหรือเก็บไว้","action_th":"กด Publish เพื่อให้มีผลทันที หรือกด Close เพื่อเก็บเป็นแบบร่างไว้ส่งทีหลัง","ui_target":"Close"}
 ]$steps$,
 'แก้รูปหรือกลุ่มเป้าหมายของแอดที่กำลังไปได้ดี โดยไม่รู้ว่าทั้งสองอย่างนับเป็นการแก้ครั้งใหญ่ที่ทำให้ชุดโฆษณากลับเข้าสู่ช่วง Learning และถูกส่งกลับเข้าคิวตรวจใหม่ ผลที่เคยนิ่งจึงเหวี่ยงอีกหลายวัน',
 'edit_running_ad',
 'แก้ผิดจังหวะครั้งเดียว ทำให้ของที่กำลังไปได้ดีกลับไปเริ่มนับหนึ่ง');

COMMIT;
