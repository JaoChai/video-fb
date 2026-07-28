package producer

import "testing"

// kie เปลี่ยนมาส่งยอดเครดิตเป็นทศนิยม ({"data":14070.59}) แต่ struct รับเป็น int
// json.Unmarshal จึงล้ม → GetCredits คืน error → ตัวกันผลิตตอนเครดิตหมด "ข้าม"
// ทุกครั้งเพราะมันตั้งใจ fail-open เมื่อเช็คไม่ได้ ผลคือด่านนี้ตายเงียบมาตลอด
// (เห็นใน log prod 2026-07-28: "kie credit pre-check skipped (non-fatal)")
func TestParseCreditResponseAcceptsFloatBalance(t *testing.T) {
	got, err := parseCreditResponse([]byte(`{"code":200,"msg":"success","data":14070.59}`))
	if err != nil {
		t.Fatalf("float balance must parse, got error: %v", err)
	}
	if got != 14070 {
		t.Errorf("credits = %d, want 14070 (ปัดลง — เศษเครดิตผลิตคลิปไม่ได้)", got)
	}
}

// payload แบบเดิมที่เป็นจำนวนเต็มต้องยังอ่านได้ ไม่งั้นแก้บั๊กหนึ่งไปสร้างอีกบั๊ก
func TestParseCreditResponseStillAcceptsIntBalance(t *testing.T) {
	got, err := parseCreditResponse([]byte(`{"code":200,"msg":"success","data":14070}`))
	if err != nil {
		t.Fatalf("integer balance must still parse, got error: %v", err)
	}
	if got != 14070 {
		t.Errorf("credits = %d, want 14070", got)
	}
}

// ยอดที่เหลือไม่ถึง 1 เครดิตต้องอ่านได้เป็น 0 ไม่ใช่ error — ผู้เรียกจะได้บล็อก
// การผลิตด้วยเหตุผลที่ถูก แทนที่จะข้ามด่านเพราะคิดว่าเช็คพัง
func TestParseCreditResponseFloorsNearlyEmpty(t *testing.T) {
	got, err := parseCreditResponse([]byte(`{"code":200,"msg":"success","data":0.6}`))
	if err != nil {
		t.Fatalf("near-empty balance must parse, got error: %v", err)
	}
	if got != 0 {
		t.Errorf("credits = %d, want 0 — เศษเครดิตต้องนับเป็นหมด", got)
	}
}

func TestParseCreditResponseRejectsNonOKCode(t *testing.T) {
	if _, err := parseCreditResponse([]byte(`{"code":401,"msg":"unauthorized","data":0}`)); err == nil {
		t.Error("a non-200 code must be an error, not a zero balance that blocks production for the wrong reason")
	}
}

func TestParseCreditResponseRejectsGarbage(t *testing.T) {
	if _, err := parseCreditResponse([]byte(`<html>gateway timeout</html>`)); err == nil {
		t.Error("unparseable body must be an error so the caller fails open on a broken check")
	}
}
