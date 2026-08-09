package publisher

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
)

const platformYouTube = "youtube"
const platformTikTok = "tiktok"

// publishFailPrefix นำหน้าเหตุผลทุกข้อความที่รอบส่งเขียนลง fail_reason — เป็นตัวเดียวกับที่
// ใช้ล้างตอนส่งสำเร็จ จึงล้างได้เฉพาะข้อความของรอบส่ง ไม่แตะเหตุผลจากด่านอื่น
const publishFailPrefix = "publish: "

func isContactInfo(title string) bool {
	lower := strings.ToLower(title)
	return strings.Contains(lower, "line id") ||
		strings.Contains(lower, "@adsvance") ||
		strings.Contains(lower, "ติดต่อทีมงาน") ||
		strings.Contains(lower, "t.me/") ||
		strings.Contains(lower, "https://")
}

// buildTikTokCaption makes a TikTok-native caption: a short hook line plus
// hashtags. It deliberately drops the YouTube SEO description, which is
// templated to end with off-platform contact links (LINE id + Telegram URL) —
// TikTok hard-suppresses reach on posts that drive users off-platform.
func buildTikTokCaption(title, hashtags string) string {
	hook := stripBrandTag(title)
	if hashtags = strings.TrimSpace(hashtags); hashtags != "" {
		return hook + "\n\n" + hashtags
	}
	return hook
}

// stripBrandTag ตัดแบรนด์แท็ก "| Ads Vance" ท้ายชื่อคลิปและจำกัดความยาว เพื่อให้ข้อความ
// อ่านเป็นพาดหัวไม่ใช่บรรทัด SEO · ใช้ทั้งแคปชัน TikTok และข้อความที่ส่งเข้าช่อง Telegram
func stripBrandTag(title string) string {
	if i := strings.LastIndex(title, "|"); i >= 0 && strings.Contains(strings.ToLower(title[i:]), "ads vance") {
		title = strings.TrimSpace(title[:i])
	}
	const maxRunes = 120
	if r := []rune(title); len(r) > maxRunes {
		title = strings.TrimSpace(string(r[:maxRunes]))
	}
	return title
}

// YouTube ตีตกทั้งการอัปโหลดถ้าชื่อหรือคำอธิบายมี '<' หรือ '>' ("Video description
// is invalid") เพราะกันโค้ด HTML — และ Zernio รายงานกลับมาแค่ "All platforms failed"
// ทำให้คลิปค้าง ready เงียบๆ ไม่มีใครรู้สาเหตุ (เกิดจริง 2026-08-02 คลิป e6973467)
// คลิปสอนเขียนเส้นทางเมนู "Columns > Customise columns" เป็นปกติ จึงต้องแปลงก่อนส่ง
// ทุกครั้ง · แทนด้วย ‹ › ที่หน้าตาเกือบเหมือนกันแทนการลบทิ้ง เพื่อให้ทั้งเส้นทางเมนูและ
// เครื่องหมายมากกว่า/น้อยกว่ายังอ่านรู้เรื่องบนหน้าคลิป
var youtubeAngleBrackets = strings.NewReplacer("<", "‹", ">", "›")

func sanitizeYouTubeText(s string) string {
	return youtubeAngleBrackets.Replace(s)
}

// youtubePlatforms ประกอบเป้าหมายโพสต์ฝั่ง YouTube ของคำขอ Zernio
// เมื่อ firstComment ไม่ว่าง มันจะไปกับ platformSpecificData แล้ว Zernio โพสต์คอมเมนต์ให้
// (เอกสารบอกว่าปักหมุดด้วย แต่ live test 2026-07-31 พบว่าไม่ปัก — ถ้าอยากได้หมุดต้องปักเองใน
// YouTube Studio) · ค่าว่าง = ไม่แนบบล็อกนี้เลย คำขอเหมือนก่อนมีฟีเจอร์นี้เป๊ะ
// รับ title เข้ามาเพราะโพสต์ 16:9 กับ 9:16 ใช้ชื่อคนละอัน (Shorts ตัดที่ 60 ตัวอักษร
// แล้วต่อท้าย #Shorts) — ส่ง target ตัวเดียวร่วมกันไม่ได้ · รับ visibility เข้ามาแทนที่จะ
// hardcode ในนี้ เพื่อให้ค่าเดียวกันป้อนทั้งระดับบนกับ platformSpecificData เสมอ
func youtubePlatforms(accountID, title, firstComment, visibility string) []PlatformTarget {
	target := PlatformTarget{Platform: platformYouTube, AccountID: accountID}
	if firstComment != "" {
		target.PlatformSpecificData = &YouTubeOptions{
			Title:        title,
			Visibility:   visibility,
			FirstComment: firstComment,
		}
	}
	return []PlatformTarget{target}
}

