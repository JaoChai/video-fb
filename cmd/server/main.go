package main

import (
	"context"
	"flag"
	"log"
	"net/http"

	"github.com/jaochai/video-fb/internal/agent"
	"github.com/jaochai/video-fb/internal/config"
	"github.com/jaochai/video-fb/internal/crawler"
	"github.com/jaochai/video-fb/internal/database"
	"github.com/jaochai/video-fb/internal/handler"
	"github.com/jaochai/video-fb/internal/orchestrator"
	"github.com/jaochai/video-fb/internal/producer"
	"github.com/jaochai/video-fb/internal/rag"
	"github.com/jaochai/video-fb/internal/repository"
	"github.com/jaochai/video-fb/internal/router"
)

func main() {
	migrateFlag := flag.Bool("migrate", false, "Run database migrations")
	crawlFlag := flag.Bool("crawl", false, "Run knowledge crawler")
	produceFlag := flag.Int("produce", 0, "Produce N clips")
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

	ragEngine := rag.NewEngine(pool, cfg.ClaudeAPIKey)

	if *crawlFlag {
		c := crawler.NewCrawler(pool, ragEngine)
		if err := c.CrawlAll(ctx); err != nil {
			log.Fatalf("Crawl failed: %v", err)
		}
		return
	}

	claude := agent.NewClaudeClient(cfg.ClaudeAPIKey, "claude-sonnet-4-6-20250514")
	questionAgent := agent.NewQuestionAgent(claude, ragEngine, pool)
	scriptAgent := agent.NewScriptAgent(claude, ragEngine)
	imageAgent := agent.NewImageAgent(claude)

	kie := producer.NewKieClient(cfg.KieAPIKey)
	ffmpeg := producer.NewFFmpegAssembler(cfg.FFmpegPath, "/tmp/fonts/NotoSansThai-Bold.ttf")
	prod := producer.NewProducer(kie, ffmpeg, cfg.ElevenLabsVoice, "/tmp/adsvance-output")

	clipsRepo := repository.NewClipsRepo(pool)
	scenesRepo := repository.NewScenesRepo(pool)
	themesRepo := repository.NewThemesRepo(pool)
	agentsRepo := repository.NewAgentsRepo(pool)

	orch := orchestrator.New(pool, questionAgent, scriptAgent, imageAgent, prod,
		clipsRepo, scenesRepo, themesRepo, agentsRepo)

	if *produceFlag > 0 {
		if err := orch.ProduceWeekly(ctx, *produceFlag); err != nil {
			log.Fatalf("Production failed: %v", err)
		}
		return
	}

	r := router.New(pool, cfg.APIKey)
	orchHandler := handler.NewOrchestratorHandler(orch)
	router.SetOrchestrator(r, orchHandler)

	addr := ":" + cfg.Port
	log.Printf("Starting server on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal(err)
	}
}
