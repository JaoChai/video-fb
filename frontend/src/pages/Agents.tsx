import { useState, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { apiFetch } from '../api';

interface Agent {
  id: string;
  agent_name: string;
  system_prompt: string;
  model: string;
  temperature: number;
  enabled: boolean;
  skills: string;
}

const AGENT_DESCRIPTIONS: Record<string, string> = {
  question: 'Generates realistic customer questions about Facebook Ads in Thai',
  script: 'Writes Q&A video scripts with 5 scenes answering customer questions',
  image: 'Creates image prompts for video scenes matching brand theme',
  analytics: 'Analyzes video performance metrics and recommends improvements',
};

const textareaStyle: React.CSSProperties = {
  width: '100%', padding: '10px 14px', borderRadius: 6,
  border: '1px solid #222', background: '#0a0a0a', color: '#fafafa',
  fontSize: 13, lineHeight: 1.6, outline: 'none', resize: 'vertical',
  fontFamily: 'inherit', transition: 'border-color 0.15s',
};

export default function AgentsPage() {
  const qc = useQueryClient();
  const { data: agents, isLoading } = useQuery({ queryKey: ['agents'], queryFn: () => apiFetch<Agent[]>('/api/v1/agents') });

  const [edits, setEdits] = useState<Record<string, Partial<Agent>>>({});
  const [dirty, setDirty] = useState<Record<string, boolean>>({});
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});

  useEffect(() => {
    if (agents) {
      const initial: Record<string, Partial<Agent>> = {};
      agents.forEach(a => {
        initial[a.id] = { system_prompt: a.system_prompt, skills: a.skills ?? '', temperature: a.temperature, enabled: a.enabled };
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
          model: agent.model,
          temperature: e.temperature ?? agent.temperature,
          enabled: e.enabled ?? agent.enabled,
          skills: e.skills ?? agent.skills ?? '',
        }),
      });
    },
    onSuccess: (_data, { id }) => {
      qc.invalidateQueries({ queryKey: ['agents'] });
      setDirty(prev => ({ ...prev, [id]: false }));
    },
  });

  const handleEdit = (id: string, field: keyof Agent, value: string | number | boolean) => {
    setEdits(prev => ({ ...prev, [id]: { ...prev[id], [field]: value } }));
    setDirty(prev => ({ ...prev, [id]: true }));
  };

  const toggleExpand = (id: string) => {
    setExpanded(prev => ({ ...prev, [id]: !prev[id] }));
  };

  return (
    <div>
      <h1 style={{ fontSize: 20, fontWeight: 600, marginBottom: 32 }}>Agents</h1>

      {isLoading ? <p style={{ color: '#555' }}>Loading...</p> : (
        <div style={{ display: 'grid', gap: 16 }}>
          {agents?.map(agent => {
            const e = edits[agent.id] ?? {};
            const isExpanded = expanded[agent.id] ?? false;
            const isDirty = dirty[agent.id] ?? false;

            return (
              <div key={agent.id} style={{
                background: '#111', borderRadius: 10, overflow: 'hidden',
                border: '1px solid #1a1a1a', transition: 'border-color 0.15s',
              }}
                onMouseEnter={ev => (ev.currentTarget.style.borderColor = '#333')}
                onMouseLeave={ev => (ev.currentTarget.style.borderColor = '#1a1a1a')}>

                {/* Header */}
                <div style={{
                  display: 'flex', justifyContent: 'space-between', alignItems: 'center',
                  padding: '18px 22px', cursor: 'pointer',
                }}
                  onClick={() => toggleExpand(agent.id)}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                    <h3 style={{ fontSize: 15, fontWeight: 600 }}>{agent.agent_name}</h3>
                    <span style={{
                      fontSize: 10, fontWeight: 500, padding: '3px 10px', borderRadius: 4,
                      background: agent.enabled ? 'rgba(34,197,94,0.12)' : 'rgba(239,68,68,0.12)',
                      color: agent.enabled ? '#22c55e' : '#ef4444',
                    }}>
                      {agent.enabled ? 'Active' : 'Disabled'}
                    </span>
                  </div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                    {/* Model badge */}
                    <Link to="/settings" style={{ textDecoration: 'none' }}
                      onClick={ev => ev.stopPropagation()}>
                      <span style={{
                        display: 'inline-flex', alignItems: 'center', gap: 6,
                        fontSize: 11, color: '#888', background: '#1a1a1a',
                        padding: '4px 12px', borderRadius: 5, border: '1px solid #222',
                        transition: 'border-color 0.15s, color 0.15s',
                      }}
                        onMouseEnter={ev => { ev.currentTarget.style.borderColor = '#444'; ev.currentTarget.style.color = '#ccc'; }}
                        onMouseLeave={ev => { ev.currentTarget.style.borderColor = '#222'; ev.currentTarget.style.color = '#888'; }}>
                        <span style={{ width: 6, height: 6, borderRadius: '50%', background: '#22c55e', flexShrink: 0 }} />
                        {agent.model}
                        <span style={{ fontSize: 9, color: '#555' }}>Settings ›</span>
                      </span>
                    </Link>
                    {/* Expand arrow */}
                    <span style={{
                      fontSize: 12, color: '#555', transition: 'transform 0.2s',
                      transform: isExpanded ? 'rotate(180deg)' : 'rotate(0deg)',
                    }}>▼</span>
                  </div>
                </div>

                {/* Description */}
                <div style={{ padding: '0 22px 14px', fontSize: 12, color: '#555' }}>
                  {AGENT_DESCRIPTIONS[agent.agent_name] ?? ''}
                </div>

                {/* Expanded content */}
                {isExpanded && (
                  <div style={{ padding: '0 22px 22px', display: 'grid', gap: 18 }}>
                    {/* Temperature + Enabled */}
                    <div style={{ display: 'flex', gap: 16, alignItems: 'center' }}>
                      <div style={{ flex: 1 }}>
                        <label style={{ display: 'block', fontSize: 11, color: '#555', marginBottom: 6, textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                          Temperature — {(e.temperature ?? agent.temperature).toFixed(1)}
                        </label>
                        <input
                          type="range" min="0" max="1" step="0.1"
                          value={e.temperature ?? agent.temperature}
                          onChange={ev => handleEdit(agent.id, 'temperature', parseFloat(ev.target.value))}
                          style={{ width: '100%', accentColor: '#fff' }}
                        />
                      </div>
                      <div>
                        <label style={{ display: 'block', fontSize: 11, color: '#555', marginBottom: 6, textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                          Enabled
                        </label>
                        <button
                          onClick={() => handleEdit(agent.id, 'enabled', !(e.enabled ?? agent.enabled))}
                          style={{
                            width: 44, height: 24, borderRadius: 12, border: 'none', cursor: 'pointer',
                            background: (e.enabled ?? agent.enabled) ? '#22c55e' : '#333',
                            position: 'relative', transition: 'background 0.2s',
                          }}>
                          <span style={{
                            position: 'absolute', top: 3, width: 18, height: 18, borderRadius: '50%',
                            background: '#fff', transition: 'left 0.2s',
                            left: (e.enabled ?? agent.enabled) ? 23 : 3,
                          }} />
                        </button>
                      </div>
                    </div>

                    {/* System Prompt */}
                    <div>
                      <label style={{ display: 'block', fontSize: 11, color: '#555', marginBottom: 6, textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                        System Prompt
                      </label>
                      <textarea
                        rows={4}
                        value={e.system_prompt ?? agent.system_prompt}
                        onChange={ev => handleEdit(agent.id, 'system_prompt', ev.target.value)}
                        style={textareaStyle}
                        onFocus={ev => (ev.target.style.borderColor = '#444')}
                        onBlur={ev => (ev.target.style.borderColor = '#222')}
                      />
                    </div>

                    {/* Skills */}
                    <div>
                      <label style={{ display: 'block', fontSize: 11, color: '#555', marginBottom: 6, textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                        Skills
                      </label>
                      <textarea
                        rows={5}
                        value={e.skills ?? agent.skills ?? ''}
                        onChange={ev => handleEdit(agent.id, 'skills', ev.target.value)}
                        placeholder={'Define what this agent can do, e.g.:\n- Search knowledge base for relevant answers\n- Generate questions in Thai language\n- Follow brand voice guidelines'}
                        style={textareaStyle}
                        onFocus={ev => (ev.target.style.borderColor = '#444')}
                        onBlur={ev => (ev.target.style.borderColor = '#222')}
                      />
                    </div>

                    {/* Save */}
                    <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                      <button
                        onClick={() => update.mutate({ id: agent.id, agent })}
                        disabled={update.isPending || !isDirty}
                        style={{
                          padding: '9px 24px', borderRadius: 6, border: 'none',
                          background: isDirty ? '#fff' : '#222', color: isDirty ? '#000' : '#555',
                          fontSize: 13, fontWeight: 600, cursor: isDirty ? 'pointer' : 'default',
                          transition: 'all 0.15s',
                        }}>
                        {update.isPending ? 'Saving...' : 'Save'}
                      </button>
                      {update.isSuccess && !isDirty && (
                        <span style={{ fontSize: 12, color: '#22c55e' }}>Saved</span>
                      )}
                    </div>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
