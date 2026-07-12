# Design Round 2 — ยกเครื่อง Design (Hyperframes + Image) ดันยอดวิวด้วย 3 วินาทีแรก

วันที่: 2026-07-12
สถานะ: Design (อนุมัติแล้ว — พร้อมทำ implementation plan)
แนวทาง: **A — ต่อยอดระบบธีม/template เดิม**

---

## 1. ปัญหา & เป้าหมาย

**บริบท:** อัพเดต content เสร็จแล้ว (Content Brain v2 LIVE 2026-07-12) เหลือ design — ทั้ง hyperframes render และ AI image — ที่เจ้าของช่องเห็นว่าคลิป "ไม่ดึง/ดูซ้ำ" 4 จุด (ยืนยันจากเจ้าของช่อง เลือกทั้ง 4):
1. **เฟรมแรกไม่หยุดนิ้ว** — คนเลื่อนผ่านใน 1-2 วิ
2. **รูปดู AI/สำเร็จรูป** — flat vector generic ไม่พรีเมียม
3. **คลิปดูนิ่ง ไม่มีชีวิต** — motion น้อย เหมือนสไลด์
4. **โครง/จังหวะยังซ้ำ** — layout/Q&A จังหวะเดียวทุกคลิป

**เป้าหมาย:** ดันยอดวิวผ่าน **3-second retention** ซึ่งเป็นตัวหลักที่กำหนดการกระจายของ Reels/Shorts/TikTok — ไม่ใช่ความสวยทั้งคลิป ทำครบ 4 จุดในแผนเดียว ("ยกเครื่องรวดเดียว")

**ตัววัดความสำเร็จ:** 3-sec retention rate ↑, average view duration ↑, ยอดวิวเฉลี่ย/คลิป ↑ (เทียบ baseline ก่อนเปลี่ยน) — วัดผ่าน metric ที่ป้อน preset performance-weighting อยู่แล้ว

---

## 2. สถานะปัจจุบัน (ตรวจจากโค้ดจริง — สิ่งที่ทำแล้ว/ยัง)

| แกน | สถานะจริง | ที่มา (ตรวจแล้ว) |
|-----|-----------|------------------|
| ธีมหมุนเวียน 4 ธีม | ✅ ทำแล้ว (editorial-bold/cinematic-photo/neon-techno/soft-3d-clay) | `internal/producer/presets.go` |
| ฟอนต์แปรจริง | ✅ Kanit/Prompt/IBM Plex/Sarabun vendored local | template `@font-face` |
| รูป AI | ⚠️ `gpt-image-2-text-to-image` ตัวเดียว (10 credits, 108-400s) → fallback `nano-banana-2-lite` → CSS | `internal/producer/kieai.go`, `producer.go` |
| Layout ใน template | ⚠️ รองรับจริงแค่ `hook/hero/stat/step/tip/cta` — **subject-left/full-bleed/type-only ยังไม่ได้ทำ** | `layout_multi_scene.html.tmpl` (data-layout), `scene_adapter.go` |
| เฟรมแรก (cover) | ❌ ไม่มีคอนเซ็ปต์ปกเฉพาะ — `.scene{opacity:0}` เป็น default แล้ว GSAP fade ขึ้น → **เฟรม 0 แทบว่าง** | template CSS `.scene{...opacity:0}` |
| Motion v2 | ⚠️ โค้ดครบ (parallax drift/entrance variety/count-up) **แต่ flag `SCENE_MOTION_V2_ENABLED` ปิดอยู่** | `audio.go:21`, `producer.go:434`, `composition_types.go` |

**สรุป:** รอบ 2026-07-02 ทำ "ธีม/ฟอนต์" ไปแล้ว รอบนี้เติม 4 จุดที่ยังเป็นรูโหว่จริง: **cover เฟรมแรก, premium image, เปิด motion, composition variety**

---

## 3. สถาปัตยกรรม (แนวทาง A)

ต่อยอดบนของเดิมทั้งหมด **ไม่รื้อ**:
- source of truth ธีม = `StylePreset` ใน `presets.go` (Go, ไม่ใช่ DB)
- render = `layout_multi_scene.html.tmpl` + hyperframes CLI (offline, ห้าม CDN)
- content → scene agent → scene_adapter → composition → template

แบ่งเป็น **4 workstream อิสระ** แต่ละอันมี feature flag แยก ปิดทุก flag = ได้ output วันนี้เป๊ะ (`Presets[0]` = editorial-bold ยังเป็น fallback สากล)

> ค่า hex/px/timing/ชื่อ model ด้านล่างเป็น **ค่าเริ่มต้นที่เสนอ** — ปรับจูนตอน implement ได้

---

## 4. Workstream 1 — Cover Scene (เฟรมแรกหยุดนิ้ว) 🔴 lever อันดับ 1

**ปัญหาราก (ตรวจจาก CSS):** `.scene{position:absolute;inset:0;opacity:0}` เป็น default → ทุกฉากเริ่มมองไม่เห็น แล้ว GSAP timeline ค่อย fade ขึ้น → **เฟรม 0 (ที่ platform ดึงไปทำปก/preview) แทบว่างเปล่า มีแต่พื้น navy** = ปกไม่ดึง = scroll

