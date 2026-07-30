package models

import "testing"

// ค่า verdict ทั้งสามต้องตรงกับ CHECK constraint ใน migration 075 เป๊ะ
// ถ้าไฟล์ใดไฟล์หนึ่งเปลี่ยนคำเดียว คลิปจะได้ Meter ว่างแล้วมิเตอร์หายไปเงียบๆ
func TestMythVerdictValues(t *testing.T) {
	for _, want := range []string{"false", "half_true", "outdated"} {
		if !ValidMythVerdict(want) {
			t.Errorf("ValidMythVerdict(%q) = false, ต้องเป็น true", want)
		}
	}
	if ValidMythVerdict("maybe") {
		t.Error(`ValidMythVerdict("maybe") = true, ต้องเป็น false`)
	}
}

// คลิปต้องจำได้ว่าตัวเองมาจากแถวคลังไหน ไม่งั้น retry เต็มรูปแบบจะโหลดคลังกลับไม่ได้
// แล้วตะแกรงข้อเท็จจริงจะปิดเงียบทั้งรอบ (บั๊กเดียวกับที่เคยเกิดกับคลิป basic)
func TestCreateClipRequestCarriesMythBelief(t *testing.T) {
	req := CreateClipRequest{MythBelief: "bm_stronger_than_personal"}
	if req.MythBelief != "bm_stronger_than_personal" {
		t.Errorf("MythBelief = %q", req.MythBelief)
	}
}
