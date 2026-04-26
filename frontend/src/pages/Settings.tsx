import { useState, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiFetch } from '../api';

interface TestResult {
  data?: {
    label?: string;
    limit_remaining?: number | null;
    is_free_tier?: boolean;
    usage_monthly?: number;
  };
  error?: string;
}

const FIELDS = [
  { key: 'openrouter_api_key', label: 'OpenRouter API Key', placeholder: 'sk-or-v1-...', secret: true, testable: true },
  { key: 'default_model', label: 'Default Model', placeholder: 'openai/gpt-5.5-pro', secret: false, testable: false },
  { key: 'kie_api_key', label: 'Kie AI API Key', placeholder: 'kie-...', secret: true, testable: false },
  { key: 'elevenlabs_voice', label: 'ElevenLabs Voice', placeholder: 'Adam', secret: false, testable: false },
  { key: 'zernio_api_key', label: 'Zernio API Key', placeholder: 'zrn-...', secret: true, testable: false },
];

export default function SettingsPage() {
  const qc = useQueryClient();
  const { data: saved } = useQuery({ queryKey: ['settings'], queryFn: () => apiFetch<Record<string, string>>('/api/v1/settings') });

  const [form, setForm] = useState<Record<string, string>>({});
  const [dirty, setDirty] = useState<Record<string, boolean>>({});
  const [testResult, setTestResult] = useState<TestResult | null>(null);
  const [showKeys, setShowKeys] = useState<Record<string, boolean>>({});

  useEffect(() => {
    if (saved) setForm(saved);
  }, [saved]);

  const save = useMutation({
    mutationFn: (data: Record<string, string>) =>
      apiFetch('/api/v1/settings', { method: 'PUT', body: JSON.stringify(data) }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['settings'] }); setDirty({}); },
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

  const hasDirty = Object.values(dirty).some(Boolean);

  return (
    <div>
      <h1 style={{ fontSize: 20, fontWeight: 600, marginBottom: 32 }}>Settings</h1>

      <div style={{ display: 'grid', gap: 16, maxWidth: 640 }}>
        {FIELDS.map(({ key, label, placeholder, secret, testable }) => (
          <div key={key}>
            <label style={{ display: 'block', fontSize: 12, color: '#555', marginBottom: 6, textTransform: 'uppercase', letterSpacing: '0.05em' }}>{label}</label>
            <div style={{ display: 'flex', gap: 8 }}>
              <input
                type={secret && !showKeys[key] ? 'password' : 'text'}
                value={form[key] ?? ''}
                placeholder={placeholder}
                onChange={e => handleChange(key, e.target.value)}
                style={{
                  flex: 1, padding: '10px 14px', borderRadius: 6,
                  border: '1px solid #222', background: '#111', color: '#fafafa',
                  fontSize: 14, outline: 'none', transition: 'border-color 0.15s',
                }}
                onFocus={e => (e.target.style.borderColor = '#444')}
                onBlur={e => (e.target.style.borderColor = '#222')}
              />
              {secret && (
                <button onClick={() => setShowKeys(prev => ({ ...prev, [key]: !prev[key] }))}
                  style={{
                    padding: '8px 14px', borderRadius: 6, border: '1px solid #222',
                    background: 'transparent', color: '#555', fontSize: 12, cursor: 'pointer',
                    transition: 'color 0.15s',
                  }}>{showKeys[key] ? 'Hide' : 'Show'}</button>
              )}
              {testable && (
                <button onClick={() => testKey.mutate(form[key] ?? '')}
                  disabled={testKey.isPending || !form[key]}
                  style={{
                    padding: '8px 16px', borderRadius: 6, border: '1px solid #222',
                    background: 'transparent', color: testKey.isPending ? '#333' : '#fafafa',
                    fontSize: 12, cursor: testKey.isPending || !form[key] ? 'default' : 'pointer',
                    opacity: !form[key] ? 0.3 : 1, transition: 'all 0.15s',
                  }}>
                  {testKey.isPending ? 'Testing...' : 'Test'}
                </button>
              )}
            </div>

            {testable && testResult && (
              <div style={{
                marginTop: 8, padding: '8px 12px', borderRadius: 6, fontSize: 12,
                background: testResult.error ? 'rgba(239,68,68,0.1)' : 'rgba(34,197,94,0.1)',
                color: testResult.error ? '#ef4444' : '#22c55e',
                border: `1px solid ${testResult.error ? 'rgba(239,68,68,0.2)' : 'rgba(34,197,94,0.2)'}`,
              }}>
                {testResult.error ? `Failed: ${testResult.error}` : (
                  <span style={{ display: 'flex', gap: 16 }}>
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

        <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginTop: 8 }}>
          <button onClick={handleSave} disabled={save.isPending || !hasDirty}
            style={{
              padding: '10px 28px', borderRadius: 6, border: 'none',
              background: hasDirty ? '#fff' : '#222', color: hasDirty ? '#000' : '#555',
              fontSize: 14, fontWeight: 600, cursor: hasDirty ? 'pointer' : 'default',
              transition: 'all 0.15s',
            }}>
            {save.isPending ? 'Saving...' : 'Save'}
          </button>
          {save.isSuccess && <span style={{ fontSize: 12, color: '#22c55e' }}>Saved</span>}
        </div>
      </div>
    </div>
  );
}
