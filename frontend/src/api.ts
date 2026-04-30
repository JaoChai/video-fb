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
