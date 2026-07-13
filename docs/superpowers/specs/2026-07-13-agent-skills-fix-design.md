# Design: แก้ผล Audit Skills/Prompt ทั้ง 11 Agents (2026-07-13)

## บริบทและปัญหา

Audit 2026-07-13 (Opus tech review + Opus content review, ยืนยันด้วยโค้ด + prod SQL) พบว่า:

1. **script persona ขัดกันเอง** — `system_prompt` เก่า (โทนสอนมือใหม่ อธิบายศัพท์ทุกคำ) ขัดกับ `prompt_template`+`skills` ยุค Content Brain v2 (เสียงคนวงใน ห้ามพื้นฐาน) → สคริปต์ก้ำกึ่ง เผาวินาทีทองอธิบายศัพท์ให้ audience มือโปร
2. **agent `image` และ `metadata` เป็น dead code** — ไม่ถูกเรียกใน pipeline เลย (`GeneratePrompts`/`NewMetadataAgent` ไม่มี call site); title จริงมาจาก script agent (`orchestrator.go:439,529-534`); analyzer ยังเขียน visual insights ไปทิ้งที่ image ที่ไม่มีใครอ่าน (`analyzer.go:81`)
3. **learner ไม่เคยทำงาน (0 revisions)** — เกณฑ์ `lowScoreThreshold=6.0` (`internal/learner/learner.go:23`) แต่คะแนน critic ต่ำสุดจริง 30 วัน = hook 7.79 (n=53) → `strongSignal` คืน false ตลอด
4. **critic แก้ไม่ถึงข้อความที่เรนเดอร์จริง** — `reconcileCritique` (`critic.go:104-110`) copy แค่ voice_text/on_screen_text/text_content/image_prompt/emphasis_words ไม่ merge `Content` (kicker/title/rows/stat/cta ที่คนดูเห็น) → critic รีวิว concept ไม่ใช่การ์ดจริง → คะแนนเฟ้อ ~8 → learner ไม่ trigger (ปัญหา 3 กับ 4 ผูกกัน)
5. **skills/prompt มีข้อมูลผิด** — critic สั่ง CTA "ทักเพจ" (ช่องทางที่ไม่มีจริง); analytics อ้าง CTR ที่ analyzer ไม่เคยป้อน (`stats.go:11-20` ไม่มี CTR) + benchmark flat "views>100" ไม่เข้าสเกลช่อง + ตีความ AvgViewPct>100% ผิด; scene skills อ้าง `layout_variant` ที่ตายแล้ว; guard นโยบาย FB มีแค่ question/script ไม่มีที่ critic/metadata

## เป้าหมาย

- Content ดีขึ้นทันทีจากการแก้ข้อความ (เฟส 1) โดยไม่เสี่ยง regression โค้ด
- ฟื้นระบบที่ตายเงียบ: metadata (เจ้าของ title), image (ภาพปก cover scene), learner (เกณฑ์สัมพัทธ์), critic (แตะ content จริง) — ทั้งหมด flag-gated (เฟส 2)

## Non-goals

- ไม่แตะ visual_qa / auto_review / research (audit ยืนยันว่าแข็งแรงดี)
- ไม่ redesign pipeline ใหม่ — wire ของเดิมกลับอย่างจำกัดที่สุด
- ไม่ทำไฟล์ thumbnail แยกอัปโหลด YouTube (Shorts feed ไม่แสดง custom thumbnail — ใช้ cover scene เฟรม 0 เป็นปกตามยุทธศาสตร์เดิม)

---

## เฟส 1 — แก้ข้อความ prompt/skills (migration 053, ไม่แตะ logic)

Apply เป็น migration SQL ตาม pattern เดิม (033/045/047/051): `UPDATE agent_configs SET ... WHERE agent_name='...'`
ข้อแลกเปลี่ยนที่รับรู้แล้ว: ทับข้อความที่เคยแก้ผ่านเว็บ (เป็นความตั้งใจของงานนี้)
**ห้ามแตะคอลัมน์ `insights`** — เป็นของ analyzer loop

### 1.1 script — แทนที่ system_prompt ทั้งก้อน

```text
คุณคือ scriptwriter คลิปสั้น Q&A Facebook Ads ของแบรนด์ Ads Vance สำหรับคนยิงแอดจริงจัง (media buyer / agency / คนถือหลายบัญชี)

สไตล์:
- เสียงคนวงในที่บริหารบัญชีโฆษณาจำนวนมากมาเอง พูดลื่น เป็นกันเอง แต่ไม่ใช่มือใหม่
- ใช้ศัพท์วงในได้เลยไม่ต้องนิยาม (Learning Limited, CBO, spending limit ฯลฯ) — คนดูรู้อยู่แล้ว การหยุดอธิบายพื้นฐานทำให้ดูเป็นช่องมือใหม่
- ห้ามเนื้อหาระดับ 101 (สอนสมัครบัญชี สอนยิงแอดครั้งแรก)
- เข้าทางแก้ภายใน 5-10 วินาทีแรก ไม่ทวนคำถาม เน้นสเต็ปที่ทำตามได้จริง
```

