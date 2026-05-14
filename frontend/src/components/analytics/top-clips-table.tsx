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
  views: 'Views',
  likes: 'Likes',
  retention_rate: 'Retention',
  watch_time_seconds: 'Watch time',
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
    const m = new Map<string, ClipPlatformDetail>()
    detail?.forEach(d => m.set(`${d.platform}-${d.post_type}`, d))
    return m
  }, [detail])

  const toggleSort = (key: SortKey) => {
    if (key === sortKey) setSortDir(sortDir === 'desc' ? 'asc' : 'desc')
    else { setSortKey(key); setSortDir('desc') }
  }

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
    return <div className="text-xs text-muted-foreground py-6 text-center">No clips yet</div>
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead className="pl-4">Title</TableHead>
          <TableHead className="hidden sm:table-cell">Category</TableHead>
          <TableHead className="text-right"><SortHeader k="views" /></TableHead>
          <TableHead className="hidden md:table-cell text-right"><SortHeader k="likes" /></TableHead>
          <TableHead className="hidden md:table-cell text-right"><SortHeader k="retention_rate" /></TableHead>
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
              onClick={() => setExpandedId(isExpanded ? null : clip.clip_id)}
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
                        {detailLoading ? (
                          <div className="flex gap-2">
                            {[1, 2].map(i => <Skeleton key={i} className="h-16 w-40" />)}
                          </div>
                        ) : platformMap.size === 0 ? (
                          <p className="text-xs text-muted-foreground">No platform data</p>
                        ) : (
                          <div className="flex flex-wrap gap-2">
                            {(['youtube', 'tiktok', 'instagram', 'facebook'] as const).flatMap(p =>
                              (['regular', 'shorts'] as const).map(t => {
                                const d = platformMap.get(`${p}-${t}`)
                                if (!d) return null
                                return (
                                  <div key={`${p}-${t}`} className="rounded-lg border bg-background p-2.5 min-w-[160px]">
                                    <div className="text-xs font-medium capitalize mb-1.5">
                                      {p} <span className="text-muted-foreground">· {t}</span>
                                    </div>
                                    <div className="grid grid-cols-2 gap-x-3 gap-y-1 text-xs">
                                      <span className="text-muted-foreground">Views</span>
                                      <span className="tabular-nums text-right">{formatNum(d.views)}</span>
                                      <span className="text-muted-foreground">Likes</span>
                                      <span className="tabular-nums text-right">{formatNum(d.likes)}</span>
                                      <span className="text-muted-foreground">Comments</span>
                                      <span className="tabular-nums text-right">{formatNum(d.comments)}</span>
                                      <span className="text-muted-foreground">Shares</span>
                                      <span className="tabular-nums text-right">{formatNum(d.shares)}</span>
                                      <span className="text-muted-foreground">Retention</span>
                                      <span className="tabular-nums text-right">{(d.retention_rate * 100).toFixed(1)}%</span>
                                      <span className="text-muted-foreground">Watch</span>
                                      <span className="tabular-nums text-right">{formatWatch(d.watch_time_seconds)}</span>
                                    </div>
                                  </div>
                                )
                              })
                            )}
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                </div>
              </TableCell>
              <TableCell className="hidden sm:table-cell text-sm text-muted-foreground">{clip.category}</TableCell>
              <TableCell className="text-right tabular-nums text-sm font-medium">{formatNum(clip.views)}</TableCell>
              <TableCell className="hidden md:table-cell text-right tabular-nums text-sm text-muted-foreground">{formatNum(clip.likes)}</TableCell>
              <TableCell className="hidden md:table-cell text-right tabular-nums text-sm text-muted-foreground">{(clip.retention_rate * 100).toFixed(1)}%</TableCell>
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
  )
}
