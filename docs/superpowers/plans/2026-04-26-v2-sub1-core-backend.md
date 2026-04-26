# Sub-Project 1: Core Backend + DB — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go REST API server with Neon PostgreSQL database, covering CRUD for all entities, deployed on Railway — the foundation for the entire Ads Vance Content Factory V2.

**Architecture:** Go HTTP server using chi router for REST endpoints, pgx for PostgreSQL, structured as a clean layered architecture (handler → service → repository). Database on Neon with pgvector extension. Deployed to Railway via Dockerfile.

**Tech Stack:** Go 1.25, chi (router), pgx v5 (PostgreSQL), Neon PostgreSQL, Railway, Docker.

---

## File Structure

```
video-fb/
├── cmd/
│   └── server/
│       └── main.go                 # Entry point, wire dependencies, start server
├── internal/
│   ├── config/
│   │   └── config.go              # Env-based config loading
│   ├── database/
│   │   ├── db.go                  # Connection pool setup
│   │   └── migrations.go         # SQL migration runner
│   ├── models/
│   │   └── models.go             # All domain structs
│   ├── repository/
│   │   ├── clips.go              # Clips CRUD
│   │   ├── scenes.go             # Scenes CRUD
│   │   ├── knowledge.go          # Knowledge sources + chunks CRUD
│   │   ├── agents.go             # Agent configs CRUD
│   │   ├── schedules.go          # Schedules CRUD
│   │   ├── themes.go             # Brand themes CRUD
│   │   ├── analytics.go          # Clip analytics CRUD
│   │   └── topics.go             # Topic history CRUD
│   ├── handler/
│   │   ├── clips.go              # Clips HTTP handlers
│   │   ├── scenes.go             # Scenes HTTP handlers
│   │   ├── knowledge.go          # Knowledge HTTP handlers
│   │   ├── agents.go             # Agent configs HTTP handlers
│   │   ├── schedules.go          # Schedules HTTP handlers
│   │   ├── themes.go             # Brand themes HTTP handlers
│   │   ├── analytics.go          # Analytics HTTP handlers
│   │   ├── topics.go             # Topic history HTTP handlers
│   │   ├── health.go             # Health check handler
│   │   └── middleware.go         # Auth + CORS + logging middleware
│   └── router/
│       └── router.go             # Route registration
├── migrations/
│   └── 001_initial_schema.sql    # Full DB schema
├── Dockerfile
├── .env.example
├── go.mod
├── go.sum
└── Makefile
```

---

## Task 1: Go Project Init + Config

**Files:**
- Create: `go.mod`
- Create: `cmd/server/main.go`
- Create: `internal/config/config.go`
- Create: `.env.example`
- Create: `Makefile`

- [ ] **Step 1: Initialize Go module**

Run:
```bash
cd /Users/jaochai/Code/video-fb && go mod init github.com/jaochai/video-fb
```

- [ ] **Step 2: Install dependencies**

Run:
```bash
go get github.com/go-chi/chi/v5@latest
go get github.com/go-chi/cors@latest
go get github.com/jackc/pgx/v5@latest
go get github.com/joho/godotenv@latest
```

- [ ] **Step 3: Create .env.example**

```env
DATABASE_URL=postgresql://user:pass@host/dbname?sslmode=require
PORT=8080
API_KEY=your-api-key-here
```

- [ ] **Step 4: Create internal/config/config.go**

```go
package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL string
	Port        string
	APIKey      string
}

func Load() *Config {
	godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return &Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Port:        port,
		APIKey:      os.Getenv("API_KEY"),
	}
}
```

- [ ] **Step 5: Create cmd/server/main.go (minimal)**

```go
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/jaochai/video-fb/internal/config"
)

func main() {
	cfg := config.Load()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	addr := ":" + cfg.Port
	log.Printf("Starting server on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Server stopped")
}
```

- [ ] **Step 6: Create Makefile**

```makefile
.PHONY: run build test migrate

run:
	go run cmd/server/main.go

build:
	go build -o bin/server cmd/server/main.go

test:
	go test ./... -v

migrate:
	go run cmd/server/main.go -migrate
```

- [ ] **Step 7: Verify it compiles and runs**

Run:
```bash
cd /Users/jaochai/Code/video-fb && go build ./... && PORT=8080 go run cmd/server/main.go &
sleep 1 && curl -s http://localhost:8080/health && kill %1
```
Expected: `{"status":"ok"}`

- [ ] **Step 8: Commit**

```bash
git add go.mod go.sum cmd/ internal/config/ .env.example Makefile
git commit -m "feat: Go project init with config and health check"
```

---

## Task 2: Database Connection + Migration Runner

**Files:**
- Create: `internal/database/db.go`
- Create: `internal/database/migrations.go`
- Create: `migrations/001_initial_schema.sql`

- [ ] **Step 1: Create internal/database/db.go**

```go
package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	config.MaxConns = 10

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
```

- [ ] **Step 2: Create migrations/001_initial_schema.sql**

