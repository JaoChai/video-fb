import { useState, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiFetch } from '../api';
import { ChevronDown } from 'lucide-react';
import { PageHeader } from '../components/page-header';
import { Card, CardHeader, CardContent } from '../components/ui/card';
import { Switch } from '../components/ui/switch';
import { Badge } from '../components/ui/badge';
import { Button } from '../components/ui/button';
import { Input } from '../components/ui/input';
import { Textarea } from '../components/ui/textarea';

interface Agent {
  id: string;
  agent_name: string;
  system_prompt: string;
  model: string;
  temperature: number;
  enabled: boolean;
  skills: string;
}

export default function AgentsPage() {
  const qc = useQueryClient();
  const { data: agents, isLoading } = useQuery({
    queryKey: ['agents'],
    queryFn: () => apiFetch<Agent[]>('/api/v1/agents'),
  });

  const [edits, setEdits] = useState<Record<string, Partial<Agent>>>({});
  const [dirty, setDirty] = useState<Record<string, boolean>>({});
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});

  useEffect(() => {
    if (agents) {
      const initial: Record<string, Partial<Agent>> = {};
      agents.forEach((a) => {
        initial[a.id] = {
          system_prompt: a.system_prompt,
          skills: a.skills ?? '',
          temperature: a.temperature,
          enabled: a.enabled,
          model: a.model,
        };
      });
      setEdits(initial);
    }
  }, [agents]);

  const update = useMutation({
    mutationFn: ({ id, agent }: { id: string; agent: Agent }) => {
      const e = edits[id] ?? {};
      return apiFetch(`/api/v1/agents/${id}`, {
        method: 'PATCH',
        body: JSON.stringify({
          system_prompt: e.system_prompt ?? agent.system_prompt,
          model: e.model ?? agent.model,
          temperature: e.temperature ?? agent.temperature,
          enabled: e.enabled ?? agent.enabled,
          skills: e.skills ?? agent.skills ?? '',
        }),
      });
    },
    onSuccess: (_data, { id }) => {
      qc.invalidateQueries({ queryKey: ['agents'] });
      setDirty((prev) => ({ ...prev, [id]: false }));
    },
  });

  const handleEdit = (id: string, field: keyof Agent, value: string | number | boolean) => {
    setEdits((prev) => ({ ...prev, [id]: { ...prev[id], [field]: value } }));
    setDirty((prev) => ({ ...prev, [id]: true }));
  };

  const toggleExpand = (id: string) => {
    setExpanded((prev) => ({ ...prev, [id]: !prev[id] }));
  };

  return (
    <div>
      <PageHeader title="Agents" description="Configure AI agent prompts and models" />

      {isLoading ? (
        <p className="text-sm text-muted-foreground">Loading...</p>
      ) : (
        <div className="grid gap-4">
          {agents?.map((agent) => {
            const e = edits[agent.id] ?? {};
            const isExpanded = expanded[agent.id] ?? false;
            const isDirty = dirty[agent.id] ?? false;

            return (
              <Card key={agent.id}>
                <CardHeader
                  className="cursor-pointer select-none"
                  onClick={() => toggleExpand(agent.id)}
                >
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                      <span className="text-sm font-medium capitalize">
                        {agent.agent_name}
                      </span>
                      <Badge variant="outline">{e.model ?? agent.model}</Badge>
                    </div>
                    <div className="flex items-center gap-3">
                      <Switch
                        checked={e.enabled ?? agent.enabled}
                        onCheckedChange={(checked) => handleEdit(agent.id, 'enabled', checked)}
                        disabled={update.isPending}
                      />
                      <ChevronDown
                        className={`h-4 w-4 text-muted-foreground transition-transform duration-200 ${
                          isExpanded ? 'rotate-180' : ''
                        }`}
                      />
                    </div>
                  </div>
                </CardHeader>

                {isExpanded && (
                  <CardContent className="grid gap-4">
                    {/* System Prompt */}
                    <div className="grid gap-2">
                      <label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                        System Prompt
                      </label>
                      <Textarea
                        rows={8}
                        value={e.system_prompt ?? agent.system_prompt}
                        onChange={(ev) =>
                          handleEdit(agent.id, 'system_prompt', ev.target.value)
                        }
                      />
                    </div>

                    {/* Model */}
                    <div className="grid gap-2">
                      <label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                        Model
                      </label>
                      <Input
                        value={e.model ?? agent.model}
                        onChange={(ev) =>
                          handleEdit(agent.id, 'model', ev.target.value)
                        }
                      />
                    </div>

                    {/* Temperature */}
                    <div className="grid gap-2">
                      <label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                        Temperature
                      </label>
                      <Input
                        type="number"
                        step={0.1}
                        min={0}
                        max={1}
                        value={e.temperature ?? agent.temperature}
                        onChange={(ev) =>
                          handleEdit(agent.id, 'temperature', parseFloat(ev.target.value))
                        }
                      />
                    </div>

                    {/* Skills */}
                    <div className="grid gap-2">
                      <label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                        Skills
                      </label>
                      <Textarea
                        rows={3}
                        value={e.skills ?? agent.skills ?? ''}
                        onChange={(ev) =>
                          handleEdit(agent.id, 'skills', ev.target.value)
                        }
                        placeholder={
                          'Define what this agent can do, e.g.:\n- Search knowledge base for relevant answers\n- Generate questions in Thai language\n- Follow brand voice guidelines'
                        }
                      />
                    </div>

                    {/* Save */}
                    <div className="flex items-center gap-3">
                      {isDirty && (
                        <Button
                          onClick={() => update.mutate({ id: agent.id, agent })}
                          disabled={update.isPending}
                        >
                          {update.isPending ? 'Saving...' : 'Save'}
                        </Button>
                      )}
                      {update.isSuccess && !isDirty && (
                        <span className="text-xs text-green-500">Saved</span>
                      )}
                    </div>
                  </CardContent>
                )}
              </Card>
            );
          })}
        </div>
      )}
    </div>
  );
}
