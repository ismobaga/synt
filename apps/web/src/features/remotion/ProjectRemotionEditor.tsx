import { useEffect, useMemo, useState } from 'react'
import type { AssetRecord, AudioTrackRecord, Project, RenderRecord, ScriptRecord, SubtitleRecord } from '../../lib/api'

type TimelineItem = {
    id: string
    label: string
    startSec: number
    endSec: number
    tone: string
    description?: string
}

type EditorScene = {
    id: string
    index: number
    startSec: number
    endSec: number
    caption: string
    narration: string
    overlayStyle: string
    mediaType: string
    mediaPath: string
    sourceFactIds: string[]
    sourceFacts: string[]
}

type EditorTrack = {
    label: string
    items: TimelineItem[]
}

type EditorModel = {
    fps: number
    width: number
    height: number
    totalDurationSec: number
    scenes: EditorScene[]
    tracks: EditorTrack[]
}

type ProjectRemotionEditorProps = {
    project: Project
    script: ScriptRecord | null
    manifestAsset?: AssetRecord | null
    audio: AudioTrackRecord[]
    subtitles: SubtitleRecord[]
    renders: RenderRecord[]
}

function asRecord(value: unknown): Record<string, unknown> {
    return value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, unknown>) : {}
}

function asArray(value: unknown): unknown[] {
    return Array.isArray(value) ? value : []
}

function toNumber(value: unknown, fallback: number) {
    const numeric = Number(value)
    return Number.isFinite(numeric) ? numeric : fallback
}

function isHttpUrl(value?: string) {
    return Boolean(value && (value.startsWith('http://') || value.startsWith('https://')))
}

function clamp(value: number, min: number, max: number) {
    return Math.min(max, Math.max(min, value))
}

function formatTime(seconds: number) {
    const safe = Math.max(0, seconds)
    const minutes = Math.floor(safe / 60)
    const remainder = safe % 60
    return `${minutes}:${remainder.toFixed(1).padStart(4, '0')}`
}

