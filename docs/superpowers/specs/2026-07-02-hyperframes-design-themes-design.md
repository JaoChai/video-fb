# Design Themes — ยกเครื่องดีไซน์ Hyperframes ให้เลิก "ซ้ำเดิม" และเพิ่มยอดวิว

วันที่: 2026-07-02
สถานะ: Design (รออนุมัติก่อนทำ implementation plan)

---

## 1. ปัญหา & เป้าหมาย

**ปัญหา (จากเจ้าของช่อง):** ทุกคลิปหน้าตา "ซ้ำๆ เดิมๆ" — รูป, element, layout, ฟอนต์, จังหวะ เหมือนกันหมดทุกคลิป น่าเบื่อ ไม่ดึงดูด บน feed = คนเลื่อนผ่าน = ยอดวิวตก

**สาเหตุราก (ตรวจจากโค้ดจริง):** ระบบถูกออกแบบมาให้ "สม่ำเสมอ" — `StylePreset` แปรได้แค่สี/art-anchor แต่ฟอนต์ล็อก Sarabun ทุกอัน, layout ตายตัว 6 แบบ, motion ชุดเดียว, โครง Q&A จังหวะเดียว, ค่า default ใช้ signature อย่างเดียว → รูปทุกคลิปเป็นเวกเตอร์แบนน้ำเงิน-ส้มเหมือนกัน

**เป้าหมาย:** สร้างระบบ **Design Themes หมุนเวียน** — ชุดดีไซน์ที่ต่างกันจริง 4 ธีม แต่ละธีมออกแบบมาอย่างดี (คุมคุณภาพ) หมุนใช้คนละธีมต่อคลิป → คลิปติดกันดูต่างกันชัด แต่ยังรู้ว่าเป็นแบรนด์เดียวกัน พร้อมยกระดับจุดที่ทำให้ดู "AI-slop" (รูป, hook, caption) เพื่อดัน 3-second retention → ยอดวิว

**ตัววัดความสำเร็จ:** 3-second retention rate ขึ้น, average view duration ขึ้น, ยอดวิวเฉลี่ยต่อคลิปขึ้น (เทียบ baseline ก่อนเปลี่ยน) — วัดผ่าน metric ที่มีอยู่ (retention feeds preset performance-weighting)

---

## 2. หลักการหลัก: ขยาย Preset → Theme

ต่อยอดจากระบบ `StylePreset` เดิม (ไม่รื้อ) โดยขยาย struct ให้ 1 ธีม = ชุดที่ประสานกัน **6 แกน**:

| แกน | เดิม | ในระบบธีม |
|-----|------|-----------|
| สีพาเลตต์ | แปร (แต่หลุด navy+orange) | **คงตระกูล navy+orange ทุกธีม** (ต่างแค่โทน/น้ำหนัก) |
| art-anchor รูป | แปร | แปร — ต่าง **สื่อ** จริง (illustration/photo/3D/techno) |
| ฟอนต์ | ล็อก Sarabun | **แปรจริง** — คู่ฟอนต์ต่อธีม |
| layout composition | ตายตัว | ชุด composition ต่อธีม + หลากหลายในคลิป |
| motion profile | ชุดเดียว | โปรไฟล์ต่อธีม (นุ่ม/สแนป/สปริง) |
| pacing | จังหวะเดียว | ปรับได้ต่อธีม (ค่า default เท่าเดิมก่อน) |

**การเปลี่ยน struct (ข้อเสนอ):** เพิ่ม field ใน `StylePreset` (หรือ type ใหม่ `Theme` ที่ embed) ที่ `internal/producer/presets.go`:
- `Font TypeTokens` (มีอยู่แล้ว — แต่จะใส่ค่าจริงต่างกัน ไม่ใช่ `Type` ทุกอัน)
- 🆕 `HeadingFont TypeTokens` — ฟอนต์หัวข้อ (แยกจากฟอนต์เนื้อ)
- 🆕 `Motion MotionProfile` — easing/duration/entrance ต่อธีม (embed/override `MotionTokens`)
- 🆕 `LayoutKit []string` — รายชื่อ composition ที่ธีมนี้อนุญาต
- 🆕 `Texture string` — CSS treatment ของธีม (grain/glass/none) inject เข้า template

---

## 3. กติกาคงที่ (Brand Invariants) — คงทุกธีม

ต่อให้ธีมต่างกันแค่ไหน 3 สิ่งนี้ต้องเหมือนกันเสมอ เพื่อให้จำแบรนด์ได้:

