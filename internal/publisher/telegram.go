package publisher

import (
	"context"
	"errors"
	"log"
	"strings"

	"github.com/jackc/pgx/v5"
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

// postTelegramForClip หาลิงก์คลิปจริงจากโพสต์ที่เพิ่งขึ้น แล้วส่งเข้าช่อง · เรียกครั้งเดียว
// ต่อคลิปแม้จะมีทั้ง 16:9 และ 9:16 — ช่องไม่ควรได้ลิงก์คลิปเดียวกันสองใบ
func (p *Publisher) postTelegramForClip(ctx context.Context, clipID, accountID, title, mainPostID, shortsPostID string) {
	if accountID == "" {
		return // ฟีเจอร์ปิด: ไม่ยิงอะไรออกไปเลย
	}
	// 16:9 ก่อนเสมอ เพราะเป็นหน้าคลิปเต็มที่ดูได้ทุกอุปกรณ์ · ตอนนี้ pipeline ผลิต 9:16
	// อย่างเดียว ทางนี้จึงตกไป Shorts เป็นปกติ
	postID := mainPostID
	if postID == "" {
		postID = shortsPostID
	}
	if postID == "" {
		return
	}

	ps, err := p.zernio.GetPost(ctx, postID)
	if err != nil {
		log.Printf("Telegram: อ่านโพสต์ %s ของคลิป %s ไม่สำเร็จ: %v", postID, clipID, err)
		return
	}
	videoURL := youtubePostURL(ps)
	if videoURL == "" {
		log.Printf("Telegram: โพสต์ %s ของคลิป %s ยังไม่มีลิงก์ YouTube — ปล่อยให้รอบเก็บตกจัดการ", postID, clipID)
		return
	}
	p.sendTelegram(ctx, clipID, accountID, title, videoURL)
}

// sweepTelegramBacklog เก็บตกคลิปที่ขึ้น YouTube แล้วแต่ยังไม่ได้เข้าช่อง (เช่นรอบที่ Zernio
// ล่มพอดี หรือรอบที่ยังไม่มี platformPostUrl) · ทีละ 1 คลิปต่อรอบเหมือน PublishTikTok
//
// กรอบ 24 ชั่วโมงคือสิ่งเดียวที่กันไม่ให้คลังคลิปเก่าทั้งหมดถูกยิงเข้าช่องรวดเดียว ห้ามถอด
// (migration 081 ปั๊ม 'skipped-backfill' ให้คลิปที่ published อยู่ก่อนแล้ว จึงไม่มีคลิปเก่า
// ค้างในกรอบนี้ตอน deploy ครั้งแรก)
func (p *Publisher) sweepTelegramBacklog(ctx context.Context, accountID string) {
	if accountID == "" {
		return
	}
	var clipID, title string
	var mainPostID, shortsPostID *string
	// updated_at คือเวลาที่แถวถูกแตะล่าสุด ไม่ใช่เวลา publish เป๊ะ — คลิปที่ยังส่งไม่สำเร็จ
	// จะอยู่ในกรอบนี้ต่อไปทุกครั้งที่มีอะไรมาอัปเดตแถว (นี่คือพฤติกรรม retry ที่ตั้งใจ)
	// ส่วนคลิปที่ส่งสำเร็จแล้วหลุดคิวทันทีเพราะ zernio_telegram_post_id ถูกตั้งค่า
	err := p.pool.QueryRow(ctx, `
		SELECT c.id, cm.youtube_title, cm.zernio_post_id, cm.zernio_shorts_post_id
		FROM clips c
		JOIN clip_metadata cm ON cm.clip_id = c.id
		WHERE c.status = 'published'
		  AND c.updated_at > NOW() - INTERVAL '24 hours'
		  AND (cm.zernio_telegram_post_id IS NULL OR cm.zernio_telegram_post_id = '')
		  AND (COALESCE(cm.zernio_post_id, '') <> '' OR COALESCE(cm.zernio_shorts_post_id, '') <> '')
		ORDER BY c.updated_at ASC LIMIT 1`).
		Scan(&clipID, &title, &mainPostID, &shortsPostID)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			log.Printf("Telegram: หาคลิปเก็บตกไม่สำเร็จ: %v", err)
		}
		return
	}

	main, shorts := "", ""
	if mainPostID != nil {
		main = *mainPostID
	}
	if shortsPostID != nil {
		shorts = *shortsPostID
	}
	log.Printf("Telegram: เก็บตกคลิป %s ที่ยังไม่ได้เข้าช่อง", clipID)
	p.postTelegramForClip(ctx, clipID, accountID, sanitizeYouTubeText(title), main, shorts)
}