```sql
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "vector";

-- Clips
CREATE TABLE IF NOT EXISTS clips (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title TEXT NOT NULL,
    question TEXT NOT NULL,
    questioner_name TEXT NOT NULL,
    answer_script TEXT NOT NULL DEFAULT '',
    voice_script TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL DEFAULT 'account',
    status TEXT NOT NULL DEFAULT 'draft',
    video_16_9_url TEXT,
    video_9_16_url TEXT,
    thumbnail_url TEXT,
    publish_date DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Scenes
CREATE TABLE IF NOT EXISTS scenes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clip_id UUID NOT NULL REFERENCES clips(id) ON DELETE CASCADE,
    scene_number INT NOT NULL,
    scene_type TEXT NOT NULL,
    text_content TEXT NOT NULL,
    image_prompt TEXT NOT NULL DEFAULT '',
    image_16_9_url TEXT,
    image_9_16_url TEXT,
    voice_text TEXT NOT NULL DEFAULT '',
    duration_seconds FLOAT NOT NULL DEFAULT 10,
    text_overlays JSONB NOT NULL DEFAULT '[]'
);

-- Clip Metadata
CREATE TABLE IF NOT EXISTS clip_metadata (
    clip_id UUID PRIMARY KEY REFERENCES clips(id) ON DELETE CASCADE,
    youtube_title TEXT,
    youtube_description TEXT,
    youtube_tags TEXT[],
    zernio_post_id TEXT,
    youtube_video_id TEXT,
    tiktok_post_id TEXT,
    ig_post_id TEXT,
    fb_post_id TEXT
);

-- Analytics
CREATE TABLE IF NOT EXISTS clip_analytics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clip_id UUID NOT NULL REFERENCES clips(id) ON DELETE CASCADE,
    platform TEXT NOT NULL,
    views INT NOT NULL DEFAULT 0,
    likes INT NOT NULL DEFAULT 0,
    comments INT NOT NULL DEFAULT 0,
    shares INT NOT NULL DEFAULT 0,
    watch_time_seconds FLOAT NOT NULL DEFAULT 0,
    retention_rate FLOAT NOT NULL DEFAULT 0,
    fetched_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Knowledge Sources
CREATE TABLE IF NOT EXISTS knowledge_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    source_type TEXT NOT NULL,
    crawl_frequency TEXT NOT NULL DEFAULT 'weekly',
    last_crawled_at TIMESTAMPTZ,
    enabled BOOLEAN NOT NULL DEFAULT TRUE
);

-- Knowledge Chunks (RAG)
CREATE TABLE IF NOT EXISTS knowledge_chunks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id UUID NOT NULL REFERENCES knowledge_sources(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    embedding VECTOR(1536),
    metadata JSONB NOT NULL DEFAULT '{}',
    url TEXT,
    crawled_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_embedding
    ON knowledge_chunks USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

-- Topic History
CREATE TABLE IF NOT EXISTS topic_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title TEXT NOT NULL,
    category TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Agent Configs
CREATE TABLE IF NOT EXISTS agent_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_name TEXT UNIQUE NOT NULL,
    system_prompt TEXT NOT NULL,
    model TEXT NOT NULL DEFAULT 'claude-sonnet-4-6-20250514',
    temperature FLOAT NOT NULL DEFAULT 0.7,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    config JSONB NOT NULL DEFAULT '{}'
);

-- Schedules
CREATE TABLE IF NOT EXISTS schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    cron_expression TEXT NOT NULL,
    action TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    last_run_at TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ
);

-- Brand Themes
CREATE TABLE IF NOT EXISTS brand_themes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL DEFAULT 'default',
    primary_color TEXT NOT NULL DEFAULT '#1a3a8f',
    secondary_color TEXT NOT NULL DEFAULT '#ffffff',
    accent_color TEXT NOT NULL DEFAULT '#f5851f',
    font_name TEXT NOT NULL DEFAULT 'Noto Sans Thai',
    logo_url TEXT,
    mascot_description TEXT DEFAULT 'Leopard astronaut riding a rocket, holding phone with Facebook icon',
    image_style TEXT DEFAULT 'Modern flat design, dark gradient background, energetic, tech-savvy',
    active BOOLEAN NOT NULL DEFAULT TRUE
);

-- Seed default agent configs
INSERT INTO agent_configs (agent_name, system_prompt, model) VALUES
    ('question', 'You generate realistic customer questions about Facebook Ads problems in Thai.', 'claude-sonnet-4-6-20250514'),
    ('script', 'You write Q&A video scripts answering Facebook Ads questions in Thai.', 'claude-sonnet-4-6-20250514'),
    ('image', 'You generate image prompts for video scenes matching brand theme.', 'claude-sonnet-4-6-20250514'),
    ('analytics', 'You analyze video performance metrics and recommend improvements.', 'claude-sonnet-4-6-20250514')
ON CONFLICT (agent_name) DO NOTHING;

-- Seed default brand theme
INSERT INTO brand_themes (name, primary_color, secondary_color, accent_color, mascot_description, active) VALUES
    ('Ads Vance Default', '#1a3a8f', '#ffffff', '#f5851f', 'Leopard astronaut riding a rocket, holding phone with Facebook icon', TRUE)
ON CONFLICT DO NOTHING;

-- Seed default schedules
INSERT INTO schedules (name, cron_expression, action) VALUES
    ('Weekly Production', '0 3 * * 1', 'produce_weekly'),
    ('Daily Publish', '0 17 * * *', 'publish_daily'),
    ('Weekly Knowledge Crawl', '0 2 * * 0', 'crawl_knowledge'),
    ('Weekly Analytics', '0 4 * * 0', 'fetch_analytics')
ON CONFLICT DO NOTHING;

-- Seed knowledge sources
INSERT INTO knowledge_sources (name, url, source_type, crawl_frequency) VALUES
    ('Meta Business Help Center', 'https://business.facebook.com/help', 'official', 'weekly'),
    ('Facebook Ads Policies', 'https://facebook.com/policies/ads', 'official', 'weekly'),
    ('Jon Loomer Digital', 'https://jonloomer.com', 'practitioner', 'weekly'),
    ('AdEspresso Blog', 'https://adespresso.com/blog', 'practitioner', 'weekly'),
    ('Social Media Examiner', 'https://socialmediaexaminer.com', 'practitioner', 'weekly'),
    ('r/FacebookAds', 'https://reddit.com/r/FacebookAds', 'community', 'daily'),
    ('r/PPC', 'https://reddit.com/r/PPC', 'community', 'daily')
ON CONFLICT DO NOTHING;
```

- [ ] **Step 3: Create internal/database/migrations.go**

```go
package database

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func RunMigrations(ctx context.Context, pool *pgxpool.Pool, migrationsDir string) error {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := migrationsDir + "/" + entry.Name()
		sql, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}

		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("execute migration %s: %w", entry.Name(), err)
		}
		fmt.Printf("Applied migration: %s\n", entry.Name())
	}

	return nil
}
```

- [ ] **Step 4: Update cmd/server/main.go to add DB + migrations**

```go
package main

import (
	"context"
	"flag"
	"log"
	"net/http"

	"github.com/jaochai/video-fb/internal/config"
	"github.com/jaochai/video-fb/internal/database"
)

func main() {
	migrateFlag := flag.Bool("migrate", false, "Run database migrations")
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
			log.Fatalf("Failed to run migrations: %v", err)
		}
		log.Println("Migrations complete")
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	addr := ":" + cfg.Port
	log.Printf("Starting server on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 5: Set up Neon database using MCP**

Use the Neon MCP tools to:
1. Create a new project called "adsvance-v2"
2. Get the connection string
3. Run migrations

```bash
# After getting DATABASE_URL from Neon MCP:
DATABASE_URL="postgresql://..." go run cmd/server/main.go -migrate
```

- [ ] **Step 6: Verify migrations ran**

Run:
```bash
DATABASE_URL="postgresql://..." go run cmd/server/main.go -migrate
```
Expected: "Applied migration: 001_initial_schema.sql" + "Migrations complete"

- [ ] **Step 7: Commit**

```bash
git add internal/database/ migrations/ cmd/server/main.go
git commit -m "feat: database connection pool and migration runner with full schema"
```

---

## Task 3: Domain Models

**Files:**
- Create: `internal/models/models.go`

- [ ] **Step 1: Create internal/models/models.go**

```go
package models

