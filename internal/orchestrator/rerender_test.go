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

func TestGateFailReason(t *testing.T) {
	got := gateFailReason("tutorial", `ui_vocab violation: scene 2 breadcrumb: "A" not in ui_vocab`)
	if !strings.HasPrefix(got, "tutorial gate: ") {
		t.Errorf("gateFailReason() = %q, ต้องขึ้นต้นด้วยชื่อตะแกรง", got)
	}
	if !strings.Contains(got, "scene 2") {
		t.Errorf("gateFailReason() = %q, ต้องพกรายละเอียดเดิมไว้ครบ", got)
	}
	// ห้ามชนกับ prefix ของรอบส่ง ไม่งั้นเหตุผลของตะแกรงจะถูก clearPublishFailure ล้างทิ้ง
	if strings.HasPrefix(got, "publish: ") {
		t.Error("เหตุผลของตะแกรงต้องไม่ขึ้นต้นด้วย publish: — รอบส่งจะล้างทิ้ง")
	}
}
