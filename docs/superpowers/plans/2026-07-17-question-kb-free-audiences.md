# Question Agent KB-free + 3-Audience Taxonomy — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** เลิกอิง KB ใน QuestionAgent — ให้ gemini-3-5-flash คิดคำถามจากเมนู pain_point 54 tags ของ 3 กลุ่มเป้าหมาย ที่ฝังใน prompt ทำให้คำถามตรงกลุ่ม หลากหลาย ไม่ตัน cooldown

**Architecture:** เปลี่ยน data + prompt เป็นหลัก (migration 057) + ตัด RAG branch ใน question.go 1 จุด. 3 กลุ่มเป้าหมายเป็นแถวใน `topic_categories` (เมนู pain_point อยู่ใน `angle_instruction` → เข้า prompt ผ่าน `{{.CategoryAngle}}`). คง cooldown/dedup/retry เดิม. JSON output shape เดิมเป๊ะ

**Tech Stack:** Go, pgx, Neon Postgres, gemini-3-5-flash ผ่าน KieLLMClient

## Global Constraints

- `renderTemplate` (internal/agent/template.go) = **string-replace เอง ไม่ใช่ text/template** → ห้ามใช้ `{{if}}`/conditional ใน prompt_template; ใช้ได้แค่ `{{.FieldName}}` ที่ตรงชื่อ field ใน `QuestionTemplateData`
- JSON output shape เดิมเป๊ะ: array ของ `{question, questioner_name, category, pain_point}` (กัน regression 052 — narration หาย)
- migration: dollar-quote ข้อความยาว, re-runnable ในไฟล์ (DELETE ก่อน INSERT), schema_migrations gate auto-run ตอน deploy
- guardrail สายเทา: คุยความเสี่ยง/กลไก/ป้องกันได้ แต่ห้าม how-to ผิดกฎหมาย (fraud/ขโมยตัวตน-บัตร/สแกมผู้บริโภค/step หลบ KYC-ปลอมเอกสาร)
- model `gemini-3-5-flash` ตั้งไว้แล้ว — ห้ามเปลี่ยน
- taxonomy 54 tags + persona ตาม spec `docs/superpowers/specs/2026-07-17-question-kb-free-audiences-design.md` (source of truth ของถ้อยคำ tag)

---

### Task 1: Migration 057 — 3 audience categories + prompt rewrite

**Files:**
- Create: `migrations/057_question_kb_free_audiences.sql`

**Interfaces:**
- Produces: 3 แถวใน `topic_categories` (`account-buyer`, `performance-advertiser`, `grey-operator`), prompt_template ใหม่ของ agent `question`, `settings.audience_personas` ชุด asker micro-cast

- [ ] **Step 1: สร้างไฟล์ migration** ด้วยเนื้อหานี้ทั้งหมด (verbatim)

```sql
-- 057 Question agent: KB-free + 3 audience groups
-- gemini-3-5-flash คิดคำถามจากเมนู pain_point ที่ฝังใน angle_instruction (ไม่อิง KB)
-- renderTemplate = string-replace (custom) NOT text/template — prompt ใช้ได้แค่ {{.Field}} ห้าม {{if}}
-- Re-runnable ในไฟล์ (DELETE ก่อน INSERT); schema_migrations gate auto-run ตอน deploy

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
```

- [ ] **Step 2: ตรวจ dollar-quote บาลานซ์** — ทุก `$ab$ $pa$ $go$ $prompt$ $personas$` ต้องเปิด-ปิดคู่ครบ, ไม่มี `$` เดี่ยวหลุดในเนื้อหา

Run: `grep -oE '\$[a-z]+\$' migrations/057_question_kb_free_audiences.sql | sort | uniq -c`
Expected: แต่ละ tag ปรากฏจำนวน**คู่** (2, 2, 2, 2, 2)

- [ ] **Step 3: Commit**

```bash
git add migrations/057_question_kb_free_audiences.sql
git commit -m "feat(question): migration 057 — KB-free + 3 audience pain_point taxonomy

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01KwBzTatE6zVX9ReRvAAhWg"
```

(การ validate SQL รันจริงบน Neon temp branch อยู่ใน Task 3 Step 1 ก่อน deploy)

---

### Task 2: ตัด RAG (KB) ออกจาก non-news path

**Files:**
- Modify: `internal/agent/question.go:125-148` (ลบ else branch, ย้าย header ข่าวเข้า news branch)

**Interfaces:**
- Consumes: prompt_template ใหม่ (Task 1) ที่มี `{{.RAGContext}}` เปล่า
- ไม่แตะ struct/dedup/cooldown/retry/store

- [ ] **Step 1: แทนบล็อก RAG** (question.go:125-148) ด้วยโค้ดนี้

```go
	var ragContext strings.Builder
	if format.FormatName == "news" {
		// News format: live web search for fresh, reliable updates.
		// Never fall back to stale KB here — that produces fabricated news.
		researchContext, err := a.research.Research(ctx, "Facebook Ads หรือ Meta ที่กระทบผู้ลงโฆษณาในไทย")
		if err != nil {
			log.Printf("QuestionAgent: research failed: %v", err)
		}
		if researchContext == "" {
			return nil, ErrNoFreshNews
		}
		ragContext.WriteString("ข้อมูลข่าว/งานวิจัยสด (อ้างอิงได้):\n")
		ragContext.WriteString(researchContext)
		ragContext.WriteString("\n---\n")
	}
	// non-news: ไม่ ground ด้วย KB — gemini คิดจากเมนู pain_point ใน prompt (ตัด rag.Search ออก)
```

