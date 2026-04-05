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
    <section className="rounded-lg border border-gray-200 bg-gray-50/70 p-3">
      <div className="mb-2 flex items-start justify-between gap-3">
        <div>
          <h4 className="text-sm font-semibold text-gray-900">{title}</h4>
          <p className="text-xs text-gray-500">{subtitle}</p>
        </div>
        <span
          className={`rounded-full px-2 py-0.5 text-[11px] font-medium ${ready ? 'bg-green-100 text-green-700' : 'bg-gray-200 text-gray-600'
            }`}
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

function prettyValue(value: unknown) {
  if (value == null || value === '') return 'No output yet.'
  if (typeof value === 'string') return value
  return JSON.stringify(value, null, 2)
}

function renderInlineValue(value?: string) {
  if (!value) return <span className="text-gray-400">—</span>
  if (value.startsWith('http://') || value.startsWith('https://')) {
    return (
      <a href={value} target="_blank" rel="noreferrer" className="break-all text-purple-700 hover:underline">
        {value}
      </a>
    )
  }
  return <span className="break-all text-gray-700">{value}</span>
}

function getPublicMediaURL(metadata: unknown): string | null {
  if (!metadata || typeof metadata !== 'object' || Array.isArray(metadata)) return null
  const candidate = (metadata as { public_url?: unknown }).public_url
  return typeof candidate === 'string' && candidate.length > 0 ? candidate : null
}

