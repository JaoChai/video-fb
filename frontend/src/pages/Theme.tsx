import { useState, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { getActiveTheme, updateTheme, getPresets, type BrandTheme } from '../api';
import { PageHeader } from '../components/page-header';
import { useToast } from '../components/ui/toaster';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '../components/ui/card';
import { Input } from '../components/ui/input';
import { Textarea } from '../components/ui/textarea';
import { Button } from '../components/ui/button';

type ThemeForm = Omit<BrandTheme, 'id'>;

function ColorField({
  label,
  value,
  onChange,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
}) {
  return (
    <div>
      <label className="block text-xs text-muted-foreground uppercase tracking-wide mb-1.5">
        {label}
      </label>
      <div className="flex items-center gap-2">
        <input
          type="color"
          value={value}
          onChange={e => onChange(e.target.value)}
          className="h-10 w-12 cursor-pointer rounded border border-input bg-background p-1"
        />
        <Input
          value={value}
          onChange={e => onChange(e.target.value)}
          placeholder="#000000"
          className="font-mono w-32"
        />
      </div>
    </div>
  );
}

function StatusBadge({ label, enabled }: { label: string; enabled: boolean }) {
  return (
    <div
      className={`inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-medium border ${
        enabled
          ? 'bg-green-500/10 text-green-600 border-green-500/30 dark:text-green-400'
          : 'bg-muted text-muted-foreground border-border'
      }`}
    >
      <span>{label}</span>
      <span className="font-semibold">{enabled ? 'ON' : 'OFF'}</span>
    </div>
  );
}

const EMPTY_FORM: ThemeForm = {
  name: '',
  primary_color: '#000000',
  secondary_color: '#000000',
  accent_color: '#000000',
  font_name: '',
  logo_url: null,
  mascot_description: null,
  image_style: null,
  active: true,
};

export default function ThemePage() {
  const qc = useQueryClient();
  const { success, error: showError } = useToast();

  const { data: theme } = useQuery({ queryKey: ['theme-active'], queryFn: getActiveTheme });
  const { data: presetsData } = useQuery({ queryKey: ['presets'], queryFn: getPresets });

  const [form, setForm] = useState<ThemeForm>(EMPTY_FORM);
  const [dirty, setDirty] = useState(false);

  useEffect(() => {
    if (theme) {
      setForm({
        name: theme.name,
        primary_color: theme.primary_color,
        secondary_color: theme.secondary_color,
        accent_color: theme.accent_color,
        font_name: theme.font_name,
        logo_url: theme.logo_url,
        mascot_description: theme.mascot_description,
        image_style: theme.image_style,
        active: theme.active,
      });
      setDirty(false);
    }
  }, [theme]);

  const saveMutation = useMutation({
    mutationFn: (body: ThemeForm) => updateTheme(theme!.id, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['theme-active'] });
      setDirty(false);
      success('บันทึก Theme แล้ว');
    },
    onError: (e) => showError(`บันทึกล้มเหลว: ${(e as Error).message}`),
  });

  const setField = <K extends keyof ThemeForm>(key: K, value: ThemeForm[K]) => {
    setForm(prev => ({ ...prev, [key]: value }));
    setDirty(true);
  };

  return (
    <div>
      <PageHeader title="Theme" description="Brand colors, fonts, and visual identity" />
      <div className="grid gap-6 max-w-2xl">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Brand Theme</CardTitle>
            <CardDescription>Edit the active theme settings</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div>
              <label className="block text-xs text-muted-foreground uppercase tracking-wide mb-1.5">
                Name
              </label>
              <Input
                value={form.name}
                onChange={e => setField('name', e.target.value)}
                placeholder="Theme name"
              />
            </div>

            <ColorField
              label="Primary Color"
              value={form.primary_color}
              onChange={v => setField('primary_color', v)}
            />
            <ColorField
              label="Secondary Color"
              value={form.secondary_color}
              onChange={v => setField('secondary_color', v)}
            />
            <ColorField
              label="Accent Color"
              value={form.accent_color}
              onChange={v => setField('accent_color', v)}
            />

            <div>
              <label className="block text-xs text-muted-foreground uppercase tracking-wide mb-1.5">
                Font Name
              </label>
              <Input
                value={form.font_name}
                onChange={e => setField('font_name', e.target.value)}
                placeholder="e.g. Noto Sans Thai"
              />
            </div>

            <div>
              <label className="block text-xs text-muted-foreground uppercase tracking-wide mb-1.5">
                Logo URL
              </label>
              <Input
                value={form.logo_url ?? ''}
                onChange={e => setField('logo_url', e.target.value || null)}
                placeholder="https://..."
              />
            </div>

            <div>
              <label className="block text-xs text-muted-foreground uppercase tracking-wide mb-1.5">
                Mascot Description
              </label>
              <Textarea
                value={form.mascot_description ?? ''}
                onChange={e => setField('mascot_description', e.target.value || null)}
                placeholder="Describe the mascot..."
              />
            </div>

            <div>
              <label className="block text-xs text-muted-foreground uppercase tracking-wide mb-1.5">
                Image Style
              </label>
              <Textarea
                value={form.image_style ?? ''}
                onChange={e => setField('image_style', e.target.value || null)}
                placeholder="Describe the image style..."
              />
            </div>

            <div className="flex items-center gap-3 pt-2">
              <Button
                onClick={() => saveMutation.mutate(form)}
                disabled={saveMutation.isPending || !dirty || !theme}
              >
                {saveMutation.isPending ? 'Saving...' : 'Save'}
              </Button>
              {saveMutation.isSuccess && (
                <span className="text-xs text-green-500">Saved</span>
              )}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">Style Presets</CardTitle>
            <CardDescription>Available visual style presets (read-only)</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex flex-wrap gap-2">
              {presetsData?.presets.map(preset => (
                <div
                  key={preset.key}
                  className="flex items-center gap-2 rounded-full border px-3 py-1.5 text-sm"
                >
                  <span
                    className="h-4 w-4 rounded-sm border"
                    style={{ backgroundColor: preset.primary_color }}
                  />
                  <span
                    className="h-4 w-4 rounded-sm border"
                    style={{ backgroundColor: preset.accent_color }}
                  />
                  <span>{preset.display_name}</span>
                </div>
              ))}
            </div>
            <div className="flex gap-3 flex-wrap">
              <StatusBadge
                label="Style Presets"
                enabled={presetsData?.style_presets_enabled ?? false}
              />
              <StatusBadge
                label="Performance"
                enabled={presetsData?.performance_enabled ?? false}
              />
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
