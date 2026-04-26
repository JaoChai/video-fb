import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiFetch } from '../api';

interface Clip {
  id: string; title: string; question: string; questioner_name: string;
  category: string; status: string; created_at: string;
}

export default function ContentPage() {
  const qc = useQueryClient();
  const { data: clips, isLoading } = useQuery({ queryKey: ['clips'], queryFn: () => apiFetch<Clip[]>('/api/v1/clips') });
  const produce = useMutation({
    mutationFn: () => apiFetch('/api/v1/orchestrator/produce?count=7', { method: 'POST' }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['clips'] }),
  });

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <h1 style={{ fontSize: 24 }}>Content Manager</h1>
        <button onClick={() => produce.mutate()} disabled={produce.isPending}
          style={{ background: '#f5851f', color: '#fff', border: 'none', padding: '10px 24px', borderRadius: 8, cursor: 'pointer', fontSize: 14 }}>
          {produce.isPending ? 'Producing...' : 'Produce 7 Clips'}
        </button>
      </div>
      {isLoading ? <p>Loading...</p> : (
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr style={{ borderBottom: '1px solid #334155' }}>
              <th style={{ textAlign: 'left', padding: 12 }}>Title</th>
              <th style={{ textAlign: 'left', padding: 12 }}>Category</th>
              <th style={{ textAlign: 'left', padding: 12 }}>Status</th>
              <th style={{ textAlign: 'left', padding: 12 }}>Created</th>
            </tr>
          </thead>
          <tbody>
            {clips?.map(clip => (
              <tr key={clip.id} style={{ borderBottom: '1px solid #1e293b' }}>
                <td style={{ padding: 12 }}>{clip.title}</td>
                <td style={{ padding: 12 }}><span style={{ background: '#1e293b', padding: '4px 12px', borderRadius: 12, fontSize: 12 }}>{clip.category}</span></td>
                <td style={{ padding: 12 }}><span style={{ background: clip.status === 'published' ? '#059669' : clip.status === 'ready' ? '#f5851f' : '#475569', padding: '4px 12px', borderRadius: 12, fontSize: 12 }}>{clip.status}</span></td>
                <td style={{ padding: 12, fontSize: 12, color: '#94a3b8' }}>{new Date(clip.created_at).toLocaleDateString('th-TH')}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
