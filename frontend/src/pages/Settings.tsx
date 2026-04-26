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

const FIELD_CONFIG = [
  { key: 'openrouter_api_key', label: 'OpenRouter API Key', placeholder: 'sk-or-v1-...', type: 'password' as const, testable: true },
  { key: 'default_model', label: 'Default Model', placeholder: 'openai/gpt-5.5-pro', type: 'text' as const, testable: false },
  { key: 'kie_api_key', label: 'Kie AI API Key', placeholder: 'kie-...', type: 'password' as const, testable: false },
  { key: 'elevenlabs_voice', label: 'ElevenLabs Voice', placeholder: 'Adam', type: 'text' as const, testable: false },
  { key: 'zernio_api_key', label: 'Zernio API Key', placeholder: 'zrn-...', type: 'password' as const, testable: false },
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
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['settings'] });
      setDirty({});
    },
  });

  const testKey = useMutation({
    mutationFn: (key: string) =>
      apiFetch<TestResult>('/api/v1/settings/test-key', {
        method: 'POST',
        body: JSON.stringify({ key }),
      }),
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

  const cardStyle: React.CSSProperties = {
    background: '#1e293b', borderRadius: 12, padding: 24, marginBottom: 16,
  };
  const inputStyle: React.CSSProperties = {
    width: '100%', padding: '10px 14px', borderRadius: 8, border: '1px solid #334155',
    background: '#0f172a', color: '#fff', fontSize: 14, outline: 'none', boxSizing: 'border-box',
  };
  const labelStyle: React.CSSProperties = {
    fontSize: 13, color: '#94a3b8', marginBottom: 6, display: 'block',
  };
  const btnStyle: React.CSSProperties = {
    padding: '8px 20px', borderRadius: 8, border: 'none', cursor: 'pointer', fontSize: 14,
  };

  return (
    <div>
      <h1 style={{ fontSize: 24, marginBottom: 24 }}>Settings</h1>

      {FIELD_CONFIG.map(({ key, label, placeholder, type, testable }) => (
        <div key={key} style={cardStyle}>
          <label style={labelStyle}>{label}</label>
          <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
            <input
              type={type === 'password' && !showKeys[key] ? 'password' : 'text'}
              value={form[key] ?? ''}
              placeholder={placeholder}
              onChange={e => handleChange(key, e.target.value)}
              style={{ ...inputStyle, flex: 1 }}
            />
            {type === 'password' && (
              <button
                onClick={() => setShowKeys(prev => ({ ...prev, [key]: !prev[key] }))}
                style={{ ...btnStyle, background: '#334155', color: '#94a3b8', minWidth: 70 }}
              >
                {showKeys[key] ? 'Hide' : 'Show'}
              </button>
            )}
            {testable && (
              <button
                onClick={() => testKey.mutate(form[key] ?? '')}
                disabled={testKey.isPending || !form[key]}
                style={{
                  ...btnStyle,
                  background: testKey.isPending ? '#475569' : '#3b82f6',
                  color: '#fff', minWidth: 120,
                }}
              >
                {testKey.isPending ? 'Testing...' : 'Test Connection'}
              </button>
            )}
          </div>

          {testable && testResult && (
            <div style={{
              marginTop: 12, padding: 12, borderRadius: 8, fontSize: 13,
              background: testResult.error ? '#450a0a' : '#052e16',
              color: testResult.error ? '#fca5a5' : '#86efac',
            }}>
              {testResult.error ? (
                <span>Failed: {testResult.error}</span>
              ) : (
                <div style={{ display: 'flex', gap: 24, flexWrap: 'wrap' }}>
                  <span>Connected!</span>
                  {testResult.data?.label && <span>Label: {testResult.data.label}</span>}
                  {testResult.data?.limit_remaining != null && (
                    <span>Credits: {testResult.data.limit_remaining.toLocaleString()}</span>
                  )}
                  {testResult.data?.usage_monthly != null && (
                    <span>Monthly Usage: ${testResult.data.usage_monthly.toFixed(4)}</span>
                  )}
                  <span>Tier: {testResult.data?.is_free_tier ? 'Free' : 'Paid'}</span>
                </div>
              )}
            </div>
          )}
        </div>
      ))}

      <div style={{ display: 'flex', gap: 12, marginTop: 8 }}>
        <button
          onClick={handleSave}
          disabled={save.isPending || Object.keys(dirty).length === 0}
          style={{
            ...btnStyle,
            background: Object.keys(dirty).length === 0 ? '#475569' : '#f5851f',
            color: '#fff', padding: '12px 32px', fontSize: 16,
          }}
        >
          {save.isPending ? 'Saving...' : 'Save Settings'}
        </button>
        {save.isSuccess && (
          <span style={{ color: '#86efac', fontSize: 14, alignSelf: 'center' }}>Saved!</span>
        )}
      </div>
    </div>
  );
}
