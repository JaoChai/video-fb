import { Card, CardContent } from '../ui/card'
import { MiniBar } from '../ui/mini-bar'
import { formatNum } from '../../lib/format'

interface PlatformTotals {
  platform: string
  views: number
  likes: number
  comments: number
  shares: number
  watch_time_seconds: number
}

interface PlatformBreakdownProps {
  data: PlatformTotals[]
}

const PLATFORM_LABEL: Record<string, string> = {
  youtube: 'YouTube',
  tiktok: 'TikTok',
  instagram: 'Instagram',
  facebook: 'Facebook',
}

const PLATFORM_COLOR: Record<string, string> = {
  youtube: 'bg-rose-500',
  tiktok: 'bg-foreground',
  instagram: 'bg-pink-500',
  facebook: 'bg-blue-500',
}

export function PlatformBreakdown({ data }: PlatformBreakdownProps) {
  if (!data || data.length === 0) {
    return (
      <Card>
        <CardContent className="p-4 text-xs text-muted-foreground">No platform data</CardContent>
      </Card>
    )
  }
  const max = Math.max(...data.map(p => p.views), 1)
  return (
    <Card>
      <CardContent className="p-4">
        <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-3">
          By Platform
        </h3>
        <div className="space-y-2.5">
          {data.map(p => (
            <div key={p.platform} className="space-y-1">
              <div className="flex items-baseline justify-between gap-3">
                <span className="text-sm font-medium">{PLATFORM_LABEL[p.platform] ?? p.platform}</span>
                <span className="text-xs tabular-nums text-muted-foreground">{formatNum(p.views)}</span>
              </div>
              <MiniBar value={p.views} max={max} barClass={PLATFORM_COLOR[p.platform] ?? 'bg-primary'} />
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  )
}
