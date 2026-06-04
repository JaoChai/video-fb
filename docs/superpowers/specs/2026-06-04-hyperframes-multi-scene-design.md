# Multi-Scene Hyperframes Video — Design Spec

วันที่: 2026-06-04
สถานะ: รอ review ก่อนทำ implementation plan
Branch: `feat/multi-scene-video`

## 1. ที่มาและเป้าหมาย

ตอนนี้ pipeline สร้างวิดีโอ Hyperframes เป็น **single-scene + overlay**: พื้นหลังเดียวตลอดคลิป มีป้ายข้อความ (cards) เด้งทับตาม timestamp script agent ถูกล็อกให้ออกแค่ 1 scene (migration 009) และ Hyperframes path ใช้ `scenes[0]` เท่านั้น

เป้าหมาย: ยกระดับเป็น **multi-scene เล่าเรื่องตามเนื้อหา** เพื่อให้คลิปน่าสนใจขึ้น — แต่ละฉากมีพื้นหลัง/layout/ข้อความของตัวเอง เปลี่ยนฉากด้วย cross-fade โดยเสียงและ caption ยังต่อเนื่อง

### Requirements (ยืนยันกับ user แล้ว)
- Multi-scene แบบ **content-driven**: script agent ตัด 3-6 ฉากตามจังหวะเล่าเรื่อง
- ความยาว **≤ 60 วินาที** ทั้ง 16:9 และ 9:16
- **ทั้ง 16:9 และ 9:16 render ผ่าน Hyperframes** (16:9 เลิกใช้ FFmpeg static) — สคริปต์/ฉาก/เสียงชุดเดียว จัด layout ตามสัดส่วน, render 2 รอบ
- **พื้นหลัง AI (GPT-image) ทุกฉาก** (ยอมช้า/แพงกว่าเพื่อความสวย)
- แต่ละฉาก: ข้อความหลักของฉาก + karaoke caption ต่อเนื่องทับทุกฉาก
- **กัน element ทับซ้อน/ล้นจอ** เป็น requirement สำคัญ

### แนวทางที่เลือก
Approach A — **Scene-timeline เดียว**: 1 Hyperframes project ต่อสัดส่วน, แต่ละฉากเป็นช่วงเวลาบน GSAP timeline เดียว, พื้นหลัง cross-fade, เสียง+caption ต่อเนื่อง (ปฏิเสธ multi-clip concat เพราะทำ caption/voice timestamp ข้ามฉากพัง, ปฏิเสธ sub-composition เพราะ overkill/เสี่ยง)

## 2. Flow end-to-end + Scene model

```
question → script agent (ออก N ฉาก)
  ฉาก = { scene_number, scene_type(hook|problem|step|win|cta),
           headline, voice_text, bg_hint }
       ↓
  composition agent ออกแบบต่อฉาก (semantic — ดูข้อ 4) + ภาพรวม
       ↓
  เสียง: TTS ต่อฉาก → ต่อกันเป็น voice.wav เดียว
         ⇒ ได้ขอบเขตเวลาแต่ละฉากแม่นยำ (sum of per-scene durations)
       ↓
  background: GPT-image ต่อฉาก × 2 สัดส่วน (gen แบบขนาน)
       ↓
  transcribe voice.wav ทั้งไฟล์ → karaoke timestamps ต่อเนื่อง
       ↓
  render 2 รอบ: 9:16 + 16:9 (ข้อมูลฉากชุดเดียว, layout ตามสัดส่วน)
       ↓
  lint + inspect (collision gate) → upload
```

### การตัดสินใจสำคัญเรื่องเวลา
เวลาเริ่ม/จบของแต่ละฉากมาจาก **TTS แยกต่อฉากแล้ว concat** (ต่อยอด PCM→WAV chunking เดิมใน openrouter.go) — ไม่ให้ agent เดาเวลา karaoke caption ยัง transcribe ทั้ง voice.wav รวดเดียว timestamp ต่อเนื่องไม่สะดุด

### นิยาม "ฉาก"
1 ฉาก = พื้นหลัง AI ของตัวเอง + ข้อความหลัก (slots) + ช่วงเวลา (จาก TTS) โดยมี karaoke caption + progress bar วิ่งทับต่อเนื่องข้ามทุกฉาก, ระหว่างฉาก cross-fade จำนวนฉาก: script ตัดเอง target 3-5 (cap 6) รวม ≤60s

## 3. กลไกกัน Overlap (verified จาก Hyperframes 0.6.70 docs/runtime ที่ติดตั้งจริง)

หลักการ: **AI คุมความหมาย, Engine คุมเรขาคณิต, `inspect` เป็นด่านสุดท้าย**

1. **AI เลือก "ช่อง" ไม่ใช่พิกัด** — composition agent ส่ง semantic slots (role + text + emphasis) + layout variant ไม่ส่ง x/y/font-size เลย (ป้องกันต้นเหตุ overlap)
2. **Flow layout (`flex column + gap + padding`)** — Hyperframes บังคับ `.scene-content` เป็น flow ห้าม `position:absolute` กับ content container (absolute สงวนของตกแต่ง) → element ซ้อนกันไม่ได้ by construction; padding ปรับตามสัดส่วน ใช้ template เดียวทั้ง 2 ratio
3. **`window.__hyperframes.fitTextFontSize(text, opts)`** (มีจริง, return `{fontSize, fits}`) — ย่อ font ข้อความยาวให้พอดี ถ้า `fits:false` → wrap/ตัด ขยายใช้กับ headline/body (ปัจจุบันใช้แค่ caption)
4. **โซนสงวน** — caption band (9:16 ~600-700px จากล่าง / 16:9 ~80-120px) + progress bar จองพื้นที่แยกจากเนื้อหาฉาก
5. **`hyperframes inspect` (alias `layout`) เป็น gate** — รันใน headless Chrome ไล่ timeline จับ `canvas_overflow` / `container_overflow` / `clipped_text` ถ้าเจอ → fallback layout ปลอดภัยกว่า (ปัจจุบัน renderer รันแค่ `lint` จะเพิ่ม `inspect`)

