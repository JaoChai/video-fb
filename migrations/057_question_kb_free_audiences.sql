-- 057 Question agent: KB-free + 3 audience groups
-- gemini-3-5-flash คิดคำถามจากเมนู pain_point ที่ฝังใน angle_instruction (ไม่อิง KB)
-- renderTemplate = string-replace (custom) NOT text/template — prompt ใช้ได้แค่ {{.Field}} ห้าม {{if}}
-- Re-runnable ในไฟล์ (DELETE ก่อน INSERT); schema_migrations gate auto-run ตอน deploy
-- หุ้ม BEGIN/COMMIT ให้ atomic: RunMigrations ทำ pool.Exec ไฟล์เดียวไม่หุ้ม transaction
-- ถ้าไม่หุ้ม statement กลางล้มจะทิ้ง prod ไว้ครึ่งๆ (หมวดถูก disable แต่ 3 แถวใหม่ยังไม่ insert)
BEGIN;

-- 1) ปิดหมวดเดิมทั้งหมด (ไม่ลบ เพื่อ rollback)
UPDATE topic_categories SET enabled = false
WHERE category_name NOT IN ('account-buyer','performance-advertiser','grey-operator');

-- 2) ใส่ 3 กลุ่มเป้าหมาย (id/created_at/enabled ใช้ default)
DELETE FROM topic_categories WHERE category_name IN ('account-buyer','performance-advertiser','grey-operator');
INSERT INTO topic_categories (category_name, display_name, angle_instruction, weight) VALUES
('account-buyer', 'คนซื้อบัญชี + ปัญหาบัญชี', $ab$Persona: The Burned Buyer — คนที่เสียเงินไปแล้วกับบัญชีที่ซื้อมา (โดนแบน/ยืนยันตัวตนค้าง/คนขายหาย) วิตก ไม่ไว้ใจ ถูกเวลาบีบเพราะร้าน/แคมเปญตายระหว่างบัญชีล่ม อยากรู้ว่าเกิดอะไรขึ้นจริงและแก้ได้ไหม ศัพท์: บัญชีโดนปิด, ยืนยันตัวตน, BM, อุทธรณ์, อุ่นบัญชี, วงเงิน, checkpoint

เมนู pain_point (เลือกค่า pain_point จากนี้เท่านั้น):
- ban_after_budget_jump: ปกติดี พอขยับงบก้าวกระโดดแล้วโดนปิด
- ban_on_first_ad: ซื้อมาโดนแบนตั้งแต่ ad แรก
- warmup_ritual_myth: สูตรอุ่นบัญชีอันไหนจริง อันไหนความเชื่อ
- appeal_bot_rejection: อุทธรณ์โดน reject ใน 10 นาที = บอทตอบ
- identity_verification_loop: ยืนยันตัวตนวนซ้ำ ส่งบัตรกี่รอบไม่ผ่าน
- business_verification_stuck: BM verify ค้างทั้งที่เอกสารถูก
- seller_clawback_risk: เจ้าของเดิมดึงบัญชี/BM คืนได้ไหม เช็คยังไง
- spending_limit_frozen: วงเงินใช้จ่ายไม่ขยับทั้งที่จ่ายตรง
- payment_method_flag: ผูกบัตรแล้วโดนธง บัตรไทยกับบัญชีนอก
- checkpoint_lock_2fa: ติด checkpoint/2FA ของเจ้าของเดิม เข้าไม่ได้
- asset_contamination_spread: เพจ/pixel/โดเมนประวัติเสียลามบัญชีใหม่
- account_age_vs_trust: อายุบัญชีสำคัญจริงไหม หรือ trust จากพฤติกรรม
- agency_vs_farmed_account: บัญชี agency กับฟาร์ม ต่างจริงตรงไหน คุ้มไหม
- pre_purchase_inspection: เช็คอะไรก่อนโอนเงินซื้อบัญชี
- timezone_currency_mismatch: บัญชีโซนเวลา/สกุลเงินนอก ยิงตลาดไทยมีผลอะไร
- page_restriction_ripple: เพจโดนจำกัดทั้งที่บัญชีโฆษณาปกติ
- recovered_account_trust_reset: บัญชีกู้คืนมา trust เท่าเดิมไหม
- disabled_balance_stuck: บัญชีปิดทั้งที่มีเงินค้าง เกิดอะไรกับเงิน$ab$, 1),
('performance-advertiser', 'คนยิงแอดจริงจัง', $pa$Persona: The Performance Operator — media buyer/เจ้าของแบรนด์ยิงงบจริงทุกวัน (1,000–100,000+ บาท/วัน) ขับด้วยการควบคุม: เลขไม่ตรง algorithm เอาแน่ไม่ได้ ผลตายไม่มีเหตุผล รู้พื้นฐานแล้ว อยากรู้กลไกหลัง black box ศัพท์: ROAS, CPM, learning phase, CBO/ABO, CAPI, pixel, ASC/Advantage+, attribution, broad, LAL

เมนู pain_point (เลือกค่า pain_point จากนี้เท่านั้น):
- scaling_roas_collapse: เพิ่มงบ ROAS พัง ลดกลับไม่ฟื้น
- learning_phase_stuck: adset ไม่ออก learning ทั้งที่ conversion พอ
- cpm_spike_no_change: CPM พุ่งทั้งที่ไม่ได้แตะอะไร
- attribution_number_mismatch: เลข Ads Manager ไม่ตรงหลังบ้าน
- capi_dedup_confusion: ต่อ CAPI แล้ว event ซ้ำ/หาย
- asc_budget_hijack: Advantage+ เทงบใส่ลูกค้าเก่า
- creative_fatigue_timing: รู้ได้ไงครีเอทีฟตายจริง ไม่ใช่แค่วันแย่
- cbo_abo_when: CBO vs ABO เงื่อนไขไหนใช้อะไร
- audience_overlap_self_bid: หลาย adset แย่ง auction กันเอง
- broad_vs_interest_2026: broad ชนะ interest จริงไหมยุคนี้
- frequency_burnout_threshold: frequency เท่าไหร่ถึงเรียก burn
- retarget_pool_shrunk: pool retarget เล็กลงหลัง signal loss
- lead_quality_garbage: ลีดถูกแต่ขยะ optimize ยังไง
- budget_underspend: ตั้งงบแล้วระบบไม่ยอมใช้
- duplicate_vs_new_adset: duplicate vs สร้างใหม่ ผลต่างกัน
- creative_test_structure: โครงเทสครีเอทีฟไม่เผางบ
- seasonal_auction_pressure: Q4/เทศกาล CPM แพง สู้หรือหลบ
- post_ios_measurement: วัดผลจริงยุค signal หาย
- hook_dropoff_3s: คน swipe ผ่านใน 3 วิ อ่าน metric ไหน
- new_account_performance_dip: ย้ายบัญชีใหม่ performance ตก$pa$, 1),
('grey-operator', 'คนยิงสายเทา', $go$Persona: The Grey Operator — รัน vertical จำกัด/นโยบายหมิ่นเหม่ หรือโครงสร้างหลายบัญชีเชิงรุก ขับด้วยเศรษฐศาสตร์การอยู่รอด: บัญชีคือของสิ้นเปลือง แบนคือ cost line เกมคืออยู่หน้า detection ศัพท์: ฟาร์มบัญชี, BM หลุด, โดนกวาด, ban wave, asset ลาม, บัญชี agency, ครีเอทีฟหมิ่นเหม่
BOUNDARY: คำถามอยู่ระดับความเสี่ยง/กลยุทธ์/กลไก/การป้องกัน ห้ามเป็น how-to ทำผิดกฎหมาย

เมนู pain_point (เลือกค่า pain_point จากนี้เท่านั้น):
- cross_account_linking: แยกทุกอย่างแล้วบัญชียังลามกัน FB link จากอะไร
- ban_wave_survival: โดนกวาดเป็นรอบ ใครรอด ใครโดน
- creative_policy_scan: ระบบ scan ครีเอทีฟจับอะไรแน่
- restricted_vertical_framing: สินค้าหมิ่นเหม่ เส้นแบ่ง ad ผ่าน/ไม่ผ่าน
- multi_bm_architecture: โครงสร้าง BM/เพจ/บัญชีหลายชุด แยกไม่ให้ลาม
- scaling_velocity_ceiling: สเกลเร็วแค่ไหนระบบธง เพดานปลอดภัย
- circumvention_flag_trigger: โดนข้อหา circumventing ทั้งที่คิดว่าไม่ได้ทำ
- cloaking_risk_reality: ความเสี่ยงจริงของ cloaking โดนจับจากอะไร
- asset_backup_strategy: สำรองเพจ/pixel/audience ก่อนบัญชีตาย
- creative_reupload_detection: รูป/วิดีโอที่เคย reject ระบบจำได้ไง
- account_burn_economics: ต้นทุนบัญชีตายเป็น cost line คำนวณจุดคุ้ม
- personal_profile_exposure: ใช้โปรไฟล์จริงเสี่ยงลามถึงตัวแค่ไหน
- review_queue_purgatory: ad ติด review นานผิดปกติ = watchlist?
- agency_account_grey_line: บัญชี agency ใช้กับ vertical เทาได้แค่ไหน
- banned_asset_data_loss: audience/data ในบัญชีแบน กู้อะไรได้
- landing_page_flag: ad ผ่านแต่ landing โดน ระบบ scan LP ลึกแค่ไหน$go$, 1);

