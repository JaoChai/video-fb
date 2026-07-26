package producer

import (
	"strings"
	"testing"
)

// วิธีอ่านเทสต์ชุดนี้: `⟦` แทน zero-width space เพื่อให้เห็นตำแหน่งที่คาดหวัง
func vis(s string) string { return strings.ReplaceAll(s, zwsp, "⟦") }

func TestGuardLoanWords_WrapsKnownWords(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{
			name: "คำทับศัพท์กลางประโยค",
			in:   "เจ้าของเก่าเคลมคืนได้ทั้งที่คุณคุมแอดมินอยู่",
			want: "เจ้าของเก่าเคลมคืนได้ทั้งที่คุณคุม" + zwsp + "แอดมิน" + zwsp + "อยู่",
		},
		{
			name: "คำประกอบต้องชนะคำย่อย",
			in:   "ทักไลน์ไอดีแอดส์แวนซ์ได้เลย",
			want: "ทักไลน์" + zwsp + "ไอดีแอดส์แวนซ์" + zwsp + "ได้เลย",
		},
		{
			// คำในรายการอยู่ติดกันสนิท — zwsp ประกบซ้อนกันสองตัวได้ ซึ่งไม่มีผลต่อ
			// การแสดงผล (วัดกับ Chromium แล้ว: ความกว้าง/สูงของกล่องเท่าเดิมเป๊ะ)
			name: "คำในรายการติดกัน",
			in:   "ทักไอดีเฟซบุ๊กได้เลย",
			want: "ทัก" + zwsp + "ไอดี" + zwsp + zwsp + "เฟซบุ๊ก" + zwsp + "ได้เลย",
		},
		{
			// คำในรายการเป็นอักษรไทยล้วน จึงไม่มีทางตรงกับ syntax ของแท็กซึ่งเป็น
			// ASCII — เทสต์นี้ปักตำแหน่ง zwsp ให้เห็นว่ามันตกนอกแท็กเสมอ
			name: "ไม่แตะแท็ก HTML ที่ highlightTitleStr ใส่ไว้",
			in:   `เปิด<span class="acc">แอดมิน</span>ให้ครบ`,
			want: `เปิด<span class="acc">` + zwsp + "แอดมิน" + zwsp + `</span>ให้ครบ`,
		},
		{
			name: "ไม่มีคำในรายการ = คืนค่าเดิม",
			in:   "เปลี่ยนโดเมนแล้วอัดงบต่อทันที",
			want: "เปลี่ยนโดเมนแล้วอัดงบต่อทันที",
		},
		{
			name: "สตริงว่าง",
			in:   "",
			want: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := GuardLoanWords(c.in)
			if got != c.want {
				t.Errorf("GuardLoanWords(%q)\n got: %s\nwant: %s", c.in, vis(got), vis(c.want))
			}
			// zwsp เป็นอักขระจัดบรรทัด ไม่ใช่เนื้อความ — ถอดออกแล้วต้องได้ต้นฉบับเป๊ะ
			if stripped := strings.ReplaceAll(got, zwsp, ""); stripped != c.in {
				t.Errorf("ข้อความเพี้ยนหลังถอด zwsp\n got: %q\nwant: %q", stripped, c.in)
			}
		})
	}
}

// เรียกซ้ำต้องได้ผลเดิม — จุดเรียกทั้งสองแห่ง (แคปชั่นกับข้อความบนจอ) อาจทับกันได้
func TestGuardLoanWords_Idempotent(t *testing.T) {
	for _, in := range []string{
		"ทีมงานเทเลแกรมตอบเร็วมากครับ",
		"ทักไอดีเฟซบุ๊กได้เลย", // คำติดกัน: zwsp ซ้อนก็ต้องไม่งอกเพิ่มเมื่อเรียกซ้ำ
	} {
		once := GuardLoanWords(in)
		twice := GuardLoanWords(once)
		if once != twice {
			t.Errorf("เรียกซ้ำแล้วผลเปลี่ยน (%q)\n1 ครั้ง: %s\n2 ครั้ง: %s", in, vis(once), vis(twice))
		}
	}
}

// รายการต้องเรียงจากยาวไปสั้นเสมอ เพราะ strings.NewReplacer เทียบตามลำดับ argument
// ไม่งั้น "แอดส์แวนซ์" จะไปชนะ "ไอดีแอดส์แวนซ์" เทสต์นี้กันคนเพิ่มคำใหม่ต่อท้ายรายการ
func TestThaiLoanWords_SortedLongestFirst(t *testing.T) {
	for i := 1; i < len(thaiLoanWords); i++ {
		prev := len([]rune(thaiLoanWords[i-1]))
		cur := len([]rune(thaiLoanWords[i]))
		if cur > prev {
			t.Errorf("รายการเรียงผิดที่ตำแหน่ง %d: %q (%d ตัว) ยาวกว่า %q (%d ตัว) ที่อยู่ก่อนหน้า",
				i, thaiLoanWords[i], cur, thaiLoanWords[i-1], prev)
		}
	}
}
