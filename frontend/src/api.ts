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

export const getActiveTheme = () => apiFetch<BrandTheme>('/api/v1/themes/active');
export const updateTheme = (id: string, body: Partial<BrandTheme>) =>
  apiFetch<void>(`/api/v1/themes/${id}`, { method: 'PATCH', body: JSON.stringify(body) });

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

export interface ClipFull {
  id: string;
  title: string;
  question: string;
  questioner_name: string;
  answer_script: string;
  voice_script: string;
  category: string;
  status: string;
  video_16_9_url: string | null;
  video_9_16_url: string | null;
  thumbnail_url: string | null;
  publish_date: string | null;
  created_at: string;
  updated_at: string;
  fail_reason?: string;
  retry_count: number;
  review_retry_count: number;
  auto_review_held: boolean;
  style_preset: string;
  content_format: string;
  production_stage: string;
  case_number?: number;
  tutorial_feature: string;
}

export interface ClipMetadata {
  clip_id: string;
  youtube_title: string | null;
  youtube_description: string | null;
  youtube_tags: string[] | null;
  zernio_post_id: string | null;
  youtube_video_id: string | null;
  tiktok_post_id: string | null;
  ig_post_id: string | null;
  fb_post_id: string | null;
  zernio_shorts_post_id: string | null;
  zernio_tiktok_post_id: string | null;
}

export interface Scene {
  id: string;
  scene_number: number;
  scene_type: string;
  image_9_16_url: string | null;
  voice_text: string;
  duration_seconds: number;
  on_screen_text: string;
  beat: string;
  layout: string;
  caption_style: string;
}

export interface SceneVerdict {
  scene_number: number;
  ok: boolean;
  issues: string[];
}

export interface VisualQAResult {
  id: string;
  clip_id: string;
  passed: boolean;
  issues: SceneVerdict[];
  created_at: string;
}

export interface ClipAnalyticsRow {
  id: string;
  platform: string;
  post_type: string;
  views: number;
  likes: number;
  comments: number;
  shares: number;
  watch_time_seconds: number;
  retention_rate: number;
  fetched_at: string;
}

export interface ScriptDebate {
  id: string;
  source: string;
  candidates: unknown;
  verdict: unknown;
  created_at: string;
}

export interface ClipDetail {
  clip: ClipFull;
  metadata: ClipMetadata | null;
  scenes: Scene[];
  visual_qa: VisualQAResult | null;
  critique: ClipCritique | null;
  auto_review: AutoReview | null;
  analytics: ClipAnalyticsRow[];
  script_debate: ScriptDebate | null;
}

export const getClipDetail = (id: string) =>
  apiFetch<ClipDetail>(`/api/v1/clips/${id}/detail`);
