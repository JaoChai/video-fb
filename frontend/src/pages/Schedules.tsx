import { useQuery } from '@tanstack/react-query';
import { apiFetch } from '../api';

interface Schedule { id: string; name: string; cron_expression: string; action: string; enabled: boolean; last_run_at: string | null; }

export default function SchedulesPage() {
  const { data: schedules, isLoading } = useQuery({ queryKey: ['schedules'], queryFn: () => apiFetch<Schedule[]>('/api/v1/schedules') });

  return (
    <div>
      <h1 style={{ fontSize: 20, fontWeight: 600, marginBottom: 32 }}>Schedules</h1>
      {isLoading ? <p style={{ color: '#555' }}>Loading...</p> : (
        <div style={{ display: 'grid', gap: 12 }}>
          {schedules?.map(s => (
            <div key={s.id} style={{
              background: '#111', borderRadius: 8, padding: '16px 24px',
              border: '1px solid #1a1a1a',
              display: 'flex', justifyContent: 'space-between', alignItems: 'center',
              transition: 'border-color 0.15s',
            }}
              onMouseEnter={e => (e.currentTarget.style.borderColor = '#333')}
              onMouseLeave={e => (e.currentTarget.style.borderColor = '#1a1a1a')}>
              <div>
                <div style={{ fontSize: 15, fontWeight: 500, marginBottom: 4 }}>{s.name}</div>
                <div style={{ display: 'flex', gap: 16, fontSize: 12, color: '#555' }}>
                  <span>{s.action}</span>
                  <span style={{ fontFamily: 'monospace' }}>{s.cron_expression}</span>
                  {s.last_run_at && <span>Last: {new Date(s.last_run_at).toLocaleString('th-TH')}</span>}
                </div>
              </div>
              <span style={{
                fontSize: 11, fontWeight: 500, padding: '3px 10px', borderRadius: 4,
                background: s.enabled ? 'rgba(34,197,94,0.15)' : 'rgba(239,68,68,0.15)',
                color: s.enabled ? '#22c55e' : '#ef4444',
              }}>
                {s.enabled ? 'Active' : 'Paused'}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
