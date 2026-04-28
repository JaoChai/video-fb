package orchestrator

import "testing"

func TestSanitizeVoiceText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "replace @adsvance with Thai phonetic",
			in:   "ติดต่อ @adsvance ได้เลย",
			want: "ติดต่อ แอดส์แวนซ์ ได้เลย",
		},
		{
			name: "replace AdsVance brand mention",
			in:   "ทีมงาน AdsVance พร้อมช่วย",
			want: "ทีมงาน แอดส์แวนซ์ พร้อมช่วย",
		},
		{
			name: "strip telegram URL",
			in:   "เข้ากลุ่มที่ https://t.me/adsvancech นะครับ",
			want: "เข้ากลุ่มที่ นะครับ",
		},
		{
			name: "strip http URL",
			in:   "ดูที่ http://example.com/path เลย",
			want: "ดูที่ เลย",
		},
		{
			name: "strip unknown @handle",
			in:   "ติดต่อ @someone อีกที",
			want: "ติดต่อ อีกที",
		},
		{
			name: "collapse whitespace after stripping",
			in:   "ก่อน    @adsvance     หลัง",
			want: "ก่อน แอดส์แวนซ์ หลัง",
		},
		{
			name: "preserve clean Thai text",
			in:   "สวัสดีครับ ผมพูดเรื่องโฆษณา Facebook",
			want: "สวัสดีครับ ผมพูดเรื่องโฆษณา Facebook",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeVoiceText(tc.in)
			if got != tc.want {
				t.Errorf("sanitizeVoiceText(%q)\n  got:  %q\n  want: %q", tc.in, got, tc.want)
			}
		})
	}
}
