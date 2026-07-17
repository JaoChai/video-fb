# QuestionAgent Cooldown Deadlock Fix — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** QuestionAgent ต้องไม่คืน 0 คำถามเพราะ pain_point ติด cooldown — regenerate โดยเลี่ยง pain_point ที่ติด แล้ว fail-open ถ้าจนตรอก เพื่อไม่ให้ production หยุดผลิตคลิปเงียบๆ

**Architecture:** แยก orchestration ของ cooldown-retry/fail-open ออกเป็น pure function `cooldownFilterRetry` ที่รับ callback (`inCooldown`, `regen`) → unit-test ได้โดยไม่แตะ DB/LLM. แล้ว wire เข้า `QuestionAgent.Generate` แทน cooldown filter เดิม (question.go:212-229). ไม่แตะ dedup loop และ category rotation — deadlock ชั้น category หายเองเมื่อ account-trust ผลิตได้

**Tech Stack:** Go, chi, pgx, testing (stdlib)

## Global Constraints

- ภาษา comment/log ใหม่: ไทย ให้เข้ากับโค้ดรอบข้าง
- ไม่มี migration — code-only fix (rollback = revert commit)
- fail-open ต้อง log ระดับเตือนเสมอเมื่อยอมรับ pain_point ที่ติด cooldown
- `maxCooldownRetries = 2` (ให้ตรงกับ `maxDedupRetries` เดิม)
- คง pattern fail-open เดิม: cooldown-check error → ถือว่าไม่ติด cooldown (question.go:219)

---

### Task 1: Pure function `cooldownFilterRetry` + unit tests

**Files:**
- Modify: `internal/agent/question.go` (เพิ่มฟังก์ชัน package-level + import `sort`)
- Test: `internal/agent/question_test.go` (เพิ่ม import `context`, `errors`)

**Interfaces:**
- Produces:
  ```go
  func cooldownFilterRetry(
      ctx context.Context,
      initial []GeneratedQuestion,
      count, maxRetries int,
      inCooldown func(context.Context, string) (bool, error),
      regen func(ctx context.Context, avoid []string, n int) ([]GeneratedQuestion, error),
  ) []GeneratedQuestion
  ```
  - `inCooldown` คืน true ถ้า pain_point ติด cooldown; error → fail-open (เก็บคำถามไว้)
  - `regen` สร้างคำถามใหม่ `n` ข้อ โดยเลี่ยง pain_point ใน `avoid` (ต้อง dedup ในตัวแล้ว)
  - คืน slice ยาว ≤ `count`; **ไม่มีวันคืน empty** ถ้า `initial` ไม่ว่างและมีอย่างน้อย 1 ตัวที่เคยถูกทิ้ง (fail-open)

- [ ] **Step 1: เขียน failing tests** — เพิ่มต่อท้าย `internal/agent/question_test.go`

