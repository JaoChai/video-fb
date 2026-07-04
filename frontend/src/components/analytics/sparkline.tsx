// เส้นแนวโน้มจิ๋ว: ยอดวิวที่เพิ่มขึ้นต่อวัน (จาก snapshot รายวัน)
export function Sparkline({ points }: { points: number[] }) {
  if (!points || points.length < 2) return null
  const w = 72
  const h = 20
  const max = Math.max(...points, 1)
  const step = w / (points.length - 1)
  const d = points
    .map((v, i) => `${i === 0 ? 'M' : 'L'}${(i * step).toFixed(1)},${(h - 1 - (v / max) * (h - 2)).toFixed(1)}`)
    .join(' ')
  return (
    <svg width={w} height={h} viewBox={`0 0 ${w} ${h}`} className="text-primary shrink-0" aria-hidden>
      <path d={d} fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" />
    </svg>
  )
}
