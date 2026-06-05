package producer

import (
	"context"
	"fmt"
	"log"
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
	// 10m headroom: multi-scene / landscape renders are heavier than the original
	// single-scene 9:16 (a 16:9 render with a CSS background timed out at 6m).
	return &HyperframesRenderer{timeout: 10 * time.Minute}
}

func (h *HyperframesRenderer) run(ctx context.Context, dir string, args ...string) error {
	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	cmd := hyperframesCmd(ctx, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	// A render can exit 0 while the page silently failed (e.g. a JS exception
	// froze every animation, producing a static video). Hyperframes prints those
	// as "[Browser:PAGEERROR]" / CDN-fetch warnings — surface them either way so
	// a "successful" but broken render is never invisible.
	if issues := scanBrowserIssues(out); len(issues) > 0 {
		log.Printf("hyperframes %v browser issues:\n%s", args, strings.Join(issues, "\n"))
	}
	if err != nil {
		return fmt.Errorf("hyperframes %v failed: %w\n%s", args, err, lastBytes(out, 600))
	}
	return nil
}

// scanBrowserIssues pulls the lines that signal a silent in-page failure (a JS
// exception or a missing CDN dependency) out of the Hyperframes CLI output.
func scanBrowserIssues(out []byte) []string {
	var hits []string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "[Browser:PAGEERROR]") ||
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
	return h.run(ctx, dir, "lint")
}

// Inspect runs Hyperframes' collision/overflow auditor (canvas_overflow,
// container_overflow, clipped_text) in headless Chrome. Use it as a gate after
// Lint so a layout with overlapping or clipped elements is caught before render.
func (h *HyperframesRenderer) Inspect(ctx context.Context, dir string) error {
	return h.run(ctx, dir, "inspect")
}

// renderWorkers parallelizes frame capture across Chrome instances. The Railway
// container has ~8GB RAM (an earlier comment wrongly assumed 32GB). Each Chrome
// worker is memory-heavy: 12 workers OOM-killed the heavier 16:9 multi-scene
// render (peaked ~7.6GB then SIGKILL), which silently fell back to a static image.
// 6 workers + standard/24fps (below) fits 8GB with headroom and still finishes
// within the timeout. Raise this only if the container's RAM is actually raised.
const renderWorkers = "6"

// Render produces an MP4 at outputPath from the composition in dir. Quality is
// standard/24fps (not high/30) so the memory-heavy multi-scene render fits the
// ~8GB container without OOM; parallel workers keep it within the timeout.
func (h *HyperframesRenderer) Render(ctx context.Context, dir, outputPath string) error {
	return h.run(ctx, dir, "render", "--output", outputPath, "--quality", "standard", "--fps", "24", "-w", renderWorkers)
}

func lastBytes(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[len(b)-n:])
}
