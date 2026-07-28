import type { ClipFull, ClipMetadata } from '../../api';
import { Row } from './Row';
import { Badge } from '../ui/badge';
import { thaiDate } from '../../lib/format';
import { CheckCircle2, ExternalLink } from 'lucide-react';

export function PublishTab({ clip, metadata }: { clip: ClipFull; metadata: ClipMetadata | null }) {
  if (!metadata) {
    return (
      <p className="text-sm text-muted-foreground">
        ยังไม่มีข้อมูลการเผยแพร่ — คลิปนี้ยังไม่ถูก publish
      </p>
    );
  }

  const tags = metadata.youtube_tags ?? [];
  // ระบบโพสต์ผ่าน Zernio ซึ่งคืน post id ของตัวเอง ไม่ใช่ video id ของ YouTube
  // (youtube_video_id ว่างทั้ง 159 แถวบน prod) จึงบอกเป็นรายชื่อแพลตฟอร์มที่ขึ้นแล้ว
  // ส่วนลิงก์ตรงจะโผล่เองถ้าวันหนึ่งระบบเก็บ video id ได้
  const postedOn = [
    metadata.zernio_shorts_post_id && 'YouTube Shorts',
    metadata.zernio_tiktok_post_id && 'TikTok',
    metadata.fb_post_id && 'Facebook',
    metadata.ig_post_id && 'Instagram',
  ].filter(Boolean) as string[];

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

      {postedOn.length > 0 && (
        <div className="flex items-center gap-1.5 flex-wrap mb-3 text-sm">
          <CheckCircle2 className="size-4 text-emerald-600" />
          <span className="text-muted-foreground">ขึ้นแล้วบน</span>
          {postedOn.map(p => (
            <Badge key={p} variant="secondary">{p}</Badge>
          ))}
        </div>
      )}

      <Row label="ชื่อบน YouTube" value={metadata.youtube_title} />
      <Row label="คำบรรยาย" value={metadata.youtube_description} pre />
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
      <Row label="เผยแพร่เมื่อ" value={thaiDate(clip.publish_date)} />
      <Row label="YouTube video id" value={metadata.youtube_video_id} />
      <Row label="TikTok post id" value={metadata.tiktok_post_id} />
      <Row label="Zernio post id" value={metadata.zernio_post_id} />
      <Row label="Zernio shorts id" value={metadata.zernio_shorts_post_id} />
      <Row label="Zernio TikTok id" value={metadata.zernio_tiktok_post_id} />
    </div>
  );
}