**แก้:**
- ฉากแรก = layout ใหม่ `cover` → **render เต็มตั้งแต่เฟรม 0** (ไม่เริ่มจาก opacity:0); การเคลื่อนเข้าใช้ punch-in เบาๆ จากสถานะที่มองเห็นแล้ว (เช่น scale 1.03→1.0 / drift เล็กน้อย) ไม่ใช่จาก invisible
- ดีไซน์ปกระดับ thumbnail: hook ≤7 คำ (Kanit Black ใหญ่สุด), รูป hero โดดเด่น 1 ภาพเต็มพื้นหลัง + scrim, **ส้มเน้น "คำสำคัญคำเดียว"**, badge ADS VANCE มุมซ้ายบน (invariant)
- ข้อความปกดึงจาก hook ที่ content brain v2 ผลิต (≤7 คำอยู่แล้ว) — ผ่าน field `on_screen_text`/hook ของ scene แรก (ต้อง verify field ที่ใช้ได้จริง)
- อยู่ใน safe-zone เดิม, Thai line-height ≥1.32

**Flag:** `COVER_SCENE_ENABLED` — off → ฉากแรกใช้พฤติกรรมเดิม

**เสี่ยง/หมายเหตุ:** ฉาก 0 ที่ opacity:1 ตั้งแต่เฟรมแรกต้องไม่ทำให้ transition ฉาก 0→1 กระตุก — ทดสอบใน render-sample

---

## 5. Workstream 2 — Premium Hero Image Tier (แก้ AI-slop คุม cost) 🟠

- **ฉากแรก/hero → image model พรีเมียมสุดบน kie**; ฉากอื่นคง `gpt-image-2` เดิม → พรีเมียมแค่ **~1 รูป/คลิป (~7 รูป/วัน)** = คุม cost/เวลา
- **per-clip seed family** — ทุกฉากในคลิปเดียวผูก seed/style anchor เดียว → รูปเข้าชุดกัน (แก้ "แต่ละฉากไม่เข้ากัน")
- **house-style + negative-prompt upgrade** ใน `buildScenePrompt` — camera/lens/lighting/grade ต่อธีม + negative exclusions (กัน glossy-plastic/oversaturated/stock-look/มือเบี้ยว) คง "no text in image" + safe-zone เดิม
- **Circuit breaker (ต่อ chain เดิม):** premium fail → `gpt-image-2` → `nano-banana-2-lite` → CSS background

**Flag:** `PREMIUM_HERO_IMAGE_ENABLED` — off → ทุกฉากใช้ gpt-image-2 เดิม

**⚠️ สมมติฐานต้อง verify ตอน implement:** kie มี text-to-image model ที่คุณภาพดีกว่า gpt-image-2 ชัดเจน — ต้องเช็ค catalog kie + ทดสอบคุณภาพ/เวลา/credits จริงก่อนสลับ **ถ้าไม่มีตัวที่ดีกว่าชัด = คงเดิมทั้งหมด (ไม่ regress) แล้วโฟกัส house-style/seed แทน**

---

## 6. Workstream 3 — Motion มีชีวิต (เปิด + ขยาย Motion v2) 🟡

- **เปิด `SCENE_MOTION_V2_ENABLED`** — โค้ดมีครบแล้ว: mid-scene parallax drift, entrance variety (punch/rise/slide), stat count-up
- **เพิ่ม continuous background drift เบาๆ** → ไม่มีเฟรมไหนนิ่งสนิท (รู้สึกเป็นวิดีโอ ไม่ใช่สไลด์เลื่อน)
- **กติกา 1 motion idea/ฉาก** — ไม่ยิงหลาย effect พร้อมกัน (กันรก); ใช้ motion profile ต่อธีมที่มีอยู่แล้ว
- คงกฎ render offline: GSAP bundle local, ห้าม non-determinism (`Math.random`/`Date.now` ใน render JS)

**Flag:** `SCENE_MOTION_V2_ENABLED` (เดิม)

**⚠️ ต้องทำก่อนเปิด prod:** eyeball 1 คลิป/ธีม (memory ว่า render-verified clean แต่ยังไม่เคยเปิดบน prod จริง) + ยืนยันไม่ตก protocolTimeout บน box 3-worker

---

## 7. Workstream 4 — Composition หลากหลายจริง (แก้โครงซ้ำในคลิป) 🟢

**ปัญหาราก (ตรวจแล้ว):** template รองรับแค่ 6 semantic layout ที่ content นั่งกลางๆ คล้ายกัน

**แก้:** เพิ่ม layout composition จริงใน template + ให้ scene agent เลือกต่อฉาก:
- `subject-left` / `subject-right` — รูปข้างหนึ่ง ข้อความอีกข้าง
- `full-bleed` — รูปเต็มจอ + text overlay ล่าง (scrim แรง)
- `type-only` — ตัวหนังสือใหญ่ล้วน พื้น navy/ภาพเบลอ (เหมาะ hook/quote)

