package scheduler

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/analyzer"
	"github.com/jaochai/video-fb/internal/crawler"
	"github.com/jaochai/video-fb/internal/orchestrator"
	"github.com/jaochai/video-fb/internal/preflight"
	"github.com/jaochai/video-fb/internal/publisher"
	"github.com/jaochai/video-fb/internal/repository"
	"github.com/robfig/cron/v3"
)

const (
	maxClipRetries      = 2
	circuitBreakerLimit = 3
)

type Scheduler struct {
	cron          *cron.Cron
	pool          *pgxpool.Pool
	publisher     *publisher.Publisher
	analyzer      *analyzer.Analyzer
	orchestrator  *orchestrator.Orchestrator
	crawler       *crawler.Crawler
	schedulesRepo *repository.SchedulesRepo
	clipsRepo     *repository.ClipsRepo
}

func New(pool *pgxpool.Pool, pub *publisher.Publisher, anlz *analyzer.Analyzer, orch *orchestrator.Orchestrator, crawl *crawler.Crawler, schedRepo *repository.SchedulesRepo, clipsRepo *repository.ClipsRepo) *Scheduler {
	loc, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		log.Printf("Scheduler: failed to load Asia/Bangkok, using UTC: %v", err)
		return &Scheduler{
			cron:          cron.New(),
			pool:          pool,
			publisher:     pub,
			analyzer:      anlz,
			orchestrator:  orch,
			crawler:       crawl,
			schedulesRepo: schedRepo,
			clipsRepo:     clipsRepo,
		}
	}
	return &Scheduler{
		cron:          cron.New(cron.WithLocation(loc)),
		pool:          pool,
		publisher:     pub,
		analyzer:      anlz,
		orchestrator:  orch,
		crawler:       crawl,
		schedulesRepo: schedRepo,
		clipsRepo:     clipsRepo,
	}
}

func (s *Scheduler) Start(ctx context.Context) error {
	schedules, err := s.schedulesRepo.ListEnabled(ctx)
	if err != nil {
		return fmt.Errorf("load schedules: %w", err)
	}

	for _, sched := range schedules {
		schedule := sched
		handler := s.handlerFor(schedule.Action)
		if handler == nil {
			log.Printf("Scheduler [%s]: unknown action %q, skipping", schedule.Name, schedule.Action)
			continue
		}

		_, err := s.cron.AddFunc(schedule.CronExpression, func() {
			log.Printf("Scheduler [%s]: executing", schedule.Name)
			if err := handler(ctx); err != nil {
				log.Printf("Scheduler [%s]: failed: %v", schedule.Name, err)
			} else {
				log.Printf("Scheduler [%s]: completed", schedule.Name)
			}
			if err := s.schedulesRepo.UpdateLastRun(ctx, schedule.ID); err != nil {
				log.Printf("Scheduler [%s]: failed to update last_run: %v", schedule.Name, err)
			}
		})
		if err != nil {
			log.Printf("Scheduler: invalid cron %q for %q: %v", schedule.CronExpression, schedule.Name, err)
			continue
		}

		log.Printf("Scheduler [%s]: registered cron %q", schedule.Name, schedule.CronExpression)
	}

	s.cron.Start()
	log.Printf("Scheduler started with %d jobs", len(s.cron.Entries()))
	return nil
}

func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
	log.Println("Scheduler stopped")
}

func (s *Scheduler) produceAndPublish(ctx context.Context) error {
	check := preflight.Run(ctx, s.pool)
	if !check.OK {
		for _, e := range check.Errors {
			log.Printf("Scheduler PRE-FLIGHT FAIL: %s", e)
		}
		return fmt.Errorf("pre-flight failed: %v", check.Errors)
	}

	failCount, err := s.clipsRepo.CountConsecutiveFailed(ctx)
	if err != nil {
		log.Printf("Scheduler: circuit breaker check error: %v", err)
	} else if failCount >= circuitBreakerLimit {
		log.Printf("Scheduler CIRCUIT BREAKER: %d of last 5 clips failed, skipping production", failCount)
		return fmt.Errorf("circuit breaker open: %d consecutive failures", failCount)
	}

	failed, err := s.clipsRepo.ListFailed(ctx, maxClipRetries)
	if err != nil {
		log.Printf("Scheduler: list failed clips error: %v", err)
	}
	for _, clip := range failed {
		log.Printf("Scheduler: retrying failed clip %s (attempt %d)", clip.ID, clip.RetryCount+1)
		if err := s.orchestrator.RetryClip(ctx, &clip); err != nil {
			log.Printf("Scheduler: retry clip %s failed again: %v", clip.ID, err)
		} else {
			log.Printf("Scheduler: retry clip %s succeeded!", clip.ID)
		}
	}

	log.Println("Scheduler: producing 1 new clip...")
	if err := s.orchestrator.ProduceWeekly(ctx, 1); err != nil {
		return fmt.Errorf("produce: %w", err)
	}

	log.Println("Scheduler: publishing ready clips...")
	if err := s.publisher.PublishReady(ctx); err != nil {
		return fmt.Errorf("publish: %w", err)
	}

	deleted, err := s.clipsRepo.DeleteOldFailed(ctx, maxClipRetries)
	if err != nil {
		log.Printf("Scheduler: cleanup error: %v", err)
	} else if deleted > 0 {
		log.Printf("Scheduler: cleaned up %d unrecoverable clips", deleted)
	}

	return nil
}

func (s *Scheduler) handlerFor(action string) func(context.Context) error {
	switch action {
	case "publish_daily":
		return s.publisher.PublishReady
	case "produce_and_publish":
		return s.produceAndPublish
	case "analyze_and_improve":
		return s.analyzer.AnalyzeAndImprove
	case "fetch_analytics":
		return s.publisher.FetchAnalytics
	case "crawl_knowledge":
		return s.crawler.CrawlAll
	default:
		return nil
	}
}
