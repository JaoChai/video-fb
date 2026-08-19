# Cloudflare Migration Phase 1: Container Tick-Dispatch Endpoint

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** เพิ่มโหมด worker ให้ `cmd/server/main.go` ที่เปิดเฉพาะ endpoint ภายใน `POST /internal/tick/{action}` แทนการเปิด public API เต็มรูปแบบ — endpoint นี้เรียก dispatch table เดิมของ `internal/scheduler` (12 actions ที่ `scheduler.go` ผูกกับ cron รายตัวอยู่แล้ว) โดยไม่แตะ `internal/orchestrator` เลย

**Architecture:** `Scheduler.handlerFor(action)` เป็น private อยู่แล้วและ map ชื่อ action → `func(context.Context) error` ครบ 12 ตัว งานคือ export ผ่าน method ใหม่ `Dispatch`, เพิ่ม HTTP handler บางๆ ที่หุ้ม `Dispatch`, แล้วให้ `main.go` เลือกว่าจะ mount public router (เดิม) หรือ tick-only mux (ใหม่) ตามค่า config `WORKER_MODE` — ส่วนที่จะรันจริงบน Cloudflare (Worker เรียก container endpoint นี้ผ่าน Workflow) เป็นแผนแยกถัดไป ไม่อยู่ในแผนนี้

**Tech Stack:** Go 1.25 stdlib `net/http` (ServeMux path patterns ของ Go 1.22+, ไม่ใช้ chi สำหรับ mux นี้เพราะไม่มี middleware/CORS ที่ต้องแชร์กับ public router)

## Global Constraints

- ห้ามแก้ `internal/orchestrator/orchestrator.go` — ตกลงไว้ในสเปกว่า business logic ไม่พอร์ต/ไม่แตะ (`docs/superpowers/specs/2026-08-19-cloudflare-migration-design.md`)
- ห้ามเปลี่ยนพฤติกรรมของ public API เดิม (`internal/router/router.go`) — โหมดปกติ (`WORKER_MODE` ไม่ตั้งหรือ `false`) ต้องทำงานเหมือนเดิมทุกประการ
- `SCHEDULER_ENABLED` เป็นตัวปิด internal cron อยู่แล้ว (ค่า default `true`) — แผนนี้ไม่เพิ่ม flag ใหม่ซ้ำซ้อน การปิด internal cron ตอน deploy จริงทำผ่าน env `SCHEDULER_ENABLED=false` ที่มีอยู่แล้ว
- ทุก error ที่ dispatch คืนต้อง map เป็น HTTP status ที่สื่อความหมายจริง: unknown action → 404, `orchestrator.ErrProductionRunning` → 409 (มี precedent เดียวกันกับ `ErrRerenderNotAllowed` ในโค้ดเดิม), อื่นๆ → 500

---

### Task 1: Export `Scheduler.Dispatch`

**Files:**
- Modify: `internal/scheduler/scheduler.go`
- Test: `internal/scheduler/dispatch_test.go`

**Interfaces:**
- Produces: `var ErrUnknownAction = errors.New("unknown action")` · `func (s *Scheduler) Dispatch(ctx context.Context, action string) error`

- [ ] **Step 1: Write the failing test**

```go
// internal/scheduler/dispatch_test.go
package scheduler

import (
	"context"
	"errors"
	"testing"
)

func TestDispatchUnknownAction(t *testing.T) {
	s := &Scheduler{}
	err := s.Dispatch(context.Background(), "does_not_exist")
	if !errors.Is(err, ErrUnknownAction) {
		t.Errorf("Dispatch(unknown) = %v, want ErrUnknownAction", err)
	}
}

func TestDispatchKnownActionResolves(t *testing.T) {
	// ไม่เรียกจริง (handler ต้องการ pool/orchestrator ที่ยังไม่ได้ inject) —
	// แค่ยืนยันว่า Dispatch หา handler เจอและไม่คืน ErrUnknownAction
	// ก่อนจะ panic ตอนเรียก h(ctx) ด้วย receiver ว่าง จึงต้องดัก panic แทน
	s := &Scheduler{}
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic from calling handler on zero-value Scheduler (nil orchestrator), got none — Dispatch may not be routing to the real handler")
		}
	}()
	_ = s.Dispatch(context.Background(), "retry_failed")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scheduler/... -run TestDispatch -v`
Expected: FAIL — `s.Dispatch` undefined (compile error) และ `ErrUnknownAction` undefined

- [ ] **Step 3: Write minimal implementation**

เพิ่มต่อท้าย `internal/scheduler/scheduler.go` (ใต้ import ที่มี `"errors"` อยู่แล้ว):

