import { useState, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiFetch } from '../api';
import { PageHeader } from '../components/page-header';
import { Button } from '../components/ui/button';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '../components/ui/card';
import { Badge } from '../components/ui/badge';
import { Input } from '../components/ui/input';
import { useToast } from '../components/ui/toaster';
import { Skeleton } from '../components/ui/skeleton';

interface ZernioAccount {
  _id: string;
  platform: string;
  displayName: string;
  username: string;
  profilePicture: string;
  profileUrl: string;
  followersCount: number;
  isActive: boolean;
  platformStatus: string;
  metadata?: {
    profileData?: {
      extraData?: {
        totalViews?: number;
        videoCount?: number;
      };
    };
  };
}

interface TestResult {
  data?: {
    label?: string;
    limit_remaining?: number | null;
    is_free_tier?: boolean;
    usage_monthly?: number;
  };
  error?: string;
}

interface Agent {
  id: string;
  agent_name: string;
  system_prompt: string;
  model: string;
  temperature: number;
  enabled: boolean;
  skills: string;
}

const TTS_VOICES = [
  'Achird', 'Charon', 'Sadaltager', 'Sulafat', 'Rasalgethi',
  'Puck', 'Iapetus', 'Schedar', 'Gacrux', 'Algieba',
  'Zephyr', 'Kore', 'Fenrir', 'Leda', 'Orus', 'Aoede',
  'Achernar', 'Alnilam', 'Despina', 'Erinome',
];

const FIELDS = [
  { key: 'openrouter_api_key', label: 'OpenRouter API Key', placeholder: 'sk-or-v1-...', secret: true, testable: true },
  { key: 'kie_api_key', label: 'Kie AI API Key (Upload)', placeholder: 'kie-...', secret: true, testable: false },
  { key: 'elevenlabs_voice', label: 'TTS Voice (Gemini)', placeholder: 'alloy', secret: false, testable: false, dropdown: TTS_VOICES },
  { key: 'zernio_api_key', label: 'Zernio API Key', placeholder: 'zrn-...', secret: true, testable: false },
];

