package scheduler

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/analyzer"
	"github.com/jaochai/video-fb/internal/publisher"
	"github.com/jaochai/video-fb/internal/repository"
	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	cron          *cron.Cron
	pool          *pgxpool.Pool
	publisher     *publisher.Publisher
	analyzer      *analyzer.Analyzer
	schedulesRepo *repository.SchedulesRepo
}

func New(pool *pgxpool.Pool, pub *publisher.Publisher, anlz *analyzer.Analyzer, schedRepo *repository.SchedulesRepo) *Scheduler {
	loc, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		log.Printf("Scheduler: failed to load Asia/Bangkok, using UTC: %v", err)
		return &Scheduler{
			cron:          cron.New(),
			pool:          pool,
			publisher:     pub,
			analyzer:      anlz,
			schedulesRepo: schedRepo,
		}
	}
	return &Scheduler{
		cron:          cron.New(cron.WithLocation(loc)),
		pool:          pool,
		publisher:     pub,
		analyzer:      anlz,
		schedulesRepo: schedRepo,
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

func (s *Scheduler) handlerFor(action string) func(context.Context) error {
	switch action {
	case "publish_daily":
		return s.publisher.PublishReady
	case "analyze_and_improve":
		return s.analyzer.AnalyzeAndImprove
	default:
		return nil
	}
}