```go
// ErrUnknownAction is returned by Dispatch when the action name isn't in
// handlerFor's table — the Cloudflare tick endpoint maps this to HTTP 404.
var ErrUnknownAction = errors.New("unknown action")

// Dispatch runs the same handler a cron tick would run for this action name.
// It exists so an external trigger (Cloudflare Workflow) can invoke exactly
// what internal cron ticks invoke today, without duplicating the table in
// handlerFor or touching orchestrator.go.
func (s *Scheduler) Dispatch(ctx context.Context, action string) error {
	h := s.handlerFor(action)
	if h == nil {
		return fmt.Errorf("%w: %q", ErrUnknownAction, action)
	}
	return h(ctx)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scheduler/... -run TestDispatch -v`
Expected: PASS (ทั้งสองเทสต์)

- [ ] **Step 5: Run full scheduler test suite to check no regression**

Run: `go test ./internal/scheduler/...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/scheduler/scheduler.go internal/scheduler/dispatch_test.go
git commit -m "feat(scheduler): export Dispatch(action) for external (Cloudflare Workflow) tick triggers"
```

---

### Task 2: Tick HTTP handler

**Files:**
- Create: `internal/handler/tick.go`
- Test: `internal/handler/tick_test.go`

**Interfaces:**
- Consumes: `scheduler.ErrUnknownAction` (Task 1) · `orchestrator.ErrProductionRunning` (already exists at `internal/orchestrator/orchestrator.go:147`)
- Produces: `func NewTickHandler(dispatch func(ctx context.Context, action string) error) http.HandlerFunc`

- [ ] **Step 1: Write the failing test**

```go
// internal/handler/tick_test.go
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jaochai/video-fb/internal/orchestrator"
	"github.com/jaochai/video-fb/internal/scheduler"
)

func TestTickHandler(t *testing.T) {
	cases := []struct {
		name       string
		dispatch   func(ctx context.Context, action string) error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "success",
			dispatch:   func(ctx context.Context, action string) error { return nil },
			wantStatus: http.StatusOK,
			wantBody:   "ok",
		},
		{
			name: "unknown action",
			dispatch: func(ctx context.Context, action string) error {
				return fmt.Errorf("%w: %q", scheduler.ErrUnknownAction, action)
			},
			wantStatus: http.StatusNotFound,
			wantBody:   "error",
		},
		{
			name: "production already running",
			dispatch: func(ctx context.Context, action string) error {
				return orchestrator.ErrProductionRunning
			},
			wantStatus: http.StatusConflict,
			wantBody:   "error",
		},
		{
			name: "other failure",
			dispatch: func(ctx context.Context, action string) error {
				return errors.New("boom")
			},
			wantStatus: http.StatusInternalServerError,
			wantBody:   "error",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.Handle("POST /internal/tick/{action}", NewTickHandler(c.dispatch))

			req := httptest.NewRequest(http.MethodPost, "/internal/tick/produce_and_publish", nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != c.wantStatus {
				t.Errorf("status = %d, want %d (body: %s)", w.Code, c.wantStatus, w.Body.String())
			}
			var got map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatalf("response not valid JSON: %v (%s)", err, w.Body.String())
			}
			if got["status"] != c.wantBody {
				t.Errorf("status field = %q, want %q", got["status"], c.wantBody)
			}
			if got["action"] != "produce_and_publish" {
				t.Errorf("action field = %q, want %q", got["action"], "produce_and_publish")
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/handler/... -run TestTickHandler -v`
Expected: FAIL — `NewTickHandler` undefined

- [ ] **Step 3: Write minimal implementation**

```go
// internal/handler/tick.go
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/jaochai/video-fb/internal/orchestrator"
	"github.com/jaochai/video-fb/internal/scheduler"
)

// NewTickHandler wraps a Dispatch func (scheduler.Scheduler.Dispatch in
// production) as an HTTP handler for POST /internal/tick/{action}. This is
// the endpoint a Cloudflare Workflow step calls instead of the Go binary's
// own internal cron — see docs/superpowers/specs/2026-08-19-cloudflare-migration-design.md.
func NewTickHandler(dispatch func(ctx context.Context, action string) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		action := r.PathValue("action")
		err := dispatch(r.Context(), action)

		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			status := http.StatusInternalServerError
			switch {
			case errors.Is(err, scheduler.ErrUnknownAction):
				status = http.StatusNotFound
			case errors.Is(err, orchestrator.ErrProductionRunning):
				status = http.StatusConflict
			}
			log.Printf("tick %q failed (%d): %v", action, status, err)
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "action": action, "error": err.Error()})
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "action": action})
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/handler/... -run TestTickHandler -v`
Expected: PASS (ทั้ง 4 case)

- [ ] **Step 5: Commit**

