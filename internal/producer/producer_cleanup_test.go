package producer

import (
	"os"
	"path/filepath"
	"testing"
)

// newCleanupProducer สร้าง Producer ที่มีแค่ workDir — CleanupClipDir ไม่แตะ
// dependency อื่นเลย จึงส่ง nil ได้ทั้งหมด
func newCleanupProducer(t *testing.T) (*Producer, string) {
	t.Helper()
	work := t.TempDir()
	return NewProducer(nil, nil, nil, nil, nil, "", work, nil), work
}

func TestCleanupClipDirRemovesClipFiles(t *testing.T) {
	p, work := newCleanupProducer(t)
	clipDir := filepath.Join(work, "clip-1")
	if err := os.MkdirAll(filepath.Join(clipDir, "composition-916"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(clipDir, "voice.wav"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := p.CleanupClipDir("clip-1"); err != nil {
		t.Fatalf("CleanupClipDir: %v", err)
	}
	if _, err := os.Stat(clipDir); !os.IsNotExist(err) {
		t.Errorf("โฟลเดอร์คลิปต้องหายไป แต่ stat ได้ err=%v", err)
	}
}

func TestCleanupClipDirRefusesEmptyID(t *testing.T) {
	p, work := newCleanupProducer(t)
	other := filepath.Join(work, "clip-2")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := p.CleanupClipDir("  "); err == nil {
		t.Error("clipID ว่างต้องคืน error ไม่ใช่ลบ workDir ทั้งก้อน")
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("workDir ต้องไม่ถูกแตะเมื่อ clipID ว่าง: %v", err)
	}
}
