import { useParams, useNavigate } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { ApiError, apiFetch, getClipDetail } from '../api';
import { OverviewTab } from '../components/clip-detail/OverviewTab';
import { ScriptTab } from '../components/clip-detail/ScriptTab';
import { ScenesTab } from '../components/clip-detail/ScenesTab';
import { QaTab } from '../components/clip-detail/QaTab';
import { StatsTab } from '../components/clip-detail/StatsTab';
import { PublishTab } from '../components/clip-detail/PublishTab';
import { StatusBadge } from '../components/status-badge';
import { Badge } from '../components/ui/badge';
import { Button } from '../components/ui/button';
import { Skeleton } from '../components/ui/skeleton';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../components/ui/tabs';
import { useToast } from '../components/ui/toaster';
import { ArrowLeft, CheckCircle2, Loader2, Lock, Trash2, VideoOff, X } from 'lucide-react';

export default function ClipDetailPage() {
  const { id = '' } = useParams();
  const navigate = useNavigate();
  const [videoError, setVideoError] = useState(false);
  const queryClient = useQueryClient();
  const { success, error: showError } = useToast();
  const [acting, setActing] = useState<'approve' | 'reject' | 'delete' | null>(null);

  const { data, isLoading, error } = useQuery({
    queryKey: ['clip-detail', id],
    queryFn: () => getClipDetail(id),
    enabled: id !== '',
  });

  if (isLoading) {
    return (
      <div className="space-y-3">
        <Skeleton className="h-6 w-32" />
        <Skeleton className="h-8 w-2/3" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }

  if (error || !data) {
    // แยก "ไม่มีคลิปนี้แล้ว" ออกจาก "ต่อ backend ไม่ได้" — ในระบบนี้การตีกลับ
    // คือการลบจริง ถ้าขึ้นว่าถูกลบทั้งที่แค่ backend กำลัง deploy ผู้ใช้จะเชื่อ
    // แล้วสั่งผลิตซ้ำโดยไม่จำเป็น
    const gone = error instanceof ApiError && error.status === 404;
    return (
      <div className="text-center py-12">
        <p className="text-sm text-muted-foreground">
          {gone ? 'ไม่พบคลิปนี้ (อาจถูกลบไปแล้ว)' : 'โหลดคลิปไม่สำเร็จ — ลองใหม่อีกครั้ง'}
        </p>
        <Button variant="ghost" className="mt-2" onClick={() => navigate('/')}>
          <ArrowLeft className="size-4" /> กลับไปหน้ารายการ
        </Button>
      </div>
    );
  }

  const { clip } = data;
  const held = clip.status === 'ready' && clip.auto_review_held;
  const reviewable = clip.status === 'needs_review';

  // แอ็กชันทั้งสามหมุนรอบเดียวกัน: ล็อกปุ่ม → ยิง API → รีเฟรชรายการคลิป →
  // toast → กลับหน้าตารางถ้าคลิปถูกลบไปแล้ว
  async function runAction(
    kind: 'approve' | 'reject' | 'delete',
    fn: () => Promise<unknown>,
    successMsg: string,
    leavePage: boolean,
  ): Promise<void> {
    setActing(kind);
    try {
      await fn();
      queryClient.invalidateQueries({ queryKey: ['clips'] });
      // คลิปที่ถูกลบไปแล้ว invalidate ไม่ได้ — query ยัง active อยู่ตอนนี้ มันจะ
      // refetch ทันทีแล้วได้ 404 แน่ๆ ก่อนจะเปลี่ยนหน้า จึงทิ้ง cache ไปเลย
      if (leavePage) {
        queryClient.removeQueries({ queryKey: ['clip-detail', id] });
      } else {
        queryClient.invalidateQueries({ queryKey: ['clip-detail', id] });
      }
      success(successMsg);
      if (leavePage) navigate('/');
    } catch (e) {
      showError(`ไม่สำเร็จ: ${e instanceof Error ? e.message : String(e)}`);
    } finally {
      setActing(null);
    }
  }

  function handleApprove(): void {
    // คลิปที่ถูกกักเป็น 'ready' อยู่แล้ว — อนุมัติคือปลดล็อกให้ publisher หยิบไปได้
    // ส่วนคลิป needs_review ต้องเลื่อนสถานะเป็น ready
    runAction(
      'approve',
      () => held
        ? apiFetch(`/api/v1/clips/${clip.id}/unhold`, { method: 'POST' })
        : apiFetch(`/api/v1/clips/${clip.id}`, {
            method: 'PATCH',
            body: JSON.stringify({ status: 'ready' }),
          }),
      held ? 'Override แล้ว — คลิปพร้อม publish รอบถัดไป' : 'อนุมัติแล้ว — คลิปพร้อม publish',
      false,
    );
  }

  function handleDelete(kind: 'reject' | 'delete'): void {
    const label = kind === 'reject' ? 'ตีกลับและลบคลิปนี้?' : 'ลบคลิปนี้?';
    if (!window.confirm(`${label}\n\n"${clip.title}"`)) return;
    runAction(
      kind,
      () => apiFetch(`/api/v1/clips/${clip.id}`, { method: 'DELETE' }),
      kind === 'reject' ? 'ตีกลับและลบคลิปแล้ว' : 'ลบคลิปแล้ว',
      true,
    );
  }

  return (
    <div>
      <div className="mb-4">
        <Button variant="ghost" size="sm" className="-ml-2 mb-2" onClick={() => navigate('/')}>
          <ArrowLeft className="size-4" /> กลับ
        </Button>
        <div className="flex items-start gap-2 flex-wrap">
          <h1 className="text-lg font-semibold leading-snug flex-1 min-w-0">{clip.title}</h1>
          <StatusBadge status={clip.status} />
          {held && (
            <Badge variant="outline" className="gap-1 border-transparent bg-amber-100 text-amber-700 text-[10px]">
              <Lock className="size-2.5" /> ถูกกัก QA
            </Badge>
          )}
          <Button
            variant="ghost"
            size="icon"
            className="size-8 text-muted-foreground hover:text-destructive"
            onClick={() => handleDelete('delete')}
            disabled={acting !== null}
            title="ลบคลิป"
          >
            {acting === 'delete' ? <Loader2 className="size-4 animate-spin" /> : <Trash2 className="size-4" />}
          </Button>
        </div>
        <p className="text-xs text-muted-foreground mt-1">
          {clip.category}
          {clip.content_format ? ` · ${clip.content_format}` : ''}
          {clip.style_preset ? ` · ${clip.style_preset}` : ''}
        </p>
      </div>

      <div className="grid gap-5 sm:grid-cols-[240px_1fr] items-start">
        <div className="sm:sticky sm:top-4">
          {clip.video_9_16_url && !videoError ? (
            <video
              src={clip.video_9_16_url}
              controls
              // ทุกแถวในตารางกดเข้ามาหน้านี้ได้แล้ว และคลิปหนึ่งตัวหนักราว 9 MB
              // — ถ้าไม่กำหนด Chrome จะดึงทั้งไฟล์ทันทีแม้ผู้ใช้เปิดมาอ่านสคริปต์เฉยๆ
              preload="metadata"
              onError={() => setVideoError(true)}
              className="w-full rounded-lg bg-black aspect-[9/16]"
            />
          ) : (
            <div className="w-full aspect-[9/16] rounded-lg bg-muted flex flex-col items-center justify-center gap-1.5 text-xs text-muted-foreground text-center p-4">
              <VideoOff className="size-5 opacity-50" />
              {clip.video_9_16_url ? (
                <span>วิดีโอหมดอายุแล้ว<br />(ไฟล์ชั่วคราวถูกลบ)</span>
              ) : (
                <span>ยังไม่มีไฟล์วิดีโอ</span>
              )}
            </div>
          )}
          {(reviewable || held) && (
            <div className="flex flex-col gap-2 mt-3">
              <Button onClick={handleApprove} disabled={acting !== null} size="sm">
                {acting === 'approve' ? <Loader2 className="size-4 animate-spin" /> : <CheckCircle2 className="size-4" />}
                {held ? 'Override — publish ทั้งที่มีตำหนิ' : 'อนุมัติ — พร้อม publish'}
              </Button>
              <Button variant="destructive" size="sm" onClick={() => handleDelete('reject')} disabled={acting !== null}>
                {acting === 'reject' ? <Loader2 className="size-4 animate-spin" /> : <X className="size-4" />}
                ตีกลับ (ลบ)
              </Button>
            </div>
          )}
        </div>

        <Tabs defaultValue="overview" className="min-w-0">
          <TabsList className="flex-wrap h-auto">
            <TabsTrigger value="overview">ภาพรวม</TabsTrigger>
            <TabsTrigger value="script">สคริปต์</TabsTrigger>
            <TabsTrigger value="scenes">ฉาก ({data.scenes.length})</TabsTrigger>
            <TabsTrigger value="qa">QA & รีวิว</TabsTrigger>
            <TabsTrigger value="stats">ตัวเลข</TabsTrigger>
            <TabsTrigger value="publish">เผยแพร่</TabsTrigger>
          </TabsList>

          <TabsContent value="overview"><OverviewTab clip={clip} /></TabsContent>
          <TabsContent value="script"><ScriptTab clip={clip} debate={data.script_debate} /></TabsContent>
          <TabsContent value="scenes"><ScenesTab scenes={data.scenes} /></TabsContent>
          <TabsContent value="qa">
            <QaTab qa={data.visual_qa} critique={data.critique} autoReview={data.auto_review} />
          </TabsContent>
          <TabsContent value="stats"><StatsTab rows={data.analytics} /></TabsContent>
          <TabsContent value="publish"><PublishTab clip={clip} metadata={data.metadata} /></TabsContent>
        </Tabs>
      </div>
    </div>
  );
}
