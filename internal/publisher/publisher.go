package publisher

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
)

const platformYouTube = "youtube"

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
		 WHERE c.status = 'ready' AND c.publish_date <= CURRENT_DATE
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

		if video169 == nil {
			continue
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

		// Post 16:9 (YouTube regular)
		// Zernio uses Content's first line as YouTube title
		result169, err := p.zernio.Post(ctx, PostRequest{
			Title:      title,
			Content:    title + "\n\n" + desc,
			Platforms:  platforms,
			MediaItems: []MediaItem{{Type: "video", URL: *video169}},
			Visibility: VisibilityPrivate,
			PublishNow: true,
		})
		if err != nil {
			log.Printf("Failed to post 16:9 for clip %s: %v", clipID, err)
			continue
		}
		log.Printf("Posted 16:9 private for clip %s → %s", clipID, result169.Post.ID)

		// Post 9:16 (YouTube Shorts)
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
				Visibility: VisibilityPrivate,
				PublishNow: true,
			})
			if err != nil {
				log.Printf("Failed to post 9:16 for clip %s: %v", clipID, err)
			} else {
				log.Printf("Posted 9:16 Shorts private for clip %s → %s", clipID, result916.Post.ID)
				p.pool.Exec(ctx,
					`UPDATE clip_metadata SET zernio_shorts_post_id = $2 WHERE clip_id = $1`,
					clipID, result916.Post.ID)
			}
		}

		status := "published"
		p.clipsRepo.Update(ctx, clipID, models.UpdateClipRequest{Status: &status})

		p.pool.Exec(ctx,
			`UPDATE clip_metadata SET zernio_post_id = $2 WHERE clip_id = $1`,
			clipID, result169.Post.ID)

		log.Printf("Published clip %s via Zernio", clipID)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate ready clips: %w", err)
	}
	return nil
}

type postRef struct {
	id    string
	label string
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
		`SELECT cm.clip_id, cm.zernio_post_id, cm.zernio_shorts_post_id
		 FROM clip_metadata cm
		 JOIN clips c ON c.id = cm.clip_id
		 WHERE c.status = 'published'
		   AND (cm.zernio_post_id IS NOT NULL OR cm.zernio_shorts_post_id IS NOT NULL)`)
	if err != nil {
		return fmt.Errorf("query published clips: %w", err)
	}
	defer rows.Close()

	type clipPost struct {
		ClipID       string
		PostID       *string
		ShortsPostID *string
	}
	var clips []clipPost
	for rows.Next() {
		var cp clipPost
		if err := rows.Scan(&cp.ClipID, &cp.PostID, &cp.ShortsPostID); err != nil {
			return fmt.Errorf("scan clip metadata: %w", err)
		}
		clips = append(clips, cp)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate published clips: %w", err)
	}
	log.Printf("FetchAnalytics: %d clips with zernio IDs / %d total published", len(clips), totalPublished)

	var success, failed, dbFailed int
	for _, cp := range clips {
		var posts []postRef
		if cp.PostID != nil && *cp.PostID != "" {
			posts = append(posts, postRef{*cp.PostID, "regular"})
		}
		if cp.ShortsPostID != nil && *cp.ShortsPostID != "" {
			posts = append(posts, postRef{*cp.ShortsPostID, "shorts"})
		}
		for _, post := range posts {
			for _, platform := range platforms {
				resp, err := p.zernio.GetAnalytics(ctx, post.id, platform)
				if err != nil {
					log.Printf("FetchAnalytics FAIL clip=%s type=%s platform=%s post=%s: %v", cp.ClipID, post.label, platform, post.id, err)
					failed++
					continue
				}
				var metrics PostMetrics
				found := false
				for _, pa := range resp.PlatformAnalytics {
					if pa.Platform == platform {
						metrics = pa.Analytics
						found = true
						break
					}
				}
				if !found {
					log.Printf("FetchAnalytics NO_PLATFORM_DATA clip=%s platform=%s post=%s syncStatus=%s", cp.ClipID, platform, post.id, resp.SyncStatus)
					failed++
					continue
				}
				watchTime, retention := 0.0, 0.0
				if platform == platformYouTube && metrics.Views > 0 {
					watchTime, retention = p.fetchYouTubeWatchTime(ctx, cp.ClipID, ytAccountID, resp)
				}
				if err := p.analytics.Create(ctx, models.ClipAnalytics{
					ClipID:           cp.ClipID,
					Platform:         platform,
					PostType:         post.label,
					Views:            metrics.Views,
					Likes:            metrics.Likes,
					Comments:         metrics.Comments,
					Shares:           metrics.Shares,
					WatchTimeSeconds: watchTime,
					RetentionRate:    retention,
				}); err != nil {
					log.Printf("FetchAnalytics DB_FAIL clip=%s platform=%s: %v", cp.ClipID, platform, err)
					dbFailed++
					continue
				}
				success++
			}
		}
	}
	log.Printf("FetchAnalytics done: %d clips, %d success, %d api_fail, %d db_fail", len(clips), success, failed, dbFailed)
	return nil
}

func (p *Publisher) fetchYouTubeWatchTime(ctx context.Context, clipID, ytAccountID string, resp *AnalyticsResponse) (watchTime, retention float64) {
	if ytAccountID == "" {
		return 0, 0
	}
	var videoID string
	for _, pa := range resp.PlatformAnalytics {
		if pa.Platform == platformYouTube {
			videoID = pa.PlatformPostID
			break
		}
	}
	if videoID == "" {
		return 0, 0
	}
	daily, err := p.zernio.GetYouTubeDailyViews(ctx, videoID, ytAccountID)
	if err != nil {
		if errors.Is(err, ErrYouTubeScopeMissing) {
			log.Printf("FetchAnalytics: YouTube analytics scope missing — re-auth needed (skipping watchTime)")
		} else {
			log.Printf("FetchAnalytics WATCHTIME_FAIL clip=%s video=%s: %v", clipID, videoID, err)
		}
		return 0, 0
	}
	if len(daily.DailyViews) == 0 {
		return 0, 0
	}
	var totalMinutes, avgDurSum float64
	for _, dv := range daily.DailyViews {
		totalMinutes += dv.EstimatedMinutesWatched
		avgDurSum += dv.AverageViewDuration
	}
	watchTime = totalMinutes * 60
	retention = (avgDurSum / float64(len(daily.DailyViews))) / 60.0
	if retention > 1.0 {
		retention = 1.0
	}
	return watchTime, retention
}
