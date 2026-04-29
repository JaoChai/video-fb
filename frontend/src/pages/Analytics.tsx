import { useQuery } from '@tanstack/react-query';
import { useMemo, useState } from 'react';
import { apiFetch } from '../api';
import { PageHeader } from '../components/page-header';
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card';
import { Eye, ThumbsUp, MessageSquare, Share2, Clock, TrendingUp } from 'lucide-react';

interface Clip { id: string; title: string; status: string; category: string; }
interface ClipAnalytics {
  id: string; clip_id: string; platform: string;
  views: number; likes: number; comments: number; shares: number;
  watch_time_seconds: number; retention_rate: number; fetched_at: string;
}

export default function AnalyticsPage() {
  const [selectedClipId, setSelectedClipId] = useState<string | null>(null);

  const { data: clips, isLoading } = useQuery({
    queryKey: ['clips'],
    queryFn: () => apiFetch<Clip[]>('/api/v1/clips'),
  });

  const { data: analytics, isLoading: analyticsLoading } = useQuery({
    queryKey: ['clip-analytics', selectedClipId],
    queryFn: () => apiFetch<ClipAnalytics[]>(`/api/v1/clips/${selectedClipId}/analytics`),
    enabled: !!selectedClipId,
  });

  const { published, stats } = useMemo(() => {
    const pub: Clip[] = [];
    const counts = { total: 0, published: 0, ready: 0, draft: 0 };
    clips?.forEach(c => {
      counts.total++;
      if (c.status === 'published') { counts.published++; pub.push(c); }
      else if (c.status === 'ready') counts.ready++;
      else if (c.status === 'draft') counts.draft++;
    });
    return {
      published: pub,
      stats: [
        { label: 'Total', value: counts.total },
        { label: 'Published', value: counts.published },
        { label: 'Ready', value: counts.ready },
        { label: 'Draft', value: counts.draft },
      ],
    };
  }, [clips]);

  const analyticsMap = useMemo(() => {
    const map = new Map<string, ClipAnalytics>();
    analytics?.forEach(a => map.set(a.platform, a));
    return map;
  }, [analytics]);

  const platforms = ['youtube', 'tiktok', 'instagram', 'facebook'];

  const kpiIcons = {
    Views: Eye,
    Likes: ThumbsUp,
    Comments: MessageSquare,
    Shares: Share2,
    'Watch Time': Clock,
    Retention: TrendingUp,
  } as const;

  return (
    <div>
      <PageHeader title="Analytics" />

      <div className="grid grid-cols-4 gap-3 mb-10">
        {stats.map(({ label, value }) => (
          <Card key={label}>
            <CardContent className="p-5">
              <div className="text-xs text-muted-foreground uppercase tracking-wide mb-2">{label}</div>
              <div className="text-3xl font-bold tabular-nums">{value}</div>
            </CardContent>
          </Card>
        ))}
      </div>

      <h2 className="text-sm font-semibold mb-4">Published Clips</h2>
      {isLoading ? (
        <p className="text-muted-foreground">Loading...</p>
      ) : published.length === 0 ? (
        <Card>
          <CardContent className="p-6">
            <p className="text-sm text-muted-foreground">No published clips yet.</p>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-2 mb-8">
          {published.map(clip => (
            <Card
              key={clip.id}
              className={`cursor-pointer transition-colors hover:bg-muted/50 ${
                clip.id === selectedClipId ? 'border-ring bg-muted/30' : ''
              }`}
              onClick={() => setSelectedClipId(clip.id === selectedClipId ? null : clip.id)}
            >
              <CardContent className="flex items-center justify-between p-3 px-4">
                <span className="text-sm">{clip.title}</span>
                <span className="text-xs text-muted-foreground">{clip.category}</span>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {selectedClipId && (
        <div>
          <h2 className="text-sm font-semibold mb-4">Platform Analytics</h2>
          {analyticsLoading ? (
            <p className="text-muted-foreground">Loading analytics...</p>
          ) : !analytics || analytics.length === 0 ? (
            <Card>
              <CardContent className="p-6">
                <p className="text-sm text-muted-foreground">No analytics data yet for this clip.</p>
              </CardContent>
            </Card>
          ) : (
            <div className="grid grid-cols-2 gap-3">
              {platforms.map(platform => {
                const data = analyticsMap.get(platform);
                if (!data) return null;

                const kpis = [
                  { label: 'Views' as const, value: data.views.toLocaleString() },
                  { label: 'Likes' as const, value: data.likes.toLocaleString() },
                  { label: 'Comments' as const, value: data.comments.toLocaleString() },
                  { label: 'Shares' as const, value: data.shares.toLocaleString() },
                  { label: 'Watch Time' as const, value: `${Math.round(data.watch_time_seconds / 60)}m` },
                  { label: 'Retention' as const, value: `${(data.retention_rate * 100).toFixed(1)}%` },
                ];

                return (
                  <Card key={platform}>
                    <CardHeader className="pb-3">
                      <CardTitle className="text-sm font-semibold capitalize">{platform}</CardTitle>
                    </CardHeader>
                    <CardContent>
                      <div className="grid grid-cols-2 gap-3">
                        {kpis.map(({ label, value }) => {
                          const Icon = kpiIcons[label];
                          return (
                            <div key={label} className="flex items-start gap-2">
                              <Icon className="h-4 w-4 text-muted-foreground mt-0.5 shrink-0" />
                              <div>
                                <div className="text-[11px] text-muted-foreground mb-0.5">{label}</div>
                                <div className="text-base font-semibold tabular-nums">{value}</div>
                              </div>
                            </div>
                          );
                        })}
                      </div>
                    </CardContent>
                  </Card>
                );
              })}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
