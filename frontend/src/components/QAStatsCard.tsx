import { useQuery } from '@tanstack/react-query';
import { apiFetch } from '../api';
import { Card } from './ui/card';
import { ShieldCheck } from 'lucide-react';

interface QAStats {
  total: number;
  passed: number;
  blocked: number;
}

// QAStatsCard shows the all-time Visual QA tally. Hidden until at least one QA
// run exists (the gate may be off, or no clip has been produced under it yet).
export function QAStatsCard() {
  const { data } = useQuery({
    queryKey: ['visual-qa-stats'],
    queryFn: () => apiFetch<QAStats>('/api/v1/visual-qa/stats'),
  });

  if (!data || data.total === 0) return null;

  const blockRate = Math.round((data.blocked / data.total) * 100);

  const stats = [
    { label: 'ตรวจทั้งหมด', value: data.total, className: 'text-foreground' },
    { label: 'ผ่าน', value: data.passed, className: 'text-emerald-600' },
    { label: 'ตีกลับ', value: data.blocked, className: 'text-amber-600' },
    { label: 'อัตราตีกลับ', value: `${blockRate}%`, className: 'text-foreground' },
  ];

  return (
    <Card className="mb-4 p-4">
      <div className="flex items-center gap-2 mb-3">
        <ShieldCheck className="size-4 text-muted-foreground" />
        <h3 className="text-sm font-semibold">Visual QA</h3>
        <span className="text-xs text-muted-foreground">ด่านตรวจคุณภาพก่อน publish</span>
      </div>
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
        {stats.map(s => (
          <div key={s.label}>
            <div className={`text-xl font-semibold tabular-nums ${s.className}`}>{s.value}</div>
            <div className="text-xs text-muted-foreground mt-0.5">{s.label}</div>
          </div>
        ))}
      </div>
    </Card>
  );
}
