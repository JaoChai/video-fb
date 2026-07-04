import { useMemo, useState } from 'react'
import { ArrowDown, ArrowUp, ChevronDown, ChevronUp } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../../api'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../ui/table'
import { Button } from '../ui/button'
import { Skeleton } from '../ui/skeleton'
import { MiniBar } from '../ui/mini-bar'
import { cn } from '../../lib/utils'
import { formatNum, formatWatch } from '../../lib/format'

export interface ClipRow {
  clip_id: string
  title: string
  category: string
  views: number
  likes: number
  comments: number
  shares: number
  retention_rate: number
  watch_time_seconds: number
}

interface ClipPlatformDetail {
  platform: string
  post_type: string
  views: number
  likes: number
  comments: number
  shares: number
  watch_time_seconds: number
  retention_rate: number
}

type SortKey = 'views' | 'likes' | 'retention_rate' | 'watch_time_seconds'

interface TopClipsTableProps {
  clips: ClipRow[]
}

const SORT_LABELS: Record<SortKey, string> = {
  views: 'ยอดวิว',
  likes: 'ไลก์',
  retention_rate: 'ดูจบ',
  watch_time_seconds: 'เวลาดู',
}

const PLATFORM_TH: Record<string, string> = {
  youtube: 'YouTube',
  tiktok: 'TikTok',
  instagram: 'Instagram',
  facebook: 'Facebook',
}
const POST_TYPE_TH: Record<string, string> = { regular: 'คลิปยาว', shorts: 'Shorts' }

/** รายละเอียดรายแพลตฟอร์มเมื่อกดขยาย — ใช้ร่วมกันทั้ง card และ table */
function PlatformDetail({
  detailLoading,
  platformMap,
}: {
  detailLoading: boolean
  platformMap: Map<string, ClipPlatformDetail>
}) {
  if (detailLoading) {
    return (
      <div className="flex gap-2">
        {[1, 2].map(i => <Skeleton key={i} className="h-16 w-40" />)}
      </div>
    )
  }
  if (platformMap.size === 0) {
    return <p className="text-xs text-muted-foreground">ยังไม่มีข้อมูลรายแพลตฟอร์ม</p>
  }
  return (
    <div className="flex flex-wrap gap-2">
      {(['youtube', 'tiktok', 'instagram', 'facebook'] as const).flatMap(p =>
        (['regular', 'shorts'] as const).map(t => {
          const d = platformMap.get(`${p}-${t}`)
          if (!d) return null
          return (
            <div key={`${p}-${t}`} className="rounded-lg border bg-background p-2.5 min-w-[160px]">
              <div className="text-xs font-medium mb-1.5">
                {PLATFORM_TH[p]} <span className="text-muted-foreground">· {POST_TYPE_TH[t]}</span>
              </div>
              <div className="grid grid-cols-2 gap-x-3 gap-y-1 text-xs">
                <span className="text-muted-foreground">ยอดวิว</span>
                <span className="tabular-nums text-right">{formatNum(d.views)}</span>
                <span className="text-muted-foreground">ไลก์</span>
                <span className="tabular-nums text-right">{formatNum(d.likes)}</span>
                <span className="text-muted-foreground">คอมเมนต์</span>
                <span className="tabular-nums text-right">{formatNum(d.comments)}</span>
                <span className="text-muted-foreground">แชร์</span>
                <span className="tabular-nums text-right">{formatNum(d.shares)}</span>
                <span className="text-muted-foreground">ดูจบ</span>
                <span className="tabular-nums text-right">{(d.retention_rate * 100).toFixed(0)}%</span>
                <span className="text-muted-foreground">เวลาดู</span>
                <span className="tabular-nums text-right">{formatWatch(d.watch_time_seconds)}</span>
              </div>
            </div>
          )
        }),
      )}
    </div>
  )
}

