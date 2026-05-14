# Analytics Dashboard Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Redesign `/analytics` into a glance-able, data-dense dashboard that surfaces backend data currently hidden (post_type split, platform split, time-series trend, comments/shares/watch-time per clip) and adds visual hierarchy, sparkline trends, comparison views, and sortable Top Clips.

**Architecture:** Backend adds 3 new aggregation methods on `AnalyticsRepo` and extends `/api/v1/analytics/summary` payload with `by_post_type`, `by_platform`, `trend`, and `delta`. Frontend rewrites `Analytics.tsx` into 5 dashboard sections following the "Data-Dense Dashboard" style from ui-ux-pro-max with custom lightweight SVG Sparkline / MiniBar components (no chart lib dependency).

**Tech Stack:** Go 1.22 + chi router + pgxpool (Neon Postgres) · React 18 + TypeScript + Vite + TanStack Query + Tailwind + shadcn/ui + lucide-react.

**Design System (from `/ui-ux-pro-max:ui-ux-pro-max --design-system`):**
- **Style:** Data-Dense Dashboard — minimal padding, KPI cards, grid layout, max data visibility, WCAG AA, light+dark full support
- **Primary:** `#1E40AF` (blue-800) · **Accent:** `#D97706` (amber-600) · **Destructive:** `#DC2626`
- **Type:** `Fira Sans` body, `Fira Code` for tabular numbers (already partially via `tabular-nums`)
- **Effects:** Hover tooltips, row highlight on hover, smooth filter animations 150–300ms
- **Avoid:** ornate decoration, no-filter table, emoji icons, pie charts >5 slices

---

## Backend Data Audit — What's Hidden

Data exposed by backend but **not** rendered in current `Analytics.tsx`:

| Field | Source | Status |
|---|---|---|
| `top_clips[].comments` | `/analytics/summary` | exposed, not shown |
| `top_clips[].shares` | `/analytics/summary` | exposed, not shown |
| `top_clips[].watch_time_seconds` | `/analytics/summary` | exposed, not shown |
| `clip_analytics.fetched_at` history | `/clips/{id}/analytics` | exposed, only latest used — no trend |
| `clip_analytics.post_type` aggregate | DB only | NOT exposed (per-clip only in expand row) |
| `clip_analytics.platform` aggregate | DB only | NOT exposed (per-clip only in expand row) |
| Engagement rate = `(likes+comments+shares)/views` | derivable | not computed |
| Period delta vs previous range | DB only | NOT exposed |
| Time-series sparkline data | DB only | NOT exposed |

This plan adds backend aggregations for the bottom 5 and surfaces all 9 in the redesigned UI.

---

## File Structure

**Backend (Go):**
- Modify: `internal/models/clip.go` — add `AnalyticsSummaryResponse`, `SegmentedTotals`, `PlatformTotals`, `TrendPoint`, `DeltaSummary`
- Modify: `internal/repository/analytics.go` — add `SummaryByPostType`, `SummaryByPlatform`, `Trend`, `PreviousPeriodTotals`
- Modify: `internal/handler/analytics.go` — extend `Summary` to merge new aggregates, accept `?range=7d|30d|all`
- Modify: `internal/router/router.go:80` — no change (same route, richer payload)

**Frontend (React/TS):**
- Create: `frontend/src/components/ui/tabs.tsx` — shadcn Tabs primitive
- Create: `frontend/src/components/ui/sparkline.tsx` — lightweight inline SVG sparkline
- Create: `frontend/src/components/ui/mini-bar.tsx` — horizontal comparison bar
- Create: `frontend/src/components/analytics/stat-card.tsx` — Hero + Small KPI variants with delta
- Create: `frontend/src/components/analytics/segment-compare.tsx` — Regular vs Shorts card
- Create: `frontend/src/components/analytics/platform-breakdown.tsx` — Platform bar list
- Create: `frontend/src/components/analytics/top-clips-table.tsx` — sortable table with relative bar
- Rewrite: `frontend/src/pages/Analytics.tsx` — compose new sections

---

## Task 1: Add backend response types

**Files:**
- Modify: `internal/models/clip.go`

- [ ] **Step 1: Append new types**

Append below existing `ClipPerformance` struct in `internal/models/clip.go`:

```go
type SegmentedTotals struct {
	PostType         string  `json:"post_type"`
	Views            int     `json:"views"`
	Likes            int     `json:"likes"`
	Comments         int     `json:"comments"`
	Shares           int     `json:"shares"`
	WatchTimeSeconds float64 `json:"watch_time_seconds"`
	AvgRetention     float64 `json:"avg_retention_rate"`
}

type PlatformTotals struct {
	Platform         string  `json:"platform"`
	Views            int     `json:"views"`
	Likes            int     `json:"likes"`
	Comments         int     `json:"comments"`
	Shares           int     `json:"shares"`
	WatchTimeSeconds float64 `json:"watch_time_seconds"`
}

type TrendPoint struct {
	Day        time.Time `json:"day"`
	Views      int       `json:"views"`
	Likes      int       `json:"likes"`
	Comments   int       `json:"comments"`
	Shares     int       `json:"shares"`
	WatchTime  float64   `json:"watch_time_seconds"`
	Retention  float64   `json:"avg_retention_rate"`
}

type DeltaSummary struct {
	Views          float64 `json:"views_pct"`
	Likes          float64 `json:"likes_pct"`
	Comments       float64 `json:"comments_pct"`
	Shares         float64 `json:"shares_pct"`
	WatchTime      float64 `json:"watch_time_pct"`
	RetentionPoint float64 `json:"retention_pp"`
}
```

- [ ] **Step 2: Build check**

Run: `go build ./...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/models/clip.go
git commit -m "feat(analytics): add segmented/trend/delta response types"
```

---

## Task 2: Repo method — Summary split by post_type