```go
import (
	"context"
	"errors"
	"testing"
)

func TestCooldownFilterRetry_RetriesPastCooldown(t *testing.T) {
	cd := map[string]bool{"low_trust_score": true}
	inCD := func(_ context.Context, pp string) (bool, error) { return cd[pp], nil }
	regenCalls := 0
	regen := func(_ context.Context, avoid []string, n int) ([]GeneratedQuestion, error) {
		regenCalls++
		return []GeneratedQuestion{{Question: "q2", PainPoint: "agency_trust_score"}}, nil
	}
	initial := []GeneratedQuestion{{Question: "q1", PainPoint: "low_trust_score"}}
	got := cooldownFilterRetry(context.Background(), initial, 1, 2, inCD, regen)
	if len(got) != 1 || got[0].PainPoint != "agency_trust_score" {
		t.Fatalf("want 1 q w/ agency_trust_score, got %+v", got)
	}
	if regenCalls != 1 {
		t.Fatalf("want 1 regen call, got %d", regenCalls)
	}
}

func TestCooldownFilterRetry_FailOpenWhenAllInCooldown(t *testing.T) {
	inCD := func(_ context.Context, _ string) (bool, error) { return true, nil }
	regen := func(_ context.Context, _ []string, n int) ([]GeneratedQuestion, error) {
		return []GeneratedQuestion{{Question: "qX", PainPoint: "low_trust_score"}}, nil
	}
	initial := []GeneratedQuestion{{Question: "q1", PainPoint: "low_trust_score"}}
	got := cooldownFilterRetry(context.Background(), initial, 1, 2, inCD, regen)
	if len(got) != 1 {
		t.Fatalf("fail-open must return 1 question, got %d", len(got))
	}
}

func TestCooldownFilterRetry_NoRetryWhenClean(t *testing.T) {
	inCD := func(_ context.Context, _ string) (bool, error) { return false, nil }
	regenCalls := 0
	regen := func(_ context.Context, _ []string, n int) ([]GeneratedQuestion, error) {
		regenCalls++
		return nil, nil
	}
	initial := []GeneratedQuestion{{Question: "q1", PainPoint: "ad_fatigue"}}
	got := cooldownFilterRetry(context.Background(), initial, 1, 2, inCD, regen)
	if len(got) != 1 || regenCalls != 0 {
		t.Fatalf("want 1 q and 0 regen, got %d q, %d regen", len(got), regenCalls)
	}
}

func TestCooldownFilterRetry_CooldownErrorFailsOpen(t *testing.T) {
	inCD := func(_ context.Context, _ string) (bool, error) { return false, errors.New("db down") }
	regen := func(_ context.Context, _ []string, n int) ([]GeneratedQuestion, error) {
		t.Fatal("regen must not run when question kept via fail-open")
		return nil, nil
	}
	initial := []GeneratedQuestion{{Question: "q1", PainPoint: "low_trust_score"}}
	got := cooldownFilterRetry(context.Background(), initial, 1, 2, inCD, regen)
	if len(got) != 1 {
		t.Fatalf("cooldown error must fail-open keep question, got %d", len(got))
	}
}
```

- [ ] **Step 2: รัน test ยืนยัน fail**

Run: `go test ./internal/agent/ -run TestCooldownFilterRetry -v`
Expected: FAIL — `undefined: cooldownFilterRetry`

- [ ] **Step 3: เพิ่มฟังก์ชัน** ใน `internal/agent/question.go` (วางก่อน `func (a *QuestionAgent) Generate`) และเพิ่ม `"sort"` ใน import block

```go
// cooldownFilterRetry ทิ้งคำถามที่ pain_point ติด cooldown แล้วขอ regen มาแทน
// (เลี่ยง pain_point ที่ทิ้งไป) สูงสุด maxRetries รอบ เพื่อไม่ให้ batch ที่ตัวแรก
// ชน cooldown คืน 0 คำถาม. fail-open: ถ้าทุกตัวติด cooldown จนครบ retry จะคืน
// คำถามตัวสุดท้ายที่ถูกทิ้งแทนการคืน empty — คลิปซ้ำนิดหน่อยยังดีกว่า produce 0 คลิป
// เงียบๆ. inCooldown error ถือว่า "ไม่ติด cooldown" (fail-open ตาม pattern เดิม)
func cooldownFilterRetry(
	ctx context.Context,
	initial []GeneratedQuestion,
	count, maxRetries int,
	inCooldown func(context.Context, string) (bool, error),
	regen func(ctx context.Context, avoid []string, n int) ([]GeneratedQuestion, error),
) []GeneratedQuestion {
	kept := make([]GeneratedQuestion, 0, len(initial))
	dropped := map[string]bool{}
	var lastDropped *GeneratedQuestion

	filter := func(qs []GeneratedQuestion) {
		for i := range qs {
			cd, err := inCooldown(ctx, qs[i].PainPoint)
			if err != nil || !cd {
				kept = append(kept, qs[i])
				continue
			}
			dropped[qs[i].PainPoint] = true
			lastDropped = &qs[i]
			log.Printf("QuestionAgent: pain_point %q in cooldown, dropped", qs[i].PainPoint)
		}
	}

	filter(initial)

	for attempt := 0; len(kept) < count && attempt < maxRetries; attempt++ {
		avoid := make([]string, 0, len(dropped))
		for pp := range dropped {
			avoid = append(avoid, pp)
		}
		sort.Strings(avoid) // prompt/ลำดับ deterministic
		fresh, err := regen(ctx, avoid, count-len(kept))
		if err != nil {
			log.Printf("QuestionAgent: cooldown regen failed (attempt %d): %v", attempt+1, err)
			break
		}
		filter(fresh)
	}

	if len(kept) == 0 && lastDropped != nil {
		log.Printf("QuestionAgent: all pain_points in cooldown after %d retries, accepting %q to avoid 0-clip stall",
			maxRetries, lastDropped.PainPoint)
		kept = append(kept, *lastDropped)
	}

	if len(kept) > count {
		kept = kept[:count]
	}
	return kept
}
```

