import { useState, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useSearchParams } from 'react-router-dom'
import { apiFetch, getSkillRevisions } from '../api'
import { PageHeader } from '../components/page-header'
import { Card, CardContent } from '../components/ui/card'
import { Badge } from '../components/ui/badge'
import { Skeleton } from '../components/ui/skeleton'
import { EmptyState } from '../components/empty-state'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../components/ui/tabs'
import { History, ChevronDown } from 'lucide-react'

interface HistoryEntry {
  id: string
  agent_name: string
  old_prompt: string
  new_prompt: string
  reason: string
  created_at: string
}

function LineDiff({ before, after }: { before: string; after: string }) {
  const beforeSet = new Set(before.split('\n').map(l => l.trim()).filter(Boolean))
  const afterLines = after.split('\n')

  return (
    <div className="text-xs font-mono bg-muted rounded-md p-3 max-h-48 overflow-y-auto">
      {afterLines.map((line, i) => {
        const trimmed = line.trim()
        const isNew = trimmed !== '' && !beforeSet.has(trimmed)
        return (
          <div
            key={i}
            className={isNew ? 'bg-green-500/20 text-green-700 dark:text-green-400 rounded px-0.5' : ''}
          >
            {line || ' '}
          </div>
        )
      })}
    </div>
  )
}

export default function PromptHistoryPage() {
  const [searchParams] = useSearchParams()
  const { data: entries, isLoading } = useQuery({
    queryKey: ['prompt-history'],
    queryFn: () => apiFetch<HistoryEntry[]>('/api/v1/agents/prompt-history'),
  })

  const { data: revisions = [] } = useQuery({
    queryKey: ['skill-revisions'],
    queryFn: getSkillRevisions,
  })

  const initialAgent = searchParams.get('agent') ?? 'all'
  const [filterAgent, setFilterAgent] = useState<string>(initialAgent)
  const [expanded, setExpanded] = useState<Record<string, boolean>>({})
  const [tab, setTab] = useState('changes')

  const agentNames = useMemo(() => {
    if (!entries) return []
    return Array.from(new Set(entries.map(e => e.agent_name))).sort()
  }, [entries])

  const filtered = useMemo(() => {
    if (!entries) return []
    return filterAgent === 'all' ? entries : entries.filter(e => e.agent_name === filterAgent)
  }, [entries, filterAgent])

  return (
    <div>
      <PageHeader title="Prompt History" description="Auto-tune history from weekly analyzer" />

      <Tabs value={tab} onValueChange={setTab}>
        <TabsList className="mb-4">
          <TabsTrigger value="changes">Prompt Changes</TabsTrigger>
          <TabsTrigger value="revisions">Skill Revisions</TabsTrigger>
        </TabsList>

        <TabsContent value="changes">
          {isLoading ? (
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
          ) : !entries?.length ? (
            <EmptyState
              icon={History}
              title="No prompt changes yet"
              description="The weekly analyzer will auto-tune agent prompts based on YouTube performance data. Changes will appear here."
            />
          ) : (
            <>
              <div className="flex items-center gap-2 mb-4">
                <span className="text-xs text-muted-foreground">Filter:</span>
                {['all', ...agentNames].map(name => (
                  <button
                    key={name}
                    type="button"
                    onClick={() => setFilterAgent(name)}
                    className={`px-2.5 py-1 rounded-full text-xs font-medium transition-colors ${
                      filterAgent === name
                        ? 'bg-primary text-primary-foreground'
                        : 'bg-muted text-muted-foreground hover:bg-muted/80'
                    }`}
                  >
                    {name === 'all' ? 'All' : name}
                  </button>
                ))}
              </div>

              {filtered.length === 0 && (
                <p className="text-sm text-muted-foreground py-8 text-center">
                  ยังไม่มีประวัติการปรับแต่งสำหรับ "{filterAgent}" —{' '}
                  <button type="button" className="underline" onClick={() => setFilterAgent('all')}>
                    ดูทั้งหมด
                  </button>
                </p>
              )}

              <div className="space-y-3">
                {filtered.map(entry => {
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
                            {new Date(entry.created_at).toLocaleString('th-TH', { dateStyle: 'short', timeStyle: 'short' })}
                          </span>
                          <ChevronDown className={`h-4 w-4 text-muted-foreground transition-transform ${isOpen ? 'rotate-180' : ''}`} />
                        </div>
                      </div>
                      {isOpen && (
                        <CardContent className="pt-0 space-y-3">
                          <div>
                            <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-1">Before</p>
                            <pre className="text-xs bg-muted rounded-md p-3 whitespace-pre-wrap max-h-48 overflow-y-auto">
                              {entry.old_prompt || '(empty)'}
                            </pre>
                          </div>
                          <div>
                            <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-1">After — highlighted lines are new</p>
                            <LineDiff before={entry.old_prompt} after={entry.new_prompt} />
                          </div>
                        </CardContent>
                      )}
                    </Card>
                  )
                })}
              </div>
            </>
          )}
        </TabsContent>

        <TabsContent value="revisions">
          {revisions.length === 0 ? (
            <p className="text-sm text-muted-foreground py-8 text-center">
              ยังไม่มีการอัพเดต skill — ระบบจะบันทึกไว้ที่นี่เมื่อ critic ปรับ skill
            </p>
          ) : (
            <div className="space-y-3">
              {revisions.map(rev => (
                <Card key={`${rev.agent_name}-${rev.created_at}`}>
                  <div className="px-4 py-3">
                    <div className="flex items-center justify-between mb-1">
                      <Badge variant="outline" className="capitalize">{rev.agent_name}</Badge>
                      <span className="text-xs text-muted-foreground">
                        {new Date(rev.created_at).toLocaleString('th-TH', { dateStyle: 'short', timeStyle: 'short' })}
                      </span>
                    </div>
                    <p className="text-sm mt-1">{rev.rationale}</p>
                    {rev.critique_window && (
                      <p className="text-xs text-muted-foreground mt-1">Window: {rev.critique_window}</p>
                    )}
                  </div>
                </Card>
              ))}
            </div>
          )}
        </TabsContent>
      </Tabs>
    </div>
  )
}
