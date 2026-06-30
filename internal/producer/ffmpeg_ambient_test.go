package producer

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBuildAmbientBed(t *testing.T) {
	ff, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "src.mp3")
	// 2s source tone.
	if out, err := exec.Command(ff, "-f", "lavfi", "-i", "sine=frequency=120:duration=2",
		"-ar", "44100", "-y", src).CombinedOutput(); err != nil {
		t.Fatalf("make src: %v\n%s", err, out)
	}
	a := NewFFmpegAssembler(ff, "")
	out := filepath.Join(dir, "ambient.mp3")
	if err := a.BuildAmbientBed(src, out, 6); err != nil {
		t.Fatalf("BuildAmbientBed: %v", err)
	}
	fi, err := os.Stat(out)
	if err != nil || fi.Size() == 0 {
		t.Fatalf("output missing/empty: %v", err)
	}
}
