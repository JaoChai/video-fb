import { useState, useMemo, useEffect } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { apiFetch, stopProduction, publishTikTok } from '../api';
import { Badge } from '../components/ui/badge';
import ProductionProgress from '../components/ProductionProgress';
import { PageHeader } from '../components/page-header';
import { StatusBadge } from '../components/status-badge';
import { Button } from '../components/ui/button';
import { Input } from '../components/ui/input';
import {
  Table, TableHeader, TableBody, TableRow, TableHead, TableCell,
} from '../components/ui/table';
import { Plus, RotateCcw, Send, Trash2, Loader2, Film, LayoutDashboard, CheckCircle2, Zap, AlertTriangle, Search, ChevronLeft, ChevronRight, ClipboardCheck, Square, Lock } from 'lucide-react';
import { useToast } from '../components/ui/toaster';
import { EmptyState } from '../components/empty-state';
import { Skeleton } from '../components/ui/skeleton';
import { ReviewDialog } from '../components/ReviewDialog';
import { QAStatsCard } from '../components/QAStatsCard';
import { cn } from '../lib/utils';

interface Clip {
  id: string; title: string; question: string; questioner_name: string;
  category: string; status: string; created_at: string;
  fail_reason?: string; retry_count: number;
  video_9_16_url?: string | null;
  style_preset: string;
  content_format: string;
  auto_review_held?: boolean;
}

type StatusFilter = 'all' | 'published' | 'ready' | 'failed' | 'producing' | 'needs_review';

const ITEMS_PER_PAGE = 10;

const FILTER_TABS: { key: StatusFilter; label: string; icon: typeof LayoutDashboard }[] = [
  { key: 'all', label: 'All', icon: LayoutDashboard },
  { key: 'needs_review', label: 'ต้องรีวิว', icon: ClipboardCheck },
  { key: 'published', label: 'Published', icon: CheckCircle2 },
  { key: 'ready', label: 'Ready', icon: Zap },
  { key: 'producing', label: 'Producing', icon: Film },
  { key: 'failed', label: 'Failed', icon: AlertTriangle },
];

function pageNumbers(current: number, total: number): (number | 'ellipsis')[] {
  const pages: (number | 'ellipsis')[] = [];
  for (let p = 1; p <= total; p++) {
    if (p === 1 || p === total || Math.abs(p - current) <= 1) {
      pages.push(p);
    } else if (pages[pages.length - 1] !== 'ellipsis') {
      pages.push('ellipsis');
    }
  }
  return pages;
}

function relativeTime(dateStr: string): string {
  const now = Date.now();
  const then = new Date(dateStr).getTime();
  const diffSec = Math.floor((now - then) / 1000);
  if (diffSec < 60) return 'เมื่อกี้';
  const diffMin = Math.floor(diffSec / 60);
  if (diffMin < 60) return `${diffMin} นาทีที่แล้ว`;
  const diffHr = Math.floor(diffMin / 60);
  if (diffHr < 24) return `${diffHr} ชม.ที่แล้ว`;
  const diffDay = Math.floor(diffHr / 24);
  if (diffDay < 30) return `${diffDay} วันที่แล้ว`;
  return new Date(dateStr).toLocaleDateString('th-TH', { day: 'numeric', month: 'short' });
}

