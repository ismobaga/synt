import { type ReactNode, useMemo, useState } from 'react'
import { PipelineProgress } from '../../components/ui/PipelineProgress'
import { StatusBadge } from '../../components/ui/StatusBadge'
import { useProjectOutputs } from '../../hooks/useProjectOutputs'
import { useProjectStatus } from '../../hooks/useProjectStatus'
import type { Project } from '../../lib/api'

const STAGE_ORDER = [
  'created',
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
  isGenerating: boolean
}

interface OutputSectionProps {
  title: string
  subtitle: string
  ready: boolean
  children: ReactNode
}

function OutputSection({ title, subtitle, ready, children }: OutputSectionProps) {
  return (
    <section className="rounded-xl border border-slate-200 bg-slate-50/80 p-3">
      <div className="mb-2 flex items-start justify-between gap-3">
        <div>
          <h4 className="text-sm font-semibold text-slate-900">{title}</h4>
          <p className="text-xs text-slate-500">{subtitle}</p>
        </div>
        <span
          className={`rounded-full px-2 py-0.5 text-[11px] font-medium ${ready ? 'bg-emerald-100 text-emerald-700' : 'bg-slate-200 text-slate-600'}`}
        >
          {ready ? 'Ready' : 'Waiting'}
        </span>
      </div>
      {children}
    </section>
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

function getMetadataValue(metadata: unknown, key: string): string | null {
  if (!metadata || typeof metadata !== 'object' || Array.isArray(metadata)) return null
  const candidate = (metadata as Record<string, unknown>)[key]
  if (candidate == null) return null
  return typeof candidate === 'string' ? candidate : String(candidate)
}

function getHttpURL(value?: string | null) {
  if (!value) return null
  return value.startsWith('http://') || value.startsWith('https://') ? value : null
}

export function ProjectCard({ project, onGenerate, onDelete, isGenerating }: ProjectCardProps) {
  const [showOutputs, setShowOutputs] = useState(project.status === 'processing' || project.status === 'failed')
  const { status, stage, error: statusError } = useProjectStatus(
    project.status === 'processing' || project.status === 'queued' ? project.id : null
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

  const mediaAssets = useMemo(() => assets.filter((asset) => asset.type !== 'manifest'), [assets])
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
        <PipelineProgress currentStage={currentStage} status={currentStatus} />
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
            <p className="mt-1 text-slate-500">{script ? 'Script ready' : 'Script pending'}</p>
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

          <OutputSection
            title="1. Script generation"
            subtitle={script ? 'Structured script JSON is available' : 'Waiting for script output'}
            ready={!!script}
          >
            {script ? (
              <div className="space-y-2">
                <div className="grid gap-2 text-xs text-slate-700 sm:grid-cols-3">
                  <div className="rounded bg-white p-2"><strong>Title:</strong> {script.title || '—'}</div>
                  <div className="rounded bg-white p-2"><strong>Hook:</strong> {script.hook || '—'}</div>
                  <div className="rounded bg-white p-2"><strong>CTA:</strong> {script.cta || '—'}</div>
                </div>
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