import (
	"encoding/json"
	"time"
)

type Clip struct {
	ID             string     `json:"id"`
	Title          string     `json:"title"`
	Question       string     `json:"question"`
	QuestionerName string     `json:"questioner_name"`
	AnswerScript   string     `json:"answer_script"`
	VoiceScript    string     `json:"voice_script"`
	Category       string     `json:"category"`
	Status         string     `json:"status"`
	Video169URL    *string    `json:"video_16_9_url"`
	Video916URL    *string    `json:"video_9_16_url"`
	ThumbnailURL   *string    `json:"thumbnail_url"`
	PublishDate    *string    `json:"publish_date"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type Scene struct {
	ID              string          `json:"id"`
	ClipID          string          `json:"clip_id"`
	SceneNumber     int             `json:"scene_number"`
	SceneType       string          `json:"scene_type"`
	TextContent     string          `json:"text_content"`
	ImagePrompt     string          `json:"image_prompt"`
	Image169URL     *string         `json:"image_16_9_url"`
	Image916URL     *string         `json:"image_9_16_url"`
	VoiceText       string          `json:"voice_text"`
	DurationSeconds float64         `json:"duration_seconds"`
	TextOverlays    json.RawMessage `json:"text_overlays"`
}

type ClipMetadata struct {
	ClipID           string   `json:"clip_id"`
	YoutubeTitle     *string  `json:"youtube_title"`
	YoutubeDesc      *string  `json:"youtube_description"`
	YoutubeTags      []string `json:"youtube_tags"`
	ZernioPostID     *string  `json:"zernio_post_id"`
	YoutubeVideoID   *string  `json:"youtube_video_id"`
	TiktokPostID     *string  `json:"tiktok_post_id"`
	IGPostID         *string  `json:"ig_post_id"`
	FBPostID         *string  `json:"fb_post_id"`
}

type ClipAnalytics struct {
	ID               string    `json:"id"`
	ClipID           string    `json:"clip_id"`
	Platform         string    `json:"platform"`
	Views            int       `json:"views"`
	Likes            int       `json:"likes"`
	Comments         int       `json:"comments"`
	Shares           int       `json:"shares"`
	WatchTimeSeconds float64   `json:"watch_time_seconds"`
	RetentionRate    float64   `json:"retention_rate"`
	FetchedAt        time.Time `json:"fetched_at"`
}

type KnowledgeSource struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	URL            string     `json:"url"`
	SourceType     string     `json:"source_type"`
	CrawlFrequency string    `json:"crawl_frequency"`
	LastCrawledAt  *time.Time `json:"last_crawled_at"`
	Enabled        bool       `json:"enabled"`
}

type KnowledgeChunk struct {
	ID        string          `json:"id"`
	SourceID  string          `json:"source_id"`
	Content   string          `json:"content"`
	Metadata  json.RawMessage `json:"metadata"`
	URL       *string         `json:"url"`
	CrawledAt time.Time       `json:"crawled_at"`
}

type TopicHistory struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Category  string    `json:"category"`
	CreatedAt time.Time `json:"created_at"`
}

type AgentConfig struct {
	ID           string          `json:"id"`
	AgentName    string          `json:"agent_name"`
	SystemPrompt string          `json:"system_prompt"`
	Model        string          `json:"model"`
	Temperature  float64         `json:"temperature"`
	Enabled      bool            `json:"enabled"`
	Config       json.RawMessage `json:"config"`
}

type Schedule struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	CronExpression string     `json:"cron_expression"`
	Action         string     `json:"action"`
	Enabled        bool       `json:"enabled"`
	LastRunAt      *time.Time `json:"last_run_at"`
	NextRunAt      *time.Time `json:"next_run_at"`
}

type BrandTheme struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	PrimaryColor      string  `json:"primary_color"`
	SecondaryColor    string  `json:"secondary_color"`
	AccentColor       string  `json:"accent_color"`
	FontName          string  `json:"font_name"`
	LogoURL           *string `json:"logo_url"`
	MascotDescription *string `json:"mascot_description"`
	ImageStyle        *string `json:"image_style"`
	Active            bool    `json:"active"`
}

// Request types for create/update
type CreateClipRequest struct {
	Title          string  `json:"title"`
	Question       string  `json:"question"`
	QuestionerName string  `json:"questioner_name"`
	Category       string  `json:"category"`
	PublishDate    *string `json:"publish_date"`
}

type UpdateClipRequest struct {
	Title          *string `json:"title"`
	Question       *string `json:"question"`
	QuestionerName *string `json:"questioner_name"`
	AnswerScript   *string `json:"answer_script"`
	VoiceScript    *string `json:"voice_script"`
	Category       *string `json:"category"`
	Status         *string `json:"status"`
	Video169URL    *string `json:"video_16_9_url"`
	Video916URL    *string `json:"video_9_16_url"`
	ThumbnailURL   *string `json:"thumbnail_url"`
	PublishDate    *string `json:"publish_date"`
}

type CreateSceneRequest struct {
	ClipID          string          `json:"clip_id"`
	SceneNumber     int             `json:"scene_number"`
	SceneType       string          `json:"scene_type"`
	TextContent     string          `json:"text_content"`
	ImagePrompt     string          `json:"image_prompt"`
	VoiceText       string          `json:"voice_text"`
	DurationSeconds float64         `json:"duration_seconds"`
	TextOverlays    json.RawMessage `json:"text_overlays"`
}

type APIResponse struct {
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/jaochai/Code/video-fb && go build ./...`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add internal/models/
git commit -m "feat: domain models for all entities"
```

---

## Task 4: Clips Repository + Handler

**Files:**
- Create: `internal/repository/clips.go`
- Create: `internal/handler/clips.go`

- [ ] **Step 1: Create internal/repository/clips.go**

```go
package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
)

