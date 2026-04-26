import { useQuery } from '@tanstack/react-query';
import { apiFetch } from '../api';

interface Source { id: string; name: string; url: string; source_type: string; crawl_frequency: string; last_crawled_at: string | null; enabled: boolean; }

export default function KnowledgePage() {
  const { data: sources, isLoading } = useQuery({ queryKey: ['knowledge'], queryFn: () => apiFetch<Source[]>('/api/v1/knowledge/sources') });

  return (
    <div>
      <h1 style={{ fontSize: 24, marginBottom: 24 }}>Knowledge Sources</h1>
      {isLoading ? <p>Loading...</p> : (
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr style={{ borderBottom: '1px solid #334155' }}>
              <th style={{ textAlign: 'left', padding: 12 }}>Source</th>
              <th style={{ textAlign: 'left', padding: 12 }}>Type</th>
              <th style={{ textAlign: 'left', padding: 12 }}>Frequency</th>
              <th style={{ textAlign: 'left', padding: 12 }}>Last Crawled</th>
              <th style={{ textAlign: 'left', padding: 12 }}>Status</th>
            </tr>
          </thead>
          <tbody>
            {sources?.map(s => (
              <tr key={s.id} style={{ borderBottom: '1px solid #1e293b' }}>
                <td style={{ padding: 12 }}><a href={s.url} target="_blank" style={{ color: '#60a5fa' }}>{s.name}</a></td>
                <td style={{ padding: 12 }}><span style={{ background: s.source_type === 'official' ? '#1d4ed8' : s.source_type === 'practitioner' ? '#7c3aed' : '#059669', padding: '4px 12px', borderRadius: 12, fontSize: 12 }}>{s.source_type}</span></td>
                <td style={{ padding: 12, fontSize: 13 }}>{s.crawl_frequency}</td>
                <td style={{ padding: 12, fontSize: 12, color: '#94a3b8' }}>{s.last_crawled_at ? new Date(s.last_crawled_at).toLocaleDateString('th-TH') : 'Never'}</td>
                <td style={{ padding: 12 }}><span style={{ background: s.enabled ? '#059669' : '#dc2626', padding: '4px 12px', borderRadius: 12, fontSize: 12 }}>{s.enabled ? 'Active' : 'Disabled'}</span></td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
