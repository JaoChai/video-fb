# Frontend UX/UI Improvements — Phase 2-7

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Improve the Ads Vance Dashboard UX across 6 areas: skeleton loaders, empty states, error boundaries, responsive sidebar, KPI cards, and prompt history viewer.

**Architecture:** Each phase is independent and can be implemented in any order. Phases 2-6 are frontend-only. Phase 7 requires a backend endpoint addition (Go handler + repository method) plus a new frontend page. All changes use existing component patterns (shadcn-style).

**Tech Stack:** React 19, Vite 8, TanStack Query 5, Tailwind CSS 4, lucide-react, Go 1.25 (backend Phase 7 only)

**No test framework** — verification is `cd frontend && npm run build` (tsc + vite build).

---

## File Structure

### New Files
| File | Purpose |
|------|---------|
| `frontend/src/components/ui/skeleton.tsx` | Skeleton placeholder component |
| `frontend/src/components/empty-state.tsx` | Reusable empty state with icon + CTA |
| `frontend/src/components/error-boundary.tsx` | React error boundary wrapper |
| `frontend/src/components/mobile-sidebar.tsx` | Mobile slide-out sidebar overlay |
| `frontend/src/components/kpi-card.tsx` | KPI stat card for Content dashboard |
| `frontend/src/pages/PromptHistory.tsx` | Prompt history viewer page |
| `internal/handler/prompt_history.go` | Backend: HTTP handler for prompt history |

### Modified Files
| File | Changes |
|------|---------|
| `frontend/src/pages/Content.tsx` | Replace loading text with skeleton, add empty state, add KPI cards |
| `frontend/src/pages/Schedules.tsx` | Replace loading text with skeleton |
| `frontend/src/pages/Analytics.tsx` | Replace loading text with skeleton, add empty state |
| `frontend/src/pages/Knowledge.tsx` | Replace loading text with skeleton, add empty state |
| `frontend/src/pages/Agents.tsx` | Replace loading text with skeleton |
| `frontend/src/pages/Settings.tsx` | Replace loading text with skeleton |
| `frontend/src/components/sidebar.tsx` | Add mobile toggle, responsive hiding |
| `frontend/src/App.tsx` | Wrap in ErrorBoundary, add Route for prompt history, responsive layout |
| `frontend/src/style.css` | Add skeleton animation keyframe |
| `internal/repository/agents.go` | Add `ListPromptHistory()` method |
| `internal/router/router.go` | Add `/api/v1/agents/prompt-history` route |

---

## Task 1: Skeleton Loader Component + Animation

**Files:**
- Create: `frontend/src/components/ui/skeleton.tsx`
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Create skeleton component**

```tsx
// frontend/src/components/ui/skeleton.tsx
import { cn } from "../../lib/utils"

export function Skeleton({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("animate-pulse rounded-md bg-muted", className)} {...props} />
}
```

- [ ] **Step 2: Add pulse animation to style.css**

Add at end of `frontend/src/style.css`:

```css
@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}
```

Note: Tailwind 4 already includes `animate-pulse` — only add keyframe if build shows it missing.

- [ ] **Step 3: Verify build**

Run: `cd frontend && npm run build`
Expected: Build succeeds with no errors

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/ui/skeleton.tsx frontend/src/style.css
git commit -m "feat: add Skeleton loader component"
```

---

## Task 2: Replace Loading States with Skeletons — All Pages

**Files:**
- Modify: `Content.tsx`, `Schedules.tsx`, `Analytics.tsx`, `Knowledge.tsx`, `Agents.tsx`, `Settings.tsx`

- [ ] **Step 1: Replace Content.tsx loading (line ~154)**

Replace:
```tsx
<p className="text-sm text-muted-foreground">Loading...</p>
```

With:
```tsx
<div className="space-y-3">
  {[1, 2, 3].map(i => (
    <div key={i} className="flex items-center gap-4 py-4">
      <Skeleton className="h-5 flex-1" />
      <Skeleton className="h-5 w-24" />
      <Skeleton className="h-6 w-20 rounded-full" />
      <Skeleton className="h-5 w-20" />
    </div>
  ))}
