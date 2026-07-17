# Design: QuestionAgent pain_point cooldown deadlock

วันที่: 2026-07-17
สถานะ: approved (รอ user review ก่อนเข้า writing-plans)

## Problem

หลัง deploy migration 056 (2026-07-16 10:48 UTC) production หยุดผลิตคลิปเงียบๆ — produce cron
ยิงครบทุกรอบแต่ได้ 0 คลิป 2 รอบติด (เย็น 07-16, เช้า 07-17) ไม่มีคลิป fail/ค้าง (ทุกคลิป
status=published) เพราะมันพังตั้งแต่ก่อนสร้าง clip row

หลักฐาน (log prod ทั้ง 2 รอบ เหมือนกันเป๊ะ):

```
Producing 1 clips — category: account-trust
QuestionAgent: pain_point "low_trust_score" in cooldown, dropped
Generated 0 questions (requested 1)   ← จบ ไม่มีอะไรให้ผลิต
```

## Root cause (ตรวจแล้ว — อ่าน log/DB/โค้ดจริง)

Deadlock 2 ชั้น:

1. **cooldown filter ไม่มี fallback** (`internal/agent/question.go:212-229`)
   dedup loop (152-205) มี retry ขอ LLM สร้างใหม่เมื่อคำถามซ้ำ แต่ pain_point cooldown เป็น
   step แยกที่ต่อท้าย — พอ pain_point ติด cooldown มัน **แค่ทิ้ง ไม่ generate ใหม่** →
   `accepted` เหลือ 0 → คืน 0 คำถาม → orchestrator ไม่ผลิตอะไร

2. **category rotation ไม่เดินหน้าเมื่อผลิตล้มเหลว** (`internal/repository/topics.go` `PickNextExclude`)
   เลือก category จาก least-used ใน 7 วัน โดยนับจากตาราง `clips` เท่านั้น + exclude หมวดที่
   ผลิตวันนี้ รอบที่ล้มเหลวไม่สร้าง clip row → `account-trust` ยังเป็นหมวด least-used ตลอด →
   รอบถัดไปเลือกซ้ำ → generate `low_trust_score` (ติด cooldown) ซ้ำ → วนไม่จบ

**ทำไม 056 กระตุ้น:** migration 056 เป็น prompt-only (ไม่แตะ JSON shape — ตั้งใจเลี่ยง
regression 052) แต่ prompt `question` ใหม่สั่ง _"เลือกคำถาม payoff ชัด ตอบจบใน 1 คลิป ไม่กว้าง"_
→ agent โฟกัสแคบลง หยิบ `low_trust_score` ซ้ำแทนการกระจาย pain_point → เปิดโปงบั๊กแฝงชั้นที่ 1

**จะไม่หายเอง:** `low_trust_score` ใช้ล่าสุด 07-15 05:02, `pain_point_cooldown_days`=5 →
ติด cooldown ถึง ~07-20 และถึงตอนนั้นก็ยังเสี่ยงวนต่อ

หมายเหตุ: **ไม่ใช่** regression 052 (จอเปล่า/narration หาย) — อาการต่างกันคนละแบบ (0 คลิป)

## Fix

### หลักการ
QuestionAgent ต้องไม่คืน 0 คำถามเพราะเหตุ cooldown เท่านั้น — production continuity >
cooldown purity (คลิปซ้ำนิดหน่อยยังดีกว่า outage 0 คลิปเงียบๆ)

### แนวทาง (Option A — surgical, ไม่ยุ่ง dedup loop เดิม)

เพิ่ม **cooldown-aware retry** ต่อจาก dedup loop เดิม แล้วปิดท้ายด้วย fail-open:

1. หลัง cooldown filter ถ้า `len(accepted) < count`:
   - generate ใหม่จำนวนที่ขาด โดยเพิ่มคำสั่งใน prompt ว่า pain_point ไหน "ติด cooldown ห้ามใช้
     ให้เลือก pain_point อื่นในหมวด `<category>`" (category คงเดิม — account-trust มีหลาย
     pain_point เช่น `agency_trust_score`)
   - คำถามใหม่ต้องผ่านทั้ง dedup (semantic) และ cooldown อีกครั้ง
   - วนได้สูงสุด `maxCooldownRetries` (เริ่มที่ 2 ให้ตรงกับ maxDedupRetries)
