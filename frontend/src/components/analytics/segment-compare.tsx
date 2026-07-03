import { Card, CardContent } from '../ui/card'
import { MiniBar } from '../ui/mini-bar'
import { MetricTooltip } from './metric-tooltip'
import { formatNum, formatWatch } from '../../lib/format'

interface SegmentTotals {
  post_type: string
  views: number
  likes: number
  comments: number
  shares: number
  watch_time_seconds: number
  avg_retention_rate: number
}

interface SegmentCompareProps {
  data: SegmentTotals[]
}

const LABELS: Record<string, string> = { regular: 'คลิปยาว', shorts: 'Shorts' }

export function SegmentCompare({ data }: SegmentCompareProps) {
  const regular = data.find(d => d.post_type === 'regular')
  const shorts = data.find(d => d.post_type === 'shorts')
  const segments = [regular, shorts].filter(Boolean) as SegmentTotals[]

  if (segments.length === 0) {
    return (
      <Card>
        <CardContent className="p-4 text-xs text-muted-foreground">ยังไม่มีข้อมูลแยกประเภท</CardContent>
      </Card>
    )
  }

  const maxViews = Math.max(...segments.map(s => s.views), 1)
  const maxWatch = Math.max(...segments.map(s => s.watch_time_seconds), 1)

  return (
    <Card>
      <CardContent className="p-4">
        <div className="flex items-center gap-1.5 mb-3">
          <h3 className="text-sm font-semibold">คลิปยาว vs Shorts</h3>
          <MetricTooltip text="เทียบผลระหว่างคลิปแบบยาวกับคลิปสั้น (Shorts) — ดูว่ารูปแบบไหนคนดูเยอะ/ดูจบมากกว่า" />
        </div>
        <div className="space-y-3">
          {segments.map(s => (
            <div key={s.post_type} className="space-y-2">
              <div className="flex items-baseline justify-between gap-2">
                <span className="text-sm font-medium">{LABELS[s.post_type] ?? s.post_type}</span>
                <span className="text-xs text-muted-foreground tabular-nums">
                  {formatNum(s.views)} วิว · {formatWatch(s.watch_time_seconds)} · ดูจบ {(s.avg_retention_rate * 100).toFixed(0)}%
                </span>
              </div>
              <MiniBar value={s.views} max={maxViews} />
              <MiniBar value={s.watch_time_seconds} max={maxWatch} barClass="bg-amber-500" />
            </div>
          ))}
          <div className="flex gap-3 pt-2 text-[10px] text-muted-foreground">
            <span className="flex items-center gap-1"><span className="size-2 rounded-sm bg-primary" /> ยอดวิว</span>
            <span className="flex items-center gap-1"><span className="size-2 rounded-sm bg-amber-500" /> เวลาดู</span>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