</div>
```

Add import: `import { Skeleton } from '../components/ui/skeleton';`

- [ ] **Step 2: Replace Schedules.tsx loading (line ~60)**

Replace:
```tsx
<p className="text-sm text-muted-foreground">Loading...</p>
```

With:
```tsx
<div className="grid gap-4">
  {[1, 2, 3].map(i => (
    <div key={i} className="rounded-xl border p-4 space-y-2">
      <div className="flex justify-between">
        <Skeleton className="h-5 w-40" />
        <Skeleton className="h-6 w-12 rounded-full" />
      </div>
      <Skeleton className="h-4 w-64" />
    </div>
  ))}
</div>
```

Add import: `import { Skeleton } from '../components/ui/skeleton';`

- [ ] **Step 3: Replace Analytics.tsx loading states (lines ~82 and ~112)**

Replace both instances of:
```tsx
<p className="text-muted-foreground">Loading...</p>
```
and
```tsx
<p className="text-muted-foreground">Loading analytics...</p>
```

First (clips loading):
```tsx
<div className="grid grid-cols-4 gap-3 mb-10">
  {[1, 2, 3, 4].map(i => (
    <div key={i} className="rounded-xl border p-5 space-y-2">
      <Skeleton className="h-3 w-16" />
      <Skeleton className="h-8 w-12" />
    </div>
  ))}
</div>
```

Second (analytics loading):
```tsx
<div className="grid grid-cols-2 gap-4">
  {[1, 2].map(i => (
    <div key={i} className="rounded-xl border p-5 space-y-3">
      <Skeleton className="h-5 w-24" />
      <div className="grid grid-cols-2 gap-3">
        {[1, 2, 3, 4].map(j => (
          <div key={j} className="space-y-1">
            <Skeleton className="h-3 w-12" />
            <Skeleton className="h-6 w-16" />
          </div>
        ))}
      </div>
    </div>
  ))}
</div>
```

Add import: `import { Skeleton } from '../components/ui/skeleton';`

- [ ] **Step 4: Replace Knowledge.tsx loading (line ~237)**

Replace:
```tsx
<p className="text-muted-foreground">Loading...</p>
```

With:
```tsx
<div className="space-y-6">
  {[1, 2].map(g => (
    <div key={g}>
      <Skeleton className="h-3 w-32 mb-3" />
      <div className="grid gap-2">
        {[1, 2, 3].map(i => (
          <div key={i} className="rounded-xl border p-4 space-y-2">
            <div className="flex justify-between">
              <Skeleton className="h-5 w-48" />
              <Skeleton className="h-4 w-16 rounded-full" />
            </div>
            <Skeleton className="h-3 w-full" />
          </div>
        ))}
      </div>
    </div>
  ))}
</div>
```

Add import: `import { Skeleton } from '../components/ui/skeleton';`

- [ ] **Step 5: Replace Agents.tsx loading (line ~100)**

Replace:
```tsx
<p className="text-sm text-muted-foreground">Loading...</p>
```

With:
```tsx
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
```

Add import: `import { Skeleton } from '../components/ui/skeleton';`

- [ ] **Step 6: Replace Settings.tsx loading (line ~269)**

Replace:
```tsx
<p className="text-sm text-muted-foreground">Loading channels...</p>
```

With:
```tsx
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
```

No new import needed (Skeleton already imported if doing tasks in order).

- [ ] **Step 7: Verify build**

Run: `cd frontend && npm run build`
Expected: Build succeeds

- [ ] **Step 8: Commit**

```bash
git add frontend/src/pages/
git commit -m "feat: replace all loading text with skeleton loaders"
```

---

## Task 3: Empty State Component

**Files:**
- Create: `frontend/src/components/empty-state.tsx`

- [ ] **Step 1: Create reusable empty state component**

```tsx
// frontend/src/components/empty-state.tsx
import type { ReactNode } from "react"
import type { LucideIcon } from "lucide-react"
import { Button } from "./ui/button"

