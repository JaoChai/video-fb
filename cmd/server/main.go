package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jaochai/video-fb/internal/agent"
	"github.com/jaochai/video-fb/internal/analyzer"
	"github.com/jaochai/video-fb/internal/config"
	"github.com/jaochai/video-fb/internal/crawler"
	"github.com/jaochai/video-fb/internal/database"
	"github.com/jaochai/video-fb/internal/handler"
	"github.com/jaochai/video-fb/internal/orchestrator"
	"github.com/jaochai/video-fb/internal/producer"
	"github.com/jaochai/video-fb/internal/progress"
	"github.com/jaochai/video-fb/internal/publisher"
	"github.com/jaochai/video-fb/internal/rag"
	"github.com/jaochai/video-fb/internal/repository"
	"github.com/jaochai/video-fb/internal/router"
	"github.com/jaochai/video-fb/internal/scheduler"
)

func main() {
	migrateFlag := flag.Bool("migrate", false, "Run database migrations")
	crawlFlag := flag.Bool("crawl", false, "Run knowledge crawler")
	produceFlag := flag.Int("produce", 0, "Produce N clips")
	publishFlag := flag.Bool("publish", false, "Publish ready clips")
	analyticsFlag := flag.Bool("analytics", false, "Fetch analytics for published clips")
	flag.Parse()

	cfg := config.Load()
	ctx := context.Background()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()
	log.Println("Connected to database")

	if *migrateFlag {
		if err := database.RunMigrations(ctx, pool, "migrations"); err != nil {
			log.Fatalf("Migrations failed: %v", err)
		}
		log.Println("Migrations complete")
		return
	}

	ragEngine := rag.NewEngine(pool)
	crawl := crawler.NewCrawler(pool, ragEngine)

	if *crawlFlag {
		if err := crawl.CrawlAll(ctx); err != nil {
			log.Fatalf("Crawl failed: %v", err)
		}
		return
	}

	llm := agent.NewLLMClient(pool)
	questionAgent := agent.NewQuestionAgent(llm, ragEngine, pool)
	scriptAgent := agent.NewScriptAgent(llm, ragEngine)
	imageAgent := agent.NewImageAgent(llm)

	kie := producer.NewKieClient(pool)
	ffmpeg := producer.NewFFmpegAssembler(cfg.FFmpegPath, "/tmp/fonts/NotoSansThai-Bold.ttf")
	voice := cfg.ElevenLabsVoice
	if voice == "" {
		var dbVoice string
		if err := pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = 'elevenlabs_voice'`).Scan(&dbVoice); err == nil && dbVoice != "" {
			voice = dbVoice
		} else {
			voice = "Adam"
		}
	}
	tracker := progress.NewTracker()
	prod := producer.NewProducer(kie, ffmpeg, voice, "/tmp/adsvance-output", tracker)

	clipsRepo := repository.NewClipsRepo(pool)
	scenesRepo := repository.NewScenesRepo(pool)
	themesRepo := repository.NewThemesRepo(pool)
	agentsRepo := repository.NewAgentsRepo(pool)
	analyticsRepo := repository.NewAnalyticsRepo(pool)

	orch := orchestrator.New(pool, questionAgent, scriptAgent, imageAgent, prod,
		clipsRepo, scenesRepo, themesRepo, agentsRepo, tracker)

	zernio := publisher.NewZernioClient(cfg.ZernioAPIKey)
	pub := publisher.NewPublisher(zernio, pool, clipsRepo, analyticsRepo)

	if *produceFlag > 0 {
		if err := orch.ProduceWeekly(ctx, *produceFlag); err != nil {
			log.Fatalf("Production failed: %v", err)
		}
		return
	}

	if *publishFlag {
		if err := pub.PublishReady(ctx); err != nil {
			log.Fatalf("Publish failed: %v", err)
		}
		return
	}

	if *analyticsFlag {
		if err := pub.FetchAnalytics(ctx); err != nil {
			log.Fatalf("Analytics failed: %v", err)
		}
		return
	}

	anlz := analyzer.New(pool, llm, agentsRepo)
	schedRepo := repository.NewSchedulesRepo(pool)
	sched := scheduler.New(pool, pub, anlz, schedRepo)
	if err := sched.Start(ctx); err != nil {
		log.Printf("Warning: scheduler start failed: %v", err)
	}

	r := router.New(pool, cfg.APIKey, ragEngine, tracker)
	orchHandler := handler.NewOrchestratorHandler(orch, tracker)
	router.SetOrchestrator(r, orchHandler)

	addr := ":" + cfg.Port
	srv := &http.Server{Addr: addr, Handler: r}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down...")
		sched.Stop()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	log.Printf("Starting server on %s", addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
