import { useQuery } from '@tanstack/react-query';
import { apiFetch } from '../api';

interface Agent { id: string; agent_name: string; system_prompt: string; model: string; temperature: number; enabled: boolean; }

export default function AgentsPage() {
  const { data: agents, isLoading } = useQuery({ queryKey: ['agents'], queryFn: () => apiFetch<Agent[]>('/api/v1/agents') });

  return (
    <div>
      <h1 style={{ fontSize: 20, fontWeight: 600, marginBottom: 32 }}>Agents</h1>
      {isLoading ? <p style={{ color: '#555' }}>Loading...</p> : (
        <div style={{ display: 'grid', gap: 12 }}>
          {agents?.map(agent => (
            <div key={agent.id} style={{
              background: '#111', borderRadius: 8, padding: '20px 24px',
              border: '1px solid #1a1a1a', transition: 'border-color 0.15s',
            }}
              onMouseEnter={e => (e.currentTarget.style.borderColor = '#333')}
              onMouseLeave={e => (e.currentTarget.style.borderColor = '#1a1a1a')}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
                <h3 style={{ fontSize: 15, fontWeight: 600 }}>{agent.agent_name}</h3>
                <span style={{
                  fontSize: 11, fontWeight: 500, padding: '3px 10px', borderRadius: 4,
                  background: agent.enabled ? 'rgba(34,197,94,0.15)' : 'rgba(239,68,68,0.15)',
                  color: agent.enabled ? '#22c55e' : '#ef4444',
                }}>
                  {agent.enabled ? 'Active' : 'Disabled'}
                </span>
              </div>
              <div style={{ display: 'flex', gap: 16, fontSize: 12, color: '#555', marginBottom: 10 }}>
                <span>{agent.model}</span>
                <span>temp {agent.temperature}</span>
              </div>
              <p style={{ fontSize: 13, color: '#888', lineHeight: 1.6 }}>{agent.system_prompt}</p>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
