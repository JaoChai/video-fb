package scheduler

import "testing"

// handlerFor ต้องรู้จัก action "tune_weights" ไม่งั้น schedule row ที่ migration
// 066 สร้างไว้จะเงียบไม่ทำอะไรเลย (log "unknown action" แล้วข้าม) และฟีเจอร์กระดาน
// คะแนนก็ตายสนิทโดยไม่มี error ที่ไหนบอกเลย
func TestHandlerForTuneWeights(t *testing.T) {
	s := &Scheduler{}
	if s.handlerFor("tune_weights") == nil {
		t.Error(`handlerFor("tune_weights") = nil — the schedule row would silently do nothing`)
	}
	if s.handlerFor("nonsense") != nil {
		t.Error("unknown actions must return nil")
	}
}