export default function SettingsPage() {
  const qc = useQueryClient();
  const { data: saved } = useQuery({ queryKey: ['settings'], queryFn: () => apiFetch<Record<string, string>>('/api/v1/settings') });
  const { data: agents } = useQuery({ queryKey: ['agents'], queryFn: () => apiFetch<Agent[]>('/api/v1/agents') });
  const { data: zernioData, isLoading: zernioLoading } = useQuery({
    queryKey: ['zernio-accounts'],
    queryFn: () => apiFetch<{ accounts: ZernioAccount[]; hasAnalyticsAccess: boolean }>('/api/v1/settings/test-zernio'),
    retry: false,
  });

  const { success, error: showError } = useToast();
  const [form, setForm] = useState<Record<string, string>>({});
  const [dirty, setDirty] = useState<Record<string, boolean>>({});
  const [testResult, setTestResult] = useState<TestResult | null>(null);
  const [showKeys, setShowKeys] = useState<Record<string, boolean>>({});
  const [agentModels, setAgentModels] = useState<Record<string, string>>({});
  const [agentModelDirty, setAgentModelDirty] = useState<Record<string, boolean>>({});

  useEffect(() => {
    if (saved) setForm(saved);
  }, [saved]);

  useEffect(() => {
    if (agents) {
      const models: Record<string, string> = {};
      agents.forEach(a => { models[a.id] = a.model; });
      setAgentModels(models);
    }
  }, [agents]);

  const save = useMutation({
    mutationFn: (data: Record<string, string>) =>
      apiFetch('/api/v1/settings', { method: 'PUT', body: JSON.stringify(data) }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['settings'] }); setDirty({}); success('บันทึกการตั้งค่าแล้ว'); },
    onError: (e) => showError(`บันทึกล้มเหลว: ${(e as Error).message}`),
  });

  const saveAgentModel = useMutation({
    mutationFn: ({ id, agent }: { id: string; agent: Agent }) =>
      apiFetch(`/api/v1/agents/${id}`, {
        method: 'PATCH',
        body: JSON.stringify({
          system_prompt: agent.system_prompt,
          model: agentModels[id],
          temperature: agent.temperature,
          enabled: agent.enabled,
          skills: agent.skills,
        }),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['agents'] });
      setAgentModelDirty({});
      success('บันทึก model แล้ว');
    },
    onError: (e) => showError(`บันทึก model ล้มเหลว: ${(e as Error).message}`),
  });

  const testKey = useMutation({
    mutationFn: (key: string) =>
      apiFetch<TestResult>('/api/v1/settings/test-key', { method: 'POST', body: JSON.stringify({ key }) }),
    onSuccess: (data) => setTestResult(data as unknown as TestResult),
    onError: (err) => setTestResult({ error: (err as Error).message }),
  });

  const handleChange = (key: string, value: string) => {
    setForm(prev => ({ ...prev, [key]: value }));
    setDirty(prev => ({ ...prev, [key]: true }));
  };

  const handleSave = () => {
    const updates: Record<string, string> = {};
    for (const key of Object.keys(dirty)) {
      if (dirty[key]) updates[key] = form[key] ?? '';
    }
    if (Object.keys(updates).length > 0) save.mutate(updates);
  };

  const handleAgentModelChange = (id: string, value: string) => {
    setAgentModels(prev => ({ ...prev, [id]: value }));
    setAgentModelDirty(prev => ({ ...prev, [id]: true }));
  };

  const handleSaveAgentModels = () => {
    if (!agents) return;
    for (const agent of agents) {
      if (agentModelDirty[agent.id]) {
        saveAgentModel.mutate({ id: agent.id, agent });
      }
    }
  };

  const hasDirty = Object.values(dirty).some(Boolean);
  const hasAgentModelDirty = Object.values(agentModelDirty).some(Boolean);

  // Separate fields by type for card grouping
  const apiKeyFields = FIELDS.filter(f => f.secret);
  const voiceField = FIELDS.find(f => f.dropdown);

  return (
    <div>
      <PageHeader title="Settings" />

      <div className="grid gap-6 max-w-2xl">
        {/* API Keys Card */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">API Keys</CardTitle>
            <CardDescription>Configure API keys for external services</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {apiKeyFields.map(({ key, label, placeholder, testable }) => (
              <div key={key}>
                <label className="block text-xs text-muted-foreground uppercase tracking-wide mb-1.5">
                  {label}
                </label>
                <div className="flex gap-2">
                  <Input
                    type={showKeys[key] ? 'text' : 'password'}
                    value={form[key] ?? ''}
                    placeholder={placeholder}
                    onChange={e => handleChange(key, e.target.value)}
                    className="flex-1"
                  />
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setShowKeys(prev => ({ ...prev, [key]: !prev[key] }))}
                  >
                    {showKeys[key] ? 'Hide' : 'Show'}
                  </Button>
                  {testable && (
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => testKey.mutate(form[key] ?? '')}
                      disabled={testKey.isPending || !form[key]}
                    >
                      {testKey.isPending ? 'Testing...' : 'Test'}
                    </Button>
                  )}
                </div>
                {testable && testResult && (
                  <div className={`mt-2 px-3 py-2 rounded-md text-xs border ${
                    testResult.error
                      ? 'bg-destructive/10 text-destructive border-destructive/20'
                      : 'bg-green-500/10 text-green-500 border-green-500/20'
                  }`}>
                    {testResult.error ? `Failed: ${testResult.error}` : (
                      <span className="flex gap-4">
                        <span>Connected</span>
                        {testResult.data?.label && <span>Label: {testResult.data.label}</span>}
                        {testResult.data?.limit_remaining != null && <span>Credits: {testResult.data.limit_remaining.toLocaleString()}</span>}
                        <span>{testResult.data?.is_free_tier ? 'Free' : 'Paid'}</span>
                      </span>
                    )}
                  </div>
                )}
              </div>
            ))}

            <div className="flex items-center gap-3 pt-2">
              <Button onClick={handleSave} disabled={save.isPending || !hasDirty}>
                {save.isPending ? 'Saving...' : 'Save'}
              </Button>
              {save.isSuccess && <span className="text-xs text-green-500">Saved</span>}
            </div>
          </CardContent>
        </Card>

        {/* Voice Settings Card */}
        {voiceField && (
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Voice Settings</CardTitle>
              <CardDescription>Select the TTS voice for video narration</CardDescription>
            </CardHeader>
            <CardContent>
              <label className="block text-xs text-muted-foreground uppercase tracking-wide mb-1.5">
                {voiceField.label}
              </label>
              <select
                value={form[voiceField.key] ?? ''}
                onChange={e => handleChange(voiceField.key, e.target.value)}
                className="h-10 w-full max-w-xs rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 cursor-pointer"
              >
                {voiceField.dropdown!.map(v => (
                  <option key={v} value={v}>{v}</option>
                ))}
              </select>
            </CardContent>
          </Card>
        )}

        {/* Connected Accounts Card */}
        <Card>
          <CardHeader>
            <div className="flex items-center gap-3">
              <CardTitle className="text-base">Connected Accounts</CardTitle>
              <Badge variant="secondary" className="text-[10px]">via Zernio</Badge>
            </div>
            <CardDescription>YouTube and social media accounts connected through Zernio</CardDescription>
          </CardHeader>
          <CardContent>
            {zernioLoading ? (
              <div className="space-y-3">
                {[1, 2].map(i => (
                  <div key={i} className="flex items-center gap-3.5 rounded-lg border p-3.5">
                    <Skeleton className="w-10 h-10 rounded-full" />
                    <div className="flex-1 space-y-1.5">
                      <Skeleton className="h-4 w-32" />
                      <Skeleton className="h-3 w-48" />
                    </div>
                    <Skeleton className="h-5 w-20 rounded-full" />
                  </div>
                ))}
              </div>
            ) : zernioData?.accounts?.length ? (
              <div className="grid gap-3">
                {zernioData.accounts.filter(a => a._id === saved?.zernio_youtube_account_id).map(account => {
                  const isSelected = true;
                  const extra = account.metadata?.profileData?.extraData;
                  return (
                    <div
                      key={account._id}
                      className={`flex items-center gap-3.5 rounded-lg p-3.5 border ${
                        isSelected
                          ? 'bg-green-500/5 border-green-500/30'
                          : 'bg-card border-border'
                      }`}
                    >
                      <img src={account.profilePicture} alt="" className="w-10 h-10 rounded-full" />
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2">
                          <span className="text-sm font-medium">{account.displayName}</span>
                          {isSelected && (
                            <Badge variant="secondary" className="text-[10px] bg-green-500/15 text-green-500 border-0">
                              Active
                            </Badge>
                          )}
                        </div>
                        <div className="flex gap-3 mt-1">
                          <span className="text-[11px] text-muted-foreground">@{account.username}</span>
                          <span className="text-[11px] text-muted-foreground capitalize">{account.platform}</span>
                          {account.followersCount > 0 && (
                            <span className="text-[11px] text-muted-foreground">{account.followersCount.toLocaleString()} subs</span>
                          )}
                          {extra?.videoCount != null && (
                            <span className="text-[11px] text-muted-foreground">{extra.videoCount} videos</span>
                          )}
                        </div>
                      </div>
                      <Badge
                        variant={account.isActive ? 'secondary' : 'destructive'}
                        className={`text-[10px] shrink-0 ${
                          account.isActive
                            ? 'bg-green-500/10 text-green-500 border-0'
                            : 'bg-red-500/10 text-red-500 border-0'
                        }`}
                      >
                        {account.isActive ? 'Connected' : 'Disconnected'}
                      </Badge>
                    </div>
                  );
                })}
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">
                No channels connected. Set Zernio API Key above and connect channels in Zernio dashboard.
              </p>
            )}
          </CardContent>
        </Card>

        {/* Agent Models Card */}
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
                  value={agentModels[agent.id] ?? agent.model}
                  onChange={e => handleAgentModelChange(agent.id, e.target.value)}
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
              <Button onClick={handleSaveAgentModels} disabled={saveAgentModel.isPending || !hasAgentModelDirty}>
                {saveAgentModel.isPending ? 'Saving...' : 'Save Models'}
              </Button>
              {saveAgentModel.isSuccess && <span className="text-xs text-green-500">Saved</span>}
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
