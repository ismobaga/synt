import { useState, useEffect, useCallback, useMemo } from 'react'
import { StatusBadge } from '../components/ui/StatusBadge'
import { CreateProjectForm } from '../features/projects/CreateProjectForm'
import { ProjectCard } from '../features/projects/ProjectCard'
import { api, type CreateProjectInput, type Project, type Template } from '../lib/api'

const STATUS_FILTERS = [
  { value: 'all', label: 'All' },
  { value: 'processing', label: 'Processing' },
  { value: 'queued', label: 'Queued' },
  { value: 'done', label: 'Complete' },
  { value: 'failed', label: 'Failed' },
  { value: 'draft', label: 'Draft' },
] as const

const APP_VIEWS = [
  { value: 'overview', label: 'Overview', emoji: '🏠' },
  { value: 'create', label: 'Create', emoji: '✨' },
  { value: 'projects', label: 'Projects', emoji: '🎬' },
  { value: 'workspace', label: 'Workspace', emoji: '🛠️' },
] as const

type StatusFilter = (typeof STATUS_FILTERS)[number]['value']
type AppView = (typeof APP_VIEWS)[number]['value']
type AppRoute = { view: AppView; projectId?: string }

function formatRelativeTime(value?: string) {
  if (!value) return 'unknown'
  const timestamp = new Date(value).getTime()
  if (Number.isNaN(timestamp)) return 'unknown'
  const diffSeconds = Math.max(0, Math.floor((Date.now() - timestamp) / 1000))
  if (diffSeconds < 60) return 'just now'
  if (diffSeconds < 3600) return `${Math.floor(diffSeconds / 60)}m ago`
  if (diffSeconds < 86400) return `${Math.floor(diffSeconds / 3600)}h ago`
  return `${Math.floor(diffSeconds / 86400)}d ago`
}

