package producer

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// hyperframesVersion pins the CLI so renders are reproducible across machines.
const hyperframesVersion = "0.6.70"

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

func (h *HyperframesRenderer) run(ctx context.Context, dir string, args ...string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	cmd := hyperframesCmd(ctx, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	// A render can exit 0 while the page silently failed (e.g. a JS exception
	// froze every animation, producing a static video). Hyperframes prints those
	// as "[Browser:PAGEERROR]" / CDN-fetch warnings — surface them either way so
	// a "successful" but broken render is never invisible.
	issues := scanBrowserIssues(out)
	if len(issues) > 0 {
		log.Printf("hyperframes %v browser issues:\n%s", args, strings.Join(issues, "\n"))
	}
	if err != nil {
		return issues, fmt.Errorf("hyperframes %v failed: %w\n%s", args, err, lastBytes(out, 600))
	}
	return issues, nil
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

// Lint runs lint+validate+inspect; use it as a guardrail before Render so a
// broken composition can fall back to a simpler layout instead of producing junk.
func (h *HyperframesRenderer) Lint(ctx context.Context, dir string) error {
	_, err := h.run(ctx, dir, "lint")
	return err
}

// Inspect runs Hyperframes' collision/overflow auditor (canvas_overflow,
// container_overflow, clipped_text) in headless Chrome. Use it as a gate after
// Lint so a layout with overlapping or clipped elements is caught before render.
func (h *HyperframesRenderer) Inspect(ctx context.Context, dir string) error {
	_, err := h.run(ctx, dir, "inspect")
	return err
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

// Render produces an MP4 at outputPath from the composition in dir. It returns
// any browser-error lines the render emitted (a render can exit 0 while a JS
// exception froze every animation into a static video). Quality is standard/24fps
// so the memory-heavy multi-scene render fits the ~8GB container without OOM.
func (h *HyperframesRenderer) Render(ctx context.Context, dir, outputPath string) ([]string, error) {
	return h.run(ctx, dir, "render", "--output", outputPath, "--quality", "standard", "--fps", "24", "-w", renderWorkers)
}

func lastBytes(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[len(b)-n:])
}
