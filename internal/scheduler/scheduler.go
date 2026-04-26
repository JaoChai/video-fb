package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/jaochai/video-fb/internal/crawler"
	"github.com/jaochai/video-fb/internal/orchestrator"
	"github.com/jaochai/video-fb/internal/publisher"
)

type Scheduler struct {
	orch      *orchestrator.Orchestrator
	publisher *publisher.Publisher
	crawler   *crawler.Crawler
}

func New(orch *orchestrator.Orchestrator, pub *publisher.Publisher, crawl *crawler.Crawler) *Scheduler {
	return &Scheduler{orch: orch, publisher: pub, crawler: crawl}
}

func (s *Scheduler) Start(ctx context.Context) {
	log.Println("Scheduler started")

	go s.runLoop(ctx, "daily-publish", 24*time.Hour, func(ctx context.Context) {
		if err := s.publisher.PublishReady(ctx); err != nil {
			log.Printf("Daily publish failed: %v", err)
		}
	})

	go s.runLoop(ctx, "weekly-produce", 7*24*time.Hour, func(ctx context.Context) {
		if err := s.orch.ProduceWeekly(ctx, 7); err != nil {
			log.Printf("Weekly production failed: %v", err)
		}
	})

	go s.runLoop(ctx, "weekly-crawl", 7*24*time.Hour, func(ctx context.Context) {
		if err := s.crawler.CrawlAll(ctx); err != nil {
			log.Printf("Weekly crawl failed: %v", err)
		}
	})

	go s.runLoop(ctx, "weekly-analytics", 7*24*time.Hour, func(ctx context.Context) {
		if err := s.publisher.FetchAnalytics(ctx); err != nil {
			log.Printf("Weekly analytics failed: %v", err)
		}
	})
}

func (s *Scheduler) runLoop(ctx context.Context, name string, interval time.Duration, fn func(context.Context)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("Scheduler [%s]: running every %v", name, interval)
	for {
		select {
		case <-ctx.Done():
			log.Printf("Scheduler [%s]: stopped", name)
			return
		case <-ticker.C:
			log.Printf("Scheduler [%s]: executing", name)
			fn(ctx)
		}
	}
}
