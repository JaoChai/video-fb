# Hyperframes Video Pipeline — Architecture Design

> ออกแบบการเปลี่ยน video pipeline จาก FFmpeg static-image → HeyGen Hyperframes (HTML→MP4 animation)
> สถานะ: design (รอ confirm ก่อน implement) — PoC สำเร็จแล้วที่ `hyperframes-poc/poc-video/`

## เป้าหมาย (จาก user)
1. วิดีโอสวยขึ้น — animation + caption แทนภาพนิ่ง
2. เก็บ GPT image (`gpt-5.4-image-2`) ไว้ แต่เปลี่ยน role เป็น gen "background art สะอาด" ให้ Hyperframes วาด text ทับ
3. agent มี skills สร้าง design/template แบบใหม่ได้เอง (ไม่ใช่ template ตายตัว)

## Composition Strategy: Hybrid (เลือกแล้ว)

เทียบ 3 ทาง:
- **(A) Go template ตายตัว** — เสถียร แต่ไม่ creative → ผิดเจตนา user
- **(B) LLM gen HTML ทั้งไฟล์** — creative สุด แต่ Hyperframes HTML มี constraint เยอะ (data-attributes, timeline registration) LLM พลาดสูง retry แพง → ไม่เหมาะ production
- **(C) Hybrid ✅** — Go มี template library (4-5 layout) + composition agent (LLM) เลือก layout + กำหนด design params (สี accent, การ์ด, จังหวะ, highlight) → Go assemble → lint → fallback

**Creativity มาจาก:** agent กำหนด layout variant + accentColor + cardType + animationSpeed + highlightWords + card content ต่อคลิป + design skills ที่ evolve ตาม performance

## Data Flow ใหม่

```
question → script → image(ROLE CHANGED: background art สะอาด) → GPT image x2
                 ↓
              TTS → voice.wav
                 ↓
   [NEW] transcribe (whisper th) → phrase segments
                 ↓
   [NEW] composition agent (LLM) → CompositionParams {layout, cards, accent, highlights}
                 ↓
   [NEW] composition builder (Go html/template) → index.html + assets
                 ↓
   [NEW] hyperframes lint → fail? → fallback template
                 ↓
   [NEW] hyperframes render → MP4 (916 + 169)
                 ↓
              upload Kie AI (เดิม ไม่แตะ)
```

จุดเสียบ: `producer.go:112-133` (แทน FFmpeg) — upload stage เดิมไม่ต้องแก้

## ไฟล์ที่ต้องทำ

**สร้างใหม่:**
- `internal/agent/composition.go` — CompositionAgent + CompositionParams/CardSpec/TranscriptSegment
- `internal/producer/composition_builder.go` — Build() + BuildFallback()
- `internal/producer/hyperframes.go` — Render() + Lint() (shell out npx)
- `internal/producer/transcriber.go` — interface + impl (OpenAI Whisper API / local)
- `internal/producer/templates/layout_*.html.tmpl` — 4-5 layout (จาก PoC) + default fallback
- `migrations/021_hyperframes_pipeline.sql` — composition agent + design_templates table + composition_style column + image prompt ใหม่

**แก้ไข:**
- `internal/producer/producer.go` — Produce() แทน assembly block, เพิ่ม feature flag `HYPERFRAMES_ENABLED`
- `cmd/server/main.go` — instantiate components ใหม่
- `internal/orchestrator/orchestrator.go` — load composition agent config
- `internal/analyzer/analyzer.go:99` — allow "composition" agent ใน self-improvement
- `internal/config/config.go` — เพิ่ม config fields
- `Dockerfile` — Node 22 + Chromium + Sarabun fonts

## Image Agent Role Change
- prompt เดิม (migration 011): "chat bubble + Thai text ฝังในภาพ" → ภาพมี text ทับไม่ได้
- prompt ใหม่: "clean background art, NO text, NO UI, dark gradient, brand colors, cinematic"
- เก็บ model `gpt-5.4-image-2` เดิม → Hyperframes วาด text/caption/animation ทับ = ภาพสวย + ตัวอักษรคม

## Self-Improvement Extension (design skills เรียนรู้เอง)
- เพิ่ม `composition_style` column ใน clips (track ว่าคลิปใช้ design อะไร)
- analyzer (`analyzer.go:99`) เพิ่ม "composition" ใน allowed set → analytics agent เขียน design insights ได้
- gatherData() join composition_style → LLM เห็นว่า design ไหน retention ดี
- guardrail เดิมใช้ได้ (style-only ≤1000 chars) + ห้ามเปลี่ยน brand colors
- ผล: design ของ composition agent ปรับตาม performance อัตโนมัติ (เหมือน insights loop เดิม)

## Design Skills (human-written, composition agent)
```
- เลือก layout ตามเนื้อหา: qa→intro_cards, news→news_headline, tips→tips_list, case→story_reveal
- accent color ตามอารมณ์: ปัญหา=แดง, โอกาส=เขียว, เทคนิค=ส้ม
- highlight คำสำคัญในชื่อ ≤2 คำ
- animation speed: เร่งด่วน→fast, เทคนิค→normal, เรื่องเล่า→slow
- card body ≤15 คำ/ใบ
```

## Infra (honest risk)
- **Transcribe:** แนะนำ OpenAI Whisper API ($0.003/คลิป) — local large-v3 (3GB RAM) เสี่ยง OOM บน Railway ที่แชร์ memory กับ Chrome. ทางเลือก: DGX Spark (GPU server ที่มีอยู่) ถ้าตั้ง whisper endpoint
- **Render:** Dockerfile เพิ่ม Node 22 + Chromium (image ~800MB→~2GB), Chrome กิน ~400-600MB/process, render ~30s/resolution. แนะนำ Railway RAM ≥1GB, render sequential ก่อน (916 แล้ว 169) กัน OOM
- **Reliability:** lint guardrail + fallback template + ถ้าพังหมด fallback FFmpeg เดิม (feature flag) → ไม่ทิ้งคลิป

## Phases (ship ทีละส่วน)
1. **Image role change** (low risk) — เปลี่ยน prompt → ภาพ background สะอาด
2. **Transcribe** — เพิ่ม transcriber (OpenAI Whisper API)
3. **Templates + builder** — 1 layout จาก PoC ผ่าน lint ก่อน แล้วเพิ่ม
4. **Renderer + lint guard** — shell out + fallback flow
5. **Composition agent** — LLM เลือก params
6. **Wire producer** — feature flag, deploy false ก่อน → switch true
7. **Dockerfile + Railway** — Node+Chrome, monitor memory
8. **Self-improvement** — analyzer extension + design_templates

แต่ละ phase ship ได้อิสระ (feature flag กัน production พัง)
