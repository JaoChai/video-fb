# Render Checks — มองเห็นด่าน QA ของ Hyperframes ก่อนจะเพิ่มความเข้ม

วันที่: 2026-08-01
สถานะ: spec (ยังไม่ลงมือ)

## ปัญหา

ระบบผลิตคลิปเรียก Hyperframes CLI 3 ด่าน แต่**เรารู้ผลของด่านเหล่านั้นแทบไม่ได้เลย**

ข้อเท็จจริงที่ตรวจสอบแล้วจากโค้ดและ prod (2026-08-01):

1. **`Lint()` ไม่เคยถูกเรียกในสายการผลิต** — `internal/producer/hyperframes.go:110` มีเมธอดพร้อมคอมเมนต์ว่า
   "use it as a guardrail before Render" แต่ call site เดียวที่มีอยู่คือ
   `internal/producer/render_sample_test.go:104` สายการผลิตจริง (`producer.go:495`) เรียกแค่
   `Inspect()` แล้วไป `Render()` เลย

2. **เหตุผลที่ inspector ฟ้องถูกล้างทิ้งทันที** — `internal/orchestrator/orchestrator.go:806`
   เขียน `SetFailReason("layout inspector: …")` ตามคอมเมนต์ที่ระบุว่า log หายภายในไม่กี่ชั่วโมง
   และถ้าไม่เก็บไว้ คนถัดไปต้องเรนเดอร์ซ้ำเพื่อหาสาเหตุ (บทเรียนจาก 2026-07-25)
   แต่บรรทัด 826 เรียก `ClearFailReason` แบบไม่มีเงื่อนไข
   (`UPDATE clips SET fail_reason = NULL`) ในทุกเส้นทาง — เหตุผลที่เพิ่งเขียนถูกล้างทิ้ง
   20 บรรทัดถัดมา

3. **ผลที่ตามมา: ประเมิน gate ไม่ได้** — query prod ย้อนหลัง 35 วัน (Neon `snowy-grass-75448787`)
   ได้คลิป 100 ตัว สถานะ `published` ทั้งหมด, `fail_reason ILIKE 'layout inspector%'` = 0 แถว
   ตัวเลข 0 นี้พิสูจน์ไม่ได้ว่า inspector ไม่เคยจับอะไรได้ เพราะข้อ 2 ลบหลักฐานทิ้งทุกครั้ง
   ในช่วงเดียวกัน visual QA (LLM) fail 21 ครั้ง (รายสัปดาห์ 11 → 7 → 1 → 2 → 0)

ผลรวม: ด่านที่ถูกที่สุดและ deterministic ที่สุด (lint) ไม่ทำงาน ด่านที่รองลงมา (inspect)
วัดผลไม่ได้ และภาระการจับตำหนิตกอยู่กับ LLM ซึ่งแพงกว่าและไม่แน่นอนกว่า

## ขอบเขต

งานนี้ทำ **เฟส 1 เท่านั้น**: ทำให้ผลของทั้ง 3 ด่านมองเห็นได้ โดยไม่เปลี่ยนพฤติกรรมการเผยแพร่
แม้แต่คลิปเดียว

**อยู่ในขอบเขต**
- เรียก `Lint()` ในสายการผลิตแบบ shadow (บันทึกผล ไม่บล็อกอะไร)
- เก็บผลทั้ง 3 ด่านลงตารางใหม่ `render_checks`
- แก้บั๊ก `ClearFailReason` ล้างทับเหตุผลที่เพิ่งเขียน

**นอกขอบเขต (ตัดสินใจแล้วว่าไม่ทำรอบนี้)**
- ไม่อัปเกรด Hyperframes CLI (ยัง pin 0.6.70 ทั้ง `Dockerfile:52`,
  `hyperframes.go:13`, `composition_builder.go:22`)
- ไม่แตะ `RENDER_ERROR_GATE_ENABLED` (ตรวจแล้วว่าไม่ได้ตั้งค่าบน Railway = ปิดอยู่)
- ไม่มี endpoint หรือหน้าเว็บใหม่ — เฟสนี้อ่านผลผ่าน SQL โดยตรง
- ไม่แตะเทมเพลต `layout_multi_scene.html.tmpl`

## แนวทางที่เลือก

ต่อยอดโครงสร้างเดิมตรงๆ ไม่สร้างชั้น abstraction ใหม่ (แนวทาง A จาก 3 ตัวเลือกที่พิจารณา)

เหตุผล: ทั้ง 3 เฟสใช้ตำแหน่งโค้ดเดียวกัน — lint รันก่อนเรนเดอร์เสมอ ต่างกันแค่ "ทำอะไรกับผล"
(เฟส 1 บันทึก → เฟส 2 ให้ fail → เฟส 3 ปรับความเข้ม) การแยกเป็นชั้นใหม่จึงเป็น abstraction
สำหรับผู้ใช้รายเดียว ซึ่ง CLAUDE.md ห้ามไว้ตรงๆ