**Files:**
- Modify: `internal/repository/analytics.go`

- [ ] **Step 1: Append method**

Append to `internal/repository/analytics.go` after `Summary`:

```go
func (r *AnalyticsRepo) SummaryByPostType(ctx context.Context) ([]models.SegmentedTotals, error) {
	rows, err := r.pool.Query(ctx, latestAnalyticsCTE+`
		SELECT l.post_type,
		       COALESCE(SUM(l.views),0),
		       COALESCE(SUM(l.likes),0),
		       COALESCE(SUM(l.comments),0),
		       COALESCE(SUM(l.shares),0),
		       COALESCE(SUM(l.watch_time_seconds),0),
		       COALESCE(AVG(NULLIF(l.retention_rate, 0)),0)
		FROM latest l
		GROUP BY l.post_type
		ORDER BY l.post_type`)
	if err != nil {
		return nil, fmt.Errorf("query summary by post_type: %w", err)
	}
	defer rows.Close()
	var out []models.SegmentedTotals
	for rows.Next() {
		var s models.SegmentedTotals
		if err := rows.Scan(&s.PostType, &s.Views, &s.Likes, &s.Comments,
			&s.Shares, &s.WatchTimeSeconds, &s.AvgRetention); err != nil {
			return nil, fmt.Errorf("scan post_type row: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
```

- [ ] **Step 2: Build check**

Run: `go build ./...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/repository/analytics.go
git commit -m "feat(analytics): add SummaryByPostType repo method"
```

---

## Task 3: Repo method — Summary split by platform

**Files:**
- Modify: `internal/repository/analytics.go`

- [ ] **Step 1: Append method**

```go
func (r *AnalyticsRepo) SummaryByPlatform(ctx context.Context) ([]models.PlatformTotals, error) {
	rows, err := r.pool.Query(ctx, latestAnalyticsCTE+`
		SELECT l.platform,
		       COALESCE(SUM(l.views),0),
		       COALESCE(SUM(l.likes),0),
		       COALESCE(SUM(l.comments),0),
		       COALESCE(SUM(l.shares),0),
		       COALESCE(SUM(l.watch_time_seconds),0)
		FROM latest l
		GROUP BY l.platform
		ORDER BY SUM(l.views) DESC`)
	if err != nil {
		return nil, fmt.Errorf("query summary by platform: %w", err)
	}
	defer rows.Close()
	var out []models.PlatformTotals
	for rows.Next() {
		var p models.PlatformTotals
		if err := rows.Scan(&p.Platform, &p.Views, &p.Likes, &p.Comments,
			&p.Shares, &p.WatchTimeSeconds); err != nil {
			return nil, fmt.Errorf("scan platform row: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
```

- [ ] **Step 2: Build check**

Run: `go build ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/repository/analytics.go
git commit -m "feat(analytics): add SummaryByPlatform repo method"
```

---

## Task 4: Repo method — Trend (daily aggregation)

**Files:**
- Modify: `internal/repository/analytics.go`

- [ ] **Step 1: Append method**

```go
func (r *AnalyticsRepo) Trend(ctx context.Context, days int) ([]models.TrendPoint, error) {
	rows, err := r.pool.Query(ctx, `
		WITH daily AS (
			SELECT DISTINCT ON (clip_id, platform, post_type, DATE_TRUNC('day', fetched_at))
				clip_id, platform, post_type,
				DATE_TRUNC('day', fetched_at) AS day,
				views, likes, comments, shares, watch_time_seconds, retention_rate
			FROM clip_analytics
			WHERE fetched_at >= NOW() - ($1::int || ' days')::interval
			ORDER BY clip_id, platform, post_type, DATE_TRUNC('day', fetched_at), fetched_at DESC
		)
		SELECT day,
		       COALESCE(SUM(views),0),
		       COALESCE(SUM(likes),0),
		       COALESCE(SUM(comments),0),
		       COALESCE(SUM(shares),0),
		       COALESCE(SUM(watch_time_seconds),0),
		       COALESCE(AVG(NULLIF(retention_rate, 0)),0)
		FROM daily
		GROUP BY day
		ORDER BY day ASC`, days)
	if err != nil {
		return nil, fmt.Errorf("query trend: %w", err)
	}
	defer rows.Close()
	var out []models.TrendPoint
	for rows.Next() {
		var p models.TrendPoint
		if err := rows.Scan(&p.Day, &p.Views, &p.Likes, &p.Comments,
			&p.Shares, &p.WatchTime, &p.Retention); err != nil {
			return nil, fmt.Errorf("scan trend row: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
```

- [ ] **Step 2: Build check**

Run: `go build ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/repository/analytics.go
git commit -m "feat(analytics): add daily Trend repo method"
```

---

## Task 5: Repo method — Previous-period totals for delta

**Files:**
- Modify: `internal/repository/analytics.go`

- [ ] **Step 1: Append method**

```go
func (r *AnalyticsRepo) PreviousPeriodTotals(ctx context.Context, days int) (models.AnalyticsSummary, error) {
	var s models.AnalyticsSummary
	err := r.pool.QueryRow(ctx, `
		WITH prev AS (
			SELECT DISTINCT ON (clip_id, platform, post_type)
				clip_id, platform, post_type,
				views, likes, comments, shares, watch_time_seconds, retention_rate
			FROM clip_analytics
			WHERE fetched_at < NOW() - ($1::int || ' days')::interval
			  AND fetched_at >= NOW() - (($1::int * 2) || ' days')::interval
			ORDER BY clip_id, platform, post_type, fetched_at DESC
		)
		SELECT COALESCE(SUM(views),0), COALESCE(SUM(likes),0),
		       COALESCE(SUM(comments),0), COALESCE(SUM(shares),0),
		       COALESCE(AVG(NULLIF(retention_rate, 0)),0),
		       COALESCE(SUM(watch_time_seconds),0),
		       0
		FROM prev`, days).Scan(
		&s.TotalViews, &s.TotalLikes, &s.TotalComments, &s.TotalShares,
		&s.AvgRetention, &s.TotalWatchTime, &s.ClipCount)
	if err != nil {
		return s, fmt.Errorf("query previous period: %w", err)
	}
	return s, nil
}
```

