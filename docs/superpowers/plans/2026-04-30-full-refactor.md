# Full Stack Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor backend (Go) and frontend (React/TS) for cleaner separation of concerns, reduced duplication, and better maintainability.

**Architecture:** Layer-by-Layer bottom-up — models → repository → orchestrator → handler → frontend hooks → frontend pages. Each task produces a buildable codebase.

**Tech Stack:** Go 1.25, chi router, pgx/v5, React 19, TanStack Query, Vite 8

**Spec:** `docs/superpowers/specs/2026-04-30-full-refactor-design.md`

---

## Phase 1: Backend Foundation

### Task 1: Split `models/models.go` into domain files

**Files:**
- Create: `internal/models/clip.go`
- Create: `internal/models/knowledge.go`
- Create: `internal/models/agent.go`
- Create: `internal/models/schedule.go`
- Create: `internal/models/theme.go`
- Create: `internal/models/request.go`
- Create: `internal/models/response.go`
- Delete: `internal/models/models.go`

- [ ] **Step 1: Create `internal/models/clip.go`**

```go
package models

import (
	"encoding/json"
	"time"
)

type Clip struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	Question       string    `json:"question"`
	QuestionerName string    `json:"questioner_name"`
	AnswerScript   string    `json:"answer_script"`
	VoiceScript    string    `json:"voice_script"`
	Category       string    `json:"category"`
	Status         string    `json:"status"`
	Video169URL    *string   `json:"video_16_9_url"`
	Video916URL    *string   `json:"video_9_16_url"`
	ThumbnailURL   *string   `json:"thumbnail_url"`
	PublishDate    *string   `json:"publish_date"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	FailReason     *string   `json:"fail_reason,omitempty"`
	RetryCount     int       `json:"retry_count"`
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
	ClipID         string   `json:"clip_id"`
	YoutubeTitle   *string  `json:"youtube_title"`
	YoutubeDesc    *string  `json:"youtube_description"`
	YoutubeTags    []string `json:"youtube_tags"`
	ZernioPostID   *string  `json:"zernio_post_id"`
	YoutubeVideoID *string  `json:"youtube_video_id"`
	TiktokPostID   *string  `json:"tiktok_post_id"`
	IGPostID       *string  `json:"ig_post_id"`
	FBPostID       *string  `json:"fb_post_id"`
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
```

- [ ] **Step 2: Create `internal/models/knowledge.go`**

```go
package models

import (
	"encoding/json"
	"time"
)

type KnowledgeSource struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Category   string `json:"category"`
	Content    string `json:"content"`
	Enabled    bool   `json:"enabled"`
	ChunkCount int    `json:"chunk_count"`
}

type KnowledgeSourceSummary struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Category       string `json:"category"`
	ContentPreview string `json:"content_preview"`
	Enabled        bool   `json:"enabled"`
	ChunkCount     int    `json:"chunk_count"`
}

type KnowledgeChunk struct {
	ID        string          `json:"id"`
	SourceID  string          `json:"source_id"`
	Content   string          `json:"content"`
	Metadata  json.RawMessage `json:"metadata"`
	URL       *string         `json:"url"`
	CrawledAt time.Time       `json:"crawled_at"`
}
```

- [ ] **Step 3: Create `internal/models/agent.go`**

```go
package models

import "encoding/json"

type AgentConfig struct {
	ID             string          `json:"id"`
	AgentName      string          `json:"agent_name"`
	SystemPrompt   string          `json:"system_prompt"`
	PromptTemplate string          `json:"prompt_template"`
	Model          string          `json:"model"`
	Temperature    float64         `json:"temperature"`
	Enabled        bool            `json:"enabled"`
	Skills         string          `json:"skills"`
	Config         json.RawMessage `json:"config"`
}

func (c *AgentConfig) BuildSystemPrompt() string {
	if c.Skills == "" {
		return c.SystemPrompt
	}
	return c.SystemPrompt + "\n\n## Skills & Guidelines\n" + c.Skills
}
```

- [ ] **Step 4: Create `internal/models/schedule.go`**

```go
package models

import "time"

type Schedule struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	CronExpression string     `json:"cron_expression"`
	Action         string     `json:"action"`
	Enabled        bool       `json:"enabled"`
	LastRunAt      *time.Time `json:"last_run_at"`
	NextRunAt      *time.Time `json:"next_run_at"`
}
```

- [ ] **Step 5: Create `internal/models/theme.go`**

```go
package models

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
```

- [ ] **Step 6: Create `internal/models/request.go`**

```go
package models

import "encoding/json"

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
```

- [ ] **Step 7: Create `internal/models/response.go`**

```go
package models

type APIResponse struct {
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}
```

- [ ] **Step 8: Delete `internal/models/models.go`**

```bash
rm internal/models/models.go
```

- [ ] **Step 9: Build check**

Run: `cd /Users/jaochai/Code/video-fb && go build ./...`
Expected: BUILD SUCCESS — all imports still reference `models` package, no changes needed in consumers.

- [ ] **Step 10: Commit**

```bash
git add internal/models/
git commit -m "refactor: split models/models.go into domain files"
```

---

### Task 2: Add Settings Repository methods

**Files:**
- Modify: `internal/repository/settings.go`

- [ ] **Step 1: Add `GetCategories` and `GetBrandAliases` to `internal/repository/settings.go`**

Add after the existing `Set` method (line 54):

```go
func (r *SettingsRepo) GetCategories(ctx context.Context) ([]string, error) {
	raw, err := r.Get(ctx, "categories")
	if err != nil {
		return nil, fmt.Errorf("read categories setting: %w", err)
	}
	var categories []string
	if err := json.Unmarshal([]byte(raw), &categories); err != nil {
		return nil, fmt.Errorf("parse categories: %w", err)
	}
	if len(categories) == 0 {
		return nil, fmt.Errorf("no categories configured")
	}
	return categories, nil
}

func (r *SettingsRepo) GetBrandAliases(ctx context.Context) (map[string]string, error) {
	raw, err := r.Get(ctx, "brand_aliases")
	if err != nil {
		return map[string]string{}, nil
	}
	var aliases map[string]string
	if err := json.Unmarshal([]byte(raw), &aliases); err != nil {
		return map[string]string{}, nil
	}
	return aliases, nil
}
```

Add `"encoding/json"` to the import block.

- [ ] **Step 2: Build check**

Run: `cd /Users/jaochai/Code/video-fb && go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 3: Commit**

```bash
git add internal/repository/settings.go
git commit -m "feat: add GetCategories and GetBrandAliases to SettingsRepo"
```

---

### Task 3: Add `UpsertMetadata` to ClipsRepo

**Files:**
- Modify: `internal/repository/clips.go`

- [ ] **Step 1: Add `UpsertMetadata` method**

Add after `CountConsecutiveFailed` method (after line 162):

```go
func (r *ClipsRepo) UpsertMetadata(ctx context.Context, m models.ClipMetadata) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO clip_metadata (clip_id, youtube_title, youtube_description, youtube_tags)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (clip_id) DO UPDATE SET youtube_title=$2, youtube_description=$3, youtube_tags=$4`,
		m.ClipID, m.YoutubeTitle, m.YoutubeDesc, m.YoutubeTags)
	if err != nil {
		return fmt.Errorf("upsert metadata for clip %s: %w", m.ClipID, err)
	}
	return nil
}
```

- [ ] **Step 2: Build check**

Run: `cd /Users/jaochai/Code/video-fb && go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 3: Commit**

```bash
git add internal/repository/clips.go
git commit -m "feat: add UpsertMetadata to ClipsRepo"
```

---

### Task 4: KieAI Config Struct

**Files:**
- Modify: `internal/producer/kieai.go`
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Replace KieClient constructor with config-based injection**

