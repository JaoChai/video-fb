import { useQuery } from '@tanstack/react-query';
import { apiFetch } from '../api';

interface Clip { id: string; title: string; status: string; category: string; }

export default function AnalyticsPage() {
  const { data: clips, isLoading } = useQuery({ queryKey: ['clips'], queryFn: () => apiFetch<Clip[]>('/api/v1/clips') });
  const published = clips?.filter(c => c.status === 'published') || [];

  const stats = [
    { label: 'Total', value: clips?.length || 0 },
    { label: 'Published', value: published.length },
    { label: 'Ready', value: clips?.filter(c => c.status === 'ready').length || 0 },
    { label: 'Draft', value: clips?.filter(c => c.status === 'draft').length || 0 },
  ];

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
        <div style={{ display: 'grid', gap: 8 }}>
          {published.map(clip => (
            <div key={clip.id} style={{
              background: '#111', borderRadius: 6, padding: '12px 16px',
              border: '1px solid #1a1a1a',
              display: 'flex', justifyContent: 'space-between', alignItems: 'center',
            }}>
              <span style={{ fontSize: 14 }}>{clip.title}</span>
              <span style={{ fontSize: 12, color: '#555' }}>{clip.category}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
