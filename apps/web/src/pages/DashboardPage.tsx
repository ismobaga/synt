import { useState, useEffect, useCallback } from 'react'
import { api, type Project, type Template, type CreateProjectInput } from '../lib/api'
import { CreateProjectForm } from '../features/projects/CreateProjectForm'
import { ProjectCard } from '../features/projects/ProjectCard'

export function DashboardPage() {
  const [projects, setProjects] = useState<Project[]>([])
  const [templates, setTemplates] = useState<Template[]>([])
  const [loading, setLoading] = useState(true)
  const [creating, setCreating] = useState(false)
  const [generating, setGenerating] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [showForm, setShowForm] = useState(false)

  const loadData = useCallback(async () => {
    try {
      const [projs, tmpls] = await Promise.all([
        api.projects.list().catch(() => []),
        api.templates.list().catch(() => []),
      ])
      setProjects(projs ?? [])
      setTemplates(tmpls ?? [])
    } catch {
      // graceful degradation
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    loadData()
  }, [loadData])

  const handleCreate = async (data: CreateProjectInput) => {
    setCreating(true)
    setError(null)
    try {
      const result = await api.projects.create(data)
      // Auto-trigger generation
      await api.projects.generate(result.id, true)
      await loadData()
      setShowForm(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create project')
    } finally {
      setCreating(false)
    }
  }

  const handleGenerate = async (id: string) => {
    setGenerating(id)
    try {
      await api.projects.generate(id, true)
      await loadData()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to generate')
    } finally {
      setGenerating(null)
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('Delete this project?')) return
    try {
      await api.projects.delete(id)
      setProjects((prev) => prev.filter((p) => p.id !== id))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete')
    }
  }

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <header className="bg-white border-b border-gray-200 sticky top-0 z-10">
        <div className="max-w-5xl mx-auto px-4 py-4 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <span className="text-2xl">🎬</span>
            <div>
              <h1 className="text-lg font-bold text-gray-900 leading-none">Synt</h1>
              <p className="text-xs text-gray-500">AI Short Video Generator</p>
            </div>
          </div>
          <button
            onClick={() => setShowForm((v) => !v)}
            className="rounded-lg bg-purple-600 px-4 py-2 text-sm font-semibold text-white hover:bg-purple-700 transition-colors"
          >
            {showForm ? '✕ Cancel' : '+ New Video'}
          </button>
        </div>
      </header>

      <main className="max-w-5xl mx-auto px-4 py-8">
        {/* Error Banner */}
        {error && (
          <div className="mb-4 rounded-lg bg-red-50 border border-red-200 px-4 py-3 text-sm text-red-700 flex items-center justify-between">
            <span>{error}</span>
            <button onClick={() => setError(null)} className="ml-2 text-red-500 hover:text-red-700">✕</button>
          </div>
        )}

        {/* Create Form */}
        {showForm && (
          <div className="mb-8 rounded-xl border border-gray-200 bg-white p-6 shadow-sm">
            <h2 className="text-base font-semibold text-gray-900 mb-4">Create New Video</h2>
            <CreateProjectForm
              templates={templates}
              onSubmit={handleCreate}
              loading={creating}
            />
          </div>
        )}

        {/* Hero when empty */}
        {!loading && projects.length === 0 && !showForm && (
          <div className="text-center py-20">
            <div className="text-6xl mb-4">🎬</div>
            <h2 className="text-2xl font-bold text-gray-900 mb-2">
              Enter a topic, get a publish-ready short video
            </h2>
            <p className="text-gray-500 mb-6 max-w-md mx-auto">
              Automatically generates script, selects visuals, adds voiceover, subtitles, music,
              and renders your final HD vertical video.
            </p>
            <button
              onClick={() => setShowForm(true)}
              className="rounded-lg bg-purple-600 px-6 py-3 text-sm font-semibold text-white hover:bg-purple-700 transition-colors"
            >
              🚀 Create Your First Video
            </button>
            <div className="mt-10 grid grid-cols-3 gap-4 max-w-lg mx-auto text-left">
              {[
                { icon: '✍️', label: 'Script', desc: 'AI generates structured JSON script' },
                { icon: '🎞️', label: 'Media', desc: 'Stock footage selected per scene' },
                { icon: '🎙️', label: 'Voice', desc: 'TTS narration with timing data' },
                { icon: '💬', label: 'Subtitles', desc: 'Phrase-based animated captions' },
                { icon: '🎵', label: 'Music', desc: 'Licensed background music' },
                { icon: '📱', label: 'Export', desc: '1080×1920 MP4 for TikTok/Reels' },
              ].map((f) => (
                <div key={f.label} className="rounded-lg border border-gray-100 bg-white p-3">
                  <div className="text-lg mb-1">{f.icon}</div>
                  <div className="text-xs font-semibold text-gray-900">{f.label}</div>
                  <div className="text-xs text-gray-500">{f.desc}</div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Projects Grid */}
        {loading ? (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {[1, 2, 3].map((i) => (
              <div key={i} className="rounded-xl border border-gray-200 bg-white p-5 animate-pulse">
                <div className="h-4 bg-gray-200 rounded mb-2 w-3/4" />
                <div className="h-3 bg-gray-100 rounded w-1/2" />
              </div>
            ))}
          </div>
        ) : projects.length > 0 ? (
          <div>
            <h2 className="text-sm font-semibold text-gray-500 uppercase tracking-wide mb-4">
              Your Projects ({projects.length})
            </h2>
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
              {projects.map((project) => (
                <ProjectCard
                  key={project.id}
                  project={project}
                  onGenerate={handleGenerate}
                  onDelete={handleDelete}
                  isGenerating={generating === project.id}
                />
              ))}
            </div>
          </div>
        ) : null}
      </main>
    </div>
  )
}