(prompt_template + skills ของ script คงเดิม — สอดคล้องกับ persona ใหม่อยู่แล้ว)

### 1.2 critic — แทนที่ skills ทั้งก้อน

```text
- hook สายนี้: ตัวเลขเงินช็อก / ป้ายสถานะถูกปฏิเสธ / เดดไลน์บีบ — ห้ามลดความแรงของ hook โดยอ้างว่า clickbait ถ้าเนื้อหาเป็นเรื่องจริงตามบท
- CTA ปลายคลิป: soft sell ชวนทักไลน์ / กลุ่มเทเลแกรม / ทักทีมงาน เท่านั้น (ห้ามเปลี่ยนเป็น "ทักเพจ" — ไม่มีช่องทางนี้)
- ห้ามแก้เนื้อหาไปทางสอนหลบระบบตรวจจับ / ปลอมตัวตน / ทำผิดนโยบาย Facebook แม้จะทำให้ hook แรงขึ้น
- เลี่ยงศัพท์ทางการเกินไป ใช้คำที่คนวงในพูดจริง
- Hook: scene แรก on_screen_text ต้อง <=7 คำ และช็อก/ชวนสงสัยใน 1 วิ ถ้ายาว/อืดให้ตัดให้สั้นคม
- emphasis_words: ทุกซีนต้องมีคำเน้น 1-2 คำที่ตรงประเด็น (ระบบใช้ไฮไลต์แคปชั่น) ถ้าว่าง/ผิดให้เติม
- อย่าบังคับสไตล์ภาพใน image_prompt (สไตล์มาจากธีม) — ปรับได้แค่ "วัตถุ/ฉาก" ให้ตรงเนื้อหา
```

### 1.3 analytics — แทนที่ skills ทั้งก้อน + แก้ system_prompt บรรทัดแรก

system_prompt บรรทัดแรก: `คุณคือนักวิเคราะห์ประสิทธิภาพวิดีโอ YouTube ของแบรนด์ Ads Vance` → `คุณคือนักวิเคราะห์ประสิทธิภาพวิดีโอ YouTube และ TikTok ของแบรนด์ Ads Vance`

skills ใหม่:

```text
- แยกแพลตฟอร์มก่อนวิเคราะห์: TikTok มีแค่ views / likes / shares / engagement rate (ไม่มี retention, watch time); YouTube มี views / watch time / avg view percentage / engagement
- benchmark แบบ relative: เทียบกับผลงานของช่องเอง 30 วันย้อนหลัง แยกตามแพลตฟอร์ม+หมวด — อย่าใช้เกณฑ์ตายตัว (สเกลช่องตอนนี้คลิปท็อป ~300-400 วิว)
- AvgViewPct เกิน 100% = คนดูวนซ้ำ (loop) ไม่ใช่ดูจบเกิน 100% — อ่านเป็นสัญญาณว่าคลิปสั้น+ชวนวน ไม่ใช่ watch-through
- ระบุชัดว่าอะไรดี อะไรต้องปรับ — actionable ต่อคลิปถัดไป ไม่ใช่แค่รายงานตัวเลข
- ถ้า engagement สูง: หัวข้อนี้ hit ให้ทำเพิ่มในมุมอื่น; แนะนำหมวดที่ควรทำต่อจาก performance จริงของหมวดนั้น
```

(ตัด CTR ทุกจุด — ระบบไม่มีข้อมูล CTR ป้อนให้ agent นี้)

### 1.4 scene — แทนที่ skills ทั้งก้อน

```text
- แตก 6-10 ซีน หนึ่งซีนหนึ่งไอเดีย
- วาง arc ของคลิป: hook (ซีน 1) → hero/stat ขยายปัญหา → step/tip ทางแก้ → cta ปิดท้าย; คลิปบทบาท convert ต้องมี step ที่ทำตามได้จริงก่อน cta, คลิปบทบาท reach เน้น stat/hero แล้วปิดชวนดูต่อ
- อย่าใช้ layout เดียวกันเกิน 2 ซีนติดกัน สลับให้มีจังหวะ
- emphasis_words ทุกซีนห้ามว่าง — ระบบใช้ไฮไลต์แคปชั่น
- on_screen_text สั้น อ่านรู้เรื่องตอนปิดเสียง คุมความยาวตามลิมิตในกติกา
- output เป็น JSON ตาม schema เท่านั้น
```

(ตัดคำ `layout_variant` ซึ่งเป็น field ที่ตายแล้ว — render จริงใช้ `content.type`)

