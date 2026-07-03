import { Heart, MessageCircle, Repeat2, Clock } from 'lucide-react'
import { Card, CardContent } from '../ui/card'
import { formatNum, formatWatch } from '../../lib/format'

interface PlatformTotals {
  platform: string
  views: number
  likes: number
  comments: number
  shares: number
  watch_time_seconds: number
  avg_retention_rate: number
}

const PLATFORM_LABEL: Record<string, string> = {
  youtube: 'YouTube',
  tiktok: 'TikTok',
  instagram: 'Instagram',
  facebook: 'Facebook',
}

// แถบสีบนหัวการ์ด = สีแบรนด์
const PLATFORM_ACCENT: Record<string, string> = {
  youtube: 'bg-rose-500',
  tiktok: 'bg-foreground',
  instagram: 'bg-pink-500',
  facebook: 'bg-blue-500',
}

export function PlatformCard({ data }: { data: PlatformTotals }) {
  const label = PLATFORM_LABEL[data.platform] ?? data.platform
  const accent = PLATFORM_ACCENT[data.platform] ?? 'bg-primary'
  const showRetention = data.avg_retention_rate > 0

  return (
    <Card className="overflow-hidden">
      <div className={`h-1.5 w-full ${accent}`} />
      <CardContent className="p-4">
        <div className="flex items-baseline justify-between gap-2">
          <span className="text-base font-semibold">{label}</span>
          {showRetention && (
            <span className="text-xs text-muted-foreground">
              ดูจบ {(data.avg_retention_rate * 100).toFixed(0)}%
            </span>
          )}
        </div>
        <div className="mt-1 text-2xl font-bold tabular-nums leading-none">{formatNum(data.views)}</div>
        <div className="text-xs text-muted-foreground">ยอดวิว</div>

        <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
          <span className="inline-flex items-center gap-1">
            <Heart className="size-3.5" /> <span className="tabular-nums">{formatNum(data.likes)}</span>
          </span>
          <span className="inline-flex items-center gap-1">
            <MessageCircle className="size-3.5" /> <span className="tabular-nums">{formatNum(data.comments)}</span>
          </span>
          <span className="inline-flex items-center gap-1">
            <Repeat2 className="size-3.5" /> <span className="tabular-nums">{formatNum(data.shares)}</span>
          </span>
          <span className="inline-flex items-center gap-1">
            <Clock className="size-3.5" /> <span className="tabular-nums">{formatWatch(data.watch_time_seconds)}</span>
          </span>
        </div>
      </CardContent>
    </Card>
  )
}
