const API_BASE = import.meta.env.VITE_API_URL || 'https://adsvance-v2-production.up.railway.app';
const API_KEY = import.meta.env.VITE_API_KEY || '';

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

export async function apiFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(API_KEY && { Authorization: `Bearer ${API_KEY}` }),
      ...options?.headers,
    },
  });

  if (!res.ok) {
    const body = await res.json().catch(() => null);
    throw new ApiError(res.status, body?.error || res.statusText);
  }

  const json = await res.json();
  return json.data;
}

export const stopProduction = () => apiFetch('/api/v1/orchestrator/stop', { method: 'POST' });
export const publishTikTok = () => apiFetch('/api/v1/orchestrator/publish-tiktok', { method: 'POST' });

export interface BrandTheme {
  id: string;
  name: string;
  primary_color: string;
  secondary_color: string;
  accent_color: string;
  font_name: string;
  logo_url: string | null;
  mascot_description: string | null;
  image_style: string | null;
  active: boolean;
}

export interface PresetsResponse {
  presets: { key: string; display_name: string; primary_color: string; accent_color: string }[];
  style_presets_enabled: boolean;
  performance_enabled: boolean;
}

export const getActiveTheme = () => apiFetch<BrandTheme>('/api/v1/themes/active');
export const updateTheme = (id: string, body: Partial<BrandTheme>) =>
  apiFetch<BrandTheme>(`/api/v1/themes/${id}`, { method: 'PATCH', body: JSON.stringify(body) });
export const getPresets = () => apiFetch<PresetsResponse>('/api/v1/presets');