interface EmptyStateProps {
  icon: LucideIcon
  title: string
  description: string
  action?: { label: string; onClick: () => void }
  children?: ReactNode
}

export function EmptyState({ icon: Icon, title, description, action, children }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center py-16 text-center">
      <div className="rounded-full bg-muted p-4 mb-4">
        <Icon className="h-8 w-8 text-muted-foreground" />
      </div>
      <h3 className="text-lg font-medium mb-1">{title}</h3>
      <p className="text-sm text-muted-foreground max-w-sm mb-4">{description}</p>
      {action && (
        <Button onClick={action.onClick}>{action.label}</Button>
      )}
      {children}
    </div>
  )
}
```

- [ ] **Step 2: Verify build**

Run: `cd frontend && npm run build`
Expected: Build succeeds

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/empty-state.tsx
git commit -m "feat: add EmptyState component"
```

---

## Task 4: Add Empty States to Pages

**Files:**
- Modify: `Content.tsx`, `Knowledge.tsx`, `Analytics.tsx`

- [ ] **Step 1: Improve Content.tsx empty state (line ~156)**

Replace:
```tsx
<p className="text-sm text-muted-foreground">
  No clips yet. Scheduler will auto-produce at noon &amp; midnight.
</p>
```

With:
```tsx
<EmptyState
  icon={Film}
  title="No clips yet"
  description="Scheduler will auto-produce clips at noon & midnight, or you can manually produce one now."
  action={{ label: '+ Produce 1 Clip', onClick: handleProduce }}
/>
```

Add imports:
```tsx
import { Film } from 'lucide-react';
import { EmptyState } from '../components/empty-state';
```

Note: `Film` icon is new; `Plus, RotateCcw, Send, Trash2, Loader2` are existing imports.

- [ ] **Step 2: Add empty state to Knowledge.tsx**

After the `{isLoading ? (` skeleton block, check if `!sources?.length` (currently no empty state exists for Knowledge).

Find the end of the loading ternary and before the grouped rendering, add:
```tsx
) : !sources?.length ? (
  <EmptyState
    icon={BookOpen}
    title="No documents yet"
    description="Add knowledge documents to help agents generate more accurate content."
    action={{ label: '+ Add Document', onClick: () => setShowNew(true) }}
  />
```

Add import: `import { EmptyState } from '../components/empty-state';`
Note: `BookOpen` is not currently imported in Knowledge.tsx — add it to lucide imports.

- [ ] **Step 3: Improve Analytics.tsx empty state for clip selection**

Currently when no clip is selected, Analytics shows a text prompt. Improve the state when there are no published clips at all.

Find the section after the stats grid where it checks `publishedClips.length === 0` and replace with:
```tsx
{publishedClips.length === 0 ? (
  <EmptyState
    icon={BarChart3}
    title="No analytics data"
    description="Publish clips to YouTube first, then analytics data will appear here."
  />
) : (
```

Add import: `import { EmptyState } from '../components/empty-state';`
Note: `BarChart3` is already imported in Analytics.tsx.

- [ ] **Step 4: Verify build**

Run: `cd frontend && npm run build`
Expected: Build succeeds

- [ ] **Step 5: Commit**

```bash
git add frontend/src/pages/Content.tsx frontend/src/pages/Knowledge.tsx frontend/src/pages/Analytics.tsx
git commit -m "feat: add empty states with icons and CTAs"
```

---

## Task 5: Error Boundary

**Files:**
- Create: `frontend/src/components/error-boundary.tsx`
- Modify: `frontend/src/App.tsx`

- [ ] **Step 1: Create ErrorBoundary component**

