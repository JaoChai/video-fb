import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../api'
import { PageHeader } from '../components/page-header'
import { Card, CardContent } from '../components/ui/card'
import { Badge } from '../components/ui/badge'
import { Skeleton } from '../components/ui/skeleton'
import { EmptyState } from '../components/empty-state'
import { History, ChevronDown } from 'lucide-react'

interface HistoryEntry {
  id: string
  agent_name: string
  old_prompt: string
  new_prompt: string
  reason: string
  created_at: string
}

export default function PromptHistoryPage() {
  const { data: entries, isLoading } = useQuery({
    queryKey: ['prompt-history'],
    queryFn: () => apiFetch<HistoryEntry[]>('/api/v1/agents/prompt-history'),
  })

  const [expanded, setExpanded] = useState<Record<string, boolean>>({})

  if (isLoading) {
    return (
      <div>
        <PageHeader title="Prompt History" description="Auto-tune history from weekly analyzer" />
        <div className="space-y-3">
          {[1, 2, 3].map(i => (
            <div key={i} className="rounded-xl border p-4 space-y-2">
              <div className="flex gap-3">
                <Skeleton className="h-5 w-20 rounded-full" />
                <Skeleton className="h-5 w-48" />
              </div>
              <Skeleton className="h-4 w-64" />
            </div>
          ))}
        </div>
      </div>
    )
  }

  if (!entries?.length) {
    return (
      <div>
        <PageHeader title="Prompt History" description="Auto-tune history from weekly analyzer" />
        <EmptyState
          icon={History}
          title="No prompt changes yet"
          description="The weekly analyzer will auto-tune agent prompts based on YouTube performance data. Changes will appear here."
        />
      </div>
    )
  }

  return (
    <div>
      <PageHeader title="Prompt History" description="Auto-tune history from weekly analyzer" />
      <div className="space-y-3">
        {entries.map(entry => {
          const isOpen = expanded[entry.id] ?? false
          return (
            <Card key={entry.id}>
              <div
                className="flex items-center justify-between px-4 py-3 cursor-pointer hover:bg-muted/50 transition-colors"
                onClick={() => setExpanded(prev => ({ ...prev, [entry.id]: !prev[entry.id] }))}
              >
                <div className="flex items-center gap-3">
                  <Badge variant="outline" className="capitalize">{entry.agent_name}</Badge>
                  <span className="text-sm">{entry.reason}</span>
                </div>
                <div className="flex items-center gap-3">
                  <span className="text-xs text-muted-foreground">
                    {new Date(entry.created_at).toLocaleDateString('th-TH')}
                  </span>
                  <ChevronDown className={`h-4 w-4 text-muted-foreground transition-transform ${isOpen ? 'rotate-180' : ''}`} />
                </div>
              </div>
              {isOpen && (
                <CardContent className="pt-0 space-y-3">
                  <div>
                    <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-1">Before</p>
                    <pre className="text-xs bg-muted rounded-md p-3 whitespace-pre-wrap max-h-48 overflow-y-auto">
                      {entry.old_prompt}
                    </pre>
                  </div>
                  <div>
                    <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-1">After</p>
                    <pre className="text-xs bg-muted rounded-md p-3 whitespace-pre-wrap max-h-48 overflow-y-auto">
                      {entry.new_prompt}
                    </pre>
                  </div>
                </CardContent>
              )}
            </Card>
          )
        })}
      </div>
    </div>
  )
}
