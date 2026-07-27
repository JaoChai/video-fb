package publisher

import "github.com/jaochai/video-fb/internal/models"

// mergeRetention เลือกว่าจะบันทึกค่า retention ชุดไหนลงแถวใหม่ของ clip_analytics
//
// ทำไมต้องมี: Zernio คืน daily-views เป็นชุดว่างเมื่อคลิปไม่มีวิวในหน้าต่างที่เขาให้มา
// เดิม fetchYouTubeDetail แปลกรณีนั้นเป็น "retention = 0" แล้ว Create ก็ INSERT ทับ
// เป็นแถวใหม่สุด — ซึ่ง consumer ทุกตัวอ่านด้วย ORDER BY fetched_at DESC ค่าที่เคยวัดได้
// จริงจึงถูกบังหายไปถาวร วัดจาก prod 2026-07-27: โดนไป 25 คลิปจาก 115
//
// กติกา: รอบไหนวัดได้จริง (AvgViewPercentage > 0) ใช้ของรอบนั้นทั้งชุด รอบไหนไม่ได้
// ให้คงค่าที่เคยวัดได้ล่าสุดไว้ทั้งชุด ยกทีละชุดเสมอ ไม่หยิบผสมข้ามรอบ
//
// ผลข้างเคียงที่ยอมรับ: คลิปที่ตายสนิทแล้วจะค้างค่าที่วัดได้ครั้งสุดท้ายไว้ตลอดไป
// ซึ่งถูกต้องตามความหมาย — retention ของคลิปที่ไม่มีคนดูแล้วย่อมไม่เปลี่ยนอีก
func mergeRetention(fresh, prev models.RetentionSnapshot) models.RetentionSnapshot {
	if fresh.AvgViewPercentage > 0 {
		return fresh
	}
	return prev
}