```tsx
// frontend/src/components/error-boundary.tsx
import { Component, type ReactNode } from "react"
import { AlertTriangle, RotateCcw } from "lucide-react"
import { Button } from "./ui/button"

interface Props {
  children: ReactNode
}

interface State {
  hasError: boolean
  error: Error | null
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false, error: null }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  handleReset = () => {
    this.setState({ hasError: false, error: null })
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="flex flex-col items-center justify-center min-h-[400px] text-center px-4">
          <div className="rounded-full bg-destructive/10 p-4 mb-4">
            <AlertTriangle className="h-8 w-8 text-destructive" />
          </div>
          <h2 className="text-lg font-semibold mb-1">Something went wrong</h2>
          <p className="text-sm text-muted-foreground max-w-md mb-4">
            {this.state.error?.message || "An unexpected error occurred."}
          </p>
          <Button onClick={this.handleReset} variant="outline">
            <RotateCcw className="h-4 w-4 mr-2" />
            Try Again
          </Button>
        </div>
      )
    }
    return this.props.children
  }
}
```

- [ ] **Step 2: Wrap main content in ErrorBoundary in App.tsx**

In `App.tsx`, add import:
```tsx
import { ErrorBoundary } from "./components/error-boundary"
```

Wrap `<main>` content:
```tsx
<main className="flex-1 overflow-y-auto px-8 py-8 max-w-5xl">
  <ErrorBoundary>
    <Routes>
      {/* ... routes unchanged ... */}
    </Routes>
  </ErrorBoundary>
</main>
```

- [ ] **Step 3: Verify build**

Run: `cd frontend && npm run build`
Expected: Build succeeds

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/error-boundary.tsx frontend/src/App.tsx
git commit -m "feat: add ErrorBoundary to catch page-level crashes"
```

---

## Task 6: Responsive Sidebar

**Files:**
- Create: `frontend/src/components/mobile-sidebar.tsx`
- Modify: `frontend/src/components/sidebar.tsx`
- Modify: `frontend/src/App.tsx`

- [ ] **Step 1: Add mobile menu button to sidebar.tsx**

Export the nav data so mobile sidebar can reuse it. At the top of `sidebar.tsx`, add `export` to the nav arrays:

```tsx
export const PIPELINE_NAV = [
  { to: "/", label: "Content", icon: LayoutDashboard },
  { to: "/schedules", label: "Schedules", icon: CalendarClock },
  { to: "/analytics", label: "Analytics", icon: BarChart3 },
]

export const CONFIG_NAV = [
  { to: "/knowledge", label: "Knowledge", icon: BookOpen },
  { to: "/agents", label: "Agents", icon: Bot },
  { to: "/settings", label: "Settings", icon: Settings },
]
```

Add `hidden md:flex` to the `<aside>` to hide on mobile:
```tsx
<aside className="hidden md:flex h-screen w-[240px] flex-col bg-sidebar sticky top-0">
```

- [ ] **Step 2: Create mobile sidebar overlay**

```tsx
// frontend/src/components/mobile-sidebar.tsx
import { useState } from "react"
import { NavLink, useLocation } from "react-router-dom"
import { Menu, X } from "lucide-react"
import { cn } from "../lib/utils"
import { PIPELINE_NAV, CONFIG_NAV } from "./sidebar"
import { Button } from "./ui/button"

function NavSection({ label, items, onClose }: {
  label: string
  items: typeof PIPELINE_NAV
  onClose: () => void
}) {
  return (
    <div>
      <p className="px-3 mb-2 text-[11px] font-semibold uppercase tracking-wider text-sidebar-foreground/40">
        {label}
      </p>
      <div className="space-y-0.5">
        {items.map(({ to, label, icon: Icon }) => (
          <NavLink
            key={to}
            to={to}
            end={to === "/"}
            onClick={onClose}
            className={({ isActive }) =>
              cn(
                "flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors",
                isActive
                  ? "bg-sidebar-accent text-accent-foreground"
                  : "text-sidebar-foreground/70 hover:bg-sidebar-muted hover:text-sidebar-foreground"
              )
            }
          >
            <Icon className="h-4 w-4" />
            {label}
          </NavLink>
        ))}
      </div>
    </div>
  )
}