type ClipsRepo struct {
	pool *pgxpool.Pool
}

func NewClipsRepo(pool *pgxpool.Pool) *ClipsRepo {
	return &ClipsRepo{pool: pool}
}

func (r *ClipsRepo) List(ctx context.Context) ([]models.Clip, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, title, question, questioner_name, answer_script, voice_script,
		        category, status, video_16_9_url, video_9_16_url, thumbnail_url,
		        publish_date::text, created_at, updated_at
		 FROM clips ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query clips: %w", err)
	}
	defer rows.Close()

	var clips []models.Clip
	for rows.Next() {
		var c models.Clip
		if err := rows.Scan(
			&c.ID, &c.Title, &c.Question, &c.QuestionerName,
			&c.AnswerScript, &c.VoiceScript, &c.Category, &c.Status,
			&c.Video169URL, &c.Video916URL, &c.ThumbnailURL,
			&c.PublishDate, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan clip: %w", err)
		}
		clips = append(clips, c)
	}
	return clips, nil
}

func (r *ClipsRepo) GetByID(ctx context.Context, id string) (*models.Clip, error) {
	var c models.Clip
	err := r.pool.QueryRow(ctx,
		`SELECT id, title, question, questioner_name, answer_script, voice_script,
		        category, status, video_16_9_url, video_9_16_url, thumbnail_url,
		        publish_date::text, created_at, updated_at
		 FROM clips WHERE id = $1`, id).Scan(
		&c.ID, &c.Title, &c.Question, &c.QuestionerName,
		&c.AnswerScript, &c.VoiceScript, &c.Category, &c.Status,
		&c.Video169URL, &c.Video916URL, &c.ThumbnailURL,
		&c.PublishDate, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get clip %s: %w", id, err)
	}
	return &c, nil
}

func (r *ClipsRepo) Create(ctx context.Context, req models.CreateClipRequest) (*models.Clip, error) {
	var c models.Clip
	err := r.pool.QueryRow(ctx,
		`INSERT INTO clips (title, question, questioner_name, category, publish_date)
		 VALUES ($1, $2, $3, $4, $5::date)
		 RETURNING id, title, question, questioner_name, answer_script, voice_script,
		           category, status, video_16_9_url, video_9_16_url, thumbnail_url,
		           publish_date::text, created_at, updated_at`,
		req.Title, req.Question, req.QuestionerName, req.Category, req.PublishDate,
	).Scan(
		&c.ID, &c.Title, &c.Question, &c.QuestionerName,
		&c.AnswerScript, &c.VoiceScript, &c.Category, &c.Status,
		&c.Video169URL, &c.Video916URL, &c.ThumbnailURL,
		&c.PublishDate, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create clip: %w", err)
	}
	return &c, nil
}

func (r *ClipsRepo) Update(ctx context.Context, id string, req models.UpdateClipRequest) (*models.Clip, error) {
	var c models.Clip
	err := r.pool.QueryRow(ctx,
		`UPDATE clips SET
			title = COALESCE($2, title),
			question = COALESCE($3, question),
			questioner_name = COALESCE($4, questioner_name),
			answer_script = COALESCE($5, answer_script),
			voice_script = COALESCE($6, voice_script),
			category = COALESCE($7, category),
			status = COALESCE($8, status),
			video_16_9_url = COALESCE($9, video_16_9_url),
			video_9_16_url = COALESCE($10, video_9_16_url),
			thumbnail_url = COALESCE($11, thumbnail_url),
			publish_date = COALESCE($12::date, publish_date),
			updated_at = NOW()
		 WHERE id = $1
		 RETURNING id, title, question, questioner_name, answer_script, voice_script,
		           category, status, video_16_9_url, video_9_16_url, thumbnail_url,
		           publish_date::text, created_at, updated_at`,
		id, req.Title, req.Question, req.QuestionerName,
		req.AnswerScript, req.VoiceScript, req.Category, req.Status,
		req.Video169URL, req.Video916URL, req.ThumbnailURL, req.PublishDate,
	).Scan(
		&c.ID, &c.Title, &c.Question, &c.QuestionerName,
		&c.AnswerScript, &c.VoiceScript, &c.Category, &c.Status,
		&c.Video169URL, &c.Video916URL, &c.ThumbnailURL,
		&c.PublishDate, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update clip %s: %w", id, err)
	}
	return &c, nil
}

func (r *ClipsRepo) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM clips WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete clip %s: %w", id, err)
	}
	return nil
}
```

- [ ] **Step 2: Create internal/handler/clips.go**

```go
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
)

type ClipsHandler struct {
	repo *repository.ClipsRepo
}

func NewClipsHandler(repo *repository.ClipsRepo) *ClipsHandler {
	return &ClipsHandler{repo: repo}
}

func (h *ClipsHandler) List(w http.ResponseWriter, r *http.Request) {
	clips, err := h.repo.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: clips})
}

func (h *ClipsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	clip, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.APIResponse{Error: "clip not found"})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: clip})
}

func (h *ClipsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.CreateClipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.APIResponse{Error: "invalid request body"})
		return
	}
	clip, err := h.repo.Create(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, models.APIResponse{Data: clip})
}

func (h *ClipsHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req models.UpdateClipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.APIResponse{Error: "invalid request body"})
		return
	}
	clip, err := h.repo.Update(r.Context(), id, req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: clip})
}

