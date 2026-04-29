import { useState, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiFetch } from '../api';
import { PageHeader } from '../components/page-header';
import { Button } from '../components/ui/button';
import { Card, CardHeader, CardContent } from '../components/ui/card';
import { Badge } from '../components/ui/badge';
import { Input } from '../components/ui/input';
import { Textarea } from '../components/ui/textarea';
import { ChevronDown, Plus, RefreshCw } from 'lucide-react';
import { useToast } from '../components/ui/toaster';

interface SourceSummary {
  id: string;
  name: string;
  category: string;
  content_preview: string;
  enabled: boolean;
  chunk_count: number;
}

interface Source extends SourceSummary {
  content: string;
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

export default function KnowledgePage() {
  const qc = useQueryClient();
  const { success, error: showError } = useToast();
  const { data: sources, isLoading } = useQuery({
    queryKey: ['knowledge'],
    queryFn: () => apiFetch<SourceSummary[]>('/api/v1/knowledge/sources'),
  });

  const [edits, setEdits] = useState<Record<string, Partial<Source>>>({});
  const [dirty, setDirty] = useState<Record<string, boolean>>({});
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const [showNew, setShowNew] = useState(false);
  const [newDoc, setNewDoc] = useState({ name: '', category: 'general', content: '' });

  const [fullSources, setFullSources] = useState<Record<string, Source>>({});

  const loadFullSource = async (id: string) => {
    if (fullSources[id]) return;
    try {
      const source = await apiFetch<Source>(`/api/v1/knowledge/sources/${id}`);
      setFullSources(prev => ({ ...prev, [id]: source }));
      setEdits(prev => ({ ...prev, [id]: { name: source.name, category: source.category, content: source.content } }));
    } catch (e) {
      console.error('Failed to load source', e);
    }
  };

  useEffect(() => {
    if (sources) {
      setEdits(prev => {
        const next = { ...prev };
        sources.forEach(s => {
          if (!next[s.id]) {
            next[s.id] = { name: s.name, category: s.category };
          }
        });
        return next;
      });
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
      success('บันทึกเอกสารแล้ว');
    },
    onError: (e) => showError(`บันทึกล้มเหลว: ${(e as Error).message}`),
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
      success('สร้างเอกสารแล้ว — กำลัง embed');
    },
    onError: (e) => showError(`สร้างเอกสารล้มเหลว: ${(e as Error).message}`),
  });