export default function ContentPage() {
  const queryClient = useQueryClient();
  const { success, error: showError } = useToast();
  const [retrying, setRetrying] = useState(false);
  const [producing, setProducing] = useState(false);
  const [publishing, setPublishing] = useState(false);
  const [stopping, setStopping] = useState(false);
  const [publishingTikTok, setPublishingTikTok] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all');
  const [searchQuery, setSearchQuery] = useState('');
  const [currentPage, setCurrentPage] = useState(1);
  const [reviewClip, setReviewClip] = useState<Clip | null>(null);

  const { data: prodStatus } = useQuery({
    queryKey: ['production-status'],
    queryFn: () => apiFetch<{ active: boolean }>('/api/v1/production/status'),
    refetchInterval: 5000,
  });

  const isProducing = prodStatus?.active ?? false;

  const { data: clips, isLoading } = useQuery({
    queryKey: ['clips'],
    queryFn: () => apiFetch<Clip[]>('/api/v1/clips'),
    refetchInterval: (isProducing || publishing || publishingTikTok) ? 5000 : false,
  });

  const statusCounts = useMemo(() => {
    if (!clips) return { all: 0, published: 0, ready: 0, failed: 0, producing: 0, needs_review: 0, retryable: 0, held: 0, publishable: 0 };
    const counts = { all: clips.length, published: 0, ready: 0, failed: 0, producing: 0, needs_review: 0, retryable: 0, held: 0, publishable: 0 };
    for (const c of clips) {
      if (c.status === 'published') counts.published++;
      else if (c.status === 'ready') {
        counts.ready++;
        // A held clip stays 'ready' but the publisher skips it, so only count
        // non-held ready clips as actually publishable — otherwise "Publish Ready (N)"
        // promises to publish clips that silently won't move.
        if (c.auto_review_held) counts.held++;
        else counts.publishable++;
      } else if (c.status === 'failed') {
        counts.failed++;
        if (c.retry_count < 2) counts.retryable++;
      } else if (c.status === 'producing') counts.producing++;
      else if (c.status === 'needs_review') counts.needs_review++;
    }
    return counts;
  }, [clips]);

  const filtered = useMemo(() => {
    if (!clips) return [];
    let result = clips;
    if (statusFilter !== 'all') {
      result = result.filter(c => c.status === statusFilter);
    }
    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase();
      result = result.filter(c =>
        c.title.toLowerCase().includes(q) || c.category.toLowerCase().includes(q)
      );
    }
    return result;
  }, [clips, statusFilter, searchQuery]);

  const totalPages = Math.max(1, Math.ceil(filtered.length / ITEMS_PER_PAGE));
  const paged = filtered.slice((currentPage - 1) * ITEMS_PER_PAGE, currentPage * ITEMS_PER_PAGE);

  useEffect(() => {
    setCurrentPage(p => Math.min(p, totalPages));
  }, [totalPages]);

  function changeFilter(f: StatusFilter) {
    setStatusFilter(f);
    setCurrentPage(1);
  }

  function changeSearch(q: string) {
    setSearchQuery(q);
    setCurrentPage(1);
  }

  async function handleRetryAll(): Promise<void> {
    setRetrying(true);
    try {
      await apiFetch('/api/v1/orchestrator/retry', { method: 'POST' });
      queryClient.invalidateQueries({ queryKey: ['production-status'] });
      success('เริ่ม retry คลิปที่ล้มเหลวแล้ว');
    } catch (e) {
      showError(`Retry ล้มเหลว: ${e instanceof Error ? e.message : String(e)}`);
    } finally {
      setRetrying(false);
    }
  }

  async function handlePublish(): Promise<void> {
    setPublishing(true);
    try {
      await apiFetch('/api/v1/orchestrator/publish', { method: 'POST' });
      queryClient.invalidateQueries({ queryKey: ['clips'] });
      success('เริ่ม publish คลิปแล้ว');
    } catch (e) {
      showError(`Publish ล้มเหลว: ${e instanceof Error ? e.message : String(e)}`);
      setPublishing(false);
      return;
    }
    setTimeout(() => setPublishing(false), 30000);
  }

  async function handleDelete(clip: Clip): Promise<void> {
    const ok = window.confirm(`ลบคลิปนี้?\n\n"${clip.title}"`);
    if (!ok) return;
    setDeletingId(clip.id);
    try {
      await apiFetch(`/api/v1/clips/${clip.id}`, { method: 'DELETE' });
      queryClient.invalidateQueries({ queryKey: ['clips'] });
      success('ลบคลิปแล้ว');
    } catch (e) {
      showError(`ลบไม่สำเร็จ: ${e instanceof Error ? e.message : String(e)}`);
    } finally {
      setDeletingId(null);
    }
  }

  async function handleProduce(): Promise<void> {
    setProducing(true);
    try {
      await apiFetch('/api/v1/orchestrator/produce', {
        method: 'POST',
        body: JSON.stringify({ count: 1 }),
      });
      queryClient.invalidateQueries({ queryKey: ['production-status'] });
      queryClient.invalidateQueries({ queryKey: ['clips'] });
      success('เริ่มผลิตคลิปแล้ว');
    } catch (e) {
      showError(`ผลิตคลิปล้มเหลว: ${e instanceof Error ? e.message : String(e)}`);
    } finally {
      setProducing(false);
    }
  }

  async function handleStop(): Promise<void> {
    setStopping(true);
    try {
      await stopProduction();
      queryClient.invalidateQueries({ queryKey: ['production-status'] });
      queryClient.invalidateQueries({ queryKey: ['clips'] });
      success('หยุดการผลิตแล้ว');
    } catch (e) {
      showError(`หยุดการผลิตล้มเหลว: ${e instanceof Error ? e.message : String(e)}`);
    } finally {
      setStopping(false);
    }
  }

  async function handlePublishTikTok(): Promise<void> {
    setPublishingTikTok(true);
    try {
      await publishTikTok();
      queryClient.invalidateQueries({ queryKey: ['clips'] });
      success('เริ่ม publish TikTok แล้ว');
    } catch (e) {
      showError(`Publish TikTok ล้มเหลว: ${e instanceof Error ? e.message : String(e)}`);
      setPublishingTikTok(false);
      return;
    }
    setTimeout(() => setPublishingTikTok(false), 30000);
  }

  return (
    <div>
      <PageHeader
        title="Content"
        actions={
          isProducing ? (
            <Button
              variant="destructive"
              size="sm"
              onClick={handleStop}
              disabled={stopping}
            >
              {stopping ? (
                <Loader2 className="size-4 animate-spin" />
              ) : (
                <Square className="size-4" />
              )}
              {stopping ? 'Stopping...' : 'Stop Production'}
            </Button>
          ) : (
            <>
              <Button onClick={handleProduce} disabled={producing} size="sm">
                {producing ? (
                  <Loader2 className="size-4 animate-spin" />
                ) : (
                  <Plus className="size-4" />
                )}
                {producing ? 'Producing...' : 'Produce 1 Clip'}
              </Button>
              {statusCounts.retryable > 0 && (
                <Button
                  variant="destructive"
                  size="sm"
                  onClick={handleRetryAll}
                  disabled={retrying}
                >
                  {retrying ? (
                    <Loader2 className="size-4 animate-spin" />
                  ) : (
                    <RotateCcw className="size-4" />
                  )}
                  {retrying ? 'Retrying...' : `Retry Failed (${statusCounts.retryable})`}
                </Button>
              )}
              {statusCounts.ready > 0 && (
                <>
                  {statusCounts.publishable > 0 && (
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={handlePublish}
                      disabled={publishing}
                    >
                      {publishing ? (
                        <Loader2 className="size-4 animate-spin" />
                      ) : (
                        <Send className="size-4" />
                      )}
                      {publishing ? 'Publishing...' : `Publish Ready (${statusCounts.publishable})`}
                    </Button>
                  )}
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={handlePublishTikTok}
                    disabled={publishingTikTok}
                  >
                    {publishingTikTok ? (
                      <Loader2 className="size-4 animate-spin" />
                    ) : (
                      <Send className="size-4" />
                    )}
                    {publishingTikTok ? 'Publishing TikTok...' : 'Publish TikTok'}
                  </Button>
                </>
              )}
            </>
          )
        }
      />

      <ProductionProgress />

      <QAStatsCard />

      {clips && clips.length > 0 && (
        <div className="flex flex-col gap-3 mb-4">
          <div className="flex items-center gap-1.5 overflow-x-auto pb-1">
            {FILTER_TABS.map(tab => {
              const count = statusCounts[tab.key];
              const active = statusFilter === tab.key;
              return (
                <button
                  key={tab.key}
                  onClick={() => changeFilter(tab.key)}
                  className={cn(
                    'inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium whitespace-nowrap transition-colors',
                    active
                      ? 'bg-primary text-primary-foreground'
                      : 'bg-muted/50 text-muted-foreground hover:bg-muted hover:text-foreground'
                  )}
                >
                  <tab.icon className="size-3.5" />
                  {tab.label}
                  <span className={cn(
                    'ml-0.5 text-xs tabular-nums px-1.5 py-0.5 rounded-md',
                    active ? 'bg-primary-foreground/20' : 'bg-background'
                  )}>
                    {count}
                  </span>
                </button>
              );
            })}
          </div>
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground pointer-events-none" />
            <Input
              placeholder="ค้นหาชื่อคลิปหรือหมวดหมู่..."
              value={searchQuery}
              onChange={e => changeSearch(e.target.value)}
              className="pl-9 h-9"
            />
          </div>
        </div>
      )}

      {isLoading ? (
        <div className="space-y-2">
          {[1, 2, 3, 4, 5].map(i => (
            <div key={i} className="flex items-center gap-4 py-3 px-4">
              <Skeleton className="h-4 flex-1" />
              <Skeleton className="h-4 w-16" />
              <Skeleton className="h-5 w-16 rounded-full" />
              <Skeleton className="h-4 w-16" />
            </div>
          ))}
        </div>
      ) : !clips?.length ? (
        <EmptyState
          icon={Film}
          title="No clips yet"
          description="Scheduler will auto-produce clips at noon & midnight, or you can manually produce one now."
          action={{ label: '+ Produce 1 Clip', onClick: handleProduce }}
        />
      ) : filtered.length === 0 ? (
        <div className="text-center py-12 text-muted-foreground">
          <Search className="size-8 mx-auto mb-2 opacity-40" />
          <p className="text-sm">ไม่พบคลิปที่ตรงกับเงื่อนไข</p>
          <button
            onClick={() => { setStatusFilter('all'); setSearchQuery(''); }}
            className="text-sm text-primary hover:underline mt-1"
          >
            ล้างตัวกรอง
          </button>
        </div>
      ) : (
        <>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="pl-4">Title</TableHead>
                <TableHead className="hidden sm:table-cell">Category</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="hidden sm:table-cell">Created</TableHead>
                <TableHead className="w-[50px]" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {paged.map(clip => {
                const reviewable = clip.status === 'needs_review';
                // A held clip is 'ready' but the publisher skips it (Visual QA gate).
                // Make it clickable too so the reasons dialog explains why it won't publish.
                const held = clip.status === 'ready' && !!clip.auto_review_held;
                const clickable = reviewable || held;
                return (
                <TableRow
                  key={clip.id}
                  onClick={clickable ? () => setReviewClip(clip) : undefined}
                  className={cn(clickable && 'cursor-pointer hover:bg-muted/50')}
                >
                  <TableCell className="pl-4 py-3">
                    <div className="text-sm font-medium leading-snug line-clamp-1">{clip.title}</div>
                    {clip.status === 'failed' && clip.fail_reason && (
                      <div className="text-xs text-destructive mt-0.5 opacity-80 line-clamp-1">
                        {clip.fail_reason}
                      </div>
                    )}
                    {reviewable && (
                      <div className="text-xs text-amber-600 mt-0.5 font-medium">
                        คลิกเพื่อรีวิว →
                      </div>
                    )}
                    {held && (
                      <div className="text-xs text-amber-600 mt-0.5 font-medium">
                        ถูกกักโดย Visual QA — คลิกดูเหตุผล/override →
                      </div>
                    )}
                    <div className="sm:hidden text-xs text-muted-foreground mt-0.5">
                      {clip.category}{clip.content_format ? ` · ${clip.content_format}` : ''} · {relativeTime(clip.created_at)}
                    </div>
                  </TableCell>
                  <TableCell className="hidden sm:table-cell text-sm text-muted-foreground">
                    <div>{clip.category}</div>
                    {clip.content_format && (
                      <div className="text-xs text-muted-foreground/70 mt-0.5">{clip.content_format}</div>
                    )}
                    {clip.style_preset && (
                      <Badge variant="outline" className="mt-1 text-[10px] px-1 py-0 h-auto">
                        {clip.style_preset}
                      </Badge>
                    )}
                  </TableCell>
                  <TableCell className="py-3">
                    <div className="flex items-center gap-1.5">
                      <StatusBadge status={clip.status} />
                      {held && (
                        <Badge
                          variant="outline"
                          className="gap-1 border-transparent bg-amber-100 text-amber-700 text-[10px] px-1.5 py-0 h-auto"
                        >
                          <Lock className="size-2.5" />
                          ถูกกัก QA
                        </Badge>
                      )}
                      {clip.status === 'failed' && clip.retry_count > 0 && (
                        <span className="text-[10px] text-muted-foreground">
                          ({clip.retry_count}/2)
                        </span>
                      )}
                      {clip.status === 'producing' && (
                        <span className="size-1.5 rounded-full bg-orange-500 animate-pulse" />
                      )}
                    </div>
                  </TableCell>
                  <TableCell className="hidden sm:table-cell text-xs text-muted-foreground whitespace-nowrap">
                    {relativeTime(clip.created_at)}
                  </TableCell>
                  <TableCell className="text-right py-3 pr-3">
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={(e) => { e.stopPropagation(); handleDelete(clip); }}
                      disabled={deletingId === clip.id}
                      title="Delete clip"
                      className="size-8 text-muted-foreground hover:text-destructive"
                    >
                      {deletingId === clip.id ? (
                        <Loader2 className="size-4 animate-spin" />
                      ) : (
                        <Trash2 className="size-4" />
                      )}
                    </Button>
                  </TableCell>
                </TableRow>
                );
              })}
            </TableBody>
          </Table>

          {totalPages > 1 && (
            <div className="flex items-center justify-between pt-4 px-1">
              <span className="text-xs text-muted-foreground tabular-nums">
                {(currentPage - 1) * ITEMS_PER_PAGE + 1}–{Math.min(currentPage * ITEMS_PER_PAGE, filtered.length)} จาก {filtered.length}
              </span>
              <div className="flex items-center gap-1">
                <Button
                  variant="outline"
                  size="icon"
                  className="size-8"
                  onClick={() => setCurrentPage(p => Math.max(1, p - 1))}
                  disabled={currentPage <= 1}
                >
                  <ChevronLeft className="size-4" />
                </Button>
                {pageNumbers(currentPage, totalPages).map((item, idx) =>
                  item === 'ellipsis' ? (
                    <span key={`e${idx}`} className="px-1 text-xs text-muted-foreground">…</span>
                  ) : (
                    <Button
                      key={item}
                      variant={item === currentPage ? 'default' : 'ghost'}
                      size="icon"
                      className="size-8 text-xs"
                      onClick={() => setCurrentPage(item)}
                    >
                      {item}
                    </Button>
                  )
                )}
                <Button
                  variant="outline"
                  size="icon"
                  className="size-8"
                  onClick={() => setCurrentPage(p => Math.min(totalPages, p + 1))}
                  disabled={currentPage >= totalPages}
                >
                  <ChevronRight className="size-4" />
                </Button>
              </div>
            </div>
          )}
        </>
      )}

      {reviewClip && (
        <ReviewDialog clip={reviewClip} onClose={() => setReviewClip(null)} />
      )}
    </div>
  );
}