func (h *ClipsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.repo.Delete(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Message: "deleted"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`
Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add internal/repository/clips.go internal/handler/clips.go
git commit -m "feat: clips repository and handler with full CRUD"
```

---

## Task 5: Remaining Repositories (Batch)

**Files:**
- Create: `internal/repository/knowledge.go`
- Create: `internal/repository/agents.go`
- Create: `internal/repository/schedules.go`
- Create: `internal/repository/themes.go`
- Create: `internal/repository/analytics.go`
- Create: `internal/repository/topics.go`
- Create: `internal/repository/scenes.go`

This task creates all remaining repositories. Each follows the same pattern as clips: List, GetByID, Create, Update, Delete (where applicable).

- [ ] **Step 1: Create internal/repository/scenes.go**

```go
package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
)

type ScenesRepo struct {
	pool *pgxpool.Pool
}

func NewScenesRepo(pool *pgxpool.Pool) *ScenesRepo {
	return &ScenesRepo{pool: pool}
}

func (r *ScenesRepo) ListByClip(ctx context.Context, clipID string) ([]models.Scene, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, clip_id, scene_number, scene_type, text_content, image_prompt,
		        image_16_9_url, image_9_16_url, voice_text, duration_seconds, text_overlays
		 FROM scenes WHERE clip_id = $1 ORDER BY scene_number`, clipID)
	if err != nil {
		return nil, fmt.Errorf("query scenes: %w", err)
	}
	defer rows.Close()

	var scenes []models.Scene
	for rows.Next() {
		var s models.Scene
		if err := rows.Scan(&s.ID, &s.ClipID, &s.SceneNumber, &s.SceneType,
			&s.TextContent, &s.ImagePrompt, &s.Image169URL, &s.Image916URL,
			&s.VoiceText, &s.DurationSeconds, &s.TextOverlays); err != nil {
			return nil, fmt.Errorf("scan scene: %w", err)
		}
		scenes = append(scenes, s)
	}
	return scenes, nil
}

func (r *ScenesRepo) Create(ctx context.Context, req models.CreateSceneRequest) (*models.Scene, error) {
	var s models.Scene
	err := r.pool.QueryRow(ctx,
		`INSERT INTO scenes (clip_id, scene_number, scene_type, text_content, image_prompt, voice_text, duration_seconds, text_overlays)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, clip_id, scene_number, scene_type, text_content, image_prompt,
		           image_16_9_url, image_9_16_url, voice_text, duration_seconds, text_overlays`,
		req.ClipID, req.SceneNumber, req.SceneType, req.TextContent,
		req.ImagePrompt, req.VoiceText, req.DurationSeconds, req.TextOverlays,
	).Scan(&s.ID, &s.ClipID, &s.SceneNumber, &s.SceneType,
		&s.TextContent, &s.ImagePrompt, &s.Image169URL, &s.Image916URL,
		&s.VoiceText, &s.DurationSeconds, &s.TextOverlays)
	if err != nil {
		return nil, fmt.Errorf("create scene: %w", err)
	}
	return &s, nil
}

func (r *ScenesRepo) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM scenes WHERE id = $1`, id)
	return err
}
```

- [ ] **Step 2: Create internal/repository/knowledge.go**

```go
package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
)

type KnowledgeRepo struct {
	pool *pgxpool.Pool
}

func NewKnowledgeRepo(pool *pgxpool.Pool) *KnowledgeRepo {
	return &KnowledgeRepo{pool: pool}
}

func (r *KnowledgeRepo) ListSources(ctx context.Context) ([]models.KnowledgeSource, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, url, source_type, crawl_frequency, last_crawled_at, enabled
		 FROM knowledge_sources ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query sources: %w", err)
	}
	defer rows.Close()

	var sources []models.KnowledgeSource
	for rows.Next() {
		var s models.KnowledgeSource
		if err := rows.Scan(&s.ID, &s.Name, &s.URL, &s.SourceType,
			&s.CrawlFrequency, &s.LastCrawledAt, &s.Enabled); err != nil {
			return nil, fmt.Errorf("scan source: %w", err)
		}
		sources = append(sources, s)
	}
	return sources, nil
}

func (r *KnowledgeRepo) CreateSource(ctx context.Context, name, url, sourceType, crawlFreq string) (*models.KnowledgeSource, error) {
	var s models.KnowledgeSource
	err := r.pool.QueryRow(ctx,
		`INSERT INTO knowledge_sources (name, url, source_type, crawl_frequency)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, name, url, source_type, crawl_frequency, last_crawled_at, enabled`,
		name, url, sourceType, crawlFreq,
	).Scan(&s.ID, &s.Name, &s.URL, &s.SourceType, &s.CrawlFrequency, &s.LastCrawledAt, &s.Enabled)
	if err != nil {
		return nil, fmt.Errorf("create source: %w", err)
	}
	return &s, nil
}

func (r *KnowledgeRepo) ToggleSource(ctx context.Context, id string, enabled bool) error {
	_, err := r.pool.Exec(ctx, `UPDATE knowledge_sources SET enabled = $2 WHERE id = $1`, id, enabled)
	return err
}

func (r *KnowledgeRepo) DeleteSource(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM knowledge_sources WHERE id = $1`, id)
	return err
}
```

- [ ] **Step 3: Create internal/repository/agents.go**

```go
package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
)

type AgentsRepo struct {
	pool *pgxpool.Pool
}

func NewAgentsRepo(pool *pgxpool.Pool) *AgentsRepo {
	return &AgentsRepo{pool: pool}
}

func (r *AgentsRepo) List(ctx context.Context) ([]models.AgentConfig, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, agent_name, system_prompt, model, temperature, enabled, config
		 FROM agent_configs ORDER BY agent_name`)
	if err != nil {
		return nil, fmt.Errorf("query agents: %w", err)
	}
	defer rows.Close()

	var agents []models.AgentConfig
	for rows.Next() {
		var a models.AgentConfig
		if err := rows.Scan(&a.ID, &a.AgentName, &a.SystemPrompt, &a.Model,
			&a.Temperature, &a.Enabled, &a.Config); err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		agents = append(agents, a)
	}
	return agents, nil
}

func (r *AgentsRepo) GetByName(ctx context.Context, name string) (*models.AgentConfig, error) {
	var a models.AgentConfig
	err := r.pool.QueryRow(ctx,
		`SELECT id, agent_name, system_prompt, model, temperature, enabled, config
		 FROM agent_configs WHERE agent_name = $1`, name,
	).Scan(&a.ID, &a.AgentName, &a.SystemPrompt, &a.Model, &a.Temperature, &a.Enabled, &a.Config)
	if err != nil {
		return nil, fmt.Errorf("get agent %s: %w", name, err)
	}
	return &a, nil
}

func (r *AgentsRepo) Update(ctx context.Context, id string, prompt, model string, temp float64, enabled bool) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE agent_configs SET system_prompt=$2, model=$3, temperature=$4, enabled=$5 WHERE id=$1`,
		id, prompt, model, temp, enabled)
	return err
}
```

- [ ] **Step 4: Create internal/repository/schedules.go**

```go
package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
)

type SchedulesRepo struct {
	pool *pgxpool.Pool
}

func NewSchedulesRepo(pool *pgxpool.Pool) *SchedulesRepo {
	return &SchedulesRepo{pool: pool}
}

