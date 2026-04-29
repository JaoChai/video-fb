import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiFetch } from '../api';
import { PageHeader } from '../components/page-header';
import { Card, CardHeader, CardDescription } from '../components/ui/card';
import { Switch } from '../components/ui/switch';
import { Badge } from '../components/ui/badge';
import { useToast } from '../components/ui/toaster';

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
  const { success, error: showError } = useToast();
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
    onSuccess: (_d, { schedule }) => {
      qc.invalidateQueries({ queryKey: ['schedules'] });
      success(`${schedule.name} ${schedule.enabled ? 'ปิด' : 'เปิด'}แล้ว`);
    },
    onError: (e) => showError(`บันทึกล้มเหลว: ${(e as Error).message}`),
  });

  return (
    <div>
      <PageHeader title="Schedules" />
      {isLoading ? (
        <p className="text-sm text-muted-foreground">Loading...</p>
      ) : (
        <div className="grid gap-4">
          {schedules?.map((s) => (
            <Card key={s.id}>
              <CardHeader>
                <div className="flex items-center justify-between">
                  <span className="text-sm font-medium">{s.name}</span>
                  <div className="flex items-center gap-3">
                    <Badge variant={s.enabled ? 'default' : 'secondary'}>
                      {s.enabled ? 'Active' : 'Inactive'}
                    </Badge>
                    <Switch
                      checked={s.enabled}
                      onCheckedChange={() => toggle.mutate({ id: s.id, schedule: s })}
                      disabled={toggle.isPending}
                    />
                  </div>
                </div>
                <CardDescription>
                  {ACTION_LABELS[s.action] || s.action}
                  {' · '}
                  {CRON_LABELS[s.cron_expression] || s.cron_expression}
                </CardDescription>
                {s.last_run_at && (
                  <p className="text-xs text-muted-foreground">
                    Last run: {new Date(s.last_run_at).toLocaleString('th-TH')}
                  </p>
                )}
              </CardHeader>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
