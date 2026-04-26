import { useQuery } from '@tanstack/react-query';
import { apiFetch } from '../api';

interface Schedule { id: string; name: string; cron_expression: string; action: string; enabled: boolean; last_run_at: string | null; }

export default function SchedulesPage() {
  const { data: schedules, isLoading } = useQuery({ queryKey: ['schedules'], queryFn: () => apiFetch<Schedule[]>('/api/v1/schedules') });

  return (
    <div>
      <h1 style={{ fontSize: 24, marginBottom: 24 }}>Schedules</h1>
      {isLoading ? <p>Loading...</p> : (
        <div style={{ display: 'grid', gap: 16 }}>
          {schedules?.map(s => (
            <div key={s.id} style={{ background: '#1e293b', borderRadius: 12, padding: 20, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <div>
                <h3 style={{ fontSize: 16, marginBottom: 4 }}>{s.name}</h3>
                <p style={{ fontSize: 13, color: '#94a3b8' }}>Action: {s.action}</p>
                <p style={{ fontSize: 13, color: '#94a3b8' }}>Cron: {s.cron_expression}</p>
                {s.last_run_at && <p style={{ fontSize: 12, color: '#64748b' }}>Last run: {new Date(s.last_run_at).toLocaleString('th-TH')}</p>}
              </div>
              <span style={{ background: s.enabled ? '#059669' : '#dc2626', padding: '6px 16px', borderRadius: 12, fontSize: 13 }}>
                {s.enabled ? 'Active' : 'Paused'}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
