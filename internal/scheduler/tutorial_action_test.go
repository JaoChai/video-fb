package scheduler

import "testing"

// handlerFor ต้องรู้จัก action ใหม่ ไม่งั้น schedule row จะเงียบไม่ทำอะไรเลย
// ตรวจเฉพาะ 2 เคสที่ไม่ต้อง dereference field ใดๆ ของ Scheduler (เลี่ยง nil panic
// จาก method value ของ field ที่ยังไม่ถูก wire ในเทส)
func TestHandlerForProduceTutorial(t *testing.T) {
	s := &Scheduler{}
	if s.handlerFor("produce_tutorial") == nil {
		t.Error(`handlerFor("produce_tutorial") = nil — the schedule row would silently do nothing`)
	}
	if s.handlerFor("nonsense") != nil {
		t.Error("unknown actions must return nil")
	}
}
