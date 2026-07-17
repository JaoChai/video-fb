# Design: Question Agent — KB-free + 3-audience pain_point taxonomy

วันที่: 2026-07-17
สถานะ: approved (รอ user review ก่อนเข้า writing-plans)
Scope: แยกจาก bug fix cooldown (deploy แล้ว) — improvement รอบใหม่ ของ QuestionAgent เท่านั้น

## Motivation

คลังความรู้ (KB) บางมาก — หมวดเนื้อหาส่วนใหญ่มี **1 knowledge_chunk/หมวด** (KB รวม 40 chunks).
AI สร้างคำถามสดจาก KB (RAG) → วัตถุดิบไอเดียน้อย → หยิบ pain_point แคบซ้ำ → ชน cooldown ง่าย
(รากเดียวกับ deadlock ที่เพิ่งแก้). User ต้องการ:
1. **เลิกอิง KB** — ให้ gemini-3-5-flash คิดคำถามจาก reasoning + prompt ที่ดี
2. เจาะ **3 กลุ่มเป้าหมาย** ให้ตรง: (A) คนซื้อบัญชี+มีปัญหาบัญชี (B) คนยิงโฆษณา FB ทุกเทคนิค
   (C) คนยิงโฆษณาสายเทา

Grounding: gemini ล้วน ไม่มี KB ไม่มี Google Search — คำถามขับด้วย scenario/pain-point ไม่ใช่
ข้อเท็จจริงสด (Fable แนะนำ; ความสดจัดการด้วย temporal-anchoring แบบ evergreen)

Model: `gemini-3-5-flash` ตั้งไว้แล้วใน `agent_configs` (agent_name='question') — ไม่เปลี่ยน

## Architecture (data/prompt เป็นหลัก, โค้ดแตะ 1 จุด)

1. **แกนหมุน = 3 กลุ่มเป้าหมาย (แทน 10 หมวด)**
   `topic_categories` (cols: enabled, weight, angle_instruction, category_name, display_name, id, created_at)
   - **ปิด** 10 แถวเดิม (`enabled=false` — ไม่ลบ เพื่อ rollback)
   - **เพิ่ม 3 แถว**: `account-buyer`, `performance-advertiser`, `grey-operator`
   - `angle_instruction` ของแต่ละแถว = persona definition + เมนู pain_point ของกลุ่มนั้น
     → ไหลเข้า prompt ผ่าน `{{.CategoryAngle}}` (มีอยู่แล้ว)
   - rotation เดิม `PickNextExclude` (least-used 7d, exclude today) หมุน 3 กลุ่ม/วัน กลุ่มละ 1 คลิป

2. **เมนู pain_point ฝังใน prompt (data-driven ผ่าน angle_instruction)** — AI เลือกจากเมนู ไม่คิดเอง
   (ต้นตอ collapse). 54 tags รวม → cooldown 5 วันมีทางหนี 16-20 ทาง/กลุ่ม ไม่มีวันตัน

3. **prompt_template rewrite** (JSON output shape เดิมเป๊ะ — กัน regression 052): specificity
   checklist + curiosity gap + scenario dice + burn-the-obvious + frame rotation + guardrail

4. **ตัด KB**: ลบ branch `rag.Search` (non-news) ใน `internal/agent/question.go`. คง news format
   (ใช้ ResearchAgent live web — ไม่ใช่ KB). `{{.RAGContext}}` เปลี่ยนเป็น conditional
   (`{{if .RAGContext}}...{{end}}`) → news ยัง inject research, non-news เว้นว่าง

5. **คง machinery**: cooldown/dedup/retry (เพิ่งแก้ commit 02778b3), archetype/role/format rotation,
   JSON struct `GeneratedQuestion{question, questioner_name, category, pain_point}` — ไม่แตะ

## Persona definitions (ลง angle_instruction)

### A. account-buyer — "The Burned Buyer"
คนที่เสียเงินไปแล้ว (บัญชีโดนแบน/ยืนยันตัวตนค้าง/คนขายหาย) วิตก ไม่ไว้ใจ ถูกเวลาบีบ (ร้าน/แคมเปญ
ตายระหว่างบัญชีล่ม). ศัพท์: บัญชีโดนปิด, ยืนยันตัวตน, BM, อุทธรณ์, อุ่นบัญชี, วงเงิน, checkpoint.
เดิมพัน: sunk cost ค่าบัญชี + รายได้รั่วทุกวันที่แอดดับ อยากรู้ว่า *เกิดอะไรขึ้นจริง* และ *แก้ได้ไหม*

