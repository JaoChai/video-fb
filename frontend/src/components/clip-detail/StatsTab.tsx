import type { ClipAnalyticsRow } from '../../api';
import { formatNum, formatWatch, thaiDateTime } from '../../lib/format';

function Metric({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="rounded-lg bg-muted/40 px-3 py-2">
      <div className="text-[11px] text-muted-foreground">{label}</div>
      <div className="text-sm font-semibold tabular-nums">{value}</div>
    </div>
  );
}

export function StatsTab({ rows }: { rows: ClipAnalyticsRow[] }) {
  if (rows.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">
        ยังไม่มีตัวเลข — คลิปที่ยังไม่เผยแพร่ หรือเพิ่งเผยแพร่แล้วยังไม่ถึงรอบดึงข้อมูล
      </p>
    );
  }

  return (
    <div className="space-y-3">
      {rows.map(r => (
        <div key={r.id} className="rounded-lg border p-3">
          <div className="flex items-center gap-2 mb-2 text-sm">
            <span className="font-medium">{r.platform}</span>
            {r.post_type && <span className="text-xs text-muted-foreground">{r.post_type}</span>}
            <span className="text-xs text-muted-foreground ml-auto">
              ดึงเมื่อ {thaiDateTime(r.fetched_at)}
            </span>
          </div>
          <div className="grid grid-cols-3 sm:grid-cols-6 gap-2">
            <Metric label="วิว" value={formatNum(r.views)} />
            <Metric label="ไลก์" value={formatNum(r.likes)} />
            <Metric label="คอมเมนต์" value={formatNum(r.comments)} />
            <Metric label="แชร์" value={formatNum(r.shares)} />
            {/* TikTok ไม่ส่ง watch time / retention มาให้เลย (ทุกแถวเป็น 0) การแสดง
                "0 นาที · 0.0%" จะอ่านเหมือนวัดแล้วไม่มีคนดู ทั้งที่ไม่มีข้อมูลตั้งแต่ต้นทาง */}
            {r.platform !== 'tiktok' && (
              <>
                <Metric label="เวลาดูรวม" value={formatWatch(r.watch_time_seconds)} />
                <Metric label="retention" value={`${(r.retention_rate * 100).toFixed(1)}%`} />
              </>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}
