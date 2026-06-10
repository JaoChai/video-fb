# Migrate Existing Agents to kie.ai (Plan 2a) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move every existing LLM agent (research, script, image, question, analytics) off the OpenRouter `LLMClient` onto the live-verified `KieLLMClient` (Claude Sonnet 4.6 / Gemini 3.5 Flash), and switch research to Gemini `googleSearch` grounding — with no behavioral/flow change so the orchestrator keeps working and master stays green.

**Architecture:** `KieLLMClient` (from Plan 1, merged to master) exposes the same `Generate`/`GenerateJSON` surface the agents already use, plus `GenerateWithSearch`. This plan is a **mechanical client swap**: change each agent's `*LLMClient` field/constructor param to `*KieLLMClient`, wire `agent.NewKieLLMClient(pool)` in `main.go`, point `agent_configs.model` rows at kie.ai model names via a migration, and have research call `GenerateWithSearch`. The pipeline shape (questions → script → image prompts → produce) is untouched — that structural redesign is Plan 2b.

**Tech Stack:** Go 1.25, pgx/pgxpool, Postgres (Neon), file-based migrations auto-applied on startup (`internal/database/migrations.go`, ordered by filename).

**Scope:** Backend only. No new agents, no orchestrator flow change, no frontend change (those are Plan 2b). The OpenRouter `LLMClient` struct in `internal/agent/llm.go` becomes unused after this but **stays** because `kiellm.go` reuses its `extractJSON` helper; do not delete it (cleanup is tracked for Plan 3). The producer's separate `OpenRouterClient` (TTS/image) is unrelated and untouched.

---

## File Structure

- **Create:** `migrations/029_agents_to_kie.sql` — repoint `agent_configs.model` for research/script/image/question/analytics to kie.ai model names.
- **Modify:** `cmd/server/main.go:75-80` — construct `KieLLMClient`, pass it to the agents.
- **Modify:** `internal/agent/research.go` — field/param type → `*KieLLMClient`; call `GenerateWithSearch`.
- **Modify:** `internal/agent/script.go` — field/param type → `*KieLLMClient`.
- **Modify:** `internal/agent/image.go` — field/param type → `*KieLLMClient`.
- **Modify:** `internal/agent/question.go` — field/param type → `*KieLLMClient`.
- **Modify:** `internal/analyzer/analyzer.go` — field/param type → `*agent.KieLLMClient`.

Each agent file keeps its single responsibility; only the LLM client type changes.

---

## Task 1: Migration — point agent models at kie.ai

**Files:**
- Create: `migrations/029_agents_to_kie.sql`

Model assignment per spec §4.6: research/image/question/analytics → `gemini-3-5-flash` (cheap/mechanical), script → `claude-sonnet-4-6` (writing quality). `question` is repointed (not removed) so it keeps working until Plan 2b retires it. `dedup` uses embeddings (no `agent_configs.model` for chat) — not touched.

- [ ] **Step 1: Write the migration**

Create `migrations/029_agents_to_kie.sql`:

```sql
-- Migration 029: Move all chat agents onto kie.ai models.
-- KieLLMClient routes by model-name prefix: claude-* -> kie.ai Claude endpoint,
-- gemini-* -> kie.ai Gemini endpoint. Research uses Gemini googleSearch grounding
-- (replaces perplexity/sonar). Idempotent: re-running just re-sets the same values.

UPDATE agent_configs SET model = 'gemini-3-5-flash' WHERE agent_name = 'research';
UPDATE agent_configs SET model = 'claude-sonnet-4-6' WHERE agent_name = 'script';
UPDATE agent_configs SET model = 'gemini-3-5-flash' WHERE agent_name = 'image';
UPDATE agent_configs SET model = 'gemini-3-5-flash' WHERE agent_name = 'question';
UPDATE agent_configs SET model = 'gemini-3-5-flash' WHERE agent_name = 'analytics';
```

- [ ] **Step 2: Apply on a Neon dev branch to verify it runs (safe-migration practice)**

Use the Neon MCP. Create a temporary branch off `adsvance-v2` (project `snowy-grass-75448787`), run the migration SQL against the branch, then confirm the values:

Run (MCP `run_sql` against the temp branch):
```sql
SELECT agent_name, model FROM agent_configs
WHERE agent_name IN ('research','script','image','question','analytics')
ORDER BY agent_name;
```
Expected rows: `analytics=gemini-3-5-flash`, `image=gemini-3-5-flash`, `question=gemini-3-5-flash`, `research=gemini-3-5-flash`, `script=claude-sonnet-4-6`. Delete the temp branch after verifying. (Production picks the file up automatically on next deploy via `RunMigrations`.)

- [ ] **Step 3: Commit**

```bash
git add migrations/029_agents_to_kie.sql
git commit -m "feat(db): migration 029 — point chat agents at kie.ai models"
```

---

## Task 2: Wire KieLLMClient in main.go + migrate research agent

