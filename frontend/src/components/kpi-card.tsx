import type { LucideIcon } from "lucide-react"
import { cn } from "../lib/utils"

interface KpiCardProps {
  label: string
  value: number
  icon: LucideIcon
  className?: string
}

export function KpiCard({ label, value, icon: Icon, className }: KpiCardProps) {
  return (
    <div className={cn("rounded-xl border bg-card p-4", className)}>
      <div className="flex items-center justify-between mb-2">
        <span className="text-xs text-muted-foreground uppercase tracking-wide">{label}</span>
        <Icon className="h-4 w-4 text-muted-foreground" />
      </div>
      <div className="text-2xl font-bold tabular-nums">{value}</div>
    </div>
  )
}
