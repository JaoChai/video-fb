import type { ClipFull, ScriptDebate } from '../../api';

interface Candidate { lens?: string; answer_script?: string }
interface Score { lens?: string; hook?: number; accuracy?: number; audience_fit?: number }
interface Verdict { scores?: Score[]; winner_lens?: string; rationale?: string }

function asCandidates(raw: unknown): Candidate[] {
  return Array.isArray(raw) ? (raw as Candidate[]) : [];
}

function asVerdict(raw: unknown): Verdict | null {
  return raw !== null && typeof raw === 'object' && !Array.isArray(raw) ? (raw as Verdict) : null;
}

function Block({ title, text }: { title: string; text: string }) {
  if (!text) return null;
  return (
    <div className="mb-4">
      <h3 className="text-sm font-semibold mb-1.5">{title}</h3>
      <p className="text-sm leading-relaxed whitespace-pre-wrap bg-muted/40 rounded-lg p-3">{text}</p>
    </div>
  );
}

export function ScriptTab({ clip, debate }: { clip: ClipFull; debate: ScriptDebate | null }) {
  const candidates = asCandidates(debate?.candidates);
  const verdict = asVerdict(debate?.verdict);

  return (
    <div>
      <Block title="สคริปต์คำตอบ" text={clip.answer_script} />
      <Block title="สคริปต์เสียงพากย์" text={clip.voice_script} />
      {!clip.answer_script && !clip.voice_script && (
        <p className="text-sm text-muted-foreground">คลิปนี้ยังไม่มีสคริปต์</p>
      )}

      {debate && (
        <div className="border-t pt-3 mt-2">
          <h3 className="text-sm font-semibold mb-1">การดีเบตสคริปต์</h3>
          <p className="text-xs text-muted-foreground mb-3">
            ที่มาของฉบับที่ใช้จริง: {debate.source}
            {verdict?.winner_lens ? ` · ผู้ชนะ: ${verdict.winner_lens}` : ''}
          </p>

          {verdict?.rationale && (
            <p className="text-sm leading-relaxed bg-muted/40 rounded-lg p-3 mb-3">{verdict.rationale}</p>
          )}

          {(verdict?.scores ?? []).map((s, i) => (
            <div key={i} className="text-xs text-muted-foreground mb-1">
              {s.lens}: hook {s.hook} · accuracy {s.accuracy} · audience_fit {s.audience_fit}
            </div>
          ))}

          {candidates.map((c, i) => (
            <details key={i} className="mt-2 rounded-lg border p-3">
              <summary className="text-sm font-medium cursor-pointer">
                ฉบับของ {c.lens ?? `มุมมองที่ ${i + 1}`}
              </summary>
              <p className="text-sm leading-relaxed whitespace-pre-wrap mt-2">{c.answer_script ?? ''}</p>
            </details>
          ))}
        </div>
      )}
    </div>
  );
}
