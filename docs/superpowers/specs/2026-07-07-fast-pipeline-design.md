# Fast Pipeline — ลดเวลารอตอนสะดุด (Approach A)

**Date:** 2026-07-07
**Goal:** ลดเวลาที่เสียไปเมื่อ pipeline สะดุด (kie.ai ช้า/ล่ม, คลิป fail รอ retry tick) และลดเวลาคลิปปกติจาก ~14 นาที → ~7-8 นาที
**Trade-off ที่ user เลือกแล้ว:** ความเร็วมาก่อนคุณภาพภาพ AI — gen ภาพช้า/พลาด → ใช้ CSS fallback ทันที

## ข้อมูลจริงที่เป็นฐานการออกแบบ (วัดจาก log prod 2026-07-05/07)

| ขั้น | เวลา (คลิปเร็วสุด 14.2 นาที) | ปัญหา |
|------|------|-------|
| LLM content | ~2m45s | — |
| TTS ×10 | ~33s | — |
| Image gen ×10 | ~5m | ทำทีละ scene; timeout 180s ×2 attempts = เสียได้ 6.6 นาทีต่อ scene ที่พัง |
| Render | ~2m10s | — (จำกัดด้วย CPU, workers=3 ห้ามเพิ่ม — เคยเกิด protocolTimeout) |
| Visual QA ×10 | ~3m26s | ทำทีละ scene |
| คลิป fail | รอ tick นานสุด 15 นาที | retry_failed = `*/15` |

## การเปลี่ยนแปลง

### Flag-gated: `PIPELINE_FAST_ENABLED` (env, default OFF — flag off = พฤติกรรมเดิม 100%)

1. **Parallel image gen** — `AssembleHyperframes916` step 2 (producer.go:337): ยิง `GenerateImage` ขนานทุก scene, จำกัด 4 พร้อมกัน (kie rate limit) circuit breaker เดิมคงไว้แบบ atomic: scene แรกที่ fail → scene ที่ยังไม่เริ่มข้ามเป็น CSS, ภาพที่เสร็จแล้วใช้ต่อ
2. **Image fail-fast + fallback chain** — เมื่อ flag on: `ImageTaskTimeout` 180s → 150s (แก้จาก 75s หลังวัดจริง 2026-07-07: kie วันช้าเสร็จใน 108-140s — 75s พลาดภาพที่เกือบเสร็จ), `ImageMaxRetries` 1 → 0. ต่อ scene: gpt-image-2 (150s, 2K, 10 เครดิต) → fail → **nano-banana-2-lite** (60s, วัดจริง ~24-28s, 4 เครดิต, 768px) → fail → css เป็นทางหนีสุดท้าย. Breaker แยกต่อ stage: primary ล่ม → scene ถัดไปเริ่มที่ nano เลย; nano ล่มด้วย → css. Worst case ทั้งคู่ล่ม: ~210s/wave
3. **Parallel visual QA** — `visualqa.go Review` (loop line 88): ตรวจ frame ขนาน จำกัด 4, fail-open ต่อ scene คงเดิม

### ไม่ gate (ปลอดภัย + rollback ง่าย)

4. **Migration 048:** schedules "Retry Failed" `*/15` → `*/5` (rollback: แก้ row กลับ)
5. **Retry nudge:** ยิงครั้งเดียวที่**ขอบ produce loop** (จบ ProduceWeekly แล้วมีคลิป fail) → รอ 15 วิ แล้วเรียก `RetryAllFailed(2, 0)` เดิมซึ่งมี production gate อยู่แล้ว. จงใจไม่วางใน failClip — failClip ถูกเรียกจาก retry path เองด้วย จะเกิด nudge ซ้อน nudge; วางที่ขอบ loop ทำให้ retry ที่ fail ซ้ำไม่ re-arm ตัวเอง (รอบสองไปทาง tick ที่มี cooldown 10 นาทีให้ upstream ฟื้น)

## ผลคาด

- คลิปปกติ: image ~5m→~1m, QA ~3.4m→~1m ⇒ รวม ~14 → **~7-8 นาที**
- kie ล่ม: เสียเพิ่มสูงสุด ~80 วิ (เดิม 6.6+ นาที)
- คลิป fail: เริ่ม retry ใน ~15 วิ (เดิมนานสุด 15 นาที)

## Error handling

- ภาพ fail รายตัว → CSS scene นั้น (พฤติกรรม BuildScenes เดิม)
- QA scene ใด error → fail-open scene นั้น (เดิม)
- Retry nudge ใช้ context.Background()+timeout กัน goroutine ค้าง; ถ้า production gate ไม่ว่างก็จบเงียบ

## Testing / Verification

- `go build ./...` + `go test ./...` (unit เดิมต้องผ่าน)
- Unit ใหม่: parallel image gen — breaker ตัดแล้ว scene ที่เหลือไม่เรียก kie; ผลลัพธ์ bgPaths เท่า sequential
- Prod verify: flip flag บน Railway → produce 1 คลิปจริง → วัดเวลาแต่ละขั้นจาก log เทียบตาราง

## Rollback

- Flag `PIPELINE_FAST_ENABLED=false` (ข้อ 1-3), แก้ schedules row กลับ (ข้อ 4), revert commit (ข้อ 5)

## สิ่งที่จงใจไม่ทำ (YAGNI)

- ไม่เพิ่ม render workers (CPU-bound, เคยพัง)
- ไม่ทำ job queue ถาวร (Approach C)
- ไม่ hedge ยิงภาพซ้ำขนาน (เปลืองเครดิต; user เลือก fail-fast แล้ว)