1. **โลโก้/badge** "ADS VANCE" มุมซ้ายบน + หมวดหมู่ (ตำแหน่ง/ขนาดคงที่) — ใช้ `BrandName` เดิม
2. **สีส้ม accent จุดเดียว** — ส้ม (`--orange`) ใช้เน้น "คำสำคัญ / ตัวเลข / CTA" เท่านั้น ห้ามใช้พร่ำเพรื่อ (accent ต้อง pop)
3. **มาสคอตเสือดาว** สไตล์เดียว render เหมือนกันทุกธีม (มุมขวาบน default; ธีม 3D Clay ให้เด่นขึ้นได้)

นอกจากนี้คง: progress bar บน, `BrandCTA` outro, โครง Q&A, safe-zone

---

## 4. แคตตาล็อกธีม (4 ธีมแรก)

> พาเลตต์อ้างตระกูลแบรนด์: navy `#0047AF`/`#062F78`, orange `#F0A030`/`#FFB454`, ink `#F6F9FF`
> ค่า hex/px/timing ด้านล่างเป็น **ค่าเริ่มต้นที่เสนอ** — ปรับจูนตอน implement ได้

### ธีม 1 — Editorial Bold (`editorial-bold`)
- **สื่อรูป:** flat editorial illustration พรีเมียม เส้นสะอาด แสง cinematic นุ่ม (ยกระดับ `signature` เดิม)
- **art-anchor:** ต่อยอด `Brand.ImageStyleAnchor()` เดิม + เพิ่มคำสั่งคุณภาพ (ดูข้อ 8)
- **พาเลตต์:** navy เต็มจอ + ส้มจุดเดียว (= `Brand` เดิม)
- **ฟอนต์:** หัวข้อ **Kanit 800/900**, เนื้อ **Sarabun 600/700**
- **motion:** entrance fade+slide-up 380ms `ease-out`, Ken Burns 1.0→1.06
- **layout:** hero-centric, แถบข้อความล่าง (โครงปัจจุบันที่ขัดเกลา)
- **เหมาะ:** เนื้อหาจริงจัง/ทางการ

### ธีม 2 — Cinematic Photo (`cinematic-photo`)
- **สื่อรูป:** ภาพถ่ายจริง editorial (เลนส์ 85mm, setting จริง: ออฟฟิศ/คน/หน้าจอแอด/เงิน) เกรดสีอุ่น เคลือบโทน navy
- **art-anchor:** สั่ง "editorial photography, 85mm f/1.4, natural window light, warm color grade, navy duotone wash" + negative "no 3D render, no illustration, no cartoon"
- **พาเลตต์:** navy grade overlay + ตัวหนังสือส้ม/ขาว
- **ฟอนต์:** หัวข้อ **Kanit 700/800**, caption **IBM Plex Sans Thai 500/600** (บนภาพต้องมี stroke/scrim)
- **motion:** slow parallax บนภาพ + text slide นุ่ม 420ms; scrim gradient เข้มขึ้นเพื่ออ่านออก
- **layout:** full-bleed photo + text ล่าง, scrim แรง
- **เหมาะ:** เคส/รีวิว/ความน่าเชื่อถือ · **ฆ่า AI-slop ได้ดีสุด**

### ธีม 3 — Neon Techno HUD (`neon-techno`)
- **สื่อรูป:** พื้นเข้ม (navy-deep/near-black) + เส้น neon ฟ้า-ไฟฟ้า/ส้ม, glassmorphism, ลาย HUD/กราฟ/วงแหวนข้อมูล
- **art-anchor:** "sleek techno HUD, crisp neon line-art, glowing strokes, dark navy background, glass panels"
- **พาเลตต์:** navy-deep พื้น + ส้ม accent + ฟ้า info เส้น (ฟ้าเป็นเส้น ไม่ใช่ accent หลัก — ส้มยังคุม)
- **ฟอนต์:** หัวข้อ **Prompt 700/800** (เรขาคณิต), เนื้อ Prompt 600
- **motion:** สแนปเร็ว 260ms `ease-in-out` + glow pulse ตอนเน้นคำ
- **layout:** glass card + HUD element, ตัวเลข/สถิติเด่น
- **เหมาะ:** ทริค/ข้อมูล/สถิติ พลังงานสูง

### ธีม 4 — Soft 3D Clay (`soft-3d-clay`)
- **สื่อรูป:** 3D claymorphism (วัตถุมนโค้ง เงานุ่ม soft studio light) โทนอุ่น
- **art-anchor:** "soft 3D clay-render, rounded shapes, soft studio shadows, matte finish, warm palette anchored to brand navy+orange"
- **พาเลตต์:** navy + ส้ม แต่ background สว่าง/อุ่นได้ (โทนเป็นมิตร)
- **ฟอนต์:** หัวข้อ **Kanit 800** (โค้งมน), เนื้อ **Prompt 600**
- **motion:** เด้งสปริง `ease-spring` 480ms, มาสคอตเด้งเข้า
- **layout:** วัตถุ 3D ลอย + มาสคอตเด่น
- **เหมาะ:** มือใหม่/อธิบายง่ายๆ/เป็นมิตร

