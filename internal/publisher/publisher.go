package publisher

import (
	"context"
	"fmt"
	"log"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
)

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
		result169, err := p.zernio.Post(ctx, PostRequest{
			Title:      title,
			Content:    desc,
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
				Content:    desc,
				Platforms:  platforms,
				MediaItems: []MediaItem{{Type: "video", URL: *video916}},
				Visibility: VisibilityPrivate,
				PublishNow: true,
			})
			if err != nil {
				log.Printf("Failed to post 9:16 for clip %s: %v", clipID, err)
			} else {
				log.Printf("Posted 9:16 Shorts private for clip %s → %s", clipID, result916.Post.ID)
			}
		}

		status := "published"
		p.clipsRepo.Update(ctx, clipID, models.UpdateClipRequest{Status: &status})

		p.pool.Exec(ctx,
			`UPDATE clip_metadata SET zernio_post_id = $2 WHERE clip_id = $1`,
			clipID, result169.Post.ID)

		log.Printf("Published clip %s via Zernio", clipID)
	}
	return nil
}

func (p *Publisher) FetchAnalytics(ctx context.Context) error {
	rows, err := p.pool.Query(ctx,
		`SELECT cm.clip_id, cm.zernio_post_id
		 FROM clip_metadata cm
		 JOIN clips c ON c.id = cm.clip_id
		 WHERE c.status = 'published' AND cm.zernio_post_id IS NOT NULL`)
	if err != nil {
		return fmt.Errorf("query published clips: %w", err)
	}
	defer rows.Close()

	type clipPost struct{ ClipID, PostID string }
	var clips []clipPost
	for rows.Next() {
		var cp clipPost
		rows.Scan(&cp.ClipID, &cp.PostID)
		clips = append(clips, cp)
	}

	platforms := []string{"youtube", "tiktok", "instagram", "facebook"}
	for _, cp := range clips {
		for _, platform := range platforms {
			stats, err := p.zernio.GetAnalytics(ctx, cp.PostID, platform)
			if err != nil {
				log.Printf("Analytics failed for %s/%s: %v", cp.ClipID, platform, err)
				continue
			}
			p.analytics.Create(ctx, models.ClipAnalytics{
				ClipID:           cp.ClipID,
				Platform:         platform,
				Views:            stats.Views,
				Likes:            stats.Likes,
				Comments:         stats.Comments,
				Shares:           stats.Shares,
				WatchTimeSeconds: stats.WatchTimeSeconds,
				RetentionRate:    stats.RetentionRate,
			})
		}
	}
	log.Printf("Fetched analytics for %d clips", len(clips))
	return nil
}
