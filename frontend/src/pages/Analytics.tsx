import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useMemo, useRef, useState } from 'react'
import { Eye, ThumbsUp, MessageSquare, Share2, Clock, TrendingUp, BarChart3, AlertTriangle } from 'lucide-react'
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
import { formatNum, formatWatch } from '../lib/format'

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

function StatusLine({ lastFetchedAt, clipCount }: { lastFetchedAt: string | null | undefined; clipCount: number | undefined }) {
  if (!lastFetchedAt) {
    return <div className="text-xs text-muted-foreground">No fetch yet</div>
  }
  const fetched = new Date(lastFetchedAt)
  const stale = Date.now() - fetched.getTime() > 36 * 3600 * 1000
  return (
    <div className="text-xs text-muted-foreground flex items-center gap-2">
      {clipCount !== undefined && <span>{clipCount} published clips</span>}
      <span>· Last updated {fetched.toLocaleString('th-TH')}</span>
      {stale && (
        <span className="inline-flex items-center gap-1 text-amber-600">
          <AlertTriangle className="size-3.5" aria-hidden />
          data over 36h old
        </span>
      )}
    </div>
  )
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
        <StatusLine lastFetchedAt={data?.last_fetched_at} clipCount={summary?.clip_count} />
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