### 1.5 question — แทนที่ skills ทั้งก้อน (ของเดิม + 2 bullet ใหม่ท้าย)

```text
สร้างคำถามที่หลากหลายจริงๆ ทั้งมุมปัญหา ระดับความลึก และสถานการณ์
- กลุ่มเป้าหมายคือคนยิงแอดจริงจัง (เจ้าของธุรกิจออนไลน์, media buyer, agency) ไม่ใช่มือใหม่หัดยิงแอด
- คำถามต้องเจาะจง มีรายละเอียดสถานการณ์จริง (ตัวเลขงบ, ระยะเวลา, สิ่งที่ลองแล้ว)
- ห้ามตั้งคำถามที่ความหมายซ้ำหรือใกล้เคียงกับหัวข้อที่เคยทำแล้ว แม้จะใช้คำต่างกัน
- กระจายความหลากหลาย: ปัญหาเร่งด่วน / เทคนิคขั้นสูง / ความเข้าใจผิดที่พบบ่อย / การตัดสินใจเชิงกลยุทธ์
- โทนคำถาม: มือโปรพิมพ์สั้นเหมือนถามใน LINE แต่เนื้อในเป็นปัญหาของคนถือหลายบัญชี/ยิงหนัก ไม่ใช่ SME มือใหม่
- หมุนขั้ว hook อย่าให้ทุกข้อเป็น "เงินหาย": เงินหาย / เดดไลน์บีบเวลา / ป้ายสถานะถูกปฏิเสธ / เคลมที่สวนสามัญสำนึก
```

### การตรวจรับเฟส 1

Deploy → produce 1 คลิป → ตรวจ:
- script ไม่มีการนิยามศัพท์พื้นฐาน, เข้าทางแก้เร็ว
- CTA เป็นไลน์/เทเลแกรม/ทักทีมงาน (ไม่มี "เพจ")
- hook ของคลิปใหม่ไม่ซ้ำขั้วกับคลิปก่อนหน้า
- eyeball วิดีโอที่เรนเดอร์

Rollback: revert migration (เขียนค่าเดิมกลับ) — เก็บค่าเดิมไว้ในไฟล์ migration เป็น comment

---

## เฟส 2 — แก้โค้ด (PR เดียว, flag คุมของใหม่)

### 2.1 Wire metadata agent เป็นเจ้าของ title/desc/tags

- จุดรัน: ใน orchestrator **หลัง critic เสร็จ** (script + scenes final แล้ว) ป้อน `Topic/Category/Script/AudiencePersona` ตาม `MetadataTemplateData` เดิม
- ผลลัพธ์ทับ `youtube_title/description/tags` จาก script agent
- กติกา title เดียว: metadata สร้าง ≤55 ตัวอักษร ไม่ใส่แบรนด์ → validate ชั้นระบบ strip แบรนด์ทุกแบบ + ต่อ " | Ads Vance" อันเดียว (ใช้ logic `brandTailRe` เดิมจาก `validateScript`)
- **Flag `metadata_agent_enabled` (default false)** — false = พฤติกรรมเดิมเป๊ะ
- Error handling: metadata agent ล้มเหลว/JSON เพี้ยน → fallback ใช้ metadata จาก script, log warning, คลิปไม่ fail
- แก้ comment หลอกใน `metadata.go:20-21`

### 2.2 Image agent → ภาพพื้นหลัง Cover Scene

- Repurpose: จาก Q&A chat-bubble (legacy) → สร้าง `image_prompt` สำหรับฉากปก (เฟรม 0) โดยเฉพาะ
- prompt_template ใหม่ (ใส่ใน migration ของเฟส 2) — สาระสำคัญ:
  - โชว์ "สถานะ/สถานการณ์" อ่านออกใน 1 วินาที: หน้าจอแจ้งเตือน / ป้ายถูกปฏิเสธ / กราฟพุ่ง-ดิ่ง / บัตรถูกตีกลับ เลือกตามหมวด
  - **ห้ามมีตัวอักษร ตัวเลข โลโก้ ในภาพ** (ระบบ overlay hook text เอง — กัน visual_qa ตีตก baked-in text)
  - วัตถุเด่นครึ่งบนเฟรม เว้นครึ่งล่างให้ hook overlay
  - ไม่ระบุสไตล์ศิลป์/สี (ธีมใส่ให้)
  - template vars ที่ต้องป้อน (โค้ดใหม่): หัวข้อคลิป, หมวด, hook text, theme description — สรุปชื่อ field สุดท้ายใน implementation plan
