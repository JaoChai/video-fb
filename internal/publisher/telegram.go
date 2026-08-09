package publisher

import (
	"context"
	"errors"
	"log"
	"strings"
)

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

// sendTelegram โพสต์ข้อความเข้าช่องแล้วบันทึก post id · ไม่คืน error โดยตั้งใจ: คลิปขึ้น
// YouTube สำเร็จไปแล้ว ความล้มเหลวตรงนี้ต้องไม่ไหลย้อนไปเปลี่ยนสถานะคลิปหรือเขียน fail_reason
func (p *Publisher) sendTelegram(ctx context.Context, clipID, accountID, title, videoURL string) {
	res, err := p.zernio.Post(ctx, PostRequest{
		Content:    telegramMessage(title, videoURL),
		Platforms:  telegramPlatforms(accountID),
		PublishNow: true,
		RequestID:  postRequestID(clipID, "telegram"),
	})

	postID := ""
	switch {
	case err == nil:
		postID = res.Post.ID
	default:
		// 409 = Zernio เห็นเนื้อหาซ้ำใน 24 ชม. แปลว่าข้อความนี้เคยส่งเข้าช่องไปแล้ว
		// บันทึก id เดิมเพื่อไม่ให้รอบเก็บตกวนมาลองใหม่ทุกชั่วโมง
		var dup *DuplicatePostError
		if errors.As(err, &dup) {
			postID = dup.ExistingPostID
			log.Printf("Telegram: คลิป %s ซ้ำกับโพสต์ %s ที่เคยส่งแล้ว บันทึกเป็นส่งแล้ว", clipID, postID)
		} else {
			log.Printf("Telegram: ส่งคลิป %s เข้าช่องไม่สำเร็จ: %v", clipID, err)
			return
		}
	}
	if postID == "" {
		log.Printf("Telegram: คลิป %s ไม่ได้ post id กลับมา ข้ามการบันทึก", clipID)
		return
	}

	// pool เป็น nil เฉพาะในเทสต์ที่ตรวจรูปคำขอ HTTP เท่านั้น — เส้นทางจริงมี pool เสมอ
	if p.pool == nil {
		return
	}
	if _, err := p.pool.Exec(ctx,
		`UPDATE clip_metadata SET zernio_telegram_post_id = $2 WHERE clip_id = $1`,
		clipID, postID); err != nil {
		// บันทึกพลาด = รอบเก็บตกจะลองใหม่ แล้ว x-request-id เดิมทำให้ Zernio คืนโพสต์เดิม
		// แทนการโพสต์ซ้ำ จึงแค่ log พอ
		log.Printf("Telegram: บันทึก telegram_post_id ของคลิป %s ไม่สำเร็จ: %v", clipID, err)
		return
	}
	log.Printf("Telegram: ส่งคลิป %s เข้าช่องแล้ว → %s", clipID, postID)
}
