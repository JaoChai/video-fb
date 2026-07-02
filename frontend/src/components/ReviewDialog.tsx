import { useEffect, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { apiFetch, getClipCritique, getClipAutoReview } from '../api';
import { Button } from './ui/button';
import { Badge } from './ui/badge';
import { useToast } from './ui/toaster';
import { CheckCircle2, X, Loader2, AlertTriangle, ShieldCheck, VideoOff } from 'lucide-react';

interface SceneVerdict {
  scene_number: number;
  ok: boolean;
  issues: string[];
}

interface VisualQAResult {
  id: string;
  clip_id: string;
  passed: boolean;
  issues: SceneVerdict[];
  created_at: string;
}

export interface ReviewClip {
  id: string;
  title: string;
  question: string;
  category: string;
  video_9_16_url?: string | null;
}

function parseScoreFields(raw: unknown): [string, number][] {
  let obj: unknown = raw;
  if (typeof raw === 'string') {
    try { obj = JSON.parse(raw); } catch { return []; }
  }
  if (obj !== null && typeof obj === 'object') {
    return Object.entries(obj as Record<string, unknown>)
      .filter((entry): entry is [string, number] => typeof entry[1] === 'number');
  }
  return [];
}

export function ReviewDialog({ clip, onClose }: { clip: ReviewClip; onClose: () => void }) {
  const queryClient = useQueryClient();
  const { success, error: showError } = useToast();
  const [acting, setActing] = useState<'approve' | 'reject' | null>(null);
  const [videoError, setVideoError] = useState(false);

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose();
    }
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onClose]);

  const { data: qa, isLoading } = useQuery({
    queryKey: ['visual-qa', clip.id],
    queryFn: () => apiFetch<VisualQAResult | null>(`/api/v1/clips/${clip.id}/visual-qa`),
  });

  const { data: critique } = useQuery({
    queryKey: ['clip-critique', clip.id],
    queryFn: () => getClipCritique(clip.id),
  });

  const { data: autoReview } = useQuery({
    queryKey: ['clip-auto-review', clip.id],
    queryFn: () => getClipAutoReview(clip.id),
  });

  const failedScenes = qa?.issues?.filter(v => !v.ok) ?? [];

  // Shared run-and-toast wrapper for the two clip actions: both flip `acting`,
  // refresh the clip list, toast, and close on success; reset `acting` on error.
  async function runAction(
    kind: 'approve' | 'reject',
    fn: () => Promise<unknown>,
    successMsg: string,
  ): Promise<void> {
    setActing(kind);
    try {
      await fn();
      queryClient.invalidateQueries({ queryKey: ['clips'] });
      success(successMsg);
      onClose();
    } catch (e) {
      const label = kind === 'approve' ? 'อนุมัติ' : 'ตีกลับ';
      showError(`${label}ไม่สำเร็จ: ${e instanceof Error ? e.message : String(e)}`);
      setActing(null);
    }
  }

  function handleApprove(): void {
    runAction(
      'approve',
      () => apiFetch(`/api/v1/clips/${clip.id}`, {
        method: 'PATCH',
        body: JSON.stringify({ status: 'ready' }),
      }),
      'อนุมัติแล้ว — คลิปพร้อม publish',
    );
  }

  function handleReject(): void {
    if (!window.confirm(`ตีกลับและลบคลิปนี้?\n\n"${clip.title}"`)) return;
    runAction(
      'reject',
      () => apiFetch(`/api/v1/clips/${clip.id}`, { method: 'DELETE' }),
      'ตีกลับและลบคลิปแล้ว',
    );
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onClick={onClose}
    >
      <div
        className="bg-background rounded-xl shadow-xl w-full max-w-3xl max-h-[90vh] overflow-y-auto"
        onClick={e => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-start justify-between gap-4 p-4 border-b sticky top-0 bg-background">
          <div className="min-w-0">
            <h2 className="text-base font-semibold leading-snug">{clip.title}</h2>
            <p className="text-xs text-muted-foreground mt-0.5">{clip.category}</p>
          </div>
          <Button variant="ghost" size="icon" className="size-8 shrink-0" onClick={onClose} title="ปิด">
            <X className="size-4" />
          </Button>
        </div>

        <div className="p-4 grid gap-4 sm:grid-cols-[auto_1fr]">
          {/* Video preview */}
          <div className="sm:w-[220px]">
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
                  <span>วิดีโอหมดอายุแล้ว<br />(ไฟล์ชั่วคราวถูกลบ — ใช้ผลตรวจด้านขวาตัดสินได้)</span>
                ) : (
                  <span>ยังไม่มีไฟล์วิดีโอ</span>
                )}
              </div>
            )}
          </div>

          {/* QA verdicts */}
          <div className="min-w-0">
            <h3 className="text-sm font-semibold mb-2">ผลตรวจ Visual QA</h3>
            {isLoading ? (
              <div className="flex items-center gap-2 text-sm text-muted-foreground">
                <Loader2 className="size-4 animate-spin" /> กำลังโหลดผลตรวจ...
              </div>
            ) : !qa ? (
              <p className="text-sm text-muted-foreground">
                ไม่พบผลตรวจ QA ของคลิปนี้ (อาจถูกตั้งสถานะด้วยมือ)
              </p>
            ) : failedScenes.length === 0 ? (
              <div className="flex items-start gap-2 text-sm text-emerald-600">
                <ShieldCheck className="size-4 mt-0.5 shrink-0" />
                <span>ไม่พบ scene ที่มีปัญหาในผลตรวจล่าสุด</span>
              </div>
            ) : (
              <div className="space-y-2">
                <p className="text-xs text-muted-foreground">
                  AI ตรวจเจอปัญหาใน {failedScenes.length} scene — อ่านแล้วดูวิดีโอประกอบก่อนตัดสินใจ
                </p>
                {failedScenes.map(v => (
                  <div
                    key={v.scene_number}
                    className="rounded-lg border border-amber-200 bg-amber-50 p-3"
                  >
                    <div className="flex items-center gap-1.5 text-sm font-medium text-amber-800">
                      <AlertTriangle className="size-3.5" />
                      Scene {v.scene_number}
                    </div>
                    <ul className="mt-1 ml-1 space-y-0.5">
                      {v.issues.map((issue, i) => (
                        <li key={i} className="text-xs text-amber-700 leading-snug">
                          • {issue}
                        </li>
                      ))}
                    </ul>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>

        {/* Critic critique — best effort, renders only when data is available */}
        {critique != null && (
          <div className="px-4 pb-4 border-t pt-3">
            <div className="flex items-center gap-2 mb-2">
              <h3 className="text-sm font-semibold">Content Critic</h3>
              {critique.applied && (
                <Badge className="bg-blue-100 text-blue-700 border-blue-200 text-xs">Applied</Badge>
              )}
            </div>
            {(() => {
              const fields = parseScoreFields(critique.score);
              return fields.length > 0 ? (
                <div className="flex flex-wrap gap-2">
                  {fields.map(([k, v]) => (
                    <span key={k} className="text-xs bg-muted rounded px-2 py-1">
                      {k}: {v}
                    </span>
                  ))}
                </div>
              ) : null;
            })()}
          </div>
        )}

        {/* Auto-review — best effort, renders only when data is available */}
        {autoReview != null && (
          <div className="px-4 pb-4 border-t pt-3">
            <div className="flex items-center gap-2 mb-2">
              <h3 className="text-sm font-semibold">Auto-review</h3>
              <Badge className="bg-blue-100 text-blue-700 border-blue-200 text-xs">
                {autoReview.decision}
              </Badge>
            </div>
            <div className="flex flex-wrap gap-2 mb-2">
              <span className="text-xs bg-muted rounded px-2 py-1">
                confidence: {autoReview.confidence}
              </span>
              {autoReview.defect_type && (
                <span className="text-xs bg-muted rounded px-2 py-1">
                  defect: {autoReview.defect_type}
                </span>
              )}
            </div>
            {autoReview.reasons?.length > 0 && (
              <ul className="ml-1 space-y-0.5">
                {autoReview.reasons.map((reason, i) => (
                  <li key={i} className="text-xs text-muted-foreground leading-snug">
                    • {reason}
                  </li>
                ))}
              </ul>
            )}
          </div>
        )}

        {/* Actions */}
        <div className="flex items-center justify-end gap-2 p-4 border-t sticky bottom-0 bg-background">
          <Button
            variant="destructive"
            onClick={handleReject}
            disabled={acting !== null}
          >
            {acting === 'reject' ? <Loader2 className="size-4 animate-spin" /> : <X className="size-4" />}
            ตีกลับ (ลบ)
          </Button>
          <Button onClick={handleApprove} disabled={acting !== null}>
            {acting === 'approve' ? <Loader2 className="size-4 animate-spin" /> : <CheckCircle2 className="size-4" />}
            อนุมัติ — พร้อม publish
          </Button>
        </div>
      </div>
    </div>
  );
}
