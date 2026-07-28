package scheduler

import "testing"

// schedule row ของ 18:00 ชี้ไป action ใหม่ ถ้า handlerFor ไม่รู้จัก แถวนั้นจะเงียบ
// ไม่ทำอะไรเลย และไม่มีคลิปเย็นทั้งวันโดยไม่มี error ให้เห็น
func TestEveningActionHasHandler(t *testing.T) {
	s := &Scheduler{}
	if s.handlerFor("produce_evening") == nil {
		t.Error(`handlerFor("produce_evening") = nil — the 18:00 schedule row would silently do nothing`)
	}
	if s.handlerFor("produce_and_publish") == nil {
		t.Error(`handlerFor("produce_and_publish") = nil — the 12:00 row would break`)
	}
}

// ชุด format ต่อ slot เป็นตัวเลือกโหมดหน้าตา สองชุดนี้ห้ามทับกันเด็ดขาด
// ไม่งั้นคลิปเที่ยงกับคลิปเย็นจะหน้าตาเหมือนกันในบางวันโดยไม่มีใครรู้
func TestSlotFormatsDoNotOverlap(t *testing.T) {
	seen := map[string]bool{}
	for _, f := range noonFormats {
		seen[f] = true
	}
	for _, f := range eveningFormats {
		if seen[f] {
			t.Errorf("format %q อยู่ทั้งสอง slot — คลิปเที่ยงกับเย็นจะได้โหมดเดียวกัน", f)
		}
	}
	if len(noonFormats) == 0 || len(eveningFormats) == 0 {
		t.Error("ทั้งสอง slot ต้องมีอย่างน้อย 1 format ไม่งั้น PickNextIn จะ error ทุกครั้ง")
	}
}
