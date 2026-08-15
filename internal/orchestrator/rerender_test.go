package orchestrator

import (
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/models"
)

func strp(s string) *string { return &s }

func TestRerenderBlockedReason(t *testing.T) {
	cases := []struct {
		name    string
		clip    *models.Clip
		blocked bool
	}{
		{
			name:    "คลิปที่ตะแกรงตีตก — สั่งได้",
			clip:    &models.Clip{Status: "needs_review", ProductionStage: "content_ready"},
			blocked: false,
		},
		{
			name:    "ready ที่ไฟล์หาย — สั่งได้",
			clip:    &models.Clip{Status: "ready", ProductionStage: "rendered"},
			blocked: false,
		},
		{
			name:    "มีไฟล์วิดีโออยู่แล้ว — ห้าม (เผาเงินซ้ำโดยไม่จำเป็น)",
			clip:    &models.Clip{Status: "needs_review", ProductionStage: "content_ready", Video916URL: strp("https://cdn/x.mp4")},
			blocked: true,
		},
		{
			name:    "กำลังผลิตอยู่ — ห้าม",
			clip:    &models.Clip{Status: "producing", ProductionStage: "content_ready"},
			blocked: true,
		},
		{
			name:    "เผยแพร่ไปแล้ว — ห้าม",
			clip:    &models.Clip{Status: "published", ProductionStage: "rendered"},
			blocked: true,
		},
		{
			name:    "ยังไม่มีฉากที่บันทึกไว้ — ห้าม (ไม่มีอะไรให้ resume)",
			clip:    &models.Clip{Status: "needs_review", ProductionStage: ""},
			blocked: true,
		},
		{
			name:    "ไม่มีคลิป",
			clip:    nil,
			blocked: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := rerenderBlockedReason(c.clip)
			if (got != "") != c.blocked {
				t.Errorf("rerenderBlockedReason() = %q, blocked=%v want %v", got, got != "", c.blocked)
			}
		})
	}
}

func TestRerenderBlockedReason_MessageNamesTheProblem(t *testing.T) {
	got := rerenderBlockedReason(&models.Clip{Status: "published", ProductionStage: "rendered"})
	if !strings.Contains(got, "published") {
		t.Errorf("ข้อความ %q ต้องบอกสถานะที่เป็นเหตุ", got)
	}
}
