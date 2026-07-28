import type { ClipFull } from '../../api';
import { Row } from './Row';

function thaiDateTime(s: string | null): string | null {
  if (!s) return null;
  const d = new Date(s);
  if (Number.isNaN(d.getTime())) return s;
  return d.toLocaleString('th-TH', { dateStyle: 'medium', timeStyle: 'short' });
}

export function OverviewTab({ clip }: { clip: ClipFull }) {
  return (
    <div>
      <Row label="คำถาม" value={clip.question} />
      <Row label="ผู้ถาม" value={clip.questioner_name} />
      <Row label="หมวดหมู่" value={clip.category} />
      <Row label="รูปแบบเนื้อหา" value={clip.content_format} />
      <Row label="ชุดสไตล์" value={clip.style_preset} />
      <Row label="ขั้นการผลิต" value={clip.production_stage} />
      <Row label="เลขคดี" value={clip.case_number} />
      <Row label="หัวข้อสอน" value={clip.tutorial_feature} />
      <Row label="retry" value={clip.retry_count > 0 ? `${clip.retry_count}/2` : null} />
      <Row label="รีวิวซ้ำ" value={clip.review_retry_count > 0 ? clip.review_retry_count : null} />
      <Row label="สาเหตุที่ล้มเหลว" value={clip.fail_reason} pre />
      <Row label="สร้างเมื่อ" value={thaiDateTime(clip.created_at)} />
      <Row label="แก้ไขล่าสุด" value={thaiDateTime(clip.updated_at)} />
      <Row label="กำหนดเผยแพร่" value={clip.publish_date} />
    </div>
  );
}