export function TopClipsTable({ clips }: TopClipsTableProps) {
  const [sortKey, setSortKey] = useState<SortKey>('views')
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('desc')
  const [expandedId, setExpandedId] = useState<string | null>(null)

  const sorted = useMemo(() => {
    const arr = [...clips]
    arr.sort((a, b) => {
      const av = a[sortKey] as number
      const bv = b[sortKey] as number
      return sortDir === 'desc' ? bv - av : av - bv
    })
    return arr
  }, [clips, sortKey, sortDir])

  const maxViews = Math.max(...sorted.map(c => c.views), 1)

  const { data: detail, isLoading: detailLoading } = useQuery({
    queryKey: ['clip-analytics', expandedId],
    queryFn: () => apiFetch<ClipPlatformDetail[]>(`/api/v1/clips/${expandedId}/analytics`),
    enabled: !!expandedId,
  })

  const platformMap = useMemo(() => {
    // detail is ordered fetched_at DESC (newest first); keep the first (latest)
    // record per platform/type so the expanded view matches the row's totals.
    const m = new Map<string, ClipPlatformDetail>()
    detail?.forEach(d => {
      const k = `${d.platform}-${d.post_type}`
      if (!m.has(k)) m.set(k, d)
    })
    return m
  }, [detail])

  const toggleSort = (key: SortKey) => {
    if (key === sortKey) setSortDir(sortDir === 'desc' ? 'asc' : 'desc')
    else { setSortKey(key); setSortDir('desc') }
  }

  const toggleExpand = (id: string) => setExpandedId(prev => (prev === id ? null : id))

  const SortHeader = ({ k, className }: { k: SortKey; className?: string }) => (
    <button
      type="button"
      onClick={() => toggleSort(k)}
      className={cn('inline-flex items-center gap-1 hover:text-foreground transition-colors', className)}
    >
      {SORT_LABELS[k]}
      {sortKey === k && (sortDir === 'desc' ? <ArrowDown className="size-3" /> : <ArrowUp className="size-3" />)}
    </button>
  )

  if (clips.length === 0) {
    return <div className="text-xs text-muted-foreground py-6 text-center">ยังไม่มีคลิป</div>
  }

  return (
    <>
      {/* มือถือ: card list */}
      <div className="space-y-2 md:hidden">
        {sorted.map((clip, idx) => {
          const isExpanded = expandedId === clip.clip_id
          return (
            <div key={clip.clip_id} className={cn('rounded-lg border p-3', isExpanded && 'bg-muted/30')}>
              <button
                type="button"
                className="flex w-full items-start gap-2 text-left"
                onClick={() => toggleExpand(clip.clip_id)}
              >
                <span className="text-xs text-muted-foreground tabular-nums w-5 pt-0.5">{idx + 1}</span>
                <div className="flex-1 min-w-0">
                  <div className="text-sm font-medium line-clamp-2">{clip.title}</div>
                  <div className="mt-1 text-xs text-muted-foreground">{clip.category}</div>
                  <div className="mt-2 flex flex-wrap gap-x-4 gap-y-0.5 text-xs tabular-nums">
                    <span><span className="text-muted-foreground">วิว </span>{formatNum(clip.views)}</span>
                    <span><span className="text-muted-foreground">ไลก์ </span>{formatNum(clip.likes)}</span>
                    <span><span className="text-muted-foreground">ดูจบ </span>{(clip.retention_rate * 100).toFixed(0)}%</span>
                  </div>
                  <div className="mt-1.5">
                    <MiniBar value={clip.views} max={maxViews} />
                  </div>
                </div>
                {isExpanded ? <ChevronUp className="size-4 shrink-0" /> : <ChevronDown className="size-4 shrink-0" />}
              </button>
              {isExpanded && (
                <div className="mt-3">
                  <PlatformDetail detailLoading={detailLoading} platformMap={platformMap} />
                </div>
              )}
            </div>
          )
        })}
      </div>

      {/* คอม: table */}
      <div className="hidden md:block">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="pl-4">ชื่อคลิป</TableHead>
              <TableHead className="hidden sm:table-cell">หมวด</TableHead>
              <TableHead className="text-right"><SortHeader k="views" /></TableHead>
              <TableHead className="text-right"><SortHeader k="likes" /></TableHead>
              <TableHead className="text-right"><SortHeader k="retention_rate" /></TableHead>
              <TableHead className="hidden lg:table-cell text-right"><SortHeader k="watch_time_seconds" /></TableHead>
              <TableHead className="w-[40px]" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {sorted.map((clip, idx) => {
              const isExpanded = expandedId === clip.clip_id
              return (
                <TableRow
                  key={clip.clip_id}
                  className={cn('cursor-pointer', isExpanded && 'bg-muted/30')}
                  onClick={() => toggleExpand(clip.clip_id)}
                >
                  <TableCell className="pl-4 py-3">
                    <div className="flex items-center gap-2">
                      <span className="text-xs text-muted-foreground tabular-nums w-5">{idx + 1}</span>
                      <div className="flex-1 min-w-0">
                        <div className="text-sm font-medium line-clamp-1">{clip.title}</div>
                        <div className="mt-1.5 max-w-[280px]">
                          <MiniBar value={clip.views} max={maxViews} />
                        </div>
                        {isExpanded && (
                          <div className="mt-3">
                            <PlatformDetail detailLoading={detailLoading} platformMap={platformMap} />
                          </div>
                        )}
                      </div>
                    </div>
                  </TableCell>
                  <TableCell className="hidden sm:table-cell text-sm text-muted-foreground">{clip.category}</TableCell>
                  <TableCell className="text-right tabular-nums text-sm font-medium">{formatNum(clip.views)}</TableCell>
                  <TableCell className="text-right tabular-nums text-sm text-muted-foreground">{formatNum(clip.likes)}</TableCell>
                  <TableCell className="text-right tabular-nums text-sm text-muted-foreground">{(clip.retention_rate * 100).toFixed(0)}%</TableCell>
                  <TableCell className="hidden lg:table-cell text-right tabular-nums text-sm text-muted-foreground">{formatWatch(clip.watch_time_seconds)}</TableCell>
                  <TableCell className="pr-3">
                    <Button variant="ghost" size="icon" className="size-7">
                      {isExpanded ? <ChevronUp className="size-4" /> : <ChevronDown className="size-4" />}
                    </Button>
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      </div>
    </>
  )
}