function buildEditorModel(
    project: Project,
    script: ScriptRecord | null,
    manifestAsset: AssetRecord | null | undefined,
    audio: AudioTrackRecord[],
    subtitles: SubtitleRecord[],
): EditorModel | null {
    const manifest = asRecord(manifestAsset?.metadata)
    const manifestResolution = asRecord(manifest.resolution)
    const scriptContent = asRecord(script?.content_json)
    const scriptScenes = asArray(scriptContent.scenes)
    const manifestScenes = asArray(manifest.scenes)

    const fps = toNumber(manifest.fps, 30)
    const width = toNumber(manifestResolution.width, 1080)
    const height = toNumber(manifestResolution.height, 1920)

    let cursor = 0
    const scenes: EditorScene[] = (manifestScenes.length > 0 ? manifestScenes : scriptScenes).map((rawScene, index) => {
        const manifestScene = asRecord(rawScene)
        const scriptScene = asRecord(scriptScenes[index])
        const media = asRecord(manifestScene.media)
        const captions = asArray(manifestScene.captions)
        const startSec = toNumber(manifestScene.start_sec, cursor)
        const fallbackDuration = toNumber(scriptScene.duration_sec, 5)
        const endSec = toNumber(manifestScene.end_sec, startSec + fallbackDuration)
        cursor = Math.max(cursor, endSec)

        return {
            id: `scene-${index + 1}`,
            index: toNumber(scriptScene.index, index + 1),
            startSec,
            endSec,
            caption:
                String(asRecord(captions[0]).text ?? scriptScene.caption ?? '').trim() || `Scene ${index + 1}`,
            narration: String(scriptScene.narration ?? '').trim(),
            overlayStyle: String(scriptScene.overlay_style ?? 'main').trim() || 'main',
            mediaType: String(media.type ?? 'visual').trim() || 'visual',
            mediaPath: String(media.path ?? '').trim(),
            sourceFactIds: asArray(scriptScene.source_fact_ids).map((entry) => String(entry).trim()).filter(Boolean),
            sourceFacts: asArray(scriptScene.source_facts).map((entry) => String(entry).trim()).filter(Boolean),
        }
    })

    const totalDurationSec = Math.max(
        toNumber(manifest.duration_sec, 0),
        scenes[scenes.length - 1]?.endSec ?? 0,
        toNumber(scriptContent.duration_sec, project.duration_sec || 30),
        1,
    )

    const voiceTracks = audio.filter((track) => track.kind === 'voiceover')
    const musicTracks = audio.filter((track) => track.kind === 'music')

    const tracks: EditorTrack[] = [
        {
            label: 'Visuals',
            items: scenes.map((scene) => ({
                id: `${scene.id}-visual`,
                label: scene.caption,
                startSec: scene.startSec,
                endSec: scene.endSec,
                tone: 'bg-violet-500/80',
                description: scene.mediaType,
            })),
        },
        {
            label: 'Captions',
            items: scenes.map((scene) => ({
                id: `${scene.id}-caption`,
                label: scene.caption || `Caption ${scene.index}`,
                startSec: scene.startSec,
                endSec: scene.endSec,
                tone: 'bg-emerald-500/80',
            })),
        },
        {
            label: 'Voiceover',
            items: voiceTracks.map((track, index) => ({
                id: `${track.id}-voice`,
                label: track.voice_name || `Voice ${index + 1}`,
                startSec: 0,
                endSec: Math.max(toNumber(track.duration_sec, totalDurationSec), 1),
                tone: 'bg-sky-500/80',
                description: track.language || 'voice track',
            })),
        },
        {
            label: 'Music',
            items: musicTracks.map((track, index) => ({
                id: `${track.id}-music`,
                label: track.voice_name || `Music ${index + 1}`,
                startSec: 0,
                endSec: Math.max(toNumber(track.duration_sec, totalDurationSec), totalDurationSec),
                tone: 'bg-amber-500/80',
            })),
        },
    ]

    if (subtitles.length > 0 && tracks[1].items.length === 0) {
        tracks[1].items.push({
            id: `${subtitles[0].id}-subtitle`,
            label: subtitles[0].format.toUpperCase(),
            startSec: 0,
            endSec: totalDurationSec,
            tone: 'bg-emerald-500/80',
        })
    }

    return { fps, width, height, totalDurationSec, scenes, tracks }
}

