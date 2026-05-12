import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useMemo, useState } from 'react';
import { apiFetch } from '../api';
import { PageHeader } from '../components/page-header';
import { Card, CardContent } from '../components/ui/card';
import { Button } from '../components/ui/button';
import {
  Table, TableHeader, TableBody, TableRow, TableHead, TableCell,
} from '../components/ui/table';
import { Eye, ThumbsUp, MessageSquare, Share2, Clock, TrendingUp, BarChart3, ChevronDown, ChevronUp } from 'lucide-react';
import { EmptyState } from '../components/empty-state';
import { Skeleton } from '../components/ui/skeleton';
import { cn } from '../lib/utils';

interface AnalyticsSummary {
  total_views: number;
  total_likes: number;
  total_comments: number;
  total_shares: number;
  avg_retention_rate: number;
  total_watch_time_seconds: number;
  clip_count: number;
}

interface ClipPerformance {
  clip_id: string;
  title: string;
  category: string;
  views: number;
  likes: number;
  comments: number;
  shares: number;
  retention_rate: number;
  watch_time_seconds: number;
}

interface ClipAnalytics {
  id: string; clip_id: string; platform: string;
  views: number; likes: number; comments: number; shares: number;
  watch_time_seconds: number; retention_rate: number; fetched_at: string;
}

interface SummaryResponse {
  summary: AnalyticsSummary;
  top_clips: ClipPerformance[] | null;
  last_fetched_at: string | null;
}

function formatNum(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return n.toLocaleString();
}

function formatWatchTime(seconds: number): string {
  const hrs = Math.floor(seconds / 3600);
  const mins = Math.round((seconds % 3600) / 60);
  if (hrs > 0) return `${hrs}h ${mins}m`;
  return `${mins}m`;
}

const KPI_CONFIG = [
  { key: 'total_views' as const, label: 'Views', icon: Eye },
  { key: 'total_likes' as const, label: 'Likes', icon: ThumbsUp },
  { key: 'total_comments' as const, label: 'Comments', icon: MessageSquare },
  { key: 'total_shares' as const, label: 'Shares', icon: Share2 },
  { key: 'avg_retention_rate' as const, label: 'Avg Retention', icon: TrendingUp },
  { key: 'total_watch_time_seconds' as const, label: 'Watch Time', icon: Clock },
];

function formatKpiValue(key: string, val: number): string {
  if (key === 'avg_retention_rate') return `${(val * 100).toFixed(1)}%`;
  if (key === 'total_watch_time_seconds') return formatWatchTime(val);
  return formatNum(val);
}