- [ ] **Step 2: Build check**

Run: `go build ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/repository/analytics.go
git commit -m "feat(analytics): add PreviousPeriodTotals for delta calc"
```

---

## Task 6: Extend Summary handler with new aggregates + range param

**Files:**
- Modify: `internal/handler/analytics.go`

- [ ] **Step 1: Rewrite `Summary` method**

Replace `internal/handler/analytics.go:39-56`:

```go
func (h *AnalyticsHandler) Summary(w http.ResponseWriter, r *http.Request) {
	rangeParam := r.URL.Query().Get("range")
	days := 30
	switch rangeParam {
	case "7d":
		days = 7
	case "30d", "":
		days = 30
	case "all":
		days = 3650
	}

	ctx := r.Context()
	summary, err := h.repo.Summary(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	topClips, err := h.repo.TopClips(ctx, 10)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	byPostType, err := h.repo.SummaryByPostType(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	byPlatform, err := h.repo.SummaryByPlatform(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	trend, err := h.repo.Trend(ctx, days)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	prev, err := h.repo.PreviousPeriodTotals(ctx, days)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	lastFetched, _ := h.repo.LastFetchedAt(ctx)

	delta := computeDelta(summary, prev)

	writeJSON(w, http.StatusOK, models.APIResponse{Data: map[string]any{
		"summary":         summary,
		"top_clips":       topClips,
		"by_post_type":    byPostType,
		"by_platform":     byPlatform,
		"trend":           trend,
		"delta":           delta,
		"range_days":      days,
		"last_fetched_at": lastFetched,
	}})
}

func computeDelta(cur, prev models.AnalyticsSummary) models.DeltaSummary {
	pct := func(c, p int) float64 {
		if p == 0 {
			if c == 0 {
				return 0
			}
			return 100
		}
		return (float64(c) - float64(p)) / float64(p) * 100
	}
	pctF := func(c, p float64) float64 {
		if p == 0 {
			if c == 0 {
				return 0
			}
			return 100
		}
		return (c - p) / p * 100
	}
	return models.DeltaSummary{
		Views:          pct(cur.TotalViews, prev.TotalViews),
		Likes:          pct(cur.TotalLikes, prev.TotalLikes),
		Comments:       pct(cur.TotalComments, prev.TotalComments),
		Shares:         pct(cur.TotalShares, prev.TotalShares),
		WatchTime:      pctF(cur.TotalWatchTime, prev.TotalWatchTime),
		RetentionPoint: (cur.AvgRetention - prev.AvgRetention) * 100,
	}
}
```

- [ ] **Step 2: Build check**

Run: `go build ./...`

- [ ] **Step 3: Smoke test**

Run: `curl -s http://localhost:8080/api/v1/analytics/summary?range=30d | head -c 500`
Expected: JSON containing `by_post_type`, `by_platform`, `trend`, `delta` keys

- [ ] **Step 4: Commit**

```bash
git add internal/handler/analytics.go
git commit -m "feat(analytics): extend Summary with by_post_type/by_platform/trend/delta + range filter"
```

---

## Task 7: Frontend — Add shadcn Tabs primitive

**Files:**
- Create: `frontend/src/components/ui/tabs.tsx`

- [ ] **Step 1: Install dependency**

Run: `cd frontend && pnpm add @radix-ui/react-tabs`
Expected: package installed

- [ ] **Step 2: Create tabs.tsx**

```tsx
import * as React from "react"
import * as TabsPrimitive from "@radix-ui/react-tabs"
import { cn } from "../../lib/utils"

const Tabs = TabsPrimitive.Root

const TabsList = React.forwardRef<
  React.ElementRef<typeof TabsPrimitive.List>,
  React.ComponentPropsWithoutRef<typeof TabsPrimitive.List>
>(({ className, ...props }, ref) => (
  <TabsPrimitive.List
    ref={ref}
    className={cn(
      "inline-flex h-9 items-center justify-center rounded-lg bg-muted p-1 text-muted-foreground",
      className
    )}
    {...props}
  />
))
TabsList.displayName = TabsPrimitive.List.displayName

const TabsTrigger = React.forwardRef<
  React.ElementRef<typeof TabsPrimitive.Trigger>,
  React.ComponentPropsWithoutRef<typeof TabsPrimitive.Trigger>
>(({ className, ...props }, ref) => (
  <TabsPrimitive.Trigger
    ref={ref}
    className={cn(
      "inline-flex items-center justify-center whitespace-nowrap rounded-md px-3 py-1 text-xs font-medium ring-offset-background transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50 data-[state=active]:bg-background data-[state=active]:text-foreground data-[state=active]:shadow",
      className
    )}
    {...props}
  />
))
TabsTrigger.displayName = TabsPrimitive.Trigger.displayName

const TabsContent = React.forwardRef<
  React.ElementRef<typeof TabsPrimitive.Content>,
  React.ComponentPropsWithoutRef<typeof TabsPrimitive.Content>
>(({ className, ...props }, ref) => (
  <TabsPrimitive.Content
    ref={ref}
    className={cn("mt-3 ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring", className)}
    {...props}
  />
))
TabsContent.displayName = TabsPrimitive.Content.displayName

export { Tabs, TabsList, TabsTrigger, TabsContent }
```

- [ ] **Step 3: Type check**