**Files:**
- Modify: `cmd/server/main.go:75-80`
- Modify: `internal/agent/research.go`

- [ ] **Step 1: Construct KieLLMClient and pass it to agents in main.go**

In `cmd/server/main.go`, the current block is:

```go
	llm := agent.NewLLMClient(pool)

	researchAgent := agent.NewResearchAgent(llm, agentsRepo)
	questionAgent := agent.NewQuestionAgent(llm, ragEngine, pool, researchAgent)
	scriptAgent := agent.NewScriptAgent(llm, ragEngine, researchAgent)
	imageAgent := agent.NewImageAgent(llm)
```

Replace the `llm` constructor line so it builds the kie client (keep the variable name `llm` to minimize churn — every agent constructor will accept `*KieLLMClient` after this plan):

```go
	llm := agent.NewKieLLMClient(pool)

	researchAgent := agent.NewResearchAgent(llm, agentsRepo)
	questionAgent := agent.NewQuestionAgent(llm, ragEngine, pool, researchAgent)
	scriptAgent := agent.NewScriptAgent(llm, ragEngine, researchAgent)
	imageAgent := agent.NewImageAgent(llm)
```

> Also find where the analyzer is constructed in `main.go` (search for `analyzer.New(`) — it is passed the same `llm`. Leaving it passes a `*KieLLMClient` where the analyzer still expects `*agent.LLMClient`; that mismatch is fixed in Task 4. Do not change the analyzer call here.

- [ ] **Step 2: Change ResearchAgent to use KieLLMClient + googleSearch**

In `internal/agent/research.go`, change the struct field and constructor type from `*LLMClient` to `*KieLLMClient`, and switch the generation call to `GenerateWithSearch`.

Change the struct:

```go
type ResearchAgent struct {
	llm        *KieLLMClient
	agentsRepo *repository.AgentsRepo
}

func NewResearchAgent(llm *KieLLMClient, agentsRepo *repository.AgentsRepo) *ResearchAgent {
	return &ResearchAgent{llm: llm, agentsRepo: agentsRepo}
}
```

Change the generation call (inside `Research`) from `a.llm.Generate(...)` to `a.llm.GenerateWithSearch(...)`:

```go
	text, err := a.llm.GenerateWithSearch(ctx, cfg.Model, cfg.BuildSystemPrompt(), userPrompt, cfg.Temperature)
	if err != nil {
		return "", fmt.Errorf("research search: %w", err)
	}
```

Update the doc comment on the struct (it references perplexity/sonar) to reflect Gemini googleSearch:

```go
// ResearchAgent finds fresh, reliable information via Gemini googleSearch
// grounding (kie.ai). No crawler or embedding pipeline needed.
//
// Design note: the prompt is deliberately simple. Strict source whitelists or
// explicit "answer empty if unsure" escape hatches make search models bail out
// and return nothing.
```

- [ ] **Step 3: Build to verify research + main wiring compile**

Run: `go build ./internal/agent/ ./cmd/server/`
Expected: FAIL — `internal/analyzer` is not built by this command, but `cmd/server` imports it; if the build pulls analyzer it will fail on the `llm` type mismatch from Step 1. If so, that is expected and resolved in Task 4. To check just this task's files in isolation, run:
`go build ./internal/agent/`
Expected: PASS (research/script/image/question still compile — script/image/question still declare `*LLMClient` fields but their own package builds; the constructor calls live in main.go).

> Note: `go build ./internal/agent/` passing here only proves the agent package compiles. Full wiring is validated in Task 5 after all agents + analyzer are migrated. Do not commit a non-building `cmd/server` — proceed straight to Tasks 3 and 4, then build everything in Task 5 before the final commit. Commit this task's research.go change now since the package compiles:

- [ ] **Step 4: Commit**

```bash
git add cmd/server/main.go internal/agent/research.go
git commit -m "feat(agent): wire KieLLMClient; research uses Gemini googleSearch"
```

---

## Task 3: Migrate script, image, question agents to KieLLMClient

**Files:**
- Modify: `internal/agent/script.go`
- Modify: `internal/agent/image.go`
- Modify: `internal/agent/question.go`

Pure type swaps — these agents already call `Generate`/`GenerateJSON`, which `KieLLMClient` provides with identical signatures.

- [ ] **Step 1: script.go**

Change the struct field and constructor param type from `*LLMClient` to `*KieLLMClient`:

```go
type ScriptAgent struct {
	llm      *KieLLMClient
	rag      *rag.Engine
	research *ResearchAgent
}

func NewScriptAgent(llm *KieLLMClient, ragEngine *rag.Engine, research *ResearchAgent) *ScriptAgent {
	return &ScriptAgent{llm: llm, rag: ragEngine, research: research}
}
```

- [ ] **Step 2: image.go**

```go
type ImageAgent struct {
	llm *KieLLMClient
}

func NewImageAgent(llm *KieLLMClient) *ImageAgent {
	return &ImageAgent{llm: llm}
}
```

