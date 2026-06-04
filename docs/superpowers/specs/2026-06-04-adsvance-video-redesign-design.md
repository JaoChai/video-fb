# ADS VANCE Video Redesign — Content-driven · Kinetic · On-brand

วันที่: 2026-06-04
สถานะ: รอ review ก่อนทำ implementation plan
ขอบเขต: multi-scene path เท่านั้น (single-scene ไม่แตะ), ไม่ใช้มาสคอตเสือดาวรอบนี้

## 1. ที่มาและเป้าหมาย

วิดีโอที่ออกจาก production ตอนนี้ "จืด ไม่น่าสนใจ และภาพไม่เกี่ยวกับเนื้อหา" ทั้งที่มี image-gen agent อยู่แล้ว จากการตรวจโค้ดจริงพบ root cause 4 ข้อ:

1. **ภาพไม่อิงเนื้อหา** — `backgroundArtPrompt(category)` (`producer.go:655`) ใช้ motif ตายตัวต่อหมวด (pixel→"glowing data tracking nodes" เหมือนกันทุกคลิป) และ multi-scene สร้าง `bg_art_prompt` จาก `bg_hint` (แค่ประเภทฉาก) ไม่ได้ดู `VoiceText` จริง + prompt บังคับ "NO text, abstract" → ได้ภาพ abstract ลอย ๆ
2. **ภาพถูกบังจนไม่เห็น** — scrim opacity 0.72–0.8 ใน `layout_multi_scene.html.tmpl` → ภาพเห็น ~20-30%
3. **design = lookup ตาราง** — สี 3 ตัวตายตัวตามอารมณ์, layout 4 แบบบังคับตามประเภทฉาก, CSS ไม่ได้ทำให้ variant ต่างกันด้วยสี
4. **แบรนด์/ลูกเล่นหาย** — ไม่มี kinetic typography, ไม่มี data callout (ตัวเลขวิ่ง), ไม่มี parallax/transition ที่มีจังหวะ

เป้าหมาย: ยกเครื่องชั้น "ออกแบบภาพ" ให้ **อิงเนื้อหาที่พูดจริง** + **มีชีวิต/จังหวะ** + **คุมโทนแบรนด์ทุกคลิป** โดยทำเป็น phase เห็น demo จริงทุกเฟส

### Decisions (ยืนยันกับ user)
- Scope = **C เต็มสูบ** (prompt+agent + template + design system + hook)
- **ไม่ใช้มาสคอต**เสือดาวรอบนี้
- โทนสี: **navy+orange เป็นหลัก** + สี semantic เดิม (warn `#ff5a52` / win `#2fd17a` / info `#3b82f6`) ใช้อย่างมีจุดหมาย — ไม่เพิ่มสีแบรนด์ใหม่
- validate ทุกเฟสด้วย **rendered demo** จริง (ต่อยอดจาก `caption_demo_render_test.go`)

## 2. องค์ประกอบ 5 ชั้น (contracts)

### 2.1 Brand Design System — แหล่งความจริงเดียว
ไฟล์ใหม่ `internal/producer/brand.go`:
```go
type BrandTokens struct {
    Colors  BrandColors   // navy เฉด, orange family, ink/muted, semantic
    Motion  MotionTokens  // ชุด easing + duration มาตรฐาน
    Type    TypeScale     // font, น้ำหนัก, scale ต่อ role
    SafeZone SafeZone     // inset 9:16/16:9 (กัน UI แพลตฟอร์มบัง)
}
var Brand = BrandTokens{ ... }      // ค่าคาโนนิคัลตัวเดียว
func (b BrandTokens) CSSVars() template.CSS        // ฉีดเข้า template
func (b BrandTokens) ImageStyleAnchor() string     // ฉีดเข้า prompt รูป (ข้อ 2.2)
```
- template ทุกตัวดึงสี/easing จากตรงนี้ (เลิก hardcode สีกระจายในแต่ละ tmpl)
- prompt รูปดึง style anchor จากตรงนี้ → ภาพทุกคลิปสไตล์เดียวกัน
- **invariant:** template และ prompt ต้องอ้าง token ชุดเดียวกัน — ห้าม hardcode สีซ้ำ