**กติกา:** ฉากติดกันในคลิปเดียว **ห้าม composition ซ้ำเกิน 2 ฉากติด**
- แต่ละ layout rule แค่ขยับตำแหน่ง/ความกว้าง/alignment — element เดิม → `hyperframes inspect` (overflow/clip guard) ยังคุมได้
- scene agent เลือกจากชุดที่ธีมอนุญาต (LayoutKit ต่อธีม)

**Flag:** `COMPOSITION_VARIETY_ENABLED` — off → ใช้ layout set เดิม

---

## 8. Brand Invariants (คงเสมอทุก workstream)

badge "ADS VANCE" มุมซ้ายบน · ส้ม accent จุดเดียว (เน้นคำ/ตัวเลข/CTA เท่านั้น) · มาสคอตเสือดาวสไตล์เดียว · progress bar บน · `BrandCTA` outro · safe-zone · โครง Q&A · Thai line-height ≥1.32 · **ห้าม negative letter-spacing** · HTML-escape ก่อน render · **ห้าม text baked ในรูป AI**

---

## 9. Agents / Data Model / Migration

- **scene agent** (`internal/agent/scene.go` + prompt): บังคับ cover hook ≤7 คำโผล่เฟรมแรก + เลือก composition ต่อฉาก (ห้ามซ้ำ 2 ติด) + ใช้ `emphasis_words` จริง
- **image agent + `buildScenePrompt`** (`brand.go`): premium hero tier + house-style/negative/per-clip seed
- **critic agent** (prompt): เพิ่มเช็ค — ปกแรงใน 3 วิ, composition ไม่ซ้ำ, เกาะธีมที่เลือก
- **visual QA** (prompt): คง fail-closed เดิม + เช็คไม่มี text baked ในรูป, สี/ฟอนต์ตรงธีม
- **migration ใหม่ ต่อจาก 052** (อัปเดต agent_configs prompts ของ scene/image/critic/visual_qa)
- **ไม่เพิ่มตารางใหม่** — reuse preset key column + preset_scores เดิม; ธีม/layout นิยามใน Go (`presets.go`) เป็น source of truth

---

## 10. Feature Flags & Rollout & Fallback

| Flag | คุม | off = |
|------|-----|-------|
| `COVER_SCENE_ENABLED` | ปกเฟรมแรก | ฉากแรกพฤติกรรมเดิม |
| `PREMIUM_HERO_IMAGE_ENABLED` | รูป hero พรีเมียม | gpt-image-2 ทุกฉาก |
| `SCENE_MOTION_V2_ENABLED` (เดิม) | motion มีชีวิต | motion เดิม |
| `COMPOSITION_VARIETY_ENABLED` | layout หลากหลาย | layout set เดิม |

**Rollout เป็นเฟส (ในแผนเดียว):** build → hyperframes lint/inspect ผ่าน → **eyeball 1 คลิป/ธีม** → เปิด flag ทีละตัว → วัด retention. Flag อิสระต่อกัน ปิดหมด = output วันนี้เป๊ะ = ปลอดภัย

---

## 11. Testing / Verification

- **Go unit tests** ต่อ component: cover scene ฉาก 0 ไม่มี `opacity:0` เริ่มต้น · composition no-repeat (ไม่ซ้ำ >2 ติด) · premium fallback chain (premium→gpt-image-2→nano→css) · motion flag on/off emit ถูก
- **hyperframes lint + inspect** เป็น gate ของ layout ใหม่ (overflow/clipped-text)
- **render-sample test** + **eyeball คลิปจริง/ธีม** ก่อน flip flag บน prod
- **Success-metric loop:** retention ป้อน preset weighting (เปิด performance-weighting เป็นเฟสหลัง)

---

## 12. Non-Goals (YAGNI)

❌ เปลี่ยน render engine / แตะ TTS / publishing
❌ รื้อ template 26KB เป็น component library (= แนวทาง C, over-engineering ตอนนี้)
❌ เขียน pacing/โครงเรื่องใหม่ (คงโครง Q&A เดิม)
❌ premium image > 1 รูป/คลิป
❌ admin UI เลือกปก/ธีม (หมุนอัตโนมัติพอ)
❌ Art Director agent คิด concept อิสระ (คุมด้วยธีม/layout kit แทน)

---

## 13. สมมติฐาน & คำถามค้าง (ต้อง verify ตอน implement)

1. **premium image model:** kie มี text-to-image ที่ดีกว่า gpt-image-2 ชัดเจน (คุณภาพ/เวลา/credits รับได้) — เช็ค catalog + ทดสอบก่อนสลับ; ไม่มี = คงเดิม
2. **Motion v2 render:** สะอาดบน prod box 3-worker จริง (eyeball ก่อนเปิด)
3. **cover hook field:** content brain v2 ให้ hook ≤7 คำที่ scene แรก ใช้เป็นข้อความปกได้ (verify field `on_screen_text`/hook)
4. **cover transition:** ฉาก 0 ที่ opacity:1 ตั้งแต่เฟรมแรก ไม่ทำ transition 0→1 กระตุก (ทดสอบ render-sample)