---

## 5. Typography (ฟอนต์)

**ปัญหาเดิม:** Sarabun อย่างเดียว = เรียบ + ไม่มีความต่าง ("Task 5 ฟอนต์ที่ถูกเลื่อน")

**เพิ่มฟอนต์ (ทั้งหมด OFL license — ใช้เชิงพาณิชย์ได้):**
- **Kanit** (400/600/700/800) — หัวข้อธีม 1,2,4
- **Prompt** (400/600/700/800) — ธีม 3, เนื้อธีม 4
- **IBM Plex Sans Thai** (400/500/600/700) — caption ธีม 2
- คง **Sarabun** — เนื้อธีม 1 + fallback

**⚠️ Gotcha สำคัญ (ตรวจจากโค้ด/memory):** render เป็น offline — CDN เข้าไม่ถึงตอน render (GSAP ต้อง bundle, ฟอนต์ต้อง `@font-face` local ไม่งั้น non-deterministic lint fail / จอค้าง) → **ฟอนต์ใหม่ต้อง vendor เป็นไฟล์ .ttf/.woff2 ลง `internal/producer/assets/fonts/`** และ copy เข้า project dir ตอน composition build เหมือน Sarabun

**Thai gotcha (คง):** line-height ≥ 1.32 สำหรับข้อความหลายบรรทัด (กันสระ/วรรณยุกต์ชนกัน), ห้าม negative letter-spacing, HTML-escape ก่อน render (เดิมมีแล้ว)

---

## 6. Layout Composition ที่หลากหลายขึ้น

เดิม 6 layout (hook/hero/stat/step/tip/cta) องค์ประกอบเป๊ะทุกครั้ง เพิ่ม **variety ในคลิป** โดยให้ scene agent เลือก composition ต่อฉากจากชุดที่ธีมอนุญาต:
- `subject-left` / `subject-right` — ภาพข้าง ข้อความอีกข้าง
- `full-bleed` — ภาพเต็มจอ + text overlay ล่าง (scrim)
- `split` — บน/ล่าง แบ่งครึ่ง
- `type-only` — ตัวหนังสือใหญ่ล้วน พื้น navy/ภาพเบลอ (เหมาะ hook/quote)
- `stat-hero` — ตัวเลขใหญ่กลางจอ

**กติกา:** ฉากติดกันในคลิปเดียวห้าม composition ซ้ำกันเกิน 2 ฉากติด (แก้ความจำเจในคลิป)

---

## 7. Motion Profile ต่อธีม

ใช้ `MotionTokens` เดิมเป็นฐาน แต่ละธีมเลือก easing/duration หลัก + "1 motion idea ต่อฉาก" (ไม่ยิงหลาย effect พร้อมกัน):
- Editorial: `ease-out` 380ms + Ken Burns
- Cinematic: parallax นุ่ม 420ms
- Techno: `ease-in-out` 260ms สแนป + glow pulse
- 3D Clay: `ease-spring` 480ms เด้ง

---

## 8. Agent ที่ต้องพัฒนา/แก้ (skills ที่รับผิดชอบ)

### 8.1 🔧 Image Agent (`internal/agent/image.go` + `buildScenePrompt` ใน brand.go)
- ใช้ **art-anchor ต่อธีม** (จาก theme ที่เลือก)
- เพิ่ม **house-style block**: camera/lens/lighting/grade + **negative-prompt exclusions** ต่อธีม (กัน "3D glossy plastic oversaturated stock-photo" ตามสไตล์ที่ไม่ต้องการ)
- 🆕 **per-clip seed/style anchor** — ทุกฉากในคลิปเดียวใช้ seed family เดียว → รูปเข้าชุดกัน (แก้ "รูปแต่ละฉากไม่เข้ากัน")
- คง safe-zone + "no text in image"

### 8.2 🔧 Scene Agent (`internal/agent/scene.go`, migration 031)
- บังคับ **hook ≤ 7 คำ** โผล่ **เฟรมแรก** ไม่มี intro delay > ~150ms (research: 3-sec retention = ตัวดันวิวหลัก)
- เลือก **composition ต่อฉาก** จาก LayoutKit ของธีม (ห้ามซ้ำเกิน 2 ติด)
- คง: 1 ไอเดีย/ฉาก, ไม่มี emoji, rows ≤ 3

