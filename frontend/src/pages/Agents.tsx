import { useEffect } from 'react';
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
import { useToast } from '../components/ui/toaster';
import { Skeleton } from '../components/ui/skeleton';
import { useEditableList } from '../hooks/useEditableList';

interface Agent {
  id: string;
  agent_name: string;
  system_prompt: string;
  prompt_template: string;
  model: string;
  temperature: number;
  enabled: boolean;
  skills: string;
}

const TEMPLATE_VARS: Record<string, string[]> = {
  question: ['Count', 'Category', 'RAGContext', 'PreviousTopics', 'PreviousNames'],
  script: ['Question', 'QuestionerName', 'Category', 'RAGContext'],
  image: ['ThemeDescription', 'QuestionerName', 'QuestionText', 'PrimaryColor', 'AccentColor'],
};

function getTemplateVars(agentName: string): string[] {
  return TEMPLATE_VARS[agentName] ?? [];
}

export default function AgentsPage() {
  const qc = useQueryClient();
  const { success, error: showError } = useToast();
  const { data: agents, isLoading } = useQuery({
    queryKey: ['agents'],
    queryFn: () => apiFetch<Agent[]>('/api/v1/agents'),
  });

  const { edits, setEdits, handleEdit, toggleExpand, isDirty, isExpanded, getEdit, resetDirty } = useEditableList<Agent & Record<string, unknown>>();

  useEffect(() => {
    if (agents) {
      const initial: Record<string, Partial<Agent>> = {};
      agents.forEach((a) => {
        initial[a.id] = {
          system_prompt: a.system_prompt,
          prompt_template: a.prompt_template ?? '',
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
          prompt_template: e.prompt_template ?? agent.prompt_template ?? '',
          model: e.model ?? agent.model,
          temperature: e.temperature ?? agent.temperature,
          enabled: e.enabled ?? agent.enabled,
          skills: e.skills ?? agent.skills ?? '',
        }),
      });
    },
    onSuccess: (_data, { id, agent }) => {
      qc.invalidateQueries({ queryKey: ['agents'] });
      resetDirty(id);
      success(`บันทึก ${agent.agent_name} แล้ว`);
    },
    onError: (e) => showError(`บันทึกล้มเหลว: ${(e as Error).message}`),
  });

  return (
    <div>
      <PageHeader title="Agents" description="Configure AI agent prompts and models" />

      {isLoading ? (
        <div className="grid gap-4">
          {[1, 2, 3, 4].map(i => (
            <div key={i} className="rounded-xl border p-4">
              <div className="flex justify-between">
                <div className="flex gap-3">
                  <Skeleton className="h-5 w-20" />
                  <Skeleton className="h-5 w-40 rounded-full" />
                </div>
                <Skeleton className="h-6 w-12 rounded-full" />
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className="grid gap-4">
          {agents?.map((agent) => {
            const e = getEdit(agent.id);

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
                          isExpanded(agent.id) ? 'rotate-180' : ''
                        }`}
                      />
                    </div>
                  </div>
                </CardHeader>

                {isExpanded(agent.id) && (
                  <CardContent className="grid gap-4">
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

                    <div className="grid gap-2">
                      <label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                        Prompt Template
                      </label>
                      <p className="text-xs text-muted-foreground">
                        ใช้ {'{{.VariableName}}'} สำหรับตัวแปร — ระบบจะแทนที่ให้อัตโนมัติตอนรัน
                      </p>
                      <Textarea
                        rows={12}
                        className="font-mono text-xs"
                        value={e.prompt_template ?? agent.prompt_template ?? ''}
                        onChange={(ev) =>
                          handleEdit(agent.id, 'prompt_template', ev.target.value)
                        }
                      />
                      <div className="flex flex-wrap gap-1.5">
                        {getTemplateVars(agent.agent_name).map((v) => (
                          <span key={v} className="px-1.5 py-0.5 rounded bg-muted text-[10px] font-mono text-muted-foreground">
                            {`{{.${v}}}`}
                          </span>
                        ))}
                      </div>
                    </div>

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

                    <div className="flex items-center gap-3">
                      {isDirty(agent.id) && (
                        <Button
                          onClick={() => update.mutate({ id: agent.id, agent })}
                          disabled={update.isPending}
                        >
                          {update.isPending ? 'Saving...' : 'Save'}
                        </Button>
                      )}
                      {update.isSuccess && !isDirty(agent.id) && (
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