export function MobileSidebar() {
  const [open, setOpen] = useState(false)
  const location = useLocation()

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
              <NavSection label="Pipeline" items={PIPELINE_NAV} onClose={() => setOpen(false)} />
              <NavSection label="Configuration" items={CONFIG_NAV} onClose={() => setOpen(false)} />
            </nav>
          </aside>
        </>
      )}
    </>
  )
}
```

- [ ] **Step 3: Update App.tsx layout for responsive**

Add import:
```tsx
import { MobileSidebar } from "./components/mobile-sidebar"
```

Update layout:
```tsx
<div className="flex min-h-screen bg-background">
  <Sidebar />
  <div className="flex-1 flex flex-col">
    <MobileSidebar />
    <main className="flex-1 overflow-y-auto px-4 py-6 md:px-8 md:py-8 max-w-5xl">
      <ErrorBoundary>
        <Routes>{/* ... */}</Routes>
      </ErrorBoundary>
    </main>
  </div>
</div>
```

- [ ] **Step 4: Verify build**

Run: `cd frontend && npm run build`
Expected: Build succeeds

- [ ] **Step 5: Test visually**

Open dev server, resize browser to <768px width. Verify:
- Desktop sidebar visible, mobile header hidden
- Mobile shows hamburger header, sidebar hidden
- Clicking menu opens overlay sidebar
- Clicking a link navigates + closes sidebar
- Clicking backdrop closes sidebar

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/sidebar.tsx frontend/src/components/mobile-sidebar.tsx frontend/src/App.tsx
git commit -m "feat: responsive sidebar with mobile hamburger menu"
```

---

## Task 7: Dashboard KPI Cards on Content Page

**Files:**
- Create: `frontend/src/components/kpi-card.tsx`
- Modify: `frontend/src/pages/Content.tsx`

- [ ] **Step 1: Create KPI card component**

```tsx
// frontend/src/components/kpi-card.tsx
import type { LucideIcon } from "lucide-react"
import { cn } from "../lib/utils"

interface KpiCardProps {
  label: string
  value: number
  icon: LucideIcon
  className?: string
}

export function KpiCard({ label, value, icon: Icon, className }: KpiCardProps) {
  return (
    <div className={cn("rounded-xl border bg-card p-4", className)}>
      <div className="flex items-center justify-between mb-2">
        <span className="text-xs text-muted-foreground uppercase tracking-wide">{label}</span>
        <Icon className="h-4 w-4 text-muted-foreground" />
      </div>
      <div className="text-2xl font-bold tabular-nums">{value}</div>
    </div>
  )
}
```

- [ ] **Step 2: Add KPI section to Content.tsx**

Add imports:
```tsx
import { KpiCard } from '../components/kpi-card';
import { LayoutDashboard, CheckCircle2, Zap, AlertTriangle } from 'lucide-react';
```

After `<ProductionProgress />` and before the table, add:
```tsx
{clips && clips.length > 0 && (
  <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-6">
    <KpiCard label="Total" value={clips.length} icon={LayoutDashboard} />
    <KpiCard label="Published" value={clips.filter(c => c.status === 'published').length} icon={CheckCircle2} />
    <KpiCard label="Ready" value={readyCount} icon={Zap} />
    <KpiCard label="Failed" value={failedCount} icon={AlertTriangle} />
  </div>
)}
```

- [ ] **Step 3: Verify build**

