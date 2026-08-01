package producer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// hyperframesVersion pins the CLI so renders are reproducible across machines.
const hyperframesVersion = "0.6.70"

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

// HyperframesRenderer shells out to the Hyperframes CLI to lint and render a
// composition project directory (the dir must contain index.html, package.json,
// meta.json, hyperframes.json and assets/).
type HyperframesRenderer struct {
	timeout time.Duration
}

func NewHyperframesRenderer() *HyperframesRenderer {
	// 20m headroom: fewer render workers (see renderWorkers) trade parallelism for
	// CPU-per-worker, so wall-clock is longer; the earlier 10m was tuned for 6
	// workers (a 16:9 render with a CSS background timed out at 6m back then).
	return &HyperframesRenderer{timeout: 20 * time.Minute}
}

// RenderErrorGateEnabled turns on treating a browser-error-flagged render as a
// failure (retry) then routing to needs_review. Off → legacy log-only behavior.
func RenderErrorGateEnabled() bool { return os.Getenv("RENDER_ERROR_GATE_ENABLED") == "true" }

// RenderGateAction is what to do about a render that tripped browser-error detection.
type RenderGateAction int

const (
	RenderGateNone   RenderGateAction = iota // do nothing (not flagged, or gate off)
	RenderGateRetry                          // fail the clip so the retry tick re-renders
	RenderGateReview                         // still broken after a retry → human review
)

// RenderGateDecision decides how to handle a render flagged with browser errors.
// First offense (retryCount 0) retries; a render still broken after at least one
// retry goes to human review. A frozen render exits 0 and looks fine to the
// still-frame vision QA, so this is the only place it can be caught.
func RenderGateDecision(flagged, gateEnabled bool, retryCount int) RenderGateAction {
	if !flagged || !gateEnabled {
		return RenderGateNone
	}
	if retryCount == 0 {
		return RenderGateRetry
	}
	return RenderGateReview
}

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

// scanBrowserIssues pulls the lines that signal a silent in-page failure (a JS
// exception or a missing CDN dependency) out of the Hyperframes CLI output.
func scanBrowserIssues(out []byte) []string {
	var hits []string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "[Browser:PAGEERROR]") ||
			strings.Contains(line, "Composition script failed") || // GSAP/timeline threw (Hyperframes wraps each script in try/catch)
			strings.Contains(line, "Failed to download CDN script") ||
			strings.Contains(line, "is not defined") {
			hits = append(hits, strings.TrimSpace(line))
		}
	}
	return hits
}

// hyperframesCmd prefers the globally-installed CLI (the Docker image installs
// the pinned version) so a render never hits the npm registry. It falls back to
// `npx hyperframes@<version>` for local dev where the CLI isn't installed globally.
func hyperframesCmd(ctx context.Context, args ...string) *exec.Cmd {
	if bin, err := exec.LookPath("hyperframes"); err == nil {
		return exec.CommandContext(ctx, bin, args...)
	}
	full := append([]string{"--yes", "hyperframes@" + hyperframesVersion}, args...)
	return exec.CommandContext(ctx, "npx", full...)
}

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

// renderWorkers parallelizes frame capture across Chrome instances on the ~8GB /
// ~8 vCPU Railway container. History: 12 workers OOM-killed the 16:9 render
// (~7.6GB then SIGKILL); 6 fixed the OOM but then OVERSUBSCRIBED the CPU — a
// single Chrome capture starved >5m and blew hyperframes' 300s protocolTimeout
// (memory peaked only ~2.5GB, so this was CPU contention, not RAM). 3 gives each
// worker enough CPU to finish a frame in seconds; it sits below hyperframes'
// default of 4, trading wall-clock for reliability on this constrained box. Raise
// only if the container's CPU is actually raised.
const renderWorkers = "3"

// Render produces an MP4 at outputPath from the composition in dir. Quality is
// standard/24fps so the memory-heavy multi-scene render fits the ~8GB container
// without OOM.
func (h *HyperframesRenderer) Render(ctx context.Context, dir, outputPath string) (CheckResult, error) {
	return h.runCheck(ctx, "render", h.timeout, dir,
		"render", "--output", outputPath, "--quality", "standard", "--fps", "24", "-w", renderWorkers)
}

func lastBytes(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[len(b)-n:])
}
