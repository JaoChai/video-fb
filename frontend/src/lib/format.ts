export function formatNum(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return n.toLocaleString()
}

// วันที่จาก API มาสองแบบ: timestamp เต็ม (created_at) กับวันที่ล้วน (publish_date)
// ทั้งคู่ต้องอ่านเป็นไทยเหมือนกัน ไม่งั้นหน้าเดียวกันมีทั้ง "27 ก.ค. 2569" และ "2026-07-27"
export function thaiDate(s: string | null | undefined): string | null {
  if (!s) return null
  const d = new Date(s)
  if (Number.isNaN(d.getTime())) return s
  return d.toLocaleDateString('th-TH', { dateStyle: 'medium' })
}

export function thaiDateTime(s: string | null | undefined): string | null {
  if (!s) return null
  const d = new Date(s)
  if (Number.isNaN(d.getTime())) return s
  return d.toLocaleString('th-TH', { dateStyle: 'medium', timeStyle: 'short' })
}

export function formatWatch(s: number): string {
  const h = Math.floor(s / 3600)
  const m = Math.round((s % 3600) / 60)
  return h > 0 ? `${h} ชม. ${m} นาที` : `${m} นาที`
}
