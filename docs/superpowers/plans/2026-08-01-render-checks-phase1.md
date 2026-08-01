# Render Checks เฟส 1 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** ทำให้ผลของด่าน lint / inspect / render ของ Hyperframes ถูกบันทึกลง DB ทุกคลิป โดยไม่เปลี่ยนพฤติกรรมการเผยแพร่แม้แต่คลิปเดียว

**Architecture:** เพิ่มตาราง append-only `render_checks` (แบบเดียวกับ `visual_qa` / `auto_reviews`) ให้เมธอดของ `HyperframesRenderer` คืน `CheckResult` ควบคู่ `error` เดิม (ผู้เรียกใช้ `error` เหมือนเดิมทุกประการ ส่วน `CheckResult` เป็นข้อมูลสังเกตการณ์ล้วน) เรียก `Lint()` ในสายการผลิตเป็นครั้งแรกแบบ shadow แล้ว orchestrator เขียนผลทั้ง 3 ด่านลง DB

**Tech Stack:** Go 1.25, pgx/v5, PostgreSQL (Neon), Hyperframes CLI 0.6.70

Spec: `docs/superpowers/specs/2026-08-01-render-checks-observability-design.md`

## Global Constraints

- **ห้ามเปลี่ยนพฤติกรรมการเผยแพร่** — `InspectFlagged` → `needs_review` และ `RenderFlagged` + `RENDER_ERROR_GATE_ENABLED` ต้องทำงานเหมือนเดิมทุกประการ
- **ห้ามแตะเวอร์ชัน Hyperframes CLI** — ยังคง `0.6.70` ทั้ง 3 จุด (`Dockerfile:52`, `internal/producer/hyperframes.go:13`, `internal/producer/composition_builder.go:22`)
- **ห้ามแตะ** `internal/producer/templates/layout_multi_scene.html.tmpl`
- **ห้ามเพิ่ม env flag ใหม่** — เฟส 1 ทำงานเสมอ ไม่มีสวิตช์
- migration ต้อง **idempotent ไม่มี goose syntax** (`RunMigrations` ไม่หุ้ม transaction — ดู `internal/database/migrations.go`)
- คอมเมนต์โค้ดใหม่: ภาษาไทยหรืออังกฤษก็ได้ ตามไฟล์ที่กำลังแก้ (ไฟล์ producer/orchestrator ใช้อังกฤษเป็นหลัก ส่วนโค้ดใหม่ล่าสุดใช้ไทยได้)
- ทุก list endpoint ที่คืน slice ต้อง `out := []T{}` ไม่ใช่ `var out []T` (กฎเดิมของโปรเจกต์ — แผนนี้ไม่มี endpoint ใหม่ แต่กฎยังใช้ถ้ามีการเพิ่ม)

---

### Task 1: ตาราง `render_checks` + repository

**Files:**
- Create: `migrations/079_render_checks.sql`
- Create: `internal/repository/renderchecks.go`
- Create: `internal/repository/renderchecks_test.go`
- Modify: `internal/models/clip.go` (เพิ่ม struct ท้ายกลุ่ม VisualQA/AutoReview ราวบรรทัด 85)

**Interfaces:**
- Produces:
  - `models.RenderCheck{ID, ClipID, Stage string; Passed bool; DurationMS int; Findings json.RawMessage; CreatedAt time.Time}`
  - `repository.NewRenderChecksRepo(pool *pgxpool.Pool) *RenderChecksRepo`
  - `(*RenderChecksRepo).Create(ctx context.Context, clipID, stage string, passed bool, durationMS int, findings []byte) error`
  - `repository.RenderCheckStage` constants: `StageLint = "lint"`, `StageInspect = "inspect"`, `StageRender = "render"`
  - `repository.ValidRenderStage(stage string) bool`

- [ ] **Step 1: เขียนเทสต์ที่ยังไม่ผ่าน**

สร้าง `internal/repository/renderchecks_test.go`:

```go
package repository

import "testing"

func TestValidRenderStage(t *testing.T) {
	for _, s := range []string{StageLint, StageInspect, StageRender} {
		if !ValidRenderStage(s) {
			t.Errorf("ValidRenderStage(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"", "LINT", "check", "render "} {
		if ValidRenderStage(s) {
			t.Errorf("ValidRenderStage(%q) = true, want false", s)
		}
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่าไม่ผ่าน**

Run: `go test ./internal/repository/ -run TestValidRenderStage -v`
Expected: FAIL — `undefined: StageLint`

- [ ] **Step 3: เขียน migration**

สร้าง `migrations/079_render_checks.sql`:

```sql
-- 079_render_checks.sql
-- บันทึกผลของด่านตรวจ Hyperframes ทุกคลิป (lint / inspect / render) แบบ append-only
-- เฟส 1 = สังเกตการณ์ล้วน ไม่มีด่านไหนเปลี่ยนพฤติกรรมการเผยแพร่
-- Idempotent; no goose syntax (RunMigrations ไม่หุ้ม transaction)

