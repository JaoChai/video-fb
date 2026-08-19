package scheduler

import (
	"context"
	"errors"
	"testing"
)

func TestDispatchUnknownAction(t *testing.T) {
	s := &Scheduler{}
	err := s.Dispatch(context.Background(), "does_not_exist")
	if !errors.Is(err, ErrUnknownAction) {
		t.Errorf("Dispatch(unknown) = %v, want ErrUnknownAction", err)
	}
}

func TestDispatchKnownActionResolves(t *testing.T) {
	// ไม่เรียกจริง (handler ต้องการ pool/orchestrator ที่ยังไม่ได้ inject) —
	// แค่ยืนยันว่า Dispatch หา handler เจอและไม่คืน ErrUnknownAction
	// ก่อนจะ panic ตอนเรียก h(ctx) ด้วย receiver ว่าง จึงต้องดัก panic แทน
	s := &Scheduler{}
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic from calling handler on zero-value Scheduler (nil orchestrator), got none — Dispatch may not be routing to the real handler")
		}
	}()
	_ = s.Dispatch(context.Background(), "retry_failed")
}