In `internal/producer/kieai.go`, replace lines 22-34:

```go
type KieConfig struct {
	ImageTaskTimeout time.Duration
	VoiceTaskTimeout time.Duration
	PollInterval     time.Duration
	MaxRetries       int
	HTTPTimeout      time.Duration
	UploadTimeout    time.Duration
}

func DefaultKieConfig() KieConfig {
	return KieConfig{
		ImageTaskTimeout: 180 * time.Second,
		VoiceTaskTimeout: 300 * time.Second,
		PollInterval:     3 * time.Second,
		MaxRetries:       5,
		HTTPTimeout:      30 * time.Second,
		UploadTimeout:    5 * time.Minute,
	}
}

type KieClient struct {
	pool         *pgxpool.Pool
	cfg          KieConfig
	client       *http.Client
	uploadClient *http.Client
}

func NewKieClient(pool *pgxpool.Pool, cfg KieConfig) *KieClient {
	return &KieClient{
		pool:         pool,
		cfg:          cfg,
		client:       &http.Client{Timeout: cfg.HTTPTimeout},
		uploadClient: &http.Client{Timeout: cfg.UploadTimeout},
	}
}
```

- [ ] **Step 2: Replace hardcoded values with config fields**

In `GenerateImage` (line 81), replace `180*time.Second` with `k.cfg.ImageTaskTimeout`:
```go
result, err := k.pollTask(ctx, taskID, k.cfg.ImageTaskTimeout)
```

In `GenerateVoice` (line 106), replace `300*time.Second` with `k.cfg.VoiceTaskTimeout`:
```go
result, err := k.pollTask(ctx, taskID, k.cfg.VoiceTaskTimeout)
```

In `pollTask` (line 180 and 188 and 203), replace `3 * time.Second` with `k.cfg.PollInterval`:
```go
time.Sleep(k.cfg.PollInterval)
```
(3 occurrences in pollTask)

Replace the package-level `const maxRetries = 5` (line 243) and use `k.cfg.MaxRetries` in `retryableCall`. Change `retryableCall` to be a method on `KieClient`:

Replace lines 243-273:
```go
func (k *KieClient) retryableCall(ctx context.Context, operation string, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt <= k.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt) * 30 * time.Second
			log.Printf("[retry] %s attempt %d/%d after %v (error: %v)", operation, attempt, k.cfg.MaxRetries, backoff, lastErr)
			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return fmt.Errorf("%s cancelled during retry: %w", operation, ctx.Err())
			case <-timer.C:
			}
		}

		lastErr = fn()
		if lastErr == nil {
			if attempt > 0 {
				log.Printf("[retry] %s succeeded on attempt %d", operation, attempt)
			}
			return nil
		}

		if !isRetryable(lastErr) {
			return lastErr
		}
	}
	return fmt.Errorf("%s failed after %d retries: %w", operation, k.cfg.MaxRetries, lastErr)
}
```

- [ ] **Step 3: Update callers of `retryableCall` to use method receiver**

In `GenerateImage` (line 71): `retryableCall(ctx,` → `k.retryableCall(ctx,`
In `GenerateVoice` (line 96): `retryableCall(ctx,` → `k.retryableCall(ctx,`
In `UploadFile` (line 327): `retryableCall(ctx,` → `k.retryableCall(ctx,`

- [ ] **Step 4: Update `cmd/server/main.go` constructor call**

Line 82: Replace `producer.NewKieClient(pool)` with:
```go
kie := producer.NewKieClient(pool, producer.DefaultKieConfig())
```

- [ ] **Step 5: Build check**