### 2.2 ภาพอิงเนื้อหาจริง (แก้ root cause #1, #2)
- **ทิ้ง** motif map ตายตัวใน `backgroundArtPrompt`
- composition agent (ข้อ 2.3) อ่าน `VoiceText` ของฉาก → output `concept` (subject 1 บรรทัดที่สื่อเนื้อหา เช่น "Facebook Ads Manager dashboard, rising performance graph, red blocked-account banner")
- ฟังก์ชันใหม่ Go-side ประกอบ prompt 3 บล็อก (ไม่พึ่ง LLM ให้จำ style):
  ```go
  func buildScenePrompt(b BrandTokens, concept, aspect string) string
  // = ImageStyleAnchor(brand, ล็อก) + "Subject: "+concept + compositionBlock(aspect)
  ```
  - **style anchor (ล็อก ทุก prompt):** flat editorial vector, navy `#0a1428` + orange `#ff6b2b`, soft lighting, no gradients-unless-essential
  - **subject (เปลี่ยนตามฉาก):** concept จาก agent
  - **composition (ล็อก):** เว้น negative space โซนตัวอักษรตาม safe-zone, uncluttered, no text/letters/logos
- **scrim ฉลาดขึ้น:** เปลี่ยนจากม่านดำเต็มจอ เป็น scrim เฉพาะโซนตัวอักษร (บน+ล่าง) ลด opacity โซนกลางให้ภาพเป็นพระเอก — แก้ใน `layout_multi_scene.html.tmpl`
- GenerateImage คงเดิม (`openai/gpt-5.4-image-2`, 2K, `openrouter.go`)

### 2.3 Design Agent ครีเอทีฟขึ้น (แก้ root cause #1, #3)
แก้ผ่าน **migration ใหม่** (อัปเดต `agent_configs` row ของ `composition_scenes` — system/skills/prompt_template) + ปลดล็อกฝั่ง Go:
- ป้อน **VoiceText จริงต่อฉาก** เข้า prompt (ปัจจุบันส่งแค่ headline+type+bg_hint ใน `producer.go:441-450`)
- ให้ agent คิด: `concept` (สำหรับภาพ), เลือก `layout_variant` ตาม**ความหมาย**เนื้อหา, เลือก `accent_color` จาก palette แบรนด์เพื่อ contrast/จังหวะ (ไม่ใช่ lookup อารมณ์ตายตัว)
- เพิ่ม layout variant: `compare_two` (เทียบ 2 ฝั่ง), `hook_punch` (ข้อ 2.5) → ขยาย `validLayoutVariants` + `Normalize()` ใน `composition.go`
- เพิ่ม slot role: `stat`, `callout` (ข้อ 2.4) → ขยาย `validSlotRoles`
- **invariant คงไว้:** ห้ามให้ agent กำหนดพิกัด/ขนาด (ระบบจัดวางกัน overlap — inspect gate เดิมยังทำงาน)

### 2.4 Template มีชีวิต/จังหวะ (แก้ root cause #4)
แก้หลักที่ `layout_multi_scene.html.tmpl` + CSS/JS helper:
- **Kinetic typography ราย คำ** — caption ปัจจุบันเด้งทั้งวรรค (`seg.text` fade) → แตกเป็นคำ stagger reveal + ไฮไลต์คำสำคัญ (gradient/เรืองแสง) กระจายเวลาในช่วง `[seg.start, next.start]` (presentation-only ต่อยอดจาก segment timing ที่เพิ่ง fix)
- **Punch-in zoom / pacing** — มี visual event ทุก ~1.5-3 วิ (zoom เบา ๆ ต่อ caption/ต่อช่วง)
- **Parallax** — bg image กับชั้นเนื้อหาเลื่อนคนละ rate (ต่อยอด Ken Burns เดิม `scale 1.0→1.12`)
- **Data callout components** (CSS/SVG, สีแบรนด์) — counter วิ่งขึ้น, ลูกศร, แท่งเทียบ; render จาก slot role `callout`/`stat`
- **Transition แบรนด์ระหว่างฉาก** — ปัจจุบัน cross-fade 0.5s → เพิ่มชุด transition (ส้มกวาด/shape wipe) ใช้ brand easing หมุนเวียนต่อ boundary
- **CSS ต่อ variant จริง** — ทำให้ data-layout 4-6 แบบหน้าตาต่างกันชัด (ปัจจุบัน CSS ไม่แยก)

