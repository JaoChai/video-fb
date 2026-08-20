package producer

import (
	"strings"
	"testing"
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
