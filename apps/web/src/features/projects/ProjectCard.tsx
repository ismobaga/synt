import { type ReactNode, useEffect, useMemo, useState } from 'react'
import { PipelineProgress } from '../../components/ui/PipelineProgress'
import { StatusBadge } from '../../components/ui/StatusBadge'
import { useProjectOutputs } from '../../hooks/useProjectOutputs'
import { useProjectStatus } from '../../hooks/useProjectStatus'
import { api, type AssetRecord, type Project, type RerunStep, type ScriptRecord, type UpdateScriptInput } from '../../lib/api'

const STAGE_ORDER = [
  'created',
  'source_fetch',
  'script_generation',
  'script_validation',
  'media_search',
  'media_prepare',
  'voice_generation',
  'subtitle_generation',
  'music_selection',
  'timeline_build',
  'render_preview',
  'render_final',
  'render_thumbnail',
  'finalize',
]

interface ProjectCardProps {
  project: Project
  onGenerate: (id: string) => void
  onDelete: (id: string) => void
  onRefreshProjects?: () => void | Promise<void>
  isGenerating: boolean
}

interface OutputSectionProps {
  title: string
  subtitle: string
  ready: boolean
  actions?: ReactNode
  children: ReactNode
}

type SubtitleStyleDraft = {
  preset: string
  position: string
  font_size: number
  primary_color: string
  outline_color: string
  bold: boolean
}

type SceneDraft = {
  index: number
  duration_sec: number
  narration: string
  caption: string
  visual_query: string
  overlay_style: string
  locked: boolean
  source_fact_ids: string[]
  source_facts: string[]
}

type ScriptDraft = {
  title: string
  hook: string
  cta: string
  language: string
  duration_sec: number
  music_mood: string
  used_source_facts: string[]
  subtitle_style: SubtitleStyleDraft
  scenes: SceneDraft[]
}

type MediaOverrideDraft = {
  id: string
  type: string
  provider: string
  url: string
  storage_path: string
  metadata: Record<string, unknown>
}

function OutputSection({ title, subtitle, ready, actions, children }: OutputSectionProps) {
  return (
    <section className="rounded-xl border border-slate-200 bg-slate-50/80 p-3">
      <div className="mb-2 flex items-start justify-between gap-3">
        <div>
          <h4 className="text-sm font-semibold text-slate-900">{title}</h4>
          <p className="text-xs text-slate-500">{subtitle}</p>
        </div>
        <div className="flex flex-wrap items-center justify-end gap-2">
          {actions}
          <span
            className={`rounded-full px-2 py-0.5 text-[11px] font-medium ${ready ? 'bg-emerald-100 text-emerald-700' : 'bg-slate-200 text-slate-600'}`}
          >
            {ready ? 'Ready' : 'Waiting'}
          </span>
        </div>
      </div>
      {children}
    </section>
  )
}

function StepActionButton({
  label,
  onClick,
  disabled = false,
  tone = 'default',
}: {
  label: string
  onClick: () => void
  disabled?: boolean
  tone?: 'default' | 'primary'
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className={`rounded-lg px-2.5 py-1 text-[11px] font-semibold transition ${tone === 'primary'
        ? 'bg-violet-600 text-white hover:bg-violet-700 disabled:bg-violet-300'
        : 'border border-slate-200 bg-white text-slate-700 hover:bg-slate-50 disabled:text-slate-400'
        } disabled:cursor-not-allowed`}
    >
      {label}
    </button>
  )
}

function hasReachedStage(currentStage: string, targetStage: string, status: string) {
  if (status === 'done') return true
  const currentIndex = STAGE_ORDER.indexOf(currentStage)
  const targetIndex = STAGE_ORDER.indexOf(targetStage)
  return currentIndex >= targetIndex && targetIndex >= 0
}

function formatDuration(value?: number) {
  if (!value) return '—'
  return `${Math.round(value * 10) / 10}s`
}

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

function prettyValue(value: unknown) {
  if (value == null || value === '') return 'No output yet.'
  if (typeof value === 'string') return value
  return JSON.stringify(value, null, 2)
}

function renderInlineValue(value?: string) {
  if (!value) return <span className="text-slate-400">—</span>
  if (value.startsWith('http://') || value.startsWith('https://')) {
    return (
      <a href={value} target="_blank" rel="noreferrer" className="break-all text-violet-700 hover:underline">
        {value}
      </a>
    )
  }
  return <span className="break-all text-slate-700">{value}</span>
}

function getPublicMediaURL(metadata: unknown): string | null {
  if (!metadata || typeof metadata !== 'object' || Array.isArray(metadata)) return null
  const candidate = (metadata as { public_url?: unknown }).public_url
  return typeof candidate === 'string' && candidate.length > 0 ? candidate : null
}

function getRenderEngineFromTemplateId(templateId: string) {
  return templateId.trim().toLowerCase().startsWith('remotion_') ? 'Remotion' : 'FFmpeg'
}

function getBaseTemplateId(templateId: string) {
  return templateId.trim().toLowerCase().startsWith('remotion_')
    ? templateId.trim().slice('remotion_'.length)
    : templateId.trim()
}

function getMetadataObject(metadata: unknown): Record<string, unknown> {
  if (!metadata || typeof metadata !== 'object' || Array.isArray(metadata)) return {}
  return metadata as Record<string, unknown>
}

function getMetadataValue(metadata: unknown, key: string): string | null {
  const candidate = getMetadataObject(metadata)[key]
  if (candidate == null) return null
  return typeof candidate === 'string' ? candidate : String(candidate)
}

function getStringList(value: unknown): string[] {
  return Array.isArray(value) ? value.map((entry) => String(entry).trim()).filter(Boolean) : []
}

function getSceneIndexFromAsset(asset: AssetRecord, fallbackIndex: number) {
  const raw = Number(getMetadataObject(asset.metadata).scene_index)
  return Number.isFinite(raw) && raw > 0 ? raw : fallbackIndex + 1
}

function normalizeSubtitleStyle(value: unknown): SubtitleStyleDraft {
  const meta = getMetadataObject(value)
  const fontSize = Number(meta.font_size)
  return {
    preset: typeof meta.preset === 'string' && meta.preset ? meta.preset : 'clean',
    position: typeof meta.position === 'string' && meta.position ? meta.position : 'bottom',
    font_size: Number.isFinite(fontSize) && fontSize > 0 ? fontSize : 14,
    primary_color: typeof meta.primary_color === 'string' && meta.primary_color ? meta.primary_color : '#FFFFFF',
    outline_color: typeof meta.outline_color === 'string' && meta.outline_color ? meta.outline_color : '#111827',
    bold: Boolean(meta.bold),
  }
}

function normalizeScriptDraft(script: ScriptRecord | null): ScriptDraft | null {
  if (!script) return null
  const content = getMetadataObject(script.content_json)
  const rawScenes = Array.isArray(content.scenes) ? content.scenes : []
  const scenes = rawScenes.map((scene, index) => {
    const value = getMetadataObject(scene)
    const duration = Number(value.duration_sec)
    return {
      index: Number(value.index) || index + 1,
      duration_sec: Number.isFinite(duration) && duration > 0 ? duration : 5,
      narration: typeof value.narration === 'string' ? value.narration : '',
      caption: typeof value.caption === 'string' ? value.caption : '',
      visual_query: typeof value.visual_query === 'string' ? value.visual_query : '',
      overlay_style: typeof value.overlay_style === 'string' && value.overlay_style ? value.overlay_style : 'main',
      locked: Boolean(value.locked),
      source_fact_ids: getStringList(value.source_fact_ids),
      source_facts: getStringList(value.source_facts),
    }
  })

  return {
    title: typeof content.title === 'string' && content.title ? content.title : script.title,
    hook: typeof content.hook === 'string' && content.hook ? content.hook : script.hook,
    cta: typeof content.cta === 'string' && content.cta ? content.cta : script.cta,
    language: typeof content.language === 'string' && content.language ? content.language : script.language,
    duration_sec: Number(content.duration_sec) || scenes.reduce((total, scene) => total + scene.duration_sec, 0) || 30,
    music_mood: typeof content.music_mood === 'string' ? content.music_mood : '',
    used_source_facts: getStringList(content.used_source_facts),
    subtitle_style: normalizeSubtitleStyle(content.subtitle_style),
    scenes,
  }
}

