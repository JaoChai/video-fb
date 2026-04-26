import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiFetch } from '../api';

interface Clip {
  id: string; title: string; question: string; questioner_name: string;
  category: string; status: string; created_at: string;
}

const statusColor: Record<string, string> = {
  published: '#22c55e',
  ready: '#f59e0b',
  draft: '#555',
};

export default function ContentPage() {
  const qc = useQueryClient();
  const { data: clips, isLoading } = useQuery({ queryKey: ['clips'], queryFn: () => apiFetch<Clip[]>('/api/v1/clips') });
  const produce = useMutation({
    mutationFn: () => apiFetch('/api/v1/orchestrator/produce?count=7', { method: 'POST' }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['clips'] }),
  });

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 32 }}>
        <h1 style={{ fontSize: 20, fontWeight: 600 }}>Content</h1>
        <button onClick={() => produce.mutate()} disabled={produce.isPending}
          style={{
            background: '#fff', color: '#000', border: 'none', padding: '8px 20px',
            borderRadius: 6, cursor: 'pointer', fontSize: 13, fontWeight: 600,
            opacity: produce.isPending ? 0.5 : 1, transition: 'opacity 0.15s',
          }}>
          {produce.isPending ? 'Producing...' : 'Produce 7 Clips'}
        </button>
      </div>
      {isLoading ? <p style={{ color: '#555' }}>Loading...</p> : !clips?.length ? (
        <p style={{ color: '#555' }}>No clips yet. Click "Produce 7 Clips" to start.</p>
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