func (r *SchedulesRepo) List(ctx context.Context) ([]models.Schedule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, cron_expression, action, enabled, last_run_at, next_run_at
		 FROM schedules ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query schedules: %w", err)
	}
	defer rows.Close()

	var schedules []models.Schedule
	for rows.Next() {
		var s models.Schedule
		if err := rows.Scan(&s.ID, &s.Name, &s.CronExpression, &s.Action,
			&s.Enabled, &s.LastRunAt, &s.NextRunAt); err != nil {
			return nil, fmt.Errorf("scan schedule: %w", err)
		}
		schedules = append(schedules, s)
	}
	return schedules, nil
}

func (r *SchedulesRepo) Update(ctx context.Context, id, cron, action string, enabled bool) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE schedules SET cron_expression=$2, action=$3, enabled=$4 WHERE id=$1`,
		id, cron, action, enabled)
	return err
}
```

- [ ] **Step 5: Create internal/repository/themes.go**

```go
package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
)

type ThemesRepo struct {
	pool *pgxpool.Pool
}

func NewThemesRepo(pool *pgxpool.Pool) *ThemesRepo {
	return &ThemesRepo{pool: pool}
}

func (r *ThemesRepo) GetActive(ctx context.Context) (*models.BrandTheme, error) {
	var t models.BrandTheme
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, primary_color, secondary_color, accent_color, font_name,
		        logo_url, mascot_description, image_style, active
		 FROM brand_themes WHERE active = TRUE LIMIT 1`,
	).Scan(&t.ID, &t.Name, &t.PrimaryColor, &t.SecondaryColor, &t.AccentColor,
		&t.FontName, &t.LogoURL, &t.MascotDescription, &t.ImageStyle, &t.Active)
	if err != nil {
		return nil, fmt.Errorf("get active theme: %w", err)
	}
	return &t, nil
}

func (r *ThemesRepo) List(ctx context.Context) ([]models.BrandTheme, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, primary_color, secondary_color, accent_color, font_name,
		        logo_url, mascot_description, image_style, active
		 FROM brand_themes ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query themes: %w", err)
	}
	defer rows.Close()

	var themes []models.BrandTheme
	for rows.Next() {
		var t models.BrandTheme
		if err := rows.Scan(&t.ID, &t.Name, &t.PrimaryColor, &t.SecondaryColor, &t.AccentColor,
			&t.FontName, &t.LogoURL, &t.MascotDescription, &t.ImageStyle, &t.Active); err != nil {
			return nil, fmt.Errorf("scan theme: %w", err)
		}
		themes = append(themes, t)
	}
	return themes, nil
}

func (r *ThemesRepo) Update(ctx context.Context, id string, t models.BrandTheme) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE brand_themes SET name=$2, primary_color=$3, secondary_color=$4,
		 accent_color=$5, font_name=$6, logo_url=$7, mascot_description=$8, image_style=$9 WHERE id=$1`,
		id, t.Name, t.PrimaryColor, t.SecondaryColor, t.AccentColor,
		t.FontName, t.LogoURL, t.MascotDescription, t.ImageStyle)
	return err
}
```

- [ ] **Step 6: Create internal/repository/analytics.go**

```go
package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
)

type AnalyticsRepo struct {
	pool *pgxpool.Pool
}

func NewAnalyticsRepo(pool *pgxpool.Pool) *AnalyticsRepo {
	return &AnalyticsRepo{pool: pool}
}

func (r *AnalyticsRepo) ListByClip(ctx context.Context, clipID string) ([]models.ClipAnalytics, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, clip_id, platform, views, likes, comments, shares,
		        watch_time_seconds, retention_rate, fetched_at
		 FROM clip_analytics WHERE clip_id = $1 ORDER BY fetched_at DESC`, clipID)
	if err != nil {
		return nil, fmt.Errorf("query analytics: %w", err)
	}
	defer rows.Close()

	var results []models.ClipAnalytics
	for rows.Next() {
		var a models.ClipAnalytics
		if err := rows.Scan(&a.ID, &a.ClipID, &a.Platform, &a.Views, &a.Likes,
			&a.Comments, &a.Shares, &a.WatchTimeSeconds, &a.RetentionRate, &a.FetchedAt); err != nil {
			return nil, fmt.Errorf("scan analytics: %w", err)
		}
		results = append(results, a)
	}
	return results, nil
}

func (r *AnalyticsRepo) Create(ctx context.Context, a models.ClipAnalytics) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO clip_analytics (clip_id, platform, views, likes, comments, shares, watch_time_seconds, retention_rate)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		a.ClipID, a.Platform, a.Views, a.Likes, a.Comments, a.Shares, a.WatchTimeSeconds, a.RetentionRate)
	return err
}
```

- [ ] **Step 7: Create internal/repository/topics.go**

```go
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
)

type TopicsRepo struct {
	pool *pgxpool.Pool
}

func NewTopicsRepo(pool *pgxpool.Pool) *TopicsRepo {
	return &TopicsRepo{pool: pool}
}

func (r *TopicsRepo) ListRecent(ctx context.Context, days int) ([]models.TopicHistory, error) {
	cutoff := time.Now().AddDate(0, 0, -days)
	rows, err := r.pool.Query(ctx,
		`SELECT id, title, category, created_at FROM topic_history
		 WHERE created_at >= $1 ORDER BY created_at DESC`, cutoff)
	if err != nil {
		return nil, fmt.Errorf("query topics: %w", err)
	}
	defer rows.Close()

	var topics []models.TopicHistory
	for rows.Next() {
		var t models.TopicHistory
		if err := rows.Scan(&t.ID, &t.Title, &t.Category, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan topic: %w", err)
		}
		topics = append(topics, t)
	}
	return topics, nil
}