### 2.5 Hook 3 วิแรก (แก้ engagement)
- layout variant `hook_punch`: ตัวใหญ่สุด เคลื่อนไหวเร็วสุด อ่านรู้เรื่องแม้ปิดเสียง pattern-interrupt
- agent ดึงประโยคเด็ดจากสคริปต์ใส่ฉากเปิดอัตโนมัติ (ฉากแรกของ multi-scene)

## 3. ลำดับสร้าง (phased — จบแต่ละเฟส = render demo จริง)

| Phase | ทำ | ไฟล์หลักที่แตะ | Demo พิสูจน์อะไร |
|------|-----|----------------|------------------|
| **1** | brand.go tokens + buildScenePrompt อิงเนื้อหา + agent ป้อน VoiceText/concept + ลด scrim | `brand.go`(new), `producer.go`, migration ใหม่, `layout_multi_scene.tmpl`(scrim) | ภาพตรงเนื้อหา + เห็นภาพชัดขึ้น |
| **2** | kinetic typography + punch-in/parallax + transition แบรนด์ | `layout_multi_scene.tmpl`, JS helper | "จืด" → มีชีวิต/จังหวะ |
| **3** | data callout/icon components + agent ปลดล็อก layout/slot/สี | `layout_multi_scene.tmpl`, `composition.go`, migration | ตัวเลข/คอนเซ็ปต์สื่อชัด, design หลากหลาย |
| **4** | hook_punch + จังหวะรวม + เก็บงาน design-system (เลิก hardcode สีที่ค้าง) | `composition.go`, `layout_multi_scene.tmpl`, `brand.go` | 3 วิแรกดึงดูด, คุมโทนทั้งคลิป |

แต่ละเฟส: build + `go test ./...` เขียว, `lint`+`inspect` ผ่าน (collision gate), render demo MP4, ส่งให้ user ดู → ค่อยเฟสถัดไป

## 4. Testing strategy
- **Unit (offline, `go test ./...`):** brand CSSVars/ImageStyleAnchor stable, buildScenePrompt มี 3 บล็อกครบ, agent decode/Normalize รับ variant/slot ใหม่, sanitize สีคงทำงาน
- **Render harness (gated):** ต่อยอด `caption_demo_render_test.go` → demo ต่อเฟส ใช้เสียง+transcript จริงของ poc, `lint`+`inspect`+`render`
- **inspect gate เดิม** ต้องไม่พัง (กัน element ทับ/ล้นจอ)

## 5. นอกขอบเขต / ความเสี่ยง
- **นอกขอบเขต:** มาสคอต, single-scene path, เปลี่ยน TTS/รุ่นโมเดลรูป
- **ความเสี่ยง:**
  - inspect gate อาจ fail ถ้า layout/slot ใหม่ทำ overflow → ต้องทดสอบทุกเฟส
  - kinetic ราย คำ ของไทยต้องตัดตาม grapheme (สระ/วรรณยุกต์ห้ามหลุดจากพยัญชนะ) — ใช้แนวเดียวกับที่วิจัยไว้ (`Intl.Segmenter` grapheme)
  - prompt รูปอิงเนื้อหา = ผล AI ไม่แน่นอน → ต้องมี fallback CSS bg เดิม (retry 3 ครั้งยังคงไว้)
  - การเปลี่ยน prompt agent ผ่าน DB migration ต้องเป็น additive/มี rollback (ตาม safe-migration)

## 6. Definition of Done
- 4 เฟสจบ, ทุกเฟสมี demo MP4 ที่ user รับ
- `go test ./...` เขียว, lint+inspect ผ่านทุก demo
- ไม่มีสี hardcode ซ้ำซ้อนนอก `brand.go` (design-system เป็นแหล่งเดียว)
- ภาพทุกฉากอิง concept จาก VoiceText (เลิก motif ตายตัวต่อหมวด)
