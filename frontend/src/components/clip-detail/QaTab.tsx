import type { AutoReview, ClipCritique, VisualQAResult } from '../../api';
import { Badge } from '../ui/badge';
import { AlertTriangle, ShieldCheck } from 'lucide-react';

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

export function QaTab({
  qa, critique, autoReview,
}: {
  qa: VisualQAResult | null;
  critique: ClipCritique | null;
  autoReview: AutoReview | null;
}) {
  const failedScenes = qa?.issues?.filter(v => !v.ok) ?? [];
  const scoreFields = parseScoreFields(critique?.score);

  return (
    <div className="space-y-5">
      <div>
        <h3 className="text-sm font-semibold mb-2">ผลตรวจ Visual QA</h3>
        {!qa ? (
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
              <div key={v.scene_number} className="rounded-lg border border-amber-200 bg-amber-50 p-3">
                <div className="flex items-center gap-1.5 text-sm font-medium text-amber-800">
                  <AlertTriangle className="size-3.5" />
                  Scene {v.scene_number}
                </div>
                <ul className="mt-1 ml-1 space-y-0.5">
                  {v.issues.map((issue, i) => (
                    <li key={i} className="text-xs text-amber-700 leading-snug">• {issue}</li>
                  ))}
                </ul>
              </div>
            ))}
          </div>
        )}
      </div>

      {critique && (
        <div className="border-t pt-3">
          <div className="flex items-center gap-2 mb-2">
            <h3 className="text-sm font-semibold">Content Critic</h3>
            {critique.applied && (
              <Badge className="bg-blue-100 text-blue-700 border-blue-200 text-xs">Applied</Badge>
            )}
          </div>
          {scoreFields.length > 0 && (
            <div className="flex flex-wrap gap-2">
              {scoreFields.map(([k, v]) => (
                <span key={k} className="text-xs bg-muted rounded px-2 py-1">{k}: {v}</span>
              ))}
            </div>
          )}
        </div>
      )}

      {autoReview && (
        <div className="border-t pt-3">
          <div className="flex items-center gap-2 mb-2">
            <h3 className="text-sm font-semibold">Auto-review</h3>
            <Badge className="bg-blue-100 text-blue-700 border-blue-200 text-xs">
              {autoReview.decision}
            </Badge>
          </div>
          <div className="flex flex-wrap gap-2 mb-2">
            <span className="text-xs bg-muted rounded px-2 py-1">confidence: {autoReview.confidence}</span>
            {autoReview.defect_type && (
              <span className="text-xs bg-muted rounded px-2 py-1">defect: {autoReview.defect_type}</span>
            )}
          </div>
          {autoReview.reasons?.length > 0 && (
            <ul className="ml-1 space-y-0.5">
              {autoReview.reasons.map((reason, i) => (
                <li key={i} className="text-xs text-muted-foreground leading-snug">• {reason}</li>
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  );
}