export function ProjectRemotionEditor({ project, script, manifestAsset, audio, subtitles, renders }: ProjectRemotionEditorProps) {
    const model = useMemo(
        () => buildEditorModel(project, script, manifestAsset, audio, subtitles),
        [project, script, manifestAsset, audio, subtitles],
    )
    const [isPlaying, setIsPlaying] = useState(false)
    const [currentTime, setCurrentTime] = useState(0)

    useEffect(() => {
        if (!model) return
        setCurrentTime((value) => clamp(value, 0, model.totalDurationSec))
    }, [model])

    useEffect(() => {
        if (!model || !isPlaying) return
        const interval = window.setInterval(() => {
            setCurrentTime((value) => {
                const next = value + 1 / model.fps
                if (next >= model.totalDurationSec) {
                    setIsPlaying(false)
                    return model.totalDurationSec
                }
                return next
            })
        }, 1000 / model.fps)

        return () => window.clearInterval(interval)
    }, [isPlaying, model])

    if (!model) {
        return <p className="text-xs text-slate-500">Build the timeline first to open the Remotion editor workspace.</p>
    }

    const activeScene = model.scenes.find((scene) => currentTime >= scene.startSec && currentTime < scene.endSec) ?? model.scenes[0] ?? null
    const progress = model.totalDurationSec > 0 ? (currentTime / model.totalDurationSec) * 100 : 0
    const renderEngine = project.template_id.toLowerCase().startsWith('remotion_') ? 'Remotion' : 'FFmpeg + Remotion workspace'
    const latestRenderable = renders.find((render) => render.status === 'done' && (render.kind === 'preview' || render.kind === 'final'))

    return (
        <div className="space-y-4">
            <div className="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-violet-200 bg-violet-50/70 px-3 py-2 text-xs text-violet-900">
                <div>
                    <span className="font-semibold">Remotion editor workspace</span>
                    <span className="ml-2 text-violet-700">{renderEngine} · {model.width}×{model.height} · {model.fps} fps</span>
                </div>
                <code className="rounded bg-white px-2 py-1 text-[11px] text-slate-700">cd apps/web && npm run remotion:studio</code>
            </div>

            <div className="grid gap-4 xl:grid-cols-[minmax(0,1.2fr)_320px]">
                <div className="space-y-3">
                    <div className="overflow-hidden rounded-2xl border border-slate-200 bg-slate-950 shadow-sm">
                        <div className="flex items-center justify-between border-b border-white/10 px-3 py-2 text-xs text-slate-200">
                            <div className="flex items-center gap-2">
                                <button
                                    type="button"
                                    onClick={() => setIsPlaying((value) => !value)}
                                    className="rounded-md bg-white/10 px-2.5 py-1 font-semibold text-white hover:bg-white/15"
                                >
                                    {isPlaying ? 'Pause' : 'Play'}
                                </button>
                                <span>{formatTime(currentTime)} / {formatTime(model.totalDurationSec)}</span>
                            </div>
                            <span>{activeScene ? `Scene ${activeScene.index}` : 'No scene selected'}</span>
                        </div>

                        <div className="mx-auto aspect-[9/16] max-h-[540px] bg-gradient-to-br from-slate-900 via-violet-900 to-fuchsia-700 p-4 text-white">
                            <div className="relative flex h-full flex-col overflow-hidden rounded-[28px] border border-white/10 bg-black/25 p-4 shadow-2xl">
                                {activeScene?.mediaPath && isHttpUrl(activeScene.mediaPath) ? (
                                    activeScene.mediaType === 'image' ? (
                                        <img src={activeScene.mediaPath} alt={activeScene.caption} className="absolute inset-0 h-full w-full object-cover opacity-45" />
                                    ) : (
                                        <video key={activeScene.mediaPath} src={activeScene.mediaPath} className="absolute inset-0 h-full w-full object-cover opacity-45" muted autoPlay loop playsInline />
                                    )
                                ) : null}
                                <div className="absolute inset-0 bg-gradient-to-t from-slate-950/80 via-slate-900/25 to-transparent" />
                                <div className="relative z-10 flex items-center justify-between text-[10px] uppercase tracking-[0.2em] text-violet-100/80">
                                    <span>Remotion editor starter</span>
                                    <span>{activeScene?.overlayStyle || 'main'}</span>
                                </div>
                                <div className="relative z-10 mt-auto space-y-3">
                                    <div className="inline-flex rounded-full bg-white/10 px-3 py-1 text-[11px] font-semibold text-violet-100">
                                        {project.topic}
                                    </div>
                                    <h4 className="text-xl font-bold leading-tight sm:text-2xl">{activeScene?.caption || script?.title || project.topic}</h4>
                                    <p className="max-w-xl text-sm leading-6 text-slate-100/90">
                                        {activeScene?.narration || 'Preview your scenes here while keeping the timeline and track items synchronized.'}
                                    </p>
                                    {activeScene?.sourceFacts?.length ? (
                                        <div className="flex flex-wrap gap-1.5 pt-1">
                                            {activeScene.sourceFacts.map((fact, index) => (
                                                <span key={`${activeScene.id}-fact-${index}`} className="rounded-full bg-emerald-500/20 px-2 py-1 text-[11px] text-emerald-100">
                                                    {fact}
                                                </span>
                                            ))}
                                        </div>
                                    ) : null}
                                </div>
                            </div>
                        </div>
                    </div>

                    <div className="rounded-2xl border border-slate-200 bg-white p-3 shadow-sm">
                        <div className="mb-2 flex items-center justify-between gap-2">
                            <div>
                                <h5 className="text-sm font-semibold text-slate-900">Timeline</h5>
                                <p className="text-xs text-slate-500">A Remotion-editor-style synchronized view of visuals, captions, and audio items.</p>
                            </div>
                            <span className="rounded-full bg-slate-100 px-2 py-1 text-[11px] text-slate-600">{model.scenes.length} scenes</span>
                        </div>

                        <input
                            type="range"
                            min={0}
                            max={Math.max(model.totalDurationSec, 1)}
                            step={1 / model.fps}
                            value={currentTime}
                            onChange={(event) => setCurrentTime(Number(event.target.value))}
                            className="w-full accent-violet-600"
                        />

                        <div className="mt-3 space-y-2">
                            {model.tracks.map((track) => (
                                <div key={track.label} className="grid grid-cols-[88px_minmax(0,1fr)] items-center gap-2">
                                    <div className="text-[11px] font-semibold uppercase tracking-wide text-slate-500">{track.label}</div>
                                    <div className="relative h-11 overflow-hidden rounded-lg border border-slate-200 bg-slate-50">
                                        <div className="pointer-events-none absolute inset-y-0 z-10 w-0.5 bg-red-500" style={{ left: `${progress}%` }} />
                                        {track.items.length > 0 ? track.items.map((item) => {
                                            const left = `${(item.startSec / model.totalDurationSec) * 100}%`
                                            const width = `${Math.max(((item.endSec - item.startSec) / model.totalDurationSec) * 100, 4)}%`
                                            return (
                                                <button
                                                    key={item.id}
                                                    type="button"
                                                    onClick={() => setCurrentTime(item.startSec)}
                                                    title={item.description || item.label}
                                                    className={`absolute top-1/2 h-7 -translate-y-1/2 overflow-hidden rounded-md px-2 text-left text-[10px] font-semibold text-white shadow-sm ${item.tone}`}
                                                    style={{ left, width }}
                                                >
                                                    <span className="truncate">{item.label}</span>
                                                </button>
                                            )
                                        }) : <div className="p-2 text-[11px] text-slate-400">No items yet</div>}
                                    </div>
                                </div>
                            ))}
                        </div>
                    </div>
                </div>

                <aside className="space-y-3">
                    <div className="rounded-2xl border border-slate-200 bg-white p-3 shadow-sm">
                        <h5 className="text-sm font-semibold text-slate-900">Inspector</h5>
                        {activeScene ? (
                            <div className="mt-2 space-y-2 text-xs text-slate-700">
                                <div><strong>Scene:</strong> {activeScene.index}</div>
                                <div><strong>Range:</strong> {formatTime(activeScene.startSec)} → {formatTime(activeScene.endSec)}</div>
                                <div><strong>Media:</strong> {activeScene.mediaType || 'visual'}</div>
                                <div><strong>Source fact IDs:</strong> {activeScene.sourceFactIds.join(', ') || '—'}</div>
                                <div className="rounded-lg bg-slate-50 p-2 text-slate-600">
                                    <strong>Narration</strong>
                                    <p className="mt-1 whitespace-pre-wrap">{activeScene.narration || 'No narration for this scene.'}</p>
                                </div>
                            </div>
                        ) : (
                            <p className="mt-2 text-xs text-slate-500">No active scene yet.</p>
                        )}
                    </div>

                    <div className="rounded-2xl border border-slate-200 bg-white p-3 shadow-sm">
                        <h5 className="text-sm font-semibold text-slate-900">Project handoff</h5>
                        <ul className="mt-2 list-disc space-y-1 pl-4 text-xs text-slate-600">
                            <li>Use this timeline view to review the composition before rerendering.</li>
                            <li>Use <code className="rounded bg-slate-100 px-1 py-0.5">timeline</code> or <code className="rounded bg-slate-100 px-1 py-0.5">preview</code> reruns after edits.</li>
                            <li>For full React composition work, launch the official Remotion Studio command shown above.</li>
                        </ul>
                        {latestRenderable ? (
                            <p className="mt-2 text-xs text-emerald-700">Latest playable render: {latestRenderable.kind} · {latestRenderable.resolution}</p>
                        ) : null}
                    </div>
                </aside>
            </div>
        </div>
    )
}
