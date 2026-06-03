# Fix OOM (Out of Memory) Crash — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Eliminate the OOM crash on Railway by reducing memory pressure from the knowledge sources endpoint — from 42KB/request polled every 11s to ~2KB/request with 30s cache.

**Architecture:** Split the knowledge list endpoint to return summaries (no content), add a detail endpoint for full content on-demand. Fix orchestrator goroutine race condition. Add frontend caching and lazy content loading.

**Tech Stack:** Go 1.25, chi router, pgx/v5, React 19, TanStack Query

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/models/models.go` | Modify | Add `KnowledgeSourceSummary` struct |
| `internal/repository/knowledge.go` | Modify | Add `ListSourceSummaries()`, `GetSourceByID()`, optimize query |
| `internal/handler/knowledge.go` | Modify | Use new repo methods, add `GetSource` handler, fix `EmbedSource` |
| `internal/router/router.go` | Modify | Add `GET /{id}` route for single source |
| `internal/handler/orchestrator.go` | Modify | Fix goroutine race condition |
| `frontend/src/App.tsx` | Modify | Add `staleTime` to QueryClient |
| `frontend/src/pages/Knowledge.tsx` | Modify | Use summary for list, lazy-load content on expand |

---

### Task 1: Add KnowledgeSourceSummary Model

**Files:**
- Modify: `internal/models/models.go:64-71`

- [ ] **Step 1: Add the summary struct**

Add after the existing `KnowledgeSource` struct (line 71):

```go
type KnowledgeSourceSummary struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Category       string `json:"category"`
	ContentPreview string `json:"content_preview"`
	Enabled        bool   `json:"enabled"`
	ChunkCount     int    `json:"chunk_count"`
}
```

- [ ] **Step 2: Verify build**

Run: `cd /Users/jaochai/Code/video-fb && go build ./...`
Expected: BUILD SUCCESS (no errors)

- [ ] **Step 3: Commit**

```bash
git add internal/models/models.go
git commit -m "feat: add KnowledgeSourceSummary model for lightweight list responses"
```

---

### Task 2: Repository — Add ListSourceSummaries + GetSourceByID

**Files:**
- Modify: `internal/repository/knowledge.go`

- [ ] **Step 1: Add `ListSourceSummaries` method**

Add after the existing `ListSources` method (line 38). This replaces the N+1 subquery with a LEFT JOIN and excludes the full `content` field:

```go
func (r *KnowledgeRepo) ListSourceSummaries(ctx context.Context) ([]models.KnowledgeSourceSummary, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT ks.id, ks.name, ks.category, LEFT(ks.content, 150), ks.enabled,
		        COALESCE(COUNT(kc.id), 0) AS chunk_count
		 FROM knowledge_sources ks
		 LEFT JOIN knowledge_chunks kc ON kc.source_id = ks.id
		 GROUP BY ks.id, ks.name, ks.category, ks.content, ks.enabled
		 ORDER BY ks.category, ks.name`)
	if err != nil {
		return nil, fmt.Errorf("query source summaries: %w", err)
	}
	defer rows.Close()

	var summaries []models.KnowledgeSourceSummary
	for rows.Next() {
		var s models.KnowledgeSourceSummary
		if err := rows.Scan(&s.ID, &s.Name, &s.Category, &s.ContentPreview, &s.Enabled, &s.ChunkCount); err != nil {
			return nil, fmt.Errorf("scan source summary: %w", err)
		}
		summaries = append(summaries, s)
	}
	return summaries, nil
}
```

- [ ] **Step 2: Add `GetSourceByID` method**

Add after `ListSourceSummaries`:

```go
func (r *KnowledgeRepo) GetSourceByID(ctx context.Context, id string) (*models.KnowledgeSource, error) {
	var s models.KnowledgeSource
	err := r.pool.QueryRow(ctx,
		`SELECT ks.id, ks.name, ks.category, ks.content, ks.enabled,
		        COALESCE((SELECT COUNT(*) FROM knowledge_chunks kc WHERE kc.source_id = ks.id), 0)
		 FROM knowledge_sources ks
		 WHERE ks.id = $1`, id,
	).Scan(&s.ID, &s.Name, &s.Category, &s.Content, &s.Enabled, &s.ChunkCount)
	if err != nil {
		return nil, fmt.Errorf("get source %s: %w", id, err)
	}
	return &s, nil
}
```

- [ ] **Step 3: Verify build**

Run: `cd /Users/jaochai/Code/video-fb && go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 4: Commit**

```bash
git add internal/repository/knowledge.go
git commit -m "feat: add ListSourceSummaries and GetSourceByID repo methods"
```

---

### Task 3: Handler — Update ListSources, Add GetSource, Fix EmbedSource

**Files:**
- Modify: `internal/handler/knowledge.go`

- [ ] **Step 1: Update `ListSources` to use summaries**

Replace the `ListSources` method (lines 26-33) with:

```go
func (h *KnowledgeHandler) ListSources(w http.ResponseWriter, r *http.Request) {
	summaries, err := h.repo.ListSourceSummaries(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: summaries})
}
```

- [ ] **Step 2: Add `GetSource` handler for single source with full content**

Add after `ListSources`:

```go
func (h *KnowledgeHandler) GetSource(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	source, err := h.repo.GetSourceByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.APIResponse{Error: "source not found"})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: source})
}
```

- [ ] **Step 3: Fix `EmbedSource` to use `GetSourceByID` instead of loading all sources**

Replace the `EmbedSource` method (lines 97-120) with:

```go
func (h *KnowledgeHandler) EmbedSource(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	source, err := h.repo.GetSourceByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.APIResponse{Error: "source not found"})
		return
	}

	n, err := h.rebuildChunks(id, source.Content)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: map[string]any{"chunks": n}})
}
```

- [ ] **Step 4: Remove old `ListSources` repo method (now unused)**

In `internal/repository/knowledge.go`, delete the original `ListSources` method (lines 19-38) since it's replaced by `ListSourceSummaries` and `GetSourceByID`.

- [ ] **Step 5: Verify build**

Run: `cd /Users/jaochai/Code/video-fb && go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 6: Commit**