-- 3) rewrite prompt_template ของ question (bare {{.RAGContext}} — non-news = ว่าง)
UPDATE agent_configs SET prompt_template = $prompt$
คุณคือผู้เชี่ยวชาญวงในด้านบัญชีโฆษณา Facebook และการยิงแอดของ Ads Vance
สร้างคำถาม {{.Count}} ข้อ (คำถามจาก "ผู้ชม" ที่จะนำไปทำคลิป Q&A) ให้ตรงกลุ่มเป้าหมายและ pain จริง

กลุ่มเป้าหมายวันนี้: {{.Category}}
{{.CategoryAngle}}

รูปแบบเนื้อหา (format): {{.FormatInstruction}}
รูปหัวข้อ/hook (archetype): {{.ArchetypeInstruction}}
บทบาทคลิป: {{.RoleInstruction}}
มุมผู้ถาม (สุ่มให้หลากหลาย): {{.AudiencePersona}}

{{.RAGContext}}

หัวข้อที่เคยทำแล้ว (ห้ามซ้ำมุม/เนื้อหา):
{{.PreviousTopics}}

ชื่อผู้ปรึกษาที่เคยใช้ (ห้ามซ้ำชื่อ):
{{.PreviousNames}}

{{.TopicStats}}

วิธีเลือกหัวข้อ (pain_point):
- เลือก pain_point จาก "เมนู pain_point" ของกลุ่มเป้าหมายด้านบนเท่านั้น (ค่า pain_point = snake_case ตามเมนู)
- แต่ละข้อในชุดต้องใช้ pain_point คนละตัว ห้ามซ้ำ
- อย่าเลือกตัวแรกของเมนูหรือตัวที่ทั่วไปที่สุด เลือกให้กระจาย
- ก่อนเขียน ให้คิดในใจว่าคำถามโหลๆ ที่คนทำคอนเทนต์ทั่วไปจะถามเรื่องนี้ 6 ข้อ แล้วหลีกเลี่ยงทั้งหมด เขียนจากมุมที่ไม่อยู่ใน 6 ข้อนั้น

