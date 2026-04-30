package producer

import (
	"encoding/binary"
	"testing"
)

func makePCM(samples []int16) []byte {
	buf := make([]byte, len(samples)*2)
	for i, s := range samples {
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(s))
	}
	return buf
}

func TestTrimPCMSilence(t *testing.T) {
	loud := int16(5000)
	silent := int16(0)

	tests := []struct {
		name         string
		samples      []int16
		trimLeading  bool
		trimTrailing bool
		wantSamples  int
	}{
		{
			name:         "trim leading silence",
			samples:      []int16{silent, silent, silent, loud, loud, loud},
			trimLeading:  true,
			trimTrailing: false,
			wantSamples:  3,
		},
		{
			name:         "trim trailing silence",
			samples:      []int16{loud, loud, loud, silent, silent, silent},
			trimLeading:  false,
			trimTrailing: true,
			wantSamples:  3,
		},
		{
			name:         "trim both",
			samples:      []int16{silent, silent, loud, loud, silent, silent},
			trimLeading:  true,
			trimTrailing: true,
			wantSamples:  2,
		},
		{
			name:         "no trim",
			samples:      []int16{silent, loud, loud, silent},
			trimLeading:  false,
			trimTrailing: false,
			wantSamples:  4,
		},
		{
			name:         "all silence trims to minimum",
			samples:      []int16{silent, silent, silent},
			trimLeading:  true,
			trimTrailing: true,
			wantSamples:  0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pcm := makePCM(tc.samples)
			got := trimPCMSilence(pcm, tc.trimLeading, tc.trimTrailing)
			gotSamples := len(got) / 2
			if gotSamples != tc.wantSamples {
				t.Errorf("trimPCMSilence() returned %d samples, want %d", gotSamples, tc.wantSamples)
			}
		})
	}
}

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
