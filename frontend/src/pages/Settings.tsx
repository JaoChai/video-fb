import { useState, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiFetch, getKieCredits } from '../api';
import { PageHeader } from '../components/page-header';
import { useToast } from '../components/ui/toaster';
import { ApiKeysCard } from '../components/settings/ApiKeysCard';
import { VoiceSettingsCard } from '../components/settings/VoiceSettingsCard';
import { ConnectedAccountsCard } from '../components/settings/ConnectedAccountsCard';
import { AgentModelsCard } from '../components/settings/AgentModelsCard';
import { Button } from '../components/ui/button';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '../components/ui/card';
import { Input } from '../components/ui/input';
import { Textarea } from '../components/ui/textarea';
import { Switch } from '../components/ui/switch';
import { Skeleton } from '../components/ui/skeleton';

interface Agent {
  id: string;
  agent_name: string;
  system_prompt: string;
  model: string;
  temperature: number;
  enabled: boolean;
  skills: string;
}

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
      extraData?: { totalViews?: number; videoCount?: number };
    };
  };
}

interface ContentSettingsCardProps {
  form: Record<string, string>;
  dirty: Record<string, boolean>;
  onSave: () => void;
  saving: boolean;
  saved: boolean;
  onChange: (key: string, value: string) => void;
}

function KieCreditsCard() {
  const { data, isLoading } = useQuery({
    queryKey: ['kie-credits'],
    queryFn: getKieCredits,
    retry: false,
  });

  const unavailable = !data || data.credits === -1 || data.error !== undefined;
  const positive = !unavailable && data!.credits > 0;

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">kie.ai Credits</CardTitle>
        <CardDescription>Current credit balance for image/video generation</CardDescription>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <Skeleton className="h-8 w-24" />
        ) : unavailable ? (
          <span className="text-sm text-muted-foreground">ดูไม่ได้</span>
        ) : (
          <span className={`text-2xl font-bold ${positive ? 'text-green-600' : 'text-red-600'}`}>
            {data!.credits.toLocaleString()}
          </span>
        )}
      </CardContent>
    </Card>
  );
}

function ContentSettingsCard({ form, dirty, onSave, saving, saved, onChange }: ContentSettingsCardProps) {
  const hasDirty = Object.values(dirty).some(Boolean);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Content & Publishing Settings</CardTitle>
        <CardDescription>Audience persona and TikTok publishing configuration</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div>
          <label className="block text-xs text-muted-foreground uppercase tracking-wide mb-1.5">
            Audience Persona
          </label>
          <Textarea
            value={form['audience_persona'] ?? ''}
            onChange={e => onChange('audience_persona', e.target.value)}
            placeholder="Describe the target audience for content generation..."
            className="min-h-[100px]"
          />
        </div>
        <div>
          <label className="block text-xs text-muted-foreground uppercase tracking-wide mb-1.5">
            TikTok Account ID (Zernio)
          </label>
          <Input
            value={form['zernio_tiktok_account_id'] ?? ''}
            onChange={e => onChange('zernio_tiktok_account_id', e.target.value)}
            placeholder="Zernio TikTok account ID..."
          />
        </div>
        <div className="flex items-center justify-between">
          <label className="text-sm">Content Preview Confirmed</label>
          <Switch
            checked={form['content_preview_confirmed'] === 'true'}
            onCheckedChange={v => onChange('content_preview_confirmed', v ? 'true' : 'false')}
          />
        </div>
        <div className="flex items-center justify-between">
          <label className="text-sm">Express Consent Given</label>
          <Switch
            checked={form['express_consent_given'] === 'true'}
            onCheckedChange={v => onChange('express_consent_given', v ? 'true' : 'false')}
          />
        </div>
        <div className="flex items-center gap-3 pt-2">
          <Button onClick={onSave} disabled={saving || !hasDirty}>
            {saving ? 'Saving...' : 'Save'}
          </Button>
          {saved && <span className="text-xs text-green-500">Saved</span>}
        </div>
      </CardContent>
    </Card>
  );
}

export default function SettingsPage() {
  const qc = useQueryClient();
  const { success, error: showError } = useToast();

  const { data: saved } = useQuery({ queryKey: ['settings'], queryFn: () => apiFetch<Record<string, string>>('/api/v1/settings') });
  const { data: agents } = useQuery({ queryKey: ['agents'], queryFn: () => apiFetch<Agent[]>('/api/v1/agents') });
  const { data: zernioData, isLoading: zernioLoading } = useQuery({
    queryKey: ['zernio-accounts'],
    queryFn: () => apiFetch<{ accounts: ZernioAccount[]; hasAnalyticsAccess: boolean }>('/api/v1/settings/test-zernio'),
    retry: false,
  });

  const [form, setForm] = useState<Record<string, string>>({});
  const [dirty, setDirty] = useState<Record<string, boolean>>({});

  useEffect(() => { if (saved) setForm(saved); }, [saved]);

  const saveMutation = useMutation({
    mutationFn: (data: Record<string, string>) =>
      apiFetch('/api/v1/settings', { method: 'PUT', body: JSON.stringify(data) }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['settings'] }); setDirty({}); success('บันทึกการตั้งค่าแล้ว'); },
    onError: (e) => showError(`บันทึกล้มเหลว: ${(e as Error).message}`),
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
    if (Object.keys(updates).length > 0) saveMutation.mutate(updates);
  };

  return (
    <div>
      <PageHeader title="Settings" />
      <div className="grid gap-6 max-w-2xl">
        <ApiKeysCard
          form={form}
          dirty={dirty}
          onSave={handleSave}
          saving={saveMutation.isPending}
          saved={saveMutation.isSuccess}
          onChange={handleChange}
        />
        <KieCreditsCard />
        <VoiceSettingsCard
          value={form['elevenlabs_voice'] ?? ''}
          onChange={handleChange}
        />
        <ContentSettingsCard
          form={form}
          dirty={dirty}
          onSave={handleSave}
          saving={saveMutation.isPending}
          saved={saveMutation.isSuccess}
          onChange={handleChange}
        />
        <ConnectedAccountsCard
          accounts={zernioData?.accounts}
          selectedId={saved?.zernio_youtube_account_id}
          loading={zernioLoading}
        />
        <AgentModelsCard agents={agents} />
      </div>
    </div>
  );
}
