import { Badge } from "./ui/badge"

const STATUS_MAP: Record<string, { label: string; variant: "default" | "secondary" | "destructive" | "outline" }> = {
  published: { label: "Published", variant: "default" },
  ready: { label: "Ready", variant: "outline" },
  producing: { label: "Producing", variant: "secondary" },
  failed: { label: "Failed", variant: "destructive" },
  draft: { label: "Draft", variant: "secondary" },
}

export function StatusBadge({ status }: { status: string }) {
  const config = STATUS_MAP[status] ?? { label: status, variant: "secondary" as const }
  return <Badge variant={config.variant}>{config.label}</Badge>
}
