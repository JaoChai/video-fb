package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/agent"
	"github.com/jaochai/video-fb/internal/analyzer"
	"github.com/jaochai/video-fb/internal/config"
	"github.com/jaochai/video-fb/internal/database"
	"github.com/jaochai/video-fb/internal/handler"
	"github.com/jaochai/video-fb/internal/learner"
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
	embedFlag := flag.Bool("embed", false, "Rebuild embeddings for all knowledge sources")
	embedTopicsFlag := flag.Bool("embed-topics", false, "Backfill embeddings for topic_history rows that lack them")
	produceFlag := flag.Int("produce", 0, "Produce N clips")
	publishFlag := flag.Bool("publish", false, "Publish ready clips")
	analyticsFlag := flag.Bool("analytics", false, "Fetch analytics for published clips")
	learnFlag := flag.Bool("learn", false, "Run the learning loop once (auto-tune upstream agent skills from critiques)")
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

	// Production runs as a detached goroutine, so a restart mid-production orphans
	// its clip in 'producing' forever. Any 'producing' clip at boot is stale — fail
	// it so it shows up as retryable instead of stuck.
	if n, err := repository.NewClipsRepo(pool).ResetStaleProducing(ctx); err != nil {
		log.Printf("Reset stale producing clips warning: %v", err)
	} else if n > 0 {
		log.Printf("Reset %d stale 'producing' clip(s) to 'failed' (interrupted by restart)", n)
	}

	ragEngine := rag.NewEngine(pool)

	if *embedFlag {
		if err := rebuildAllEmbeddings(ctx, pool, ragEngine); err != nil {
			log.Fatalf("Embed failed: %v", err)
		}
		return
	}

	if *embedTopicsFlag {
		if err := backfillTopicEmbeddings(ctx, pool, ragEngine); err != nil {
			log.Fatalf("Embed topics failed: %v", err)
		}
		return
	}

	llm := agent.NewKieLLMClient(pool)
	agentsRepo := repository.NewAgentsRepo(pool)
	researchAgent := agent.NewResearchAgent(llm, agentsRepo)
	questionAgent := agent.NewQuestionAgent(llm, ragEngine, pool, researchAgent)
	scriptAgent := agent.NewScriptAgent(llm, ragEngine, researchAgent)
	imageAgent := agent.NewImageAgent(llm)
	sceneAgent := agent.NewSceneAgent(llm)
	criticAgent := agent.NewCriticAgent(llm)
	visualQAAgent := agent.NewVisualQAAgent(llm)
	autoReviewAgent := agent.NewAutoReviewAgent(llm)

	kie := producer.NewKieClient(pool, producer.DefaultKieConfig())
	r2 := producer.NewR2Client(pool)
	ffmpeg := producer.NewFFmpegAssembler(cfg.FFmpegPath, "/tmp/fonts/NotoSansThai-Bold.ttf")
	tracker := progress.NewTracker()
	orClient := producer.NewOpenRouterClient(pool)
	prod := producer.NewProducer(pool, kie, r2, orClient, ffmpeg, cfg.ElevenLabsVoice, "/tmp/adsvance-output", tracker)

	fontsDir := os.Getenv("FONTS_DIR")
	if fontsDir == "" {
		fontsDir = "internal/producer/assets/fonts"
	}
	if abs, err := filepath.Abs(fontsDir); err == nil {
		fontsDir = abs
	}
	prod.EnableHyperframes(fontsDir)

	clipsRepo := repository.NewClipsRepo(pool)
	scenesRepo := repository.NewScenesRepo(pool)
	critiquesRepo := repository.NewCritiquesRepo(pool)
	visualQARepo := repository.NewVisualQARepo(pool)
	autoReviewsRepo := repository.NewAutoReviewsRepo(pool)
	skillRevisionsRepo := repository.NewSkillRevisionsRepo(pool)
	learnerAgent := agent.NewLearnerAgent(llm)
	learnerSvc := learner.New(agentsRepo, critiquesRepo, learnerAgent, skillRevisionsRepo)
	themesRepo := repository.NewThemesRepo(pool)
	analyticsRepo := repository.NewAnalyticsRepo(pool)
	settingsRepo := repository.NewSettingsRepo(pool)
	formatsRepo := repository.NewFormatsRepo(pool)

	orch := orchestrator.New(questionAgent, scriptAgent, imageAgent, sceneAgent, criticAgent, visualQAAgent, autoReviewAgent, prod,
		clipsRepo, scenesRepo, critiquesRepo, visualQARepo, autoReviewsRepo, themesRepo, agentsRepo, analyticsRepo, settingsRepo, formatsRepo, tracker)

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

	if *learnFlag {
		if err := learnerSvc.RunOnce(ctx); err != nil {
			log.Fatalf("Learning loop failed: %v", err)
		}
		return
	}

	anlz := analyzer.New(pool, llm, agentsRepo)
	schedRepo := repository.NewSchedulesRepo(pool)
	sched := scheduler.New(pool, pub, anlz, orch, schedRepo, clipsRepo, learnerSvc)
	if err := sched.Start(ctx); err != nil {
		log.Printf("Warning: scheduler start failed: %v", err)
	}

	r := router.New(pool, cfg.APIKey, ragEngine, tracker, pub, func() {
		if err := sched.Reload(ctx); err != nil {
			log.Printf("Scheduler reload failed: %v", err)
		}
	}, prod)
	orchHandler := handler.NewOrchestratorHandler(orch, tracker, pub)
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

// backfillTopicEmbeddings generates embeddings for topic_history rows that lack
// them so semantic dedup can compare new questions against old topics too.
func backfillTopicEmbeddings(ctx context.Context, pool *pgxpool.Pool, engine *rag.Engine) error {
	rows, err := pool.Query(ctx, `SELECT id, title FROM topic_history WHERE embedding IS NULL ORDER BY created_at`)
	if err != nil {
		return fmt.Errorf("query topics: %w", err)
	}
	defer rows.Close()

	type topic struct{ ID, Title string }
	var topics []topic
	for rows.Next() {
		var t topic
		if err := rows.Scan(&t.ID, &t.Title); err != nil {
			return fmt.Errorf("scan topic: %w", err)
		}
		topics = append(topics, t)
	}

	log.Printf("Backfilling embeddings for %d topics...", len(topics))
	done := 0
	for i, t := range topics {
		embedding, err := engine.GenerateEmbedding(ctx, t.Title)
		if err != nil {
			log.Printf("[%d/%d] ERROR embedding %q: %v", i+1, len(topics), t.Title, err)
			continue
		}
		if _, err := pool.Exec(ctx,
			`UPDATE topic_history SET embedding = $2::vector WHERE id = $1`,
			t.ID, rag.FormatVector(embedding)); err != nil {
			log.Printf("[%d/%d] ERROR updating %q: %v", i+1, len(topics), t.Title, err)
			continue
		}
		done++
	}
	log.Printf("Topic embeddings complete: %d/%d", done, len(topics))
	return nil
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