// postRequestID สร้าง id รูป UUID แบบ deterministic ต่อ (clip, format) — retry รอบถัดไป
// ของคลิปเดิมจึงส่ง x-request-id เดิมเสมอ ให้ Zernio จับ replay ได้แทนที่จะสร้างโพสต์ซ้ำ
func postRequestID(clipID, format string) string {
	sum := sha1.Sum([]byte("adsvance-publish:" + clipID + ":" + format))
	sum[6] = sum[6]&0x0f | 0x50 // version 5
	sum[8] = sum[8]&0x3f | 0x80 // RFC 4122 variant
	h := hex.EncodeToString(sum[:16])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

type Publisher struct {
	zernio    *ZernioClient
	pool      *pgxpool.Pool
	clipsRepo *repository.ClipsRepo
	analytics *repository.AnalyticsRepo
}

func NewPublisher(zernio *ZernioClient, pool *pgxpool.Pool, clips *repository.ClipsRepo, analytics *repository.AnalyticsRepo) *Publisher {
	return &Publisher{zernio: zernio, pool: pool, clipsRepo: clips, analytics: analytics}
}

// publishLockKey กันสองรอบส่งทำงานทับกัน · ตั้งแต่รอบส่งมี schedule ของตัวเองทุก 15
// นาที มันจึงชนกับขั้น publish ที่แปะท้ายรอบผลิตได้ (ผลิตเสร็จคาบเกี่ยวนาที :15 พอดี)
// สองรอบจะ SELECT คลิป ready ตัวเดียวกันแล้วอัปขึ้น YouTube ซ้ำ เพราะการโพสต์เกิดก่อน
// UPDATE status เสมอ — ล็อกนี้คือสิ่งที่ทำให้เพิ่มความถี่รอบส่งได้อย่างปลอดภัย
const publishLockKey int64 = 8213401

func (p *Publisher) PublishReady(ctx context.Context) error {
	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire publish lock conn: %w", err)
	}
	defer conn.Release()
	var locked bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, publishLockKey).Scan(&locked); err != nil {
		return fmt.Errorf("publish lock: %w", err)
	}
	if !locked {
		log.Println("PublishReady: another publish run is in flight, skipping this tick")
		return nil
	}
	// ปลดล็อกด้วย connection เดิมเสมอ และใช้ context.Background() เพราะถ้า ctx ถูก
	// cancel กลางทาง (deploy/stop) การปลดล็อกด้วย ctx เดิมจะไม่ทำงาน แล้วล็อกจะค้าง
	// จนกว่า connection จะตาย = รอบส่งหยุดทั้งระบบเงียบๆ · ต้อง log ถ้าปลดไม่สำเร็จ
	// เพราะอาการของมันคือ "ไม่มีอะไรเกิดขึ้น" ซึ่งหน้าตาเหมือนตอนไม่มีคลิปให้ส่งเป๊ะ
	defer func() {
		if _, err := conn.Exec(context.Background(),
			`SELECT pg_advisory_unlock($1)`, publishLockKey); err != nil {
			log.Printf("CRITICAL PublishReady: ปลด advisory lock ไม่สำเร็จ: %v — รอบส่งจะข้ามไปเรื่อยๆ จนกว่า connection นี้จะถูกรีไซเคิล", err)
		}
	}()

	// ดันท้ายคิวเฉพาะคลิปที่ "ส่งพลาด" (fail_reason ขึ้นต้นด้วย publishFailPrefix) ให้คลิปดี
	// ได้คิวก่อน — กัน head-of-line blocking แบบเหตุ 08-08 (คลิปเสีย 1 ตัวกินคิวทุกรอบเพราะ
	// LIMIT 1 + เรียงตามวันเก่าสุด) · เหตุผลจากด่านอื่น (layout inspector ฯลฯ) ไม่เกี่ยวกับรอบส่ง
	// ต้องไม่โดนดัน ไม่งั้นคลิปที่คน approve จาก needs_review ถูกท้ายคิวทั้งที่ไม่เคยส่งพลาด
	rows, err := p.pool.Query(ctx,
		`SELECT c.id, cm.youtube_title, cm.youtube_description, c.video_16_9_url, c.video_9_16_url, c.thumbnail_url
		 FROM clips c
		 JOIN clip_metadata cm ON c.id = cm.clip_id
		 WHERE c.status = 'ready' AND c.auto_review_held = FALSE AND c.publish_date <= CURRENT_DATE
		 ORDER BY (c.fail_reason IS NOT NULL AND starts_with(c.fail_reason, $1)), c.publish_date ASC LIMIT 1`,
		publishFailPrefix)
	if err != nil {
		return fmt.Errorf("query ready clips: %w", err)
	}
	defer rows.Close()

	var ytAccountID string
	p.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = 'zernio_youtube_account_id'`).Scan(&ytAccountID)

	// คอมเมนต์ติดต่อทีมงานใต้คลิป (ไม่ปักหมุด ดูเหตุผลที่ youtubePlatforms) · อ่านครั้งเดียวต่อรอบเหมือน ytAccountID
	// ค่าว่าง (หรือไม่มีแถวนี้) = ปิดฟีเจอร์ ไม่ต้อง deploy — Scan ที่ error จะทิ้ง
	// firstComment ไว้เป็น "" ซึ่งคือสถานะปิดพอดี จึงไม่ต้องจัดการ error แยก
	var firstComment string
	_ = p.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = 'youtube_first_comment'`).Scan(&firstComment)
	firstComment = strings.TrimSpace(firstComment)

	// ช่อง Telegram ที่จะส่งคลิปเข้าไปหลังขึ้น YouTube · ค่าว่าง (หรือไม่มีแถวนี้) = ปิดฟีเจอร์
	// อ่านครั้งเดียวต่อรอบเหมือน ytAccountID — Scan ที่ error ทิ้งค่าไว้เป็น "" ซึ่งคือสถานะปิดพอดี
	var tgAccountID string
	_ = p.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = 'zernio_telegram_account_id'`).Scan(&tgAccountID)
	tgAccountID = strings.TrimSpace(tgAccountID)

	for rows.Next() {
		var clipID, title string
		var description *string
		var video169, video916, thumb *string
		if err := rows.Scan(&clipID, &title, &description, &video169, &video916, &thumb); err != nil {
			return fmt.Errorf("scan clip: %w", err)
		}

		if isContactInfo(title) {
			var clipTitle string
			if err := p.pool.QueryRow(ctx, `SELECT title FROM clips WHERE id = $1`, clipID).Scan(&clipTitle); err == nil && clipTitle != "" {
				log.Printf("Title validation: '%s' looks like contact info, using clip question instead", title)
				title = clipTitle
			}
		}

		desc := ""
		if description != nil {
			desc = *description
		}

		// แปลงครั้งเดียวตรงนี้ให้ครอบทั้งโพสต์ 16:9 และ 9:16 (shortsTitle ต่อยอดจาก title)
		title = sanitizeYouTubeText(title)
		desc = sanitizeYouTubeText(desc)

		if ytAccountID == "" {
			log.Printf("No Zernio accounts configured, skipping clip %s", clipID)
			continue
		}

		// Post whatever formats this clip actually has. The hyperframes pipeline
		// produces 9:16 only; the legacy pipeline produced 16:9 (+ a 9:16 Short).
		// Publish each format that exists rather than requiring 16:9.
		var mainPostID, shortsPostID string
		var postErr error

		// 16:9 (YouTube regular), if present. Zernio uses Content's first line as the title.
		if video169 != nil && *video169 != "" {
			result169, err := p.zernio.Post(ctx, PostRequest{
				Title:      title,
				Content:    title + "\n\n" + desc,
				Platforms:  youtubePlatforms(ytAccountID, title, firstComment, VisibilityPublic),
				MediaItems: []MediaItem{{Type: "video", URL: *video169}},
				Visibility: VisibilityPublic,
				PublishNow: true,
				RequestID:  postRequestID(clipID, "169"),
			})
			if err != nil {
				if id, ok := p.adoptDuplicate(ctx, err); ok {
					mainPostID = id
				} else {
					log.Printf("Failed to post 16:9 for clip %s: %v", clipID, err)
					p.recordPublishFailure(ctx, clipID, err)
					continue
				}
			} else {
				log.Printf("Posted 16:9 public for clip %s → %s", clipID, result169.Post.ID)
				mainPostID = result169.Post.ID
			}
		}

		// 9:16 (YouTube Shorts), if present.
		if video916 != nil && *video916 != "" {
			shortsTitle := title
			if utf8.RuneCountInString(shortsTitle) > 60 {
				runes := []rune(shortsTitle)
				shortsTitle = string(runes[:60])
			}
			shortsTitle += " #Shorts"

			result916, err := p.zernio.Post(ctx, PostRequest{
				Title:      shortsTitle,
				Content:    shortsTitle + "\n\n" + desc,
				Platforms:  youtubePlatforms(ytAccountID, shortsTitle, firstComment, VisibilityPublic),
				MediaItems: []MediaItem{{Type: "video", URL: *video916}},
				Visibility: VisibilityPublic,
				PublishNow: true,
				RequestID:  postRequestID(clipID, "916"),
			})
			if err != nil {
				if id, ok := p.adoptDuplicate(ctx, err); ok {
					shortsPostID = id
				} else {
					log.Printf("Failed to post 9:16 for clip %s: %v", clipID, err)
					postErr = err
				}
			} else {
				log.Printf("Posted 9:16 Shorts public for clip %s → %s", clipID, result916.Post.ID)
				shortsPostID = result916.Post.ID
			}
		}

		// Nothing posted (no usable video, or every post failed) → leave the clip
		// 'ready' so a later run retries it instead of marking an empty publish.
		if mainPostID == "" && shortsPostID == "" {
			log.Printf("No video published for clip %s, leaving as ready", clipID)
			if postErr != nil {
				p.recordPublishFailure(ctx, clipID, postErr)
			}
			continue
		}

		// Persist status + post ids atomically. The YouTube post already happened,
		// so a silent DB failure here would leave the clip 'ready' and re-post it
		// (duplicate upload) on the next run — log loudly if the commit fails.
		if err := p.recordPublished(ctx, clipID, mainPostID, shortsPostID); err != nil {
			log.Printf("CRITICAL clip %s posted to YouTube (main=%q shorts=%q) but DB commit FAILED: %v — will be re-published next run", clipID, mainPostID, shortsPostID, err)
			continue
		}

		// เงื่อนไข postErr == nil: คลิปที่มีทั้ง 16:9 และ 9:16 อาจขึ้นได้แค่รูปแบบเดียว
		// อีกตัวเพิ่งล้มไป — เหตุผลนั้นยังใหม่อยู่ ห้ามล้างทิ้ง
		if postErr == nil {
			p.clearPublishFailure(ctx, clipID)
		}

		log.Printf("Published clip %s via Zernio (main=%q shorts=%q)", clipID, mainPostID, shortsPostID)

		// ส่งเข้าช่อง Telegram ท้ายสุดเสมอ — คลิปขึ้น YouTube และ DB commit ไปแล้ว
		// อะไรที่พังหลังจากนี้จึงกระทบแค่ช่อง Telegram ช่องเดียว
		p.postTelegramForClip(ctx, clipID, tgAccountID, title, mainPostID, shortsPostID)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate ready clips: %w", err)
	}

	// เก็บตกท้ายรอบ: คลิปที่ขึ้น YouTube แล้วแต่ยังไม่เข้าช่อง Telegram (จำกัด 24 ชม.)
	p.sweepTelegramBacklog(ctx, tgAccountID)
	return nil
}

// adoptDuplicate ตัดสิน 409: โพสต์เดิมขึ้นจริงแล้ว → คืน id ให้บันทึกเป็นสำเร็จ · ยังไม่ขึ้น
// (scheduled/publishing/failed) → ไม่ adopt ปล่อยให้วน retry ตามเดิม เพราะ mark published
// ทั้งที่ของจริงยังไม่ขึ้นคือการโกหก DB แบบเดียวกับที่เพิ่งแก้มา
func (p *Publisher) adoptDuplicate(ctx context.Context, cause error) (string, bool) {
	var dup *DuplicatePostError
	if !errors.As(cause, &dup) {
		return "", false
	}
	ps, err := p.zernio.GetPost(ctx, dup.ExistingPostID)
	if err != nil {
		log.Printf("adoptDuplicate: เช็คสถานะโพสต์ %s ไม่สำเร็จ: %v", dup.ExistingPostID, err)
		return "", false
	}
	if ps.Status != "published" {
		log.Printf("adoptDuplicate: โพสต์ %s สถานะ %s ยังไม่ adopt", ps.ID, ps.Status)
		return "", false
	}
	log.Printf("adoptDuplicate: โพสต์ %s ขึ้นแล้วจริง — บันทึกเป็นสำเร็จ", ps.ID)
	return ps.ID, true
}

// recordPublished marks the clip 'published' and records its Zernio post ids in a
// single transaction, so a partial write can't leave the clip 'published' without
// ids (orphaned from analytics) or 'ready' with ids (re-posted next run).
// recordPublishFailure เขียนเหตุผลลง fail_reason ให้เห็นบนหน้าคลิป · คลิปที่ส่งไม่ผ่าน
// ยังคาสถานะ 'ready' แล้ววนใหม่ทุกชั่วโมง ถ้าไม่บันทึกไว้ อาการที่คนใช้เห็นคือ "กด publish
// แล้วไม่มีอะไรเกิดขึ้น" ต้องไปไล่ log บน Railway ถึงจะรู้สาเหตุ (เกิดจริง 2026-08-02)
// เขียนไม่สำเร็จก็แค่ log ต่อ — การส่งคลิปสำคัญกว่าการบันทึกเหตุผล
func (p *Publisher) recordPublishFailure(ctx context.Context, clipID string, cause error) {
	if err := p.clipsRepo.SetFailReason(ctx, clipID, publishFailPrefix+cause.Error()); err != nil {
		log.Printf("บันทึก fail_reason ของคลิป %s ไม่สำเร็จ: %v", clipID, err)
	}
}

// clearPublishFailure ล้างเหตุผลของรอบส่งเมื่อคลิปขึ้นสำเร็จแล้ว เพื่อไม่ให้ข้อความล้มเหลว
// ของรอบก่อนค้างโชว์อยู่บนคลิปที่ขึ้นไปแล้ว · ล้างเฉพาะข้อความที่ขึ้นต้นด้วย publishFailPrefix
// เหตุผลจากด่านอื่น (เช่น "layout inspector: ...") ต้องอยู่ต่อ คนถัดไปยังต้องใช้
func (p *Publisher) clearPublishFailure(ctx context.Context, clipID string) {
	if err := p.clipsRepo.ClearFailReasonPrefixed(ctx, clipID, publishFailPrefix); err != nil {
		log.Printf("ล้าง fail_reason ของคลิป %s ไม่สำเร็จ: %v", clipID, err)
	}
}

func (p *Publisher) recordPublished(ctx context.Context, clipID, mainPostID, shortsPostID string) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`UPDATE clips SET status = 'published', updated_at = NOW() WHERE id = $1`, clipID); err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	if mainPostID != "" {
		if _, err := tx.Exec(ctx,
			`UPDATE clip_metadata SET zernio_post_id = $2 WHERE clip_id = $1`, clipID, mainPostID); err != nil {
			return fmt.Errorf("set zernio_post_id: %w", err)
		}
	}
	if shortsPostID != "" {
		if _, err := tx.Exec(ctx,
			`UPDATE clip_metadata SET zernio_shorts_post_id = $2 WHERE clip_id = $1`, clipID, shortsPostID); err != nil {
			return fmt.Errorf("set zernio_shorts_post_id: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// PublishTikTok posts the 9:16 video of the newest not-yet-posted clip to TikTok
// (newest-first so fresh content goes up first). TikTok-only: it does NOT touch
// YouTube or clip.status — it just records zernio_tiktok_post_id so the clip is
// not re-posted. One clip per call (mirrors PublishReady's drip).
func (p *Publisher) PublishTikTok(ctx context.Context) error {
	var ttAccountID string
	if err := p.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = 'zernio_tiktok_account_id'`).Scan(&ttAccountID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("read zernio_tiktok_account_id setting: %w", err)
	}
	if ttAccountID == "" {
		log.Printf("PublishTikTok: no zernio_tiktok_account_id configured, skipping")
		return nil
	}

	var clipID, title string
	var video916, clipTitle *string
	// Only drip clips whose video lives on permanent storage (R2). Pre-R2 clips
	// point at kie temp URLs that expire in ~1 day — the drip reaches them days
	// later, TikTok fails to fetch the file, and the file is gone for good
	// (52 backlog clips hit this after the 2026-07-04 R2 cutover).
	err := p.pool.QueryRow(ctx,
		`SELECT c.id, cm.youtube_title, c.video_9_16_url, c.title
		 FROM clips c JOIN clip_metadata cm ON c.id = cm.clip_id
		 WHERE c.video_9_16_url IS NOT NULL AND c.video_9_16_url <> ''
		   AND c.status IN ('ready','published') AND c.auto_review_held = FALSE
		   AND (cm.zernio_tiktok_post_id IS NULL OR cm.zernio_tiktok_post_id = '')
		   AND c.video_9_16_url LIKE (SELECT value || '%' FROM settings WHERE key = 'r2_public_base_url')
		 ORDER BY c.created_at DESC LIMIT 1`).
		Scan(&clipID, &title, &video916, &clipTitle)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("PublishTikTok: no clip pending for TikTok")
			return nil
		}
		return fmt.Errorf("query clip for tiktok: %w", err)
	}

	if isContactInfo(title) && clipTitle != nil && *clipTitle != "" {
		log.Printf("TikTok title validation: '%s' looks like contact info, using clip question instead", title)
		title = *clipTitle
	}

	// TikTok caption: short hook + hashtags, NO YouTube SEO description (which
	// carries off-platform LINE/Telegram links that suppress reach). Hashtags
	// are tunable via the tiktok_hashtags setting without a redeploy.
	var hashtags string
	_ = p.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = 'tiktok_hashtags'`).Scan(&hashtags)
	caption := buildTikTokCaption(title, hashtags)

	result, err := p.zernio.Post(ctx, PostRequest{
		Title:      stripBrandTag(title),
		Content:    caption,
		Platforms:  []PlatformTarget{{Platform: "tiktok", AccountID: ttAccountID}},
		MediaItems: []MediaItem{{Type: "video", URL: *video916}},
		PublishNow: true,
		TikTokSettings: &TikTokSettings{
			PrivacyLevel:            "PUBLIC_TO_EVERYONE",
			AllowComment:            true,
			AllowDuet:               true,
			AllowStitch:             true,
			ContentPreviewConfirmed: true,
			ExpressConsentGiven:     true,
		},
	})
	if err != nil {
		return fmt.Errorf("post tiktok for clip %s: %w", clipID, err)
	}

	// The TikTok post already happened and isn't idempotent; the selection query is
	// newest-first, so a failed record would re-post THIS clip every run (duplicate)
	// and block newer clips. Retry the (idempotent) record write before giving up.
	var recErr error
	for attempt := 1; attempt <= 3; attempt++ {
		if _, recErr = p.pool.Exec(ctx,
			`UPDATE clip_metadata SET zernio_tiktok_post_id = $2 WHERE clip_id = $1`,
			clipID, result.Post.ID); recErr == nil {
			break
		}
		log.Printf("PublishTikTok: record tiktok_post_id attempt %d/3 for clip %s failed: %v", attempt, clipID, recErr)
		time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
	}
	if recErr != nil {
		log.Printf("CRITICAL clip %s posted to TikTok (%s) but recording tiktok_post_id FAILED after retries: %v — may be re-posted next run", clipID, result.Post.ID, recErr)
		return nil
	}
	log.Printf("Posted to TikTok for clip %s → %s", clipID, result.Post.ID)
	return nil
}

type postRef struct {
	id       string
	platform string
	label    string
}

func (p *Publisher) configuredPlatforms(ctx context.Context) ([]string, error) {
	prows, err := p.pool.Query(ctx,
		`SELECT key FROM settings WHERE key LIKE 'zernio_%_account_id' AND value != ''`)
	if err != nil {
		return nil, fmt.Errorf("query platform settings: %w", err)
	}
	defer prows.Close()

	var platforms []string
	for prows.Next() {
		var key string
		if err := prows.Scan(&key); err != nil {
			return nil, fmt.Errorf("scan platform key: %w", err)
		}
		platform := strings.TrimPrefix(key, "zernio_")
		platform = strings.TrimSuffix(platform, "_account_id")
		platforms = append(platforms, platform)
	}
	return platforms, nil
}

func (p *Publisher) FetchAnalytics(ctx context.Context) error {
	platforms, err := p.configuredPlatforms(ctx)
	if err != nil {
		return fmt.Errorf("configured platforms: %w", err)
	}
	if len(platforms) == 0 {
		log.Println("FetchAnalytics: no platforms configured, skipping")
		return nil
	}
	log.Printf("FetchAnalytics: platforms=%v", platforms)

	var ytAccountID string
	_ = p.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = 'zernio_youtube_account_id'`).Scan(&ytAccountID)

	var totalPublished int
	_ = p.pool.QueryRow(ctx, `SELECT COUNT(*) FROM clips WHERE status = 'published'`).Scan(&totalPublished)

	rows, err := p.pool.Query(ctx,
		`SELECT cm.clip_id, cm.zernio_post_id, cm.zernio_shorts_post_id, cm.zernio_tiktok_post_id
		 FROM clip_metadata cm
		 JOIN clips c ON c.id = cm.clip_id
		 WHERE c.status = 'published'
		   AND (cm.zernio_post_id IS NOT NULL OR cm.zernio_shorts_post_id IS NOT NULL OR cm.zernio_tiktok_post_id IS NOT NULL)`)
	if err != nil {
		return fmt.Errorf("query published clips: %w", err)
	}
	defer rows.Close()

	type clipPost struct {
		ClipID       string
		PostID       *string
		ShortsPostID *string
		TikTokPostID *string
	}
	var clips []clipPost
	for rows.Next() {
		var cp clipPost
		if err := rows.Scan(&cp.ClipID, &cp.PostID, &cp.ShortsPostID, &cp.TikTokPostID); err != nil {
			return fmt.Errorf("scan clip metadata: %w", err)
		}
		clips = append(clips, cp)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate published clips: %w", err)
	}
	log.Printf("FetchAnalytics: %d clips with zernio IDs / %d total published", len(clips), totalPublished)

	// Each Zernio post targets a single platform, and its post ID lives in a
	// platform-specific column, so pair every post ID with its own platform.
	// TikTok is a 9:16 short-form video, so it's counted under the "shorts" segment.
	configured := make(map[string]bool, len(platforms))
	for _, pl := range platforms {
		configured[pl] = true
	}

	var success, failed, dbFailed int
	for _, cp := range clips {
		var posts []postRef
		if cp.PostID != nil && *cp.PostID != "" {
			posts = append(posts, postRef{*cp.PostID, platformYouTube, "regular"})
		}
		if cp.ShortsPostID != nil && *cp.ShortsPostID != "" {
			posts = append(posts, postRef{*cp.ShortsPostID, platformYouTube, "shorts"})
		}
		if cp.TikTokPostID != nil && *cp.TikTokPostID != "" {
			posts = append(posts, postRef{*cp.TikTokPostID, platformTikTok, "shorts"})
		}
		for _, post := range posts {
			if !configured[post.platform] {
				continue
			}
			resp, err := p.zernio.GetAnalytics(ctx, post.id, post.platform)
			if err != nil {
				log.Printf("FetchAnalytics FAIL clip=%s type=%s platform=%s post=%s: %v", cp.ClipID, post.label, post.platform, post.id, err)
				failed++
				continue
			}
			status, errMsg := resolvePostStatus(resp, post.platform)
			var errPtr *string
			if errMsg != "" {
				errPtr = &errMsg
			}
			if err := p.analytics.UpsertPublishStatus(ctx, models.ClipPublishStatus{
				ClipID: cp.ClipID, Platform: post.platform, PostType: post.label,
				ZernioPostID: post.id, Status: status, ErrorMessage: errPtr,
			}); err != nil {
				log.Printf("FetchAnalytics STATUS_FAIL clip=%s platform=%s: %v", cp.ClipID, post.platform, err)
			}
			// A failed post never went live — record its publish status above, but
			// don't write a 0-view analytics row that would masquerade as real content.
			if status == "failed" {
				continue
			}
			var metrics PostMetrics
			found := false
			for _, pa := range resp.PlatformAnalytics {
				if pa.Platform == post.platform {
					metrics = pa.Analytics
					found = true
					break
				}
			}
			// TikTok (and some platforms) return a flat top-level "analytics"
			// object with no platformAnalytics entry; fall back to it. LastUpdated
			// is non-empty only when Zernio actually returned analytics.
			if !found && resp.Analytics.LastUpdated != "" {
				metrics = resp.Analytics
				found = true
			}
			if !found {
				log.Printf("FetchAnalytics NO_PLATFORM_DATA clip=%s platform=%s post=%s syncStatus=%s", cp.ClipID, post.platform, post.id, resp.SyncStatus)
				failed++
				continue
			}
			var detail ytDetail
			if post.platform == platformYouTube && metrics.Views > 0 {
				detail = p.fetchYouTubeDetail(ctx, cp.ClipID, ytAccountID, resp)
			}
			// รอบที่ Zernio ไม่คืน daily-views จะได้ detail เป็นศูนย์ ถ้าปล่อยศูนย์ลง DB
			// มันจะกลายเป็นแถวใหม่สุดแล้วบังค่าที่เคยวัดได้จริง — ดู mergeRetention
			ret := models.RetentionSnapshot{
				WatchTimeSeconds:  detail.WatchTime,
				RetentionRate:     detail.Retention,
				AvgViewPercentage: detail.AvgViewPct,
			}
			if ret.AvgViewPercentage == 0 {
				// ยิงเพิ่มเฉพาะตอนจำเป็น ทางที่ดึงข้อมูลได้ปกติจึงไม่มี query เกิน
				prev, perr := p.analytics.LastKnownRetention(ctx, cp.ClipID, post.platform, post.label)
				if perr != nil {
					log.Printf("FetchAnalytics CARRY_FAIL clip=%s platform=%s type=%s: %v",
						cp.ClipID, post.platform, post.label, perr)
				}
				ret = mergeRetention(ret, prev)
			}
			if err := p.analytics.Create(ctx, models.ClipAnalytics{
				ClipID:            cp.ClipID,
				Platform:          post.platform,
				PostType:          post.label,
				Views:             metrics.Views,
				Likes:             metrics.Likes,
				Comments:          metrics.Comments,
				Shares:            metrics.Shares,
				WatchTimeSeconds:  ret.WatchTimeSeconds,
				RetentionRate:     ret.RetentionRate,
				EngagementRate:    metrics.EngagementRate,
				AvgViewPercentage: ret.AvgViewPercentage,
				SubscribersGained: detail.SubsGained,
				SubscribersLost:   detail.SubsLost,
			}); err != nil {
				log.Printf("FetchAnalytics DB_FAIL clip=%s platform=%s: %v", cp.ClipID, post.platform, err)
				dbFailed++
				continue
			}
			for _, dv := range detail.Daily {
				if err := p.analytics.UpsertDaily(ctx, models.ClipAnalyticsDaily{
					ClipID: cp.ClipID, Platform: post.platform, PostType: post.label,
					Date: dv.Date, Views: dv.Views,
					EstimatedMinutesWatched: dv.EstimatedMinutesWatched,
					AverageViewDuration:     dv.AverageViewDuration,
					AvgViewPercentage:       dv.AverageViewPercentage / 100.0,
					SubscribersGained:       dv.SubscribersGained,
					SubscribersLost:         dv.SubscribersLost,
					Likes:                   dv.Likes, Comments: dv.Comments, Shares: dv.Shares,
				}); err != nil {
					log.Printf("FetchAnalytics DAILY_FAIL clip=%s platform=%s date=%s: %v", cp.ClipID, post.platform, dv.Date, err)
				}
			}
			success++
		}
	}
	log.Printf("FetchAnalytics done: %d clips, %d success, %d api_fail, %d db_fail", len(clips), success, failed, dbFailed)
	return nil
}