func (r *TopicsRepo) Create(ctx context.Context, title, category string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO topic_history (title, category) VALUES ($1, $2)`, title, category)
	return err
}
```

- [ ] **Step 8: Verify all compile**

Run: `go build ./...`
Expected: No errors.

- [ ] **Step 9: Commit**

```bash
git add internal/repository/
git commit -m "feat: all repositories - scenes, knowledge, agents, schedules, themes, analytics, topics"
```

---

## Task 6: Remaining Handlers + Middleware + Router

**Files:**
- Create: `internal/handler/health.go`
- Create: `internal/handler/middleware.go`
- Create: `internal/handler/knowledge.go`
- Create: `internal/handler/agents.go`
- Create: `internal/handler/schedules.go`
- Create: `internal/handler/themes.go`
- Create: `internal/handler/analytics.go`
- Create: `internal/handler/scenes.go`
- Create: `internal/router/router.go`

- [ ] **Step 1: Create internal/handler/health.go**

```go
package handler

import "net/http"

func HealthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
```

- [ ] **Step 2: Create internal/handler/middleware.go**

```go
package handler

import (
	"net/http"
	"strings"
)

func APIKeyAuth(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}

			auth := r.Header.Get("Authorization")
			token := strings.TrimPrefix(auth, "Bearer ")
			if token == "" || token != apiKey {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 3: Create internal/handler/scenes.go**

```go
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
)

type ScenesHandler struct {
	repo *repository.ScenesRepo
}

func NewScenesHandler(repo *repository.ScenesRepo) *ScenesHandler {
	return &ScenesHandler{repo: repo}
}

func (h *ScenesHandler) ListByClip(w http.ResponseWriter, r *http.Request) {
	clipID := chi.URLParam(r, "clipId")
	scenes, err := h.repo.ListByClip(r.Context(), clipID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: scenes})
}

func (h *ScenesHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.CreateSceneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.APIResponse{Error: "invalid request"})
		return
	}
	scene, err := h.repo.Create(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, models.APIResponse{Data: scene})
}

func (h *ScenesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.repo.Delete(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Message: "deleted"})
}
```

- [ ] **Step 4: Create internal/handler/knowledge.go**

```go
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
)

type KnowledgeHandler struct {
	repo *repository.KnowledgeRepo
}

func NewKnowledgeHandler(repo *repository.KnowledgeRepo) *KnowledgeHandler {
	return &KnowledgeHandler{repo: repo}
}

func (h *KnowledgeHandler) ListSources(w http.ResponseWriter, r *http.Request) {
	sources, err := h.repo.ListSources(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: sources})
}

func (h *KnowledgeHandler) CreateSource(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name           string `json:"name"`
		URL            string `json:"url"`
		SourceType     string `json:"source_type"`
		CrawlFrequency string `json:"crawl_frequency"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.APIResponse{Error: "invalid request"})
		return
	}
	source, err := h.repo.CreateSource(r.Context(), req.Name, req.URL, req.SourceType, req.CrawlFrequency)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, models.APIResponse{Data: source})
}

func (h *KnowledgeHandler) ToggleSource(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct{ Enabled bool `json:"enabled"` }
	json.NewDecoder(r.Body).Decode(&req)
	if err := h.repo.ToggleSource(r.Context(), id, req.Enabled); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Message: "updated"})
}

func (h *KnowledgeHandler) DeleteSource(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.repo.DeleteSource(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Message: "deleted"})
}
```

- [ ] **Step 5: Create internal/handler/agents.go**

```go
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
)

type AgentsHandler struct {
	repo *repository.AgentsRepo
}

func NewAgentsHandler(repo *repository.AgentsRepo) *AgentsHandler {
	return &AgentsHandler{repo: repo}
}

func (h *AgentsHandler) List(w http.ResponseWriter, r *http.Request) {
	agents, err := h.repo.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: agents})
}

func (h *AgentsHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		SystemPrompt string  `json:"system_prompt"`
		Model        string  `json:"model"`
		Temperature  float64 `json:"temperature"`
		Enabled      bool    `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.APIResponse{Error: "invalid request"})
		return
	}
	if err := h.repo.Update(r.Context(), id, req.SystemPrompt, req.Model, req.Temperature, req.Enabled); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Message: "updated"})
}
```

- [ ] **Step 6: Create internal/handler/schedules.go**

```go
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
)

type SchedulesHandler struct {
	repo *repository.SchedulesRepo
}

func NewSchedulesHandler(repo *repository.SchedulesRepo) *SchedulesHandler {
	return &SchedulesHandler{repo: repo}
}

func (h *SchedulesHandler) List(w http.ResponseWriter, r *http.Request) {
	schedules, err := h.repo.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: schedules})
}

func (h *SchedulesHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		CronExpression string `json:"cron_expression"`
		Action         string `json:"action"`
		Enabled        bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.APIResponse{Error: "invalid request"})
		return
	}
	if err := h.repo.Update(r.Context(), id, req.CronExpression, req.Action, req.Enabled); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Message: "updated"})
}
```

- [ ] **Step 7: Create internal/handler/themes.go**

```go
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
)

type ThemesHandler struct {
	repo *repository.ThemesRepo
}

func NewThemesHandler(repo *repository.ThemesRepo) *ThemesHandler {
	return &ThemesHandler{repo: repo}
}

func (h *ThemesHandler) List(w http.ResponseWriter, r *http.Request) {
	themes, err := h.repo.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: themes})
}

func (h *ThemesHandler) GetActive(w http.ResponseWriter, r *http.Request) {
	theme, err := h.repo.GetActive(r.Context())
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.APIResponse{Error: "no active theme"})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: theme})
}

func (h *ThemesHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var t models.BrandTheme
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeJSON(w, http.StatusBadRequest, models.APIResponse{Error: "invalid request"})
		return
	}
	if err := h.repo.Update(r.Context(), id, t); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Message: "updated"})
}
```

- [ ] **Step 8: Create internal/handler/analytics.go**

```go
package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
)

type AnalyticsHandler struct {
	repo *repository.AnalyticsRepo
}

func NewAnalyticsHandler(repo *repository.AnalyticsRepo) *AnalyticsHandler {
	return &AnalyticsHandler{repo: repo}
}

func (h *AnalyticsHandler) ListByClip(w http.ResponseWriter, r *http.Request) {
	clipID := chi.URLParam(r, "clipId")
	analytics, err := h.repo.ListByClip(r.Context(), clipID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: analytics})
}
```

- [ ] **Step 9: Create internal/router/router.go**

```go
package router

