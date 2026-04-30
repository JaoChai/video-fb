import { useState, useEffect } from 'react';
import { apiFetch } from '../../api';
import { Button } from '../ui/button';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '../ui/card';
import { Badge } from '../ui/badge';
import { Input } from '../ui/input';
import { useMutationWithToast } from '../../hooks/useMutationWithToast';

interface Agent {
  id: string;
  agent_name: string;
  system_prompt: string;
  model: string;
  temperature: number;
  enabled: boolean;
  skills: string;
}

interface AgentModelsCardProps {
  agents: Agent[] | undefined;
}

export function AgentModelsCard({ agents }: AgentModelsCardProps) {
  const [models, setModels] = useState<Record<string, string>>({});
  const [dirty, setDirty] = useState<Record<string, boolean>>({});

  useEffect(() => {
    if (agents) {
      const m: Record<string, string> = {};
      agents.forEach(a => { m[a.id] = a.model; });
      setModels(m);
    }
  }, [agents]);

  const saveAgentModel = useMutationWithToast<unknown, { id: string; agent: Agent }>({
    mutationFn: ({ id, agent }) =>
      apiFetch(`/api/v1/agents/${id}`, {
        method: 'PATCH',
        body: JSON.stringify({
          system_prompt: agent.system_prompt,
          model: models[id],
          temperature: agent.temperature,
          enabled: agent.enabled,
          skills: agent.skills,
        }),
      }),
    invalidateKeys: [['agents']],
    successMsg: 'บันทึก model แล้ว',
    onSuccess: () => setDirty({}),
  });

  const handleChange = (id: string, value: string) => {
    setModels(prev => ({ ...prev, [id]: value }));
    setDirty(prev => ({ ...prev, [id]: true }));
  };

  const handleSave = () => {
    if (!agents) return;
    for (const agent of agents) {
      if (dirty[agent.id]) {
        saveAgentModel.mutate({ id: agent.id, agent });
      }
    }
  };

  const hasDirty = Object.values(dirty).some(Boolean);

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-3">
          <CardTitle className="text-base">Agent Models</CardTitle>
          <Badge variant="secondary" className="text-[10px]">Assign model per agent</Badge>
        </div>
        <CardDescription>Configure which LLM model each agent uses</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        {agents?.map(agent => (
          <div
            key={agent.id}
            className="flex items-center gap-3 rounded-lg border bg-card p-3.5"
          >
            <div className="w-[120px] shrink-0">
              <span className="text-sm font-medium">{agent.agent_name}</span>
            </div>
            <Input
              value={models[agent.id] ?? agent.model}
              onChange={e => handleChange(agent.id, e.target.value)}
              placeholder="openai/gpt-4.1"
              className="flex-1 text-[13px]"
            />
            <Badge
              variant={agent.enabled ? 'secondary' : 'destructive'}
              className={`text-[10px] shrink-0 ${
                agent.enabled
                  ? 'bg-green-500/10 text-green-500 border-0'
                  : 'bg-red-500/10 text-red-500 border-0'
              }`}
            >
              {agent.enabled ? 'ON' : 'OFF'}
            </Badge>
          </div>
        ))}

        <div className="flex items-center gap-3 pt-1">
          <Button onClick={handleSave} disabled={saveAgentModel.isPending || !hasDirty}>
            {saveAgentModel.isPending ? 'Saving...' : 'Save Models'}
          </Button>
          {saveAgentModel.isSuccess && <span className="text-xs text-green-500">Saved</span>}
        </div>
      </CardContent>
    </Card>
  );
}
