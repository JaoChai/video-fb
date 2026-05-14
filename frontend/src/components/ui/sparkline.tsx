import { useMemo } from 'react'
import { cn } from '../../lib/utils'

interface SparklineProps {
  data: number[]
  className?: string
  strokeClass?: string
  fillClass?: string
  height?: number
}

export function Sparkline({
  data,
  className,
  strokeClass = 'stroke-primary',
  fillClass = 'fill-primary/10',
  height = 32,
}: SparklineProps) {
  const { path, area } = useMemo(() => {
    if (data.length === 0) return { path: '', area: '' }
    const w = 100
    const h = height
    const max = Math.max(...data, 1)
    const min = Math.min(...data, 0)
    const range = max - min || 1
    const stepX = w / Math.max(data.length - 1, 1)
    const pts = data.map((v, i) => {
      const x = i * stepX
      const y = h - ((v - min) / range) * h
      return [x, y] as const
    })
    const path = pts.map(([x, y], i) => `${i === 0 ? 'M' : 'L'}${x.toFixed(2)},${y.toFixed(2)}`).join(' ')
    const area = `${path} L${w},${h} L0,${h} Z`
    return { path, area }
  }, [data, height])

  if (data.length === 0) {
    return <div className={cn('h-8 w-full bg-muted/30 rounded', className)} />
  }

  return (
    <svg
      viewBox={`0 0 100 ${height}`}
      preserveAspectRatio="none"
      className={cn('w-full', className)}
      style={{ height }}
      aria-hidden="true"
    >
      <path d={area} className={fillClass} />
      <path d={path} className={strokeClass} fill="none" strokeWidth={1.5} vectorEffect="non-scaling-stroke" />
    </svg>
  )
}