import (
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jaochai/video-fb/internal/handler"
	"github.com/jaochai/video-fb/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

func New(pool *pgxpool.Pool, apiKey string) *chi.Mux {
	r := chi.NewRouter()

	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	r.Use(handler.APIKeyAuth(apiKey))

	r.Get("/health", handler.HealthCheck)

	clips := handler.NewClipsHandler(repository.NewClipsRepo(pool))
	r.Route("/api/v1/clips", func(r chi.Router) {
		r.Get("/", clips.List)
		r.Post("/", clips.Create)
		r.Get("/{id}", clips.Get)
		r.Patch("/{id}", clips.Update)
		r.Delete("/{id}", clips.Delete)
	})

	scenes := handler.NewScenesHandler(repository.NewScenesRepo(pool))
	r.Route("/api/v1/clips/{clipId}/scenes", func(r chi.Router) {
		r.Get("/", scenes.ListByClip)
		r.Post("/", scenes.Create)
	})
	r.Delete("/api/v1/scenes/{id}", scenes.Delete)

	knowledge := handler.NewKnowledgeHandler(repository.NewKnowledgeRepo(pool))
	r.Route("/api/v1/knowledge/sources", func(r chi.Router) {
		r.Get("/", knowledge.ListSources)
		r.Post("/", knowledge.CreateSource)
		r.Patch("/{id}", knowledge.ToggleSource)
		r.Delete("/{id}", knowledge.DeleteSource)
	})

	agents := handler.NewAgentsHandler(repository.NewAgentsRepo(pool))
	r.Route("/api/v1/agents", func(r chi.Router) {
		r.Get("/", agents.List)
		r.Patch("/{id}", agents.Update)
	})

	schedules := handler.NewSchedulesHandler(repository.NewSchedulesRepo(pool))
	r.Route("/api/v1/schedules", func(r chi.Router) {
		r.Get("/", schedules.List)
		r.Patch("/{id}", schedules.Update)
	})

	themes := handler.NewThemesHandler(repository.NewThemesRepo(pool))
	r.Route("/api/v1/themes", func(r chi.Router) {
		r.Get("/", themes.List)
		r.Get("/active", themes.GetActive)
		r.Patch("/{id}", themes.Update)
	})

	analytics := handler.NewAnalyticsHandler(repository.NewAnalyticsRepo(pool))
	r.Get("/api/v1/clips/{clipId}/analytics", analytics.ListByClip)

	return r
}
```

- [ ] **Step 10: Update cmd/server/main.go to use router**

```go
package main

import (
	"context"
	"flag"
	"log"
	"net/http"

	"github.com/jaochai/video-fb/internal/config"
	"github.com/jaochai/video-fb/internal/database"
	"github.com/jaochai/video-fb/internal/router"
)

func main() {
	migrateFlag := flag.Bool("migrate", false, "Run database migrations")
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
			log.Fatalf("Failed to run migrations: %v", err)
		}
		log.Println("Migrations complete")
		return
	}

	r := router.New(pool, cfg.APIKey)

	addr := ":" + cfg.Port
	log.Printf("Starting server on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 11: Verify it compiles**

Run: `go build ./...`
Expected: No errors.

- [ ] **Step 12: Commit**

```bash
git add internal/handler/ internal/router/ cmd/server/main.go
git commit -m "feat: all REST handlers, middleware, and router with full API endpoints"
```

---

## Task 7: Dockerfile + Railway Deploy

**Files:**
- Create: `Dockerfile`

- [ ] **Step 1: Create Dockerfile**

```dockerfile
FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /server cmd/server/main.go

FROM alpine:3.21
RUN apk --no-cache add ca-certificates
COPY --from=builder /server /server
COPY migrations/ /migrations/

ENV PORT=8080
EXPOSE 8080

CMD ["/server"]
```

- [ ] **Step 2: Build and test Docker image locally**

Run:
```bash
docker build -t adsvance-v2 . && docker run --rm -p 8080:8080 -e DATABASE_URL="$DATABASE_URL" -e API_KEY=test adsvance-v2 &
sleep 2 && curl -s -H "Authorization: Bearer test" http://localhost:8080/health && docker stop $(docker ps -q --filter ancestor=adsvance-v2)
```
Expected: `{"status":"ok"}`

- [ ] **Step 3: Deploy to Railway using MCP**

Use Railway MCP tools to:
1. Create project "adsvance-v2"
2. Link to this directory
3. Set environment variables (DATABASE_URL, API_KEY, PORT)
4. Deploy

- [ ] **Step 4: Verify Railway deployment**

```bash
curl -s -H "Authorization: Bearer $API_KEY" https://<railway-url>/health
```
Expected: `{"status":"ok"}`

- [ ] **Step 5: Commit**

```bash
git add Dockerfile
git commit -m "feat: Dockerfile for Railway deployment"
```

---

## API Endpoints Summary

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check (no auth) |
| GET | `/api/v1/clips` | List all clips |
| POST | `/api/v1/clips` | Create clip |
| GET | `/api/v1/clips/{id}` | Get clip |
| PATCH | `/api/v1/clips/{id}` | Update clip |
| DELETE | `/api/v1/clips/{id}` | Delete clip |
| GET | `/api/v1/clips/{clipId}/scenes` | List scenes for clip |
| POST | `/api/v1/clips/{clipId}/scenes` | Create scene |
| DELETE | `/api/v1/scenes/{id}` | Delete scene |
| GET | `/api/v1/clips/{clipId}/analytics` | Get clip analytics |
| GET | `/api/v1/knowledge/sources` | List knowledge sources |
| POST | `/api/v1/knowledge/sources` | Add knowledge source |
| PATCH | `/api/v1/knowledge/sources/{id}` | Toggle source enabled |
| DELETE | `/api/v1/knowledge/sources/{id}` | Delete source |
| GET | `/api/v1/agents` | List agent configs |
| PATCH | `/api/v1/agents/{id}` | Update agent config |
| GET | `/api/v1/schedules` | List schedules |
| PATCH | `/api/v1/schedules/{id}` | Update schedule |
| GET | `/api/v1/themes` | List themes |
| GET | `/api/v1/themes/active` | Get active theme |
| PATCH | `/api/v1/themes/{id}` | Update theme |

## Task Summary

| Task | Component | Key Outcome |
|------|-----------|------------|
| 1 | Project Init + Config | Go module, config, health check, Makefile |
| 2 | Database + Migrations | Neon connection, full schema with seeds |
| 3 | Domain Models | All Go structs for DB entities |
| 4 | Clips Repo + Handler | Full CRUD for clips |
| 5 | All Other Repos | scenes, knowledge, agents, schedules, themes, analytics, topics |
| 6 | All Handlers + Router | REST endpoints, auth middleware, CORS, chi router |
| 7 | Docker + Railway | Dockerfile, Railway deployment |
