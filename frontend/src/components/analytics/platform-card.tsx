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

// แถบสีด้านซ้าย = สีแบรนด์
const PLATFORM_ACCENT: Record<string, string> = {
  youtube: 'bg-rose-500',
  tiktok: 'bg-foreground',
  instagram: 'bg-pink-500',
  facebook: 'bg-blue-500',
}

function Stat({ icon: Icon, label, value }: { icon: typeof Heart; label: string; value: string }) {
  return (
    <div className="flex flex-col items-center gap-0.5">
      <span className="inline-flex items-center gap-1 text-sm font-semibold tabular-nums">
        <Icon className="size-4 text-muted-foreground" />
        {value}
      </span>
      <span className="text-[11px] text-muted-foreground">{label}</span>
    </div>
  )
}

export function PlatformCard({ data }: { data: PlatformTotals }) {
  const label = PLATFORM_LABEL[data.platform] ?? data.platform
  const accent = PLATFORM_ACCENT[data.platform] ?? 'bg-primary'
  const showRetention = data.avg_retention_rate > 0

  return (
    <Card className="overflow-hidden">
      <div className="flex items-stretch">
        <div className={`w-1.5 shrink-0 ${accent}`} aria-hidden />
        <CardContent className="flex flex-1 flex-wrap items-center justify-between gap-x-6 gap-y-3 p-4">
          {/* ชื่อแพลตฟอร์ม + ยอดวิว */}
          <div className="flex items-center gap-4">
            <span className={`inline-block size-2.5 rounded-full ${accent}`} aria-hidden />
            <div>
              <div className="text-base font-semibold leading-tight">{label}</div>
              {showRetention && (
                <div className="text-xs text-muted-foreground">ดูจบเฉลี่ย {(data.avg_retention_rate * 100).toFixed(0)}%</div>
              )}
            </div>
            <div className="ml-2 border-l pl-4">
              <div className="text-2xl font-bold tabular-nums leading-none">{formatNum(data.views)}</div>
              <div className="text-xs text-muted-foreground">ยอดวิว</div>
            </div>
          </div>

          {/* การมีส่วนร่วม */}
          <div className="flex items-center gap-5">
            <Stat icon={Heart} label="ไลก์" value={formatNum(data.likes)} />
            <Stat icon={MessageCircle} label="คอมเมนต์" value={formatNum(data.comments)} />
            <Stat icon={Repeat2} label="แชร์" value={formatNum(data.shares)} />
            <Stat icon={Clock} label="เวลาดู" value={formatWatch(data.watch_time_seconds)} />
          </div>
        </CardContent>
      </div>
    </Card>
  )
}
