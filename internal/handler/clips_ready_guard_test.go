package handler

import (
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/models"
)

func strp(s string) *string { return &s }

// คลิปที่ยังไม่มีไฟล์วิดีโอต้องเข้าสถานะ ready ไม่ได้ — รอบส่งจะหยิบมันไปวนซ้ำโดยส่งอะไร
// ไม่ได้เลย (เหตุ 2026-08-14 คิวตัน 21 ชั่วโมง)
func TestReadyBlockedReason(t *testing.T) {
	cases := []struct {
		name    string
		clip    *models.Clip
		blocked bool
	}{
		{"ไม่มีไฟล์เลย", &models.Clip{}, true},
		{"url ว่าง", &models.Clip{Video916URL: strp("")}, true},
		{"มี 9:16", &models.Clip{Video916URL: strp("https://cdn/x.mp4")}, false},
		{"มี 16:9", &models.Clip{Video169URL: strp("https://cdn/x.mp4")}, false},
		{"คลิปหาย (nil) ปล่อยให้ชั้นบนจัดการ", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := readyBlockedReason(c.clip)
			if (got != "") != c.blocked {
				t.Errorf("readyBlockedReason() = %q, blocked=%v want %v", got, got != "", c.blocked)
			}
			if c.blocked && !strings.Contains(got, "เรนเดอร์") {
				t.Errorf("ข้อความ %q ต้องบอกทางออก (สั่งเรนเดอร์)", got)
			}
		})
	}
}
