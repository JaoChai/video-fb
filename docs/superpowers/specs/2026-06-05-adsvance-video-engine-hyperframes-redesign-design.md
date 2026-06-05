# ADS VANCE Video Engine — Hyperframes-only Redesign (Design Spec)

วันที่: 2026-06-05
สถานะ: อนุมัติดีไซน์แล้ว (รอตรวจ spec → เขียน implementation plan)

## เป้าหมาย

รื้อระบบผลิตวิดีโอให้ **ใช้ Hyperframes เป็นทางเดียว** และยึด **แบรนด์ ADS VANCE จากโลโก้จริง** เป็น CI ของช่อง
โดยเน้น 3 โจทย์งานคราฟต์ที่ผู้ใช้สั่ง:

1. ตัดซีน (cut/transition) ให้เนียน ไม่กระตุก
2. จัดช่องไฟ (kerning/spacing) และตำแหน่ง text/object ให้ถูกหลักและสวย รวมถึง caption
3. ใช้ **GPT image (gpt-image-2)** สร้างภาพให้น่าสนใจและเข้ากับเนื้อหาช่อง

ระบบปัจจุบัน live บน prod (cron เที่ยง) → migration ทำแบบ phased ไม่ให้ของเดิมพัง

## บริบทระบบปัจจุบัน (ก่อนแก้)

