import { useQuery } from '@tanstack/react-query';
import { apiFetch } from '../api';
import ProductionProgress from '../components/ProductionProgress';

interface Clip {
  id: string; title: string; question: string; questioner_name: string;
  category: string; status: string; created_at: string;
}

const statusColor: Record<string, string> = {
  published: '#22c55e',
  ready: '#f59e0b',
  producing: '#f5851f',
  failed: '#ef4444',
  draft: '#555',
};

export default function ContentPage() {
  const { data: clips, isLoading } = useQuery({ queryKey: ['clips'], queryFn: () => apiFetch<Clip[]>('/api/v1/clips') });
  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 32 }}>
        <h1 style={{ fontSize: 20, fontWeight: 600 }}>Content</h1>
      </div>
      <ProductionProgress />
      {isLoading ? <p style={{ color: '#555' }}>Loading...</p> : !clips?.length ? (
        <p style={{ color: '#555' }}>No clips yet. Scheduler will auto-produce at noon & midnight.</p>
      ) : (
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr style={{ borderBottom: '1px solid #1a1a1a' }}>
              {['Title', 'Category', 'Status', 'Created'].map(h => (
                <th key={h} style={{ textAlign: 'left', padding: '10px 12px', fontSize: 12, fontWeight: 500, color: '#555', textTransform: 'uppercase', letterSpacing: '0.05em' }}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {clips.map(clip => (
              <tr key={clip.id} style={{ borderBottom: '1px solid #111', transition: 'background 0.15s' }}
                onMouseEnter={e => (e.currentTarget.style.background = '#111')}
                onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}>
                <td style={{ padding: '12px', fontSize: 14 }}>{clip.title}</td>
                <td style={{ padding: '12px', fontSize: 13, color: '#888' }}>{clip.category}</td>
                <td style={{ padding: '12px' }}>
                  <span style={{
                    display: 'inline-flex', alignItems: 'center', gap: 6, fontSize: 12,
                  }}>
                    <span style={{ width: 6, height: 6, borderRadius: '50%', background: statusColor[clip.status] || '#555' }} />
                    {clip.status}
                  </span>
                </td>
                <td style={{ padding: '12px', fontSize: 12, color: '#555' }}>{new Date(clip.created_at).toLocaleDateString('th-TH')}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
