import { useQuery } from '@tanstack/react-query';
import { apiFetch } from '../api';

interface Source { id: string; name: string; url: string; source_type: string; crawl_frequency: string; last_crawled_at: string | null; enabled: boolean; }

const typeColor: Record<string, string> = {
  official: '#888',
  practitioner: '#666',
  community: '#555',
};

export default function KnowledgePage() {
  const { data: sources, isLoading } = useQuery({ queryKey: ['knowledge'], queryFn: () => apiFetch<Source[]>('/api/v1/knowledge/sources') });

  return (
    <div>
      <h1 style={{ fontSize: 20, fontWeight: 600, marginBottom: 32 }}>Knowledge Sources</h1>
      {isLoading ? <p style={{ color: '#555' }}>Loading...</p> : (
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr style={{ borderBottom: '1px solid #1a1a1a' }}>
              {['Source', 'Type', 'Frequency', 'Last Crawled', 'Status'].map(h => (
                <th key={h} style={{ textAlign: 'left', padding: '10px 12px', fontSize: 12, fontWeight: 500, color: '#555', textTransform: 'uppercase', letterSpacing: '0.05em' }}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {sources?.map(s => (
              <tr key={s.id} style={{ borderBottom: '1px solid #111', transition: 'background 0.15s' }}
                onMouseEnter={e => (e.currentTarget.style.background = '#111')}
                onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}>
                <td style={{ padding: '12px' }}>
                  <a href={s.url} target="_blank" rel="noopener noreferrer" style={{ fontSize: 14, borderBottom: '1px solid #333' }}>{s.name}</a>
                </td>
                <td style={{ padding: '12px', fontSize: 12, color: typeColor[s.source_type] || '#555' }}>{s.source_type}</td>
                <td style={{ padding: '12px', fontSize: 13, color: '#888' }}>{s.crawl_frequency}</td>
                <td style={{ padding: '12px', fontSize: 12, color: '#555' }}>{s.last_crawled_at ? new Date(s.last_crawled_at).toLocaleDateString('th-TH') : '—'}</td>
                <td style={{ padding: '12px' }}>
                  <span style={{
                    display: 'inline-flex', alignItems: 'center', gap: 6, fontSize: 12,
                  }}>
                    <span style={{ width: 6, height: 6, borderRadius: '50%', background: s.enabled ? '#22c55e' : '#ef4444' }} />
                    {s.enabled ? 'Active' : 'Disabled'}
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