```bash
git add internal/handler/tick.go internal/handler/tick_test.go
git commit -m "feat(handler): add /internal/tick/{action} handler wrapping scheduler.Dispatch"
```

---

### Task 3: Worker mode in `main.go`

**Files:**
- Modify: `internal/config/config.go`
- Modify: `cmd/server/main.go`
- Test: `internal/config/config_test.go` (ไฟล์ใหม่ — ยังไม่มี test ของ config package)

**Interfaces:**
- Consumes: `scheduler.New(...)` (มีอยู่แล้ว, ไม่เปลี่ยน signature) · `handler.NewTickHandler` (Task 2) · `handler.HealthCheck` (มีอยู่แล้วที่ `internal/handler/health.go:5`)
- Produces: `Config.WorkerMode bool`

- [ ] **Step 1: Write the failing test**

```go
// internal/config/config_test.go
package config

import "testing"

func TestLoadWorkerModeDefaultsFalse(t *testing.T) {
	t.Setenv("WORKER_MODE", "")
	cfg := Load()
	if cfg.WorkerMode {
		t.Error("WorkerMode should default to false when WORKER_MODE is unset")
	}
}

func TestLoadWorkerModeTrue(t *testing.T) {
	t.Setenv("WORKER_MODE", "true")
	cfg := Load()
	if !cfg.WorkerMode {
		t.Error("WorkerMode should be true when WORKER_MODE=true")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/... -v`
Expected: FAIL — `cfg.WorkerMode` undefined (compile error)

- [ ] **Step 3: Write minimal implementation**

`internal/config/config.go` — เพิ่ม field ในฟิลด์ `Config` struct ต่อจาก `SchedulerEnabled`:

```go
	SchedulerEnabled bool
	WorkerMode       bool
```

และในฟังก์ชัน `Load()` ต่อจาก `SchedulerEnabled: envBool("SCHEDULER_ENABLED", true),`:

```go
		SchedulerEnabled: envBool("SCHEDULER_ENABLED", true),
		WorkerMode:       envBool("WORKER_MODE", false),
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/... -v`
Expected: PASS

- [ ] **Step 5: Wire worker mode into `main.go`**

`cmd/server/main.go` — แทนที่บล็อกนี้ (บรรทัด 182-188 ปัจจุบัน):

```go
	r := router.New(pool, cfg.APIKey, ragEngine, tracker, pub, func() {
		if err := sched.Reload(ctx); err != nil {
			log.Printf("Scheduler reload failed: %v", err)
		}
	}, prod)
	orchHandler := handler.NewOrchestratorHandler(orch, tracker, pub)
	router.SetOrchestrator(r, orchHandler)
```

ด้วย:

```go
	var h http.Handler
	if cfg.WorkerMode {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /health", handler.HealthCheck)
		mux.Handle("POST /internal/tick/{action}", handler.NewTickHandler(sched.Dispatch))
		h = mux
		log.Println("WORKER_MODE=true — เปิดเฉพาะ /health และ /internal/tick/{action} (ไม่มี public API)")
	} else {
		r := router.New(pool, cfg.APIKey, ragEngine, tracker, pub, func() {
			if err := sched.Reload(ctx); err != nil {
				log.Printf("Scheduler reload failed: %v", err)
			}
		}, prod)
		orchHandler := handler.NewOrchestratorHandler(orch, tracker, pub)
		router.SetOrchestrator(r, orchHandler)
		h = r
	}
```

และบรรทัดถัดมาที่สร้าง server เปลี่ยนจาก `srv := &http.Server{Addr: addr, Handler: r}` เป็น `srv := &http.Server{Addr: addr, Handler: h}`

**หมายเหตุสำคัญ:** `sched := scheduler.New(...)` และ `if cfg.SchedulerEnabled { sched.Start(ctx) }` (บรรทัด 169-180 เดิม) **ต้องยังสร้าง `sched` เหมือนเดิมทั้งสองโหมด** — worker mode ยังต้องมี `sched` object ไว้ให้ `handler.NewTickHandler(sched.Dispatch)` เรียกใช้ แค่ **ไม่ควร** ให้ `sched.Start(ctx)` ทำงานตอน deploy จริงบน Cloudflare (ปิดผ่าน env `SCHEDULER_ENABLED=false` ตามที่ระบุใน Global Constraints — ไม่ใช่โค้ดที่ต้องแก้ในแผนนี้)

- [ ] **Step 6: Build to verify it compiles**

Run: `go build ./...`
Expected: สำเร็จ ไม่มี error

- [ ] **Step 7: Run full test suite**

Run: `go test ./...`
Expected: PASS ทั้งหมด (ไม่มี regression ในแพ็กเกจอื่น)

