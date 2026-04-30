package producer

import (
	"testing"
)

func TestSplitVoiceText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxRunes int
		want     []string
	}{
		{
			name:     "short text stays as single chunk",
			text:     "สวัสดีครับ... ผมพูดเรื่องโฆษณา",
			maxRunes: 400,
			want:     []string{"สวัสดีครับ... ผมพูดเรื่องโฆษณา"},
		},
		{
			name:     "empty text returns empty",
			text:     "",
			maxRunes: 400,
			want:     nil,
		},
		{
			name:     "no separator keeps as single chunk",
			text:     "ข้อความยาวมากแต่ไม่มีจุดพัก",
			maxRunes: 400,
			want:     []string{"ข้อความยาวมากแต่ไม่มีจุดพัก"},
		},
		{
			name:     "splits at separator when exceeding max",
			text:     "ส่วนแรกของข้อความที่ยาว... ส่วนที่สองของข้อความ... ส่วนที่สามยาวมาก",
			maxRunes: 30,
			want: []string{
				"ส่วนแรกของข้อความที่ยาว",
				"ส่วนที่สองของข้อความ",
				"ส่วนที่สามยาวมาก",
			},
		},
		{
			name:     "real voice script splits into 2 chunks",
			text:     "คุณต้นถามมาว่า... เช็คแล้วทำไมข้อมูลไม่ขึ้น... เรื่องนี้เจอบ่อย... สาเหตุหลักมีสองจุด... วิธีแก้มีสามขั้น... ขั้นแรก... โหลดส่วนเสริมเข้าเบราว์เซอร์... ขั้นสอง... รอประมาณยี่สิบสี่ชั่วโมง... ขั้นสาม... เข้าไปกดเทสต์อีเวนต์... ติดต่อทีมงานได้เลยครับ",
			maxRunes: 150,
			want: []string{
				"คุณต้นถามมาว่า... เช็คแล้วทำไมข้อมูลไม่ขึ้น... เรื่องนี้เจอบ่อย... สาเหตุหลักมีสองจุด... วิธีแก้มีสามขั้น... ขั้นแรก... โหลดส่วนเสริมเข้าเบราว์เซอร์",
				"ขั้นสอง... รอประมาณยี่สิบสี่ชั่วโมง... ขั้นสาม... เข้าไปกดเทสต์อีเวนต์... ติดต่อทีมงานได้เลยครับ",
			},
		},
		{
			name:     "single segment over max stays as one chunk",
			text:     "ข้อความยาวมากที่ไม่มีจุดพักเลยจนเกิน limit",
			maxRunes: 10,
			want:     []string{"ข้อความยาวมากที่ไม่มีจุดพักเลยจนเกิน limit"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := splitVoiceText(tc.text, tc.maxRunes)
			if len(got) != len(tc.want) {
				t.Fatalf("splitVoiceText() returned %d chunks, want %d\n  got:  %v\n  want: %v", len(got), len(tc.want), got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("chunk[%d]\n  got:  %q\n  want: %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}