- [ ] **Step 3: question.go**

Change only the `llm` field type and the constructor param type (leave the `deduper`/`rag`/`pool` fields and body as-is):

```go
type QuestionAgent struct {
	llm      *KieLLMClient
	rag      *rag.Engine
	pool     *pgxpool.Pool
	deduper  *Deduper
	research *ResearchAgent
}

func NewQuestionAgent(llm *KieLLMClient, ragEngine *rag.Engine, pool *pgxpool.Pool, research *ResearchAgent) *QuestionAgent {
	return &QuestionAgent{llm: llm, rag: ragEngine, pool: pool, deduper: NewDeduper(pool, ragEngine), research: research}
}
```

- [ ] **Step 4: Build the agent package**

Run: `go build ./internal/agent/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/script.go internal/agent/image.go internal/agent/question.go
git commit -m "feat(agent): migrate script/image/question to KieLLMClient"
```

---

## Task 4: Migrate analyzer (analytics agent) to KieLLMClient

**Files:**
- Modify: `internal/analyzer/analyzer.go`

The analyzer holds `*agent.LLMClient` and calls `GenerateJSON`. Swap the type so the `llm` passed from `main.go` (now `*agent.KieLLMClient`) matches.

- [ ] **Step 1: Change the analyzer's llm type**

In `internal/analyzer/analyzer.go`, change the struct field and `New` constructor parameter:

```go
type Analyzer struct {
	pool       *pgxpool.Pool
	llm        *agent.KieLLMClient
	agentsRepo *repository.AgentsRepo
}

func New(pool *pgxpool.Pool, llm *agent.KieLLMClient, agentsRepo *repository.AgentsRepo) *Analyzer {
	return &Analyzer{pool: pool, llm: llm, agentsRepo: agentsRepo}
}
```

The `a.llm.GenerateJSON(...)` call at line ~79 is unchanged — `KieLLMClient` has the same `GenerateJSON` signature.

- [ ] **Step 2: Build everything**

Run: `go build ./...`
Expected: PASS — `cmd/server` now wires `*KieLLMClient` into agents and analyzer consistently.

- [ ] **Step 3: Commit**

```bash
git add internal/analyzer/analyzer.go
git commit -m "feat(analyzer): migrate analytics agent to KieLLMClient"
```

---

## Task 5: Full verification

**Files:** none (verification only)

- [ ] **Step 1: Build, vet, and test the whole module**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: all PASS. Existing tests (including `internal/agent` Plan 1 tests, dedup, research `isSubstantialResearch`, producer, orchestrator) stay green — no test asserted the OpenRouter type, so the swap is invisible to them.

- [ ] **Step 2: Confirm no stray OpenRouter LLM usage remains in agents/analyzer**

Run: `grep -rn "NewLLMClient\|\*LLMClient\|\*agent.LLMClient" internal/ cmd/`
Expected: zero matches in `research.go`, `script.go`, `image.go`, `question.go`, `analyzer.go`, `main.go`. The only remaining references to `LLMClient` should be its definition in `internal/agent/llm.go` (kept for `extractJSON`). If `main.go` still shows `NewLLMClient`, Task 2 Step 1 was missed.

- [ ] **Step 3: Confirm the live wire formats are exercised by the real models**

This reuses Plan 1's live-verified facts (Claude system+stream:false, Gemini SSE, googleSearch). No new live call is required for Plan 2a — the agents call the same `KieLLMClient` methods already proven against kie.ai on 2026-06-10. (A real end-to-end agent call happens naturally in Plan 2b/3 testing.)

---

## Self-Review Notes

- **Spec coverage:** Implements the "all LLM agents on kie.ai" requirement (spec §2 model table, §4.2) for the *existing* agents, and research→Gemini googleSearch (§4.6 research row). New agents (scene, metadata), the `image`→`imageprompt` rename, `question` removal, topic-driven orchestrator, scene/clip columns, and frontend alignment are deliberately deferred to **Plan 2b** to keep this increment compiling and behavior-stable.
- **No placeholders:** every edit shows the exact resulting Go/SQL.
- **Type consistency:** every agent + analyzer field becomes `*KieLLMClient` / `*agent.KieLLMClient`; `main.go` builds one `agent.NewKieLLMClient(pool)` and passes it to all. `Generate`/`GenerateJSON`/`GenerateWithSearch` signatures match Plan 1 exactly.
- **Risk:** `question`/`image`/`script` model values now point at kie.ai but their prompt templates are unchanged — they will run on the new provider with old prompts. That is acceptable for 2a (still produces valid output); prompt/registry redesign is Plan 2b. The Q&A flow and static-image producer continue to work end-to-end.
- **Honesty flag:** research's `cfg.Temperature` is sent to Gemini; verified accepted (Plan 1 live call). If a specific agent returns empty under Gemini where sonar didn't, that is a prompt-tuning issue for Plan 2b, not a client defect.
