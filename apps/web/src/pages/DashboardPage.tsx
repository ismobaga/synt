import { useState, useEffect, useCallback, useMemo } from 'react'
import { api, type Project, type Template, type CreateProjectInput } from '../lib/api'
import { CreateProjectForm } from '../features/projects/CreateProjectForm'
import { ProjectCard } from '../features/projects/ProjectCard'

const STATUS_FILTERS = [
  { value: 'all', label: 'All' },
  { value: 'processing', label: 'Processing' },
  { value: 'queued', label: 'Queued' },
  { value: 'done', label: 'Complete' },
  { value: 'failed', label: 'Failed' },
  { value: 'draft', label: 'Draft' },
] as const

type StatusFilter = (typeof STATUS_FILTERS)[number]['value']

export function DashboardPage() {
  const [projects, setProjects] = useState<Project[]>([])
  const [templates, setTemplates] = useState<Template[]>([])
  const [loading, setLoading] = useState(true)
  const [creating, setCreating] = useState(false)
  const [generating, setGenerating] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [showForm, setShowForm] = useState(false)
  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all')

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

  const stats = useMemo(
    () => ({
      total: projects.length,
      active: projects.filter((project) => project.status === 'processing' || project.status === 'queued').length,
      complete: projects.filter((project) => project.status === 'done').length,
      failed: projects.filter((project) => project.status === 'failed').length,
    }),
    [projects]
  )

  const filteredProjects = useMemo(() => {
    const query = search.trim().toLowerCase()

    return [...projects]
      .sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime())
      .filter((project) => {
        if (statusFilter !== 'all' && project.status !== statusFilter) return false
        if (!query) return true

        return [project.topic, project.platform, project.tone, project.language, project.template_id]
          .join(' ')
          .toLowerCase()
          .includes(query)
      })
  }, [projects, search, statusFilter])

  const handleCreate = async (data: CreateProjectInput) => {
    setCreating(true)
    setError(null)
    try {
      const result = await api.projects.create(data)
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
    <div className="min-h-screen bg-slate-50">
      <header className="sticky top-0 z-10 border-b border-slate-200/80 bg-white/90 backdrop-blur">
        <div className="mx-auto flex max-w-6xl items-center justify-between px-4 py-4">
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-br from-violet-600 to-fuchsia-600 text-lg text-white shadow-sm">
              🎬
            </div>
            <div>
              <h1 className="text-lg font-bold text-slate-900 leading-none">Synt</h1>
              <p className="text-xs text-slate-500">AI short-video studio</p>
            </div>
          </div>
          <button
            onClick={() => setShowForm((value) => !value)}
            className="rounded-lg bg-violet-600 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-violet-700"
          >
            {showForm ? '✕ Close form' : '+ New Video'}
          </button>
        </div>
      </header>

      <main className="mx-auto max-w-6xl px-4 py-6 md:py-8">
        {error && (
          <div className="mb-4 flex items-center justify-between rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            <span>{error}</span>
            <button onClick={() => setError(null)} className="ml-2 text-red-500 hover:text-red-700">
              ✕
            </button>
          </div>
        )}

        <section className="mb-6 overflow-hidden rounded-3xl bg-gradient-to-br from-slate-950 via-violet-900 to-fuchsia-700 text-white shadow-xl">
          <div className="grid gap-6 px-5 py-6 lg:grid-cols-[1.3fr_0.9fr] lg:px-8 lg:py-8">
            <div>
              <span className="inline-flex rounded-full border border-white/15 bg-white/10 px-3 py-1 text-[11px] font-semibold tracking-wide text-violet-100">
                Prompt → voice → visuals → final render
              </span>
              <h2 className="mt-3 text-2xl font-bold tracking-tight sm:text-3xl">
                Turn a single topic into a ready-to-post short.
              </h2>
              <p className="mt-2 max-w-2xl text-sm text-violet-100/90 sm:text-base">
                Generate the script, stock media, narration, subtitles, music, and export pipeline in one place.
              </p>
              <div className="mt-4 flex flex-wrap gap-2">
                <button
                  onClick={() => setShowForm(true)}
                  className="rounded-lg bg-white px-4 py-2 text-sm font-semibold text-slate-900 transition hover:bg-violet-50"
                >
                  ✨ Start a new video
                </button>
                <button
                  onClick={() => setStatusFilter('processing')}
                  className="rounded-lg border border-white/20 bg-white/10 px-4 py-2 text-sm font-medium text-white transition hover:bg-white/15"
                >
                  View live jobs
                </button>
              </div>
            </div>

            <div className="grid gap-3 sm:grid-cols-2">
              {[
                { label: 'Projects', value: stats.total, tone: 'bg-white/10' },
                { label: 'Active now', value: stats.active, tone: 'bg-blue-500/20' },
                { label: 'Completed', value: stats.complete, tone: 'bg-emerald-500/20' },
                { label: 'Templates', value: templates.length, tone: 'bg-fuchsia-500/20' },
              ].map((item) => (
                <div key={item.label} className={`rounded-2xl border border-white/10 ${item.tone} p-4`}>
                  <p className="text-xs uppercase tracking-[0.2em] text-violet-100/80">{item.label}</p>
                  <p className="mt-2 text-2xl font-bold">{item.value}</p>
                </div>
              ))}
            </div>
          </div>
        </section>

        {showForm && (
          <div className="mb-8 rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
            <div className="mb-4 flex items-center justify-between gap-2">
              <div>
                <h2 className="text-base font-semibold text-slate-900">Create New Video</h2>
                <p className="text-sm text-slate-500">Pick a topic, style, and template to start the pipeline.</p>
              </div>
            </div>
            <CreateProjectForm templates={templates} onSubmit={handleCreate} loading={creating} />
          </div>
        )}

        {!loading && !showForm && (
          <section className="mb-6 rounded-2xl border border-slate-200 bg-white p-4 shadow-sm">
            <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
              <div>
                <h2 className="text-base font-semibold text-slate-900">Project workspace</h2>
                <p className="text-sm text-slate-500">Filter by status or search by topic, platform, language, or template.</p>
              </div>
              <div className="w-full lg:max-w-sm">
                <input
                  type="search"
                  value={search}
                  onChange={(event) => setSearch(event.target.value)}
                  placeholder="Search projects…"
                  className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-700 outline-none ring-0 transition focus:border-violet-500"
                />
              </div>
            </div>

            <div className="mt-3 flex flex-wrap gap-2">
              {STATUS_FILTERS.map((filter) => {
                const count = filter.value === 'all'
                  ? projects.length
                  : projects.filter((project) => project.status === filter.value).length

                return (
                  <button
                    key={filter.value}
                    onClick={() => setStatusFilter(filter.value)}
                    className={`rounded-full px-3 py-1.5 text-xs font-semibold transition ${statusFilter === filter.value
                        ? 'bg-violet-600 text-white'
                        : 'bg-slate-100 text-slate-600 hover:bg-slate-200'
                      }`}
                  >
                    {filter.label} ({count})
                  </button>
                )
              })}
            </div>
          </section>
        )}

        {!loading && projects.length === 0 && !showForm && (
          <div className="py-12 text-center">
            <div className="mb-4 text-6xl">🎬</div>
            <h2 className="mb-2 text-2xl font-bold text-slate-900">Enter a topic, get a publish-ready short video</h2>
            <p className="mx-auto mb-6 max-w-md text-slate-500">
              Automatically generates the script, visuals, voiceover, subtitles, music, and final HD vertical video.
            </p>
            <button
              onClick={() => setShowForm(true)}
              className="rounded-lg bg-violet-600 px-6 py-3 text-sm font-semibold text-white transition-colors hover:bg-violet-700"
            >
              🚀 Create Your First Video
            </button>
            <div className="mx-auto mt-10 grid max-w-3xl gap-4 text-left sm:grid-cols-2 lg:grid-cols-3">
              {[
                { icon: '✍️', label: 'Script', desc: 'Structured story beats and CTA' },
                { icon: '🎞️', label: 'Media', desc: 'Scene-by-scene stock footage' },
                { icon: '🎙️', label: 'Voice', desc: 'Narration with timing metadata' },
                { icon: '💬', label: 'Subtitles', desc: 'Caption text ready for burn-in' },
                { icon: '🎵', label: 'Music', desc: 'Background track selection' },
                { icon: '📱', label: 'Export', desc: 'Vertical MP4 for Shorts and Reels' },
              ].map((feature) => (
                <div key={feature.label} className="rounded-xl border border-slate-200 bg-white p-4 shadow-sm">
                  <div className="mb-1 text-lg">{feature.icon}</div>
                  <div className="text-sm font-semibold text-slate-900">{feature.label}</div>
                  <div className="text-xs text-slate-500">{feature.desc}</div>
                </div>
              ))}
            </div>
          </div>
        )}

        {loading ? (
          <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
            {[1, 2, 3].map((index) => (
              <div key={index} className="animate-pulse rounded-xl border border-slate-200 bg-white p-5">
                <div className="mb-2 h-4 w-3/4 rounded bg-slate-200" />
                <div className="h-3 w-1/2 rounded bg-slate-100" />
              </div>
            ))}
          </div>
        ) : filteredProjects.length > 0 ? (
          <div>
            <div className="mb-4 flex items-center justify-between gap-2">
              <h2 className="text-sm font-semibold uppercase tracking-wide text-slate-500">
                Projects ({filteredProjects.length})
              </h2>
              {stats.failed > 0 && <span className="text-xs text-red-500">{stats.failed} project(s) need attention</span>}
            </div>
            <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
              {filteredProjects.map((project) => (
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
        ) : projects.length > 0 ? (
          <div className="rounded-2xl border border-dashed border-slate-300 bg-white px-6 py-10 text-center shadow-sm">
            <p className="text-base font-semibold text-slate-900">No projects match this filter.</p>
            <p className="mt-1 text-sm text-slate-500">Try another status filter or search term.</p>
          </div>
        ) : null}
      </main>
    </div>
  )
}