Run: `cd /Users/jaochai/Code/video-fb && go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 6: Commit**

```bash
git add internal/producer/kieai.go cmd/server/main.go
git commit -m "refactor: inject KieConfig instead of hardcoded timeouts"
```

---

## Phase 2: Backend Orchestrator

### Task 5: Simplify Agent method signatures

**Files:**
- Modify: `internal/agent/question.go`
- Modify: `internal/agent/script.go`
- Modify: `internal/agent/image.go`
- Modify: `internal/orchestrator/orchestrator.go`
- Modify: `internal/models/agent.go`

- [ ] **Step 1: Refactor `ScriptAgent.Generate` to accept `*models.AgentConfig`**

In `internal/agent/script.go`, replace line 45:
```go
func (a *ScriptAgent) Generate(ctx context.Context, question, questionerName, category, model, systemPrompt string, temperature float64, promptTemplate string) (*GeneratedScript, error) {
```
with:
```go
func (a *ScriptAgent) Generate(ctx context.Context, question, questionerName, category string, cfg *models.AgentConfig) (*GeneratedScript, error) {
```

Update the body — replace references:
- `model` → `cfg.Model`
- `systemPrompt` → `cfg.BuildSystemPrompt()`
- `temperature` → `cfg.Temperature`
- `promptTemplate` → `cfg.PromptTemplate`

Line 57: `renderTemplate(promptTemplate,` → `renderTemplate(cfg.PromptTemplate,`
Line 68: `a.llm.GenerateJSON(ctx, model, systemPrompt, userPrompt, temperature,` → `a.llm.GenerateJSON(ctx, cfg.Model, cfg.BuildSystemPrompt(), userPrompt, cfg.Temperature,`

Add import: `"github.com/jaochai/video-fb/internal/models"`

- [ ] **Step 2: Refactor `ImageAgent.GeneratePrompts` to accept `*models.AgentConfig`**

In `internal/agent/image.go`, replace line 32:
```go
func (a *ImageAgent) GeneratePrompts(ctx context.Context, scenes []GeneratedScene, theme *models.BrandTheme, questionerName, model, systemPrompt string, temperature float64, promptTemplate string) ([]SceneImagePrompts, error) {
```
with:
```go
func (a *ImageAgent) GeneratePrompts(ctx context.Context, scenes []GeneratedScene, theme *models.BrandTheme, questionerName string, cfg *models.AgentConfig) ([]SceneImagePrompts, error) {
```

Update the body:
Line 43: `renderTemplate(promptTemplate,` → `renderTemplate(cfg.PromptTemplate,`
Line 55: `a.llm.GenerateJSON(ctx, model, systemPrompt, userPrompt, temperature,` → `a.llm.GenerateJSON(ctx, cfg.Model, cfg.BuildSystemPrompt(), userPrompt, cfg.Temperature,`

- [ ] **Step 3: Refactor `QuestionAgent.Generate` to accept `*models.AgentConfig`**

In `internal/agent/question.go`, replace line 37:
```go
func (a *QuestionAgent) Generate(ctx context.Context, count int, category, model, systemPrompt string, temperature float64, promptTemplate string) ([]GeneratedQuestion, error) {
```
with:
```go
func (a *QuestionAgent) Generate(ctx context.Context, count int, category string, cfg *models.AgentConfig) ([]GeneratedQuestion, error) {
```

Update the body:
Line 87: `renderTemplate(promptTemplate,` → `renderTemplate(cfg.PromptTemplate,`
Line 99: `a.llm.GenerateJSON(ctx, model, systemPrompt, userPrompt, temperature,` → `a.llm.GenerateJSON(ctx, cfg.Model, cfg.BuildSystemPrompt(), userPrompt, cfg.Temperature,`

Add import: `"github.com/jaochai/video-fb/internal/models"`

- [ ] **Step 4: Update orchestrator callers**

In `internal/orchestrator/orchestrator.go`:

Line 118 — replace:
```go
questions, err := o.questionAgent.Generate(ctx, count, category, qaCfg.Model, qaCfg.BuildSystemPrompt(), qaCfg.Temperature, qaCfg.PromptTemplate)
```
with:
```go
questions, err := o.questionAgent.Generate(ctx, count, category, qaCfg)
```

Line 202 — replace:
```go
script, err := o.scriptAgent.Generate(ctx, q.Question, q.QuestionerName, q.Category, scriptCfg.Model, scriptCfg.BuildSystemPrompt(), scriptCfg.Temperature, scriptCfg.PromptTemplate)
```
with:
```go
script, err := o.scriptAgent.Generate(ctx, q.Question, q.QuestionerName, q.Category, scriptCfg)
```

Line 227 — replace:
```go
imagePrompts, err := o.imageAgent.GeneratePrompts(ctx, script.Scenes, theme, q.QuestionerName, imageCfg.Model, imageCfg.BuildSystemPrompt(), imageCfg.Temperature, imageCfg.PromptTemplate)
```
with:
```go
imagePrompts, err := o.imageAgent.GeneratePrompts(ctx, script.Scenes, theme, q.QuestionerName, imageCfg)
```

Line 316 — replace:
```go
imagePrompts, err := o.imageAgent.GeneratePrompts(ctx, genScenes, theme, questionerName, imageCfg.Model, imageCfg.BuildSystemPrompt(), imageCfg.Temperature, imageCfg.PromptTemplate)
```
with:
```go
imagePrompts, err := o.imageAgent.GeneratePrompts(ctx, genScenes, theme, questionerName, imageCfg)
```

- [ ] **Step 5: Build check**

Run: `cd /Users/jaochai/Code/video-fb && go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 6: Commit**

```bash
git add internal/agent/ internal/orchestrator/orchestrator.go
git commit -m "refactor: simplify agent method signatures to accept AgentConfig"
```

---

### Task 6: Refactor Orchestrator — remove direct DB access + extract methods

**Files:**
- Modify: `internal/orchestrator/orchestrator.go`

- [ ] **Step 1: Add `settingsRepo` to Orchestrator struct and remove `pool`**

Replace the struct and constructor (lines 44-74):

```go
type Orchestrator struct {
	questionAgent *agent.QuestionAgent
	scriptAgent   *agent.ScriptAgent
	imageAgent    *agent.ImageAgent
	producer      *producer.Producer
	clipsRepo     *repository.ClipsRepo
	scenesRepo    *repository.ScenesRepo
	themesRepo    *repository.ThemesRepo
	agentsRepo    *repository.AgentsRepo
	settingsRepo  *repository.SettingsRepo
	tracker       *progress.Tracker
}

func New(
	qa *agent.QuestionAgent,
	sa *agent.ScriptAgent,
	ia *agent.ImageAgent,
	prod *producer.Producer,
	clips *repository.ClipsRepo,
	scenes *repository.ScenesRepo,
	themes *repository.ThemesRepo,
	agents *repository.AgentsRepo,
	settings *repository.SettingsRepo,
	tracker *progress.Tracker,
) *Orchestrator {
	return &Orchestrator{
		questionAgent: qa, scriptAgent: sa, imageAgent: ia,
		producer: prod, clipsRepo: clips, scenesRepo: scenes,
		themesRepo: themes, agentsRepo: agents, settingsRepo: settings,
		tracker: tracker,
	}
}
```

Remove `"github.com/jackc/pgx/v5/pgxpool"` from imports.

- [ ] **Step 2: Replace raw SQL in `ProduceWeekly` with repo methods**

Replace lines 79-102 (the categories and brand_aliases queries):

```go
categories, err := o.settingsRepo.GetCategories(ctx)
if err != nil {
	return fmt.Errorf("read categories: %w", err)
}
category := categories[weekNum%len(categories)]

brandAliases, err := o.settingsRepo.GetBrandAliases(ctx)
if err != nil {
	return fmt.Errorf("read brand aliases: %w", err)
}
```

- [ ] **Step 3: Replace raw SQL in `RetryClip` with repo method**

Replace lines 253-261 in `RetryClip`:
```go
brandAliases, err := o.settingsRepo.GetBrandAliases(ctx)
if err != nil {
	return o.failClip(ctx, clip.ID, fmt.Errorf("read brand aliases: %w", err))
}
```

- [ ] **Step 4: Replace `o.pool.Exec` in `produceClipWithID` with repo method**

Replace lines 241-245:
```go
o.clipsRepo.UpsertMetadata(ctx, models.ClipMetadata{
	ClipID:       clipID,
	YoutubeTitle: &script.YoutubeTitle,
	YoutubeDesc:  &script.YoutubeDescription,
	YoutubeTags:  script.YoutubeTags,
})
```

- [ ] **Step 5: Update `cmd/server/main.go` — pass `settingsRepo` instead of `pool`**

Line 94 — replace:
```go
orch := orchestrator.New(pool, questionAgent, scriptAgent, imageAgent, prod,
	clipsRepo, scenesRepo, themesRepo, agentsRepo, tracker)
```
with:
```go
settingsRepo := repository.NewSettingsRepo(pool)
orch := orchestrator.New(questionAgent, scriptAgent, imageAgent, prod,
	clipsRepo, scenesRepo, themesRepo, agentsRepo, settingsRepo, tracker)
```

Note: `settingsRepo` for the handler is already created at line 81 of router.go. The orchestrator gets its own instance — same pool, safe to share.

- [ ] **Step 6: Build check**

Run: `cd /Users/jaochai/Code/video-fb && go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 7: Commit**

```bash
git add internal/orchestrator/orchestrator.go cmd/server/main.go
git commit -m "refactor: remove direct DB access from orchestrator, use settingsRepo"
```

---

### Task 7: Move retry logic from handler to orchestrator

**Files:**
- Modify: `internal/orchestrator/orchestrator.go`
- Modify: `internal/handler/orchestrator.go`

- [ ] **Step 1: Add `RetryAllFailed` method to orchestrator**

Add to `internal/orchestrator/orchestrator.go` after the `RetryClip` method:

```go
func (o *Orchestrator) RetryAllFailed(ctx context.Context, maxRetries int) error {
	failed, err := o.clipsRepo.ListFailed(ctx, maxRetries)
	if err != nil {
		return fmt.Errorf("list failed: %w", err)
	}
	if len(failed) == 0 {
		return nil
	}

	o.tracker.StartProduction(len(failed))
	defer o.tracker.FinishProduction()

	for i, clip := range failed {
		if ctx.Err() != nil {
			o.tracker.AddErrorLog(fmt.Sprintf("Retry stopped at clip %d/%d", i+1, len(failed)))
			break
		}
		c := clip
		o.tracker.StartClip(i+1, c.Title)
		log.Printf("Retrying clip %s (%s)", c.ID, c.Title)
		if err := o.RetryClip(ctx, &c); err != nil {
			log.Printf("Retry failed for %s: %v", c.ID, err)
			o.tracker.AddErrorLog(fmt.Sprintf("Retry %s failed: %v", c.ID, err))
		} else {
			log.Printf("Retry succeeded for %s", c.ID)
			o.tracker.CompleteStep("complete")
		}
	}
	return nil
}
```

- [ ] **Step 2: Slim down `handler/orchestrator.go` RetryFailed**

Replace the entire `RetryFailed` method (lines 80-127) in `internal/handler/orchestrator.go`:

```go
func (h *OrchestratorHandler) RetryFailed(w http.ResponseWriter, r *http.Request) {
	if s := h.tracker.GetStatus(); s.Active {
		writeJSON(w, http.StatusConflict, models.APIResponse{Error: "Production already in progress"})
		return
	}

	writeJSON(w, http.StatusAccepted, models.APIResponse{
		Message: "Retrying failed clips in background",
	})

	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		h.tracker.SetCancelFunc(cancel)
		defer cancel()

		if err := h.orch.RetryAllFailed(ctx, 2); err != nil {
			log.Printf("Retry all failed: %v", err)
			h.tracker.AddErrorLog(err.Error())
		}
	}()
}
```

Remove the `clipsRepo` field from `OrchestratorHandler` struct and constructor since it's no longer needed:

Replace lines 18-27:
```go
type OrchestratorHandler struct {
	orch    *orchestrator.Orchestrator
	tracker *progress.Tracker
	pub     *publisher.Publisher
}

func NewOrchestratorHandler(orch *orchestrator.Orchestrator, tracker *progress.Tracker, pub *publisher.Publisher) *OrchestratorHandler {
	return &OrchestratorHandler{orch: orch, tracker: tracker, pub: pub}
}
```

Remove unused imports: `"fmt"`, `"strconv"` (check if `TriggerWeekly` still needs them — it uses `strconv.Atoi` so keep `strconv`; `fmt` is used in TriggerWeekly's error path — check and keep if needed).

Actually, `fmt` is not used in the handler anymore after removing RetryFailed's `fmt.Sprintf` calls. Let's check — `TriggerWeekly` doesn't use `fmt`. But we need `"log"` for the retry goroutine. Clean up imports to:

```go
import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/orchestrator"
	"github.com/jaochai/video-fb/internal/progress"
	"github.com/jaochai/video-fb/internal/publisher"
)
```

Remove `"github.com/jaochai/video-fb/internal/repository"` and `"fmt"` from imports.

- [ ] **Step 3: Update `cmd/server/main.go` — remove `clipsRepo` from handler constructor**

Line 129 — replace:
```go
orchHandler := handler.NewOrchestratorHandler(orch, tracker, pub, clipsRepo)
```
with:
```go
orchHandler := handler.NewOrchestratorHandler(orch, tracker, pub)
```

- [ ] **Step 4: Build check**

Run: `cd /Users/jaochai/Code/video-fb && go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 5: Commit**

```bash
git add internal/orchestrator/orchestrator.go internal/handler/orchestrator.go cmd/server/main.go
git commit -m "refactor: move retry logic from handler to orchestrator"
```

---

## Phase 3: Frontend Foundation

### Task 8: Improve `api.ts` error handling

**Files:**
- Modify: `frontend/src/api.ts`

- [ ] **Step 1: Replace `frontend/src/api.ts` with improved version**

```tsx
const API_BASE = import.meta.env.VITE_API_URL || 'https://adsvance-v2-production.up.railway.app';
const API_KEY = import.meta.env.VITE_API_KEY || '';

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
    this.name = 'ApiError';
  }
}

export async function apiFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(API_KEY && { Authorization: `Bearer ${API_KEY}` }),
      ...options?.headers,
    },
  });

  if (!res.ok) {
    const body = await res.json().catch(() => null);
    throw new ApiError(res.status, body?.error || res.statusText);
  }

  const json = await res.json();
  return json.data;
}
```

- [ ] **Step 2: Build check**

Run: `cd /Users/jaochai/Code/video-fb/frontend && npm run build`
Expected: BUILD SUCCESS — `ApiError` is a drop-in replacement, all existing `catch (e) { (e as Error).message }` patterns still work.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/api.ts
git commit -m "refactor: add ApiError class and HTTP status checking to api.ts"
```

---

### Task 9: Add route constants

**Files:**
- Create: `frontend/src/lib/routes.ts`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/components/sidebar.tsx`

- [ ] **Step 1: Create `frontend/src/lib/routes.ts`**

```tsx
export const ROUTES = {
  CONTENT: '/',
  SCHEDULES: '/schedules',
  ANALYTICS: '/analytics',
  KNOWLEDGE: '/knowledge',
  AGENTS: '/agents',
  PROMPT_HISTORY: '/prompt-history',
  SETTINGS: '/settings',
} as const;
```

- [ ] **Step 2: Update `frontend/src/components/sidebar.tsx` to use ROUTES**

Replace lines 1-28 imports and nav arrays:

```tsx
import { useState, useEffect } from "react"
import { NavLink } from "react-router-dom"
import { cn } from "../lib/utils"
import { ROUTES } from "../lib/routes"
import {
  LayoutDashboard,
  CalendarClock,
  BarChart3,
  BookOpen,
  Bot,
  History,
  Settings,
  Moon,
  Sun,
} from "lucide-react"
import { Button } from "./ui/button"

export const PIPELINE_NAV = [
  { to: ROUTES.CONTENT, label: "Content", icon: LayoutDashboard },
  { to: ROUTES.SCHEDULES, label: "Schedules", icon: CalendarClock },
  { to: ROUTES.ANALYTICS, label: "Analytics", icon: BarChart3 },
]

export const CONFIG_NAV = [
  { to: ROUTES.KNOWLEDGE, label: "Knowledge", icon: BookOpen },
  { to: ROUTES.AGENTS, label: "Agents", icon: Bot },
  { to: ROUTES.PROMPT_HISTORY, label: "Prompt History", icon: History },
  { to: ROUTES.SETTINGS, label: "Settings", icon: Settings },
]
```

- [ ] **Step 3: Update `frontend/src/App.tsx` to use ROUTES**

```tsx
import { BrowserRouter, Routes, Route } from "react-router-dom"
import { ErrorBoundary } from "./components/error-boundary"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { ToastProvider } from "./components/ui/toaster"
import { Sidebar } from "./components/sidebar"
import { MobileSidebar } from "./components/mobile-sidebar"
import { ROUTES } from "./lib/routes"
import ContentPage from "./pages/Content"
import AgentsPage from "./pages/Agents"
import KnowledgePage from "./pages/Knowledge"
import SchedulesPage from "./pages/Schedules"
import AnalyticsPage from "./pages/Analytics"
import SettingsPage from "./pages/Settings"
import PromptHistoryPage from "./pages/PromptHistory"

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      refetchOnWindowFocus: false,
    },
  },
})

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ToastProvider>
        <BrowserRouter>
          <div className="flex min-h-screen bg-background">
            <Sidebar />
            <div className="flex-1 flex flex-col">
              <MobileSidebar />
              <main className="flex-1 overflow-y-auto px-4 py-6 md:px-8 md:py-8 max-w-5xl">
                <ErrorBoundary>
                  <Routes>
                    <Route path={ROUTES.CONTENT} element={<ContentPage />} />
                    <Route path={ROUTES.SCHEDULES} element={<SchedulesPage />} />
                    <Route path={ROUTES.ANALYTICS} element={<AnalyticsPage />} />
                    <Route path={ROUTES.KNOWLEDGE} element={<KnowledgePage />} />
                    <Route path={ROUTES.AGENTS} element={<AgentsPage />} />
                    <Route path={ROUTES.PROMPT_HISTORY} element={<PromptHistoryPage />} />
                    <Route path={ROUTES.SETTINGS} element={<SettingsPage />} />
                  </Routes>
                </ErrorBoundary>
              </main>
            </div>
          </div>
        </BrowserRouter>
      </ToastProvider>
    </QueryClientProvider>
  )
}
```

- [ ] **Step 4: Build check**

Run: `cd /Users/jaochai/Code/video-fb/frontend && npm run build`
Expected: BUILD SUCCESS

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/routes.ts frontend/src/App.tsx frontend/src/components/sidebar.tsx
git commit -m "refactor: add route constants, use in App.tsx and sidebar"
```

---

### Task 10: Create `useEditableList` hook

**Files:**
- Create: `frontend/src/hooks/useEditableList.ts`

- [ ] **Step 1: Create `frontend/src/hooks/useEditableList.ts`**

```tsx
import { useState, useCallback } from 'react';

export function useEditableList<T extends Record<string, unknown>>(
  items: T[] | undefined,
  idKey: keyof T = 'id' as keyof T,
) {
  const [edits, setEdits] = useState<Record<string, Partial<T>>>({});
  const [dirty, setDirty] = useState<Record<string, boolean>>({});
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});

  const handleEdit = useCallback(
    (id: string, field: keyof T, value: T[keyof T]) => {
      setEdits(prev => ({ ...prev, [id]: { ...prev[id], [field]: value } as Partial<T> }));
      setDirty(prev => ({ ...prev, [id]: true }));
    },
    [],
  );

  const toggleExpand = useCallback((id: string) => {
    setExpanded(prev => ({ ...prev, [id]: !prev[id] }));
  }, []);

  const resetDirty = useCallback((id?: string) => {
    if (id) {
      setDirty(prev => ({ ...prev, [id]: false }));
    } else {
      setDirty({});
    }
  }, []);

  const getEdit = useCallback(
    (id: string) => edits[id] ?? ({} as Partial<T>),
    [edits],
  );

  const isDirty = useCallback((id: string) => dirty[id] ?? false, [dirty]);

  const isExpanded = useCallback((id: string) => expanded[id] ?? false, [expanded]);

  return {
    edits,
    setEdits,
    dirty,
    expanded,
    handleEdit,
    toggleExpand,
    resetDirty,
    getEdit,
    isDirty,
    isExpanded,
  };
}
```

- [ ] **Step 2: Build check**

Run: `cd /Users/jaochai/Code/video-fb/frontend && npm run build`
Expected: BUILD SUCCESS (new file, no consumers yet)

- [ ] **Step 3: Commit**

```bash
git add frontend/src/hooks/useEditableList.ts
git commit -m "feat: add useEditableList shared hook"
```

---

### Task 11: Create `useMutationWithToast` hook

**Files:**
- Create: `frontend/src/hooks/useMutationWithToast.ts`

- [ ] **Step 1: Create `frontend/src/hooks/useMutationWithToast.ts`**

```tsx
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useToast } from '../components/ui/toaster';

interface Options<TData, TVariables> {
  mutationFn: (variables: TVariables) => Promise<TData>;
  invalidateKeys?: string[][];
  successMsg: string;
  onSuccess?: () => void;
}

export function useMutationWithToast<TData = unknown, TVariables = void>(
  opts: Options<TData, TVariables>,
) {
  const qc = useQueryClient();
  const { success, error: showError } = useToast();

  return useMutation({
    mutationFn: opts.mutationFn,
    onSuccess: () => {
      if (opts.invalidateKeys) {
        opts.invalidateKeys.forEach(key => qc.invalidateQueries({ queryKey: key }));
      }
      success(opts.successMsg);
      opts.onSuccess?.();
    },
    onError: (e: Error) => showError(`${opts.successMsg.replace('แล้ว', 'ล้มเหลว')}: ${e.message}`),
  });
}
```

- [ ] **Step 2: Build check**

Run: `cd /Users/jaochai/Code/video-fb/frontend && npm run build`
Expected: BUILD SUCCESS

- [ ] **Step 3: Commit**

```bash
git add frontend/src/hooks/useMutationWithToast.ts
git commit -m "feat: add useMutationWithToast shared hook"
```

---

## Phase 4: Frontend Pages

### Task 12: Unify sidebar — merge mobile into desktop

**Files:**
- Modify: `frontend/src/components/sidebar.tsx`
- Delete: `frontend/src/components/mobile-sidebar.tsx`
- Modify: `frontend/src/App.tsx`

- [ ] **Step 1: Add mobile sidebar into `frontend/src/components/sidebar.tsx`**

Add `MobileSidebar` export after the existing `Sidebar` function. Replace `PIPELINE_NAV` and `CONFIG_NAV` references with the already-exported constants:

```tsx
export function MobileSidebar() {
  const [open, setOpen] = useState(false)

  return (
    <>
      <header className="md:hidden sticky top-0 z-40 flex items-center gap-3 border-b bg-background px-4 py-3">
        <Button variant="ghost" size="icon" onClick={() => setOpen(true)} className="cursor-pointer">
          <Menu className="h-5 w-5" />
        </Button>
        <span className="text-sm font-semibold">Ads Vance</span>
      </header>

      {open && (
        <>
          <div className="fixed inset-0 z-50 bg-black/50" onClick={() => setOpen(false)} />
          <aside className="fixed inset-y-0 left-0 z-50 w-[280px] bg-sidebar flex flex-col">
            <div className="flex items-center justify-between px-6 py-5">
              <span className="text-lg font-bold tracking-tight text-sidebar-foreground">
                Ads Vance
              </span>
              <Button
                variant="ghost"
                size="icon"
                onClick={() => setOpen(false)}
                className="text-sidebar-foreground/60 hover:text-sidebar-foreground cursor-pointer"
              >
                <X className="h-5 w-5" />
              </Button>
            </div>
            <nav className="flex-1 px-3 py-2 space-y-6">
              <NavSection label="Pipeline" items={PIPELINE_NAV} onItemClick={() => setOpen(false)} />
              <NavSection label="Configuration" items={CONFIG_NAV} onItemClick={() => setOpen(false)} />
            </nav>
          </aside>
        </>
      )}
    </>
  )
}
```

Add `Menu` and `X` to the lucide-react imports:
```tsx
import {
  LayoutDashboard, CalendarClock, BarChart3, BookOpen, Bot, History, Settings,
  Moon, Sun, Menu, X,
} from "lucide-react"
```

- [ ] **Step 2: Delete `frontend/src/components/mobile-sidebar.tsx`**

```bash
rm frontend/src/components/mobile-sidebar.tsx
```

- [ ] **Step 3: Update import in `frontend/src/App.tsx`**

Replace:
```tsx
import { MobileSidebar } from "./components/mobile-sidebar"
```
with:
```tsx
import { MobileSidebar } from "./components/sidebar"
```

(The named import `{ MobileSidebar }` stays the same — just the path changes.)

- [ ] **Step 4: Build check**

Run: `cd /Users/jaochai/Code/video-fb/frontend && npm run build`
Expected: BUILD SUCCESS

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/sidebar.tsx frontend/src/App.tsx
git rm frontend/src/components/mobile-sidebar.tsx
git commit -m "refactor: unify sidebar — merge mobile into desktop component"
```

---

### Task 13: Split Settings.tsx into sub-components

**Files:**
- Create: `frontend/src/components/settings/ApiKeysCard.tsx`
- Create: `frontend/src/components/settings/VoiceSettingsCard.tsx`
- Create: `frontend/src/components/settings/ConnectedAccountsCard.tsx`
- Create: `frontend/src/components/settings/AgentModelsCard.tsx`
- Modify: `frontend/src/pages/Settings.tsx`

- [ ] **Step 1: Create `frontend/src/components/settings/ApiKeysCard.tsx`**

```tsx
import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { apiFetch } from '../../api';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '../ui/card';
import { Button } from '../ui/button';
import { Input } from '../ui/input';

interface TestResult {
  data?: {
    label?: string;
    limit_remaining?: number | null;
    is_free_tier?: boolean;
  };
  error?: string;
}

const API_KEY_FIELDS = [
  { key: 'openrouter_api_key', label: 'OpenRouter API Key', placeholder: 'sk-or-v1-...', testable: true },
  { key: 'kie_api_key', label: 'Kie AI API Key (Upload)', placeholder: 'kie-...' },
  { key: 'zernio_api_key', label: 'Zernio API Key', placeholder: 'zrn-...' },
];

interface Props {
  form: Record<string, string>;
  dirty: boolean;
  onSave: () => void;
  saving: boolean;
  saved: boolean;
  onChange: (key: string, value: string) => void;
}

export function ApiKeysCard({ form, dirty, onSave, saving, saved, onChange }: Props) {
  const [showKeys, setShowKeys] = useState<Record<string, boolean>>({});
  const [testResult, setTestResult] = useState<TestResult | null>(null);

  const testKey = useMutation({
    mutationFn: (key: string) =>
      apiFetch<TestResult>('/api/v1/settings/test-key', { method: 'POST', body: JSON.stringify({ key }) }),
    onSuccess: (data) => setTestResult(data as unknown as TestResult),
    onError: (err) => setTestResult({ error: (err as Error).message }),
  });

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">API Keys</CardTitle>
        <CardDescription>Configure API keys for external services</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {API_KEY_FIELDS.map(({ key, label, placeholder, testable }) => (
          <div key={key}>
            <label className="block text-xs text-muted-foreground uppercase tracking-wide mb-1.5">
              {label}
            </label>
            <div className="flex gap-2">
              <Input
                type={showKeys[key] ? 'text' : 'password'}
                value={form[key] ?? ''}
                placeholder={placeholder}
                onChange={e => onChange(key, e.target.value)}
                className="flex-1"
              />
              <Button
                variant="outline"
                size="sm"
                onClick={() => setShowKeys(prev => ({ ...prev, [key]: !prev[key] }))}
              >
                {showKeys[key] ? 'Hide' : 'Show'}
              </Button>
              {testable && (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => testKey.mutate(form[key] ?? '')}
                  disabled={testKey.isPending || !form[key]}
                >
                  {testKey.isPending ? 'Testing...' : 'Test'}
                </Button>
              )}
            </div>
            {testable && testResult && (
              <div className={`mt-2 px-3 py-2 rounded-md text-xs border ${
                testResult.error
                  ? 'bg-destructive/10 text-destructive border-destructive/20'
                  : 'bg-green-500/10 text-green-500 border-green-500/20'
              }`}>
                {testResult.error ? `Failed: ${testResult.error}` : (
                  <span className="flex gap-4">
                    <span>Connected</span>
                    {testResult.data?.label && <span>Label: {testResult.data.label}</span>}
                    {testResult.data?.limit_remaining != null && <span>Credits: {testResult.data.limit_remaining.toLocaleString()}</span>}
                    <span>{testResult.data?.is_free_tier ? 'Free' : 'Paid'}</span>
                  </span>
                )}
              </div>
            )}
          </div>
        ))}

        <div className="flex items-center gap-3 pt-2">
          <Button onClick={onSave} disabled={saving || !dirty}>
            {saving ? 'Saving...' : 'Save'}
          </Button>
          {saved && <span className="text-xs text-green-500">Saved</span>}
        </div>
      </CardContent>
    </Card>
  );
}
```

- [ ] **Step 2: Create `frontend/src/components/settings/VoiceSettingsCard.tsx`**

```tsx
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '../ui/card';

const TTS_VOICES = [
  'Achird', 'Charon', 'Sadaltager', 'Sulafat', 'Rasalgethi',
  'Puck', 'Iapetus', 'Schedar', 'Gacrux', 'Algieba',
  'Zephyr', 'Kore', 'Fenrir', 'Leda', 'Orus', 'Aoede',
  'Achernar', 'Alnilam', 'Despina', 'Erinome',
];

interface Props {
  value: string;
  onChange: (value: string) => void;
}

export function VoiceSettingsCard({ value, onChange }: Props) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Voice Settings</CardTitle>
        <CardDescription>Select the TTS voice for video narration</CardDescription>
      </CardHeader>
      <CardContent>
        <label className="block text-xs text-muted-foreground uppercase tracking-wide mb-1.5">
          TTS Voice (Gemini)
        </label>
        <select
          value={value}
          onChange={e => onChange(e.target.value)}
          className="h-10 w-full max-w-xs rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 cursor-pointer"
        >
          {TTS_VOICES.map(v => (
            <option key={v} value={v}>{v}</option>
          ))}
        </select>
      </CardContent>
    </Card>
  );
}
```

- [ ] **Step 3: Create `frontend/src/components/settings/ConnectedAccountsCard.tsx`**

```tsx
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '../ui/card';
import { Badge } from '../ui/badge';
import { Skeleton } from '../ui/skeleton';

interface ZernioAccount {
  _id: string;
  platform: string;
  displayName: string;
  username: string;
  profilePicture: string;
  followersCount: number;
  isActive: boolean;
  metadata?: {
    profileData?: {
      extraData?: {
        totalViews?: number;
        videoCount?: number;
      };
    };
  };
}

interface Props {
  accounts: ZernioAccount[] | undefined;
  selectedId: string | undefined;
  loading: boolean;
}

export function ConnectedAccountsCard({ accounts, selectedId, loading }: Props) {
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-3">
          <CardTitle className="text-base">Connected Accounts</CardTitle>
          <Badge variant="secondary" className="text-[10px]">via Zernio</Badge>
        </div>
        <CardDescription>YouTube and social media accounts connected through Zernio</CardDescription>
      </CardHeader>
      <CardContent>
        {loading ? (
          <div className="space-y-3">
            {[1, 2].map(i => (
              <div key={i} className="flex items-center gap-3.5 rounded-lg border p-3.5">
                <Skeleton className="w-10 h-10 rounded-full" />
                <div className="flex-1 space-y-1.5">
                  <Skeleton className="h-4 w-32" />
                  <Skeleton className="h-3 w-48" />
                </div>
                <Skeleton className="h-5 w-20 rounded-full" />
              </div>
            ))}
          </div>
        ) : accounts?.length ? (
          <div className="grid gap-3">
            {accounts.filter(a => a._id === selectedId).map(account => {
              const extra = account.metadata?.profileData?.extraData;
              return (
                <div
                  key={account._id}
                  className="flex items-center gap-3.5 rounded-lg p-3.5 border bg-green-500/5 border-green-500/30"
                >
                  <img src={account.profilePicture} alt="" className="w-10 h-10 rounded-full" />
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-medium">{account.displayName}</span>
                      <Badge variant="secondary" className="text-[10px] bg-green-500/15 text-green-500 border-0">
                        Active
                      </Badge>
                    </div>
                    <div className="flex gap-3 mt-1">
                      <span className="text-[11px] text-muted-foreground">@{account.username}</span>
                      <span className="text-[11px] text-muted-foreground capitalize">{account.platform}</span>
                      {account.followersCount > 0 && (
                        <span className="text-[11px] text-muted-foreground">{account.followersCount.toLocaleString()} subs</span>
                      )}
                      {extra?.videoCount != null && (
                        <span className="text-[11px] text-muted-foreground">{extra.videoCount} videos</span>
                      )}
                    </div>
                  </div>
                  <Badge
                    variant={account.isActive ? 'secondary' : 'destructive'}
                    className={`text-[10px] shrink-0 ${
                      account.isActive
                        ? 'bg-green-500/10 text-green-500 border-0'
                        : 'bg-red-500/10 text-red-500 border-0'
                    }`}
                  >
                    {account.isActive ? 'Connected' : 'Disconnected'}
                  </Badge>
                </div>
              );
            })}
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">
            No channels connected. Set Zernio API Key above and connect channels in Zernio dashboard.
          </p>
        )}
      </CardContent>
    </Card>
  );
}
```

- [ ] **Step 4: Create `frontend/src/components/settings/AgentModelsCard.tsx`**

```tsx
import { useState, useEffect } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { apiFetch } from '../../api';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '../ui/card';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import { useToast } from '../ui/toaster';

interface Agent {
  id: string;
  agent_name: string;
  system_prompt: string;
  model: string;
  temperature: number;
  enabled: boolean;
  skills: string;
}

interface Props {
  agents: Agent[] | undefined;
}

export function AgentModelsCard({ agents }: Props) {
  const qc = useQueryClient();
  const { success, error: showError } = useToast();
  const [models, setModels] = useState<Record<string, string>>({});
  const [dirty, setDirty] = useState<Record<string, boolean>>({});

  useEffect(() => {
    if (agents) {
      const m: Record<string, string> = {};
      agents.forEach(a => { m[a.id] = a.model; });
      setModels(m);
    }
  }, [agents]);

  const save = useMutation({
    mutationFn: ({ id, agent }: { id: string; agent: Agent }) =>
      apiFetch(`/api/v1/agents/${id}`, {
        method: 'PATCH',
        body: JSON.stringify({
          system_prompt: agent.system_prompt,
          model: models[id],
          temperature: agent.temperature,
          enabled: agent.enabled,
          skills: agent.skills,
        }),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['agents'] });
      setDirty({});
      success('บันทึก model แล้ว');
    },
    onError: (e) => showError(`บันทึก model ล้มเหลว: ${(e as Error).message}`),
  });

  const handleChange = (id: string, value: string) => {
    setModels(prev => ({ ...prev, [id]: value }));
    setDirty(prev => ({ ...prev, [id]: true }));
  };

  const handleSave = () => {
    if (!agents) return;
    for (const agent of agents) {
      if (dirty[agent.id]) {
        save.mutate({ id: agent.id, agent });
      }
    }
  };

  const hasDirty = Object.values(dirty).some(Boolean);

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-3">
          <CardTitle className="text-base">Agent Models</CardTitle>
          <Badge variant="secondary" className="text-[10px]">Assign model per agent</Badge>
        </div>
        <CardDescription>Configure which LLM model each agent uses</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        {agents?.map(agent => (
          <div key={agent.id} className="flex items-center gap-3 rounded-lg border bg-card p-3.5">
            <div className="w-[120px] shrink-0">
              <span className="text-sm font-medium">{agent.agent_name}</span>
            </div>
            <Input
              value={models[agent.id] ?? agent.model}
              onChange={e => handleChange(agent.id, e.target.value)}
              placeholder="openai/gpt-4.1"
              className="flex-1 text-[13px]"
            />
            <Badge
              variant={agent.enabled ? 'secondary' : 'destructive'}
              className={`text-[10px] shrink-0 ${
                agent.enabled
                  ? 'bg-green-500/10 text-green-500 border-0'
                  : 'bg-red-500/10 text-red-500 border-0'
              }`}
            >
              {agent.enabled ? 'ON' : 'OFF'}
            </Badge>
          </div>
        ))}

        <div className="flex items-center gap-3 pt-1">
          <Button onClick={handleSave} disabled={save.isPending || !hasDirty}>
            {save.isPending ? 'Saving...' : 'Save Models'}
          </Button>
          {save.isSuccess && <span className="text-xs text-green-500">Saved</span>}
        </div>
      </CardContent>
    </Card>
  );
}
```

- [ ] **Step 5: Rewrite `frontend/src/pages/Settings.tsx` to compose sub-components**

```tsx
import { useState, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiFetch } from '../api';
import { PageHeader } from '../components/page-header';
import { useToast } from '../components/ui/toaster';
import { ApiKeysCard } from '../components/settings/ApiKeysCard';
import { VoiceSettingsCard } from '../components/settings/VoiceSettingsCard';
import { ConnectedAccountsCard } from '../components/settings/ConnectedAccountsCard';
import { AgentModelsCard } from '../components/settings/AgentModelsCard';

interface Agent {
  id: string;
  agent_name: string;
  system_prompt: string;
  model: string;
  temperature: number;
  enabled: boolean;
  skills: string;
}

interface ZernioData {
  accounts: Array<{
    _id: string;
    platform: string;
    displayName: string;
    username: string;
    profilePicture: string;
    followersCount: number;
    isActive: boolean;
    metadata?: {
      profileData?: {
        extraData?: {
          totalViews?: number;
          videoCount?: number;
        };
      };
    };
  }>;
  hasAnalyticsAccess: boolean;
}

export default function SettingsPage() {
  const qc = useQueryClient();
  const { success, error: showError } = useToast();

  const { data: saved } = useQuery({ queryKey: ['settings'], queryFn: () => apiFetch<Record<string, string>>('/api/v1/settings') });
  const { data: agents } = useQuery({ queryKey: ['agents'], queryFn: () => apiFetch<Agent[]>('/api/v1/agents') });
  const { data: zernioData, isLoading: zernioLoading } = useQuery({
    queryKey: ['zernio-accounts'],
    queryFn: () => apiFetch<ZernioData>('/api/v1/settings/test-zernio'),
    retry: false,
  });

  const [form, setForm] = useState<Record<string, string>>({});
  const [dirty, setDirty] = useState<Record<string, boolean>>({});

  useEffect(() => {
    if (saved) setForm(saved);
  }, [saved]);

  const save = useMutation({
    mutationFn: (data: Record<string, string>) =>
      apiFetch('/api/v1/settings', { method: 'PUT', body: JSON.stringify(data) }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['settings'] }); setDirty({}); success('บันทึกการตั้งค่าแล้ว'); },
    onError: (e) => showError(`บันทึกล้มเหลว: ${(e as Error).message}`),
  });

  const handleChange = (key: string, value: string) => {
    setForm(prev => ({ ...prev, [key]: value }));
    setDirty(prev => ({ ...prev, [key]: true }));
  };

  const handleSave = () => {
    const updates: Record<string, string> = {};
    for (const key of Object.keys(dirty)) {
      if (dirty[key]) updates[key] = form[key] ?? '';
    }
    if (Object.keys(updates).length > 0) save.mutate(updates);
  };

  const hasDirty = Object.values(dirty).some(Boolean);

  return (
    <div>
      <PageHeader title="Settings" />
      <div className="grid gap-6 max-w-2xl">
        <ApiKeysCard
          form={form}
          dirty={hasDirty}
          onSave={handleSave}
          saving={save.isPending}
          saved={save.isSuccess}
          onChange={handleChange}
        />
        <VoiceSettingsCard
          value={form['elevenlabs_voice'] ?? ''}
          onChange={v => handleChange('elevenlabs_voice', v)}
        />
        <ConnectedAccountsCard
          accounts={zernioData?.accounts}
          selectedId={saved?.zernio_youtube_account_id}
          loading={zernioLoading}
        />
        <AgentModelsCard agents={agents} />
      </div>
    </div>
  );
}
```

- [ ] **Step 6: Build check**

Run: `cd /Users/jaochai/Code/video-fb/frontend && npm run build`
Expected: BUILD SUCCESS

- [ ] **Step 7: Commit**

```bash
git add frontend/src/components/settings/ frontend/src/pages/Settings.tsx
git commit -m "refactor: split Settings.tsx into ApiKeys, Voice, Accounts, AgentModels cards"
```

---

### Task 14: Apply `useEditableList` hook to Agents page

**Files:**
- Modify: `frontend/src/pages/Agents.tsx`

- [ ] **Step 1: Refactor Agents.tsx to use `useEditableList`**

```tsx
import { useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiFetch } from '../api';
import { ChevronDown } from 'lucide-react';
import { PageHeader } from '../components/page-header';
import { Card, CardHeader, CardContent } from '../components/ui/card';
import { Switch } from '../components/ui/switch';
import { Badge } from '../components/ui/badge';
import { Button } from '../components/ui/button';
import { Input } from '../components/ui/input';
import { Textarea } from '../components/ui/textarea';
import { useToast } from '../components/ui/toaster';
import { Skeleton } from '../components/ui/skeleton';
import { useEditableList } from '../hooks/useEditableList';

interface Agent {
  id: string;
  agent_name: string;
  system_prompt: string;
  prompt_template: string;
  model: string;
  temperature: number;
  enabled: boolean;
  skills: string;
}

const TEMPLATE_VARS: Record<string, string[]> = {
  question: ['Count', 'Category', 'RAGContext', 'PreviousTopics', 'PreviousNames'],
  script: ['Question', 'QuestionerName', 'Category', 'RAGContext'],
  image: ['ThemeDescription', 'QuestionerName', 'QuestionText', 'PrimaryColor', 'AccentColor'],
};

function getTemplateVars(agentName: string): string[] {
  return TEMPLATE_VARS[agentName] ?? [];
}

export default function AgentsPage() {
  const qc = useQueryClient();
  const { success, error: showError } = useToast();
  const { data: agents, isLoading } = useQuery({
    queryKey: ['agents'],
    queryFn: () => apiFetch<Agent[]>('/api/v1/agents'),
  });

  const { edits, setEdits, handleEdit, toggleExpand, isDirty, isExpanded, getEdit, resetDirty } = useEditableList<Agent>(agents);

  useEffect(() => {
    if (agents) {
      const initial: Record<string, Partial<Agent>> = {};
      agents.forEach((a) => {
        initial[a.id] = {
          system_prompt: a.system_prompt,
          prompt_template: a.prompt_template ?? '',
          skills: a.skills ?? '',
          temperature: a.temperature,
          enabled: a.enabled,
          model: a.model,
        };
      });
      setEdits(initial);
    }
  }, [agents, setEdits]);

  const update = useMutation({
    mutationFn: ({ id, agent }: { id: string; agent: Agent }) => {
      const e = getEdit(id);
      return apiFetch(`/api/v1/agents/${id}`, {
        method: 'PATCH',
        body: JSON.stringify({
          system_prompt: e.system_prompt ?? agent.system_prompt,
          prompt_template: e.prompt_template ?? agent.prompt_template ?? '',
          model: e.model ?? agent.model,
          temperature: e.temperature ?? agent.temperature,
          enabled: e.enabled ?? agent.enabled,
          skills: e.skills ?? agent.skills ?? '',
        }),
      });
    },
    onSuccess: (_data, { id, agent }) => {
      qc.invalidateQueries({ queryKey: ['agents'] });
      resetDirty(id);
      success(`บันทึก ${agent.agent_name} แล้ว`);
    },
    onError: (e) => showError(`บันทึกล้มเหลว: ${(e as Error).message}`),
  });

  return (
    <div>
      <PageHeader title="Agents" description="Configure AI agent prompts and models" />

      {isLoading ? (
        <div className="grid gap-4">
          {[1, 2, 3, 4].map(i => (
            <div key={i} className="rounded-xl border p-4">
              <div className="flex justify-between">
                <div className="flex gap-3">
                  <Skeleton className="h-5 w-20" />
                  <Skeleton className="h-5 w-40 rounded-full" />
                </div>
                <Skeleton className="h-6 w-12 rounded-full" />
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className="grid gap-4">
          {agents?.map((agent) => {
            const e = getEdit(agent.id);

            return (
              <Card key={agent.id}>
                <CardHeader
                  className="cursor-pointer select-none"
                  onClick={() => toggleExpand(agent.id)}
                >
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                      <span className="text-sm font-medium capitalize">
                        {agent.agent_name}
                      </span>
                      <Badge variant="outline">{e.model ?? agent.model}</Badge>
                    </div>
                    <div className="flex items-center gap-3">
                      <Switch
                        checked={e.enabled ?? agent.enabled}
                        onCheckedChange={(checked) => handleEdit(agent.id, 'enabled', checked as Agent['enabled'])}
                        disabled={update.isPending}
                      />
                      <ChevronDown
                        className={`h-4 w-4 text-muted-foreground transition-transform duration-200 ${
                          isExpanded(agent.id) ? 'rotate-180' : ''
                        }`}
                      />
                    </div>
                  </div>
                </CardHeader>

                {isExpanded(agent.id) && (
                  <CardContent className="grid gap-4">
                    <div className="grid gap-2">
                      <label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                        System Prompt
                      </label>
                      <Textarea
                        rows={8}
                        value={e.system_prompt ?? agent.system_prompt}
                        onChange={(ev) => handleEdit(agent.id, 'system_prompt', ev.target.value as Agent['system_prompt'])}
                      />
                    </div>

                    <div className="grid gap-2">
                      <label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                        Prompt Template
                      </label>
                      <p className="text-xs text-muted-foreground">
                        ใช้ {'{{.VariableName}}'} สำหรับตัวแปร — ระบบจะแทนที่ให้อัตโนมัติตอนรัน
                      </p>
                      <Textarea
                        rows={12}
                        className="font-mono text-xs"
                        value={e.prompt_template ?? agent.prompt_template ?? ''}
                        onChange={(ev) => handleEdit(agent.id, 'prompt_template', ev.target.value as Agent['prompt_template'])}
                      />
                      <div className="flex flex-wrap gap-1.5">
                        {getTemplateVars(agent.agent_name).map((v) => (
                          <span key={v} className="px-1.5 py-0.5 rounded bg-muted text-[10px] font-mono text-muted-foreground">
                            {`{{.${v}}}`}
                          </span>
                        ))}
                      </div>
                    </div>

                    <div className="grid gap-2">
                      <label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Model</label>
                      <Input
                        value={e.model ?? agent.model}
                        onChange={(ev) => handleEdit(agent.id, 'model', ev.target.value as Agent['model'])}
                      />
                    </div>

                    <div className="grid gap-2">
                      <label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Temperature</label>
                      <Input
                        type="number"
                        step={0.1}
                        min={0}
                        max={1}
                        value={e.temperature ?? agent.temperature}
                        onChange={(ev) => handleEdit(agent.id, 'temperature', parseFloat(ev.target.value) as Agent['temperature'])}
                      />
                    </div>

                    <div className="grid gap-2">
                      <label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Skills</label>
                      <Textarea
                        rows={3}
                        value={e.skills ?? agent.skills ?? ''}
                        onChange={(ev) => handleEdit(agent.id, 'skills', ev.target.value as Agent['skills'])}
                        placeholder="Define what this agent can do..."
                      />
                    </div>

                    <div className="flex items-center gap-3">
                      {isDirty(agent.id) && (
                        <Button
                          onClick={() => update.mutate({ id: agent.id, agent })}
                          disabled={update.isPending}
                        >
                          {update.isPending ? 'Saving...' : 'Save'}
                        </Button>
                      )}
                      {update.isSuccess && !isDirty(agent.id) && (
                        <span className="text-xs text-green-500">Saved</span>
                      )}
                    </div>
                  </CardContent>
                )}
              </Card>
            );
          })}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Build check**

Run: `cd /Users/jaochai/Code/video-fb/frontend && npm run build`
Expected: BUILD SUCCESS

- [ ] **Step 3: Commit**

```bash
git add frontend/src/pages/Agents.tsx
git commit -m "refactor: apply useEditableList hook to Agents page"
```

---

### Task 15: Final build verification

**Files:** None (verification only)

- [ ] **Step 1: Full backend build**

Run: `cd /Users/jaochai/Code/video-fb && go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 2: Full frontend build**

Run: `cd /Users/jaochai/Code/video-fb/frontend && npm run build`
Expected: BUILD SUCCESS

- [ ] **Step 3: Run existing tests**

Run: `cd /Users/jaochai/Code/video-fb && go test ./...`
Expected: PASS (sanitize_test.go should still pass)

- [ ] **Step 4: Review total changes**

Run: `git diff --stat master~N..HEAD` (where N = number of refactor commits)
Verify: all expected files changed, no unexpected deletions.
