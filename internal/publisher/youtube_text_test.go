package publisher

import "testing"

func TestSanitizeYouTubeText(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{
			// เคสจริงที่ทำให้คลิป e6973467 ค้าง ready ทั้งวัน (2026-08-02)
			name: "เส้นทางเมนูในคลิปสอน",
			in:   "มีครบ 4 ขั้น: เข้า Columns > Customise columns > Save as preset",
			want: "มีครบ 4 ขั้น: เข้า Columns › Customise columns › Save as preset",
		},
		{
			name: "เครื่องหมายน้อยกว่า",
			in:   "คุม CPM < 100 บาท",
			want: "คุม CPM ‹ 100 บาท",
		},
		{
			name: "แท็ก HTML",
			in:   "<b>เด่น</b>",
			want: "‹b›เด่น‹/b›",
		},
		{
			name: "ข้อความปกติไม่ถูกแตะ",
			in:   "ตั้งคอลัมน์ Frequency คู่ CPM | Ads Vance",
			want: "ตั้งคอลัมน์ Frequency คู่ CPM | Ads Vance",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sanitizeYouTubeText(c.in); got != c.want {
				t.Errorf("sanitizeYouTubeText(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
