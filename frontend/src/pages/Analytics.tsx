import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useMemo, useRef, useState } from 'react'
import {
  Eye, ThumbsUp, MessageSquare, Share2, Clock, TrendingUp, BarChart3, AlertTriangle,
  ChevronDown, ChevronUp,
} from 'lucide-react'
import { apiFetch, getPresetPerformance, type PresetScore } from '../api'
import { PageHeader } from '../components/page-header'
import { Button } from '../components/ui/button'
import { Card, CardContent } from '../components/ui/card'
import { Skeleton } from '../components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '../components/ui/tabs'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import { EmptyState } from '../components/empty-state'
import { StatCard } from '../components/analytics/stat-card'
import { SegmentCompare } from '../components/analytics/segment-compare'
import { PlatformCard } from '../components/analytics/platform-card'
import { TopClipsTable, type ClipRow } from '../components/analytics/top-clips-table'
import { MetricTooltip } from '../components/analytics/metric-tooltip'
import { formatNum, formatWatch } from '../lib/format'

interface Summary {
  total_views: number
  total_likes: number
  total_comments: number
  total_shares: number
  avg_retention_rate: number
  total_watch_time_seconds: number
  clip_count: number
}

interface Delta {
  views_pct: number
  likes_pct: number
  comments_pct: number
  shares_pct: number
  watch_time_pct: number
  retention_pp: number
}

interface TrendPoint {
  day: string
  views: number
  likes: number
  comments: number
  shares: number
  watch_time_seconds: number
  avg_retention_rate: number
}

interface SegmentTotals {
  post_type: string
  views: number
  likes: number
  comments: number
  shares: number
  watch_time_seconds: number
  avg_retention_rate: number
}

interface PlatformTotals {
  platform: string
  views: number
  likes: number
  comments: number
  shares: number
  watch_time_seconds: number
  avg_retention_rate: number
}

interface SummaryResponse {
  summary: Summary
  top_clips: ClipRow[] | null
  by_post_type: SegmentTotals[] | null
  by_platform: PlatformTotals[] | null
  trend: TrendPoint[] | null
  delta: Delta
  range_days: number
  last_fetched_at: string | null
}

type Range = '7d' | '30d' | 'all'

function StatusLine({ lastFetchedAt, clipCount }: { lastFetchedAt: string | null | undefined; clipCount: number | undefined }) {
  if (!lastFetchedAt) {
    return <div className="text-xs text-muted-foreground">ยังไม่เคยดึงข้อมูล</div>
  }
  const fetched = new Date(lastFetchedAt)
  const stale = Date.now() - fetched.getTime() > 36 * 3600 * 1000
  return (
    <div className="text-xs text-muted-foreground flex flex-wrap items-center gap-2">
      {clipCount !== undefined && <span>เผยแพร่แล้ว {clipCount} คลิป</span>}
      <span>· อัปเดตเมื่อ {fetched.toLocaleString('th-TH')}</span>
      {stale && (
        <span className="inline-flex items-center gap-1 text-amber-600">
          <AlertTriangle className="size-3.5" aria-hidden />
          ข้อมูลเก่ากว่า 36 ชม.
        </span>
      )}
    </div>
  )
}

