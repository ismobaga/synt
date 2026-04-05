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

export interface ProjectStatusResponse {
  id: string
  status: string
  current_stage: string
  error_message?: string
  updated_at?: string
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

export interface ScriptRecord {
  id: string
  project_id: string
  title: string
  hook: string
  cta: string
  language: string
  content_json: unknown
  created_at: string
}

export interface AssetRecord {
  id: string
  project_id?: string
  type: string
  source: string
  provider: string
  url: string
  storage_path: string
  mime_type: string
  width: number
  height: number
  duration_sec: number
  license_info?: unknown
  metadata?: unknown
  created_at: string
}

export interface AudioTrackRecord {
  id: string
  project_id: string
  kind: string
  voice_name: string
  language: string
  storage_path: string
  duration_sec: number
  metadata?: unknown
  created_at: string
}

export interface SubtitleRecord {
  id: string
  project_id: string
  format: string
  storage_path: string
  content?: unknown
  created_at: string
}

export interface RenderRecord {
  id: string
  project_id: string
  kind: string
  resolution: string
  fps: number
  storage_path: string
  thumbnail_path: string
  status: string
  metadata?: unknown
  created_at: string
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

function decodeMaybeBase64JSON(value: unknown): unknown {
  if (value == null || value === '') return null
  if (typeof value !== 'string') return value

  const trimmed = value.trim()
  const parseMaybe = (input: string) => {
    try {
      return JSON.parse(input)
    } catch {
      return input
    }
  }

  if (trimmed.startsWith('{') || trimmed.startsWith('[')) {
    return parseMaybe(trimmed)
  }

  try {
    const binary = atob(trimmed)
    const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0))
    const decoded = new TextDecoder().decode(bytes)
    return parseMaybe(decoded)
  } catch {
    return trimmed
  }
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
    status: (id: string) => request<ProjectStatusResponse>(`/v1/projects/${id}/status`),
    retry: (id: string) => request<void>(`/v1/projects/${id}/retry`, { method: 'POST' }),
    getScript: (id: string) =>
      request<ScriptRecord>(`/v1/projects/${id}/script`).then((script) => ({
        ...script,
        content_json: decodeMaybeBase64JSON(script.content_json),
      })),
    getAssets: (id: string) =>
      request<AssetRecord[]>(`/v1/projects/${id}/assets`).then((assets) =>
        assets.map((asset) => ({
          ...asset,
          license_info: decodeMaybeBase64JSON(asset.license_info),
          metadata: decodeMaybeBase64JSON(asset.metadata),
        }))
      ),
    getAudio: (id: string) =>
      request<AudioTrackRecord[]>(`/v1/projects/${id}/audio`).then((tracks) =>
        tracks.map((track) => ({
          ...track,
          metadata: decodeMaybeBase64JSON(track.metadata),
        }))
      ),
    getSubtitles: (id: string) =>
      request<SubtitleRecord[]>(`/v1/projects/${id}/subtitles`).then((subtitles) =>
        subtitles.map((subtitle) => ({
          ...subtitle,
          content: decodeMaybeBase64JSON(subtitle.content),
        }))
      ),
    getRender: (id: string) =>
      request<RenderRecord[]>(`/v1/projects/${id}/render`).then((renders) =>
        renders.map((render) => ({
          ...render,
          metadata: decodeMaybeBase64JSON(render.metadata),
        }))
      ),
    renderPreview: (id: string) => request<void>(`/v1/projects/${id}/render/preview`, { method: 'POST' }),
    renderFinal: (id: string) => request<void>(`/v1/projects/${id}/render/final`, { method: 'POST' }),
  },
  templates: {
    list: () => request<Template[]>('/v1/templates'),
  },
}
