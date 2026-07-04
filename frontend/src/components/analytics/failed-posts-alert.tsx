import { AlertTriangle } from 'lucide-react'
import { Card, CardContent } from '../ui/card'

export interface PublishFailure {
  clip_id: string
  title: string
  platform: string
  post_type: string
  error_message: string
  checked_at: string
}

const PLATFORM_LABEL: Record<string, string> = {
  youtube: 'YouTube',
  tiktok: 'TikTok',
}

// แปลง error จาก Zernio เป็นภาษาคน — เคสที่รู้จักแปลไทย ที่เหลือโชว์ข้อความดิบ
function reasonThai(msg: string): string {
  if (!msg) return 'ไม่ทราบสาเหตุ'
  if (msg.toLowerCase().includes('could not download')) {
    return 'ไฟล์วิดีโอหมดอายุก่อนแพลตฟอร์มดึงไปโพสต์'
  }
  return msg
}

export function FailedPostsAlert({ failures }: { failures: PublishFailure[] }) {
  if (!failures.length) return null
  return (
    <Card className="border-amber-300 bg-amber-50 dark:bg-amber-950/20">
      <CardContent className="pt-4">
        <div className="mb-2 flex items-center gap-2 text-sm font-semibold text-amber-700 dark:text-amber-400">
          <AlertTriangle className="size-4" aria-hidden />
          โพสต์ไม่สำเร็จ {failures.length} รายการ — คลิปเหล่านี้ไม่มียอดและถูกกันออกจากข้อมูลที่ AI ใช้เรียนรู้
        </div>
        <ul className="space-y-1.5">
          {failures.map(f => (
            <li key={`${f.clip_id}-${f.platform}-${f.post_type}`} className="text-sm">
              <span className="font-medium">{f.title}</span>
              <span className="text-muted-foreground"> · {PLATFORM_LABEL[f.platform] ?? f.platform} — {reasonThai(f.error_message)}</span>
            </li>
          ))}
        </ul>
      </CardContent>
    </Card>
  )
}
