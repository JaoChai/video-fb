package producer

import (
	"os"
	"path/filepath"
	"testing"
)

// The render is offline — every font a theme references MUST be vendored as a
// local .ttf, or @font-face falls back to a system font (or nothing) at render.
func TestVendoredFontsPresent(t *testing.T) {
	want := []string{
		"Sarabun-Regular.ttf", "Sarabun-SemiBold.ttf", "Sarabun-Bold.ttf", "Sarabun-ExtraBold.ttf",
		"Kanit-Bold.ttf", "Kanit-ExtraBold.ttf", "Kanit-Black.ttf",
		"Prompt-SemiBold.ttf", "Prompt-Bold.ttf", "Prompt-ExtraBold.ttf",
		"IBMPlexSansThai-Medium.ttf", "IBMPlexSansThai-SemiBold.ttf",
	}
	dir := filepath.Join("assets", "fonts")
	for _, f := range want {
		info, err := os.Stat(filepath.Join(dir, f))
		if err != nil {
			t.Errorf("missing vendored font %s: %v", f, err)
			continue
		}
		if info.Size() < 20_000 {
			t.Errorf("font %s too small (%d bytes) — likely a failed/HTML download", f, info.Size())
		}
	}
}
