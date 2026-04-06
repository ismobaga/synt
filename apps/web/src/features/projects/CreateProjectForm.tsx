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

const RENDER_ENGINES = [
  {
    value: 'ffmpeg' as const,
    label: 'FFmpeg',
    description: 'Current stable render pipeline',
  },
  {
    value: 'remotion' as const,
    label: 'Remotion (React)',
    description: 'Programmatic React composition, experimental',
  },
]

const EXAMPLE_TOPICS = [
  '3 AI tools every small business should know',
  'Why sleep quality matters more than you think',
  'A punchy promo for a new finance app',
  '5 beginner mistakes when learning Python',
]

export function CreateProjectForm({ templates, onSubmit, loading }: CreateProjectFormProps) {
  const [form, setForm] = useState<CreateProjectInput>({
    topic: '',
    language: 'en',
    platform: 'youtube_shorts',
    duration_sec: 30,
    tone: 'educational',
    template_id: templates[0]?.id ?? 'fast_caption_v1',
    render_engine: 'ffmpeg',
    auto_render: false,
    source_notes: '',
  })
  const [sourceUrlsText, setSourceUrlsText] = useState('')

  const handleChange = (field: keyof CreateProjectInput, value: string | number | boolean) => {
    const normalizedValue = field === 'auto_render' ? value === true || value === 'true' : value
    setForm((prev) => ({ ...prev, [field]: normalizedValue }))
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!form.topic.trim()) return

    const sourceUrls = Array.from(
      new Set(
        sourceUrlsText
          .split(/\r?\n|,/)
          .map((value) => value.trim())
          .filter(Boolean)
      )
    )

    onSubmit({
      ...form,
      topic: form.topic.trim(),
      source_urls: sourceUrls,
      source_notes: form.source_notes?.trim() || undefined,
    })
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-5">
      <div>
        <label className="mb-1 block text-sm font-medium text-slate-700">
          Topic or creative brief <span className="text-red-500">*</span>
        </label>
        <textarea
          value={form.topic}
          onChange={(e) => handleChange('topic', e.target.value)}
          placeholder="e.g. Create a 30-second short explaining 5 AI tools that save small businesses time."
          rows={3}
          className="w-full rounded-xl border border-slate-300 px-3 py-2.5 text-sm text-slate-800 focus:border-violet-500 focus:outline-none focus:ring-1 focus:ring-violet-500"
          required
        />
        <p className="mt-1 text-xs text-slate-500">
          Be specific about the angle or audience to get better script, voice, and media choices.
        </p>
      </div>

      <div className="grid gap-4 lg:grid-cols-[1.1fr_0.9fr]">
        <div>
          <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-500">Quick ideas</p>
          <div className="flex flex-wrap gap-2">
            {EXAMPLE_TOPICS.map((topic) => (
              <button
                key={topic}
                type="button"
                onClick={() => handleChange('topic', topic)}
                className="rounded-full border border-slate-200 bg-slate-50 px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:border-violet-300 hover:bg-violet-50 hover:text-violet-700"
              >
                {topic}
              </button>
            ))}
          </div>
        </div>

        <div>
          <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-500">Render engine</p>
          <div className="grid gap-2 sm:grid-cols-2">
            {RENDER_ENGINES.map((engine) => {
              const selected = form.render_engine === engine.value
              return (
                <button
                  key={engine.value}
                  type="button"
                  onClick={() => handleChange('render_engine', engine.value)}
                  className={`rounded-xl border px-3 py-3 text-left transition ${selected
                    ? 'border-violet-500 bg-violet-50 text-violet-900'
                    : 'border-slate-200 bg-white text-slate-700 hover:border-violet-200 hover:bg-violet-50/50'
                    }`}
                >
                  <div className="text-sm font-semibold">{engine.label}</div>
                  <div className="mt-1 text-xs text-slate-500">{engine.description}</div>
                </button>
              )
            })}
          </div>
        </div>
      </div>

      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <div>
          <label className="mb-1 block text-sm font-medium text-slate-700">Platform</label>
          <select
            value={form.platform}
            onChange={(e) => handleChange('platform', e.target.value)}
            className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-violet-500 focus:outline-none"
          >
            {PLATFORMS.map((platform) => (
              <option key={platform.value} value={platform.value}>
                {platform.label}
              </option>
            ))}
          </select>
        </div>

        <div>
          <label className="mb-1 block text-sm font-medium text-slate-700">Duration</label>
          <select
            value={form.duration_sec}
            onChange={(e) => handleChange('duration_sec', parseInt(e.target.value, 10))}
            className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-violet-500 focus:outline-none"
          >
            {DURATIONS.map((duration) => (
              <option key={duration.value} value={duration.value}>
                {duration.label}
              </option>
            ))}
          </select>
        </div>

        <div>
          <label className="mb-1 block text-sm font-medium text-slate-700">Tone</label>
          <select
            value={form.tone}
            onChange={(e) => handleChange('tone', e.target.value)}
            className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-violet-500 focus:outline-none"
          >
            {TONES.map((tone) => (
              <option key={tone.value} value={tone.value}>
                {tone.label}
              </option>
            ))}
          </select>
        </div>

        <div>
          <label className="mb-1 block text-sm font-medium text-slate-700">Template</label>
          <select
            value={form.template_id}
            onChange={(e) => handleChange('template_id', e.target.value)}
            className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-violet-500 focus:outline-none"
          >
            {templates.map((template) => (
              <option key={template.id} value={template.id}>
                {template.name}
              </option>
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

      <div className="grid gap-4 lg:grid-cols-[1.1fr_0.9fr]">
        <div className="space-y-4">
          <div>
            <label className="mb-1 block text-sm font-medium text-slate-700">Source material URLs</label>
            <textarea
              value={sourceUrlsText}
              onChange={(e) => setSourceUrlsText(e.target.value)}
              rows={4}
              placeholder={'https://example.com/article\nhttps://youtu.be/your-video-id'}
              className="w-full rounded-xl border border-slate-300 px-3 py-2.5 text-sm text-slate-800 focus:border-violet-500 focus:outline-none focus:ring-1 focus:ring-violet-500"
            />
            <p className="mt-1 text-xs text-slate-500">
              Optional. Paste one article or YouTube URL per line. Webpage text and video transcripts are fetched before script generation.
            </p>
          </div>

          <div>
            <label className="mb-1 block text-sm font-medium text-slate-700">Reference notes</label>
            <textarea
              value={form.source_notes ?? ''}
              onChange={(e) => handleChange('source_notes', e.target.value)}
              rows={3}
              placeholder="Optional facts, angle, or points that must be included."
              className="w-full rounded-xl border border-slate-300 px-3 py-2.5 text-sm text-slate-800 focus:border-violet-500 focus:outline-none focus:ring-1 focus:ring-violet-500"
            />
          </div>
        </div>

        <div className="grid gap-3 md:grid-cols-[1fr_auto] md:items-end lg:grid-cols-1">
          <div>
            <label className="mb-1 block text-sm font-medium text-slate-700">Language</label>
            <select
              value={form.language}
              onChange={(e) => handleChange('language', e.target.value)}
              className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-violet-500 focus:outline-none"
            >
              <option value="en">English</option>
              <option value="es">Spanish</option>
              <option value="fr">French</option>
              <option value="de">German</option>
              <option value="pt">Portuguese</option>
            </select>
          </div>

          <div className="space-y-3">
            <label className="inline-flex items-start gap-2 rounded-xl border border-violet-100 bg-violet-50 px-4 py-3 text-xs text-violet-900">
              <input
                type="checkbox"
                checked={form.auto_render !== false}
                onChange={(e) => handleChange('auto_render', e.target.checked)}
                className="mt-0.5"
              />
              <span>
                <span className="block font-semibold">Auto render after generation</span>
                <span className="mt-1 block text-violet-700">Turn this off to pause in review mode so you can edit the script, scenes, and media before preview/final render.</span>
              </span>
            </label>

            <div className="rounded-xl border border-violet-100 bg-violet-50 px-4 py-3 text-xs text-violet-900">
              <div className="font-semibold">Current setup</div>
              <div className="mt-1">
                {form.duration_sec}s · {form.language.toUpperCase()} · {form.tone} · {(form.render_engine ?? 'ffmpeg') === 'remotion' ? 'Remotion' : 'FFmpeg'}
              </div>
              <div className="mt-1 text-violet-700">
                {sourceUrlsText.trim() ? `${sourceUrlsText.split(/\r?\n|,/).map((value) => value.trim()).filter(Boolean).length} source link(s)` : 'No sources attached yet'}
              </div>
              <div className="mt-1 text-violet-700">{form.auto_render !== false ? 'Auto-render on' : 'Review mode before render'}</div>
            </div>
          </div>
        </div>
      </div>

      {(form.render_engine ?? 'ffmpeg') === 'remotion' && (
        <div className="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-xs text-amber-900">
          <div className="font-semibold">Remotion mode enabled</div>
          <p className="mt-1">
            This keeps the normal content pipeline but tags the project for a React-based Remotion composition flow.
          </p>
        </div>
      )}

      <button
        type="submit"
        disabled={loading || !form.topic.trim()}
        className="w-full rounded-xl bg-violet-600 px-4 py-3 text-sm font-semibold text-white transition-colors hover:bg-violet-700 disabled:cursor-not-allowed disabled:opacity-50"
      >
        {loading ? 'Creating…' : '🚀 Generate Video'}
      </button>
    </form>
  )
}
