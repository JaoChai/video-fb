import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { apiFetch } from '../api';
import ProductionProgress from '../components/ProductionProgress';
import { PageHeader } from '../components/page-header';
import { StatusBadge } from '../components/status-badge';
import { Button } from '../components/ui/button';
import {
  Table, TableHeader, TableBody, TableRow, TableHead, TableCell,
} from '../components/ui/table';
import { Plus, RotateCcw, Send, Trash2, Loader2, Film } from 'lucide-react';
import { useToast } from '../components/ui/toaster';
import { EmptyState } from '../components/empty-state';
import { Skeleton } from '../components/ui/skeleton';

interface Clip {
  id: string; title: string; question: string; questioner_name: string;
  category: string; status: string; created_at: string;
  fail_reason?: string; retry_count: number;
}

export default function ContentPage() {
  const queryClient = useQueryClient();
  const { success, error: showError } = useToast();
  const [retrying, setRetrying] = useState(false);
  const [producing, setProducing] = useState(false);
  const [publishing, setPublishing] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  const { data: prodStatus } = useQuery({
    queryKey: ['production-status'],
    queryFn: () => apiFetch<{ active: boolean }>('/api/v1/production/status'),
  });

  const isProducing = prodStatus?.active ?? false;

  const { data: clips, isLoading } = useQuery({
    queryKey: ['clips'],
    queryFn: () => apiFetch<Clip[]>('/api/v1/clips'),
    refetchInterval: isProducing ? 5000 : false,
  });

  const failedCount = clips?.filter(c => c.status === 'failed' && c.retry_count < 2).length ?? 0;
  const readyCount = clips?.filter(c => c.status === 'ready').length ?? 0;

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
    } finally {
      setPublishing(false);
    }
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

  return (
    <div>
      <PageHeader
        title="Content"
        actions={
          !isProducing ? (
            <>
              <Button onClick={handleProduce} disabled={producing} size="sm">
                {producing ? (
                  <Loader2 className="size-4 animate-spin" />
                ) : (
                  <Plus className="size-4" />
                )}
                {producing ? 'Producing...' : 'Produce 1 Clip'}
              </Button>
              {failedCount > 0 && (
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
                  {retrying ? 'Retrying...' : `Retry Failed (${failedCount})`}
                </Button>
              )}
              {readyCount > 0 && (
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
                  {publishing ? 'Publishing...' : `Publish Ready (${readyCount})`}
                </Button>
              )}
            </>
          ) : undefined
        }
      />

      <ProductionProgress />

      {isLoading ? (
        <div className="space-y-3">
          {[1, 2, 3].map(i => (
            <div key={i} className="flex items-center gap-4 py-4">
              <Skeleton className="h-5 flex-1" />
              <Skeleton className="h-5 w-24" />
              <Skeleton className="h-6 w-20 rounded-full" />
              <Skeleton className="h-5 w-20" />
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
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Title</TableHead>
              <TableHead>Category</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Created</TableHead>
              <TableHead className="w-[60px]" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {clips.map(clip => (
              <TableRow key={clip.id}>
                <TableCell>
                  <div className="text-sm">{clip.title}</div>
                  {clip.status === 'failed' && clip.fail_reason && (
                    <div className="text-xs text-destructive mt-1 opacity-80">
                      {clip.fail_reason}
                    </div>
                  )}
                </TableCell>
                <TableCell className="text-muted-foreground">
                  {clip.category}
                </TableCell>
                <TableCell>
                  <div className="flex items-center gap-2">
                    <StatusBadge status={clip.status} />
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
                <TableCell className="text-xs text-muted-foreground">
                  {new Date(clip.created_at).toLocaleDateString('th-TH')}
                </TableCell>
                <TableCell className="text-right">
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => handleDelete(clip)}
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
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  );
}