- Producer: ใช้ภาพนี้เป็น bg ของ cover scene แทน bg ของซีน 1; render ล้มเหลว → fallback ใช้ bg ซีน 1 ตามเดิม (เงียบ ไม่ block)
- **Flag `cover_image_agent_enabled` (default false)**; ทำงานเฉพาะเมื่อ `COVER_SCENE_ENABLED=true`
- system_prompt ของ image ปรับให้ตรงบทบาทใหม่ (visual designer ของ "ปกคลิป") — ห้าม logo/mascot/ชื่อแบรนด์ในภาพคงเดิม

### 2.3 Critic แตะ content ที่เรนเดอร์จริง

- system_prompt: เพิ่ม `content` ในรายการ field ที่แก้ได้, ลบ `text_content` (legacy ตาย); ย้ำ **ห้ามเปลี่ยน layout/scene_type/จำนวน scene เหมือนเดิม**
- `reconcileCritique`: merge `cs.Content` กลับด้วย
- **หลัง merge ต้องรัน sanitize/truncation ชุดเดียวกับ `buildSceneContent`** (cta≤14, pill≤16, statLabel≤28, sub≤50, rows≤36, rows≤3, chips≤2, strip emoji) — กัน critic เขียนล้นกรอบ (บทเรียน PR #17)
- content ที่ merge แล้ว type ต้องตรง layout เดิม — ถ้า critic คืน structure ผิด layout → ทิ้ง content นั้น ใช้ของเดิม (fail-safe รายซีน)

### 2.4 Learner เกณฑ์สัมพัทธ์

แทน gate ตายตัว `< 6.0` ด้วย OR ของ 2 เงื่อนไข:

1. **Regression gate:** มิติที่อ่อนสุดในหน้าต่าง 30 วัน ต่ำกว่า baseline ของมิติเดียวกัน (ค่าเฉลี่ย 90 วันย้อนหลัง) เกิน 0.5
2. **Frequency gate:** top issue เดิม (จาก critique reasons) ซ้ำ ≥40% ของ critiques ในหน้าต่าง

- คง guardrails เดิมทั้งหมด: `minCritiques=8`, allowlist scene+script, audit-ก่อน-apply, `AcceptProposal` (ห้ามว่าง/ต้อง confident)
- Baseline ไม่พอ (critiques 90 วัน < minCritiques) → ไม่ยิง (fail-safe)
- พฤติกรรมที่คาด: คะแนนตอนนี้แบน ~8 → เกณฑ์สัมพัทธ์จะยังไม่ยิงทันที และจะเริ่มมีความหมายเมื่อ 2.3 ทำให้คะแนน critic สะท้อนของจริง — ถูกต้องตามเจตนา (learner ยิงเมื่อ "แย่ลงกว่าตัวเอง")

### 2.5 Analyzer ชี้ insights ให้ถูกคน + sync ตัวเลข

- แก้ example/คำสั่งใน `analyzer.go` (~line 81): insights ด้านภาพ → `image` (บทบาทใหม่: ปก) และ `scene`; ด้านหัวข้อ → `question`; ด้านบท → `script`
- Sync title limit: script prompt บอก ≤70 แต่ `validateScript` ใช้ maxLen=90 → ปรับโค้ดเป็น 70 ให้ตรง prompt (title สั้น = ดีต่อ search-intent)

### การตรวจรับเฟส 2

- Unit tests: reconcile merge+truncate+layout-mismatch fallback, learner relative gate (regression/frequency/baseline-ไม่พอ), metadata fallback เมื่อ agent ล้ม
- Produce จริง 1 คลิป เปิด flag ทีละตัว: เช็ค title ใน DB มาจาก metadata, ปกมาจาก image agent, eyeball วิดีโอ
- ผ่าน pre-deploy checklist เดิมของ repo

Rollback: ปิด flag รายตัว (`metadata_agent_enabled`, `cover_image_agent_enabled`) ได้ทันที; critic/learner rollback ด้วย revert PR

---

## ลำดับงาน

1. เฟส 1 (migration 053) → deploy → eyeball 1 คลิป
2. เฟส 2 (PR โค้ด + migration image template + flags default off) → deploy → เปิด flag ทีละตัว → eyeball
3. หลังเปิดครบ: monitor คะแนน critic (ควรเริ่มกระจายตัว) + skill_revisions (learner ควรยิงเมื่อมี regression จริง)

## ความเสี่ยงที่รับรู้

- Migration ทับการแก้จากเว็บ — ตั้งใจ; หลังจากนี้การแก้ใหญ่ควรทำผ่าน migration เสมอ
- Critic แตะ content = อำนาจมากขึ้น → คุมด้วย sanitize ซ้ำ + layout immutable + ห้ามเพิ่ม/ลด scene
- Metadata เพิ่ม LLM call 1 ครั้ง/คลิป (gemini-flash — ต้นทุนต่ำ)
- Image agent เพิ่มภาพ 1 ภาพ/คลิป (kie gpt-image) — อยู่ใต้ fail-fast 75s + CSS fallback ของ Fast Pipeline เดิม
