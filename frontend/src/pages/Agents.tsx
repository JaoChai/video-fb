import { useQuery } from '@tanstack/react-query';
import { apiFetch } from '../api';

interface Agent { id: string; agent_name: string; system_prompt: string; model: string; temperature: number; enabled: boolean; }

export default function AgentsPage() {
  const { data: agents, isLoading } = useQuery({ queryKey: ['agents'], queryFn: () => apiFetch<Agent[]>('/api/v1/agents') });

  return (
    <div>
      <h1 style={{ fontSize: 24, marginBottom: 24 }}>Agent Configuration</h1>
      {isLoading ? <p>Loading...</p> : (
        <div style={{ display: 'grid', gap: 16 }}>
          {agents?.map(agent => (
            <div key={agent.id} style={{ background: '#1e293b', borderRadius: 12, padding: 20 }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 12 }}>
                <h3 style={{ color: '#f5851f', fontSize: 18 }}>{agent.agent_name}</h3>
                <span style={{ background: agent.enabled ? '#059669' : '#dc2626', padding: '4px 12px', borderRadius: 12, fontSize: 12 }}>
                  {agent.enabled ? 'Active' : 'Disabled'}
                </span>
              </div>
              <p style={{ fontSize: 13, color: '#94a3b8', marginBottom: 8 }}>Model: {agent.model}</p>
              <p style={{ fontSize: 13, color: '#94a3b8', marginBottom: 8 }}>Temperature: {agent.temperature}</p>
              <p style={{ fontSize: 13, color: '#cbd5e1' }}>{agent.system_prompt}</p>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
