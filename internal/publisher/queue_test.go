package publisher

import (
	"strings"
	"testing"
)

// หัวใจของเหตุ 2026-08-14: คลิปที่ส่งไม่ออกต้องไม่ขวางใบถัดไป
func TestPublishFirst_SkipsFailingCandidateAndPublishesNext(t *testing.T) {
	cands := []publishCandidate{{ClipID: "เสีย"}, {ClipID: "ดี"}, {ClipID: "ไม่ควรถูกแตะ"}}
	var tried []string
	got := publishFirst(cands, func(c publishCandidate) bool {
		tried = append(tried, c.ClipID)
		return c.ClipID == "ดี"
	})
	if got != "ดี" {
		t.Errorf("publishFirst() = %q, want %q", got, "ดี")
	}
	want := []string{"เสีย", "ดี"}
	if strings.Join(tried, ",") != strings.Join(want, ",") {
		t.Errorf("ลองส่ง %v, want %v (ต้องหยุดทันทีที่สำเร็จ)", tried, want)
	}
}

func TestPublishFirst_NoCandidateSucceeds(t *testing.T) {
	cands := []publishCandidate{{ClipID: "a"}, {ClipID: "b"}}
	calls := 0
	got := publishFirst(cands, func(publishCandidate) bool { calls++; return false })
	if got != "" {
		t.Errorf("publishFirst() = %q, want \"\"", got)
	}
	if calls != 2 {
		t.Errorf("เรียก send %d ครั้ง, want 2 (ต้องลองครบทุกใบก่อนยอมแพ้)", calls)
	}
}

func TestPublishFirst_EmptyList(t *testing.T) {
	if got := publishFirst(nil, func(publishCandidate) bool { return true }); got != "" {
		t.Errorf("publishFirst(nil) = %q, want \"\"", got)
	}
}

// ล็อกเจตนาของ SQL ไว้ด้วยการอ่านสตริง เพราะโปรเจกต์นี้ไม่มี integration test ต่อ Postgres
// สามอย่างนี้คือสิ่งที่ถ้าหลุดไปแล้วคิวจะกลับไปตันแบบเดิมโดยไม่มีเทสต์ไหนร้อง
func TestReadyClipsQuery_KeepsAntiBlockingGuards(t *testing.T) {
	for _, want := range []string{
		"starts_with(c.fail_reason",            // ดันคลิปที่ส่งพลาดไปท้ายคิว
		"COALESCE(c.video_9_16_url, '') <> ''", // คลิปไร้ไฟล์วิดีโอไม่เข้าคิวตั้งแต่ต้น
		"LIMIT $2",                             // ผู้สมัครหลายใบ ไม่ใช่ LIMIT 1
	} {
		if !strings.Contains(readyClipsQuery, want) {
			t.Errorf("readyClipsQuery ขาด %q", want)
		}
	}
	if strings.Contains(readyClipsQuery, "LIMIT 1") {
		t.Error("readyClipsQuery ยังเป็น LIMIT 1 — คลิปใบเดียวจะยึดหัวคิวได้อีก")
	}
}

func TestPublishCandidate_HasVideo(t *testing.T) {
	cases := []struct {
		name string
		c    publishCandidate
		want bool
	}{
		{"ไม่มีทั้งคู่", publishCandidate{}, false},
		{"มี 9:16", publishCandidate{Video916: "https://cdn/x.mp4"}, true},
		{"มี 16:9", publishCandidate{Video169: "https://cdn/x.mp4"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.c.hasVideo(); got != c.want {
				t.Errorf("hasVideo() = %v, want %v", got, c.want)
			}
		})
	}
}
