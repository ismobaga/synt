import { useEffect, useMemo, useState } from 'react'
import type { AssetRecord, AudioTrackRecord, Project, RenderRecord, ScriptRecord, SubtitleRecord } from '../../lib/api'

type TimelineItem = {
    id: string
    label: string
    startSec: number
    endSec: number
    tone: string
    description?: string
    sceneId?: string
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

type InspectorTab = 'scene' | 'script' | 'sources'

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

function formatTimecode(seconds: number, fps: number) {
    const safe = Math.max(0, seconds)
    const minutes = Math.floor(safe / 60)
    const secs = Math.floor(safe % 60)
    const frames = Math.floor((safe - Math.floor(safe)) * fps)
    return `${String(minutes).padStart(2, '0')}:${String(secs).padStart(2, '0')}:${String(frames).padStart(2, '0')}`
}

function buildTimeTicks(totalDurationSec: number) {
    const safeDuration = Math.max(totalDurationSec, 1)
    const step = safeDuration <= 12 ? 1 : safeDuration <= 30 ? 2 : safeDuration <= 90 ? 5 : 10
    const ticks = [] as Array<{ value: number; left: string }>

    for (let value = 0; value <= safeDuration; value += step) {
        ticks.push({ value, left: `${(value / safeDuration) * 100}%` })
    }

    if (ticks[ticks.length - 1]?.value !== safeDuration) {
        ticks.push({ value: safeDuration, left: '100%' })
    }

    return ticks
}

function mediaLabel(mediaType: string, mediaPath: string) {
    if (!mediaPath) return mediaType || 'visual'
    const parts = mediaPath.split('/').filter(Boolean)
    return parts[parts.length - 1] || mediaType || 'visual'
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
                tone: 'bg-gradient-to-r from-violet-500 to-fuchsia-500',
                description: mediaLabel(scene.mediaType, scene.mediaPath),
                sceneId: scene.id,
            })),
        },
        {
            label: 'Captions',
            items: scenes.map((scene) => ({
                id: `${scene.id}-caption`,
                label: scene.caption || `Caption ${scene.index}`,
                startSec: scene.startSec,
                endSec: scene.endSec,
                tone: 'bg-gradient-to-r from-emerald-500 to-teal-500',
                description: scene.overlayStyle || 'caption overlay',
                sceneId: scene.id,
            })),
        },
        {
            label: 'Voiceover',
            items: voiceTracks.map((track, index) => ({
                id: `${track.id}-voice`,
                label: track.voice_name || `Voice ${index + 1}`,
                startSec: 0,
                endSec: Math.max(toNumber(track.duration_sec, totalDurationSec), 1),
                tone: 'bg-gradient-to-r from-sky-500 to-cyan-500',
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
                tone: 'bg-gradient-to-r from-amber-500 to-orange-500',
                description: 'music bed',
            })),
        },
    ]

    if (subtitles.length > 0 && tracks[1].items.length === 0) {
        tracks[1].items.push({
            id: `${subtitles[0].id}-subtitle`,
            label: subtitles[0].format.toUpperCase(),
            startSec: 0,
            endSec: totalDurationSec,
            tone: 'bg-gradient-to-r from-emerald-500 to-teal-500',
            description: 'subtitle track',
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
    const [selectedSceneId, setSelectedSceneId] = useState<string | null>(null)
    const [selectedTrack, setSelectedTrack] = useState('Visuals')
    const [zoomLevel, setZoomLevel] = useState(140)
    const [loopPlayback, setLoopPlayback] = useState(true)
    const [showSafeArea, setShowSafeArea] = useState(true)
    const [inspectorTab, setInspectorTab] = useState<InspectorTab>('scene')

    useEffect(() => {
        if (!model) return
        setCurrentTime((value) => clamp(value, 0, model.totalDurationSec))
        setSelectedSceneId((value) => {
            if (value && model.scenes.some((scene) => scene.id === value)) return value
            return model.scenes[0]?.id ?? null
        })
    }, [model])

    useEffect(() => {
        if (!model || !isPlaying) return
        const interval = window.setInterval(() => {
            setCurrentTime((value) => {
                const next = value + 1 / model.fps
                if (next >= model.totalDurationSec) {
                    if (loopPlayback) return 0
                    setIsPlaying(false)
                    return model.totalDurationSec
                }
                return next
            })
        }, 1000 / model.fps)

        return () => window.clearInterval(interval)
    }, [isPlaying, loopPlayback, model])

    if (!model) {
        return <p className="text-xs text-slate-500">Build the timeline first to open the Remotion editor workspace.</p>
    }

    const jumpTo = (time: number, sceneId?: string, trackLabel?: string) => {
        setCurrentTime(clamp(time, 0, model.totalDurationSec))
        if (sceneId) setSelectedSceneId(sceneId)
        if (trackLabel) setSelectedTrack(trackLabel)
    }

    const activeScene = model.scenes.find((scene) => currentTime >= scene.startSec && currentTime < scene.endSec)
        ?? model.scenes[model.scenes.length - 1]
        ?? null
    const inspectedScene = model.scenes.find((scene) => scene.id === selectedSceneId) ?? activeScene
    const progress = model.totalDurationSec > 0 ? (currentTime / model.totalDurationSec) * 100 : 0
    const renderEngine = project.template_id.toLowerCase().startsWith('remotion_') ? 'Remotion' : 'FFmpeg + Remotion workspace'
    const latestRenderable = renders.find((render) => render.status === 'done' && (render.kind === 'preview' || render.kind === 'final'))
    const timeTicks = buildTimeTicks(model.totalDurationSec)
    const factCount = model.scenes.reduce((total, scene) => total + scene.sourceFacts.length, 0)
    const totalTracks = model.tracks.filter((track) => track.items.length > 0).length

    return (
        <div className="space-y-4">
            <div className="overflow-hidden rounded-2xl border border-slate-200 bg-gradient-to-r from-slate-950 via-violet-950 to-fuchsia-900 p-4 text-white shadow-sm">
                <div className="flex flex-wrap items-start justify-between gap-3">
                    <div>
                        <p className="text-[11px] font-semibold uppercase tracking-[0.25em] text-violet-200">Remotion studio</p>
                        <h4 className="mt-1 text-lg font-semibold">Rich editor workspace for timing, shots, captions, and source grounding</h4>
                        <p className="mt-1 max-w-2xl text-sm text-slate-200/90">
                            Review every beat before rerendering, inspect grounded facts, and hand off to the full React composition flow when needed.
                        </p>
                    </div>
                    <div className="flex flex-wrap items-center gap-2 text-xs">
                        <span className="rounded-full border border-white/15 bg-white/10 px-2.5 py-1">{renderEngine}</span>
                        <span className="rounded-full border border-white/15 bg-white/10 px-2.5 py-1">{model.width}×{model.height}</span>
                        <span className="rounded-full border border-white/15 bg-white/10 px-2.5 py-1">{model.fps} fps</span>
                        <code className="rounded-lg bg-white px-2.5 py-1 text-[11px] text-slate-800">cd apps/web && npm run remotion:studio</code>
                    </div>
                </div>

                <div className="mt-4 grid gap-2 sm:grid-cols-2 xl:grid-cols-4">
                    <div className="rounded-xl border border-white/10 bg-white/5 p-3">
                        <div className="text-[10px] uppercase tracking-[0.2em] text-slate-300">Sequence</div>
                        <div className="mt-1 text-lg font-semibold">{model.scenes.length} scenes</div>
                        <div className="text-xs text-slate-300">Hook to CTA mapped on one strip</div>
                    </div>
                    <div className="rounded-xl border border-white/10 bg-white/5 p-3">
                        <div className="text-[10px] uppercase tracking-[0.2em] text-slate-300">Runtime</div>
                        <div className="mt-1 text-lg font-semibold">{formatTime(model.totalDurationSec)}</div>
                        <div className="text-xs text-slate-300">Current playhead {formatTimecode(currentTime, model.fps)}</div>
                    </div>
                    <div className="rounded-xl border border-white/10 bg-white/5 p-3">
                        <div className="text-[10px] uppercase tracking-[0.2em] text-slate-300">Tracks</div>
                        <div className="mt-1 text-lg font-semibold">{totalTracks}</div>
                        <div className="text-xs text-slate-300">Visuals, captions, voiceover, and music</div>
                    </div>
                    <div className="rounded-xl border border-white/10 bg-white/5 p-3">
                        <div className="text-[10px] uppercase tracking-[0.2em] text-slate-300">Grounding</div>
                        <div className="mt-1 text-lg font-semibold">{factCount}</div>
                        <div className="text-xs text-slate-300">Referenced source facts across the cut</div>
                    </div>
                </div>
            </div>

            <div className="grid gap-4 xl:grid-cols-[minmax(0,1.2fr)_340px]">
                <div className="space-y-3">
                    <div className="overflow-hidden rounded-2xl border border-slate-200 bg-slate-950 shadow-sm">
                        <div className="flex flex-wrap items-center justify-between gap-2 border-b border-white/10 px-3 py-2 text-xs text-slate-200">
                            <div className="flex flex-wrap items-center gap-2">
                                <button
                                    type="button"
                                    onClick={() => setIsPlaying((value) => !value)}
                                    className="rounded-md bg-violet-500 px-2.5 py-1 font-semibold text-white hover:bg-violet-400"
                                >
                                    {isPlaying ? 'Pause' : 'Play'}
                                </button>
                                <button
                                    type="button"
                                    onClick={() => jumpTo(currentTime - 1, activeScene?.id)}
                                    className="rounded-md bg-white/10 px-2 py-1 hover:bg-white/15"
                                >
                                    -1s
                                </button>
                                <button
                                    type="button"
                                    onClick={() => jumpTo(currentTime + 1, activeScene?.id)}
                                    className="rounded-md bg-white/10 px-2 py-1 hover:bg-white/15"
                                >
                                    +1s
                                </button>
                                <button
                                    type="button"
                                    onClick={() => setLoopPlayback((value) => !value)}
                                    className={`rounded-md px-2 py-1 ${loopPlayback ? 'bg-emerald-500/20 text-emerald-100' : 'bg-white/10 hover:bg-white/15'}`}
                                >
                                    Loop {loopPlayback ? 'on' : 'off'}
                                </button>
                                <button
                                    type="button"
                                    onClick={() => setShowSafeArea((value) => !value)}
                                    className={`rounded-md px-2 py-1 ${showSafeArea ? 'bg-sky-500/20 text-sky-100' : 'bg-white/10 hover:bg-white/15'}`}
                                >
                                    Safe area {showSafeArea ? 'on' : 'off'}
                                </button>
                            </div>
                            <span>{formatTime(currentTime)} / {formatTime(model.totalDurationSec)} · {activeScene ? `Scene ${activeScene.index}` : 'No scene selected'}</span>
                        </div>

                        <div className="mx-auto aspect-[9/16] max-h-[560px] bg-gradient-to-br from-slate-900 via-violet-900 to-fuchsia-700 p-4 text-white">
                            <div className="relative flex h-full flex-col overflow-hidden rounded-[28px] border border-white/10 bg-black/25 p-4 shadow-2xl">
                                {activeScene?.mediaPath && isHttpUrl(activeScene.mediaPath) ? (
                                    activeScene.mediaType === 'image' ? (
                                        <img src={activeScene.mediaPath} alt={activeScene.caption} className="absolute inset-0 h-full w-full object-cover opacity-45" />
                                    ) : (
                                        <video key={activeScene.mediaPath} src={activeScene.mediaPath} className="absolute inset-0 h-full w-full object-cover opacity-45" muted autoPlay loop playsInline />
                                    )
                                ) : null}
                                <div className="absolute inset-0 bg-gradient-to-t from-slate-950/85 via-slate-900/30 to-transparent" />
                                {showSafeArea ? (
                                    <div className="pointer-events-none absolute inset-5 rounded-[22px] border border-dashed border-white/30" />
                                ) : null}
                                <div className="relative z-10 flex items-center justify-between text-[10px] uppercase tracking-[0.2em] text-violet-100/80">
                                    <span>Preview canvas</span>
                                    <span>{activeScene?.overlayStyle || 'main'}</span>
                                </div>
                                <div className="relative z-10 mt-auto space-y-3">
                                    <div className="inline-flex rounded-full bg-white/10 px-3 py-1 text-[11px] font-semibold text-violet-100">
                                        {project.topic}
                                    </div>
                                    <h4 className="text-xl font-bold leading-tight sm:text-2xl">{activeScene?.caption || script?.title || project.topic}</h4>
                                    <p className="max-w-xl text-sm leading-6 text-slate-100/90">
                                        {activeScene?.narration || 'Preview your scenes here while keeping the timeline, captions, and audio tracks synchronized.'}
                                    </p>
                                    <div className="overflow-hidden rounded-full bg-white/10">
                                        <div className="h-1.5 rounded-full bg-gradient-to-r from-violet-400 to-fuchsia-400" style={{ width: `${progress}%` }} />
                                    </div>
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

                        <div className="border-t border-white/10 bg-slate-950/70 px-3 py-3">
                            <div className="mb-2 flex items-center justify-between gap-2 text-xs text-slate-300">
                                <span className="font-semibold text-white">Scene strip</span>
                                <span>Click any beat to jump the playhead</span>
                            </div>
                            <div className="flex gap-2 overflow-x-auto pb-1">
                                {model.scenes.map((scene) => {
                                    const active = scene.id === inspectedScene?.id
                                    return (
                                        <button
                                            key={scene.id}
                                            type="button"
                                            onClick={() => jumpTo(scene.startSec, scene.id, 'Visuals')}
                                            className={`min-w-[160px] rounded-xl border px-3 py-2 text-left transition ${active ? 'border-violet-300 bg-violet-500/20 text-white' : 'border-white/10 bg-white/5 text-slate-200 hover:border-violet-300/60 hover:bg-white/10'}`}
                                        >
                                            <div className="text-[10px] uppercase tracking-[0.2em] text-slate-300">Scene {scene.index}</div>
                                            <div className="mt-1 line-clamp-2 text-sm font-semibold">{scene.caption}</div>
                                            <div className="mt-1 text-[11px] text-slate-300">{formatTime(scene.startSec)} → {formatTime(scene.endSec)} · {scene.mediaType}</div>
                                        </button>
                                    )
                                })}
                            </div>
                        </div>
                    </div>

                    <div className="rounded-2xl border border-slate-200 bg-white p-3 shadow-sm">
                        <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
                            <div>
                                <h5 className="text-sm font-semibold text-slate-900">Zoomable timeline</h5>
                                <p className="text-xs text-slate-500">Review synchronized visuals, captions, and audio clips in a denser studio layout.</p>
                            </div>
                            <div className="flex items-center gap-2 text-xs text-slate-600">
                                <span>Zoom</span>
                                <input
                                    type="range"
                                    min={100}
                                    max={220}
                                    step={10}
                                    value={zoomLevel}
                                    onChange={(event) => setZoomLevel(Number(event.target.value))}
                                    className="w-24 accent-violet-600"
                                />
                                <span className="w-10 text-right font-semibold text-slate-800">{zoomLevel}%</span>
                            </div>
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

                        <div className="mt-3 overflow-x-auto">
                            <div style={{ width: `${zoomLevel}%`, minWidth: '100%' }}>
                                <div className="grid grid-cols-[96px_minmax(0,1fr)] items-center gap-2 pb-2">
                                    <div className="text-[11px] font-semibold uppercase tracking-wide text-slate-500">Time</div>
                                    <div className="relative h-6 rounded-md border border-slate-200 bg-slate-50">
                                        {timeTicks.map((tick) => (
                                            <div key={`tick-${tick.value}`} className="absolute inset-y-0" style={{ left: tick.left }}>
                                                <div className="h-full border-l border-slate-200" />
                                                <span className="absolute left-1 top-1 text-[10px] text-slate-500">{formatTime(tick.value)}</span>
                                            </div>
                                        ))}
                                        <div className="pointer-events-none absolute inset-y-0 z-10 w-0.5 bg-red-500" style={{ left: `${progress}%` }} />
                                    </div>
                                </div>

                                <div className="space-y-2">
                                    {model.tracks.map((track) => {
                                        const isTrackSelected = selectedTrack === track.label
                                        return (
                                            <div key={track.label} className="grid grid-cols-[96px_minmax(0,1fr)] items-center gap-2">
                                                <button
                                                    type="button"
                                                    onClick={() => setSelectedTrack(track.label)}
                                                    className={`rounded-lg border px-2 py-2 text-left text-[11px] font-semibold uppercase tracking-wide ${isTrackSelected ? 'border-violet-200 bg-violet-50 text-violet-700' : 'border-slate-200 bg-slate-50 text-slate-500 hover:border-violet-200 hover:bg-violet-50/60'}`}
                                                >
                                                    <div>{track.label}</div>
                                                    <div className="mt-1 text-[10px] normal-case tracking-normal text-slate-500">{track.items.length} clips</div>
                                                </button>
                                                <div className={`relative h-12 overflow-hidden rounded-lg border ${isTrackSelected ? 'border-violet-200 bg-violet-50/40' : 'border-slate-200 bg-slate-50'}`}>
                                                    <div className="pointer-events-none absolute inset-y-0 z-10 w-0.5 bg-red-500" style={{ left: `${progress}%` }} />
                                                    {track.items.length > 0 ? track.items.map((item) => {
                                                        const left = `${(item.startSec / model.totalDurationSec) * 100}%`
                                                        const width = `${Math.max(((item.endSec - item.startSec) / model.totalDurationSec) * 100, 4)}%`
                                                        const itemActive = item.sceneId ? item.sceneId === inspectedScene?.id : false
                                                        return (
                                                            <button
                                                                key={item.id}
                                                                type="button"
                                                                onClick={() => jumpTo(item.startSec, item.sceneId, track.label)}
                                                                title={item.description || item.label}
                                                                className={`absolute top-1/2 h-8 -translate-y-1/2 overflow-hidden rounded-md px-2 text-left text-[10px] font-semibold text-white shadow-sm ${item.tone} ${itemActive ? 'ring-2 ring-white/80' : ''}`}
                                                                style={{ left, width }}
                                                            >
                                                                <span className="truncate">{item.label}</span>
                                                            </button>
                                                        )
                                                    }) : <div className="p-2 text-[11px] text-slate-400">No items yet</div>}
                                                </div>
                                            </div>
                                        )
                                    })}
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                <aside className="space-y-3">
                    <div className="rounded-2xl border border-slate-200 bg-white p-3 shadow-sm">
                        <div className="flex items-center justify-between gap-2">
                            <h5 className="text-sm font-semibold text-slate-900">Inspector</h5>
                            <span className="rounded-full bg-slate-100 px-2 py-1 text-[11px] text-slate-600">{selectedTrack}</span>
                        </div>

                        <div className="mt-3 flex gap-1 rounded-xl bg-slate-100 p-1 text-[11px] font-medium text-slate-600">
                            {(['scene', 'script', 'sources'] as InspectorTab[]).map((tab) => (
                                <button
                                    key={tab}
                                    type="button"
                                    onClick={() => setInspectorTab(tab)}
                                    className={`flex-1 rounded-lg px-2 py-1.5 capitalize ${inspectorTab === tab ? 'bg-white text-slate-900 shadow-sm' : 'hover:text-slate-900'}`}
                                >
                                    {tab}
                                </button>
                            ))}
                        </div>

                        {inspectorTab === 'scene' ? (
                            inspectedScene ? (
                                <div className="mt-3 space-y-2 text-xs text-slate-700">
                                    <div className="rounded-lg bg-slate-50 p-2.5">
                                        <div className="text-[10px] uppercase tracking-[0.2em] text-slate-500">Selected clip</div>
                                        <div className="mt-1 text-sm font-semibold text-slate-900">Scene {inspectedScene.index}: {inspectedScene.caption}</div>
                                        <div className="mt-1 text-slate-600">{formatTime(inspectedScene.startSec)} → {formatTime(inspectedScene.endSec)} · {inspectedScene.mediaType}</div>
                                    </div>
                                    <div><strong>Overlay style:</strong> {inspectedScene.overlayStyle || 'main'}</div>
                                    <div><strong>Media file:</strong> {mediaLabel(inspectedScene.mediaType, inspectedScene.mediaPath)}</div>
                                    <div><strong>Source fact IDs:</strong> {inspectedScene.sourceFactIds.join(', ') || '—'}</div>
                                    <div className="rounded-lg bg-slate-50 p-2.5 text-slate-600">
                                        <strong>Narration</strong>
                                        <p className="mt-1 whitespace-pre-wrap">{inspectedScene.narration || 'No narration for this scene.'}</p>
                                    </div>
                                </div>
                            ) : (
                                <p className="mt-3 text-xs text-slate-500">No active scene yet.</p>
                            )
                        ) : null}

                        {inspectorTab === 'script' ? (
                            <div className="mt-3 space-y-2 text-xs text-slate-700">
                                <div className="rounded-lg bg-slate-50 p-2.5">
                                    <div className="text-[10px] uppercase tracking-[0.2em] text-slate-500">Title</div>
                                    <div className="mt-1 text-sm font-semibold text-slate-900">{script?.title || project.topic}</div>
                                </div>
                                <div><strong>Status:</strong> {project.status}</div>
                                <div><strong>Template:</strong> {project.template_id}</div>
                                <div><strong>Duration target:</strong> {project.duration_sec}s</div>
                                <div><strong>Latest render:</strong> {latestRenderable ? `${latestRenderable.kind} · ${latestRenderable.resolution}` : 'No completed render yet'}</div>
                            </div>
                        ) : null}

                        {inspectorTab === 'sources' ? (
                            <div className="mt-3 space-y-2 text-xs text-slate-700">
                                {inspectedScene?.sourceFacts?.length ? (
                                    <div className="space-y-2">
                                        {inspectedScene.sourceFacts.map((fact, index) => (
                                            <div key={`${inspectedScene.id}-source-${index}`} className="rounded-lg bg-emerald-50 p-2 text-emerald-900">
                                                <span className="font-semibold">Fact {index + 1}:</span> {fact}
                                            </div>
                                        ))}
                                    </div>
                                ) : (
                                    <p className="rounded-lg bg-slate-50 p-2.5 text-slate-500">No grounded facts are attached to the current scene yet.</p>
                                )}
                            </div>
                        ) : null}
                    </div>

                    <div className="rounded-2xl border border-slate-200 bg-white p-3 shadow-sm">
                        <h5 className="text-sm font-semibold text-slate-900">Next render handoff</h5>
                        <ul className="mt-2 list-disc space-y-1 pl-4 text-xs text-slate-600">
                            <li>Use this studio to review pacing before rerunning <code className="rounded bg-slate-100 px-1 py-0.5">timeline</code> or <code className="rounded bg-slate-100 px-1 py-0.5">preview</code>.</li>
                            <li>Keep the playhead on a clip, then inspect narration and sources on the right.</li>
                            <li>Launch the full Remotion Studio command above for composition-level React edits.</li>
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
