import { cn } from '../../lib/utils'

interface MiniBarProps {
  value: number
  max: number
  className?: string
  barClass?: string
}

export function MiniBar({ value, max, className, barClass = 'bg-primary' }: MiniBarProps) {
  const pct = max <= 0 ? 0 : Math.max(0, Math.min(100, (value / max) * 100))
  return (
    <div className={cn('h-1.5 w-full rounded-full bg-muted overflow-hidden', className)}>
      <div className={cn('h-full rounded-full transition-all duration-300', barClass)} style={{ width: `${pct}%` }} />
    </div>
  )
}
