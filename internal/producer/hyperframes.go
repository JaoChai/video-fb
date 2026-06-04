package producer

import (
	"context"
	"fmt"
	"os/exec"
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
	if err != nil {
		return fmt.Errorf("hyperframes %v failed: %w\n%s", args, err, lastBytes(out, 600))
	}
	return nil
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

// renderWorkers parallelizes frame capture across Chrome instances. Hyperframes'
// auto setting caps at ~6 regardless of host size; our container has 32 vCPU /
// 32GB, so 12 workers (~3GB) cuts render time enough to keep high/30 within the
// timeout — including the heavier 16:9 render — WITHOUT lowering quality.
const renderWorkers = "12"

// Render produces a high-quality (high/30fps) MP4 at outputPath from the
// composition in dir, using parallel render workers to stay within the timeout.
func (h *HyperframesRenderer) Render(ctx context.Context, dir, outputPath string) error {
	return h.run(ctx, dir, "render", "--output", outputPath, "--quality", "high", "--fps", "30", "-w", renderWorkers)
}

func lastBytes(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[len(b)-n:])
}
