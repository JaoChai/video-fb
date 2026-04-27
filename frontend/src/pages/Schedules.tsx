import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiFetch } from '../api';

interface Schedule {
  id: string;
  name: string;
  cron_expression: string;
  action: string;
  enabled: boolean;
  last_run_at: string | null;
  next_run_at: string | null;
}

const ACTION_LABELS: Record<string, string> = {
  publish_daily: 'Post คลิปขึ้น YouTube',
  produce_and_publish: 'ผลิตคลิป + Post YouTube (Private)',
  analyze_and_improve: 'วิเคราะห์ + ปรับปรุง Agent',
};

const CRON_LABELS: Record<string, string> = {
  '0 12 * * *': 'ทุกวัน เที่ยงวัน (12:00 น.)',
  '0 0 * * *': 'ทุกวัน เที่ยงคืน (00:00 น.)',
  '30 23 * * *': 'ทุกวัน 23:30 น.',
  '0 3 * * 1': 'ทุกวันจันทร์ 03:00 น.',
};

export default function SchedulesPage() {
  const qc = useQueryClient();
  const { data: schedules, isLoading } = useQuery({
    queryKey: ['schedules'],
    queryFn: () => apiFetch<Schedule[]>('/api/v1/schedules'),
  });

  const toggle = useMutation({
    mutationFn: ({ id, schedule }: { id: string; schedule: Schedule }) =>
      apiFetch(`/api/v1/schedules/${id}`, {
        method: 'PATCH',
        body: JSON.stringify({
          cron_expression: schedule.cron_expression,
          action: schedule.action,
          enabled: !schedule.enabled,
        }),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['schedules'] }),
  });

  return (
    <div>
      <h1 style={{ fontSize: 20, fontWeight: 600, marginBottom: 32 }}>Schedules</h1>
      {isLoading ? (
        <p style={{ color: '#555' }}>Loading...</p>
      ) : (
        <div style={{ display: 'grid', gap: 12 }}>
          {schedules?.map((s) => (
            <div
              key={s.id}
              style={{
                background: '#111',
                borderRadius: 8,
                padding: '16px 24px',
                border: '1px solid #1a1a1a',
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                transition: 'border-color 0.15s',
              }}
              onMouseEnter={(e) => (e.currentTarget.style.borderColor = '#333')}
              onMouseLeave={(e) => (e.currentTarget.style.borderColor = '#1a1a1a')}
            >
              <div>
                <div style={{ fontSize: 15, fontWeight: 500, marginBottom: 4 }}>{s.name}</div>
                <div style={{ fontSize: 13, color: '#888', marginBottom: 6 }}>
                  {ACTION_LABELS[s.action] || s.action}
                </div>
                <div style={{ display: 'flex', gap: 16, fontSize: 12, color: '#555' }}>
                  <span style={{ fontFamily: 'monospace' }}>
                    {CRON_LABELS[s.cron_expression] || s.cron_expression}
                  </span>
                  {s.last_run_at && (
                    <span>Last: {new Date(s.last_run_at).toLocaleString('th-TH')}</span>
                  )}
                </div>
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                <span style={{ fontSize: 12, color: '#888', minWidth: 60 }}>
                  {s.enabled ? 'Active' : 'Paused'}
                </span>
                <button
                  onClick={() => toggle.mutate({ id: s.id, schedule: s })}
                  disabled={toggle.isPending}
                  style={{
                    width: 44,
                    height: 24,
                    borderRadius: 12,
                    border: 'none',
                    cursor: toggle.isPending ? 'default' : 'pointer',
                    background: s.enabled ? '#22c55e' : '#333',
                    position: 'relative',
                    transition: 'background 0.2s',
                    opacity: toggle.isPending ? 0.6 : 1,
                  }}
                >
                  <span
                    style={{
                      position: 'absolute',
                      top: 3,
                      width: 18,
                      height: 18,
                      borderRadius: '50%',
                      background: '#fff',
                      transition: 'left 0.2s',
                      left: s.enabled ? 23 : 3,
                    }}
                  />
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