ทางเลือกที่ปฏิเสธ: รัน lint แบบ async หลังเรนเดอร์ (ไม่เพิ่ม latency แต่ขัดกับเป้าหมาย
fail-fast ของเฟส 2 และจะต้องย้ายโค้ดใหม่ทั้งหมดในเฟสถัดไป)

## สถาปัตยกรรม

### Schema — `migrations/079_render_checks.sql`

append-only ตามแบบ `045_auto_review.sql` (idempotent ไม่มี goose syntax)

```sql
CREATE TABLE IF NOT EXISTS render_checks (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clip_id     UUID NOT NULL REFERENCES clips(id) ON DELETE CASCADE,
    stage       TEXT NOT NULL,               -- lint | inspect | render
    passed      BOOLEAN NOT NULL,
    duration_ms INT NOT NULL DEFAULT 0,
    findings    JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_render_checks_clip_id ON render_checks (clip_id);
CREATE INDEX IF NOT EXISTS idx_render_checks_stage_created ON render_checks (stage, created_at DESC);
```

`duration_ms` มีไว้ตอบคำถามที่ยังไม่มีใครวัด: `hyperframes lint` กินเวลาเท่าไรกับเทมเพลตของเรา

ปริมาณข้อมูล: 3 แถว/คลิป × 3 คลิป/วัน ≈ 9 แถว/วัน — ไม่ต้องมีนโยบายลบ

### ไฟล์ที่แตะ

| ไฟล์ | การเปลี่ยนแปลง |
|---|---|
| `migrations/079_render_checks.sql` | ใหม่ |
| `internal/repository/renderchecks.go` | ใหม่ — `Create()` ตามแบบ `visualqa.go` |
| `internal/producer/hyperframes.go` | `Lint`/`Inspect`/`Render` คืน `CheckResult`; lint ได้ timeout ของตัวเอง |
| `internal/producer/producer.go` | เรียก `Lint` ก่อน `Inspect`; พก `[]CheckResult` ขึ้นไปใน `ProduceResult` |
| `internal/orchestrator/orchestrator.go` | เขียน `render_checks`; แก้ `ClearFailReason` |

### `CheckResult`

```go
type CheckResult struct {
    Stage      string   // lint | inspect | render
    Passed     bool
    DurationMS int
    Findings   []string
}
```

โครงเดียวใช้ได้ทั้ง 3 ด่าน `Findings` ของด่าน render คือผลของ `scanBrowserIssues` ที่มีอยู่แล้ว

### Timeout ของ lint

`HyperframesRenderer.timeout` ปัจจุบันคือ 20 นาที ซึ่งตั้งไว้สำหรับการเรนเดอร์
(เหตุผลอยู่ในคอมเมนต์: workers 3 ตัวแลก wall-clock กับความเสถียร)

lint เป็น static analysis ไม่เปิดเบราว์เซอร์ จึงได้ timeout แยกที่ **2 นาที** — ด่านที่ไม่มีสิทธิ์
บล็อกอะไรเลยต้องไม่มีสิทธิ์หน่วงสายการผลิตด้วย

## Data flow

```
AssembleHyperframes916()
  BuildScenes()
    ↓
  Lint()      ← ใหม่ (timeout 2 นาที)          → CheckResult{stage:"lint"}
    ↓  เฟส 1: ผลลัพธ์ไม่มีผลต่อการไหลของงาน
  Inspect()                                     → CheckResult{stage:"inspect"}
    ↓  พฤติกรรมเดิมทุกประการ
  Render()                                      → CheckResult{stage:"render"}
    ↓
  assembleOutput.checks []CheckResult
    ↓
  ProduceResult.Checks
    ↓
orchestrator: renderChecksRepo.Create() × 3 (non-fatal)
```

พฤติกรรมที่ **ไม่เปลี่ยน**:
- `InspectFlagged` → `downgradeIfReady` → `needs_review` เหมือนเดิม
- `RenderFlagged` ยังขึ้นกับ `RENDER_ERROR_GATE_ENABLED` เหมือนเดิม
- visual QA / auto-review / audio gate ไม่ถูกแตะ

### การแก้ `ClearFailReason`

```go
if status == "ready" {
    o.clipsRepo.ClearFailReason(ctx, clipID)
}
```

เจตนาเดิม (ล้างเหตุผลเก่าเมื่อคลิปกลับมาดี) ยังอยู่ครบ เปลี่ยนแค่ไม่ล้างทับเหตุผลที่เพิ่งเขียน
ในรอบเดียวกัน

ผลข้างเคียงที่ตรวจแล้ว: `fail_reason` ถูกใช้เพื่อ**แสดงผลอย่างเดียว** ไม่มี logic ใดอ่านค่านี้
เพื่อตัดสินใจ (`OverviewTab.tsx:18` แสดงเสมอ, `Content.tsx:421` แสดงเฉพาะ `status === 'failed'`)
การแก้นี้จึงทำให้คลิป `needs_review` มีเหตุผลแสดงบนหน้า clip detail ซึ่งเป็นเจตนาเดิมของโค้ด
และไม่กระทบเส้นทางตัดสินใจใด

