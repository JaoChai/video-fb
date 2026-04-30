import { useState, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiFetch } from '../api';
import { PageHeader } from '../components/page-header';
import { useToast } from '../components/ui/toaster';
import { ApiKeysCard } from '../components/settings/ApiKeysCard';
import { VoiceSettingsCard } from '../components/settings/VoiceSettingsCard';
import { ConnectedAccountsCard } from '../components/settings/ConnectedAccountsCard';
import { AgentModelsCard } from '../components/settings/AgentModelsCard';

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
        <VoiceSettingsCard
          value={form['elevenlabs_voice'] ?? ''}
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