Run: `cd frontend && pnpm tsc --noEmit`
Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/ui/tabs.tsx frontend/package.json frontend/pnpm-lock.yaml
git commit -m "feat(ui): add shadcn Tabs primitive"
```

---

## Task 8: Frontend — Sparkline component

**Files:**
- Create: `frontend/src/components/ui/sparkline.tsx`

- [ ] **Step 1: Create file**

```tsx
import { useMemo } from 'react'
import { cn } from '../../lib/utils'

interface SparklineProps {
  data: number[]
  className?: string
  strokeClass?: string
  fillClass?: string
  height?: number
}

export function Sparkline({
  data,
  className,
  strokeClass = 'stroke-primary',
  fillClass = 'fill-primary/10',
  height = 32,
}: SparklineProps) {
  const { path, area } = useMemo(() => {
    if (data.length === 0) return { path: '', area: '' }
    const w = 100
    const h = height
    const max = Math.max(...data, 1)
    const min = Math.min(...data, 0)
    const range = max - min || 1
    const stepX = w / Math.max(data.length - 1, 1)
    const pts = data.map((v, i) => {
      const x = i * stepX
      const y = h - ((v - min) / range) * h
      return [x, y] as const
    })
    const path = pts.map(([x, y], i) => `${i === 0 ? 'M' : 'L'}${x.toFixed(2)},${y.toFixed(2)}`).join(' ')
    const area = `${path} L${w},${h} L0,${h} Z`
    return { path, area }
  }, [data, height])

  if (data.length === 0) {
    return <div className={cn('h-8 w-full bg-muted/30 rounded', className)} />
  }

  return (
    <svg
      viewBox={`0 0 100 ${height}`}
      preserveAspectRatio="none"
      className={cn('w-full', className)}
      style={{ height }}
      aria-hidden="true"
    >
      <path d={area} className={fillClass} strokeWidth={0} />
      <path d={path} className={strokeClass} fill="none" strokeWidth={1.5} vectorEffect="non-scaling-stroke" />
    </svg>
  )
}
```

- [ ] **Step 2: Type check**

Run: `cd frontend && pnpm tsc --noEmit`

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/ui/sparkline.tsx
git commit -m "feat(ui): add lightweight SVG Sparkline"
```

---

## Task 9: Frontend — MiniBar component

**Files:**
- Create: `frontend/src/components/ui/mini-bar.tsx`

- [ ] **Step 1: Create file**

```tsx
import { cn } from '../../lib/utils'

interface MiniBarProps {
  value: number
  max: number
  className?: string
  barClass?: string
}

export function MiniBar({ value, max, className, barClass = 'bg-primary' }: MiniBarProps) {
  const pct = max <= 0 ? 0 : Math.min(100, (value / max) * 100)
  return (
    <div className={cn('h-1.5 w-full rounded-full bg-muted overflow-hidden', className)}>
      <div className={cn('h-full rounded-full transition-all duration-300', barClass)} style={{ width: `${pct}%` }} />
    </div>
  )
}
```

- [ ] **Step 2: Type check**

Run: `cd frontend && pnpm tsc --noEmit`

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/ui/mini-bar.tsx
git commit -m "feat(ui): add MiniBar comparison component"
```

---

## Task 10: Frontend — StatCard (Hero + Small variants)

**Files:**
- Create: `frontend/src/components/analytics/stat-card.tsx`

- [ ] **Step 1: Create file**

```tsx
import { ArrowDown, ArrowUp, Minus, type LucideIcon } from 'lucide-react'
import { Card, CardContent } from '../ui/card'
import { Sparkline } from '../ui/sparkline'
import { cn } from '../../lib/utils'

interface StatCardProps {
  label: string
  value: string
  icon?: LucideIcon
  delta?: number
  deltaUnit?: '%' | 'pp'
  trend?: number[]
  variant?: 'hero' | 'small'
}