## Error handling

| สถานการณ์ | พฤติกรรม |
|---|---|
| lint พบปัญหาในเทมเพลต | `passed=false`, `findings` = บรรทัดที่ CLI ฟ้อง → บันทึกแล้วไปต่อ |
| รัน lint ไม่ได้ (CLI หาย / timeout / npx ล้ม) | `passed=false`, `findings[0]` ขึ้นต้นด้วย `runner_error:` → บันทึกแล้วไปต่อ |
| เขียน `render_checks` ไม่สำเร็จ | log non-fatal เหมือน `visualQARepo.Create` — คลิปไม่ตกเพราะตารางสถิติ |
| ด่านใดด่านหนึ่งล้ม | ด่านถัดไปยังรันต่อ เพื่อให้ได้ข้อมูลครบ 3 ด่านทุกคลิป |

การแยก `runner_error:` ออกจาก finding จริงเป็นเงื่อนไขจำเป็นของเฟส 2 — ไม่งั้นตอนเปิด gate
คลิปจะถูกบล็อกเพราะ npx ล่ม ซึ่งคนละเรื่องกับเทมเพลตพัง

## Testing

| เทสต์ | ระดับ |
|---|---|
| `CheckResult` แยก 3 กรณี: ผ่าน / เจอ finding / runner error | unit ล้วน ไม่แตะ CLI |
| `ClearFailReason` ไม่ถูกเรียกเมื่อ `status=needs_review` | ต่อยอด `internal/orchestrator/status_gate_test.go` |
| `RenderChecksRepo.Create` mapping | unit ตามแบบ `critiques_test.go` (ไม่แตะ DB จริง) |
| lint จริงกับเทมเพลตจริง | ต่อยอด `render_sample_test.go` guard ด้วย env เหมือน `HF_SAMPLE=1` เพื่อให้ CI ไม่ต้องมี Node/Chromium |

## เกณฑ์ความสำเร็จของเฟส 1

หลัง deploy ~2 สัปดาห์ (~40 คลิป) ต้องตอบ 3 คำถามนี้ได้จาก SQL:

1. `lint` fail กี่เปอร์เซ็นต์ของคลิป และ finding ที่พบซ้ำคืออะไร
2. `lint` กินเวลากี่วินาที (median / p95)
3. `inspect` เคย flag จริงกี่ครั้ง และองค์ประกอบไหน

ตัวเลขทั้งสามคือสิ่งที่ตัดสินว่าเฟส 2 และ 3 ควรทำอย่างไร

## เฟสถัดไป (ยังไม่ตัดสินใจ — รอข้อมูลจากเฟส 1)

- **เฟส 2 — fail fast**: ให้ lint ที่ fail หยุดงานก่อนเรนเดอร์ ไม่เสีย CPU 20 นาทีกับคลิปที่พังแน่ๆ
  รูปแบบการหยุด (retry อัตโนมัติ หรือ `needs_review`) ขึ้นกับอัตรา fail ที่วัดได้จริง
- **เฟส 3 — เพิ่มความเข้มของ gate**: พิจารณาเปิด `RENDER_ERROR_GATE_ENABLED` และประเมินว่า
  ควรอัปเกรด CLI ไป 0.7.x เพื่อใช้ `check` (รวม lint + runtime error + layout + contrast +
  `sweep_static` ที่จับคลิปจอนิ่ง) แทน `lint`+`inspect` หรือไม่ — การอัปเกรดมีความเสี่ยงด้าน
  สภาพแวดล้อมสูง (chromium pin, npx manifest, worker tuning) จึงต้องมี spec ของตัวเอง

## ความเสี่ยงที่รู้ตัว

- **ยังไม่เคยรัน `hyperframes lint` กับเทมเพลตจริง** ไม่รู้ว่าจะ fail กี่ % — นี่คือเหตุผลที่
  เฟส 1 เป็น shadow ถ้าเปิด gate ทันทีแล้ว lint ไม่ชอบ pattern ที่สร้าง scene ด้วย JS
  คลิปจะถูกบล็อกทั้งหมด
- **lint เป็น static analysis จึงมองไม่เห็นฉากของเรา** — scene wrapper ทุกอันถูกสร้างด้วย
  `document.createElement` ตอนโหลดหน้า (`composition.go:455-460`) lint จึงตรวจได้เฉพาะ
  โครงร่างคงที่ (root, progress, badges, capStage, audio) ไม่ใช่ตัวฉาก ค่าของ lint ในระบบนี้
  จึงจำกัดกว่าที่เอกสาร Hyperframes สื่อ — ข้อมูลจากเฟส 1 จะบอกว่าจำกัดแค่ไหน
- **เวลาเรนเดอร์เพิ่มขึ้นเล็กน้อย** เท่ากับเวลาที่ lint ใช้ ซึ่งจะวัดได้จาก `duration_ms`
