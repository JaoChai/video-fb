import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { apiFetch } from '../api';
import ProductionProgress from '../components/ProductionProgress';

interface Clip {
  id: string; title: string; question: string; questioner_name: string;
  category: string; status: string; created_at: string;
  fail_reason?: string; retry_count: number;
}

const statusColor: Record<string, string> = {
  published: '#22c55e',
  ready: '#f59e0b',
  producing: '#f5851f',
  failed: '#ef4444',
  draft: '#555',
};

export default function ContentPage() {
  const queryClient = useQueryClient();
  const { data: clips, isLoading } = useQuery({ queryKey: ['clips'], queryFn: () => apiFetch<Clip[]>('/api/v1/clips') });
  const [retrying, setRetrying] = useState(false);

  const failedCount = clips?.filter(c => c.status === 'failed').length || 0;

  const handleRetryAll = async () => {
    setRetrying(true);
    try {
      await apiFetch('/api/v1/orchestrator/retry', { method: 'POST' });
      setTimeout(() => queryClient.invalidateQueries({ queryKey: ['clips'] }), 2000);
    } catch (e) {
      console.error('Retry failed:', e);
    } finally {
      setRetrying(false);
    }
  };

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 32 }}>
        <h1 style={{ fontSize: 20, fontWeight: 600 }}>Content</h1>
        {failedCount > 0 && (
          <button
            onClick={handleRetryAll}
            disabled={retrying}
            style={{
              padding: '8px 16px', fontSize: 13, fontWeight: 500,
              background: retrying ? '#333' : '#ef4444', color: '#fff',
              border: 'none', borderRadius: 6, cursor: retrying ? 'not-allowed' : 'pointer',
              opacity: retrying ? 0.6 : 1, transition: 'all 0.15s',
            }}
          >
            {retrying ? 'Retrying...' : `Retry Failed (${failedCount})`}
          </button>
        )}
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
                <td style={{ padding: '12px', fontSize: 14 }}>
                  {clip.title}
                  {clip.status === 'failed' && clip.fail_reason && (
                    <div style={{ fontSize: 11, color: '#ef4444', marginTop: 4, opacity: 0.8 }}>
                      {clip.fail_reason}
                    </div>
                  )}
                </td>
                <td style={{ padding: '12px', fontSize: 13, color: '#888' }}>{clip.category}</td>
                <td style={{ padding: '12px' }}>
                  <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6, fontSize: 12 }}>
                    <span style={{ width: 6, height: 6, borderRadius: '50%', background: statusColor[clip.status] || '#555' }} />
                    {clip.status}
                    {clip.status === 'failed' && clip.retry_count > 0 && (
                      <span style={{ fontSize: 10, color: '#888' }}>({clip.retry_count}/2)</span>
                    )}
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