หมายเหตุ (honest): Hyperframes **ไม่มี** auto-layout constraint-solver หรือ safe-zone API อัตโนมัติ — safe-zone = ตัวเลข + padding manual การการันตีไม่ overlap มาจาก flow layout + inspect gate

## 4. เปลี่ยน 2 Agents

### Script agent (migration ใหม่ แทน 009)
prompt ใหม่: แตกคำตอบเป็น 3-6 ฉากตามจังหวะเล่าเรื่อง, รวม voice ≤60s แต่ละฉากคืน:
```json
{ "scene_number": 1, "scene_type": "hook|problem|step|win|cta",
  "headline": "ข้อความใหญ่บนจอ (สั้น)",
  "voice_text": "บทพากย์ของฉากนี้",
  "bg_hint": "ใบ้บรรยากาศพื้นหลัง" }
```

### Composition agent (semantic เท่านั้น)
input: ฉากทั้งหมด + ภาพรวม → output ต่อฉาก:
```json
{ "scene_number": 1,
  "layout_variant": "hook_big|list_steps|stat_reveal|quote_cta",
  "slots": [{ "role": "headline|body|badge|step", "text": "...", "emphasis": ["คำ"] }],
  "accent_color": "#rrggbb", "bg_art_prompt": "art ไม่มีตัวหนังสือ",
  "animation_speed": "fast|normal|slow" }
```
+ ภาพรวม: brand, kicker, highlight words **ไม่มีพิกัด/ฟอนต์ไซส์**

### การแบ่งงาน
- script = "พูดอะไร / แบ่งฉากไหน / voice ต่อฉาก / ใบ้ bg"
- composition = "แต่ละฉากหน้าตายังไง (layout + slot + สี + bg prompt)"
- เวลาฉาก = จาก TTS ต่อฉาก (ไม่ใช่ agent)
- caption = Whisper บน voice.wav

### Layout library
เริ่ม 3-4 variant (`hook_big`, `list_steps`, `stat_reveal`, `quote_cta`) เป็น flex-column template เพิ่มทีหลังได้ analyzer เดิมเรียนรู้ได้ว่า variant ไหน retention ดี (ต่อยอด `composition_style`)

## 5. Render + Error Handling + Test

### Template ใหม่ (1 template คุม 2 สัดส่วน)
- พื้นหลังหลายชั้น (1/ฉาก) — GSAP cross-fade ระหว่างฉากบน timeline เดียว
- เนื้อหาฉาก = `.scene-content` flex-column โผล่/หายตามช่วงเวลาฉาก
- caption band + progress bar ต่อเนื่องทับทุกฉาก
- padding/ขนาดปรับตามสัดส่วน (ไม่ hardcode 1080/1920)

### Backgrounds
GPT-image ต่อฉาก × 2 สัดส่วน gen แบบขนาน (errgroup) ลด latency, prompt จาก `bg_art_prompt`

### 2 สัดส่วน
render 2 รอบจากข้อมูลฉากชุดเดียว — 16:9 เปลี่ยนจาก FFmpeg static มาเป็น Hyperframes

### คุม ≤60s
prompt script กำกับ + ตรวจหลัง TTS จริง (รวมทุกฉาก) ถ้าเกิน → ย่อ/ตัดฉากท้าย

### Error handling (safety net หลายชั้น)
- bg AI ฉากไหน gen ไม่ได้ → ฉากนั้นใช้ CSS bg
- composition agent คืนเพี้ยน → default layout
- `inspect` เจอ overlap/ล้น → retry layout ปลอดภัยกว่า
- render ล้มทั้งหมด → คง FFmpeg static เป็น last-resort (ต่อสัดส่วน) ไม่ให้เสียคลิป

### Testing
- unit: template render N ฉาก, ไม่มี `{{}}` ค้าง, slots ครบ, fitText fallback, ≤60s logic
- golden test (ต่อยอด composition_test เดิม)
- `hfslice` CLI ขยาย render multi-scene local + รัน `inspect` ดูผลจริงก่อน deploy
- pipeline: lint + inspect เป็น gate

## 6. ลำดับ build (คร่าว — writing-plans จะลงละเอียด)
1. script agent multi-scene (migration ใหม่ + struct + validate)
2. composition agent slots (migration prompt + struct)
3. template ใหม่ + layout variants (flex-column, cross-fade)
4. producer/builder (per-scene TTS concat, per-scene/-ratio bg, 2 renders)
5. inspect gate (เพิ่มใน HyperframesRenderer)
6. test + hfslice multi-scene runner

## 7. Out of scope
- ไม่เพิ่ม layout variant เกิน 3-4 ตัวในเฟสแรก
- ไม่ทำ sub-composition / multi-clip concat
- ไม่แตะ publish/scheduler flow (คงเดิม)
- ไม่ทำ auto-layout constraint solver (พึ่ง flow layout + inspect)

## 8. ความเสี่ยง
- Template rewrite เป็นงานใหญ่สุด (cross-fade หลาย bg + per-scene flow + 2 ratio) — เสี่ยง visual regression ต้องดูจาก render จริง + inspect
- ค่า API สูงขึ้น (GPT-image หลายรูป/คลิป × 2 ratio) + render นานขึ้น (2 รอบ) — ยอมรับแล้ว
- per-scene TTS แล้ว concat ต้องคุม gap/ความเงียบระหว่างฉากให้เนียน
