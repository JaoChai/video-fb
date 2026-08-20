package producer

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseCgroupBytes(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    int64
		wantOK  bool
	}{
		{"cgroup v2 ไม่จำกัด", "max\n", 0, false},
		{"ค่าปกติ 8GB", "8589934592\n", 8589934592, true},
		{"มีช่องว่างหน้าหลัง", "  1048576 \n", 1048576, true},
		{"sentinel ของ v1 = ไม่จำกัด", "9223372036854771712", 0, false},
		{"ว่าง", "", 0, false},
		{"ไม่ใช่ตัวเลข", "abc", 0, false},
		{"ศูนย์", "0", 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := parseCgroupBytes(c.in)
			if got != c.want || ok != c.wantOK {
				t.Errorf("parseCgroupBytes(%q) = (%d, %v), want (%d, %v)", c.in, got, ok, c.want, c.wantOK)
			}
		})
	}
}

func TestHostSnapshotAlwaysReportsDisk(t *testing.T) {
	got := hostSnapshot(t.TempDir())
	if !strings.Contains(got, "disk_free") {
		t.Errorf("hostSnapshot = %q, ต้องมี disk_free เสมอ (statfs ใช้ได้ทุก OS ที่เรารัน)", got)
	}
	if !strings.Contains(got, "mem ") {
		t.Errorf("hostSnapshot = %q, ต้องมีช่อง mem เสมอ (นอก Linux ให้เป็น n/a ไม่ใช่หายไป)", got)
	}
}

func TestHostSnapshotWithCgroupValues(t *testing.T) {
	tmpdir := t.TempDir()
	saveCurr := cgroupMemCurrentPaths
	saveMax := cgroupMemMaxPaths
	t.Cleanup(func() {
		cgroupMemCurrentPaths = saveCurr
		cgroupMemMaxPaths = saveMax
	})

	currFile := tmpdir + "/memory.current"
	maxFile := tmpdir + "/memory.max"
	cgroupMemCurrentPaths = []string{currFile}
	cgroupMemMaxPaths = []string{maxFile}

	// Test limited memory (2143MB / 8192MB)
	if err := os.WriteFile(currFile, []byte("2247294976\n"), 0644); err != nil {
		t.Fatalf("WriteFile current: %v", err)
	}
	if err := os.WriteFile(maxFile, []byte("8589934592\n"), 0644); err != nil {
		t.Fatalf("WriteFile max: %v", err)
	}
	got := hostSnapshot(tmpdir)
	if !strings.Contains(got, "mem 2143/8192MB") {
		t.Errorf("hostSnapshot = %q, want to contain 'mem 2143/8192MB'", got)
	}

	// Test unlimited memory (max file contains "max")
	if err := os.WriteFile(maxFile, []byte("max\n"), 0644); err != nil {
		t.Fatalf("WriteFile max=unlimited: %v", err)
	}
	got = hostSnapshot(tmpdir)
	if !strings.Contains(got, "mem 2143MB/unlimited") {
		t.Errorf("hostSnapshot = %q, want to contain 'mem 2143MB/unlimited'", got)
	}
}

func TestLogHostDuring(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	oldOutput := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() {
		log.SetOutput(oldOutput)
	})

	stop := make(chan struct{})
	done := make(chan struct{})

	// Run logHostDuring in background with 1ms interval (fast enough to produce samples)
	go func() {
		logHostDuring("render", t.TempDir(), 1*time.Millisecond, stop)
		close(done)
	}()

	// Let it tick a few times
	time.Sleep(10 * time.Millisecond)
	close(stop)

	// Wait for goroutine to stop with timeout
	select {
	case <-done:
		// Success, goroutine stopped
	case <-time.After(5 * time.Second):
		t.Fatal("logHostDuring did not stop after close(stop)")
	}

	// Check output contains at least one host sample
	output := buf.String()
	if !strings.Contains(output, "host render:") {
		t.Errorf("logHostDuring output = %q, want to contain 'host render:'", output)
	}
}