Run: `cd frontend && npm run build`
Expected: Build succeeds

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/kpi-card.tsx frontend/src/pages/Content.tsx
git commit -m "feat: add KPI summary cards to Content dashboard"
```

---

## Task 8: Prompt History — Backend Endpoint

**Files:**
- Modify: `internal/repository/agents.go`
- Create: `internal/handler/prompt_history.go`
- Modify: `internal/router/router.go`

- [ ] **Step 1: Add ListPromptHistory to repository**

Add to `internal/repository/agents.go`:

```go
type PromptHistoryEntry struct {
	ID        string    `json:"id"`
	AgentName string    `json:"agent_name"`
	OldPrompt string    `json:"old_prompt"`
	NewPrompt string    `json:"new_prompt"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

func (r *AgentsRepo) ListPromptHistory(ctx context.Context, limit int) ([]PromptHistoryEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, agent_name, old_prompt, new_prompt, reason, created_at
		 FROM agent_prompt_history
		 ORDER BY created_at DESC
		 LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("query prompt history: %w", err)
	}
	defer rows.Close()

	var entries []PromptHistoryEntry
	for rows.Next() {
		var e PromptHistoryEntry
		if err := rows.Scan(&e.ID, &e.AgentName, &e.OldPrompt, &e.NewPrompt, &e.Reason, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan prompt history: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, nil
}
```

Add `"time"` to imports if not already present.

- [ ] **Step 2: Create prompt history handler**

```go
// internal/handler/prompt_history.go
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/jaochai/video-fb/internal/repository"
)

type PromptHistoryHandler struct {
	repo *repository.AgentsRepo
}

func NewPromptHistoryHandler(repo *repository.AgentsRepo) *PromptHistoryHandler {
	return &PromptHistoryHandler{repo: repo}
}

func (h *PromptHistoryHandler) List(w http.ResponseWriter, r *http.Request) {
	entries, err := h.repo.ListPromptHistory(r.Context(), 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if entries == nil {
		entries = []repository.PromptHistoryEntry{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"data": entries})
}
```

- [ ] **Step 3: Register route in router.go**

In `internal/router/router.go`, find the agents route section and add:

```go
promptHistory := handler.NewPromptHistoryHandler(repository.NewAgentsRepo(pool))
r.Get("/api/v1/agents/prompt-history", promptHistory.List)
```

Add this BEFORE the `r.Route("/api/v1/agents", ...)` block to prevent route conflicts.

- [ ] **Step 4: Verify build**

Run: `go build ./...`
Expected: Build succeeds

- [ ] **Step 5: Commit**

```bash
git add internal/repository/agents.go internal/handler/prompt_history.go internal/router/router.go
git commit -m "feat: add GET /api/v1/agents/prompt-history endpoint"
```

---

## Task 9: Prompt History — Frontend Page

**Files:**
- Create: `frontend/src/pages/PromptHistory.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/components/sidebar.tsx`

- [ ] **Step 1: Create PromptHistory page**

```tsx
// frontend/src/pages/PromptHistory.tsx
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { apiFetch } from '../api';
import { PageHeader } from '../components/page-header';
import { Card, CardContent } from '../components/ui/card';
import { Badge } from '../components/ui/badge';
import { Skeleton } from '../components/ui/skeleton';
import { EmptyState } from '../components/empty-state';
import { History, ChevronDown } from 'lucide-react';

interface HistoryEntry {
  id: string;
  agent_name: string;
  old_prompt: string;
  new_prompt: string;
  reason: string;
  created_at: string;
}

export default function PromptHistoryPage() {
  const { data: entries, isLoading } = useQuery({
    queryKey: ['prompt-history'],
    queryFn: () => apiFetch<HistoryEntry[]>('/api/v1/agents/prompt-history'),
  });

  const [expanded, setExpanded] = useState<Record<string, boolean>>({});

  if (isLoading) {
    return (
      <div>
        <PageHeader title="Prompt History" description="Auto-tune history from weekly analyzer" />
        <div className="space-y-3">
          {[1, 2, 3].map(i => (
            <div key={i} className="rounded-xl border p-4 space-y-2">
              <div className="flex gap-3">
                <Skeleton className="h-5 w-20 rounded-full" />
                <Skeleton className="h-5 w-48" />
              </div>
              <Skeleton className="h-4 w-64" />
            </div>
          ))}
        </div>
      </div>
    );
  }

  if (!entries?.length) {
    return (
      <div>
        <PageHeader title="Prompt History" description="Auto-tune history from weekly analyzer" />
        <EmptyState
          icon={History}
          title="No prompt changes yet"
          description="The weekly analyzer will auto-tune agent prompts based on YouTube performance data. Changes will appear here."
        />
      </div>
    );
  }

  return (
    <div>
      <PageHeader title="Prompt History" description="Auto-tune history from weekly analyzer" />
      <div className="space-y-3">
        {entries.map(entry => {
          const isOpen = expanded[entry.id] ?? false;
          return (
            <Card key={entry.id}>
              <div
                className="flex items-center justify-between px-4 py-3 cursor-pointer hover:bg-muted/50 transition-colors"
                onClick={() => setExpanded(prev => ({ ...prev, [entry.id]: !prev[entry.id] }))}
              >
                <div className="flex items-center gap-3">
                  <Badge variant="outline" className="capitalize">{entry.agent_name}</Badge>
                  <span className="text-sm">{entry.reason}</span>
                </div>
                <div className="flex items-center gap-3">
                  <span className="text-xs text-muted-foreground">
                    {new Date(entry.created_at).toLocaleDateString('th-TH')}
                  </span>
                  <ChevronDown className={`h-4 w-4 text-muted-foreground transition-transform ${isOpen ? 'rotate-180' : ''}`} />
                </div>
              </div>
              {isOpen && (
                <CardContent className="pt-0 space-y-3">
                  <div>
                    <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-1">Before</p>
                    <pre className="text-xs bg-muted rounded-md p-3 whitespace-pre-wrap max-h-48 overflow-y-auto">
                      {entry.old_prompt}
                    </pre>
                  </div>
                  <div>
                    <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-1">After</p>
                    <pre className="text-xs bg-muted rounded-md p-3 whitespace-pre-wrap max-h-48 overflow-y-auto">
                      {entry.new_prompt}
                    </pre>
                  </div>
                </CardContent>
              )}
            </Card>
          );
        })}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Add route to App.tsx**

Add import:
```tsx
import PromptHistoryPage from "./pages/PromptHistory"
```

Add route inside `<Routes>` after agents:
```tsx
<Route path="/prompt-history" element={<PromptHistoryPage />} />
```

- [ ] **Step 3: Add link to sidebar (under Agents)**

In `sidebar.tsx`, add `History` to lucide imports and add to `CONFIG_NAV`:

```tsx
import { ..., History } from "lucide-react"

export const CONFIG_NAV = [
  { to: "/knowledge", label: "Knowledge", icon: BookOpen },
  { to: "/agents", label: "Agents", icon: Bot },
  { to: "/prompt-history", label: "Prompt History", icon: History },
  { to: "/settings", label: "Settings", icon: Settings },
]
```

- [ ] **Step 4: Verify frontend build**

Run: `cd frontend && npm run build`
Expected: Build succeeds

- [ ] **Step 5: Commit**

```bash
git add frontend/src/pages/PromptHistory.tsx frontend/src/App.tsx frontend/src/components/sidebar.tsx
git commit -m "feat: add Prompt History page with collapsible diff viewer"
```

---

## Verification Checklist

After all tasks are complete:

- [ ] `go build ./...` passes (backend)
- [ ] `cd frontend && npm run build` passes (frontend)
- [ ] All 6 pages show skeletons instead of "Loading..." text
- [ ] Content, Knowledge, Analytics show empty states when no data
- [ ] App does not crash on component errors (ErrorBoundary catches)
- [ ] Sidebar collapses on viewport < 768px, hamburger menu works
- [ ] Content page shows KPI cards (Total, Published, Ready, Failed)
- [ ] Prompt History page loads and shows entries (or empty state)
- [ ] `GET /api/v1/agents/prompt-history` returns JSON with data array