### B. performance-advertiser — "The Performance Operator"
media buyer/เจ้าของแบรนด์ยิงงบจริงทุกวัน (฿1,000–100,000+/วัน) ขับด้วย *การควบคุม*: เลขที่ไม่ตรง,
algorithm ที่เอาแน่ไม่ได้, ผลที่ตายโดยไม่มีเหตุผล. ศัพท์: ROAS, CPM, learning phase, CBO/ABO,
CAPI, pixel, ASC/Advantage+, attribution, broad, LAL. เดิมพัน: margin — รู้พื้นฐานแล้ว อยากรู้กลไกหลัง black box

### C. grey-operator — "The Grey Operator"
รัน vertical จำกัด/นโยบายหมิ่นเหม่ หรือโครงสร้างหลายบัญชีเชิงรุก ขับด้วย *เศรษฐศาสตร์การอยู่รอด*:
บัญชีคือของสิ้นเปลือง แบนคือ cost line เกมคืออยู่หน้า detection. ศัพท์: ฟาร์มบัญชี, BM หลุด, โดนกวาด,
ban wave, asset ลาม, บัญชี agency, ครีเอทีฟหมิ่นเหม่. เดิมพัน: ทั้ง operation ตายใน 48 ชม.
**Boundary: คำถามอยู่ระดับความเสี่ยง/กลยุทธ์/กลไก ไม่ใช่ how-to ทำผิดกฎหมาย**

## Pain-point taxonomy (54 tags — ลง angle_instruction เป็นเมนู)

กติกา tag: ระบุ**กลไก/จังหวะเฉพาะ** ไม่ใช่หมวดกว้าง (`account_banned` กว้างไป = 1 tag = 1 cooldown =
ตัน; `ban_after_budget_jump` vs `ban_on_first_ad` vs `ban_wave_collateral` = 3 วันคอนเทนต์ต่างกัน)

### A. account-buyer (18)
`ban_after_budget_jump` บัญชีปกติ พอขยับงบก้าวกระโดดแล้วโดนปิด · `ban_on_first_ad` ซื้อมาโดนแบนตั้งแต่ ad แรก · `warmup_ritual_myth` สูตรอุ่นบัญชีอันไหนจริง/ความเชื่อ · `appeal_bot_rejection` อุทธรณ์โดน reject ใน 10 นาที = บอท · `identity_verification_loop` ยืนยันตัวตนวนซ้ำ · `business_verification_stuck` BM verify ค้างทั้งที่เอกสารถูก · `seller_clawback_risk` เจ้าของเดิมดึงบัญชีคืนได้ไหม · `spending_limit_frozen` วงเงินไม่ขยับ · `payment_method_flag` ผูกบัตรแล้วโดนธง · `checkpoint_lock_2fa` ติด checkpoint/2FA เจ้าของเดิม · `asset_contamination_spread` เพจ/pixel ประวัติเสียลามบัญชีใหม่ · `account_age_vs_trust` อายุบัญชีสำคัญจริงไหม · `agency_vs_farmed_account` บัญชี agency vs ฟาร์ม ต่างจริงตรงไหน · `pre_purchase_inspection` เช็คอะไรก่อนโอนเงินซื้อ · `timezone_currency_mismatch` บัญชีโซนเวลา/สกุลเงินนอก · `page_restriction_ripple` เพจโดนจำกัดทั้งที่บัญชีปกติ · `recovered_account_trust_reset` บัญชีกู้คืน trust เท่าเดิมไหม · `disabled_balance_stuck` บัญชีปิดทั้งที่มีเงินค้าง

### B. performance-advertiser (20)
`scaling_roas_collapse` เพิ่มงบ ROAS พัง ลดกลับไม่ฟื้น · `learning_phase_stuck` adset ไม่ออก learning · `cpm_spike_no_change` CPM พุ่งทั้งที่ไม่แตะอะไร · `attribution_number_mismatch` เลข Ads Manager ไม่ตรงหลังบ้าน · `capi_dedup_confusion` CAPI event ซ้ำ/หาย · `asc_budget_hijack` Advantage+ เทงบใส่ลูกค้าเก่า · `creative_fatigue_timing` รู้ได้ไงครีเอทีฟตายจริง · `cbo_abo_when` CBO vs ABO เงื่อนไขไหน · `audience_overlap_self_bid` adset แย่ง auction กันเอง · `broad_vs_interest_2026` broad ชนะ interest จริงไหม · `frequency_burnout_threshold` frequency เท่าไหร่ = burn · `retarget_pool_shrunk` pool retarget เล็กลงหลัง signal loss · `lead_quality_garbage` ลีดถูกแต่ขยะ · `budget_underspend` ตั้งงบแล้วระบบไม่ใช้ · `duplicate_vs_new_adset` duplicate vs สร้างใหม่ ผลต่าง · `creative_test_structure` โครงเทสครีเอทีฟไม่เผางบ · `seasonal_auction_pressure` Q4/เทศกาล CPM แพง · `post_ios_measurement` วัดผลยุค signal หาย · `hook_dropoff_3s` คน swipe ผ่านใน 3 วิ อ่าน metric ไหน · `new_account_performance_dip` ย้ายบัญชีใหม่ performance ตก

