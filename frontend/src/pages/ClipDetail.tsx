import { useParams, useNavigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { getClipDetail } from '../api';
import { OverviewTab } from '../components/clip-detail/OverviewTab';
import { ScriptTab } from '../components/clip-detail/ScriptTab';
import { ScenesTab } from '../components/clip-detail/ScenesTab';
import { StatusBadge } from '../components/status-badge';
import { Badge } from '../components/ui/badge';
import { Button } from '../components/ui/button';
import { Skeleton } from '../components/ui/skeleton';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../components/ui/tabs';
import { ArrowLeft, Lock, VideoOff } from 'lucide-react';

export default function ClipDetailPage() {
  const { id = '' } = useParams();
  const navigate = useNavigate();
  const [videoError, setVideoError] = useState(false);

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
    return (
      <div className="text-center py-12">
        <p className="text-sm text-muted-foreground">ไม่พบคลิปนี้ (อาจถูกลบไปแล้ว)</p>
        <Button variant="ghost" className="mt-2" onClick={() => navigate('/')}>
          <ArrowLeft className="size-4" /> กลับไปหน้ารายการ
        </Button>
      </div>
    );
  }

  const { clip } = data;
  const held = clip.status === 'ready' && clip.auto_review_held;

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
          <TabsContent value="qa" />
          <TabsContent value="stats" />
          <TabsContent value="publish" />
        </Tabs>
      </div>
    </div>
  );
}
