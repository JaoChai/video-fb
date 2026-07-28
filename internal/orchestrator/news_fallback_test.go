package orchestrator

import "testing"

// เส้นทางนี้ทำงานก็ต่อเมื่อ agent หาข่าวสดไม่เจอ ซึ่งเกิดไม่บ่อยพอที่จะเห็นบั๊กจากการใช้จริง
// แต่เมื่อเกิดแล้วเลือกผิด คลิปทั้งรอบจะ render ผิดโหมด — สเปกจึงระบุให้ fallback
// ห้ามออกนอกชุดของ slot เด็ดขาด
func TestRemainingWithoutNews(t *testing.T) {
	t.Run("ตัด news ออกแต่คงตัวอื่นของ slot", func(t *testing.T) {
		got, err := remainingWithoutNews([]string{"case_story", "news"})
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if len(got) != 1 || got[0] != "case_story" {
			t.Errorf("got %v, want [case_story]", got)
		}
	})

	t.Run("slot ที่มีแต่ news ต้อง error ไม่ใช่ยืม format ของ slot อื่น", func(t *testing.T) {
		got, err := remainingWithoutNews([]string{"news"})
		if err == nil {
			t.Fatalf("err = nil, want error — got %v (คลิปจะ render ผิดโหมด)", got)
		}
	})

	t.Run("manual produce (ไม่ล็อก slot) ต้องไม่ถูกจำกัด", func(t *testing.T) {
		got, err := remainingWithoutNews(nil)
		if err != nil {
			t.Fatalf("err = %v, want nil — manual /produce ต้องยังทำงานได้", err)
		}
		if len(got) != 0 {
			t.Errorf("got %v, want empty (= ไม่จำกัด)", got)
		}
	})

	t.Run("ชุดที่ไม่มี news อยู่แล้วต้องผ่านครบ", func(t *testing.T) {
		got, err := remainingWithoutNews([]string{"qa", "tips"})
		if err != nil || len(got) != 2 {
			t.Errorf("got %v, err %v — want [qa tips], nil", got, err)
		}
	})
}