  const embedSource = useMutation({
    mutationFn: (id: string) =>
      apiFetch<{ chunks: number }>(`/api/v1/knowledge/sources/${id}/embed`, { method: 'POST' }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['knowledge'] });
      success('Embed สำเร็จ');
    },
    onError: (e) => showError(`Embed ล้มเหลว: ${(e as Error).message}`),
  });

  const deleteSource = useMutation({
    mutationFn: (id: string) =>
      apiFetch(`/api/v1/knowledge/sources/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['knowledge'] });
      success('ลบเอกสารแล้ว');
    },
    onError: (e) => showError(`ลบล้มเหลว: ${(e as Error).message}`),
  });

  const [rebuildingAll, setRebuildingAll] = useState(false);
  const [rebuildProgress, setRebuildProgress] = useState('');

  const rebuildAll = async () => {
    if (!sources) return;
    setRebuildingAll(true);
    for (let i = 0; i < sources.length; i++) {
      const s = sources[i];
      setRebuildProgress(`${i + 1}/${sources.length} — ${s.name}`);
      try {
        await apiFetch(`/api/v1/knowledge/sources/${s.id}/embed`, { method: 'POST' });
      } catch (e) {
        console.error(`embed ${s.name} failed`, e);
      }
    }
    setRebuildingAll(false);
    setRebuildProgress('');
    qc.invalidateQueries({ queryKey: ['knowledge'] });
    success('Rebuild embeddings สำเร็จทั้งหมด');
  };

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
    const willExpand = !expanded[id];
    setExpanded(prev => ({ ...prev, [id]: willExpand }));
    if (willExpand) {
      loadFullSource(id);
    }
  };

  const grouped = sources?.reduce((acc, s) => {
    const cat = s.category || 'general';
    if (!acc[cat]) acc[cat] = [];
    acc[cat].push(s);
    return acc;
  }, {} as Record<string, SourceSummary[]>);

  return (
    <div>
      <PageHeader
        title="Knowledge Base"
        actions={
          <>
            <Button variant="outline" size="sm" onClick={rebuildAll} disabled={rebuildingAll}>
              <RefreshCw className={`h-4 w-4 mr-2 ${rebuildingAll ? 'animate-spin' : ''}`} />
              {rebuildingAll ? `Embedding ${rebuildProgress}` : 'Rebuild Embeddings'}
            </Button>
            <Button size="sm" onClick={() => setShowNew(true)}>
              <Plus className="h-4 w-4 mr-2" />
              Add Document
            </Button>
          </>
        }
      />

      {/* New document form */}
      {showNew && (
        <Card className="border-dashed mb-5">
          <CardHeader className="pb-4">
            <h3 className="text-sm font-semibold">New Document</h3>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex gap-3">
              <Input
                placeholder="Document name"
                value={newDoc.name}
                onChange={e => setNewDoc(prev => ({ ...prev, name: e.target.value }))}
                className="flex-1"
              />
              <select
                value={newDoc.category}
                onChange={e => setNewDoc(prev => ({ ...prev, category: e.target.value }))}
                className="h-10 rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 w-[180px]"
              >
                {CATEGORIES.map(c => (
                  <option key={c} value={c}>{categoryLabels[c]}</option>
                ))}
              </select>
            </div>
            <Textarea
              rows={8}
              placeholder="Paste or write knowledge content here..."
              value={newDoc.content}
              onChange={e => setNewDoc(prev => ({ ...prev, content: e.target.value }))}
            />
            <div className="flex gap-2">
              <Button
                onClick={() => createSource.mutate(newDoc)}
                disabled={createSource.isPending || !newDoc.name || !newDoc.content}
              >
                {createSource.isPending ? 'Saving...' : 'Save & Embed'}
              </Button>
              <Button variant="outline" onClick={() => setShowNew(false)}>
                Cancel
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {isLoading ? (
        <p className="text-muted-foreground">Loading...</p>
      ) : (
        <div className="grid gap-6">
          {grouped && Object.entries(grouped).map(([cat, items]) => (
            <div key={cat}>
              <h2 className="text-xs font-medium text-muted-foreground uppercase tracking-wider mb-3 px-1">
                {categoryLabels[cat] || cat} — {items.length} docs
              </h2>
              <div className="grid gap-2">
                {items.map(source => {
                  const e = edits[source.id] ?? {};
                  const isExpanded = expanded[source.id] ?? false;
                  const isDirty = dirty[source.id] ?? false;

                  return (
                    <Card key={source.id} className="overflow-hidden">
                      {/* Header */}
                      <div
                        className="flex items-center justify-between px-4 py-3 cursor-pointer hover:bg-muted/50 transition-colors"
                        onClick={() => toggleExpand(source.id)}
                      >
                        <div className="flex items-center gap-2.5">
                          <span className="text-sm font-medium">{source.name}</span>
                          <Badge variant="secondary" className="text-[10px] px-2 py-0">
                            {source.chunk_count} chunks
                          </Badge>
                        </div>
                        <div className="flex items-center gap-2">
                          <span
                            className={`w-1.5 h-1.5 rounded-full ${
                              source.enabled ? 'bg-green-500' : 'bg-red-500'
                            }`}
                          />
                          <ChevronDown
                            className={`h-4 w-4 text-muted-foreground transition-transform duration-200 ${
                              isExpanded ? 'rotate-180' : ''
                            }`}
                          />
                        </div>
                      </div>

                      {/* Preview (collapsed) */}
                      {!isExpanded && (
                        <div className="px-4 pb-3 text-xs text-muted-foreground truncate">
                          {source.content_preview}...
                        </div>
                      )}

                      {/* Expanded */}
                      {isExpanded && (
                        <CardContent className="pt-0 space-y-3">
                          <div className="flex gap-3">
                            <Input
                              value={e.name ?? source.name}
                              onChange={ev => handleEdit(source.id, 'name', ev.target.value)}
                              className="flex-1"
                            />
                            <select
                              value={e.category ?? source.category}
                              onChange={ev => handleEdit(source.id, 'category', ev.target.value)}
                              className="h-10 rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 w-[180px]"
                            >
                              {CATEGORIES.map(c => (
                                <option key={c} value={c}>{categoryLabels[c]}</option>
                              ))}
                            </select>
                          </div>
                          {fullSources[source.id] ? (
                            <Textarea
                              rows={12}
                              value={e.content ?? fullSources[source.id].content}
                              onChange={ev => handleEdit(source.id, 'content', ev.target.value)}
                            />
                          ) : (
                            <p className="text-muted-foreground text-sm py-2">Loading content...</p>
                          )}
                          <div className="flex items-center gap-2">
                            <Button
                              onClick={() => handleSave(source.id)}
                              disabled={updateSource.isPending || !isDirty}
                              size="sm"
                            >
                              {updateSource.isPending ? 'Saving...' : 'Save'}
                            </Button>
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => embedSource.mutate(source.id)}
                              disabled={embedSource.isPending}
                            >
                              {embedSource.isPending ? 'Embedding...' : 'Embed'}
                            </Button>
                            <Button
                              variant="ghost"
                              size="sm"
                              className="text-destructive hover:text-destructive"
                              onClick={() => {
                                if (confirm('Delete this document?')) deleteSource.mutate(source.id);
                              }}
                            >
                              Delete
                            </Button>
                            {updateSource.isSuccess && !isDirty && (
                              <span className="text-xs text-green-500">Saved</span>
                            )}
                          </div>
                        </CardContent>
                      )}
                    </Card>
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
