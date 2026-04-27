package publisher

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
)

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
		var clipID, title, desc string
		var video169, video916, thumb *string
		if err := rows.Scan(&clipID, &title, &desc, &video169, &video916, &thumb); err != nil {
			return fmt.Errorf("scan clip: %w", err)
		}

		if video169 == nil {
			continue
		}

		var platforms []PlatformTarget
		if ytAccountID != "" {
			platforms = append(platforms, PlatformTarget{Platform: "youtube", AccountID: ytAccountID})
		}
		if len(platforms) == 0 {
			log.Printf("No Zernio accounts configured, skipping clip %s", clipID)
			continue
		}

		result, err := p.zernio.Post(ctx, PostRequest{
			Content:    title + "\n\n" + desc,
			Platforms:  platforms,
			MediaURLs:  []string{*video169},
			PublishNow: true,
		})
		if err != nil {
			log.Printf("Failed to publish clip %s: %v", clipID, err)
			continue
		}

		status := "published"
		p.clipsRepo.Update(ctx, clipID, models.UpdateClipRequest{Status: &status})

		p.pool.Exec(ctx,
			`UPDATE clip_metadata SET zernio_post_id = $2 WHERE clip_id = $1`,
			clipID, result.ID)

		if yt, ok := result.Platforms["youtube"]; ok {
			p.pool.Exec(ctx, `UPDATE clip_metadata SET youtube_video_id = $2 WHERE clip_id = $1`, clipID, yt.PostID)
		}
		if tt, ok := result.Platforms["tiktok"]; ok {
			p.pool.Exec(ctx, `UPDATE clip_metadata SET tiktok_post_id = $2 WHERE clip_id = $1`, clipID, tt.PostID)
		}
		if ig, ok := result.Platforms["instagram"]; ok {
			p.pool.Exec(ctx, `UPDATE clip_metadata SET ig_post_id = $2 WHERE clip_id = $1`, clipID, ig.PostID)
		}
		if fb, ok := result.Platforms["facebook"]; ok {
			p.pool.Exec(ctx, `UPDATE clip_metadata SET fb_post_id = $2 WHERE clip_id = $1`, clipID, fb.PostID)
		}

		log.Printf("Published clip %s via Zernio → %s", clipID, result.ID)
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