export function ProjectCard({ project, onGenerate, onDelete, isGenerating }: ProjectCardProps) {
  const [showOutputs, setShowOutputs] = useState(false)
  const { status, stage, error: statusError } = useProjectStatus(
    project.status === 'processing' || project.status === 'queued' ? project.id : null
  )

  const currentStatus = status || project.status
  const currentStage = stage || project.current_stage
  const currentError = statusError || project.error_message

  const isActive = currentStatus === 'queued' || currentStatus === 'processing'
  const isDone = currentStatus === 'done'
  const isFailed = currentStatus === 'failed'

  const { script, assets, audio, subtitles, renders, loading, error, refresh } = useProjectOutputs(
    project.id,
    showOutputs,
    showOutputs && isActive
  )

  const mediaAssets = useMemo(() => assets.filter((asset) => asset.type !== 'manifest'), [assets])
  const manifestAsset = useMemo(() => assets.find((asset) => asset.type === 'manifest'), [assets])
  const voiceTracks = audio.filter((track) => track.kind === 'voiceover')
  const musicTracks = audio.filter((track) => track.kind === 'music')

  const platformLabels: Record<string, string> = {
    youtube_shorts: '▶ YouTube Shorts',
    tiktok: '♪ TikTok',
    instagram_reels: '◉ Instagram Reels',
  }

  return (
    <div className="rounded-xl border border-gray-200 bg-white p-5 shadow-sm transition-shadow hover:shadow-md">
      <div className="mb-3 flex items-start justify-between gap-2">
        <div>
          <h3 className="text-sm font-semibold leading-snug text-gray-900">{project.topic}</h3>
          <p className="mt-0.5 text-xs text-gray-500">
            {platformLabels[project.platform] ?? project.platform} · {project.duration_sec}s
          </p>
        </div>
        <StatusBadge status={currentStatus} />
      </div>

      <div className="mb-3">
        <PipelineProgress currentStage={currentStage} status={currentStatus} />
      </div>

      {currentError && (
        <p className="mb-3 rounded bg-red-50 p-2 text-xs text-red-600">{currentError}</p>
      )}

      <div className="mt-3 flex flex-wrap gap-2">
        {currentStatus === 'draft' && (
          <button
            onClick={() => onGenerate(project.id)}
            disabled={isGenerating}
            className="flex-1 rounded-lg bg-purple-600 px-3 py-1.5 text-xs font-semibold text-white transition-colors hover:bg-purple-700 disabled:opacity-50"
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

        {isDone && (
          <button className="flex-1 rounded-lg bg-green-600 px-3 py-1.5 text-xs font-semibold text-white transition-colors hover:bg-green-700">
            ↓ Download
          </button>
        )}

        <button
          onClick={() => setShowOutputs((value) => !value)}
          className="rounded-lg border border-purple-200 px-3 py-1.5 text-xs font-medium text-purple-700 hover:bg-purple-50 transition-colors"
        >
          {showOutputs ? 'Hide outputs' : 'View step outputs'}
        </button>

        {showOutputs && (
          <button
            onClick={() => void refresh()}
            className="rounded-lg border border-gray-200 px-3 py-1.5 text-xs font-medium text-gray-600 hover:bg-gray-50 transition-colors"
          >
            Refresh
          </button>
        )}

        <button
          onClick={() => onDelete(project.id)}
          className="rounded-lg border border-gray-200 px-3 py-1.5 text-xs font-medium text-gray-600 transition-colors hover:bg-gray-50"
        >
          Delete
        </button>
      </div>

      {showOutputs && (
        <div className="mt-4 space-y-3 border-t border-gray-100 pt-4">
          {error && <p className="rounded bg-red-50 p-2 text-xs text-red-600">{error}</p>}
          {loading && <p className="text-xs text-gray-500">Refreshing step outputs…</p>}

          <OutputSection
            title="1. Script generation"
            subtitle={script ? 'Structured script JSON is available' : 'Waiting for script output'}
            ready={!!script}
          >
            {script ? (
              <div className="space-y-2">
                <div className="grid gap-2 text-xs text-gray-700 sm:grid-cols-3">
                  <div className="rounded bg-white p-2"><strong>Title:</strong> {script.title || '—'}</div>
                  <div className="rounded bg-white p-2"><strong>Hook:</strong> {script.hook || '—'}</div>
                  <div className="rounded bg-white p-2"><strong>CTA:</strong> {script.cta || '—'}</div>
                </div>
                <pre className="max-h-56 overflow-auto rounded bg-slate-950 p-3 text-[11px] text-slate-100">{prettyValue(script.content_json)}</pre>
              </div>
            ) : (
              <p className="text-xs text-gray-500">No script has been generated yet.</p>
            )}
          </OutputSection>

          <OutputSection
            title="2. Script validation"
            subtitle="Moderation / validation step status"
            ready={hasReachedStage(currentStage, 'script_validation', currentStatus)}
          >
            <p className="text-xs text-gray-700">
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
                  <div key={asset.id} className="rounded bg-white p-2 text-xs text-gray-700">
                    <div className="mb-1 flex flex-wrap items-center gap-2">
                      <span className="rounded-full bg-blue-50 px-2 py-0.5 font-medium text-blue-700">{asset.type}</span>
                      <span className="rounded-full bg-gray-100 px-2 py-0.5 text-gray-600">{asset.provider || asset.source}</span>
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
              <p className="text-xs text-gray-500">No media assets found yet.</p>
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
                  <div key={`${asset.id}-prepared`} className="rounded bg-white p-2 text-xs text-gray-700">
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
              <p className="text-xs text-gray-500">Prepared asset output will appear here.</p>
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
                  const playableURL = publicURL ?? (track.storage_path?.startsWith('http://') || track.storage_path?.startsWith('https://') ? track.storage_path : null)

                  return (
                    <div key={track.id} className="rounded bg-white p-2 text-xs text-gray-700">
                      <div><strong>Voice:</strong> {track.voice_name || '—'}</div>
                      <div><strong>Language:</strong> {track.language || '—'}</div>
                      <div><strong>Duration:</strong> {formatDuration(track.duration_sec)}</div>
                      <div><strong>Worker path:</strong> {renderInlineValue(track.storage_path)}</div>
                      <div><strong>Public URL:</strong> {renderInlineValue(publicURL ?? undefined)}</div>
                      {playableURL && (
                        <audio controls className="mt-2 w-full">
                          <source src={playableURL} />
                        </audio>
                      )}
                      {Boolean(track.metadata) && (
                        <pre className="mt-2 max-h-40 overflow-auto rounded bg-slate-950 p-2 text-[11px] text-slate-100">{prettyValue(track.metadata)}</pre>
                      )}
                    </div>
                  )
                })}
              </div>
            ) : (
              <p className="text-xs text-gray-500">Voiceover output will appear here.</p>
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
                  <div key={subtitle.id} className="rounded bg-white p-2 text-xs text-gray-700">
                    <div><strong>Format:</strong> {subtitle.format}</div>
                    <div><strong>Storage path:</strong> {renderInlineValue(subtitle.storage_path)}</div>
                    <pre className="mt-2 max-h-40 overflow-auto rounded bg-slate-950 p-2 text-[11px] text-slate-100">{prettyValue(subtitle.content)}</pre>
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-xs text-gray-500">Subtitles will show here when ready.</p>
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
                  <div key={track.id} className="rounded bg-white p-2 text-xs text-gray-700">
                    <div><strong>Track type:</strong> {track.kind}</div>
                    <div><strong>Duration:</strong> {formatDuration(track.duration_sec)}</div>
                    <div><strong>Storage path:</strong> {renderInlineValue(track.storage_path)}</div>
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-xs text-gray-500">Selected music will appear here.</p>
            )}
          </OutputSection>

          <OutputSection
            title="8. Timeline build"
            subtitle="Render manifest created from script + media + audio"
            ready={!!manifestAsset || hasReachedStage(currentStage, 'timeline_build', currentStatus)}
          >
            {manifestAsset ? (
              <div className="space-y-2">
                <div className="rounded bg-white p-2 text-xs text-gray-700">
                  <div><strong>Manifest path:</strong> {renderInlineValue(manifestAsset.storage_path)}</div>
                </div>
                <pre className="max-h-56 overflow-auto rounded bg-slate-950 p-3 text-[11px] text-slate-100">{prettyValue(manifestAsset.metadata)}</pre>
              </div>
            ) : (
              <p className="text-xs text-gray-500">The timeline manifest will appear here once built.</p>
            )}
          </OutputSection>

          <OutputSection
            title="9. Render outputs"
            subtitle="Preview/final video render status and output files"
            ready={renders.length > 0 || hasReachedStage(currentStage, 'render_preview', currentStatus)}
          >
            {renders.length > 0 ? (
              <div className="space-y-2">
                {renders.map((render) => (
                  <div key={render.id} className="rounded bg-white p-2 text-xs text-gray-700">
                    <div className="mb-1 flex flex-wrap items-center gap-2">
                      <span className="rounded-full bg-gray-100 px-2 py-0.5 font-medium text-gray-700">{render.kind}</span>
                      <span>{render.resolution}</span>
                      <span>{render.fps} fps</span>
                      <span className={`rounded-full px-2 py-0.5 ${render.status === 'done' ? 'bg-green-100 text-green-700' : render.status === 'failed' ? 'bg-red-100 text-red-700' : 'bg-yellow-100 text-yellow-700'}`}>
                        {render.status}
                      </span>
                    </div>
                    <div><strong>Video path:</strong> {renderInlineValue(render.storage_path)}</div>
                    <div><strong>Thumbnail:</strong> {renderInlineValue(render.thumbnail_path)}</div>
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-xs text-gray-500">Preview and final render outputs will show here.</p>
            )}
          </OutputSection>
        </div>
      )}
    </div>
  )
}
