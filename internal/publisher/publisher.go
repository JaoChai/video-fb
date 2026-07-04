package publisher

import (
	"context"
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

func isContactInfo(title string) bool {
	lower := strings.ToLower(title)
	return strings.Contains(lower, "line id") ||
		strings.Contains(lower, "@adsvance") ||
		strings.Contains(lower, "ติดต่อทีมงาน") ||
		strings.Contains(lower, "t.me/") ||
		strings.Contains(lower, "https://")
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

func (p *Publisher) PublishReady(ctx context.Context) error {
	rows, err := p.pool.Query(ctx,
		`SELECT c.id, cm.youtube_title, cm.youtube_description, c.video_16_9_url, c.video_9_16_url, c.thumbnail_url
		 FROM clips c
		 JOIN clip_metadata cm ON c.id = cm.clip_id
		 WHERE c.status = 'ready' AND c.auto_review_held = FALSE AND c.publish_date <= CURRENT_DATE
		 ORDER BY c.publish_date ASC LIMIT 1`)
	if err != nil {
		return fmt.Errorf("query ready clips: %w", err)
	}
	defer rows.Close()

	var ytAccountID string
	p.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = 'zernio_youtube_account_id'`).Scan(&ytAccountID)

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

		var platforms []PlatformTarget
		if ytAccountID != "" {
			platforms = append(platforms, PlatformTarget{Platform: "youtube", AccountID: ytAccountID})
		}
		if len(platforms) == 0 {
			log.Printf("No Zernio accounts configured, skipping clip %s", clipID)
			continue
		}

		// Post whatever formats this clip actually has. The hyperframes pipeline
		// produces 9:16 only; the legacy pipeline produced 16:9 (+ a 9:16 Short).
		// Publish each format that exists rather than requiring 16:9.
		var mainPostID, shortsPostID string

		// 16:9 (YouTube regular), if present. Zernio uses Content's first line as the title.
		if video169 != nil && *video169 != "" {
			result169, err := p.zernio.Post(ctx, PostRequest{
				Title:      title,
				Content:    title + "\n\n" + desc,
				Platforms:  platforms,
				MediaItems: []MediaItem{{Type: "video", URL: *video169}},
				Visibility: VisibilityPublic,
				PublishNow: true,
			})
			if err != nil {
				log.Printf("Failed to post 16:9 for clip %s: %v", clipID, err)
				continue
			}
			log.Printf("Posted 16:9 public for clip %s → %s", clipID, result169.Post.ID)
			mainPostID = result169.Post.ID
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
				Platforms:  platforms,
				MediaItems: []MediaItem{{Type: "video", URL: *video916}},
				Visibility: VisibilityPublic,
				PublishNow: true,
			})
			if err != nil {
				log.Printf("Failed to post 9:16 for clip %s: %v", clipID, err)
			} else {
				log.Printf("Posted 9:16 Shorts public for clip %s → %s", clipID, result916.Post.ID)
				shortsPostID = result916.Post.ID
			}
		}

		// Nothing posted (no usable video, or every post failed) → leave the clip
		// 'ready' so a later run retries it instead of marking an empty publish.
		if mainPostID == "" && shortsPostID == "" {
			log.Printf("No video published for clip %s, leaving as ready", clipID)
			continue
		}

		// Persist status + post ids atomically. The YouTube post already happened,
		// so a silent DB failure here would leave the clip 'ready' and re-post it
		// (duplicate upload) on the next run — log loudly if the commit fails.
		if err := p.recordPublished(ctx, clipID, mainPostID, shortsPostID); err != nil {
			log.Printf("CRITICAL clip %s posted to YouTube (main=%q shorts=%q) but DB commit FAILED: %v — will be re-published next run", clipID, mainPostID, shortsPostID, err)
			continue
		}

		log.Printf("Published clip %s via Zernio (main=%q shorts=%q)", clipID, mainPostID, shortsPostID)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate ready clips: %w", err)
	}
	return nil
}

// recordPublished marks the clip 'published' and records its Zernio post ids in a
// single transaction, so a partial write can't leave the clip 'published' without
// ids (orphaned from analytics) or 'ready' with ids (re-posted next run).
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
	var description, video916, clipTitle *string
	err := p.pool.QueryRow(ctx,
		`SELECT c.id, cm.youtube_title, cm.youtube_description, c.video_9_16_url, c.title
		 FROM clips c JOIN clip_metadata cm ON c.id = cm.clip_id
		 WHERE c.video_9_16_url IS NOT NULL AND c.video_9_16_url <> ''
		   AND c.status IN ('ready','published') AND c.auto_review_held = FALSE
		   AND (cm.zernio_tiktok_post_id IS NULL OR cm.zernio_tiktok_post_id = '')
		 ORDER BY c.created_at DESC LIMIT 1`).
		Scan(&clipID, &title, &description, &video916, &clipTitle)
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
	desc := ""
	if description != nil {
		desc = *description
	}

	result, err := p.zernio.Post(ctx, PostRequest{
		Title:      title,
		Content:    title + "\n\n" + desc,
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
			if err := p.analytics.Create(ctx, models.ClipAnalytics{
				ClipID:            cp.ClipID,
				Platform:          post.platform,
				PostType:          post.label,
				Views:             metrics.Views,
				Likes:             metrics.Likes,
				Comments:          metrics.Comments,
				Shares:            metrics.Shares,
				WatchTimeSeconds:  detail.WatchTime,
				RetentionRate:     detail.Retention,
				EngagementRate:    metrics.EngagementRate,
				AvgViewPercentage: detail.AvgViewPct,
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
	return normalizePublishStatus(resp.Status), ""
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
