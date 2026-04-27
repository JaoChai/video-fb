import { useQuery } from '@tanstack/react-query';
import { useMemo, useState } from 'react';
import { apiFetch } from '../api';

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

  return (
    <div>
      <h1 style={{ fontSize: 20, fontWeight: 600, marginBottom: 32 }}>Analytics</h1>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 12, marginBottom: 40 }}>
        {stats.map(({ label, value }) => (
          <div key={label} style={{
            background: '#111', borderRadius: 8, padding: '20px 24px',
            border: '1px solid #1a1a1a',
          }}>
            <div style={{ fontSize: 12, color: '#555', marginBottom: 8, textTransform: 'uppercase', letterSpacing: '0.05em' }}>{label}</div>
            <div style={{ fontSize: 28, fontWeight: 700, fontVariantNumeric: 'tabular-nums' }}>{value}</div>
          </div>
        ))}
      </div>

      <h2 style={{ fontSize: 15, fontWeight: 600, marginBottom: 16 }}>Published Clips</h2>
      {isLoading ? <p style={{ color: '#555' }}>Loading...</p> : published.length === 0 ? (
        <p style={{ color: '#555', fontSize: 14 }}>No published clips yet.</p>
      ) : (
        <div style={{ display: 'grid', gap: 8, marginBottom: 32 }}>
          {published.map(clip => (
            <div key={clip.id} onClick={() => setSelectedClipId(clip.id === selectedClipId ? null : clip.id)} style={{
              background: clip.id === selectedClipId ? '#1a1a1a' : '#111',
              borderRadius: 6, padding: '12px 16px',
              border: clip.id === selectedClipId ? '1px solid #333' : '1px solid #1a1a1a',
              display: 'flex', justifyContent: 'space-between', alignItems: 'center',
              cursor: 'pointer', transition: 'background 0.15s',
            }}>
              <span style={{ fontSize: 14 }}>{clip.title}</span>
              <span style={{ fontSize: 12, color: '#555' }}>{clip.category}</span>
            </div>
          ))}
        </div>
      )}

      {selectedClipId && (
        <div>
          <h2 style={{ fontSize: 15, fontWeight: 600, marginBottom: 16 }}>
            Platform Analytics
          </h2>
          {analyticsLoading ? <p style={{ color: '#555' }}>Loading analytics...</p> :
           !analytics || analytics.length === 0 ? (
            <p style={{ color: '#555', fontSize: 14 }}>No analytics data yet for this clip.</p>
          ) : (
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: 12 }}>
              {platforms.map(platform => {
                const data = analyticsMap.get(platform);
                if (!data) return null;
                return (
                  <div key={platform} style={{
                    background: '#111', borderRadius: 8, padding: 20,
                    border: '1px solid #1a1a1a',
                  }}>
                    <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 16, textTransform: 'capitalize' }}>
                      {platform}
                    </div>
                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
                      {[
                        { label: 'Views', value: data.views.toLocaleString() },
                        { label: 'Likes', value: data.likes.toLocaleString() },
                        { label: 'Comments', value: data.comments.toLocaleString() },
                        { label: 'Shares', value: data.shares.toLocaleString() },
                        { label: 'Watch Time', value: `${Math.round(data.watch_time_seconds / 60)}m` },
                        { label: 'Retention', value: `${(data.retention_rate * 100).toFixed(1)}%` },
                      ].map(({ label, value }) => (
                        <div key={label}>
                          <div style={{ fontSize: 11, color: '#555', marginBottom: 4 }}>{label}</div>
                          <div style={{ fontSize: 16, fontWeight: 600, fontVariantNumeric: 'tabular-nums' }}>{value}</div>
                        </div>
                      ))}
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