- [ ] **Step 8: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go cmd/server/main.go
git commit -m "feat(server): add WORKER_MODE — serves only /health + /internal/tick/{action} when set"
```

---

### Task 4: Manual smoke test (local, safe — no production side effects)

**Files:** ไม่มีไฟล์ใหม่ — ขั้นตอนตรวจด้วยมือก่อนถือว่าแผนนี้เสร็จ

- [ ] **Step 1: รันเซิร์ฟเวอร์ในโหมด worker กับ DB dev/local**

```bash
WORKER_MODE=true DATABASE_URL="$DATABASE_URL" API_KEY=unused go run ./cmd/server
```

Expected log: `WORKER_MODE=true — เปิดเฉพาะ /health และ /internal/tick/{action} (ไม่มี public API)`

- [ ] **Step 2: ตรวจ /health**

```bash
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/health
```

Expected: `200`

- [ ] **Step 3: ตรวจ unknown action คืน 404 (ไม่กระทบข้อมูลจริง — action ไม่มีอยู่จริงจึงไม่ถูก dispatch ไปเรียกอะไร)**

```bash
curl -s -o /dev/null -w "%{http_code}\n" -X POST http://localhost:8080/internal/tick/does_not_exist
```

Expected: `404`

- [ ] **Step 4: ตรวจว่า public API ปิดจริงในโหมดนี้**

```bash
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/api/v1/clips
```

Expected: `404` (ServeMux ไม่รู้จัก route นี้ — ยืนยันว่า public router ไม่ได้ mount)

- [ ] **Step 5: ตรวจว่าโหมดปกติ (ไม่ตั้ง WORKER_MODE) ยังเหมือนเดิม**

```bash
DATABASE_URL="$DATABASE_URL" API_KEY=unused go run ./cmd/server &
sleep 2
curl -s -o /dev/null -w "%{http_code}\n" -H "Authorization: Bearer unused" http://localhost:8080/api/v1/clips
kill %1
```

Expected: `200` (public API ทำงานปกติเหมือนก่อนแผนนี้)

**ไม่ต้องทดสอบเรียก action จริง** (`produce_and_publish` ฯลฯ) ในขั้นนี้ — action เหล่านั้นเรียก LLM/kie/Zernio จริงและสร้างคลิปจริง ไม่ใช่ smoke test ที่ปลอดภัย การตรวจว่า dispatch ไปถูก handler แล้วอยู่ใน Task 1's `TestDispatchKnownActionResolves` (ยืนยันด้วย panic ที่มาจาก handler จริงถูกเรียก ไม่ใช่ `ErrUnknownAction`)

---

## สิ่งที่แผนนี้ยังไม่ทำ (ตั้งใจ — อยู่ในแผนถัดไป)

- ยังไม่มี Cloudflare project (`cf/` scaffold), Container binding, Workflow, หรือ Cron Trigger — แผนนี้ทำแค่ฝั่ง Go ที่รันได้ทั้งบน Railway (โหมดปกติ, ทดสอบ regression) และพร้อมสำหรับ deploy เป็น Cloudflare Container image ในแผนถัดไปโดยไม่ต้องแก้ไฟล์เหล่านี้อีก
- ยังไม่พอร์ต public API 55 routes เป็น Worker TypeScript — เป็นแผนแยกที่ใหญ่กว่านี้มาก ควรเขียนหลังแผนนี้ execute เสร็จ

## Self-Review

**Spec coverage:** แผนนี้ครอบคลุมส่วน "Container: Go เดิม → step endpoints" ของ spec (ฉบับปรับปรุงหลัง brainstorming — tick-level แทน per-clip 3-step) ครบ: dispatch table export, HTTP wrapper, worker-mode wiring, ทดสอบว่าไม่กระทบโหมดปกติ ส่วนที่เหลือของ spec (Cloudflare scaffold, Workflow, API port, cron, cutover) ตั้งใจแยกเป็นแผนถัดไปตามที่แจ้งไว้ข้างต้น

**Placeholder scan:** ไม่มี TBD/TODO — ทุก step มีโค้ดจริงที่ตรวจสอบ signature กับโค้ดปัจจุบันแล้ว (`internal/scheduler/scheduler.go`, `internal/handler/health.go:5`, `internal/orchestrator/orchestrator.go:147`, `internal/config/config.go`)

**Type consistency:** `Dispatch(ctx context.Context, action string) error` (Task 1) ตรงกับ `dispatch func(ctx context.Context, action string) error` ที่ `NewTickHandler` รับ (Task 2) ตรงกับที่ `main.go` เรียก `handler.NewTickHandler(sched.Dispatch)` (Task 3) — ตรวจ method value binding แล้ว ใช้ได้ตรงเพราะ `sched` เป็น `*scheduler.Scheduler` และ `Dispatch` มี pointer receiver
