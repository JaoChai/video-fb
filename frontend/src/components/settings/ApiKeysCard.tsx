import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { apiFetch } from '../../api';
import { Button } from '../ui/button';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '../ui/card';
import { Input } from '../ui/input';

interface TestResult {
  data?: {
    label?: string;
    limit_remaining?: number | null;
    is_free_tier?: boolean;
    usage_monthly?: number;
  };
  error?: string;
}

const API_KEY_FIELDS = [
  { key: 'openrouter_api_key', label: 'OpenRouter API Key', placeholder: 'sk-or-v1-...', testable: true },
  { key: 'kie_api_key', label: 'Kie AI API Key (Upload)', placeholder: 'kie-...', testable: false },
  { key: 'zernio_api_key', label: 'Zernio API Key', placeholder: 'zrn-...', testable: false },
];

interface ApiKeysCardProps {
  form: Record<string, string>;
  dirty: Record<string, boolean>;
  onSave: () => void;
  saving: boolean;
  saved: boolean;
  onChange: (key: string, value: string) => void;
}

export function ApiKeysCard({ form, dirty, onSave, saving, saved, onChange }: ApiKeysCardProps) {
  const [showKeys, setShowKeys] = useState<Record<string, boolean>>({});
  const [testResult, setTestResult] = useState<TestResult | null>(null);

  const testKey = useMutation({
    mutationFn: (key: string) =>
      apiFetch<TestResult>('/api/v1/settings/test-key', { method: 'POST', body: JSON.stringify({ key }) }),
    onSuccess: (data) => setTestResult(data as unknown as TestResult),
    onError: (err) => setTestResult({ error: (err as Error).message }),
  });

  const hasDirty = Object.values(dirty).some(Boolean);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">API Keys</CardTitle>
        <CardDescription>Configure API keys for external services</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {API_KEY_FIELDS.map(({ key, label, placeholder, testable }) => (
          <div key={key}>
            <label className="block text-xs text-muted-foreground uppercase tracking-wide mb-1.5">
              {label}
            </label>
            <div className="flex gap-2">
              <Input
                type={showKeys[key] ? 'text' : 'password'}
                value={form[key] ?? ''}
                placeholder={placeholder}
                onChange={e => onChange(key, e.target.value)}
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
          <Button onClick={onSave} disabled={saving || !hasDirty}>
            {saving ? 'Saving...' : 'Save'}
          </Button>
          {saved && <span className="text-xs text-green-500">Saved</span>}
        </div>
      </CardContent>
    </Card>
  );
}
