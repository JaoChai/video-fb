package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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
	embedFlag := flag.Bool("embed", false, "Rebuild embeddings for all knowledge sources")
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

	if err := database.RunMigrations(ctx, pool, "migrations"); err != nil {
		log.Printf("Auto-migration warning: %v", err)
	}

	if *migrateFlag {
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

	if *embedFlag {
		if err := rebuildAllEmbeddings(ctx, pool, ragEngine); err != nil {
			log.Fatalf("Embed failed: %v", err)
		}
		return
	}

	llm := agent.NewLLMClient(pool)
	questionAgent := agent.NewQuestionAgent(llm, ragEngine, pool)
	scriptAgent := agent.NewScriptAgent(llm, ragEngine)
	imageAgent := agent.NewImageAgent(llm)

	kie := producer.NewKieClient(pool)
	ffmpeg := producer.NewFFmpegAssembler(cfg.FFmpegPath, "/tmp/fonts/NotoSansThai-Bold.ttf")
	tracker := progress.NewTracker()
	orClient := producer.NewOpenRouterClient(pool)
	prod := producer.NewProducer(pool, kie, orClient, ffmpeg, cfg.ElevenLabsVoice, "/tmp/adsvance-output", tracker)

	clipsRepo := repository.NewClipsRepo(pool)
	scenesRepo := repository.NewScenesRepo(pool)
	themesRepo := repository.NewThemesRepo(pool)
	agentsRepo := repository.NewAgentsRepo(pool)
	analyticsRepo := repository.NewAnalyticsRepo(pool)

	orch := orchestrator.New(pool, questionAgent, scriptAgent, imageAgent, prod,
		clipsRepo, scenesRepo, themesRepo, agentsRepo, tracker)

	zernio := publisher.NewZernioClient(cfg.ZernioAPIKey, pool)
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
	sched := scheduler.New(pool, pub, anlz, orch, crawl, schedRepo, clipsRepo)
	if err := sched.Start(ctx); err != nil {
		log.Printf("Warning: scheduler start failed: %v", err)
	}

	r := router.New(pool, cfg.APIKey, ragEngine, tracker)
	orchHandler := handler.NewOrchestratorHandler(orch, tracker, pub, clipsRepo)
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

func rebuildAllEmbeddings(ctx context.Context, pool *pgxpool.Pool, engine *rag.Engine) error {
	rows, err := pool.Query(ctx, `SELECT id, name, content FROM knowledge_sources WHERE enabled = true ORDER BY name`)
	if err != nil {
		return fmt.Errorf("query sources: %w", err)
	}
	defer rows.Close()

	type source struct{ ID, Name, Content string }
	var sources []source
	for rows.Next() {
		var s source
		rows.Scan(&s.ID, &s.Name, &s.Content)
		sources = append(sources, s)
	}

	log.Printf("Embedding %d knowledge sources...", len(sources))
	for i, s := range sources {
		log.Printf("[%d/%d] %s (%d chars)", i+1, len(sources), s.Name, len(s.Content))

		pool.Exec(ctx, `DELETE FROM knowledge_chunks WHERE source_id = $1`, s.ID)

		chunks := rag.ChunkText(s.Content, 200, 30)
		stored := 0
		for _, chunk := range chunks {
			if len(strings.Fields(chunk)) < 10 {
				continue
			}
			embedding, err := engine.GenerateEmbedding(ctx, chunk)
			if err != nil {
				log.Printf("  ERROR embedding chunk: %v", err)
				continue
			}
			if err := engine.StoreChunk(ctx, s.ID, chunk, "", embedding); err != nil {
				log.Printf("  ERROR storing chunk: %v", err)
				continue
			}
			stored++
		}
		log.Printf("  → %d chunks stored", stored)
	}
	log.Println("All embeddings complete")
	return nil
}