### C. grey-operator (16)
`cross_account_linking` แยกทุกอย่างแล้วบัญชียังลามกัน FB link จากอะไร · `ban_wave_survival` โดนกวาดเป็นรอบ ใครรอด/โดน · `creative_policy_scan` ระบบ scan ครีเอทีฟจับอะไร · `restricted_vertical_framing` สินค้าหมิ่นเหม่ เส้นแบ่ง ad ผ่าน/ไม่ผ่าน · `multi_bm_architecture` โครงสร้าง BM/เพจ/บัญชีหลายชุด แยกยังไงไม่ให้ลาม · `scaling_velocity_ceiling` สเกลเร็วแค่ไหนระบบธง · `circumvention_flag_trigger` โดนข้อหา circumventing ทั้งที่คิดว่าไม่ได้ทำ · `cloaking_risk_reality` ความเสี่ยงจริงของ cloaking โดนจับจากอะไร · `asset_backup_strategy` สำรองเพจ/pixel/audience ก่อนบัญชีตาย · `creative_reupload_detection` รูป/วิดีโอที่เคย reject ระบบจำได้ไง · `account_burn_economics` ต้นทุนบัญชีตายเป็น cost line คำนวณจุดคุ้ม · `personal_profile_exposure` ใช้โปรไฟล์จริงเสี่ยงลามถึงตัวแค่ไหน · `review_queue_purgatory` ad ติด review นานผิดปกติ = watchlist? · `agency_account_grey_line` บัญชี agency ใช้กับ vertical เทาได้แค่ไหน · `banned_asset_data_loss` audience/data ในบัญชีแบน กู้อะไรได้ · `landing_page_flag` ad ผ่านแต่ landing โดน ระบบ scan LP ลึกแค่ไหน

## prompt_template ใหม่ (โครง — JSON shape เดิม)

คงตัวแปรที่มี: `{{.Count}} {{.Category}} {{.CategoryAngle}} {{.FormatInstruction}}
{{.ArchetypeInstruction}} {{.RoleInstruction}} {{.AudiencePersona}} {{.PreviousTopics}}
{{.PreviousNames}} {{.TopicStats}}`. เปลี่ยน `{{.RAGContext}}` เป็น conditional.

โครงเนื้อหา prompt (รายละเอียดถ้อยคำเต็มให้ writing-plans/implementer ร่างตาม outline นี้):
1. บทบาท + งาน: สร้างคำถาม insider {{.Count}} ข้อ เสียงคนวงใน
2. กลุ่มเป้าหมาย + เมนู: `{{.Category}}` + `{{.CategoryAngle}}` (persona + เมนู pain_point)
3. format/archetype/role เดิม
4. **เลือก pain_point จากเมนู**: แต่ละข้อในชุดต้อง pain_point ต่างกัน, ห้ามเลือกตัวแรก/generic ที่สุด,
   ห้ามใช้ที่ติด cooldown (retry loop เติม avoid-list ให้อยู่แล้ว)
5. **specificity (hard)**: ตัวเลขจริง ≥2 (งบ ฿/วัน, ระยะเวลา, metric) + สิ่งที่ลองแล้ว 1 อย่างที่ไม่เวิร์ค
   + บริบทธุรกิจ (สุ่ม vertical) + ศัพท์ insider ≥1 (ไม่อธิบาย) + ห้ามคำถาม google ได้
