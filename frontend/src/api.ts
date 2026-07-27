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
  apiFetch<void>(`/api/v1/themes/${id}`, { method: 'PATCH', body: JSON.stringify(body) });
export const getPresets = () => apiFetch<PresetsResponse>('/api/v1/presets');

export interface PresetScore {
  preset: string;
  avg_retention: number;
  n: number;
}

export interface ClipCritique {
  clip_id: string;
  score: unknown;
  changes: unknown;
  applied: boolean;
  created_at: string;
}

export interface SkillRevision {
  agent_name: string;
  rationale: string;
  critique_window: number;
  created_at: string;
}

export const getPresetPerformance = () => apiFetch<PresetScore[]>('/api/v1/presets/performance');
export const getKieCredits = () => apiFetch<{ credits: number; error?: string }>('/api/v1/status/kie-credits');
export const getClipCritique = (id: string) => apiFetch<ClipCritique | null>(`/api/v1/clips/${id}/critique`);
export const getSkillRevisions = () => apiFetch<SkillRevision[]>('/api/v1/agents/skill-revisions');

export interface AutoReview {
  decision: string;
  confidence: number;
  defect_type: string;
  reasons: string[];
  created_at: string;
}

export const getClipAutoReview = (clipId: string) =>
  apiFetch<AutoReview | null>(`/api/v1/clips/${clipId}/auto-review`);

export interface FormulaScore {
  computed_at: string;
  dimension: string;
  value: string;
  platform: string;
  n: number;
  median_pct: number;
  median_retention: number;
  flop_rate: number;
  score_final: number;
}

export interface WeightRevision {
  dimension: string;
  value: string;
  old_weight: number;
  new_weight: number;
  score_final: number;
  n: number;
  computed_at: string;
  created_at: string;
}

export const getFormulaScores = () =>
  apiFetch<{ computed_at: string; scores: FormulaScore[] }>('/api/v1/formula-scores');
export const getWeightRevisions = () =>
  apiFetch<WeightRevision[]>('/api/v1/weight-revisions');
