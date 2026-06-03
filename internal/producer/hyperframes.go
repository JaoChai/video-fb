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
	return &HyperframesRenderer{timeout: 6 * time.Minute}
}

func (h *HyperframesRenderer) run(ctx context.Context, dir string, args ...string) error {
	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	full := append([]string{"--yes", "hyperframes@" + hyperframesVersion}, args...)
	cmd := exec.CommandContext(ctx, "npx", full...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("hyperframes %v failed: %w\n%s", args, err, lastBytes(out, 600))
	}
	return nil
}

// Lint runs lint+validate+inspect; use it as a guardrail before Render so a
// broken composition can fall back to a simpler layout instead of producing junk.
func (h *HyperframesRenderer) Lint(ctx context.Context, dir string) error {
	return h.run(ctx, dir, "lint")
}

// Render produces an MP4 at outputPath from the composition in dir.
func (h *HyperframesRenderer) Render(ctx context.Context, dir, outputPath string) error {
	return h.run(ctx, dir, "render", "--output", outputPath, "--quality", "high", "--fps", "30")
}

func lastBytes(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[len(b)-n:])
}