6. **curiosity gap**: ใส่ความย้อนแย้ง (ทำถูกแต่ผลตรงข้าม / เลข 2 แหล่งไม่ตรง / สิ่งที่ทุกคนสอนกลับพัง)
7. **scenario dice**: สุ่ม vertical × ระดับงบ × ช่วงชีวิต(ก่อนซื้อ→setup→อุ่น→สเกล→วิกฤต→กู้คืน) × FB surface
8. **burn-the-obvious**: สั่งให้คิดคำถามโหลๆ 6 ข้อในใจก่อนแล้วหลีกเลี่ยง (instruction — **ไม่เพิ่ม field ใน JSON**)
9. **frame rotation**: หมุนกรอบ ("ทำไม X ทั้งที่ Y" / "A vs B อันไหนจริง" / "มืออาชีพทำยังไง" /
   "เลขเท่าไหร่ผิดปกติ" / "เช็คยังไงก่อนสาย" / "เบื้องหลังตอน X"), ห้ามซ้ำกรอบในชุดเดียว
10. **temporal anchoring**: ผูกกับ "การเปลี่ยนแปลง" แบบ evergreen (หลังอัปเดต/ช่วง ban wave/ก่อน Q4) —
    ห้ามระบุวันที่ที่จะเก่า
11. anti-repeat: `{{.PreviousTopics}}` `{{.PreviousNames}}`
12. **guardrail (สำคัญ — แทนกฎ blanket เดิม)**: คำถามสะท้อน pain กลุ่มสายเทาได้ (บัญชีถูกแบน, หลายบัญชี,
    vertical จำกัด, detection) และคุยระดับ **ความเสี่ยง/กลไก/การป้องกัน** ได้ — แต่ **ห้าม** เป็น how-to
    ปฏิบัติการทำผิดกฎหมาย: fraud, ขโมยตัวตน/บัตร, สแกมทำร้ายผู้บริโภค, หรือ step-by-step หลบ KYC/ปลอมเอกสาร
13. JSON output (เดิมเป๊ะ): array ของ `{question(≤120 ตัวอักษร), questioner_name(ไทย,สมมุติ),
    category, pain_point(snake_case)}`

ตัวอย่างคุณภาพเป้าหมาย (ไทย) — ใส่ใน prompt 1-2 ตัวอย่าง/กลุ่มเพื่อ anchor:
- A: "ซื้อบัญชีมา อุ่นงบวันละ 300 อยู่ 5 วันตามสูตร พอขยับ 1,500 โดนปิดใน 2 ชม. — ผมอุ่นผิด หรือบัญชีมีตำหนิตั้งแต่ก่อนซื้อ ดูยังไงแต่แรก?"
- B: "งบ 3,000/วัน ROAS 4 นิ่งทั้งเดือน เพิ่มเป็น 6,000 ร่วงเหลือ 1.8 ใน 3 วัน ลดกลับก็ไม่ฟื้น — ทำไม algorithm จำค่าเดิมไม่ได้ มืออาชีพสเกลยังไง?"
- C: "รัน 4 บัญชี แยก BM แยกเพจ แยกครีเอทีฟหมด บัญชีนึงแบนแล้วอีก 3 ตามใน 48 ชม. — FB link เราจากตรงไหน?"

## Code change (1 จุด)

`internal/agent/question.go` — path non-news (else branch ที่เรียก `a.rag.Search`) ลบออก →
`ragContext` ว่างสำหรับ non-news. คง path news (ResearchAgent) เดิม. `renderTemplate` เดิมยังส่ง
`RAGContext` (ว่าง) — prompt ใหม่ห่อ `{{if .RAGContext}}`. ไม่แตะ struct/dedup/cooldown/retry.

## Testing

- migration apply (schema_migrations gate) — verify 3 แถวใหม่ enabled, 10 เดิม disabled
- `go build ./... && go test ./internal/agent/` เขียว
- eyeball: trigger produce 3 คลิป → คำถามแต่ละคลิปตรง persona กลุ่มนั้น, เจาะจง (ตัวเลข+สิ่งที่ลองแล้ว),
  pain_point ต่างกัน 3 ตัว, ไม่มีเนื้อหา how-to ผิดกฎหมาย
- ดู log ไม่มี "Generated 0 questions"

## Rollback

- revert migration + revert โค้ด question.go (KB search กลับมา). 10 หมวดเดิมยังอยู่ (disabled) →
  set enabled=true คืน + ปิด 3 แถวใหม่ ได้ทันทีโดยไม่ deploy

## Out of scope (YAGNI)

- ไม่แตะ script/scene/critic agents (คนละ prompt)
- ไม่ลบ KB tables (แค่เลิกใช้ใน question flow; agents อื่น/news ยังใช้ได้)
- ไม่เพิ่ม Google Search grounding (gemini ล้วนพอ)
- ไม่แตะ category rotation logic / cooldown / dedup (เพิ่งแก้ ทำงานดี)
- persona/asker micro-casting เป็น instruction ใน prompt ไม่ทำ DB rotation เพิ่ม
