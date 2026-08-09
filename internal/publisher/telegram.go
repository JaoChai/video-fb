package publisher

import "strings"

const platformTelegram = "telegram"

// Zernio ส่งข้อความ Telegram ด้วย parseMode "HTML" เป็นค่า default ⇒ ตัวอักษรสามตัวนี้ใน
// ชื่อคลิปจะถูกตีความเป็นแท็ก ต้องแปลงก่อนเสมอ · ต้องแปลง & ก่อนตัวอื่นเสมอ ไม่งั้นจะไป
// แปลง & ที่ตัวเองเพิ่งสร้างซ้ำ (strings.NewReplacer สแกนรอบเดียวจึงปลอดภัยอยู่แล้ว)
var telegramHTMLEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// telegramMessage ประกอบข้อความที่จะขึ้นช่อง: พาดหัวบรรทัดแรก เว้นบรรทัด แล้วลิงก์เปล่า
// ปล่อยลิงก์เปล่าโดยตั้งใจ เพราะ Telegram จะดึงการ์ดพรีวิว (ภาพปก + ชื่อคลิป + ชื่อช่อง)
// จาก YouTube มาแสดงให้เอง — ใส่ <a href> ครอบแล้วการ์ดจะหาย
func telegramMessage(title, videoURL string) string {
	return telegramHTMLEscaper.Replace(stripBrandTag(title)) + "\n\n" + videoURL
}

// telegramPlatforms ประกอบเป้าหมายโพสต์ฝั่ง Telegram · ไม่แนบ platformSpecificData เพราะ
// ค่า default ของ Zernio (parseMode HTML) คือสิ่งที่เราต้องการอยู่แล้ว และฟิลด์นั้นผูก type
// ไว้กับ YouTubeOptions — เลี่ยงการแก้เส้นทาง YouTube เพื่อฟีเจอร์ที่ไม่ต้องใช้
func telegramPlatforms(accountID string) []PlatformTarget {
	return []PlatformTarget{{Platform: platformTelegram, AccountID: accountID}}
}

// youtubePostURL ดึงลิงก์คลิปจริงจากผลของ GetPost · คืนค่าว่างเมื่อยังไม่มี แปลว่า "ยังไม่พร้อม
// ส่ง" ไม่ใช่ "ประกอบ URL เองจาก video id" — เดา URL ผิดรูป (watch vs shorts) แล้วช่องจะได้
// ลิงก์เสียโดยไม่มีใครรู้
func youtubePostURL(ps *PostStatus) string {
	for _, pl := range ps.Platforms {
		if pl.Platform == platformYouTube && pl.PlatformPostURL != "" {
			return pl.PlatformPostURL
		}
	}
	return ""
}
