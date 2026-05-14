import { Card, CardContent } from '../ui/card'
import { MiniBar } from '../ui/mini-bar'
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

const LABELS: Record<string, string> = { regular: 'Regular', shorts: 'Shorts' }

export function SegmentCompare({ data }: SegmentCompareProps) {
  const regular = data.find(d => d.post_type === 'regular')
  const shorts = data.find(d => d.post_type === 'shorts')
  const segments = [regular, shorts].filter(Boolean) as SegmentTotals[]

  if (segments.length === 0) {
    return (
      <Card>
        <CardContent className="p-4 text-xs text-muted-foreground">No segmented data</CardContent>
      </Card>
    )
  }

  const maxViews = Math.max(...segments.map(s => s.views), 1)
  const maxWatch = Math.max(...segments.map(s => s.watch_time_seconds), 1)

  return (
    <Card>
      <CardContent className="p-4">
        <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-3">
          Regular vs Shorts
        </h3>
        <div className="space-y-3">
          {segments.map(s => (
            <div key={s.post_type} className="space-y-2">
              <div className="flex items-baseline justify-between">
                <span className="text-sm font-medium">{LABELS[s.post_type] ?? s.post_type}</span>
                <span className="text-xs text-muted-foreground tabular-nums">
                  {formatNum(s.views)} views · {formatWatch(s.watch_time_seconds)} · {(s.avg_retention_rate * 100).toFixed(1)}% retention
                </span>
              </div>
              <MiniBar value={s.views} max={maxViews} />
              <MiniBar value={s.watch_time_seconds} max={maxWatch} barClass="bg-amber-500" />
            </div>
          ))}
          <div className="flex gap-3 pt-2 text-[10px] text-muted-foreground">
            <span className="flex items-center gap-1"><span className="size-2 rounded-sm bg-primary" /> Views</span>
            <span className="flex items-center gap-1"><span className="size-2 rounded-sm bg-amber-500" /> Watch time</span>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