export default function AnalyticsPage() {
  const [expandedClipId, setExpandedClipId] = useState<string | null>(null);
  const queryClient = useQueryClient();

  const { data: summaryData, isLoading } = useQuery({
    queryKey: ['analytics-summary'],
    queryFn: () => apiFetch<SummaryResponse>('/api/v1/analytics/summary'),
  });

  const triggerFetch = useMutation({
    mutationFn: () => apiFetch('/api/v1/analytics/fetch', { method: 'POST' }),
    onSuccess: () => {
      setTimeout(() => queryClient.invalidateQueries({ queryKey: ['analytics-summary'] }), 15000);
    },
  });

  const { data: clipAnalytics, isLoading: detailLoading } = useQuery({
    queryKey: ['clip-analytics', expandedClipId],
    queryFn: () => apiFetch<ClipAnalytics[]>(`/api/v1/clips/${expandedClipId}/analytics`),
    enabled: !!expandedClipId,
  });

  const summary = summaryData?.summary;
  const topClips = summaryData?.top_clips ?? [];

  const platformMap = useMemo(() => {
    if (detailLoading) return new Map<string, ClipAnalytics>();
    const map = new Map<string, ClipAnalytics>();
    clipAnalytics?.forEach(a => map.set(a.platform, a));
    return map;
  }, [clipAnalytics, detailLoading]);

  return (
    <div>
      <PageHeader title="Analytics" />

      <div className="mb-4 flex items-center justify-between">
        <div className="text-xs text-muted-foreground">
          {summaryData?.last_fetched_at
            ? <>Last updated: {new Date(summaryData.last_fetched_at).toLocaleString('th-TH')}
                {Date.now() - new Date(summaryData.last_fetched_at).getTime() > 36 * 3600 * 1000 && (
                  <span className="ml-2 text-amber-600">⚠ data over 36h old</span>
                )}</>
            : 'No fetch yet'}
        </div>
        <Button
          size="sm"
          variant="outline"
          disabled={triggerFetch.isPending}
          onClick={() => triggerFetch.mutate()}
        >
          {triggerFetch.isPending ? 'Fetching…' : 'Refresh now'}
        </Button>
      </div>

      {isLoading ? (
        <div className="grid grid-cols-2 md:grid-cols-3 gap-3 mb-8">
          {[1, 2, 3, 4, 5, 6].map(i => (
            <div key={i} className="rounded-xl border p-4 space-y-2">
              <Skeleton className="h-3 w-16" />
              <Skeleton className="h-7 w-20" />
            </div>
          ))}
        </div>
      ) : !summary || summary.clip_count === 0 ? (
        <EmptyState
          icon={BarChart3}
          title="No analytics data"
          description="Publish clips to YouTube first, then analytics data will appear here."
        />
      ) : (
        <>
          <div className="grid grid-cols-2 md:grid-cols-3 gap-3 mb-8">
            {KPI_CONFIG.map(({ key, label, icon: Icon }) => (
              <Card key={key}>
                <CardContent className="p-4">
                  <div className="flex items-center gap-2 mb-2">
                    <Icon className="size-4 text-muted-foreground" />
                    <span className="text-xs text-muted-foreground uppercase tracking-wide">{label}</span>
                  </div>
                  <div className="text-2xl font-bold tabular-nums">
                    {formatKpiValue(key, summary[key])}
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>

          <div className="mb-2 flex items-center justify-between">
            <h2 className="text-sm font-semibold">Top Clips</h2>
            <span className="text-xs text-muted-foreground">{summary.clip_count} published clips</span>
          </div>

          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="pl-4">Title</TableHead>
                <TableHead className="hidden sm:table-cell">Category</TableHead>
                <TableHead className="text-right">Views</TableHead>
                <TableHead className="hidden md:table-cell text-right">Likes</TableHead>
                <TableHead className="hidden md:table-cell text-right">Retention</TableHead>
                <TableHead className="w-[40px]" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {topClips.map((clip, idx) => {
                const isExpanded = expandedClipId === clip.clip_id;
                return (
                  <TableRow
                    key={clip.clip_id}
                    className={cn('cursor-pointer', isExpanded && 'bg-muted/30')}
                    onClick={() => setExpandedClipId(isExpanded ? null : clip.clip_id)}
                  >
                    <TableCell className="pl-4 py-3">
                      <div className="flex items-center gap-2">
                        <span className="text-xs text-muted-foreground tabular-nums w-5">{idx + 1}</span>
                        <div>
                          <div className="text-sm font-medium line-clamp-1">{clip.title}</div>
                          <div className="sm:hidden text-xs text-muted-foreground mt-0.5">{clip.category}</div>
                          {isExpanded && (
                            <div className="mt-3 space-y-2">
                              {detailLoading ? (
                                <div className="flex gap-3">
                                  {[1, 2].map(i => <Skeleton key={i} className="h-16 w-40" />)}
                                </div>
                              ) : platformMap.size === 0 ? (
                                <p className="text-xs text-muted-foreground">No platform data</p>
                              ) : (
                                <div className="flex flex-wrap gap-2">
                                  {['youtube', 'tiktok', 'instagram', 'facebook'].map(p => {
                                    const d = platformMap.get(p);
                                    if (!d) return null;
                                    return (
                                      <div key={p} className="rounded-lg border bg-background p-2.5 min-w-[140px]">
                                        <div className="text-xs font-medium capitalize mb-1.5">{p}</div>
                                        <div className="grid grid-cols-2 gap-x-3 gap-y-1 text-xs">
                                          <span className="text-muted-foreground">Views</span>
                                          <span className="tabular-nums text-right">{formatNum(d.views)}</span>
                                          <span className="text-muted-foreground">Likes</span>
                                          <span className="tabular-nums text-right">{formatNum(d.likes)}</span>
                                          <span className="text-muted-foreground">Retention</span>
                                          <span className="tabular-nums text-right">{(d.retention_rate * 100).toFixed(1)}%</span>
                                          <span className="text-muted-foreground">Watch</span>
                                          <span className="tabular-nums text-right">{formatWatchTime(d.watch_time_seconds)}</span>
                                        </div>
                                      </div>
                                    );
                                  })}
                                </div>
                              )}
                            </div>
                          )}
                        </div>
                      </div>
                    </TableCell>
                    <TableCell className="hidden sm:table-cell text-sm text-muted-foreground">
                      {clip.category}
                    </TableCell>
                    <TableCell className="text-right tabular-nums text-sm font-medium">
                      {formatNum(clip.views)}
                    </TableCell>
                    <TableCell className="hidden md:table-cell text-right tabular-nums text-sm text-muted-foreground">
                      {formatNum(clip.likes)}
                    </TableCell>
                    <TableCell className="hidden md:table-cell text-right tabular-nums text-sm text-muted-foreground">
                      {(clip.retention_rate * 100).toFixed(1)}%
                    </TableCell>
                    <TableCell className="pr-3">
                      <Button variant="ghost" size="icon" className="size-7">
                        {isExpanded ? <ChevronUp className="size-4" /> : <ChevronDown className="size-4" />}
                      </Button>
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </>
      )}
    </div>
  );
}
