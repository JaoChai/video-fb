import { Badge } from "./ui/badge"

type StatusConfig = {
  label: string
  variant: "default" | "secondary" | "destructive" | "outline"
  className?: string
}

const STATUS_MAP: Record<string, StatusConfig> = {
  published: { label: "Published", variant: "default" },
  ready: { label: "Ready", variant: "outline" },
  producing: { label: "Producing", variant: "secondary" },
  failed: { label: "Failed", variant: "destructive" },
  draft: { label: "Draft", variant: "secondary" },
  needs_review: {
    label: "ต้องรีวิว",
    variant: "outline",
    className: "border-transparent bg-amber-100 text-amber-700",
  },
}

export function StatusBadge({ status }: { status: string }) {
  const config = STATUS_MAP[status] ?? { label: status, variant: "secondary" as const }
  return <Badge variant={config.variant} className={config.className}>{config.label}</Badge>
}