function getRouteFromHash(): AppRoute {
  if (typeof window === 'undefined') return { view: 'overview' }
  const hash = window.location.hash.replace(/^#/, '').replace(/^\//, '')
  if (!hash) return { view: 'overview' }

  const parts = hash.split('/').filter(Boolean)
  if (parts[0] === 'create') return { view: 'create' }
  if (parts[0] === 'projects' && parts[1]) return { view: 'workspace', projectId: parts[1] }
  if (parts[0] === 'projects') return { view: 'projects' }
  if (parts[0] === 'workspace' && parts[1]) return { view: 'workspace', projectId: parts[1] }
  return { view: 'overview' }
}

function routeToHash(route: AppRoute) {
  switch (route.view) {
    case 'create':
      return '#/create'
    case 'projects':
      return '#/projects'
    case 'workspace':
      return route.projectId ? `#/projects/${route.projectId}` : '#/projects'
    default:
      return '#/overview'
  }
}

function MetricCard({ label, value, hint }: { label: string; value: string | number; hint: string }) {
  return (
    <div className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm">
      <p className="text-[11px] font-semibold uppercase tracking-[0.2em] text-slate-500">{label}</p>
      <p className="mt-2 text-2xl font-bold text-slate-900">{value}</p>
      <p className="mt-1 text-xs text-slate-500">{hint}</p>
    </div>
  )
}

function ProjectQuickCard({
  project,
  onOpen,
  onGenerate,
  onDelete,
  isGenerating,
}: {
  project: Project
  onOpen: (id: string) => void
  onGenerate: (id: string) => void
  onDelete: (id: string) => void
  isGenerating: boolean
}) {
  const isActive = project.status === 'processing' || project.status === 'queued'
  const renderEngine = project.template_id.toLowerCase().startsWith('remotion_') ? 'Remotion' : 'FFmpeg'

  return (
    <article className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm transition hover:-translate-y-0.5 hover:shadow-md">
      <div className="mb-3 flex items-start justify-between gap-3">
        <div>
          <h3 className="text-sm font-semibold text-slate-900 line-clamp-2">{project.topic}</h3>
          <p className="mt-1 text-xs text-slate-500">
            {project.platform.replace('_', ' ')} · {project.duration_sec}s · {project.language.toUpperCase()}
          </p>
        </div>
        <StatusBadge status={project.status} />
      </div>

      <div className="mb-3 flex flex-wrap gap-1.5 text-[11px] text-slate-600">
        <span className="rounded-full bg-slate-100 px-2 py-1">{project.tone}</span>
        <span className="rounded-full bg-violet-50 px-2 py-1 text-violet-700">{renderEngine}</span>
        <span className="rounded-full bg-slate-100 px-2 py-1">{project.current_stage || 'created'}</span>
      </div>

      <div className="mb-4 rounded-xl bg-slate-50 px-3 py-2 text-xs text-slate-600">
        Updated {formatRelativeTime(project.updated_at)}
        {project.error_message ? <div className="mt-1 text-red-600 line-clamp-2">{project.error_message}</div> : null}
      </div>

      <div className="flex flex-wrap gap-2">
        <button
          onClick={() => onOpen(project.id)}
          className="rounded-lg bg-violet-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-violet-700"
        >
          Open workspace
        </button>
        {(project.status === 'draft' || project.status === 'failed') && (
          <button
            onClick={() => onGenerate(project.id)}
            disabled={isGenerating}
            className="rounded-lg border border-slate-200 px-3 py-1.5 text-xs font-medium text-slate-700 hover:bg-slate-50 disabled:opacity-50"
          >
            {isGenerating ? 'Starting…' : isActive ? 'Resume' : 'Generate'}
          </button>
        )}
        <button
          onClick={() => onDelete(project.id)}
          className="rounded-lg border border-slate-200 px-3 py-1.5 text-xs font-medium text-slate-600 hover:bg-slate-50"
        >
          Delete
        </button>
      </div>
    </article>
  )
}

export function DashboardPage() {
  const [projects, setProjects] = useState<Project[]>([])
  const [templates, setTemplates] = useState<Template[]>([])
  const [loading, setLoading] = useState(true)
  const [creating, setCreating] = useState(false)
  const [generating, setGenerating] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all')
  const [route, setRoute] = useState<AppRoute>(() => getRouteFromHash())

  const navigate = useCallback((nextRoute: AppRoute) => {
    if (typeof window !== 'undefined') {
      window.location.hash = routeToHash(nextRoute)
    }
    setRoute(nextRoute)
  }, [])

  useEffect(() => {
    const syncRoute = () => setRoute(getRouteFromHash())
    if (typeof window !== 'undefined' && !window.location.hash) {
      window.location.hash = routeToHash({ view: 'overview' })
    }
    window.addEventListener('hashchange', syncRoute)
    return () => window.removeEventListener('hashchange', syncRoute)
  }, [])

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

  const selectedProject = useMemo(() => {
    if (route.projectId) {
      return projects.find((project) => project.id === route.projectId) ?? null
    }
    return filteredProjects[0] ?? projects[0] ?? null
  }, [projects, filteredProjects, route.projectId])

  const recentProjects = useMemo(() => filteredProjects.slice(0, 4), [filteredProjects])
  const needsAttention = useMemo(
    () => projects.filter((project) => project.status === 'failed' || project.status === 'processing' || project.status === 'queued').slice(0, 5),
    [projects]
  )

  const handleCreate = async (data: CreateProjectInput) => {
    setCreating(true)
    setError(null)
    try {
      const result = await api.projects.create(data)
      await api.projects.generate(result.id, data.auto_render !== false)
      await loadData()
      navigate({ view: 'workspace', projectId: result.id })
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
      navigate({ view: 'workspace', projectId: id })
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
      if (route.projectId === id) {
        navigate({ view: 'projects' })
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete')
    }
  }

  const renderOverview = () => (
    <div className="space-y-5">
      <section className="overflow-hidden rounded-3xl bg-gradient-to-br from-slate-950 via-violet-900 to-fuchsia-700 text-white shadow-xl">
        <div className="grid gap-6 px-5 py-6 xl:grid-cols-[1.15fr_0.85fr] xl:px-8 xl:py-8">
          <div>
            <span className="inline-flex rounded-full border border-white/15 bg-white/10 px-3 py-1 text-[11px] font-semibold tracking-wide text-violet-100">
              Multi-page studio workflow
            </span>
            <h2 className="mt-3 text-2xl font-bold tracking-tight sm:text-3xl">A shorter, cleaner workspace for creation and review.</h2>
            <p className="mt-2 max-w-2xl text-sm text-violet-100/90 sm:text-base">
              Create videos from one page, manage projects from another, and open a focused workspace when you need outputs, editing, and Remotion timeline review.
            </p>
            <div className="mt-4 flex flex-wrap gap-2">
              <button onClick={() => navigate({ view: 'create' })} className="rounded-lg bg-white px-4 py-2 text-sm font-semibold text-slate-900 hover:bg-violet-50">
                ✨ New video
              </button>
              <button onClick={() => navigate({ view: 'projects' })} className="rounded-lg border border-white/20 bg-white/10 px-4 py-2 text-sm font-medium text-white hover:bg-white/15">
                Browse projects
              </button>
            </div>
          </div>

          <div className="grid gap-3 sm:grid-cols-2">
            <MetricCard label="Projects" value={stats.total} hint="Total workspaces created" />
            <MetricCard label="Active now" value={stats.active} hint="Queued or processing" />
            <MetricCard label="Completed" value={stats.complete} hint="Ready-to-review videos" />
            <MetricCard label="Templates" value={templates.length} hint="Available starting layouts" />
          </div>
        </div>
      </section>

      <div className="grid gap-5 xl:grid-cols-[1.15fr_0.85fr]">
        <section className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm">
          <div className="mb-3 flex items-center justify-between gap-2">
            <div>
              <h3 className="text-base font-semibold text-slate-900">Recent projects</h3>
              <p className="text-sm text-slate-500">Open a focused workspace instead of scrolling through one long page.</p>
            </div>
            <button onClick={() => navigate({ view: 'projects' })} className="text-xs font-semibold text-violet-700 hover:underline">
              View all
            </button>
          </div>
          {recentProjects.length > 0 ? (
            <div className="space-y-3">
              {recentProjects.map((project) => (
                <ProjectQuickCard
                  key={project.id}
                  project={project}
                  onOpen={(id) => navigate({ view: 'workspace', projectId: id })}
                  onGenerate={handleGenerate}
                  onDelete={handleDelete}
                  isGenerating={generating === project.id}
                />
              ))}
            </div>
          ) : (
            <div className="rounded-xl border border-dashed border-slate-200 bg-slate-50 p-6 text-sm text-slate-500">
              No projects yet. Create your first video to start the pipeline.
            </div>
          )}
        </section>

        <section className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm">
          <h3 className="text-base font-semibold text-slate-900">Needs attention</h3>
          <p className="mt-1 text-sm text-slate-500">Jump back into live or failed projects quickly.</p>
          <div className="mt-3 space-y-3">
            {needsAttention.length > 0 ? needsAttention.map((project) => (
              <button
                key={project.id}
                onClick={() => navigate({ view: 'workspace', projectId: project.id })}
                className="w-full rounded-xl border border-slate-200 bg-slate-50 px-3 py-3 text-left hover:border-violet-200 hover:bg-violet-50/50"
              >
                <div className="flex items-center justify-between gap-2">
                  <span className="text-sm font-semibold text-slate-900 line-clamp-1">{project.topic}</span>
                  <StatusBadge status={project.status} />
                </div>
                <p className="mt-1 text-xs text-slate-500">{project.current_stage || 'created'} · updated {formatRelativeTime(project.updated_at)}</p>
              </button>
            )) : (
              <div className="rounded-xl border border-dashed border-slate-200 bg-slate-50 p-5 text-sm text-slate-500">
                No urgent items right now.
              </div>
            )}
          </div>
        </section>
      </div>
    </div>
  )

  const renderCreate = () => (
    <div className="space-y-5">
      <section className="rounded-3xl border border-slate-200 bg-white p-6 shadow-sm">
        <div className="mb-5">
          <p className="text-xs font-semibold uppercase tracking-[0.2em] text-violet-600">Create</p>
          <h2 className="mt-1 text-2xl font-bold text-slate-900">Start a new video project</h2>
          <p className="mt-1 text-sm text-slate-500">This page is only for setup, so the workspace stays focused on review and iteration.</p>
        </div>
        <CreateProjectForm templates={templates} onSubmit={handleCreate} loading={creating} />
      </section>
    </div>
  )

  const renderProjects = () => (
    <div className="space-y-5">
      <section className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm">
        <div className="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
          <div>
            <h2 className="text-base font-semibold text-slate-900">Project library</h2>
            <p className="text-sm text-slate-500">Search and filter here, then open a dedicated workspace for details.</p>
          </div>
          <div className="w-full xl:max-w-sm">
            <input
              type="search"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder="Search projects…"
              className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-700 outline-none transition focus:border-violet-500"
            />
          </div>
        </div>

        <div className="mt-3 flex flex-wrap gap-2">
          {STATUS_FILTERS.map((filter) => {
            const count = filter.value === 'all' ? projects.length : projects.filter((project) => project.status === filter.value).length
            return (
              <button
                key={filter.value}
                onClick={() => setStatusFilter(filter.value)}
                className={`rounded-full px-3 py-1.5 text-xs font-semibold transition ${statusFilter === filter.value ? 'bg-violet-600 text-white' : 'bg-slate-100 text-slate-600 hover:bg-slate-200'}`}
              >
                {filter.label} ({count})
              </button>
            )
          })}
        </div>
      </section>

      {loading ? (
        <div className="grid gap-4 lg:grid-cols-2 xl:grid-cols-3">
          {[1, 2, 3].map((index) => (
            <div key={index} className="animate-pulse rounded-xl border border-slate-200 bg-white p-5">
              <div className="mb-2 h-4 w-3/4 rounded bg-slate-200" />
              <div className="h-3 w-1/2 rounded bg-slate-100" />
            </div>
          ))}
        </div>
      ) : filteredProjects.length > 0 ? (
        <div className="grid gap-4 lg:grid-cols-2 xl:grid-cols-3">
          {filteredProjects.map((project) => (
            <ProjectQuickCard
              key={project.id}
              project={project}
              onOpen={(id) => navigate({ view: 'workspace', projectId: id })}
              onGenerate={handleGenerate}
              onDelete={handleDelete}
              isGenerating={generating === project.id}
            />
          ))}
        </div>
      ) : (
        <div className="rounded-2xl border border-dashed border-slate-300 bg-white px-6 py-10 text-center shadow-sm">
          <p className="text-base font-semibold text-slate-900">No projects match this filter.</p>
          <p className="mt-1 text-sm text-slate-500">Try another filter or create a new workspace.</p>
        </div>
      )}
    </div>
  )

  const renderWorkspace = () => (
    <div className="space-y-5">
      <section className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm">
        <div className="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.2em] text-violet-600">Workspace</p>
            <h2 className="mt-1 text-xl font-bold text-slate-900">Focused project review</h2>
            <p className="text-sm text-slate-500">One project at a time for step outputs, editing, Remotion review, and reruns.</p>
          </div>
          <div className="flex flex-wrap gap-2">
            <button onClick={() => navigate({ view: 'projects' })} className="rounded-lg border border-slate-200 px-3 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50">
              ← Back to library
            </button>
            <button onClick={() => navigate({ view: 'create' })} className="rounded-lg bg-violet-600 px-3 py-2 text-sm font-semibold text-white hover:bg-violet-700">
              + New video
            </button>
          </div>
        </div>
      </section>

      <div className="grid gap-5 xl:grid-cols-[320px_minmax(0,1fr)]">
        <aside className="space-y-4 rounded-2xl border border-slate-200 bg-white p-4 shadow-sm xl:sticky xl:top-4 xl:self-start">
          <div>
            <h3 className="text-sm font-semibold text-slate-900">Projects</h3>
            <p className="text-xs text-slate-500">Switch workspace without going back to the big list.</p>
          </div>
          <div className="max-h-[70vh] space-y-2 overflow-auto pr-1">
            {(filteredProjects.length > 0 ? filteredProjects : projects).map((project) => {
              const active = selectedProject?.id === project.id
              return (
                <button
                  key={project.id}
                  onClick={() => navigate({ view: 'workspace', projectId: project.id })}
                  className={`w-full rounded-xl border px-3 py-3 text-left transition ${active ? 'border-violet-300 bg-violet-50' : 'border-slate-200 bg-slate-50 hover:border-violet-200 hover:bg-violet-50/50'}`}
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className="text-sm font-semibold text-slate-900 line-clamp-2">{project.topic}</span>
                    <StatusBadge status={project.status} />
                  </div>
                  <p className="mt-1 text-[11px] text-slate-500">{project.current_stage || 'created'} · {formatRelativeTime(project.updated_at)}</p>
                </button>
              )
            })}
          </div>
        </aside>

        <div className="min-w-0">
          {selectedProject ? (
            <ProjectCard
              project={selectedProject}
              onGenerate={handleGenerate}
              onDelete={handleDelete}
              onRefreshProjects={loadData}
              isGenerating={generating === selectedProject.id}
              defaultShowOutputs
            />
          ) : (
            <div className="rounded-2xl border border-dashed border-slate-300 bg-white px-6 py-12 text-center shadow-sm">
              <p className="text-base font-semibold text-slate-900">Choose a project to open its workspace.</p>
              <p className="mt-1 text-sm text-slate-500">Your step outputs, editor, and Remotion timeline will open here.</p>
            </div>
          )}
        </div>
      </div>
    </div>
  )

  const currentView = route.view

  return (
    <div className="min-h-screen bg-slate-100">
      <div className="mx-auto max-w-7xl px-4 py-4 lg:py-6">
        <div className="grid gap-4 lg:grid-cols-[220px_minmax(0,1fr)]">
          <aside className="rounded-3xl border border-slate-200 bg-white p-4 shadow-sm lg:sticky lg:top-4 lg:self-start">
            <div className="mb-5 flex items-center gap-3">
              <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-br from-violet-600 to-fuchsia-600 text-lg text-white shadow-sm">
                🎬
              </div>
              <div>
                <h1 className="text-lg font-bold leading-none text-slate-900">Synt</h1>
                <p className="text-xs text-slate-500">AI video studio</p>
              </div>
            </div>

            <nav className="space-y-2">
              {APP_VIEWS.map((view) => {
                const active = currentView === view.value || (view.value === 'projects' && currentView === 'workspace')
                return (
                  <button
                    key={view.value}
                    onClick={() => navigate({ view: view.value, projectId: view.value === 'workspace' ? selectedProject?.id : undefined })}
                    className={`flex w-full items-center gap-2 rounded-xl px-3 py-2 text-sm font-medium transition ${active ? 'bg-violet-600 text-white' : 'text-slate-700 hover:bg-slate-100'}`}
                  >
                    <span>{view.emoji}</span>
                    <span>{view.label}</span>
                  </button>
                )
              })}
            </nav>

            <div className="mt-5 rounded-2xl bg-slate-50 p-3 text-xs text-slate-600">
              <div className="font-semibold text-slate-900">Studio status</div>
              <div className="mt-2">{stats.active} active · {stats.failed} need attention</div>
            </div>
          </aside>

          <div className="min-w-0 space-y-4">
            <div className="rounded-2xl border border-slate-200 bg-white p-3 shadow-sm lg:hidden">
              <div className="flex flex-wrap gap-2">
                {APP_VIEWS.map((view) => {
                  const active = currentView === view.value || (view.value === 'projects' && currentView === 'workspace')
                  return (
                    <button
                      key={view.value}
                      onClick={() => navigate({ view: view.value, projectId: view.value === 'workspace' ? selectedProject?.id : undefined })}
                      className={`rounded-full px-3 py-1.5 text-xs font-semibold ${active ? 'bg-violet-600 text-white' : 'bg-slate-100 text-slate-700'}`}
                    >
                      {view.emoji} {view.label}
                    </button>
                  )
                })}
              </div>
            </div>

            {error && (
              <div className="flex items-center justify-between rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
                <span>{error}</span>
                <button onClick={() => setError(null)} className="ml-2 text-red-500 hover:text-red-700">✕</button>
              </div>
            )}

            {currentView === 'overview' && renderOverview()}
            {currentView === 'create' && renderCreate()}
            {currentView === 'projects' && renderProjects()}
            {currentView === 'workspace' && renderWorkspace()}
          </div>
        </div>
      </div>
    </div>
  )
}
