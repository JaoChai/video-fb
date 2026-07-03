import { ArrowDown, ArrowUp, Minus, type LucideIcon } from 'lucide-react'
import { Card, CardContent } from '../ui/card'
import { Sparkline } from '../ui/sparkline'
import { MetricTooltip } from './metric-tooltip'
import { cn } from '../../lib/utils'

interface StatCardProps {
  label: string
  value: string
  icon?: LucideIcon
  delta?: number
  deltaUnit?: '%' | 'pp'
  trend?: number[]
  variant?: 'hero' | 'small'
  tooltip?: string
}

export function StatCard({
  label,
  value,
  icon: Icon,
  delta,
  deltaUnit = '%',
  trend,
  variant = 'small',
  tooltip,
}: StatCardProps) {
  const hasDelta = typeof delta === 'number' && Number.isFinite(delta)
  const positive = hasDelta && delta! > 0.05
  const negative = hasDelta && delta! < -0.05
  const ArrowIcon = positive ? ArrowUp : negative ? ArrowDown : Minus
  const deltaColor = positive
    ? 'text-emerald-600 dark:text-emerald-400'
    : negative
    ? 'text-rose-600 dark:text-rose-400'
    : 'text-muted-foreground'
  const deltaWord = positive ? 'ดีขึ้นกว่าช่วงก่อน' : negative ? 'ลดลงจากช่วงก่อน' : 'เท่าเดิม'

  if (variant === 'hero') {
    return (
      <Card>
        <CardContent className="p-5">
          <div className="flex items-center gap-1.5 mb-2">
            {Icon && <Icon className="size-4 text-muted-foreground" />}
            <span className="text-sm text-muted-foreground">{label}</span>
            {tooltip && <MetricTooltip text={tooltip} />}
          </div>
          <div className="text-3xl font-bold tabular-nums leading-none">{value}</div>
          {hasDelta && (
            <div className={cn('flex items-center gap-1 mt-2 text-xs font-medium', deltaColor)}>
              <ArrowIcon className="size-3.5" />
              <span className="tabular-nums">
                {Math.abs(delta!).toFixed(1)}{deltaUnit}
              </span>
              <span className="text-muted-foreground font-normal">{deltaWord}</span>
            </div>
          )}
          {trend && trend.length > 0 && (
            <div className="mt-3">
              <Sparkline
                data={trend}
                strokeClass={cn(positive ? 'stroke-emerald-500' : negative ? 'stroke-rose-500' : 'stroke-primary')}
                fillClass={cn(positive ? 'fill-emerald-500/10' : negative ? 'fill-rose-500/10' : 'fill-primary/10')}
                height={56}
              />
            </div>
          )}
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardContent className="p-3">
        <div className="flex items-center gap-1 mb-1">
          {Icon && <Icon className="size-3.5 text-muted-foreground" />}
          <span className="text-xs text-muted-foreground">{label}</span>
          {tooltip && <MetricTooltip text={tooltip} />}
        </div>
        <div className="text-xl font-semibold tabular-nums leading-tight">{value}</div>
        {hasDelta && (
          <div className={cn('flex items-center gap-0.5 mt-1 text-[11px] font-medium', deltaColor)}>
            <ArrowIcon className="size-3" />
            <span className="tabular-nums">{Math.abs(delta!).toFixed(1)}{deltaUnit}</span>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
