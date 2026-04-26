import { useState, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiFetch } from '../api';

interface Source {
  id: string;
  name: string;
  category: string;
  content: string;
  enabled: boolean;
  chunk_count: number;
}

const CATEGORIES = ['pain_points', 'terminology', 'audience', 'content_strategy', 'guidelines', 'general'];

const categoryLabels: Record<string, string> = {
  pain_points: 'Pain Points',
  terminology: 'Terminology',
  audience: 'Audience',
  content_strategy: 'Content Strategy',
  guidelines: 'Guidelines',
  general: 'General',
};

const textareaStyle: React.CSSProperties = {
  width: '100%', padding: '10px 14px', borderRadius: 6,
  border: '1px solid #222', background: '#0a0a0a', color: '#fafafa',
  fontSize: 13, lineHeight: 1.7, outline: 'none', resize: 'vertical',
  fontFamily: 'inherit', transition: 'border-color 0.15s',
};

const inputStyle: React.CSSProperties = {
  flex: 1, padding: '10px 14px', borderRadius: 6,
  border: '1px solid #222', background: '#0a0a0a', color: '#fafafa',
  fontSize: 14, outline: 'none', transition: 'border-color 0.15s',
};

export default function KnowledgePage() {
  const qc = useQueryClient();
  const { data: sources, isLoading } = useQuery({
    queryKey: ['knowledge'],
    queryFn: () => apiFetch<Source[]>('/api/v1/knowledge/sources'),
  });

  const [edits, setEdits] = useState<Record<string, Partial<Source>>>({});
  const [dirty, setDirty] = useState<Record<string, boolean>>({});
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const [showNew, setShowNew] = useState(false);
  const [newDoc, setNewDoc] = useState({ name: '', category: 'general', content: '' });

  useEffect(() => {
    if (sources) {
      const initial: Record<string, Partial<Source>> = {};
      sources.forEach(s => {
        initial[s.id] = { name: s.name, category: s.category, content: s.content };
      });
      setEdits(initial);
    }
  }, [sources]);

  const updateSource = useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<Source> }) =>
      apiFetch(`/api/v1/knowledge/sources/${id}`, {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    onSuccess: (_d, { id }) => {
      qc.invalidateQueries({ queryKey: ['knowledge'] });
      setDirty(prev => ({ ...prev, [id]: false }));
    },
  });

  const createSource = useMutation({
    mutationFn: (data: typeof newDoc) =>
      apiFetch('/api/v1/knowledge/sources', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['knowledge'] });
      setNewDoc({ name: '', category: 'general', content: '' });
      setShowNew(false);
    },
  });

  const deleteSource = useMutation({
    mutationFn: (id: string) =>
      apiFetch(`/api/v1/knowledge/sources/${id}`, { method: 'DELETE' }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['knowledge'] }),
  });

  const rebuildAll = useMutation({
    mutationFn: () =>
      apiFetch('/api/v1/knowledge/rebuild', { method: 'POST' }),
  });

  const handleEdit = (id: string, field: keyof Source, value: string) => {
    setEdits(prev => ({ ...prev, [id]: { ...prev[id], [field]: value } }));
    setDirty(prev => ({ ...prev, [id]: true }));
  };

  const handleSave = (id: string) => {
    const e = edits[id];
    if (!e) return;
    updateSource.mutate({ id, data: e });
  };

  const toggleExpand = (id: string) => {
    setExpanded(prev => ({ ...prev, [id]: !prev[id] }));
  };

  const grouped = sources?.reduce((acc, s) => {
    const cat = s.category || 'general';
    if (!acc[cat]) acc[cat] = [];
    acc[cat].push(s);
    return acc;
  }, {} as Record<string, Source[]>);

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 32 }}>
        <h1 style={{ fontSize: 20, fontWeight: 600 }}>Knowledge Base</h1>
        <div style={{ display: 'flex', gap: 8 }}>
          <button onClick={() => rebuildAll.mutate()}
            disabled={rebuildAll.isPending}
            style={{
              padding: '8px 16px', borderRadius: 6, border: '1px solid #222',
              background: 'transparent', color: rebuildAll.isPending ? '#333' : '#888',
              fontSize: 12, cursor: 'pointer', transition: 'all 0.15s',
            }}>
            {rebuildAll.isPending ? 'Rebuilding...' : 'Rebuild Embeddings'}
          </button>
          <button onClick={() => setShowNew(true)}
            style={{
              padding: '8px 20px', borderRadius: 6, border: 'none',
              background: '#fff', color: '#000', fontSize: 12, fontWeight: 600,
              cursor: 'pointer', transition: 'all 0.15s',
            }}>
            + Add Document
          </button>
        </div>
      </div>

      {rebuildAll.isSuccess && (
        <div style={{
          marginBottom: 16, padding: '8px 14px', borderRadius: 6, fontSize: 12,
          background: 'rgba(34,197,94,0.1)', color: '#22c55e',
          border: '1px solid rgba(34,197,94,0.2)',
        }}>
          Embeddings are rebuilding in background. Refresh to see updated chunk counts.
        </div>
      )}

      {/* New document form */}
      {showNew && (
        <div style={{
          background: '#111', borderRadius: 10, padding: 22, marginBottom: 20,
          border: '1px solid #333',
        }}>
          <h3 style={{ fontSize: 14, fontWeight: 600, marginBottom: 16 }}>New Document</h3>
          <div style={{ display: 'flex', gap: 12, marginBottom: 14 }}>
            <input
              type="text" placeholder="Document name"
              value={newDoc.name}
              onChange={e => setNewDoc(prev => ({ ...prev, name: e.target.value }))}
              style={inputStyle}
              onFocus={e => (e.target.style.borderColor = '#444')}
              onBlur={e => (e.target.style.borderColor = '#222')}
            />
            <select
              value={newDoc.category}
              onChange={e => setNewDoc(prev => ({ ...prev, category: e.target.value }))}
              style={{ ...inputStyle, flex: 'none', width: 180 }}>
              {CATEGORIES.map(c => (
                <option key={c} value={c}>{categoryLabels[c]}</option>
              ))}
            </select>
          </div>
          <textarea
            rows={8} placeholder="Paste or write knowledge content here..."
            value={newDoc.content}
            onChange={e => setNewDoc(prev => ({ ...prev, content: e.target.value }))}
            style={textareaStyle}
            onFocus={e => (e.target.style.borderColor = '#444')}
            onBlur={e => (e.target.style.borderColor = '#222')}
          />
          <div style={{ display: 'flex', gap: 8, marginTop: 14 }}>
            <button onClick={() => createSource.mutate(newDoc)}
              disabled={createSource.isPending || !newDoc.name || !newDoc.content}
              style={{
                padding: '9px 24px', borderRadius: 6, border: 'none',
                background: newDoc.name && newDoc.content ? '#fff' : '#222',
                color: newDoc.name && newDoc.content ? '#000' : '#555',
                fontSize: 13, fontWeight: 600, cursor: newDoc.name && newDoc.content ? 'pointer' : 'default',
              }}>
              {createSource.isPending ? 'Saving...' : 'Save & Embed'}
            </button>
            <button onClick={() => setShowNew(false)}
              style={{
                padding: '9px 16px', borderRadius: 6, border: '1px solid #222',
                background: 'transparent', color: '#888', fontSize: 13, cursor: 'pointer',
              }}>
              Cancel
            </button>
          </div>
        </div>
      )}

      {isLoading ? <p style={{ color: '#555' }}>Loading...</p> : (
        <div style={{ display: 'grid', gap: 24 }}>
          {grouped && Object.entries(grouped).map(([cat, items]) => (
            <div key={cat}>
              <h2 style={{
                fontSize: 11, fontWeight: 500, color: '#555', textTransform: 'uppercase',
                letterSpacing: '0.08em', marginBottom: 10, padding: '0 4px',
              }}>
                {categoryLabels[cat] || cat} — {items.length} docs
              </h2>
              <div style={{ display: 'grid', gap: 8 }}>
                {items.map(source => {
                  const e = edits[source.id] ?? {};
                  const isExpanded = expanded[source.id] ?? false;
                  const isDirty = dirty[source.id] ?? false;

                  return (
                    <div key={source.id} style={{
                      background: '#111', borderRadius: 8, overflow: 'hidden',
                      border: '1px solid #1a1a1a', transition: 'border-color 0.15s',
                    }}
                      onMouseEnter={ev => (ev.currentTarget.style.borderColor = '#333')}
                      onMouseLeave={ev => (ev.currentTarget.style.borderColor = '#1a1a1a')}>

                      {/* Header */}
                      <div style={{
                        display: 'flex', justifyContent: 'space-between', alignItems: 'center',
                        padding: '14px 18px', cursor: 'pointer',
                      }}
                        onClick={() => toggleExpand(source.id)}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                          <span style={{ fontSize: 14, fontWeight: 500 }}>{source.name}</span>
                          <span style={{
                            fontSize: 10, color: '#555', background: '#1a1a1a',
                            padding: '2px 8px', borderRadius: 4,
                          }}>
                            {source.chunk_count} chunks
                          </span>
                        </div>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                          <span style={{
                            width: 6, height: 6, borderRadius: '50%',
                            background: source.enabled ? '#22c55e' : '#ef4444',
                          }} />
                          <span style={{
                            fontSize: 12, color: '#555', transition: 'transform 0.2s',
                            transform: isExpanded ? 'rotate(180deg)' : 'rotate(0deg)',
                          }}>▼</span>
                        </div>
                      </div>

                      {/* Preview */}
                      {!isExpanded && (
                        <div style={{
                          padding: '0 18px 12px', fontSize: 12, color: '#555',
                          overflow: 'hidden', whiteSpace: 'nowrap', textOverflow: 'ellipsis',
                        }}>
                          {source.content.slice(0, 120)}...
                        </div>
                      )}

                      {/* Expanded */}
                      {isExpanded && (
                        <div style={{ padding: '0 18px 18px', display: 'grid', gap: 14 }}>
                          <div style={{ display: 'flex', gap: 12 }}>
                            <input
                              type="text"
                              value={e.name ?? source.name}
                              onChange={ev => handleEdit(source.id, 'name', ev.target.value)}
                              style={inputStyle}
                              onFocus={ev => (ev.target.style.borderColor = '#444')}
                              onBlur={ev => (ev.target.style.borderColor = '#222')}
                            />
                            <select
                              value={e.category ?? source.category}
                              onChange={ev => handleEdit(source.id, 'category', ev.target.value)}
                              style={{ ...inputStyle, flex: 'none', width: 180 }}>
                              {CATEGORIES.map(c => (
                                <option key={c} value={c}>{categoryLabels[c]}</option>
                              ))}
                            </select>
                          </div>
                          <textarea
                            rows={12}
                            value={e.content ?? source.content}
                            onChange={ev => handleEdit(source.id, 'content', ev.target.value)}
                            style={textareaStyle}
                            onFocus={ev => (ev.target.style.borderColor = '#444')}
                            onBlur={ev => (ev.target.style.borderColor = '#222')}
                          />
                          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                            <button onClick={() => handleSave(source.id)}
                              disabled={updateSource.isPending || !isDirty}
                              style={{
                                padding: '9px 24px', borderRadius: 6, border: 'none',
                                background: isDirty ? '#fff' : '#222', color: isDirty ? '#000' : '#555',
                                fontSize: 13, fontWeight: 600,
                                cursor: isDirty ? 'pointer' : 'default',
                              }}>
                              {updateSource.isPending ? 'Saving...' : 'Save & Re-embed'}
                            </button>
                            <button onClick={() => {
                              if (confirm('Delete this document?')) deleteSource.mutate(source.id);
                            }}
                              style={{
                                padding: '9px 16px', borderRadius: 6, border: '1px solid #222',
                                background: 'transparent', color: '#ef4444', fontSize: 12,
                                cursor: 'pointer', transition: 'all 0.15s',
                              }}>
                              Delete
                            </button>
                            {updateSource.isSuccess && !isDirty && (
                              <span style={{ fontSize: 12, color: '#22c55e' }}>Saved — re-embedding in background</span>
                            )}
                          </div>
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