- [ ] **Step 4: รัน test ยืนยัน pass**

Run: `go test ./internal/agent/ -run TestCooldownFilterRetry -v`
Expected: PASS ทั้ง 4 tests

- [ ] **Step 5: Commit**

```bash
git add internal/agent/question.go internal/agent/question_test.go
git commit -m "feat(question): cooldown-aware retry + fail-open (pure fn + tests)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01KwBzTatE6zVX9ReRvAAhWg"
```

---

### Task 2: Wire `cooldownFilterRetry` เข้า `Generate`

**Files:**
- Modify: `internal/agent/question.go:212-229` (แทน cooldown filter เดิมด้วยการเรียก pure fn + closure)

**Interfaces:**
- Consumes: `cooldownFilterRetry` (Task 1), `a.deduper.PainPointInCooldown(ctx, pp, days) (bool, error)`, `a.deduper.CheckQuestions`, `filterBySimilarity`, `a.deduper.threshold` (มีอยู่แล้วในไฟล์เดียวกัน)

- [ ] **Step 1: แทนบล็อก cooldown เดิม** (question.go:212-229) ด้วยโค้ดนี้

```go
	// pain_point cooldown: กันหัวข้อเดิมเปลี่ยนมุม (flag on เท่านั้น — painCooldownDays > 0)
	// ถ้า pain_point ที่ได้ติด cooldown ให้ generate ใหม่โดยเลี่ยง pain_point นั้น
	// fail-open ถ้าทุกอันติด cooldown เพื่อไม่ให้ produce คืน 0 คลิป (ดู cooldownFilterRetry)
	if a.painCooldownDays > 0 && len(accepted) > 0 {
		const maxCooldownRetries = 2
		regen := func(ctx context.Context, avoid []string, n int) ([]GeneratedQuestion, error) {
			p := userPrompt + fmt.Sprintf(
				"\n\npain_point เหล่านี้ติด cooldown ห้ามใช้ ให้เลือก pain_point อื่นในหมวด %s:\n- %s\nสร้าง %d ข้อ",
				category, strings.Join(avoid, "\n- "), n)
			var qs []GeneratedQuestion
			if err := a.llm.GenerateJSON(ctx, cfg.Model, cfg.BuildSystemPrompt(), p, cfg.Temperature, &qs); err != nil {
				return nil, err
			}
			sims, _, derr := a.deduper.CheckQuestions(ctx, qs)
			if derr != nil {
				return qs, nil // dedup ล่ม → รับ fresh batch ไปก่อน (สอดคล้อง fallback lexical เดิม)
			}
			passed, _ := filterBySimilarity(qs, sims, a.deduper.threshold)
			return passed, nil
		}
		accepted = cooldownFilterRetry(ctx, accepted, count, maxCooldownRetries,
			func(ctx context.Context, pp string) (bool, error) {
				return a.deduper.PainPointInCooldown(ctx, pp, a.painCooldownDays)
			}, regen)
	}
```

หมายเหตุ: คำถามที่ได้จาก regen จะไม่มี embedding ใน `allEmbeddings` → store step (question.go:231-242) จะ INSERT แบบไม่มี embedding (branch ที่มีอยู่แล้ว) — ยอมรับได้

- [ ] **Step 2: build + vet + รัน test ทั้ง package**

Run: `go build ./... && go vet ./internal/agent/ && go test ./internal/agent/ -v`
Expected: build ผ่าน, vet ไม่มี warning, test package agent ผ่านทั้งหมด (รวม 4 tests ใหม่ + เดิม)

- [ ] **Step 3: อ่าน diff ยืนยัน surgical** — ต้องแตะเฉพาะ block cooldown (212-229 เดิม) + import `sort` + ฟังก์ชันใหม่ ไม่มีอย่างอื่นเปลี่ยน