- render มี 3 ทาง: multi-scene Hyperframes (`internal/producer/templates/layout_multi_scene.html.tmpl`), single-scene karaoke (`assembleHyperframes916` ใน `producer.go` + `layout_dynamic_karaoke.html.tmpl`), และ FFmpeg ภาพนิ่ง (`ffmpeg.go`) เป็น fallback
- รูปสร้างผ่าน OpenRouter โมเดล `openai/gpt-5.4-image-2` (`internal/producer/openrouter.go` → `GenerateImage`)
- TTS ผ่าน OpenRouter (`google/gemini-3.1-flash-tts-preview`); caption timing เป็น ground-truth จากบท (ไม่พึ่ง Whisper ASR)
- brand tokens อยู่ใน `internal/producer/brand.go` (`Brand`, `Motion`, `Type`, `CSSVars()`, `ImageStyleAnchor()`, `SafeZone()`) — ค่าสีปัจจุบันเป็น navy เกือบดำ (#0a1428) + ส้มอมแดง (#ff6b2b)
- agent: question / script / image / composition (single) / composition_scenes / research / dedup (`internal/agent/`)
- `openai_api_key` มีในตาราง `settings` แล้ว (ใช้โดย `internal/producer/transcriber.go`)
- เครื่องยนต์ render เป็น **single index.html + GSAP timeline เดียวต่อคลิป** — deterministic, inspect-safe, พิสูจน์แล้วว่าไม่กระตุก (เก็บไว้ ไม่รื้อ)

## การตัดสินใจที่ได้ข้อสรุปแล้ว (จาก brainstorming)

| # | หัวข้อ | สรุป |
|---|--------|------|
| 1 | Palette / CI | **Royal blue #0047AF + amber #F0A030 + navy-ink #0A2358 + white** (ค่าสุ่มจากไฟล์โลโก้จริง) |
| 2 | Caption style | **word-pop** สำหรับฉาก hook, **phrase-block** สำหรับฉากเนื้อหา |
| 3 | มาสคอต | **intro/outro bumper + pop เป็นช่วง** (ชี้สถิติ/ยกนิ้ว) → ต้องมีหลายท่า |
| 4 | GPT image | **OpenAI ตรง `gpt-image-2`** (ไม่ผ่าน OpenRouter), ใช้ `/edits` + reference สร้างมาสคอต, hero art เฉพาะฉากคุ้ม |
| 5 | Fallback | **Hyperframes ล้วน** — ตัด FFmpeg + single-scene; fail → retry → ถ้ายังไม่ได้ ข้ามคลิป + alert (ไม่มีวิดีโอสำรอง) |
| 6 | บทบาทรูป | **Hero art เฉพาะฉาก hook/stat/closing**; ฉาก content/list ใช้พื้น brand flat เพื่อให้ caption อ่านง่าย |
| 7 | ระดับรื้อ | **Refactor + retheme** — เก็บ single-timeline เดิม, รื้อฝั่ง Go เป็นโมดูล |

## ค่าสีจริงที่สุ่มจากโลโก้ (อ้างอิง)

- พื้นหลัง royal blue: `#0047AF`
- amber/ทอง (เสือชีตาห์): `#F0A030` (สว่าง), `#E8A030` (กลาง), `#C07028` (เงา)
- navy เส้นขอบ: `#0A2358` (~#082058/#082858)
- ขาว: `#FFFFFF`

## สถาปัตยกรรมเป้าหมาย

### Brand tokens ใหม่ (`brand.go`)
ชุด token เดียว เป็น source of truth, inject เข้า template ผ่าน `CSSVars()` (กลไกเดิม):

```
--blue       #0047AF   พื้นหลักของแบรนด์
--blue-hi    #1A5FD0   glow บน / ผิวยกขอบ
--blue-deep  #062F78   vignette / ขอบล่าง
--navy-ink   #0A2358   panel / การ์ด / เส้นขอบ
--amber      #F0A030   accent หลัก (เน้นคำ / CTA / badge / step number)
--amber-deep #C07028   เงา gradient ทอง
--ink        #F6F9FF   ตัวอักษรหลัก
--muted      #BCD2FF   ตัวรอง / คำอธิบาย
```
- Motion tokens (eases/durations) และ Type tokens (Sarabun + 4 น้ำหนัก) คงเดิม
- `ImageStyleAnchor()` rewrite เป็นโทน "royal blue #0047AF + amber #F0A030" (เลิกใช้ navy+orange)
- ชื่อ CSS var ในเทมเพลตทั้งสองไฟล์ต้องอัปเดตให้ตรงชุดใหม่ (ปัจจุบัน hardcode `--navy-*`, `--orange-*`)

### โมดูลฝั่ง render (Go)
```
ScenesParams ─▶ [scene-block builders]  ─▶ index.html (1 ไฟล์)
                [transition library  ]      + GSAP timeline เดียว (deterministic)
                [bumper module       ]      → hyperframes lint → inspect → render → MP4
                [brand tokens        ]
```
- **scene-block builders** — แยกฟังก์ชัน/เทมเพลตย่อยต่อ layout variant (hook_big, hook_punch, list_steps, stat_reveal, compare_two, quote_cta) เพื่อ test เดี่ยวได้
- **transition library** — รวบ wipe/iris/bar ที่มีอยู่ในเทมเพลตให้เป็นชุด transition ที่ตั้งชื่อได้ ทุกตัวใช้ **transform / opacity / clip-path เท่านั้น** (ตาม Hyperframes guide: ห้ามอนิเมต width/height/top/left ตรงๆ จะทำเฟรมค้าง) → inspect-safe, ไม่กระตุก
- **bumper module** — ฉาก intro (มาสคอตขี่จรวด ~1.0–1.5s + tagline) และ outro (มาสคอต + CTA "ติดตาม/ทักแชต")
- **ลบ** `ffmpeg.go` และ single-scene path (`assembleHyperframes916`, `layout_dynamic_karaoke.html.tmpl`, `multiscene.go` ที่ไม่ใช้แล้ว) — ตรวจ dependency ก่อนลบจริง

### Determinism (กุญแจ "ไม่กระตุก") — ยึดตาม Hyperframes
- ห้าม `Math.random()` / `Date.now()` / network fetch ตอน render (GSAP, ฟอนต์, รูป ต้องเป็น local asset — ของเดิมทำแล้ว)
- lock fps + dimensions + duration ที่รู้ค่าแน่นอน
- อนิเมตเฉพาะ transform/opacity/clip-path

## Agent ทุกตัว — การออกแบบ

### 1. Question Agent — คงเดิม
ดึงข่าว + persona → สร้าง Q&A ตาม category

### 2. Script Agent (`internal/agent/script.go`) — เพิ่ม field
เพิ่ม `role` ให้แต่ละ scene: `hook | content | stat | compare | closing`
- ใช้ขับ **caption style** (hook→word_pop, อื่นๆ→phrase_block) และ **bg_mode** (hook/stat/closing→hero, content/compare/list→flat)
- ยังคง: scene number, text_content (headline), voice_text, duration, bg_hint

### 3. Composition (Scenes) Agent (`internal/agent/composition.go` → `DecideScenes`) — สมองหลัก
ต่อฉากคืน:
- `layout_variant` (ใน 6 variant เดิม)
- `accent_color` — บังคับ sanitize ให้อยู่ในชุดแบรนด์ (default `--amber`)
- `slots[]` — role(headline/body/badge/step/stat/callout/quote/cta) + text + emphasis (ไม่มีพิกัด)
- `caption_style` — `word_pop | phrase_block` (default จาก scene.role)
- `bg_mode` — `hero | flat` (default จาก scene.role)
- `bg_art_prompt` — โจทย์ภาพ text-free (เฉพาะเมื่อ hero)
- `mascot_cue` — `none | point | thumbs | think` (ฉากไหนให้มาสคอต pop)

`Normalize()` ขยายให้ครอบคลุม field ใหม่ (default + validate)

### 4. Hero-Art Image Agent — ต่อ OpenAI ตรง gpt-image-2
- client ใหม่ `internal/producer/openai_image.go` (แทน `GenerateImage` ฝั่ง OpenRouter สำหรับ hero art)
- endpoint: `POST https://api.openai.com/v1/images/generations`, `model: "gpt-image-2"`, `quality: "high"`, `output_format: "png"`, รับ `b64_json` (decode แบบเดิม `saveBase64Image`)
- ขนาด: 9:16 = `864x1536`, 16:9 = `1536x864` (หาร 16 ลงตัว, aspect ในช่วง 1:3–3:1) — สูงขึ้นได้ถ้าต้องการ (`1152x2048` / `2048x1152`)
- prompt = `ImageStyleAnchor()` (royal blue+amber ใหม่) + subject(bg_art_prompt) + safe-zone(`SafeZone()`) + "no text/letters/logos"
- เรียกเฉพาะฉาก `bg_mode == hero`
- key: อ่านจาก settings `openai_api_key` (มีอยู่แล้ว)

### 5. Mascot Pose Library — ออฟไลน์ (ไม่ใช่ per-video)
- เครื่องมือ/สคริปต์สร้างครั้งเดียว: รับโลโก้ต้นแบบ → `POST /v1/images/edits` (`model: gpt-image-2`, `style_intensity: "high"`, `background: "transparent"`) → PNG พื้นใส 5–6 ท่า:
  `rocket` (bumper), `point_left`, `point_right`, `thumbs_up`, `think`, `wave`
- เก็บเป็น asset ใน repo (เช่น `assets/mascot/*.png`) → bumper/มาสคอต overlay อ้างชื่อท่า
- ข้อดี: คุมความเหมือน, ราคา/เวลา render คงที่, deterministic (ไม่ generate ตอน render)

### 6. Caption / Transcript — คงแนวเดิม
ใช้ ground-truth text + timing จากบท/TTS bounds; เพิ่มสวิตช์ render ตาม `caption_style`:
- **word_pop** (hook): เด้งทีละคำ, คำคีย์ (ยาวสุด) ขยาย + สีทอง — ตรรกะเดิมใน `layout_multi_scene.html.tmpl`
- **phrase_block** (content): วลีเต็มขึ้นทั้งก้อน fade เดียว, คำคีย์สีทอง + ขีดทองใต้คำ, สูงสุด 2 บรรทัด

## งานคราฟต์ 3 ข้อ — พารามิเตอร์ที่ล็อก

### A. ตัดซีนเนียน ไม่กระตุก
- cross-fade overlap `0.4s` ระหว่างฉาก
- transition library: diagonal-swipe / bar-sweep / iris (clip-path+opacity) สลับต่อ boundary
- punch-in เบา (scale 0.985→1.0→1.03→1.0) ให้มีจังหวะโดยไม่ดันของหลุดเฟรม
- hard-clear (`set opacity:0`) ปลายฉาก กัน stale frame ตอน seek

### B. ช่องไฟ/ตำแหน่ง
- safe-margin 8% รอบเฟรม
- caption ล่าง 1/3, headline กลางบน — ไม่ชนกัน
- type scale ต่อ role + `fitTextFontSize` คุมความกว้าง (มี fallback ตามความยาว string)
- letter-spacing headline `−0.01em` (hook_punch `−0.03em`), line-height 1.08–1.34 ตาม role
- caption สูงสุด 2 บรรทัด, ตัดวลีตามจังหวะพูด

### C. รูปน่าสนใจ (gpt-image-2)
- hero art เฉพาะฉากคุ้ม + scrim ทับให้ตัวอักษรอ่านออก
- style anchor ล็อกโทน royal blue+amber → ทุกคลิปเป็นชุดเดียวกัน
- มาสคอตหลายท่าเพิ่มชีวิตชีวา

## Migration (phased)

1. **Brand tokens + retheme** — เปลี่ยนค่าสีใน `brand.go` + อัปเดตชื่อ var ในเทมเพลต (เห็นผลทันที, ไม่พังของเดิม)
2. **gpt-image-2 (OpenAI ตรง)** — client ใหม่ + retheme prompt + ขนาดใหม่
3. **Mascot pose library + bumper module** — สร้าง asset + intro/outro
4. **Script `role` + Composition agent** — caption A/B + hero/flat + mascot_cue
5. **ตัด FFmpeg/single-scene + retry/alert** — เหลือทาง Hyperframes เดียว
6. **ทดสอบ render จริง 1 คลิป → ตรวจ inspect/ภาพออก → เปิด cron**

## Success Criteria (ตรวจได้)

- [ ] render 1 คลิป (9:16 + 16:9) ผ่าน lint + inspect ไม่มี clipped/overlap, เฟรมไม่ค้าง
- [ ] ทุกพื้นผิว/accent ใช้ค่าจากชุด token ใหม่ (royal blue + amber) — ไม่มี navy#0a1428/#ff6b2b หลงเหลือใน output
- [ ] caption: ฉาก hook = word-pop, ฉากเนื้อหา = phrase-block, อยู่ใน 2 บรรทัด, ในเขต safe
- [ ] มาสคอต: intro + outro bumper ขึ้นจริง + pop ตาม mascot_cue
- [ ] hero art มาจาก OpenAI gpt-image-2 (เฉพาะฉาก hero), ฉากอื่นเป็น brand flat
- [ ] ไม่มี code path FFmpeg / single-scene เหลือ; Hyperframes fail → retry → ข้าม+alert
- [ ] go build + go test ผ่าน (รวม render/golden tests ที่มี)

## ความเสี่ยง / ข้อควรระวัง

- พื้น royal blue สดกว่าพื้นเข้มเดิม → contrast caption น้อยลง: ชดเชยด้วย scrim/panel เข้ม (navy-ink) + text-shadow แรง → ต้องตรวจ inspect จริง
- ตัด fallback = ถ้า render พังจะไม่มีวิดีโอวันนั้น → ต้องมี retry + alert ที่เชื่อถือได้ ก่อนเปิด cron
- ราคา/เวลา gpt-image-2 quality:high สูงกว่าเดิม → จำกัด hero เฉพาะฉากคุ้ม
- gpt-image-2 ขนาดต้องหาร 16 ลงตัว (1920x1080 ใช้ไม่ได้ตรงๆ) → ใช้ 1536x864 แล้วให้ renderer สเกล

## อ้างอิงไฟล์ที่เกี่ยว

- `internal/producer/brand.go`, `internal/producer/templates/layout_multi_scene.html.tmpl`
- `internal/producer/openrouter.go` (image/TTS), `internal/producer/openai_image.go` (ใหม่)
- `internal/producer/producer.go`, `internal/producer/ffmpeg.go` (ลบ), `internal/producer/multiscene.go`
- `internal/agent/script.go`, `internal/agent/composition.go`, `internal/agent/image.go`
- `internal/producer/transcriber.go` (อ้างอิงรูปแบบอ่าน `openai_api_key`)
- Hyperframes docs (Context7): determinism, transform-only animation, RenderConfig fps/quality/workers
- OpenAI docs (Context7): gpt-image-2 size rules, /images/edits + reference, b64_json, style_intensity