CREATE TABLE IF NOT EXISTS render_checks (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clip_id     UUID NOT NULL REFERENCES clips(id) ON DELETE CASCADE,
    stage       TEXT NOT NULL,               -- lint | inspect | render
    passed      BOOLEAN NOT NULL,
    duration_ms INT NOT NULL DEFAULT 0,      -- วัดว่า lint แพงแค่ไหน (ยังไม่เคยมีใครวัด)
    findings    JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_render_checks_clip_id ON render_checks (clip_id);
CREATE INDEX IF NOT EXISTS idx_render_checks_stage_created ON render_checks (stage, created_at DESC);
```

- [ ] **Step 4: เขียน model**

เพิ่มท้ายกลุ่ม `VisualQAStats` ใน `internal/models/clip.go`:

```go
// RenderCheck คือผลของด่านตรวจ Hyperframes หนึ่งด่านของคลิปหนึ่งตัว
// (lint | inspect | render). เก็บแบบ append-only — 3 แถวต่อคลิป.
type RenderCheck struct {
	ID         string          `json:"id"`
	ClipID     string          `json:"clip_id"`
	Stage      string          `json:"stage"`
	Passed     bool            `json:"passed"`
	DurationMS int             `json:"duration_ms"`
	Findings   json.RawMessage `json:"findings"`
	CreatedAt  time.Time       `json:"created_at"`
}
```

(`encoding/json` และ `time` ถูก import ในไฟล์นี้อยู่แล้ว — ตรวจก่อนเพิ่ม import ใหม่)

- [ ] **Step 5: เขียน repository**

สร้าง `internal/repository/renderchecks.go`:

```go
package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ด่านตรวจของ Hyperframes ที่เราบันทึกผล — ชื่อต้องตรงกับคอลัมน์ stage
const (
	StageLint    = "lint"
	StageInspect = "inspect"
	StageRender  = "render"
)

// ValidRenderStage กันชื่อด่านที่พิมพ์ผิดไม่ให้ลง DB เงียบๆ — คอลัมน์ stage เป็น
// TEXT อิสระ ถ้าเขียน "Lint" ปนไป สถิติที่ GROUP BY stage จะแตกโดยไม่มีใครรู้
func ValidRenderStage(stage string) bool {
	return stage == StageLint || stage == StageInspect || stage == StageRender
}

type RenderChecksRepo struct {
	pool *pgxpool.Pool
}

func NewRenderChecksRepo(pool *pgxpool.Pool) *RenderChecksRepo {
	return &RenderChecksRepo{pool: pool}
}

// Create appends one render-check row. findings คือ JSON array ที่ encode มาแล้ว
// (ผู้เรียกใช้ producer.CheckResult.FindingsJSON()).
func (r *RenderChecksRepo) Create(ctx context.Context, clipID, stage string, passed bool, durationMS int, findings []byte) error {
	if !ValidRenderStage(stage) {
		return fmt.Errorf("invalid render check stage %q", stage)
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO render_checks (clip_id, stage, passed, duration_ms, findings)
		 VALUES ($1, $2, $3, $4, $5)`,
		clipID, stage, passed, durationMS, findings)
	return err
}
```

- [ ] **Step 6: รันเทสต์และ build ให้ผ่าน**

Run: `go test ./internal/repository/ -run TestValidRenderStage -v && go build ./...`
Expected: PASS ทั้งคู่

- [ ] **Step 7: Commit**

```bash
git add migrations/079_render_checks.sql internal/repository/renderchecks.go internal/repository/renderchecks_test.go internal/models/clip.go
git commit -m "feat(qa): ตาราง render_checks + repository (มิเกรชัน 079)"
```

---

### Task 2: `CheckResult` — ให้ทุกด่านคืนผลที่วัดได้

**Files:**
- Modify: `internal/producer/hyperframes.go` (ทั้งไฟล์ — เมธอด `run`, `Lint`, `Inspect`, `Render`)
- Create: `internal/producer/hyperframes_check_test.go`
- Modify: `internal/producer/render_sample_test.go:100-112` (call site ที่ต้องแก้ให้ compile)

**Interfaces:**
- Consumes: ไม่มี (Task นี้ไม่พึ่ง Task 1)
- Produces:
  - `producer.CheckResult{Stage string; Passed bool; DurationMS int; Findings []string}`
  - `(CheckResult).FindingsJSON() []byte` — คืน JSON array เสมอ (nil → `[]`)
  - `producer.classifyRunError(err error, args []string) []string`
  - `(*HyperframesRenderer).Lint(ctx, dir) (CheckResult, error)`
  - `(*HyperframesRenderer).Inspect(ctx, dir) (CheckResult, error)`
  - `(*HyperframesRenderer).Render(ctx, dir, outputPath string) (CheckResult, error)`

**หลักการที่ห้ามพลาด:** ค่า `error` ที่แต่ละเมธอดคืนต้อง**เหมือนเดิมเป๊ะ** — ผู้เรียกทุกรายยังตัดสินใจจาก `error` เหมือนเดิม `CheckResult` เป็นข้อมูลเพิ่มล้วนๆ

- [ ] **Step 1: เขียนเทสต์ที่ยังไม่ผ่าน**

สร้าง `internal/producer/hyperframes_check_test.go`:

```go
package producer

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestFindingsJSON_NilBecomesEmptyArray(t *testing.T) {
	got := string(CheckResult{}.FindingsJSON())
	if got != "[]" {
		t.Fatalf("FindingsJSON() = %s, want []", got)
	}
}

func TestFindingsJSON_EncodesLines(t *testing.T) {
	c := CheckResult{Findings: []string{"canvas_overflow: #cta", `has "quotes"`}}
	got := string(c.FindingsJSON())
	if !strings.Contains(got, "canvas_overflow") || !strings.HasPrefix(got, "[") {
		t.Fatalf("FindingsJSON() = %s, want a JSON array containing the finding", got)
	}
}

// exit != 0 แปลว่า CLI รันแล้วและ "ไม่ผ่าน" — เป็น finding จริงของเทมเพลต
func TestClassifyRunError_ExitErrorIsNotRunnerError(t *testing.T) {
	got := classifyRunError(&exec.ExitError{}, []string{"lint"})
	for _, line := range got {
		if strings.HasPrefix(line, "runner_error:") {
			t.Fatalf("exit error must not be classified as runner_error, got %v", got)
		}
	}
	if len(got) == 0 {
		t.Fatal("exit error must still produce a finding line")
	}
}

// รัน CLI ไม่ได้เลย (ไม่มี binary / timeout) เป็นคนละเรื่องกับเทมเพลตพัง
// เฟส 2 จะเปิด gate จาก finding จริงเท่านั้น จึงต้องแยกให้ออกตั้งแต่ตอนนี้
func TestClassifyRunError_RunnerErrorIsPrefixed(t *testing.T) {
	got := classifyRunError(errors.New("exec: \"hyperframes\": executable file not found"), []string{"lint"})
	if len(got) != 1 || !strings.HasPrefix(got[0], "runner_error:") {
		t.Fatalf("classifyRunError() = %v, want one runner_error: line", got)
	}
}

func TestClassifyRunError_NilIsEmpty(t *testing.T) {
	if got := classifyRunError(nil, []string{"lint"}); len(got) != 0 {
		t.Fatalf("classifyRunError(nil) = %v, want empty", got)
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่าไม่ผ่าน**

Run: `go test ./internal/producer/ -run 'TestFindingsJSON|TestClassifyRunError' -v`
Expected: FAIL — `undefined: CheckResult`, `undefined: classifyRunError`

- [ ] **Step 3: เพิ่ม `CheckResult` + `classifyRunError` ใน `hyperframes.go`**

เพิ่ม import `encoding/json` และ `errors` แล้วแทรกโค้ดนี้ต่อจากคอนสแตนต์ `hyperframesVersion`:

```go
// CheckResult คือผลของด่านตรวจหนึ่งด่านในรูปที่บันทึกลง render_checks ได้
// มันเป็นข้อมูลสังเกตการณ์ล้วน — การตัดสินใจทุกอย่างยังอิง error ที่คืนคู่กันมา
type CheckResult struct {
	Stage      string
	Passed     bool
	DurationMS int
	Findings   []string
}

// FindingsJSON encode Findings สำหรับคอลัมน์ jsonb — คืน "[]" เสมอเมื่อไม่มี
// finding เพื่อไม่ให้คอลัมน์เก็บ null (คิวรีสถิติจะได้ไม่ต้องเผื่อ null)
func (c CheckResult) FindingsJSON() []byte {
	if len(c.Findings) == 0 {
		return []byte("[]")
	}
	b, err := json.Marshal(c.Findings)
	if err != nil {
		return []byte("[]")
	}
	return b
}

// lintTimeout ให้ lint แยกจาก HyperframesRenderer.timeout (20 นาที ซึ่งตั้งไว้
// สำหรับการเรนเดอร์). lint เป็น static analysis ไม่เปิดเบราว์เซอร์ — ด่านที่ไม่มี
// สิทธิ์บล็อกอะไรเลยต้องไม่มีสิทธิ์หน่วงสายการผลิตด้วย
const lintTimeout = 2 * time.Minute

// classifyRunError แยก "CLI รันแล้วบอกว่าไม่ผ่าน" (exit != 0 = finding จริงของ
// เทมเพลต) ออกจาก "รัน CLI ไม่ได้เลย" (binary หาย / timeout / npx ล้ม) โดยติด
// คำนำหน้า runner_error: ให้กรณีหลัง เฟส 2 จะเปิด gate จาก finding จริงเท่านั้น
// — ไม่งั้นคลิปจะถูกบล็อกเพราะ npx ล่ม ซึ่งคนละเรื่องกับเทมเพลตพัง
func classifyRunError(err error, args []string) []string {
	if err == nil {
		return nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return []string{fmt.Sprintf("failed: hyperframes %v exited non-zero", args)}
	}
	return []string{fmt.Sprintf("runner_error: hyperframes %v could not run: %v", args, err)}
}
```

- [ ] **Step 4: รันเทสต์ให้ผ่าน**

Run: `go test ./internal/producer/ -run 'TestFindingsJSON|TestClassifyRunError' -v`
Expected: PASS ทั้ง 5 เทสต์

- [ ] **Step 5: เปลี่ยน `run` ให้วัดเวลาและคืน `CheckResult`**

แทนที่เมธอด `run` เดิมทั้งก้อนด้วย:

```go
// runCheck รันคำสั่ง CLI หนึ่งคำสั่งแล้วคืนทั้งผลที่บันทึกได้ (CheckResult) และ
// error แบบเดิมเป๊ะ — ผู้เรียกทุกรายยังตัดสินใจจาก error เหมือนก่อนหน้านี้
func (h *HyperframesRenderer) runCheck(ctx context.Context, stage string, timeout time.Duration, dir string, args ...string) (CheckResult, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	started := time.Now()
	cmd := hyperframesCmd(ctx, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	res := CheckResult{
		Stage:      stage,
		Passed:     err == nil,
		DurationMS: int(time.Since(started).Milliseconds()),
	}

	// A render can exit 0 while the page silently failed (e.g. a JS exception
	// froze every animation, producing a static video). Hyperframes prints those
	// as "[Browser:PAGEERROR]" / CDN-fetch warnings — surface them either way so
	// a "successful" but broken render is never invisible.
	issues := scanBrowserIssues(out)
	res.Findings = append(res.Findings, issues...)
	if len(issues) > 0 {
		log.Printf("hyperframes %v browser issues:\n%s", args, strings.Join(issues, "\n"))
	}
	if err != nil {
		res.Findings = append(res.Findings, classifyRunError(err, args)...)
		return res, fmt.Errorf("hyperframes %v failed: %w\n%s", args, err, lastBytes(out, 600))
	}
	return res, nil
}
```

- [ ] **Step 6: เปลี่ยน `Lint` / `Inspect` / `Render`**

```go
// Lint runs the static composition linter. เฟส 1 เรียกแบบ shadow — ผู้เรียก
// บันทึกผลแล้วไปต่อเสมอ ไม่ว่าจะผ่านหรือไม่ (ดู spec 2026-08-01)
func (h *HyperframesRenderer) Lint(ctx context.Context, dir string) (CheckResult, error) {
	return h.runCheck(ctx, "lint", lintTimeout, dir, "lint")
}

// Inspect runs Hyperframes' collision/overflow auditor (canvas_overflow,
// container_overflow, clipped_text) in headless Chrome. A flagged result routes
// the clip to needs_review (orchestrator), it does not stop the render.
func (h *HyperframesRenderer) Inspect(ctx context.Context, dir string) (CheckResult, error) {
	return h.runCheck(ctx, "inspect", h.timeout, dir, "inspect")
}

// Render produces an MP4 at outputPath from the composition in dir. Quality is
// standard/24fps so the memory-heavy multi-scene render fits the ~8GB container
// without OOM.
func (h *HyperframesRenderer) Render(ctx context.Context, dir, outputPath string) (CheckResult, error) {
	return h.runCheck(ctx, "render", h.timeout, dir,
		"render", "--output", outputPath, "--quality", "standard", "--fps", "24", "-w", renderWorkers)
}
```

- [ ] **Step 7: แก้ call site ในเทสต์เดิมให้ compile**

ใน `internal/producer/render_sample_test.go` เปลี่ยน 3 จุด (บรรทัด ~104, ~108, ~111):

```go
	if _, err := r.Lint(ctx, dir); err != nil {
		t.Fatalf("lint: %v", err)
	}
	// Inspect guards A2 repositioning against text overflow / caption collision.
	if _, err := r.Inspect(ctx, dir); err != nil {
		t.Fatalf("inspect (overflow/clip): %v", err)
	}
	if _, err := r.Render(ctx, dir, out); err != nil {
		t.Fatalf("render mp4: %v", err)
	}
```

เทสต์ตัวนี้ (`TestRenderSampleA1A4`, guard ด้วย `HF_SAMPLE=1`) คือการครอบคลุม "lint จริงกับเทมเพลตจริง" ที่ spec ระบุไว้ — มันเรียก `Lint` กับโปรเจกต์ที่ประกอบจากเทมเพลตจริงอยู่แล้ว จึงไม่ต้องเขียนเทสต์ integration ใหม่ ใช้รันด้วยมือเมื่อมี Node/Chromium:

Run (ไม่บังคับ ต้องมี Node 22 + Chromium + ไฟล์เสียงของ PoC): `HF_SAMPLE=1 HF_OUT=/tmp/sample.mp4 go test ./internal/producer/ -run TestRenderSampleA1A4 -v`

- [ ] **Step 8: แก้ call site ในสายการผลิตให้ compile (ยังไม่เพิ่ม Lint)**

ใน `internal/producer/producer.go:495-499` เปลี่ยนเป็น:

```go
	if _, err := p.hf.renderer.Inspect(ctx, projectDir); err != nil {
		inspectFlagged, inspectDetail = true, err.Error()
		log.Printf("hyperframes inspect flagged layout issues for clip %s: %v", clipID, err)
	}
	renderRes, err := p.hf.renderer.Render(ctx, projectDir, "output.mp4")
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
```

แล้วแก้บรรทัดที่ใช้ `renderIssues` (ใน `return &assembleOutput{...}`) เป็น:

```go
		renderFlagged:  len(renderRes.Findings) > 0,
```

**ทำไมยังเทียบเท่าของเดิม:** เดิม `renderFlagged` มาจาก `scanBrowserIssues` ล้วน ตอนนี้ `Findings` อาจมีบรรทัดจาก `classifyRunError` ปนได้ — แต่กรณีนั้น `Render` คืน error และฟังก์ชันนี้ `return` ทิ้งไปก่อนถึงบรรทัดนี้เสมอ ค่าที่ไปถึง `renderFlagged` จึงเป็น browser issue ล้วนเหมือนเดิม

ก่อนลบเมธอด `run` เดิม ให้ยืนยันว่าไม่มีผู้เรียกอื่นเหลือ:

Run: `grep -rn "\.run(ctx" internal/producer/`
Expected: ไม่มีผลลัพธ์

- [ ] **Step 9: build + รันเทสต์ทั้งแพ็กเกจ**

Run: `go build ./... && go test ./internal/producer/ -v 2>&1 | tail -20`
Expected: build ผ่าน, เทสต์เดิมทั้งหมดยังเขียว (เทสต์ที่ต้องใช้ Node/Chromium จะขึ้น SKIP ตามปกติ)

- [ ] **Step 10: Commit**

```bash
git add internal/producer/hyperframes.go internal/producer/hyperframes_check_test.go internal/producer/render_sample_test.go internal/producer/producer.go
git commit -m "refactor(qa): ให้ด่าน Hyperframes คืน CheckResult ควบคู่ error เดิม"
```

---

### Task 3: เรียก `Lint` ในสายการผลิต + พาผลขึ้นถึง `ProduceResult`

**Files:**
- Modify: `internal/producer/producer.go` — struct `assembleOutput` (บรรทัด ~303), struct `ProduceResult` (บรรทัด ~66-92), ฟังก์ชัน `AssembleHyperframes916` (บรรทัด ~490-512), ฟังก์ชัน `ProduceHyperframes916` (บรรทัด ~574-583)

**Interfaces:**
- Consumes: `producer.CheckResult` (Task 2)
- Produces: `ProduceResult.Checks []CheckResult` — เรียงตามลำดับที่รันจริง (lint, inspect, render)

- [ ] **Step 1: เพิ่มฟิลด์ใน `assembleOutput`**

```go
type assembleOutput struct {
	mp4Path        string
	sceneDurations []float64
	inspectFlagged bool
	inspectDetail  string
	audioFlagged   bool
	renderFlagged  bool // populated by the render browser-error gate
	// checks คือผลของทุกด่านที่รันจริง เรียงตามลำดับ (lint, inspect, render)
	// เฟส 1 ใช้เพื่อบันทึกอย่างเดียว ไม่มีด่านไหนเปลี่ยนเส้นทางของคลิป
	checks []CheckResult
}
```

- [ ] **Step 2: เพิ่มฟิลด์ใน `ProduceResult`**

เพิ่มต่อท้าย `RenderFlagged`:

```go
	// Checks คือผลดิบของด่าน lint / inspect / render สำหรับบันทึกลง render_checks
	// (เฟส 1 = สังเกตการณ์ล้วน). ว่างสำหรับสายการผลิตแบบ static Produce
	Checks []CheckResult
```

- [ ] **Step 3: เรียก `Lint` แบบ shadow ก่อน `Inspect`**

ใน `AssembleHyperframes916` แทนที่บล็อกตั้งแต่ `inspectFlagged, inspectDetail := false, ""` จนถึงก่อน `audioFlagged := probeVoiceSilent(voicePath)` ด้วย:

```go
	var checks []CheckResult

	// เฟส 1 (spec 2026-08-01): lint เป็นด่านสังเกตการณ์ — บันทึกผลแล้วไปต่อเสมอ
	// เราไม่เคยรัน lint กับเทมเพลตนี้มาก่อน จึงยังไม่รู้ว่ามันจะฟ้องอะไรบ้าง
	// การให้มันบล็อกคลิปตั้งแต่วันแรกจึงเสี่ยงเกินไป
	lintRes, lintErr := p.hf.renderer.Lint(ctx, projectDir)
	checks = append(checks, lintRes)
	if lintErr != nil {
		log.Printf("hyperframes lint (shadow, ไม่บล็อก) clip %s: %v", clipID, lintErr)
	}

	inspectFlagged, inspectDetail := false, ""
	inspectRes, inspectErr := p.hf.renderer.Inspect(ctx, projectDir)
	checks = append(checks, inspectRes)
	if inspectErr != nil {
		inspectFlagged, inspectDetail = true, inspectErr.Error()
		log.Printf("hyperframes inspect flagged layout issues for clip %s: %v", clipID, inspectErr)
	}

	renderRes, err := p.hf.renderer.Render(ctx, projectDir, "output.mp4")
	checks = append(checks, renderRes)
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
```

- [ ] **Step 4: ส่ง `checks` ออกจาก `AssembleHyperframes916`**

ใน `return &assembleOutput{...}` เพิ่มบรรทัด:

```go
		checks:         checks,
```

- [ ] **Step 5: ส่งต่อขึ้น `ProduceResult`**

ใน `ProduceHyperframes916` เพิ่มใน `return &ProduceResult{...}`:

```go
		Checks:            out.checks,
```

- [ ] **Step 6: build + เทสต์**

Run: `go build ./... && go test ./internal/producer/ 2>&1 | tail -5`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/producer/producer.go
git commit -m "feat(qa): เรียก Lint ในสายการผลิตแบบ shadow + พาผล 3 ด่านขึ้นถึง ProduceResult"
```

---

### Task 4: orchestrator เขียน `render_checks` + แก้บั๊ก `ClearFailReason`

**Files:**
- Modify: `internal/orchestrator/orchestrator.go` — struct `Orchestrator` (บรรทัด ~61-97), `New(...)` (บรรทัด ~99-140), บล็อกท้าย `runHyperframes*` (บรรทัด ~800-830)
- Modify: `cmd/server/main.go:117-134`
- Modify: `internal/orchestrator/status_gate_test.go` (เพิ่มเทสต์ใหม่ท้ายไฟล์)

**Interfaces:**
- Consumes: `repository.NewRenderChecksRepo`, `(*RenderChecksRepo).Create`, `repository.StageLint/StageInspect/StageRender` (Task 1); `ProduceResult.Checks`, `CheckResult.FindingsJSON()` (Task 2-3)
- Produces: `orchestrator.shouldClearFailReason(status string) bool`

- [ ] **Step 1: เขียนเทสต์ที่ยังไม่ผ่าน**

เพิ่มท้าย `internal/orchestrator/status_gate_test.go`:

```go
// fail_reason ของคลิป needs_review คือคำอธิบายเดียวที่เหลือว่าทำไมมันถูกกัก
// (log หายภายในไม่กี่ชั่วโมง). ก่อนหน้านี้ ClearFailReason ถูกเรียกแบบไม่มี
// เงื่อนไขทุกครั้ง จึงล้างเหตุผลที่เพิ่งเขียนไป 20 บรรทัดก่อนหน้าเสมอ
func TestShouldClearFailReason(t *testing.T) {
	if !shouldClearFailReason("ready") {
		t.Error(`shouldClearFailReason("ready") = false, want true — คลิปที่กลับมาดีต้องล้างเหตุผลเก่า`)
	}
	for _, s := range []string{"needs_review", "failed"} {
		if shouldClearFailReason(s) {
			t.Errorf("shouldClearFailReason(%q) = true, want false — เหตุผลต้องอยู่ให้คนอ่าน", s)
		}
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่าไม่ผ่าน**

Run: `go test ./internal/orchestrator/ -run TestShouldClearFailReason -v`
Expected: FAIL — `undefined: shouldClearFailReason`

- [ ] **Step 3: เขียนฟังก์ชันและแก้ call site**

เพิ่มฟังก์ชันไว้ใกล้ `downgradeIfReady` ใน `internal/orchestrator/orchestrator.go`:

```go
// shouldClearFailReason: ล้าง fail_reason เฉพาะเมื่อคลิปจบรอบนี้แบบไม่มีปัญหา
// คลิปที่ถูกกัก (needs_review) ต้องเก็บเหตุผลไว้ ไม่งั้นคนถัดไปต้องเรนเดอร์ซ้ำ
// เพื่อหาว่าอะไรพัง (บทเรียนจาก 2026-07-25)
func shouldClearFailReason(status string) bool { return status == "ready" }
```

แล้วเปลี่ยนบรรทัด `o.clipsRepo.ClearFailReason(ctx, clipID)` (บรรทัด ~825) เป็น:

```go
	if shouldClearFailReason(status) {
		o.clipsRepo.ClearFailReason(ctx, clipID)
	}
```

- [ ] **Step 4: รันเทสต์ให้ผ่าน**

Run: `go test ./internal/orchestrator/ -run TestShouldClearFailReason -v`
Expected: PASS

- [ ] **Step 5: เพิ่ม repo เข้า Orchestrator**

ใน struct `Orchestrator` เพิ่มถัดจาก `autoReviewsRepo`:

```go
	renderChecksRepo    *repository.RenderChecksRepo
```

ใน `New(...)` เพิ่มพารามิเตอร์ถัดจาก `autoreviews *repository.AutoReviewsRepo`:

```go
	renderChecks *repository.RenderChecksRepo,
```

และในตัว `return &Orchestrator{...}` เพิ่ม:

```go
		renderChecksRepo: renderChecks,
```

- [ ] **Step 6: เขียนผลลง DB**

แทรกก่อนบล็อก `// A hyperframes layout-inspector flag means visible overflow/clip` (บรรทัด ~798):

```go
	// เฟส 1 (spec 2026-08-01): บันทึกผลทุกด่านไว้ก่อน — ตัวเลขชุดนี้คือสิ่งที่จะ
	// ตัดสินว่าเฟส 2/3 ควรทำอย่างไร. เขียนไม่สำเร็จถือเป็น non-fatal เหมือน
	// visualQARepo.Create — คลิปต้องไม่ตกเพราะตารางสถิติ
	for _, c := range result.Checks {
		if err := o.renderChecksRepo.Create(ctx, clipID, c.Stage, c.Passed, c.DurationMS, c.FindingsJSON()); err != nil {
			log.Printf("render_checks: persist %s for clip %s failed (non-fatal): %v", c.Stage, clipID, err)
		}
	}
```

- [ ] **Step 7: wiring ใน main.go**

หลังบรรทัด `autoReviewsRepo := repository.NewAutoReviewsRepo(pool)` (บรรทัด 120) เพิ่ม:

```go
	renderChecksRepo := repository.NewRenderChecksRepo(pool)
```

แล้วเพิ่ม `renderChecksRepo` ในลิสต์อาร์กิวเมนต์ของ `orchestrator.New(...)` ถัดจาก `autoReviewsRepo` (บรรทัด 132)

- [ ] **Step 8: build + เทสต์ทั้งโปรเจกต์**

Run: `go build ./... && go test ./... 2>&1 | grep -v "^ok\|no test files" | head -20`
Expected: build ผ่าน ไม่มีเทสต์ตก

- [ ] **Step 9: Commit**

```bash
git add internal/orchestrator/orchestrator.go internal/orchestrator/status_gate_test.go cmd/server/main.go
git commit -m "feat(qa): เขียนผล 3 ด่านลง render_checks + แก้บั๊ก ClearFailReason ล้างทับเหตุผลที่เพิ่งเขียน"
```

---

### Task 5: ตรวจกับ Neon branch แล้วเก็บคิวรีสถิติไว้ใช้จริง

**Files:**
- Create: `docs/superpowers/reports/2026-08-01-render-checks-queries.md`

**Interfaces:**
- Consumes: ตาราง `render_checks` (Task 1), ข้อมูลที่เขียนจริง (Task 4)

- [ ] **Step 1: ทดสอบ migration บน Neon branch (ห้ามรันกับ prod)**

สร้าง branch ทดสอบผ่าน Neon MCP (`create_branch` บนโปรเจกต์ `snowy-grass-75448787`) แล้วรัน SQL ในไฟล์ `migrations/079_render_checks.sql` กับ branch นั้นด้วย `run_sql`
Expected: สร้างตารางสำเร็จ และรันซ้ำครั้งที่สองต้องไม่ error (idempotent)

- [ ] **Step 2: ตรวจว่ารันซ้ำได้จริง**

รัน SQL เดิมซ้ำอีกครั้งบน branch เดียวกัน
Expected: ไม่มี error (ทุกคำสั่งมี `IF NOT EXISTS`)

- [ ] **Step 3: ลบ branch ทดสอบ**

ลบ branch ที่สร้างในขั้นที่ 1 ด้วย `delete_branch`

- [ ] **Step 4: เขียนคิวรีที่จะใช้เก็บสถิติ**

สร้าง `docs/superpowers/reports/2026-08-01-render-checks-queries.md`:

````markdown
# คิวรีสถิติ render_checks (เฟส 1)

ใช้ตอบ 3 คำถามที่ตัดสินว่าเฟส 2/3 ควรทำอย่างไร รันหลัง deploy ~2 สัปดาห์ (~40 คลิป)
โปรเจกต์ Neon: `snowy-grass-75448787`

## 1. แต่ละด่าน fail กี่เปอร์เซ็นต์

```sql
SELECT stage,
       count(*) AS runs,
       count(*) FILTER (WHERE NOT passed) AS failed,
       round(100.0 * count(*) FILTER (WHERE NOT passed) / count(*), 1) AS fail_pct
FROM render_checks
GROUP BY stage ORDER BY stage;
```

## 2. lint กินเวลาเท่าไร

```sql
SELECT stage,
       percentile_disc(0.5) WITHIN GROUP (ORDER BY duration_ms) AS median_ms,
       percentile_disc(0.95) WITHIN GROUP (ORDER BY duration_ms) AS p95_ms,
       max(duration_ms) AS max_ms
FROM render_checks GROUP BY stage ORDER BY stage;
```

## 3. finding ที่พบซ้ำ (แยก runner_error ออกจาก finding จริง)

```sql
SELECT stage,
       f.value #>> '{}' AS finding,
       count(*) AS hits
FROM render_checks rc, jsonb_array_elements(rc.findings) AS f(value)
WHERE f.value #>> '{}' NOT LIKE 'runner_error:%'
GROUP BY stage, finding
ORDER BY hits DESC LIMIT 20;
```

## 4. รัน CLI ไม่ได้บ่อยแค่ไหน (ต้องแยกจากข้อ 3 ก่อนเปิด gate ในเฟส 2)

```sql
SELECT stage, count(*) AS runner_errors
FROM render_checks rc, jsonb_array_elements(rc.findings) AS f(value)
WHERE f.value #>> '{}' LIKE 'runner_error:%'
GROUP BY stage ORDER BY stage;
```

## เกณฑ์ตัดสินเฟส 2

- ข้อ 1 บอกว่าเปิด gate แล้วคลิปจะถูกบล็อกกี่ % — ถ้า lint fail เกิน ~20% ต้องแก้เทมเพลตก่อน ไม่ใช่เปิด gate
- ข้อ 2 บอกว่า lint คุ้มค่าที่จะรันก่อนเรนเดอร์ไหม (ถ้า median เกิน ~60 วินาที ต้องคิดใหม่)
- ข้อ 4 ต้องใกล้ 0 ก่อนเปิด gate — ไม่งั้นเราจะบล็อกคลิปเพราะ npx ล่ม
````

- [ ] **Step 5: Commit**

```bash
git add docs/superpowers/reports/2026-08-01-render-checks-queries.md
git commit -m "docs(qa): คิวรีสถิติ render_checks สำหรับตัดสินเฟส 2"
```

---

## หลัง merge — สิ่งที่ต้องรู้

- migration 079 จะรันอัตโนมัติตอน deploy (`cmd/server/main.go:54` เรียก `RunMigrations`)
- **ห้าม deploy ระหว่างที่มีคลิปกำลังผลิต** — การผลิตเป็น detached goroutine (ดู `project_production_non_durable`)
- คลิปแรกหลัง deploy จะได้ 3 แถวใน `render_checks` ทันที ตรวจได้ด้วย:

```sql
SELECT stage, passed, duration_ms, findings FROM render_checks ORDER BY created_at DESC LIMIT 3;
```

- ถ้าแถว `lint` ขึ้น `passed=false` ทุกคลิป **นั่นเป็นข้อมูล ไม่ใช่เหตุฉุกเฉิน** — เฟส 1 ออกแบบมาเพื่อค้นพบเรื่องนี้ให้ได้โดยไม่มีคลิปไหนถูกบล็อก