// normalizePublishStatus maps Zernio's status strings onto our 4-value enum.
func normalizePublishStatus(s string) string {
	switch s {
	case "published", "failed", "scheduled":
		return s
	case "pending", "processing":
		return "scheduled"
	default:
		return "unknown"
	}
}

// resolvePostStatus prefers the matching platformAnalytics entry's status and
// errorMessage, falling back to the response-level status.
func resolvePostStatus(resp *AnalyticsResponse, platform string) (string, string) {
	for _, pa := range resp.PlatformAnalytics {
		if pa.Platform == platform && pa.Status != "" {
			return normalizePublishStatus(pa.Status), pa.ErrorMessage
		}
	}
	return normalizePublishStatus(resp.Status), resp.Message
}

// ytDetail carries everything the daily-views endpoint gives us for one video.
type ytDetail struct {
	WatchTime  float64 // seconds, summed over the window
	Retention  float64 // legacy: avg view duration / clip duration, capped at 1
	AvgViewPct float64 // fraction from YouTube's own averageViewPercentage, view-weighted, NOT capped
	SubsGained int
	SubsLost   int
	Daily      []DailyViewEntry
}

func (p *Publisher) fetchYouTubeDetail(ctx context.Context, clipID, ytAccountID string, resp *AnalyticsResponse) ytDetail {
	var d ytDetail
	if ytAccountID == "" {
		return d
	}
	var videoID string
	for _, pa := range resp.PlatformAnalytics {
		if pa.Platform == platformYouTube {
			videoID = pa.PlatformPostID
			break
		}
	}
	if videoID == "" {
		return d
	}
	daily, err := p.zernio.GetYouTubeDailyViews(ctx, videoID, ytAccountID)
	if err != nil {
		if errors.Is(err, ErrYouTubeScopeMissing) {
			log.Printf("FetchAnalytics: YouTube analytics scope missing — re-auth needed (skipping watchTime)")
		} else {
			log.Printf("FetchAnalytics WATCHTIME_FAIL clip=%s video=%s: %v", clipID, videoID, err)
		}
		return d
	}
	if len(daily.DailyViews) == 0 {
		return d
	}
	d.Daily = daily.DailyViews

	var totalMinutes, avgDurSum, pctWeighted float64
	var viewSum int
	for _, dv := range daily.DailyViews {
		totalMinutes += dv.EstimatedMinutesWatched
		avgDurSum += dv.AverageViewDuration
		d.SubsGained += dv.SubscribersGained
		d.SubsLost += dv.SubscribersLost
		if dv.Views > 0 {
			pctWeighted += dv.AverageViewPercentage * float64(dv.Views)
			viewSum += dv.Views
		}
	}
	d.WatchTime = totalMinutes * 60
	if viewSum > 0 {
		d.AvgViewPct = pctWeighted / float64(viewSum) / 100.0
	}

	// Legacy retention: avg view duration / total clip duration, capped at 1.
	var clipDuration float64
	if err := p.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(duration_seconds), 0) FROM scenes WHERE clip_id = $1`, clipID,
	).Scan(&clipDuration); err != nil {
		log.Printf("FetchAnalytics RETENTION_FAIL clip=%s: query clip duration: %v", clipID, err)
		return d
	}
	if clipDuration > 0 {
		avgViewDuration := avgDurSum / float64(len(daily.DailyViews))
		d.Retention = avgViewDuration / clipDuration
		if d.Retention > 1.0 {
			d.Retention = 1.0
		}
	}
	return d
}
