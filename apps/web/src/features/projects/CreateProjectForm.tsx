import { useState } from 'react'
import type { CreateProjectInput, Template } from '../../lib/api'

interface CreateProjectFormProps {
  templates: Template[]
  onSubmit: (data: CreateProjectInput) => void
  loading: boolean
}

const PLATFORMS = [
  { value: 'youtube_shorts', label: 'YouTube Shorts' },
  { value: 'tiktok', label: 'TikTok' },
  { value: 'instagram_reels', label: 'Instagram Reels' },
]

const TONES = [
  { value: 'educational', label: 'Educational' },
  { value: 'entertaining', label: 'Entertaining' },
  { value: 'inspirational', label: 'Inspirational' },
  { value: 'promotional', label: 'Promotional' },
  { value: 'professional', label: 'Professional' },
  { value: 'casual', label: 'Casual' },
]

const DURATIONS = [
  { value: 15, label: '15 seconds' },
  { value: 30, label: '30 seconds' },
  { value: 60, label: '60 seconds' },
]

export function CreateProjectForm({ templates, onSubmit, loading }: CreateProjectFormProps) {
  const [form, setForm] = useState<CreateProjectInput>({
    topic: '',
    language: 'en',
    platform: 'youtube_shorts',
    duration_sec: 30,
    tone: 'educational',
    template_id: templates[0]?.id ?? 'fast_caption_v1',
  })

  const handleChange = (field: keyof CreateProjectInput, value: string | number) => {
    setForm((prev) => ({ ...prev, [field]: value }))
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!form.topic.trim()) return
    onSubmit(form)
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div>
        <label className="block text-sm font-medium text-gray-700 mb-1">
          Topic or Keyword <span className="text-red-500">*</span>
        </label>
        <input
          type="text"
          value={form.topic}
          onChange={(e) => handleChange('topic', e.target.value)}
          placeholder="e.g. 5 AI tools for small businesses"
          className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-purple-500 focus:outline-none focus:ring-1 focus:ring-purple-500"
          required
        />
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Platform</label>
          <select
            value={form.platform}
            onChange={(e) => handleChange('platform', e.target.value)}
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-purple-500 focus:outline-none"
          >
            {PLATFORMS.map((p) => (
              <option key={p.value} value={p.value}>{p.label}</option>
            ))}
          </select>
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Duration</label>
          <select
            value={form.duration_sec}
            onChange={(e) => handleChange('duration_sec', parseInt(e.target.value))}
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-purple-500 focus:outline-none"
          >
            {DURATIONS.map((d) => (
              <option key={d.value} value={d.value}>{d.label}</option>
            ))}
          </select>
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Tone</label>
          <select
            value={form.tone}
            onChange={(e) => handleChange('tone', e.target.value)}
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-purple-500 focus:outline-none"
          >
            {TONES.map((t) => (
              <option key={t.value} value={t.value}>{t.label}</option>
            ))}
          </select>
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Template</label>
          <select
            value={form.template_id}
            onChange={(e) => handleChange('template_id', e.target.value)}
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-purple-500 focus:outline-none"
          >
            {templates.map((t) => (
              <option key={t.id} value={t.id}>{t.name}</option>
            ))}
            {templates.length === 0 && (
              <>
                <option value="fast_caption_v1">Fast Captions</option>
                <option value="minimal_clean_v1">Minimal Clean</option>
                <option value="promo_bold_v1">Promo Bold</option>
              </>
            )}
          </select>
        </div>
      </div>

      <div>
        <label className="block text-sm font-medium text-gray-700 mb-1">Language</label>
        <select
          value={form.language}
          onChange={(e) => handleChange('language', e.target.value)}
          className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-purple-500 focus:outline-none"
        >
          <option value="en">English</option>
          <option value="es">Spanish</option>
          <option value="fr">French</option>
          <option value="de">German</option>
          <option value="pt">Portuguese</option>
        </select>
      </div>

      <button
        type="submit"
        disabled={loading || !form.topic.trim()}
        className="w-full rounded-lg bg-purple-600 px-4 py-2.5 text-sm font-semibold text-white hover:bg-purple-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
      >
        {loading ? 'Creating…' : '🚀 Generate Video'}
      </button>
    </form>
  )
}