```bash
git add internal/handler/knowledge.go internal/repository/knowledge.go
git commit -m "fix: reduce ListSources response from 42KB to ~2KB by excluding content field"
```

---

### Task 4: Router — Add GET /{id} Route

**Files:**
- Modify: `internal/router/router.go:45-52`

- [ ] **Step 1: Add the GET /{id} route**

In the knowledge sources route group (line 45-52), add `r.Get("/{id}", knowledge.GetSource)` between the existing POST and PUT routes:

```go
r.Route("/api/v1/knowledge/sources", func(r chi.Router) {
	r.Get("/", knowledge.ListSources)
	r.Post("/", knowledge.CreateSource)
	r.Get("/{id}", knowledge.GetSource)
	r.Put("/{id}", knowledge.UpdateSource)
	r.Patch("/{id}", knowledge.ToggleSource)
	r.Delete("/{id}", knowledge.DeleteSource)
	r.Post("/{id}/embed", knowledge.EmbedSource)
})
```

- [ ] **Step 2: Verify build**

Run: `cd /Users/jaochai/Code/video-fb && go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 3: Commit**

```bash
git add internal/router/router.go
git commit -m "feat: add GET /knowledge/sources/{id} endpoint for single source detail"
```

---

### Task 5: Fix Orchestrator Race Condition

**Files:**
- Modify: `internal/handler/orchestrator.go`

- [ ] **Step 1: Fix the goroutine race condition**

Replace the entire `TriggerWeekly` method (lines 19-38) with:

```go
func (h *OrchestratorHandler) TriggerWeekly(w http.ResponseWriter, r *http.Request) {
	countStr := r.URL.Query().Get("count")
	count := 7
	if countStr != "" {
		if n, err := strconv.Atoi(countStr); err == nil && n > 0 {
			count = n
		}
	}

	writeJSON(w, http.StatusAccepted, models.APIResponse{
		Message: "Weekly production started in background",
	})

	go func() {
		ctx := context.Background()
		if err := h.orch.ProduceWeekly(ctx, count); err != nil {
			log.Printf("Weekly production failed: %v", err)
		}
	}()
}
```

Key changes:
- Respond BEFORE starting the goroutine (no race on `w`)
- Use `context.Background()` instead of `r.Context()` (request context cancels when handler returns)
- Log errors instead of writing to closed ResponseWriter

- [ ] **Step 2: Add missing imports**

Ensure these imports are present at the top of the file:

```go
import (
	"context"
	"log"
	"net/http"
	"strconv"

	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/orchestrator"
)
```

- [ ] **Step 3: Verify build**

Run: `cd /Users/jaochai/Code/video-fb && go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 4: Commit**