export default function AnalyticsPage() {
  const [range, setRange] = useState<Range>('30d')
  const [presetOpen, setPresetOpen] = useState(false)
  const queryClient = useQueryClient()

  const { data, isLoading } = useQuery({
    queryKey: ['analytics-summary', range],
    queryFn: () => apiFetch<SummaryResponse>(`/api/v1/analytics/summary?range=${range}`),
  })

  const { data: presetPerf } = useQuery({
    queryKey: ['preset-performance'],
    queryFn: getPresetPerformance,
  })

  const refreshTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const triggerFetch = useMutation({
    mutationFn: () => apiFetch('/api/v1/analytics/fetch', { method: 'POST' }),
    onSuccess: () => {
      if (refreshTimerRef.current) clearTimeout(refreshTimerRef.current)
      refreshTimerRef.current = setTimeout(() => {
        queryClient.invalidateQueries({ queryKey: ['analytics-summary'] })
      }, 15000)
    },
  })

  const summary = data?.summary
  const trend = data?.trend ?? []
  const delta = data?.delta

  const trendSeries = useMemo(() => ({
    views: trend.map(t => t.views),
    retention: trend.map(t => t.avg_retention_rate * 100),
  }), [trend])

  const sortedPresets: PresetScore[] = useMemo(
    () => [...(presetPerf ?? [])].sort((a, b) => b.avg_retention - a.avg_retention),
    [presetPerf],
  )

  const platforms = data?.by_platform ?? []

  return (
    <div>
      <PageHeader title="ภาพรวมสถิติ" />

      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <StatusLine lastFetchedAt={data?.last_fetched_at} clipCount={summary?.clip_count} />
        <div className="flex items-center gap-2">
          <Tabs value={range} onValueChange={v => setRange(v as Range)}>
            <TabsList>
              <TabsTrigger value="7d">7 วัน</TabsTrigger>
              <TabsTrigger value="30d">30 วัน</TabsTrigger>
              <TabsTrigger value="all">ทั้งหมด</TabsTrigger>
            </TabsList>
          </Tabs>
          <Button size="sm" variant="outline" disabled={triggerFetch.isPending} onClick={() => triggerFetch.mutate()}>
            {triggerFetch.isPending ? 'กำลังดึง…' : 'อัปเดตล่าสุด'}
          </Button>
        </div>
      </div>

      {isLoading ? (
        <div className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            {[1, 2].map(i => <Skeleton key={i} className="h-40" />)}
          </div>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
            {[1, 2, 3, 4].map(i => <Skeleton key={i} className="h-20" />)}
          </div>
        </div>
      ) : !summary || summary.clip_count === 0 ? (
        <EmptyState
          icon={BarChart3}
          title="ยังไม่มีข้อมูลสถิติ"
          description="เผยแพร่คลิปขึ้น YouTube หรือ TikTok ก่อน แล้วสถิติจะแสดงที่นี่"
        />
      ) : (
        <div className="space-y-6">
          {/* คนดูทั้งหมด + ดูจบเฉลี่ย */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            <StatCard
              variant="hero"
              label="คนดูทั้งหมด"
              tooltip="จำนวนครั้งที่คลิปถูกเปิดดู รวมทุกแพลตฟอร์ม"
              value={formatNum(summary.total_views)}
              icon={Eye}
              delta={delta?.views_pct}
              trend={trendSeries.views}
            />
            <StatCard
              variant="hero"
              label="ดูจบเฉลี่ย"
              tooltip="โดยเฉลี่ยคนดูคลิปจนจบกี่เปอร์เซ็นต์ — ยิ่งสูงยิ่งดี"
              value={`${(summary.avg_retention_rate * 100).toFixed(1)}%`}
              icon={TrendingUp}
              delta={delta?.retention_pp}
              deltaUnit="pp"
              trend={trendSeries.retention}
            />
          </div>

          {/* แยกแพลตฟอร์ม */}
          {platforms.length > 0 && (
            <div>
              <div className="mb-2 flex items-center gap-1.5">
                <h2 className="text-sm font-semibold">แยกตามแพลตฟอร์ม</h2>
                <MetricTooltip text="ยอดวิวและการมีส่วนร่วมแยกตามแต่ละแพลตฟอร์มที่เผยแพร่" />
              </div>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                {platforms.map(p => <PlatformCard key={p.platform} data={p} />)}
              </div>
            </div>
          )}

          {/* การมีส่วนร่วม */}
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
            <StatCard label="ไลก์" tooltip="จำนวนคนกดถูกใจ" value={formatNum(summary.total_likes)} icon={ThumbsUp} delta={delta?.likes_pct} />
            <StatCard label="คอมเมนต์" tooltip="จำนวนความคิดเห็น" value={formatNum(summary.total_comments)} icon={MessageSquare} delta={delta?.comments_pct} />
            <StatCard label="แชร์" tooltip="จำนวนครั้งที่ถูกแชร์ต่อ" value={formatNum(summary.total_shares)} icon={Share2} delta={delta?.shares_pct} />
            <StatCard label="เวลาดูรวม" tooltip="เวลาที่คนดูคลิปรวมกันทั้งหมด" value={formatWatch(summary.total_watch_time_seconds)} icon={Clock} delta={delta?.watch_time_pct} />
          </div>

          {/* คลิปยาว vs Shorts */}
          <div>
            <SegmentCompare data={data?.by_post_type ?? []} />
          </div>

          {/* คลิปที่ดีที่สุด */}
          <div>
            <div className="mb-2 flex items-center justify-between gap-2">
              <h2 className="text-sm font-semibold">คลิปที่ดีที่สุด</h2>
              <span className="text-xs text-muted-foreground">แตะเพื่อดูรายละเอียดแต่ละแพลตฟอร์ม</span>
            </div>
            <TopClipsTable clips={data?.top_clips ?? []} />
          </div>

          {/* ผลตามธีม (พับเก็บได้) */}
          <div>
            <button
              type="button"
              onClick={() => setPresetOpen(o => !o)}
              className="mb-2 flex w-full items-center justify-between gap-2 text-sm font-semibold"
            >
              <span className="inline-flex items-center gap-1.5">
                ผลตามธีมคลิป
                <MetricTooltip text="เปรียบเทียบอัตราดูจบเฉลี่ยของแต่ละธีม/สไตล์คลิป เพื่อดูว่าธีมไหนคนดูชอบกว่า" />
              </span>
              {presetOpen ? <ChevronUp className="size-4" /> : <ChevronDown className="size-4" />}
            </button>
            {presetOpen && (
              <Card>
                <CardContent className="pt-4">
                  {sortedPresets.length === 0 ? (
                    <p className="text-sm text-muted-foreground py-2">
                      ยังมีข้อมูลไม่พอ — ระบบกำลังสะสมอัตราดูจบต่อธีม
                    </p>
                  ) : (
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>ธีม</TableHead>
                          <TableHead>ดูจบ</TableHead>
                          <TableHead>จำนวนคลิป</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {sortedPresets.map(row => (
                          <TableRow key={row.preset}>
                            <TableCell className="font-medium">{row.preset}</TableCell>
                            <TableCell>{(row.avg_retention * 100).toFixed(1)}%</TableCell>
                            <TableCell>{row.n}</TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  )}
                </CardContent>
              </Card>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