function makeMediaOverrideDrafts(assets: AssetRecord[]) {
  return assets.reduce<Record<string, MediaOverrideDraft>>((acc, asset) => {
    acc[asset.id] = {
      id: asset.id,
      type: asset.type || 'video',
      provider: asset.provider || asset.source || 'manual_override',
      url: asset.url || '',
      storage_path: asset.storage_path || asset.url || '',
      metadata: getMetadataObject(asset.metadata),
    }
    return acc
  }, {})
}

function getHttpURL(value?: string | null) {
  if (!value) return null
  return value.startsWith('http://') || value.startsWith('https://') ? value : null
}

export function ProjectCard({ project, onGenerate, onDelete, onRefreshProjects, isGenerating }: ProjectCardProps) {
  const [showOutputs, setShowOutputs] = useState(project.status === 'processing' || project.status === 'failed')
  const [showEditor, setShowEditor] = useState(false)
  const [scriptDraft, setScriptDraft] = useState<ScriptDraft | null>(null)
  const [mediaDrafts, setMediaDrafts] = useState<Record<string, MediaOverrideDraft>>({})
  const [editorNotice, setEditorNotice] = useState<string | null>(null)
  const [editorError, setEditorError] = useState<string | null>(null)
  const [savingScript, setSavingScript] = useState(false)
  const [savingMedia, setSavingMedia] = useState(false)
  const [rerunningStep, setRerunningStep] = useState<RerunStep | null>(null)
  const { status, stage, error: statusError, steps } = useProjectStatus(
    project.status === 'processing' || project.status === 'queued' || project.status === 'failed' || project.status === 'done' || showOutputs
      ? project.id
      : null
  )

  const currentStatus = status || project.status
  const currentStage = stage || project.current_stage
  const currentError = statusError || project.error_message

  const isActive = currentStatus === 'queued' || currentStatus === 'processing'
  const isDone = currentStatus === 'done'
  const isFailed = currentStatus === 'failed'
  const shouldLoadOutputs = showOutputs || isDone || isFailed

  const { script, assets, audio, subtitles, renders, loading, error, refresh } = useProjectOutputs(
    project.id,
    shouldLoadOutputs,
    shouldLoadOutputs && isActive
  )

  const sourceAssets = useMemo(
    () => assets.filter((asset) => asset.type === 'source_material' || asset.type === 'source_note'),
    [assets]
  )
  const mediaAssets = useMemo(
    () => assets.filter((asset) => asset.type === 'image' || asset.type === 'video'),
    [assets]
  )
  const sourceFactBank = useMemo(() => {
    const seen = new Set<string>()
    return sourceAssets.flatMap((asset) => {
      const facts = getStringList(getMetadataObject(asset.metadata).grounding_facts)
      return facts.filter((fact) => {
        if (seen.has(fact)) return false
        seen.add(fact)
        return true
      })
    })
  }, [sourceAssets])
  const sceneAssetMap = useMemo(() => {
    const map = new Map<number, AssetRecord>()
    mediaAssets.forEach((asset, index) => {
      const sceneIndex = getSceneIndexFromAsset(asset, index)
      const existing = map.get(sceneIndex)
      const isManual = Boolean(getMetadataObject(asset.metadata).manual_override)
      const existingIsManual = existing ? Boolean(getMetadataObject(existing.metadata).manual_override) : false
      if (!existing || (isManual && !existingIsManual) || new Date(asset.created_at).getTime() >= new Date(existing.created_at).getTime()) {
        map.set(sceneIndex, asset)
      }
    })
    return map
  }, [mediaAssets])
  const manifestAsset = useMemo(() => assets.find((asset) => asset.type === 'manifest'), [assets])
  const displayRenders = useMemo(() => {
    const rank = (status: string) => {
      switch (status) {
        case 'done':
          return 3
        case 'processing':
          return 2
        case 'queued':
          return 1
        default:
          return 0
      }
    }

    const byKey = new Map<string, (typeof renders)[number]>()
    for (const render of renders) {
      const key = `${render.kind}:${render.resolution}`
      const existing = byKey.get(key)
      if (!existing || rank(render.status) > rank(existing.status)) {
        byKey.set(key, render)
      }
    }

    const order = ['preview', 'final']
    return Array.from(byKey.values()).sort((a, b) => {
      const aIndex = order.includes(a.kind) ? order.indexOf(a.kind) : order.length
      const bIndex = order.includes(b.kind) ? order.indexOf(b.kind) : order.length
      return aIndex - bIndex
    })
  }, [renders])

  const voiceTracks = audio.filter((track) => track.kind === 'voiceover')
  const musicTracks = audio.filter((track) => track.kind === 'music')
  const renderEngineLabel = getRenderEngineFromTemplateId(project.template_id)
  const displayTemplateId = getBaseTemplateId(project.template_id)
  const latestFinalRender = useMemo(
    () => displayRenders.find((render) => render.kind === 'final' && render.status === 'done') ?? displayRenders.find((render) => render.status === 'done') ?? null,
    [displayRenders]
  )
  const latestPreviewRender = useMemo(
    () => displayRenders.find((render) => render.kind === 'preview' && render.status === 'done') ?? null,
    [displayRenders]
  )
  const latestVoiceTrack = voiceTracks[0] ?? null
  const finalVideoURL = latestFinalRender
    ? getHttpURL(latestFinalRender.storage_path) ?? getPublicMediaURL(latestFinalRender.metadata)
    : null
  const previewVideoURL = latestPreviewRender
    ? getHttpURL(latestPreviewRender.storage_path) ?? getPublicMediaURL(latestPreviewRender.metadata)
    : null
  const latestVoiceURL = latestVoiceTrack
    ? getPublicMediaURL(latestVoiceTrack.metadata) ?? getHttpURL(latestVoiceTrack.storage_path)
    : null
  const sceneReview = scriptDraft?.scenes ?? []
  const isWorking = savingScript || savingMedia || Boolean(rerunningStep)
  const getMediaAssetForScene = (scene: SceneDraft, fallbackIndex: number) =>
    sceneAssetMap.get(scene.index || fallbackIndex + 1) ?? mediaAssets[fallbackIndex] ?? null

  useEffect(() => {
    if (!showEditor) return
    setScriptDraft((current) => current ?? normalizeScriptDraft(script))
    setMediaDrafts((current) => (Object.keys(current).length > 0 ? current : makeMediaOverrideDrafts(mediaAssets)))
  }, [showEditor, script, mediaAssets])

  const resetEditorFromOutputs = () => {
    setScriptDraft(normalizeScriptDraft(script))
    setMediaDrafts(makeMediaOverrideDrafts(mediaAssets))
    setEditorNotice(null)
    setEditorError(null)
  }

  const toggleEditor = () => {
    setShowOutputs(true)
    setShowEditor((value) => {
      const next = !value
      if (next) {
        setScriptDraft(normalizeScriptDraft(script))
        setMediaDrafts(makeMediaOverrideDrafts(mediaAssets))
        setEditorNotice(null)
        setEditorError(null)
      }
      return next
    })
  }

  const handleSceneChange = (sceneIndex: number, field: keyof SceneDraft, value: string | number | boolean | string[]) => {
    setScriptDraft((current) => {
      if (!current) return current
      const scenes = current.scenes.map((scene, index) => {
        if (index !== sceneIndex) return scene
        const nextScene = { ...scene, [field]: value }
        if (field === 'source_fact_ids') {
          const ids = Array.isArray(value) ? value.map((entry) => String(entry).trim().toUpperCase()).filter(Boolean) : []
          nextScene.source_fact_ids = ids
          nextScene.source_facts = ids
            .map((id) => {
              const match = /^F(\d+)$/.exec(id)
              if (!match) return null
              const factIndex = Number(match[1]) - 1
              return factIndex >= 0 ? sourceFactBank[factIndex] ?? null : null
            })
            .filter((fact): fact is string => Boolean(fact))
        }
        return nextScene
      })
      const nextDuration = scenes.reduce((total, scene) => total + Number(scene.duration_sec || 0), 0)
      const usedFacts = Array.from(new Set(scenes.flatMap((scene) => scene.source_facts)))
      return { ...current, scenes, duration_sec: nextDuration || current.duration_sec, used_source_facts: usedFacts }
    })
  }

  const moveScene = (sceneIndex: number, direction: 'up' | 'down') => {
    setScriptDraft((current) => {
      if (!current) return current
      const targetIndex = direction === 'up' ? sceneIndex - 1 : sceneIndex + 1
      if (targetIndex < 0 || targetIndex >= current.scenes.length) return current
      const scenes = [...current.scenes]
      ;[scenes[sceneIndex], scenes[targetIndex]] = [scenes[targetIndex], scenes[sceneIndex]]
      const normalizedScenes = scenes.map((scene, index) => ({ ...scene, index: index + 1 }))
      return {
        ...current,
        scenes: normalizedScenes,
        duration_sec: normalizedScenes.reduce((total, scene) => total + Number(scene.duration_sec || 0), 0) || current.duration_sec,
      }
    })
  }

  const handleSubtitleStyleChange = (field: keyof SubtitleStyleDraft, value: string | number | boolean) => {
    setScriptDraft((current) => {
      if (!current) return current
      return {
        ...current,
        subtitle_style: {
          ...current.subtitle_style,
          [field]: value,
        },
      }
    })
  }

  const handleMediaDraftChange = (assetId: string, field: keyof MediaOverrideDraft, value: string) => {
    setMediaDrafts((current) => ({
      ...current,
      [assetId]: {
        ...(current[assetId] ?? {
          id: assetId,
          type: 'video',
          provider: 'manual_override',
          url: '',
          storage_path: '',
          metadata: {},
        }),
        [field]: value,
      },
    }))
  }

  const handleSaveScript = async () => {
    if (!scriptDraft) return
    setSavingScript(true)
    setEditorError(null)
    setEditorNotice(null)

    const payload: UpdateScriptInput = {
      title: scriptDraft.title.trim(),
      hook: scriptDraft.hook.trim(),
      cta: scriptDraft.cta.trim(),
      language: scriptDraft.language,
      duration_sec: Math.max(1, Math.round(scriptDraft.duration_sec || 30)),
      music_mood: scriptDraft.music_mood,
      used_source_facts: scriptDraft.used_source_facts,
      subtitle_style: scriptDraft.subtitle_style,
      scenes: scriptDraft.scenes.map((scene, index) => ({
        index: scene.index || index + 1,
        duration_sec: Math.max(1, Math.round(Number(scene.duration_sec) || 1)),
        narration: scene.narration.trim(),
        caption: scene.caption.trim(),
        visual_query: scene.visual_query.trim(),
        overlay_style: scene.overlay_style,
        locked: scene.locked,
        source_fact_ids: scene.source_fact_ids,
        source_facts: scene.source_facts,
      })),
    }

    try {
      await api.projects.updateScript(project.id, payload)
      await refresh()
      await Promise.resolve(onRefreshProjects?.())
      setEditorNotice('Script changes saved. Rerun the timeline or preview to apply only the edited scenes; frozen scenes keep their media.')
    } catch (err) {
      setEditorError(err instanceof Error ? err.message : 'Failed to save script changes')
    } finally {
      setSavingScript(false)
    }
  }

  const handleSaveMedia = async () => {
    if (mediaAssets.length === 0) return
    setSavingMedia(true)
    setEditorError(null)
    setEditorNotice(null)
    try {
      await Promise.all(
        sceneReview.map((scene, index) => {
          const asset = getMediaAssetForScene(scene, index)
          if (!asset) return Promise.resolve()
          const draft = mediaDrafts[asset.id]
          if (!draft) return Promise.resolve()
          return api.projects.updateAsset(project.id, asset.id, {
            type: draft.type,
            provider: draft.provider,
            url: draft.url.trim(),
            storage_path: (draft.storage_path || draft.url).trim(),
            mime_type: asset.mime_type,
            metadata: {
              ...getMetadataObject(asset.metadata),
              ...draft.metadata,
              scene_index: scene.index || index + 1,
              scene_locked: scene.locked,
              manual_override: true,
              override_updated_at: new Date().toISOString(),
            },
          })
        })
      )
      await refresh()
      setEditorNotice('Media overrides saved. Rerun the timeline or preview to apply them; manual replacements stay pinned on later media reruns.')
    } catch (err) {
      setEditorError(err instanceof Error ? err.message : 'Failed to save media replacements')
    } finally {
      setSavingMedia(false)
    }
  }

  const handleRerunStep = async (step: RerunStep) => {
    setRerunningStep(step)
    setEditorError(null)
    setEditorNotice(null)
    try {
      await api.projects.rerunStep(project.id, step)
      await Promise.resolve(onRefreshProjects?.())
      await refresh()
      setShowOutputs(true)
      setEditorNotice(`Queued a rerun from the ${step.replace('_', ' ')} step.`)
    } catch (err) {
      setEditorError(err instanceof Error ? err.message : 'Failed to queue rerun')
    } finally {
      setRerunningStep(null)
    }
  }

  const platformLabels: Record<string, string> = {
    youtube_shorts: '▶ YouTube Shorts',
    tiktok: '♪ TikTok',
    instagram_reels: '◉ Instagram Reels',
  }

  return (
    <div className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm transition-shadow hover:shadow-md">
      <div className="mb-3 flex items-start justify-between gap-3">
        <div>
          <h3 className="text-sm font-semibold leading-snug text-slate-900">{project.topic}</h3>
          <p className="mt-0.5 text-xs text-slate-500">
            {platformLabels[project.platform] ?? project.platform} · {project.duration_sec}s
          </p>
        </div>
        <StatusBadge status={currentStatus} />
      </div>

      <div className="mb-3 flex flex-wrap gap-1.5 text-[11px] text-slate-600">
        <span className="rounded-full bg-slate-100 px-2 py-1 font-medium">{project.language.toUpperCase()}</span>
        <span className="rounded-full bg-slate-100 px-2 py-1 font-medium">{project.tone}</span>
        <span className="rounded-full bg-violet-50 px-2 py-1 font-medium text-violet-700">{renderEngineLabel}</span>
        <span className="rounded-full bg-slate-100 px-2 py-1 font-medium">{displayTemplateId}</span>
        <span className="rounded-full bg-slate-100 px-2 py-1">Updated {formatRelativeTime(project.updated_at)}</span>
        {isActive && <span className="rounded-full bg-blue-50 px-2 py-1 text-blue-700">Live refresh enabled</span>}
      </div>

      <div className="mb-3">
        <PipelineProgress currentStage={currentStage} status={currentStatus} steps={steps} />
      </div>

      {currentError && <p className="mb-3 rounded-lg bg-red-50 p-2 text-xs text-red-600">{currentError}</p>}

      {shouldLoadOutputs && (latestFinalRender || latestPreviewRender || latestVoiceTrack || mediaAssets[0]) && (
        <div className="mb-3 grid gap-2 sm:grid-cols-3">
          <div className="rounded-xl border border-slate-200 bg-slate-50 p-3 text-xs text-slate-700">
            <p className="text-[10px] font-semibold uppercase tracking-wide text-slate-500">Render</p>
            <p className="mt-1 font-semibold text-slate-900">
              {latestFinalRender ? `${renderEngineLabel} render ready` : latestPreviewRender ? `${renderEngineLabel} preview available` : 'Still rendering'}
            </p>
            {finalVideoURL && (
              <a href={finalVideoURL} target="_blank" rel="noreferrer" className="mt-2 inline-block text-violet-700 hover:underline">
                Open final MP4
              </a>
            )}
          </div>

          <div className="rounded-xl border border-slate-200 bg-slate-50 p-3 text-xs text-slate-700">
            <p className="text-[10px] font-semibold uppercase tracking-wide text-slate-500">Voice</p>
            <p className="mt-1 font-semibold text-slate-900">{latestVoiceTrack?.voice_name || 'Waiting for narration'}</p>
            <p className="mt-1 text-slate-500">{latestVoiceTrack ? formatDuration(latestVoiceTrack.duration_sec) : 'No audio yet'}</p>
            {latestVoiceURL && (
              <a href={latestVoiceURL} target="_blank" rel="noreferrer" className="mt-2 inline-block text-violet-700 hover:underline">
                Listen audio
              </a>
            )}
          </div>

          <div className="rounded-xl border border-slate-200 bg-slate-50 p-3 text-xs text-slate-700">
            <p className="text-[10px] font-semibold uppercase tracking-wide text-slate-500">Assets</p>
            <p className="mt-1 font-semibold text-slate-900">{mediaAssets.length} media / {subtitles.length} subtitles</p>
            <p className="mt-1 text-slate-500">
              {sourceAssets.length > 0 ? `${sourceAssets.length} source reference(s) attached` : script ? 'Script ready' : 'Script pending'}
            </p>
          </div>
        </div>
      )}

      <div className="mt-3 flex flex-wrap gap-2">
        {currentStatus === 'draft' && (
          <button
            onClick={() => onGenerate(project.id)}
            disabled={isGenerating}
            className="flex-1 rounded-lg bg-violet-600 px-3 py-1.5 text-xs font-semibold text-white transition-colors hover:bg-violet-700 disabled:opacity-50"
          >
            {isGenerating ? 'Starting…' : '▶ Generate'}
          </button>
        )}

        {isFailed && (
          <button
            onClick={() => onGenerate(project.id)}
            disabled={isGenerating}
            className="flex-1 rounded-lg bg-orange-500 px-3 py-1.5 text-xs font-semibold text-white transition-colors hover:bg-orange-600 disabled:opacity-50"
          >
            ↺ Retry
          </button>
        )}

        {finalVideoURL && (
          <a
            href={finalVideoURL}
            target="_blank"
            rel="noreferrer"
            className="rounded-lg bg-emerald-600 px-3 py-1.5 text-xs font-semibold text-white transition-colors hover:bg-emerald-700"
          >
            ↓ Open final
          </a>
        )}

        {previewVideoURL && (
          <a
            href={previewVideoURL}
            target="_blank"
            rel="noreferrer"
            className="rounded-lg border border-emerald-200 px-3 py-1.5 text-xs font-medium text-emerald-700 transition-colors hover:bg-emerald-50"
          >
            Preview
          </a>
        )}

        <button
          onClick={() => setShowOutputs((value) => !value)}
          className="rounded-lg border border-violet-200 px-3 py-1.5 text-xs font-medium text-violet-700 transition-colors hover:bg-violet-50"
        >
          {showOutputs ? 'Hide outputs' : 'View step outputs'}
        </button>

        {script && (
          <button
            onClick={toggleEditor}
            className="rounded-lg border border-emerald-200 px-3 py-1.5 text-xs font-medium text-emerald-700 transition-colors hover:bg-emerald-50"
          >
            {showEditor ? 'Close editor' : '✏️ Editing studio'}
          </button>
        )}

        {showOutputs && (
          <button
            onClick={() => void refresh()}
            className="rounded-lg border border-slate-200 px-3 py-1.5 text-xs font-medium text-slate-600 transition-colors hover:bg-slate-50"
          >
            Refresh
          </button>
        )}

        <button
          onClick={() => onDelete(project.id)}
          className="rounded-lg border border-slate-200 px-3 py-1.5 text-xs font-medium text-slate-600 transition-colors hover:bg-slate-50"
        >
          Delete
        </button>
      </div>

      {showOutputs && (
        <div className="mt-4 space-y-3 border-t border-slate-100 pt-4">
          {error && <p className="rounded bg-red-50 p-2 text-xs text-red-600">{error}</p>}
          {loading && <p className="text-xs text-slate-500">Refreshing step outputs…</p>}
          {editorError && <p className="rounded bg-red-50 p-2 text-xs text-red-600">{editorError}</p>}
          {editorNotice && <p className="rounded bg-emerald-50 p-2 text-xs text-emerald-700">{editorNotice}</p>}

          {script && (
            <OutputSection
              title="✏️ Editing studio"
              subtitle="Edit the script before the next render, reorder or freeze scenes, replace media manually, review grounded facts, and rerun from any step."
              ready={!!script}
              actions={
                <>
                  <StepActionButton
                    label={showEditor ? 'Hide editor' : 'Open editor'}
                    onClick={toggleEditor}
                    tone="primary"
                  />
                  <StepActionButton
                    label={rerunningStep === 'timeline' ? 'Queueing…' : 'Rerun timeline'}
                    onClick={() => void handleRerunStep('timeline')}
                    disabled={isWorking}
                  />
                </>
              }
            >
              {showEditor && scriptDraft ? (
                <div className="space-y-3">
                  <div className="grid gap-3 lg:grid-cols-2">
                    <label className="text-xs font-medium text-slate-700">
                      Title
                      <input
                        value={scriptDraft.title}
                        onChange={(event) => setScriptDraft((current) => current ? { ...current, title: event.target.value } : current)}
                        className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-800"
                      />
                    </label>
                    <label className="text-xs font-medium text-slate-700">
                      Duration (seconds)
                      <input
                        type="number"
                        min={5}
                        value={scriptDraft.duration_sec}
                        onChange={(event) => setScriptDraft((current) => current ? { ...current, duration_sec: Number(event.target.value) || current.duration_sec } : current)}
                        className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-800"
                      />
                    </label>
                    <label className="text-xs font-medium text-slate-700 lg:col-span-2">
                      Hook
                      <textarea
                        value={scriptDraft.hook}
                        onChange={(event) => setScriptDraft((current) => current ? { ...current, hook: event.target.value } : current)}
                        rows={2}
                        className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-800"
                      />
                    </label>
                    <label className="text-xs font-medium text-slate-700 lg:col-span-2">
                      CTA
                      <textarea
                        value={scriptDraft.cta}
                        onChange={(event) => setScriptDraft((current) => current ? { ...current, cta: event.target.value } : current)}
                        rows={2}
                        className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-800"
                      />
                    </label>
                  </div>

                  <div className="rounded-xl border border-amber-200 bg-amber-50/60 p-3">
                    <div className="mb-2 flex items-center justify-between gap-2">
                      <div>
                        <h5 className="text-sm font-semibold text-slate-900">Grounding review</h5>
                        <p className="text-xs text-slate-500">Use the fact bank to keep claims sourced, and freeze scenes you do not want media search to change.</p>
                      </div>
                    </div>
                    {sourceFactBank.length > 0 ? (
                      <div className="space-y-2 text-xs text-slate-700">
                        <div>
                          <p className="mb-1 font-semibold text-slate-800">Source fact bank</p>
                          <ul className="list-disc space-y-1 pl-4 text-slate-600">
                            {sourceFactBank.map((fact, index) => (
                              <li key={`fact-bank-${index}`}><span className="font-semibold text-slate-700">F{index + 1}:</span> {fact}</li>
                            ))}
                          </ul>
                        </div>
                        {scriptDraft.used_source_facts.length > 0 && (
                          <div>
                            <p className="mb-1 font-semibold text-slate-800">Facts currently used in the script</p>
                            <div className="flex flex-wrap gap-1.5">
                              {scriptDraft.used_source_facts.map((fact, index) => (
                                <span key={`used-fact-${index}`} className="rounded-full bg-white px-2 py-1 text-[11px] text-slate-700">{fact}</span>
                              ))}
                            </div>
                          </div>
                        )}
                      </div>
                    ) : (
                      <p className="text-xs text-slate-500">No extracted source fact bank is available for this project yet.</p>
                    )}
                  </div>

                  <div className="rounded-xl border border-slate-200 bg-white p-3">
                    <div className="mb-2 flex items-center justify-between gap-2">
                      <div>
                        <h5 className="text-sm font-semibold text-slate-900">Subtitle & style adjustments</h5>
                        <p className="text-xs text-slate-500">These settings are applied to the next render.</p>
                      </div>
                    </div>
                    <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-5">
                      <label className="text-xs font-medium text-slate-700">
                        Preset
                        <select
                          value={scriptDraft.subtitle_style.preset}
                          onChange={(event) => handleSubtitleStyleChange('preset', event.target.value)}
                          className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"
                        >
                          <option value="clean">Clean</option>
                          <option value="bold">Bold</option>
                          <option value="hook">Hook</option>
                        </select>
                      </label>
                      <label className="text-xs font-medium text-slate-700">
                        Position
                        <select
                          value={scriptDraft.subtitle_style.position}
                          onChange={(event) => handleSubtitleStyleChange('position', event.target.value)}
                          className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"
                        >
                          <option value="bottom">Bottom</option>
                          <option value="center">Center</option>
                          <option value="top">Top</option>
                        </select>
                      </label>
                      <label className="text-xs font-medium text-slate-700">
                        Font size
                        <input
                          type="number"
                          min={10}
                          max={40}
                          value={scriptDraft.subtitle_style.font_size}
                          onChange={(event) => handleSubtitleStyleChange('font_size', Number(event.target.value) || 14)}
                          className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"
                        />
                      </label>
                      <label className="text-xs font-medium text-slate-700">
                        Text color
                        <input
                          type="color"
                          value={scriptDraft.subtitle_style.primary_color}
                          onChange={(event) => handleSubtitleStyleChange('primary_color', event.target.value)}
                          className="mt-1 h-10 w-full rounded-lg border border-slate-300 px-2"
                        />
                      </label>
                      <label className="text-xs font-medium text-slate-700">
                        Outline color
                        <input
                          type="color"
                          value={scriptDraft.subtitle_style.outline_color}
                          onChange={(event) => handleSubtitleStyleChange('outline_color', event.target.value)}
                          className="mt-1 h-10 w-full rounded-lg border border-slate-300 px-2"
                        />
                      </label>
                    </div>
                    <label className="mt-3 inline-flex items-center gap-2 text-xs font-medium text-slate-700">
                      <input
                        type="checkbox"
                        checked={scriptDraft.subtitle_style.bold}
                        onChange={(event) => handleSubtitleStyleChange('bold', event.target.checked)}
                      />
                      Use bold subtitle emphasis
                    </label>
                  </div>

                  <div className="space-y-3">
                    {sceneReview.map((scene, index) => {
                      const asset = getMediaAssetForScene(scene, index)
                      const draft = asset ? mediaDrafts[asset.id] : null
                      const draftPreviewURL = draft ? getHttpURL(draft.url) ?? getHttpURL(draft.storage_path) : null
                      const assetPreviewURL = asset ? getHttpURL(asset.url) ?? getHttpURL(asset.storage_path) ?? getPublicMediaURL(asset.metadata) : null
                      const mediaPreviewURL = draftPreviewURL ?? assetPreviewURL
                      return (
                        <div key={`${scene.index}-${index}`} className="rounded-xl border border-slate-200 bg-white p-3">
                          <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
                            <div>
                              <h5 className="text-sm font-semibold text-slate-900">Scene {scene.index || index + 1}</h5>
                              <p className="text-xs text-slate-500">Review narration, caption, visual query, citations, and media override for this scene.</p>
                            </div>
                            <div className="flex flex-wrap items-center gap-2">
                              <span className={`rounded-full px-2 py-1 text-[11px] ${scene.locked ? 'bg-amber-100 text-amber-700' : 'bg-slate-100 text-slate-600'}`}>{scene.locked ? 'Frozen' : 'Editable'}</span>
                              <span className="rounded-full bg-slate-100 px-2 py-1 text-[11px] text-slate-600">{formatDuration(scene.duration_sec)}</span>
                              <StepActionButton label="↑" onClick={() => moveScene(index, 'up')} disabled={isWorking || index === 0} />
                              <StepActionButton label="↓" onClick={() => moveScene(index, 'down')} disabled={isWorking || index === sceneReview.length - 1} />
                            </div>
                          </div>

                          <div className="grid gap-3 lg:grid-cols-2">
                            <label className="text-xs font-medium text-slate-700 lg:col-span-2">
                              Narration
                              <textarea
                                value={scene.narration}
                                onChange={(event) => handleSceneChange(index, 'narration', event.target.value)}
                                rows={3}
                                className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"
                              />
                            </label>
                            <label className="text-xs font-medium text-slate-700">
                              Caption
                              <input
                                value={scene.caption}
                                onChange={(event) => handleSceneChange(index, 'caption', event.target.value)}
                                className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"
                              />
                            </label>
                            <label className="text-xs font-medium text-slate-700">
                              Overlay style
                              <select
                                value={scene.overlay_style}
                                onChange={(event) => handleSceneChange(index, 'overlay_style', event.target.value)}
                                className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"
                              >
                                <option value="hook">Hook</option>
                                <option value="main">Main</option>
                                <option value="cta">CTA</option>
                              </select>
                            </label>
                            <label className="text-xs font-medium text-slate-700">
                              Visual query
                              <input
                                value={scene.visual_query}
                                onChange={(event) => handleSceneChange(index, 'visual_query', event.target.value)}
                                className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"
                              />
                            </label>
                            <label className="text-xs font-medium text-slate-700">
                              Scene length (sec)
                              <input
                                type="number"
                                min={1}
                                value={scene.duration_sec}
                                onChange={(event) => handleSceneChange(index, 'duration_sec', Number(event.target.value) || 1)}
                                className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"
                              />
                            </label>
                            <label className="text-xs font-medium text-slate-700 lg:col-span-2">
                              Source fact IDs (comma separated)
                              <input
                                value={scene.source_fact_ids.join(', ')}
                                onChange={(event) => handleSceneChange(index, 'source_fact_ids', event.target.value.split(',').map((value) => value.trim()).filter(Boolean))}
                                placeholder="F1, F2"
                                className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"
                              />
                            </label>
                          </div>

                          <label className="mt-3 inline-flex items-center gap-2 text-xs font-medium text-slate-700">
                            <input
                              type="checkbox"
                              checked={scene.locked}
                              onChange={(event) => handleSceneChange(index, 'locked', event.target.checked)}
                            />
                            Freeze this scene's current media during media reruns
                          </label>

                          {(scene.source_facts.length > 0 || scene.source_fact_ids.length > 0) && (
                            <div className="mt-3 rounded-xl border border-violet-100 bg-violet-50/70 p-3 text-xs text-slate-700">
                              <p className="font-semibold text-slate-800">Grounding used in this scene</p>
                              {scene.source_fact_ids.length > 0 && <p className="mt-1 text-slate-600">Fact IDs: {scene.source_fact_ids.join(', ')}</p>}
                              {scene.source_facts.length > 0 && (
                                <ul className="mt-2 list-disc space-y-1 pl-4 text-slate-600">
                                  {scene.source_facts.map((fact, factIndex) => (
                                    <li key={`${scene.index}-source-fact-${factIndex}`}>{fact}</li>
                                  ))}
                                </ul>
                              )}
                            </div>
                          )}

                          <div className="mt-3 rounded-xl border border-dashed border-slate-200 bg-slate-50 p-3">
                            <div className="mb-2">
                              <h6 className="text-xs font-semibold uppercase tracking-wide text-slate-600">Manual media replacement</h6>
                              <p className="text-xs text-slate-500">Paste your own image/video URL or local path and rerun from the timeline.</p>
                            </div>
                            {asset && draft ? (
                              <div className="grid gap-3 md:grid-cols-2">
                                <label className="text-xs font-medium text-slate-700">
                                  Media type
                                  <select
                                    value={draft.type}
                                    onChange={(event) => handleMediaDraftChange(asset.id, 'type', event.target.value)}
                                    className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"
                                  >
                                    <option value="video">Video</option>
                                    <option value="image">Image</option>
                                  </select>
                                </label>
                                <label className="text-xs font-medium text-slate-700">
                                  Provider label
                                  <input
                                    value={draft.provider}
                                    onChange={(event) => handleMediaDraftChange(asset.id, 'provider', event.target.value)}
                                    className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"
                                  />
                                </label>
                                <label className="text-xs font-medium text-slate-700 md:col-span-2">
                                  Media URL or local path
                                  <input
                                    value={draft.url}
                                    onChange={(event) => handleMediaDraftChange(asset.id, 'url', event.target.value)}
                                    className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"
                                  />
                                </label>
                                <label className="text-xs font-medium text-slate-700 md:col-span-2">
                                  Prepared storage path (optional)
                                  <input
                                    value={draft.storage_path}
                                    onChange={(event) => handleMediaDraftChange(asset.id, 'storage_path', event.target.value)}
                                    className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"
                                  />
                                </label>
                                <div className="md:col-span-2 text-[11px] text-slate-500">
                                  Current asset: {renderInlineValue(asset.url || asset.storage_path)}
                                </div>
                                {mediaPreviewURL && (
                                  <div className="md:col-span-2 overflow-hidden rounded-lg border border-slate-200 bg-black/5">
                                    {draft?.type === 'image' || asset.type === 'image' ? (
                                      <img src={mediaPreviewURL} alt={`Scene ${scene.index} preview`} className="max-h-48 w-full object-cover" />
                                    ) : (
                                      <video src={mediaPreviewURL} controls className="max-h-48 w-full bg-black" />
                                    )}
                                  </div>
                                )}
                              </div>
                            ) : (
                              <p className="text-xs text-slate-500">Media will appear here after the search step completes.</p>
                            )}
                          </div>
                        </div>
                      )
                    })}
                  </div>

                  <div className="flex flex-wrap gap-2">
                    <StepActionButton label={savingScript ? 'Saving…' : 'Save script edits'} onClick={() => void handleSaveScript()} disabled={isWorking} tone="primary" />
                    <StepActionButton label={savingMedia ? 'Saving media…' : 'Save media overrides'} onClick={() => void handleSaveMedia()} disabled={isWorking || mediaAssets.length === 0} />
                    <StepActionButton label="Reset editor" onClick={resetEditorFromOutputs} disabled={isWorking} />
                    <StepActionButton label={rerunningStep === 'script' ? 'Queueing…' : 'Rerun from script'} onClick={() => void handleRerunStep('script')} disabled={isWorking} />
                    <StepActionButton label={rerunningStep === 'preview' ? 'Queueing…' : 'Render preview'} onClick={() => void handleRerunStep('preview')} disabled={isWorking} />
                    <StepActionButton label={rerunningStep === 'final' ? 'Queueing…' : 'Render final'} onClick={() => void handleRerunStep('final')} disabled={isWorking} />
                  </div>
                </div>
              ) : (
                <p className="text-xs text-slate-600">Open the editor to review the script before the next render, adjust each scene, replace media manually, and tune caption styling.</p>
              )}
            </OutputSection>
          )}

          <OutputSection
            title="0. Source material"
            subtitle={sourceAssets.length > 0 ? 'Reference URLs, fetched webpage text, and video transcripts' : 'No source material provided'}
            ready={sourceAssets.length > 0 || hasReachedStage(currentStage, 'source_fetch', currentStatus)}
            actions={<StepActionButton label={rerunningStep === 'source' ? 'Queueing…' : 'Refetch'} onClick={() => void handleRerunStep('source')} disabled={isWorking} />}
          >
            {sourceAssets.length > 0 ? (
              <div className="space-y-2">
                {sourceAssets.map((asset) => {
                  const noteText = getMetadataValue(asset.metadata, 'notes') ?? getMetadataValue(asset.metadata, 'text')
                  const title = getMetadataValue(asset.metadata, 'title')
                  const fetchStatus = getMetadataValue(asset.metadata, 'fetch_status')
                  const fetchError = getMetadataValue(asset.metadata, 'fetch_error')
                  const contentText = getMetadataValue(asset.metadata, 'content_text')
                  const transcriptText = getMetadataValue(asset.metadata, 'transcript_text')
                  const transcriptSource = getMetadataValue(asset.metadata, 'transcript_source')
                  const groundingQuality = getMetadataValue(asset.metadata, 'grounding_quality')
                  const groundingFacts = Array.isArray(getMetadataObject(asset.metadata).grounding_facts)
                    ? (getMetadataObject(asset.metadata).grounding_facts as unknown[]).map((value) => String(value))
                    : []
                  return (
                    <div key={asset.id} className="rounded bg-white p-2 text-xs text-slate-700">
                      <div className="mb-1 flex flex-wrap items-center gap-2">
                        <span className="rounded-full bg-violet-50 px-2 py-0.5 font-medium text-violet-700">{asset.type === 'source_material' ? 'URL' : 'Note'}</span>
                        <span className="rounded-full bg-slate-100 px-2 py-0.5 text-slate-600">{asset.provider || asset.source}</span>
                        {fetchStatus && (
                          <span className={`rounded-full px-2 py-0.5 ${fetchStatus === 'fetched' ? 'bg-emerald-100 text-emerald-700' : fetchStatus === 'failed' ? 'bg-red-100 text-red-700' : 'bg-slate-100 text-slate-600'}`}>
                            {fetchStatus}
                          </span>
                        )}
                      </div>
                      {asset.url && <div><strong>Link:</strong> {renderInlineValue(asset.url)}</div>}
                      {title && <div className="mt-1"><strong>Title:</strong> {title}</div>}
                      {groundingQuality && <div className="mt-1"><strong>Grounding quality:</strong> {groundingQuality}</div>}
                      {transcriptSource && <div className="mt-1"><strong>Transcript source:</strong> {transcriptSource}</div>}
                      {noteText && <p className="mt-1 whitespace-pre-wrap text-slate-600">{noteText}</p>}
                      {contentText && (
                        <details className="mt-2 rounded border border-slate-200 bg-slate-50 p-2">
                          <summary className="cursor-pointer font-semibold text-slate-700">Fetched webpage content</summary>
                          <p className="mt-2 whitespace-pre-wrap text-slate-600">{contentText}</p>
                        </details>
                      )}
                      {transcriptText && (
                        <details className="mt-2 rounded border border-slate-200 bg-slate-50 p-2">
                          <summary className="cursor-pointer font-semibold text-slate-700">Fetched video transcript</summary>
                          <p className="mt-2 whitespace-pre-wrap text-slate-600">{transcriptText}</p>
                        </details>
                      )}
                      {groundingFacts.length > 0 && (
                        <details className="mt-2 rounded border border-slate-200 bg-slate-50 p-2">
                          <summary className="cursor-pointer font-semibold text-slate-700">Grounded facts extracted</summary>
                          <ul className="mt-2 list-disc space-y-1 pl-4 text-slate-600">
                            {groundingFacts.map((fact, index) => (
                              <li key={`${asset.id}-fact-${index}`}>{fact}</li>
                            ))}
                          </ul>
                        </details>
                      )}
                      {fetchError && <p className="mt-2 text-red-600">{fetchError}</p>}
                    </div>
                  )
                })}
              </div>
            ) : (
              <p className="text-xs text-slate-500">Add article links, YouTube URLs, or notes during project creation to guide the script.</p>
            )}
          </OutputSection>

          <OutputSection
            title="1. Script generation"
            subtitle={script ? 'Structured script JSON is available' : 'Waiting for script output'}
            ready={!!script}
            actions={<StepActionButton label={rerunningStep === 'script' ? 'Queueing…' : 'Rerun'} onClick={() => void handleRerunStep('script')} disabled={isWorking} />}
          >
            {script ? (
              <div className="space-y-2">
                <div className="grid gap-2 text-xs text-slate-700 sm:grid-cols-3">
                  <div className="rounded bg-white p-2"><strong>Title:</strong> {script.title || '—'}</div>
                  <div className="rounded bg-white p-2"><strong>Hook:</strong> {script.hook || '—'}</div>
                  <div className="rounded bg-white p-2"><strong>CTA:</strong> {script.cta || '—'}</div>
                </div>
                {normalizeScriptDraft(script)?.used_source_facts.length ? (
                  <div className="rounded bg-emerald-50 p-2 text-xs text-emerald-800">
                    <strong>Grounded facts used:</strong>
                    <ul className="mt-1 list-disc space-y-1 pl-4">
                      {normalizeScriptDraft(script)?.used_source_facts.map((fact, index) => (
                        <li key={`script-fact-${index}`}>{fact}</li>
                      ))}
                    </ul>
                  </div>
                ) : null}
                <pre className="max-h-56 overflow-auto rounded bg-slate-950 p-3 text-[11px] text-slate-100">{prettyValue(script.content_json)}</pre>
              </div>
            ) : (
              <p className="text-xs text-slate-500">No script has been generated yet.</p>
            )}
          </OutputSection>

          <OutputSection
            title="2. Script validation"
            subtitle="Moderation / validation step status"
            ready={hasReachedStage(currentStage, 'script_validation', currentStatus)}
            actions={<StepActionButton label={rerunningStep === 'script' ? 'Queueing…' : 'Run again'} onClick={() => void handleRerunStep('script')} disabled={isWorking} />}
          >
            <p className="text-xs text-slate-700">
              {isFailed && currentStage === 'script_validation'
                ? currentError || 'Validation failed.'
                : hasReachedStage(currentStage, 'media_search', currentStatus)
                  ? 'Validation passed and the pipeline continued to media search.'
                  : hasReachedStage(currentStage, 'script_validation', currentStatus)
                    ? 'Validation is running now.'
                    : 'Waiting for validation to start.'}
            </p>
          </OutputSection>

          <OutputSection
            title="3. Media search"
            subtitle="Image and video assets found for each scene"
            ready={mediaAssets.length > 0 || hasReachedStage(currentStage, 'media_search', currentStatus)}
            actions={<StepActionButton label={rerunningStep === 'media' ? 'Queueing…' : 'Rerun'} onClick={() => void handleRerunStep('media')} disabled={isWorking} />}
          >
            {mediaAssets.length > 0 ? (
              <div className="space-y-2">
                {mediaAssets.map((asset) => (
                  <div key={asset.id} className="rounded bg-white p-2 text-xs text-slate-700">
                    <div className="mb-1 flex flex-wrap items-center gap-2">
                      <span className="rounded-full bg-blue-50 px-2 py-0.5 font-medium text-blue-700">{asset.type}</span>
                      <span className="rounded-full bg-slate-100 px-2 py-0.5 text-slate-600">{asset.provider || asset.source}</span>
                      <span>{asset.width || 0}×{asset.height || 0}</span>
                      {asset.duration_sec > 0 && <span>{formatDuration(asset.duration_sec)}</span>}
                    </div>
                    <div><strong>URL:</strong> {renderInlineValue(asset.url)}</div>
                    {Boolean(asset.license_info) && (
                      <pre className="mt-2 max-h-32 overflow-auto rounded bg-slate-950 p-2 text-[11px] text-slate-100">{prettyValue(asset.license_info)}</pre>
                    )}
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-xs text-slate-500">No media assets found yet.</p>
            )}
          </OutputSection>

          <OutputSection
            title="4. Media preparation"
            subtitle="Prepared file paths and metadata for rendering"
            ready={hasReachedStage(currentStage, 'media_prepare', currentStatus)}
            actions={<StepActionButton label={rerunningStep === 'media_prepare' ? 'Queueing…' : 'Rerun prep'} onClick={() => void handleRerunStep('media_prepare')} disabled={isWorking} />}
          >
            {mediaAssets.length > 0 ? (
              <div className="space-y-2">
                {mediaAssets.map((asset) => (
                  <div key={`${asset.id}-prepared`} className="rounded bg-white p-2 text-xs text-slate-700">
                    <div><strong>Type:</strong> {asset.type}</div>
                    <div><strong>Storage path:</strong> {renderInlineValue(asset.storage_path)}</div>
                    <div><strong>MIME:</strong> {asset.mime_type || 'not set yet'}</div>
                    {Boolean(asset.metadata) && (
                      <pre className="mt-2 max-h-32 overflow-auto rounded bg-slate-950 p-2 text-[11px] text-slate-100">{prettyValue(asset.metadata)}</pre>
                    )}
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-xs text-slate-500">Prepared asset output will appear here.</p>
            )}
          </OutputSection>

          <OutputSection
            title="5. Voice generation"
            subtitle="Voiceover audio created from the script narration"
            ready={voiceTracks.length > 0 || hasReachedStage(currentStage, 'voice_generation', currentStatus)}
            actions={<StepActionButton label={rerunningStep === 'voice' ? 'Queueing…' : 'Regenerate'} onClick={() => void handleRerunStep('voice')} disabled={isWorking} />}
          >
            {voiceTracks.length > 0 ? (
              <div className="space-y-2">
                {voiceTracks.map((track) => {
                  const publicURL = getPublicMediaURL(track.metadata)
                  const playableURL = publicURL ?? getHttpURL(track.storage_path)
                  const engine = getMetadataValue(track.metadata, 'provider') ?? getMetadataValue(track.metadata, 'engine')
                  const model = getMetadataValue(track.metadata, 'model') ?? getMetadataValue(track.metadata, 'tts_model')
                  const transcript = getMetadataValue(track.metadata, 'transcript')

                  return (
                    <div key={track.id} className="rounded bg-white p-2 text-xs text-slate-700">
                      <div><strong>Voice:</strong> {track.voice_name || '—'}</div>
                      <div><strong>Language:</strong> {track.language || '—'}</div>
                      <div><strong>Duration:</strong> {formatDuration(track.duration_sec)}</div>
                      {engine && <div><strong>Engine:</strong> {engine}</div>}
                      {model && <div><strong>Model:</strong> {model}</div>}
                      <div><strong>Worker path:</strong> {renderInlineValue(track.storage_path)}</div>
                      <div><strong>Public URL:</strong> {renderInlineValue(publicURL ?? undefined)}</div>
                      {playableURL && (
                        <audio controls className="mt-2 w-full">
                          <source src={playableURL} />
                        </audio>
                      )}
                      {transcript && (
                        <details className="mt-2 rounded border border-slate-200 bg-slate-50 p-2">
                          <summary className="cursor-pointer font-semibold text-slate-700">Narration transcript</summary>
                          <p className="mt-2 whitespace-pre-wrap text-slate-600">{transcript}</p>
                        </details>
                      )}
                      {Boolean(track.metadata) && (
                        <pre className="mt-2 max-h-40 overflow-auto rounded bg-slate-950 p-2 text-[11px] text-slate-100">{prettyValue(track.metadata)}</pre>
                      )}
                    </div>
                  )
                })}
              </div>
            ) : (
              <p className="text-xs text-slate-500">Voiceover output will appear here.</p>
            )}
          </OutputSection>

          <OutputSection
            title="6. Subtitle generation"
            subtitle="Phrase-based caption output"
            ready={subtitles.length > 0 || hasReachedStage(currentStage, 'subtitle_generation', currentStatus)}
            actions={<StepActionButton label={rerunningStep === 'subtitles' ? 'Queueing…' : 'Regenerate'} onClick={() => void handleRerunStep('subtitles')} disabled={isWorking} />}
          >
            {subtitles.length > 0 ? (
              <div className="space-y-2">
                {subtitles.map((subtitle) => (
                  <div key={subtitle.id} className="rounded bg-white p-2 text-xs text-slate-700">
                    <div><strong>Format:</strong> {subtitle.format}</div>
                    <div><strong>Storage path:</strong> {renderInlineValue(subtitle.storage_path)}</div>
                    <pre className="mt-2 max-h-40 overflow-auto rounded bg-slate-950 p-2 text-[11px] text-slate-100">{prettyValue(subtitle.content)}</pre>
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-xs text-slate-500">Subtitles will show here when ready.</p>
            )}
          </OutputSection>

          <OutputSection
            title="7. Music selection"
            subtitle="Background track selected for the video"
            ready={musicTracks.length > 0 || hasReachedStage(currentStage, 'music_selection', currentStatus)}
            actions={<StepActionButton label={rerunningStep === 'music' ? 'Queueing…' : 'Reselect'} onClick={() => void handleRerunStep('music')} disabled={isWorking} />}
          >
            {musicTracks.length > 0 ? (
              <div className="space-y-2">
                {musicTracks.map((track) => (
                  <div key={track.id} className="rounded bg-white p-2 text-xs text-slate-700">
                    <div><strong>Track type:</strong> {track.kind}</div>
                    <div><strong>Duration:</strong> {formatDuration(track.duration_sec)}</div>
                    <div><strong>Storage path:</strong> {renderInlineValue(track.storage_path)}</div>
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-xs text-slate-500">Selected music will appear here.</p>
            )}
          </OutputSection>

          <OutputSection
            title="8. Timeline build"
            subtitle="Render manifest created from script + media + audio"
            ready={!!manifestAsset || hasReachedStage(currentStage, 'timeline_build', currentStatus)}
            actions={<StepActionButton label={rerunningStep === 'timeline' ? 'Queueing…' : 'Rebuild'} onClick={() => void handleRerunStep('timeline')} disabled={isWorking} />}
          >
            {manifestAsset ? (
              <div className="space-y-2">
                <div className="rounded bg-white p-2 text-xs text-slate-700">
                  <div><strong>Manifest path:</strong> {renderInlineValue(manifestAsset.storage_path)}</div>
                </div>
                <pre className="max-h-56 overflow-auto rounded bg-slate-950 p-3 text-[11px] text-slate-100">{prettyValue(manifestAsset.metadata)}</pre>
              </div>
            ) : (
              <p className="text-xs text-slate-500">The timeline manifest will appear here once built.</p>
            )}
          </OutputSection>

          <OutputSection
            title="9. Render outputs"
            subtitle="Preview/final video render status and output files"
            ready={displayRenders.length > 0 || hasReachedStage(currentStage, 'render_preview', currentStatus)}
            actions={
              <>
                <StepActionButton label={rerunningStep === 'preview' ? 'Queueing…' : 'Preview'} onClick={() => void handleRerunStep('preview')} disabled={isWorking} />
                <StepActionButton label={rerunningStep === 'final' ? 'Queueing…' : 'Final'} onClick={() => void handleRerunStep('final')} disabled={isWorking} />
              </>
            }
          >
            {displayRenders.length > 0 ? (
              <div className="space-y-2">
                {displayRenders.map((render) => {
                  const renderPublicURL = getPublicMediaURL(render.metadata)
                  const videoURL = getHttpURL(render.storage_path) ?? renderPublicURL
                  const thumbnailURL = getHttpURL(render.thumbnail_path)

                  return (
                    <div key={render.id} className="rounded bg-white p-2 text-xs text-slate-700">
                      <div className="mb-1 flex flex-wrap items-center gap-2">
                        <span className="rounded-full bg-slate-100 px-2 py-0.5 font-medium text-slate-700">{render.kind}</span>
                        <span>{render.resolution}</span>
                        <span>{render.fps} fps</span>
                        <span className={`rounded-full px-2 py-0.5 ${render.status === 'done' ? 'bg-emerald-100 text-emerald-700' : render.status === 'failed' ? 'bg-red-100 text-red-700' : 'bg-yellow-100 text-yellow-700'}`}>
                          {render.status}
                        </span>
                      </div>
                      <div><strong>Video path:</strong> {renderInlineValue(render.storage_path)}</div>
                      <div><strong>Thumbnail:</strong> {renderInlineValue(render.thumbnail_path)}</div>
                      {videoURL && render.status === 'done' && (
                        <video controls className="mt-2 w-full rounded border border-slate-200 bg-black">
                          <source src={videoURL} />
                        </video>
                      )}
                      {thumbnailURL && (
                        <img src={thumbnailURL} alt={`${render.kind} thumbnail`} className="mt-2 max-h-40 rounded border border-slate-200" />
                      )}
                    </div>
                  )
                })}
              </div>
            ) : (
              <p className="text-xs text-slate-500">Preview and final render outputs will show here.</p>
            )}
          </OutputSection>
        </div>
      )}
    </div>
  )
}