```bash
git add internal/handler/orchestrator.go
git commit -m "fix: orchestrator goroutine race condition — respond before background work"
```

---

### Task 6: Frontend — Add QueryClient Caching

**Files:**
- Modify: `frontend/src/App.tsx:10`

- [ ] **Step 1: Add staleTime and refetchOnWindowFocus config**

Replace line 10:
```tsx
const queryClient = new QueryClient();
```

With:
```tsx
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      refetchOnWindowFocus: false,
    },
  },
});
```

This means data is considered fresh for 30 seconds — no re-fetches within that window.

- [ ] **Step 2: Verify build**

Run: `cd /Users/jaochai/Code/video-fb/frontend && npm run build`
Expected: BUILD SUCCESS

- [ ] **Step 3: Commit**

```bash
git add frontend/src/App.tsx
git commit -m "fix: add 30s staleTime to QueryClient to reduce polling pressure"
```

---

### Task 7: Frontend — Lazy-Load Content on Expand

**Files:**
- Modify: `frontend/src/pages/Knowledge.tsx`

This is the most impactful frontend change. The list now returns `content_preview` instead of `content`. Full content is fetched only when the user expands a source.

- [ ] **Step 1: Update Source interface and add SourceSummary**

Replace the `Source` interface (lines 5-12) with:

```tsx
interface SourceSummary {
  id: string;
  name: string;
  category: string;
  content_preview: string;
  enabled: boolean;
  chunk_count: number;
}

interface Source extends SourceSummary {
  content: string;
}
```

- [ ] **Step 2: Update the list query to use SourceSummary**

Replace line 40-43:
```tsx
const { data: sources, isLoading } = useQuery({
    queryKey: ['knowledge'],
    queryFn: () => apiFetch<Source[]>('/api/v1/knowledge/sources'),
});
```

With:
```tsx
const { data: sources, isLoading } = useQuery({
    queryKey: ['knowledge'],
    queryFn: () => apiFetch<SourceSummary[]>('/api/v1/knowledge/sources'),
});
```

- [ ] **Step 3: Add state for loaded full sources and a fetch function**

After the `showNew`/`newDoc` state declarations (line 49), add:

```tsx
const [fullSources, setFullSources] = useState<Record<string, Source>>({});

const loadFullSource = async (id: string) => {
  if (fullSources[id]) return;
  try {
    const source = await apiFetch<Source>(`/api/v1/knowledge/sources/${id}`);
    setFullSources(prev => ({ ...prev, [id]: source }));
    setEdits(prev => ({ ...prev, [id]: { name: source.name, category: source.category, content: source.content } }));
  } catch (e) {
    console.error('Failed to load source', e);
  }
};
```

- [ ] **Step 4: Update useEffect to NOT populate content from list**

Replace the useEffect (lines 51-59):

```tsx
useEffect(() => {
  if (sources) {
    const initial: Record<string, Partial<Source>> = {};
    sources.forEach(s => {
      initial[s.id] = { name: s.name, category: s.category, content: s.content };
    });
    setEdits(initial);
  }
}, [sources]);
```

With:

```tsx
useEffect(() => {
  if (sources) {
    const initial: Record<string, Partial<Source>> = {};
    sources.forEach(s => {
      if (!edits[s.id]) {
        initial[s.id] = { name: s.name, category: s.category };
      }
    });
    setEdits(prev => ({ ...initial, ...prev }));
  }
}, [sources]);
```

- [ ] **Step 5: Update toggleExpand to trigger content load**

Replace the `toggleExpand` function (lines 129-131):

