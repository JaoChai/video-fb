package scheduler

import "testing"

// แถว schedule ที่ action ไม่มี handler จะ "รันสำเร็จ" ทุกรอบโดยไม่ทำอะไรเลย —
// เงียบสนิท ไม่มี error ไม่มีคลิป เทสต์นี้คือสิ่งเดียวที่จับได้ก่อน deploy
func TestMythActionHasHandler(t *testing.T) {
	s := &Scheduler{}
	if s.handlerFor("produce_myth") == nil {
		t.Error(`handlerFor("produce_myth") = nil — แถว schedule 09:00 จะไม่ทำอะไรเลย`)
	}
	for _, a := range []string{"produce_and_publish", "produce_evening", "produce_tutorial", "produce_basic"} {
		if s.handlerFor(a) == nil {
			t.Errorf("handlerFor(%q) = nil — ช่องเดิมพัง", a)
		}
	}
}