### 8.3 🔧 Caption (composition/template)
- เปลี่ยนจากไฮไลต์ "คำที่ยาวสุด" → ไฮไลต์ **คำ emphasis จริง** (`emphasis_words` มีอยู่แล้วใน scene schema แต่ caption ใช้ความยาวแทน) — ต่อธีมใช้สี/สเกลเน้นตาม motion profile

### 8.4 🔧 Critic Agent (`internal/agent/critic.go`, migration 034)
- เพิ่มเช็ค: คลิปเกาะ **ธีมที่เลือก** (art-anchor ตรง, สีตรง), hook แรงใน 3 วิ, `emphasis_words` มีจริง

### 8.5 🔧 Visual QA (`internal/agent/visualqa.go`, migration 036)
- คงกติกา fail-closed เดิม + เพิ่ม: ตรวจว่าฟอนต์/สี match ธีม, ไม่มี text baked ในรูป

---

## 9. การเลือกธีม (Selection)

**ใช้กลไกเดิม** `PickPresetWeighted` / `PickPreset` (presets.go) — ไม่เขียนใหม่:
- default: **avoid-last uniform** (ไม่ซ้ำธีมคลิปก่อนหน้า)
- ถ้าเปิด `STYLE_PRESETS_PERFORMANCE_ENABLED`: epsilon-greedy เอนไปธีม retention สูง (โครงมีอยู่แล้ว)
- คลิปเก็บ `theme_key` (เดิมเก็บ preset key อยู่แล้ว — ใช้ field เดิม)

---

## 10. Data Model / Migration

- `agent_configs`: อัปเดต prompt ของ scene/critic/visual_qa/image (migration ใหม่ ต่อจาก 045)
- ไม่ต้องเพิ่มตารางใหม่ — reuse preset key column + preset_scores สำหรับ performance-weighting
- ธีมนิยามใน Go (`presets.go`) เป็น source of truth (ไม่ใช่ DB) — เหมือนเดิม

---

## 11. Feature Flag & Rollout & Fallback

- คง `STYLE_PRESETS_ENABLED` — **off = ใช้ `editorial-bold` (signature ยกระดับ) อย่างเดียว** = ปลอดภัย
- `Presets[0]` ยังเป็น fallback สากล → flag off หรือ selection พัง = ได้ธีม default เสมอ ไม่พัง
- image fail → downgrade CSS background (เดิมมี circuit breaker)
- **Rollout เป็นเฟส** (ดูข้อ 13)

---

## 12. Non-Goals (YAGNI)

- ❌ Art Director agent คิด concept อิสระ (เลือก "ธีมหมุน" แทน — คุมคุณภาพกว่า)
- ❌ ไม่ทำครบ 6 ธีมรอบแรก (เริ่ม 4)
- ❌ ไม่เปลี่ยน rendering engine / ไม่แตะ TTS / publishing
- ❌ ไม่เพิ่ม pacing/โครงเรื่องใหม่รอบแรก (ค่า default เดิม)
- ❌ ไม่ทำ UI เลือกธีมใน admin รอบแรก (หมุนอัตโนมัติพอ)

---

## 13. เฟสการทำ (เสนอ)

**Phase 1 — Quick wins (ยกคุณภาพ ไม่ต้องรอธีมครบ):**
- hook ≤7 คำเฟรมแรก + emphasis-word highlight + image house-style/negative/seed anchor
- เพิ่มฟอนต์ Kanit/Prompt (vendor) ใช้กับ default theme
→ ดัน retention ได้เร็วก่อนธีมครบ

**Phase 2 — Theme system:**
- ขยาย struct + LayoutKit + motion profile + template รองรับหลาย composition/ฟอนต์/texture
- ทำครบ 4 ธีม + เปิด avoid-last rotation

**Phase 3 — เรียนรู้:**
- เปิด performance-weighting (epsilon-greedy) + วัดผล retention ต่อธีม + ปรับจูน

---

## 14. สมมติฐาน & คำถามค้าง

- **สมมติ:** ภาพถ่ายจริง (ธีม 2) — gpt-image-2 สร้าง photoreal ได้คุณภาพพอ (ยังไม่ยืนยัน คุณภาพจริง — ต้องทดสอบตอน implement; ถ้าไม่ผ่านคุณภาพ อาจสลับธีม 2 เป็นสื่ออื่น)
- **สมมติ:** metric retention ต่อคลิปมีเก็บพอสำหรับ performance-weighting (โครงมี แต่ยังไม่ตรวจว่ามีข้อมูลพอ)
- **คำถามค้าง:** อยากทำ Phase 1 ก่อนเห็นผลเร็ว หรือทำ Phase 2 (ธีม) ให้เห็นความต่างก่อน? (เสนอ: Phase 1 ก่อน)