```tsx
const toggleExpand = (id: string) => {
  setExpanded(prev => ({ ...prev, [id]: !prev[id] }));
};
```

With:

```tsx
const toggleExpand = (id: string) => {
  const willExpand = !expanded[id];
  setExpanded(prev => ({ ...prev, [id]: willExpand }));
  if (willExpand) {
    loadFullSource(id);
  }
};
```

- [ ] **Step 6: Update preview to use content_preview**

Replace line 278-279 (the collapsed preview):

```tsx
{source.content.slice(0, 120)}...
```

With:

```tsx
{source.content_preview}...
```

- [ ] **Step 7: Update expanded view to use fullSources for content**

In the expanded textarea (line 303-310), replace:

```tsx
<textarea
  rows={12}
  value={e.content ?? source.content}
  onChange={ev => handleEdit(source.id, 'content', ev.target.value)}
  style={textareaStyle}
  onFocus={ev => (ev.target.style.borderColor = '#444')}
  onBlur={ev => (ev.target.style.borderColor = '#222')}
/>
```

With:

```tsx
{fullSources[source.id] ? (
  <textarea
    rows={12}
    value={e.content ?? fullSources[source.id].content}
    onChange={ev => handleEdit(source.id, 'content', ev.target.value)}
    style={textareaStyle}
    onFocus={ev => (ev.target.style.borderColor = '#444')}
    onBlur={ev => (ev.target.style.borderColor = '#222')}
  />
) : (
  <p style={{ color: '#555', fontSize: 13, padding: '10px 0' }}>Loading content...</p>
)}
```

- [ ] **Step 8: Update grouped type**

Replace line 133-138:
```tsx
const grouped = sources?.reduce((acc, s) => {
  const cat = s.category || 'general';
  if (!acc[cat]) acc[cat] = [];
  acc[cat].push(s);
  return acc;
}, {} as Record<string, Source[]>);
```

With:
```tsx
const grouped = sources?.reduce((acc, s) => {
  const cat = s.category || 'general';
  if (!acc[cat]) acc[cat] = [];
  acc[cat].push(s);
  return acc;
}, {} as Record<string, SourceSummary[]>);
```

- [ ] **Step 9: Clear fullSources cache on mutation success**

In the `updateSource` mutation `onSuccess` (line 67-70), add cache clearing:

```tsx
onSuccess: (_d, { id }) => {
  qc.invalidateQueries({ queryKey: ['knowledge'] });
  setDirty(prev => ({ ...prev, [id]: false }));
  setFullSources(prev => { const next = { ...prev }; delete next[id]; return next; });
},
```

- [ ] **Step 10: Verify build**

Run: `cd /Users/jaochai/Code/video-fb/frontend && npm run build`
Expected: BUILD SUCCESS (tsc + vite build)

- [ ] **Step 11: Commit**

```bash
git add frontend/src/pages/Knowledge.tsx
git commit -m "fix: lazy-load knowledge content on expand to reduce memory pressure"
```

---

### Task 8: Final Build Verification & Deploy

- [ ] **Step 1: Full backend build**

Run: `cd /Users/jaochai/Code/video-fb && go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 2: Full frontend build**

Run: `cd /Users/jaochai/Code/video-fb/frontend && npm run build`
Expected: BUILD SUCCESS

- [ ] **Step 3: Push to trigger Railway deploy**

```bash
git push origin master
```

- [ ] **Step 4: Monitor Railway deployment**

Check deployment status — should no longer OOM because:
- List response: ~2KB instead of 42KB (content excluded)
- Polling reduced: 30s staleTime instead of instant re-fetch
- Combined: ~99% reduction in memory pressure from this endpoint

---

## Impact Summary

| Metric | Before | After |
|--------|--------|-------|
| List response size | 42KB | ~2KB |
| Polling interval | ~0s (instant stale) | 30s staleTime |
| Memory per list request | Full content loaded | Summary only |
| EmbedSource memory | All sources loaded | Single source |
| Orchestrator | Race condition + leaked context | Clean background job |
| DB query pattern | N+1 correlated subquery | Single LEFT JOIN |