หมายเหตุ: struct field `a.rag` จะไม่ถูกเรียกใน Generate อีก แต่ package `rag` ยังใช้ (`rag.Engine` type, `rag.FormatVector` ใน store step) → ไม่ต้องแตะ import/constructor (unused struct field ไม่ทำให้ build fail; YAGNI ไม่ลบ field)

- [ ] **Step 2: build + vet + test package agent**

Run: `go build ./... && go vet ./internal/agent/ && go test ./internal/agent/`
Expected: build ผ่าน, vet เงียบ, test ผ่านทั้งหมด (รวม cooldownFilterRetry 5 tests). ถ้า sandbox บล็อก Go build cache ("operation not permitted") รัน go ด้วย sandbox ปิด

- [ ] **Step 3: อ่าน diff ยืนยัน surgical**

Run: `git diff internal/agent/question.go`
Expected: เปลี่ยนเฉพาะบล็อก 125-148 (ลบ else + เพิ่ม header ข่าว) ไม่มีอย่างอื่น

- [ ] **Step 4: Commit**

```bash
git add internal/agent/question.go
git commit -m "refactor(question): drop KB/RAG grounding for non-news questions

gemini reasons from the in-prompt pain_point menu instead of thin KB.
News format keeps live research.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01KwBzTatE6zVX9ReRvAAhWg"
```

---

### Task 3: Validate SQL, deploy, eyeball verify

**Files:** ไม่มีไฟล์ใหม่ (operational + quality gate)

- [ ] **Step 1: validate migration SQL บน Neon temp branch** (controller ทำ ผ่าน Neon MCP)
  - `create_branch` (project `snowy-grass-75448787`) → รันเนื้อหา `057_*.sql` ทั้งไฟล์ด้วย `run_sql_transaction` บน branch นั้น → ต้องไม่มี error
  - verify บน branch: `SELECT category_name, enabled FROM topic_categories WHERE enabled=true;` ต้องได้ 3 แถวใหม่เท่านั้น; `SELECT left(prompt_template,80) FROM agent_configs WHERE agent_name='question';` ขึ้นข้อความใหม่
  - `delete_branch` ทิ้ง temp branch
  - ถ้า SQL error → กลับไปแก้ Task 1, re-review

- [ ] **Step 2: /simplify บน diff โค้ด** (user pref) — รันบน commit Task 2, แก้เท่าที่ชี้

- [ ] **Step 3: push → Railway auto-deploy + migration auto-apply**

```bash
git push origin master
```
รอ backend service `adsvance-v2` (project `6decf46f-26c0-44b2-b066-1f30cc11f24d`) deploy SUCCESS ผ่าน `get-status`. migration 057 apply เองตอน boot (RunMigrations)

- [ ] **Step 4: verify prod state** — Neon `run_sql` project `snowy-grass-75448787`

```sql
SELECT filename FROM schema_migrations WHERE filename LIKE '057%';
SELECT category_name, enabled FROM topic_categories WHERE enabled = true ORDER BY category_name;
```
Expected: 057 applied; 3 แถว enabled = account-buyer, grey-operator, performance-advertiser

- [ ] **Step 5: eyeball — trigger produce แล้วดูคุณภาพคำถาม**

trigger produce (รอ cron รอบถัดไป หรือ POST produce). ดู log produce + query คลิปใหม่:
```sql
SELECT category, question, title FROM clips ORDER BY created_at DESC LIMIT 3;
```
ตรวจ (ยอมรับได้ต้องครบ):
- category = 1 ใน 3 กลุ่มใหม่
- คำถามตรง persona กลุ่มนั้น + เจาะจง (มีตัวเลข + สิ่งที่ลองแล้ว)
- ไม่มีเนื้อหา how-to ผิดกฎหมาย (fraud/ปลอมเอกสาร/หลบ KYC แบบสอนทำ)
- log ไม่มี "Generated 0 questions"; ถ้าชน cooldown เห็น retry แล้ว "Generated N questions"

- [ ] **Step 6: update memory** — เขียนผลลง [[project_question_cooldown_deadlock]] หรือ memory ใหม่ (KB-free + 3 audiences live), เชื่อม [[project_content_brain_v2]]

---

## Self-Review

**1. Spec coverage:**
- ตัด KB → Task 2 (ลบ RAG branch) ✓
- gemini ล้วน (model เดิม) → ไม่แตะ model ✓
- 3 กลุ่มแทน 10 หมวด → Task 1 §1-2 ✓
- เมนู 54 pain_point ใน angle_instruction → Task 1 §2 ✓
- prompt levers (specificity/curiosity/dice/burn-obvious/frame/temporal) → Task 1 §3 prompt ✓
- guardrail สายเทา → Task 1 §3 + Global Constraints ✓
- JSON shape เดิม → prompt output block เดิม ✓
- news format คงอยู่ → Task 2 news branch ✓
- rollback (10 หมวด disabled ไม่ลบ) → Task 1 §1 ✓
- asker micro-cast → Task 1 §4 ✓

**2. Placeholder scan:** ไม่มี TBD/TODO — migration SQL + โค้ดครบ. Task 3 มี operational fallback ("รอ cron หรือ POST") = ทางเลือกจริง ไม่ใช่ placeholder

**3. Type consistency:** prompt ใช้เฉพาะ field ที่มีใน `QuestionTemplateData` (Count, Category, CategoryAngle, FormatInstruction, ArchetypeInstruction, RoleInstruction, AudiencePersona, RAGContext, PreviousTopics, PreviousNames, TopicStats) — ตรงกับ struct จริง. JSON output keys ตรงกับ `GeneratedQuestion{question, questioner_name, category, pain_point}`. `renderTemplate` string-replace → ไม่มี `{{if}}` (ตาม Global Constraints)
