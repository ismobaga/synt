const API_BASE = import.meta.env.VITE_API_URL || ''

export interface Project {
  id: string
  topic: string
  language: string
  platform: string
  duration_sec: number
  tone: string
  template_id: string
  status: string
  current_stage: string
  error_message?: string
  created_at: string
  updated_at: string
}

export interface CreateProjectInput {
  topic: string
  language: string
  platform: string
  duration_sec: number
  tone: string
  template_id: string
}

export interface Template {
  id: string
  name: string
  category: string
  config: Record<string, unknown>
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error || `HTTP ${res.status}`)
  }
  if (res.status === 204) return undefined as T
  return res.json()
}

export const api = {
  projects: {
    list: () => request<Project[]>('/v1/projects'),
    get: (id: string) => request<Project>(`/v1/projects/${id}`),
    create: (data: CreateProjectInput) =>
      request<{ id: string; status: string; current_stage: string }>('/v1/projects', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    delete: (id: string) => request<void>(`/v1/projects/${id}`, { method: 'DELETE' }),
    generate: (id: string, autoRender = true) =>
      request<{ status: string; current_stage: string }>(`/v1/projects/${id}/generate`, {
        method: 'POST',
        body: JSON.stringify({ auto_render: autoRender }),
      }),
    status: (id: string) =>
      request<{ id: string; status: string; current_stage: string; error_message?: string }>(`/v1/projects/${id}/status`),
    retry: (id: string) => request<void>(`/v1/projects/${id}/retry`, { method: 'POST' }),
    getScript: (id: string) => request<unknown>(`/v1/projects/${id}/script`),
    getAssets: (id: string) => request<unknown[]>(`/v1/projects/${id}/assets`),
    getAudio: (id: string) => request<unknown[]>(`/v1/projects/${id}/audio`),
    getSubtitles: (id: string) => request<unknown[]>(`/v1/projects/${id}/subtitles`),
    getRender: (id: string) => request<unknown[]>(`/v1/projects/${id}/render`),
    renderPreview: (id: string) => request<void>(`/v1/projects/${id}/render/preview`, { method: 'POST' }),
    renderFinal: (id: string) => request<void>(`/v1/projects/${id}/render/final`, { method: 'POST' }),
  },
  templates: {
    list: () => request<Template[]>('/v1/templates'),
  },
}