Run: `git diff internal/agent/question.go`
Expected: เห็นเฉพาะการเปลี่ยนที่ตั้งใจ

- [ ] **Step 4: Commit**

```bash
git add internal/agent/question.go
git commit -m "fix(question): regenerate on pain_point cooldown instead of dropping to zero

แก้ deadlock ที่ produce cron ยิงแล้วได้ 0 คลิป (pain_point ติด cooldown ถูกทิ้ง
ไม่มี fallback). category rotation หายเองเมื่อหมวดที่ค้างผลิตได้ 1 คลิป.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01KwBzTatE6zVX9ReRvAAhWg"
```

---

### Task 3: Simplify pass, deploy & catch-up verification

**Files:** ไม่มีไฟล์โค้ดใหม่ (เป็น operational + quality gate)

- [ ] **Step 1: /simplify บน diff** — รัน `/simplify` (หรือ code-simplifier) บน 2 commit ที่แก้ ตาม user preference (simplify ก่อน finalize). แก้เฉพาะที่ simplifier ชี้ commit เพิ่มถ้ามี

- [ ] **Step 2: push master → Railway auto-deploy**

```bash
git push origin master
```
รอ deploy backend สำเร็จ — ตรวจ `mcp__plugin_railway_railway__get-status` project `6decf46f-26c0-44b2-b066-1f30cc11f24d` ว่า service `adsvance-v2` latestDeployment.status = SUCCESS และ createdAt ใหม่กว่าตอนนี้

- [ ] **Step 3: trigger produce catch-up เก็บคลิปที่ขาด**

เรียก `POST /api/v1/orchestrator/produce?count=1` (ต้องใช้ API key — ดึงจาก settings/env ถ้าจำเป็น; ถ้าเรียกจาก local ต้องมี base URL prod). ถ้าเรียกไม่สะดวก รอ cron รอบเที่ยง (05:00 UTC) แล้วตรวจแทน

- [ ] **Step 4: verify clip row ใหม่เกิดจริง** — Neon run_sql project `snowy-grass-75448787`

```sql
SELECT id, status, production_stage, category, created_at
FROM clips ORDER BY created_at DESC LIMIT 3;
```
Expected: มี row ที่ `created_at` หลัง deploy, เดินถึง `production_stage` != แรกเริ่ม (ไม่ค้าง)

- [ ] **Step 5: verify log cooldown-retry ทำงาน** — Railway get-logs service `adsvance-v2` ช่วงเวลา produce
  - ถ้าชน cooldown: ต้องเห็น "in cooldown, dropped" ตามด้วยผลลัพธ์ "Generated 1 questions" (ไม่ใช่ 0)
  - ยืนยันว่าไม่มี "Generated 0 questions (requested 1)" อีก

- [ ] **Step 6: อัปเดต memory** — เขียน project memory ผลลัพธ์ + gotcha (cooldown ต้องมี fallback เสมอ) เชื่อมกับ `[[project_content_retention_principles]]` และ `[[project_content_brain_v2]]`

---

## Self-Review

**1. Spec coverage:**
- cooldown-retry + fail-open → Task 1 (pure fn) + Task 2 (wiring) ✓
- testability seam (inject callbacks) → Task 1 signature ✓
- test cases 1-4 ใน spec → 4 tests ใน Task 1 ✓
- deploy + catch-up + verify → Task 3 ✓
- ไม่แตะ category rotation / migration 056 → ไม่มี task แตะ (ตั้งใจ) ✓
- simplify ก่อน commit (user pref) → Task 3 Step 1 ✓

**2. Placeholder scan:** ไม่มี TBD/TODO — โค้ดครบทุก step. Task 3 Step 3 มีเงื่อนไข "ถ้าเรียกไม่สะดวก" ซึ่งเป็น operational fallback จริง ไม่ใช่ placeholder

**3. Type consistency:** `cooldownFilterRetry` signature ตรงกันระหว่าง Task 1 (นิยาม) และ Task 2 (เรียกใช้); `regen`/`inCooldown` callback types ตรงกัน; ใช้ `GeneratedQuestion`, `filterBySimilarity`, `a.deduper.threshold`, `PainPointInCooldown`, `CheckQuestions` ตามที่มีจริงในไฟล์ ✓