2. **fail-open สุดท้าย:** ถ้าครบ retry แล้วยัง `len(accepted) == 0` และมีคำถามที่ถูกทิ้งเพราะ
   cooldown อย่างน้อย 1 ข้อ → log ดังๆ (`WARN: all pain_points in cooldown, accepting <pp>
   to avoid 0-clip stall`) + รับคำถามที่ดีที่สุด 1 ข้อไป (ตรงกับ pattern fail-open ที่
   question.go:219 ใช้อยู่แล้วสำหรับ cooldown DB error)

ผลข้างเคียงที่ตั้งใจ: พอ account-trust ผลิตคลิปได้ 1 คลิป rotation ก็เดินต่อ →
**deadlock ชั้น 2 (category) หายเอง ไม่ต้องแตะ `PickNextExclude`** (YAGNI)

### Testability seam (จุดสำคัญ)

`QuestionAgent` ผูกกับ concrete `*KieLLMClient` + `*Deduper` ที่ backed ด้วย `*pgxpool.Pool`
จริง — ไม่มี interface seam และ `question_test.go` ปัจจุบันเทสต์แค่ template render ไม่เคย
mock LLM/DB มาก่อน

เพื่อ TDD retry loop โดยไม่ต้องรื้อ dependency ทั้งก้อน: **แยก orchestration ของ
retry/fail-open ออกเป็น pure unit** ที่รับ callback ฉีดเข้าไป เช่น

- `generateFn func(ctx, avoidPainPoints []string, n int) ([]GeneratedQuestion, error)`
- `cooldownFn func(ctx, painPoint string) (bool, error)`
- `dedupFn` (หรือให้ผ่าน dedup ในตัว generateFn)

แล้วเทสต์ยิงเฉพาะ pure unit นี้ — method จริงบน QuestionAgent เป็น thin wrapper ที่ผูก
callback เข้ากับ llm/deduper จริง (รายละเอียดโครง signature ให้ writing-plans ตัดสิน)

### Test cases (TDD)

1. LLM คืน pain_point ติด cooldown รอบแรก → คืน pain_point อื่นรอบ retry → assert คืน
   คำถามที่ไม่ติด cooldown, ไม่เรียก generate เกิน retry ที่กำหนด
2. LLM คืน pain_point ติด cooldown ทุกครั้งจนครบ retry → assert fail-open คืน **1** คำถาม
   (ไม่ใช่ 0) + มี log warn
3. รอบแรกผ่านฉลุย (ไม่มีอะไรติด cooldown) → assert ไม่ retry เลย (ไม่ regress path ปกติ)
4. cooldown DB error → คง fail-open เดิม (รับคำถามไป) ไม่พังทั้ง batch

## Deploy & verification

1. deploy (Railway auto-deploy จาก master push)
2. `POST /api/v1/orchestrator/produce?count=1` เก็บคลิปที่ขาด
3. verify: มี clip row ใหม่ (created_at หลัง deploy) เดินถึง production_stage=rendered /
   status=published — ตรวจผ่าน Neon run_sql
4. ดู log produce รอบนั้น: ต้องเห็น cooldown-retry ทำงาน (ถ้าชน) และ "Generated 1 questions"
   ไม่ใช่ 0

## Rollback

revert commit เดียว (fix นี้เป็น code-only ไม่มี migration) — กลับไปพฤติกรรมเดิม (ซึ่งจะ
deadlock อีกจนถึง ~07-20) ดังนั้น rollback เฉพาะกรณี fix ทำ produce path พังหนักกว่าเดิม

## Out of scope (YAGNI)

- ไม่แตะ category rotation logic (`PickNextExclude`) — หายเองจากชั้น 1
- ไม่แก้ migration 056 prompt (prompt ทำงานถูกตามเจตนา ปมอยู่ที่โค้ด cooldown)
- ไม่เพิ่ม unblock manual/DB surgery (user เลือกข้าม → แก้ราก+deploy+catch-up แทน)