กติกาความเจาะจง (บังคับทุกข้อ):
- ตัวเลขจริงอย่างน้อย 2 ตัว (งบ บาท/วัน, ระยะเวลา, หรือ metric เช่น ROAS/CPM/จำนวนรอบอุทธรณ์)
- ระบุสิ่งที่ลองแล้วและไม่เวิร์ค 1 อย่าง
- ใส่บริบทธุรกิจ (สุ่มประเภทสินค้า เช่น ครีม/คอร์สออนไลน์/dropship/คลินิก/ร้านอาหาร/อสังหา/ประกัน)
- ใช้ศัพท์วงในอย่างน้อย 1 คำแบบคนใช้จริง ไม่ต้องอธิบายศัพท์
- ห้ามคำถามกว้างที่ google หาได้ เช่น "ยิงแอดยังไงให้ปัง"

ทำให้คนอยากดู (curiosity gap):
- ใส่ความย้อนแย้งที่ต้องให้คนวงในเฉลย เช่น ทำถูกทุกอย่างแต่ผลตรงข้าม / ตัวเลข 2 แหล่งไม่ตรงกัน / สิ่งที่ทุกคนสอนกลับทำให้พัง
- หมุนกรอบคำถามให้ต่างกันในแต่ละข้อ: "ทำไม X ทั้งที่ Y" / "A vs B อันไหนจริง" / "มืออาชีพทำยังไง" / "เลขเท่าไหร่ถึงผิดปกติ" / "เช็คยังไงก่อนสายเกินไป" / "เบื้องหลังตอนที่ X เกิดอะไร"
- ผูกกับการเปลี่ยนแปลงได้แต่ห้ามระบุวันที่จริง เช่น หลังอัปเดตล่าสุด, ช่วง ban wave, ก่อนช่วงเทศกาล

กติกาความปลอดภัย (สำคัญ):
- คำถามสะท้อน pain ของคนยิงสายเทาได้ (บัญชีโดนแบน, รันหลายบัญชี, vertical จำกัด, การตรวจจับ) และพูดระดับความเสี่ยง/กลไก/การป้องกันเชิงโครงสร้างได้
- ห้ามคำถามที่ขอวิธีลงมือทำสิ่งผิดกฎหมาย: ฉ้อโกง, ขโมยตัวตน/บัตร, สแกมทำร้ายผู้บริโภค, หรือขั้นตอนปลอมเอกสาร/หลบ KYC แบบ step-by-step

ตอบกลับเป็น JSON array ของ object แต่ละข้อ:
- "question": คำถาม/หัวข้อคลิปภาษาไทย เจาะจงครบรายละเอียด ประมาณ 120-220 ตัวอักษร
- "questioner_name": ชื่อคนถามภาษาไทย (สมมุติ)
- "category": ใส่ค่าเดียวกับกลุ่มเป้าหมายด้านบน
- "pain_point": pain_point ภาษาอังกฤษ snake_case ตามเมนู
$prompt$ WHERE agent_name = 'question';

-- 4) asker micro-cast (orthogonal ต่อ audience group) — เพิ่มความหลากหลายมุมผู้ถาม
UPDATE settings SET value = $personas$["มือใหม่เพิ่งซื้อบัญชีใบแรก","เจ้าของแบรนด์ที่จ้าง agency ยิงให้","media buyer รันให้ลูกค้าหลายเจ้า","dropshipper ยิงเอง","คนกลับมายิงใหม่หลังเคยโดนแบน"]$personas$ WHERE key = 'audience_personas';

COMMIT;
