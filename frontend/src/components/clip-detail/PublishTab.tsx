import type { ClipFull, ClipMetadata } from '../../api';
import { ExternalLink } from 'lucide-react';

function Row({ label, value }: { label: string; value: string | null | undefined }) {
  if (!value) return null;
  return (
    <div className="flex gap-3 py-2 border-b last:border-0 text-sm">
      <span className="text-muted-foreground w-32 shrink-0">{label}</span>
      <span className="min-w-0 break-words whitespace-pre-wrap">{value}</span>
    </div>
  );
}

export function PublishTab({ clip, metadata }: { clip: ClipFull; metadata: ClipMetadata | null }) {
  if (!metadata) {
    return (
      <p className="text-sm text-muted-foreground">
        ยังไม่มีข้อมูลการเผยแพร่ — คลิปนี้ยังไม่ถูก publish
      </p>
    );
  }

  const tags = metadata.youtube_tags ?? [];

  return (
    <div>
      {metadata.youtube_video_id && (
        <a
          href={`https://www.youtube.com/watch?v=${metadata.youtube_video_id}`}
          target="_blank"
          rel="noreferrer"
          className="inline-flex items-center gap-1.5 text-sm text-primary hover:underline mb-3"
        >
          <ExternalLink className="size-3.5" /> เปิดคลิปบน YouTube
        </a>
      )}

      <Row label="ชื่อบน YouTube" value={metadata.youtube_title} />
      <Row label="คำบรรยาย" value={metadata.youtube_description} />
      {tags.length > 0 && (
        <div className="flex gap-3 py-2 border-b text-sm">
          <span className="text-muted-foreground w-32 shrink-0">แท็ก</span>
          <div className="flex flex-wrap gap-1 min-w-0">
            {tags.map(t => (
              <span key={t} className="text-xs bg-muted rounded px-1.5 py-0.5">{t}</span>
            ))}
          </div>
        </div>
      )}
      <Row label="เผยแพร่เมื่อ" value={clip.publish_date} />
      <Row label="YouTube video id" value={metadata.youtube_video_id} />
      <Row label="TikTok post id" value={metadata.tiktok_post_id} />
      <Row label="Zernio post id" value={metadata.zernio_post_id} />
      <Row label="Zernio shorts id" value={metadata.zernio_shorts_post_id} />
      <Row label="Zernio TikTok id" value={metadata.zernio_tiktok_post_id} />
    </div>
  );
}