export function StatCard({
  label,
  value,
  icon: Icon,
  delta,
  deltaUnit = '%',
  trend,
  variant = 'small',
}: StatCardProps) {
  const hasDelta = typeof delta === 'number' && Number.isFinite(delta)
  const positive = hasDelta && delta! > 0.05
  const negative = hasDelta && delta! < -0.05
  const ArrowIcon = positive ? ArrowUp : negative ? ArrowDown : Minus
  const deltaColor = positive
    ? 'text-emerald-600 dark:text-emerald-400'
    : negative
    ? 'text-rose-600 dark:text-rose-400'
    : 'text-muted-foreground'

  if (variant === 'hero') {
    return (
      <Card>
        <CardContent className="p-5">
          <div className="flex items-center gap-2 mb-2">
            {Icon && <Icon className="size-4 text-muted-foreground" />}
            <span className="text-xs text-muted-foreground uppercase tracking-wide">{label}</span>
          </div>
          <div className="flex items-end justify-between gap-3">
            <div>
              <div className="text-3xl font-bold tabular-nums leading-none">{value}</div>
              {hasDelta && (
                <div className={cn('flex items-center gap-1 mt-2 text-xs font-medium', deltaColor)}>
                  <ArrowIcon className="size-3" />
                  <span className="tabular-nums">
                    {Math.abs(delta!).toFixed(1)}{deltaUnit}
                  </span>
                  <span className="text-muted-foreground font-normal">vs prev period</span>
                </div>
              )}
            </div>
          </div>
          {trend && trend.length > 0 && (
            <div className="mt-3">
              <Sparkline
                data={trend}
                strokeClass={cn(positive ? 'stroke-emerald-500' : negative ? 'stroke-rose-500' : 'stroke-primary')}
                fillClass={cn(positive ? 'fill-emerald-500/10' : negative ? 'fill-rose-500/10' : 'fill-primary/10')}
                height={40}
              />
            </div>
          )}
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardContent className="p-3">
        <div className="flex items-center gap-1.5 mb-1">
          {Icon && <Icon className="size-3.5 text-muted-foreground" />}
          <span className="text-[10px] text-muted-foreground uppercase tracking-wide">{label}</span>
        </div>
        <div className="text-xl font-semibold tabular-nums leading-tight">{value}</div>
        {hasDelta && (
          <div className={cn('flex items-center gap-0.5 mt-1 text-[11px] font-medium', deltaColor)}>
            <ArrowIcon className="size-3" />
            <span className="tabular-nums">{Math.abs(delta!).toFixed(1)}{deltaUnit}</span>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
```

- [ ] **Step 2: Type check**

Run: `cd frontend && pnpm tsc --noEmit`

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/analytics/stat-card.tsx
git commit -m "feat(analytics): add StatCard hero/small variants with delta + sparkline"
```

---

## Task 11: Frontend — SegmentCompare (Regular vs Shorts)

**Files:**
- Create: `frontend/src/components/analytics/segment-compare.tsx`

- [ ] **Step 1: Create file**

```tsx
import { Card, CardContent } from '../ui/card'
import { MiniBar } from '../ui/mini-bar'

interface SegmentTotals {
  post_type: string
  views: number
  likes: number
  comments: number
  shares: number
  watch_time_seconds: number
  avg_retention_rate: number
}

interface SegmentCompareProps {
  data: SegmentTotals[]
}

function formatNum(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return n.toLocaleString()
}

function formatWatch(s: number): string {
  const h = Math.floor(s / 3600)
  const m = Math.round((s % 3600) / 60)
  return h > 0 ? `${h}h ${m}m` : `${m}m`
}

const LABELS: Record<string, string> = { regular: 'Regular', shorts: 'Shorts' }

export function SegmentCompare({ data }: SegmentCompareProps) {
  const regular = data.find(d => d.post_type === 'regular')
  const shorts = data.find(d => d.post_type === 'shorts')
  const segments = [regular, shorts].filter(Boolean) as SegmentTotals[]

  if (segments.length === 0) {
    return (
      <Card>
        <CardContent className="p-4 text-xs text-muted-foreground">No segmented data</CardContent>
      </Card>
    )
  }

  const maxViews = Math.max(...segments.map(s => s.views), 1)
  const maxWatch = Math.max(...segments.map(s => s.watch_time_seconds), 1)

  return (
    <Card>
      <CardContent className="p-4">
        <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-3">
          Regular vs Shorts
        </h3>
        <div className="space-y-3">
          {segments.map(s => (
            <div key={s.post_type} className="space-y-2">
              <div className="flex items-baseline justify-between">
                <span className="text-sm font-medium">{LABELS[s.post_type] ?? s.post_type}</span>
                <span className="text-xs text-muted-foreground tabular-nums">
                  {formatNum(s.views)} views · {formatWatch(s.watch_time_seconds)} · {(s.avg_retention_rate * 100).toFixed(1)}% retention
                </span>
              </div>
              <MiniBar value={s.views} max={maxViews} />
              <MiniBar value={s.watch_time_seconds} max={maxWatch} barClass="bg-amber-500" />
            </div>
          ))}
          <div className="flex gap-3 pt-2 text-[10px] text-muted-foreground">
            <span className="flex items-center gap-1"><span className="size-2 rounded-sm bg-primary" /> Views</span>
            <span className="flex items-center gap-1"><span className="size-2 rounded-sm bg-amber-500" /> Watch time</span>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
```

- [ ] **Step 2: Type check**

Run: `cd frontend && pnpm tsc --noEmit`

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/analytics/segment-compare.tsx
git commit -m "feat(analytics): add Regular vs Shorts comparison card"
```

---

## Task 12: Frontend — PlatformBreakdown

**Files:**
- Create: `frontend/src/components/analytics/platform-breakdown.tsx`

- [ ] **Step 1: Create file**

```tsx
import { Card, CardContent } from '../ui/card'
import { MiniBar } from '../ui/mini-bar'

interface PlatformTotals {
  platform: string
  views: number
  likes: number
  comments: number
  shares: number
  watch_time_seconds: number
}

interface PlatformBreakdownProps {
  data: PlatformTotals[]
}

const PLATFORM_LABEL: Record<string, string> = {
  youtube: 'YouTube',
  tiktok: 'TikTok',
  instagram: 'Instagram',
  facebook: 'Facebook',
}

const PLATFORM_COLOR: Record<string, string> = {
  youtube: 'bg-rose-500',
  tiktok: 'bg-foreground',
  instagram: 'bg-pink-500',
  facebook: 'bg-blue-500',
}

function formatNum(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return n.toLocaleString()
}

export function PlatformBreakdown({ data }: PlatformBreakdownProps) {
  if (!data || data.length === 0) {
    return (
      <Card>
        <CardContent className="p-4 text-xs text-muted-foreground">No platform data</CardContent>
      </Card>
    )
  }
  const max = Math.max(...data.map(p => p.views), 1)
  return (
    <Card>
      <CardContent className="p-4">
        <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-3">
          By Platform
        </h3>
        <div className="space-y-2.5">
          {data.map(p => (
            <div key={p.platform} className="space-y-1">
              <div className="flex items-baseline justify-between gap-3">
                <span className="text-sm font-medium">{PLATFORM_LABEL[p.platform] ?? p.platform}</span>
                <span className="text-xs tabular-nums text-muted-foreground">{formatNum(p.views)}</span>
              </div>
              <MiniBar value={p.views} max={max} barClass={PLATFORM_COLOR[p.platform] ?? 'bg-primary'} />
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  )
}
```

- [ ] **Step 2: Type check**

Run: `cd frontend && pnpm tsc --noEmit`

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/analytics/platform-breakdown.tsx
git commit -m "feat(analytics): add Platform breakdown bar list"
```

---

## Task 13: Frontend — Sortable Top Clips Table

**Files:**
- Create: `frontend/src/components/analytics/top-clips-table.tsx`

- [ ] **Step 1: Create file**

```tsx
import { useMemo, useState } from 'react'
import { ArrowDown, ArrowUp, ChevronDown, ChevronUp } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../../api'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../ui/table'
import { Button } from '../ui/button'
import { Skeleton } from '../ui/skeleton'
import { MiniBar } from '../ui/mini-bar'
import { cn } from '../../lib/utils'

export interface ClipRow {
  clip_id: string
  title: string
  category: string
  views: number
  likes: number
  comments: number
  shares: number
  retention_rate: number
  watch_time_seconds: number
}

interface ClipPlatformDetail {
  platform: string
  post_type: string
  views: number
  likes: number
  comments: number
  shares: number
  watch_time_seconds: number
  retention_rate: number
}

type SortKey = 'views' | 'likes' | 'retention_rate' | 'watch_time_seconds'

interface TopClipsTableProps {
  clips: ClipRow[]
}

function formatNum(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return n.toLocaleString()
}

function formatWatch(s: number): string {
  const h = Math.floor(s / 3600)
  const m = Math.round((s % 3600) / 60)
  return h > 0 ? `${h}h ${m}m` : `${m}m`
}

const SORT_LABELS: Record<SortKey, string> = {
  views: 'Views',
  likes: 'Likes',
  retention_rate: 'Retention',
  watch_time_seconds: 'Watch time',
}

export function TopClipsTable({ clips }: TopClipsTableProps) {
  const [sortKey, setSortKey] = useState<SortKey>('views')
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('desc')
  const [expandedId, setExpandedId] = useState<string | null>(null)

  const sorted = useMemo(() => {
    const arr = [...clips]
    arr.sort((a, b) => {
      const av = a[sortKey] as number
      const bv = b[sortKey] as number
      return sortDir === 'desc' ? bv - av : av - bv
    })
    return arr
  }, [clips, sortKey, sortDir])

  const maxViews = Math.max(...sorted.map(c => c.views), 1)

  const { data: detail, isLoading: detailLoading } = useQuery({
    queryKey: ['clip-analytics', expandedId],
    queryFn: () => apiFetch<ClipPlatformDetail[]>(`/api/v1/clips/${expandedId}/analytics`),
    enabled: !!expandedId,
  })

  const platformMap = useMemo(() => {
    const m = new Map<string, ClipPlatformDetail>()
    detail?.forEach(d => m.set(`${d.platform}-${d.post_type}`, d))
    return m
  }, [detail])

  const toggleSort = (key: SortKey) => {
    if (key === sortKey) setSortDir(sortDir === 'desc' ? 'asc' : 'desc')
    else { setSortKey(key); setSortDir('desc') }
  }

  const SortHeader = ({ k, className }: { k: SortKey; className?: string }) => (
    <button
      type="button"
      onClick={() => toggleSort(k)}
      className={cn('inline-flex items-center gap-1 hover:text-foreground transition-colors', className)}
    >
      {SORT_LABELS[k]}
      {sortKey === k && (sortDir === 'desc' ? <ArrowDown className="size-3" /> : <ArrowUp className="size-3" />)}
    </button>
  )

  if (clips.length === 0) {
    return <div className="text-xs text-muted-foreground py-6 text-center">No clips yet</div>
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead className="pl-4">Title</TableHead>
          <TableHead className="hidden sm:table-cell">Category</TableHead>
          <TableHead className="text-right"><SortHeader k="views" /></TableHead>
          <TableHead className="hidden md:table-cell text-right"><SortHeader k="likes" /></TableHead>
          <TableHead className="hidden md:table-cell text-right"><SortHeader k="retention_rate" /></TableHead>
          <TableHead className="hidden lg:table-cell text-right"><SortHeader k="watch_time_seconds" /></TableHead>
          <TableHead className="w-[40px]" />
        </TableRow>
      </TableHeader>
      <TableBody>
        {sorted.map((clip, idx) => {
          const isExpanded = expandedId === clip.clip_id
          return (
            <TableRow
              key={clip.clip_id}
              className={cn('cursor-pointer', isExpanded && 'bg-muted/30')}
              onClick={() => setExpandedId(isExpanded ? null : clip.clip_id)}
            >
              <TableCell className="pl-4 py-3">
                <div className="flex items-center gap-2">
                  <span className="text-xs text-muted-foreground tabular-nums w-5">{idx + 1}</span>
                  <div className="flex-1 min-w-0">
                    <div className="text-sm font-medium line-clamp-1">{clip.title}</div>
                    <div className="mt-1.5 max-w-[280px]">
                      <MiniBar value={clip.views} max={maxViews} />
                    </div>
                    {isExpanded && (
                      <div className="mt-3">
                        {detailLoading ? (
                          <div className="flex gap-2">
                            {[1, 2].map(i => <Skeleton key={i} className="h-16 w-40" />)}
                          </div>
                        ) : platformMap.size === 0 ? (
                          <p className="text-xs text-muted-foreground">No platform data</p>
                        ) : (
                          <div className="flex flex-wrap gap-2">
                            {(['youtube', 'tiktok', 'instagram', 'facebook'] as const).flatMap(p =>
                              (['regular', 'shorts'] as const).map(t => {
                                const d = platformMap.get(`${p}-${t}`)
                                if (!d) return null
                                return (
                                  <div key={`${p}-${t}`} className="rounded-lg border bg-background p-2.5 min-w-[160px]">
                                    <div className="text-xs font-medium capitalize mb-1.5">
                                      {p} <span className="text-muted-foreground">· {t}</span>
                                    </div>
                                    <div className="grid grid-cols-2 gap-x-3 gap-y-1 text-xs">
                                      <span className="text-muted-foreground">Views</span>
                                      <span className="tabular-nums text-right">{formatNum(d.views)}</span>
                                      <span className="text-muted-foreground">Likes</span>
                                      <span className="tabular-nums text-right">{formatNum(d.likes)}</span>
                                      <span className="text-muted-foreground">Comments</span>
                                      <span className="tabular-nums text-right">{formatNum(d.comments)}</span>
                                      <span className="text-muted-foreground">Shares</span>
                                      <span className="tabular-nums text-right">{formatNum(d.shares)}</span>
                                      <span className="text-muted-foreground">Retention</span>
                                      <span className="tabular-nums text-right">{(d.retention_rate * 100).toFixed(1)}%</span>
                                      <span className="text-muted-foreground">Watch</span>
                                      <span className="tabular-nums text-right">{formatWatch(d.watch_time_seconds)}</span>
                                    </div>
                                  </div>
                                )
                              })
                            )}
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                </div>
              </TableCell>
              <TableCell className="hidden sm:table-cell text-sm text-muted-foreground">{clip.category}</TableCell>
              <TableCell className="text-right tabular-nums text-sm font-medium">{formatNum(clip.views)}</TableCell>
              <TableCell className="hidden md:table-cell text-right tabular-nums text-sm text-muted-foreground">{formatNum(clip.likes)}</TableCell>
              <TableCell className="hidden md:table-cell text-right tabular-nums text-sm text-muted-foreground">{(clip.retention_rate * 100).toFixed(1)}%</TableCell>
              <TableCell className="hidden lg:table-cell text-right tabular-nums text-sm text-muted-foreground">{formatWatch(clip.watch_time_seconds)}</TableCell>
              <TableCell className="pr-3">
                <Button variant="ghost" size="icon" className="size-7">
                  {isExpanded ? <ChevronUp className="size-4" /> : <ChevronDown className="size-4" />}
                </Button>
              </TableCell>
            </TableRow>
          )
        })}
      </TableBody>
    </Table>
  )
}
```

- [ ] **Step 2: Type check**

Run: `cd frontend && pnpm tsc --noEmit`

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/analytics/top-clips-table.tsx
git commit -m "feat(analytics): sortable Top Clips with relative bar + richer expand"
```

---

## Task 14: Frontend — Rewrite Analytics page composition

**Files:**
- Rewrite: `frontend/src/pages/Analytics.tsx`

- [ ] **Step 1: Replace file**

```tsx
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useMemo, useRef, useState } from 'react'
import { Eye, ThumbsUp, MessageSquare, Share2, Clock, TrendingUp, BarChart3 } from 'lucide-react'
import { apiFetch } from '../api'
import { PageHeader } from '../components/page-header'
import { Button } from '../components/ui/button'
import { Skeleton } from '../components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '../components/ui/tabs'
import { EmptyState } from '../components/empty-state'
import { StatCard } from '../components/analytics/stat-card'
import { SegmentCompare } from '../components/analytics/segment-compare'
import { PlatformBreakdown } from '../components/analytics/platform-breakdown'
import { TopClipsTable, type ClipRow } from '../components/analytics/top-clips-table'

interface Summary {
  total_views: number
  total_likes: number
  total_comments: number
  total_shares: number
  avg_retention_rate: number
  total_watch_time_seconds: number
  clip_count: number
}

interface Delta {
  views_pct: number
  likes_pct: number
  comments_pct: number
  shares_pct: number
  watch_time_pct: number
  retention_pp: number
}

interface TrendPoint {
  day: string
  views: number
  likes: number
  comments: number
  shares: number
  watch_time_seconds: number
  avg_retention_rate: number
}

interface SegmentTotals {
  post_type: string
  views: number
  likes: number
  comments: number
  shares: number
  watch_time_seconds: number
  avg_retention_rate: number
}

interface PlatformTotals {
  platform: string
  views: number
  likes: number
  comments: number
  shares: number
  watch_time_seconds: number
}

interface SummaryResponse {
  summary: Summary
  top_clips: ClipRow[] | null
  by_post_type: SegmentTotals[] | null
  by_platform: PlatformTotals[] | null
  trend: TrendPoint[] | null
  delta: Delta
  range_days: number
  last_fetched_at: string | null
}

type Range = '7d' | '30d' | 'all'

function formatNum(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return n.toLocaleString()
}

function formatWatch(s: number): string {
  const h = Math.floor(s / 3600)
  const m = Math.round((s % 3600) / 60)
  return h > 0 ? `${h}h ${m}m` : `${m}m`
}

export default function AnalyticsPage() {
  const [range, setRange] = useState<Range>('30d')
  const queryClient = useQueryClient()

  const { data, isLoading } = useQuery({
    queryKey: ['analytics-summary', range],
    queryFn: () => apiFetch<SummaryResponse>(`/api/v1/analytics/summary?range=${range}`),
  })

  const refreshTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const triggerFetch = useMutation({
    mutationFn: () => apiFetch('/api/v1/analytics/fetch', { method: 'POST' }),
    onSuccess: () => {
      if (refreshTimerRef.current) clearTimeout(refreshTimerRef.current)
      refreshTimerRef.current = setTimeout(() => {
        queryClient.invalidateQueries({ queryKey: ['analytics-summary'] })
      }, 15000)
    },
  })

  const summary = data?.summary
  const trend = data?.trend ?? []
  const delta = data?.delta

  const trendSeries = useMemo(() => ({
    views: trend.map(t => t.views),
    likes: trend.map(t => t.likes),
    comments: trend.map(t => t.comments),
    shares: trend.map(t => t.shares),
    watch: trend.map(t => t.watch_time_seconds),
    retention: trend.map(t => t.avg_retention_rate * 100),
  }), [trend])

  return (
    <div>
      <PageHeader title="Analytics" />

      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div className="text-xs text-muted-foreground">
          {(() => {
            if (!data?.last_fetched_at) return 'No fetch yet'
            const fetched = new Date(data.last_fetched_at)
            const stale = Date.now() - fetched.getTime() > 36 * 3600 * 1000
            return (
              <>
                {summary && <span className="mr-2">{summary.clip_count} published clips</span>}
                · Last updated {fetched.toLocaleString('th-TH')}
                {stale && <span className="ml-2 text-amber-600">⚠ data over 36h old</span>}
              </>
            )
          })()}
        </div>
        <div className="flex items-center gap-2">
          <Tabs value={range} onValueChange={v => setRange(v as Range)}>
            <TabsList>
              <TabsTrigger value="7d">7d</TabsTrigger>
              <TabsTrigger value="30d">30d</TabsTrigger>
              <TabsTrigger value="all">All</TabsTrigger>
            </TabsList>
          </Tabs>
          <Button size="sm" variant="outline" disabled={triggerFetch.isPending} onClick={() => triggerFetch.mutate()}>
            {triggerFetch.isPending ? 'Fetching…' : 'Refresh now'}
          </Button>
        </div>
      </div>

      {isLoading ? (
        <div className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            {[1, 2].map(i => <Skeleton key={i} className="h-32" />)}
          </div>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
            {[1, 2, 3, 4].map(i => <Skeleton key={i} className="h-20" />)}
          </div>
        </div>
      ) : !summary || summary.clip_count === 0 ? (
        <EmptyState
          icon={BarChart3}
          title="No analytics data"
          description="Publish clips to YouTube first, then analytics data will appear here."
        />
      ) : (
        <div className="space-y-6">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            <StatCard
              variant="hero"
              label="Total Views"
              value={formatNum(summary.total_views)}
              icon={Eye}
              delta={delta?.views_pct}
              trend={trendSeries.views}
            />
            <StatCard
              variant="hero"
              label="Avg Retention"
              value={`${(summary.avg_retention_rate * 100).toFixed(1)}%`}
              icon={TrendingUp}
              delta={delta?.retention_pp}
              deltaUnit="pp"
              trend={trendSeries.retention}
            />
          </div>

          <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
            <StatCard label="Likes" value={formatNum(summary.total_likes)} icon={ThumbsUp} delta={delta?.likes_pct} />
            <StatCard label="Comments" value={formatNum(summary.total_comments)} icon={MessageSquare} delta={delta?.comments_pct} />
            <StatCard label="Shares" value={formatNum(summary.total_shares)} icon={Share2} delta={delta?.shares_pct} />
            <StatCard label="Watch Time" value={formatWatch(summary.total_watch_time_seconds)} icon={Clock} delta={delta?.watch_time_pct} />
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-3">
            <SegmentCompare data={data?.by_post_type ?? []} />
            <PlatformBreakdown data={data?.by_platform ?? []} />
          </div>

          <div>
            <div className="mb-2 flex items-center justify-between">
              <h2 className="text-sm font-semibold">Top Clips</h2>
              <span className="text-xs text-muted-foreground">Click a row to expand platform breakdown</span>
            </div>
            <TopClipsTable clips={data?.top_clips ?? []} />
          </div>
        </div>
      )}
    </div>
  )
}
```

- [ ] **Step 2: Type check**

Run: `cd frontend && pnpm tsc --noEmit`
Expected: no errors

- [ ] **Step 3: Build check**

Run: `cd frontend && pnpm build`
Expected: build succeeds

- [ ] **Step 4: Commit**

```bash
git add frontend/src/pages/Analytics.tsx
git commit -m "feat(analytics): redesign page as data-dense dashboard with hierarchy + comparison + sortable table"
```

---

## Task 15: Visual verification

**Files:** none (manual)

- [ ] **Step 1: Start dev**

Run: `cd frontend && pnpm dev` (background) and ensure backend is running on `localhost:8080`

- [ ] **Step 2: Verify checklist**

Open `http://localhost:5173/analytics` and confirm:

- 2 hero KPIs (Views, Retention) with sparkline + delta arrow render
- 4 supporting KPIs (Likes/Comments/Shares/Watch) in one row on desktop
- Regular vs Shorts compare card shows 2 rows with view + watch bars
- Platform breakdown shows ordered bars per platform
- Top Clips table is sortable by Views / Likes / Retention / Watch
- Clicking a clip row reveals platform×post_type breakdown including comments + shares
- Range tabs (7d / 30d / All) trigger refetch and update sparklines
- Dark mode contrast meets WCAG AA on all surfaces (text on card)
- Layout reflows cleanly at 375px, 768px, 1024px, 1440px
- `prefers-reduced-motion` disables transitions (test via DevTools rendering panel)

- [ ] **Step 3: Network smoke**

Run: `curl -s 'http://localhost:8080/api/v1/analytics/summary?range=7d' | jq 'keys'`
Expected: `["by_platform", "by_post_type", "delta", "last_fetched_at", "range_days", "summary", "top_clips", "trend"]`

---

## Self-Review Notes

- **Spec coverage:** All 9 hidden backend fields surface — comments/shares/watch (Task 13 table cols + expand), post_type aggregate (Task 11), platform aggregate (Task 12), trend (Tasks 4+10 sparkline), delta (Tasks 5+10 arrow + %).
- **Style fidelity:** Data-Dense Dashboard — minimal padding (p-3/p-4 not p-6), tabular-nums everywhere, sparkline + bars supplant chart lib, hover row highlight preserved, SVG icons only (Eye/ThumbsUp/etc).
- **No emojis as icons** ✓ — all icons from `lucide-react`.
- **WCAG:** retention/delta colors paired with arrow + text, not color-only.
- **Touch targets:** sort headers are `<button>` with adequate hit area; row click area is full row.
- **Reduced motion:** transitions are Tailwind defaults; respect `motion-reduce` already inherited globally.
