import { useQuery } from '@tanstack/react-query';
import { apiFetch } from '../api';

interface Clip { id: string; title: string; status: string; category: string; }

export default function AnalyticsPage() {
  const { data: clips, isLoading } = useQuery({ queryKey: ['clips'], queryFn: () => apiFetch<Clip[]>('/api/v1/clips') });
  const published = clips?.filter(c => c.status === 'published') || [];

  return (
    <div>
      <h1 style={{ fontSize: 24, marginBottom: 24 }}>Analytics Dashboard</h1>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 16, marginBottom: 32 }}>
        {[
          { label: 'Total Clips', value: clips?.length || 0, color: '#3b82f6' },
          { label: 'Published', value: published.length, color: '#059669' },
          { label: 'Ready', value: clips?.filter(c => c.status === 'ready').length || 0, color: '#f5851f' },
          { label: 'Draft', value: clips?.filter(c => c.status === 'draft').length || 0, color: '#64748b' },
        ].map(({ label, value, color }) => (
          <div key={label} style={{ background: '#1e293b', borderRadius: 12, padding: 20, borderLeft: `4px solid ${color}` }}>
            <p style={{ fontSize: 13, color: '#94a3b8', marginBottom: 8 }}>{label}</p>
            <p style={{ fontSize: 32, fontWeight: 'bold' }}>{value}</p>
          </div>
        ))}
      </div>
      <h2 style={{ fontSize: 18, marginBottom: 16 }}>Published Clips</h2>
      {isLoading ? <p>Loading...</p> : published.length === 0 ? <p style={{ color: '#64748b' }}>No published clips yet</p> : (
        <div style={{ display: 'grid', gap: 12 }}>
          {published.map(clip => (
            <div key={clip.id} style={{ background: '#1e293b', borderRadius: 8, padding: 16, display: 'flex', justifyContent: 'space-between' }}>
              <span>{clip.title}</span>
              <span style={{ color: '#94a3b8', fontSize: 12 }}>{clip.category}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
