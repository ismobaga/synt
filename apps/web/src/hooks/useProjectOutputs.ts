import { useCallback, useEffect, useRef, useState } from 'react'
import {
    api,
    type AssetRecord,
    type AudioTrackRecord,
    type RenderRecord,
    type ScriptRecord,
    type SubtitleRecord,
} from '../lib/api'

const POLL_INTERVAL = 4000

interface ProjectOutputsState {
    script: ScriptRecord | null
    assets: AssetRecord[]
    audio: AudioTrackRecord[]
    subtitles: SubtitleRecord[]
    renders: RenderRecord[]
}

const EMPTY_OUTPUTS: ProjectOutputsState = {
    script: null,
    assets: [],
    audio: [],
    subtitles: [],
    renders: [],
}

export function useProjectOutputs(projectId: string | null, enabled: boolean, live = false) {
    const [outputs, setOutputs] = useState<ProjectOutputsState>(EMPTY_OUTPUTS)
    const [loading, setLoading] = useState(false)
    const [error, setError] = useState<string | null>(null)
    const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null)

    const refresh = useCallback(async () => {
        if (!projectId || !enabled) return

        setLoading(true)
        try {
            const [script, assets, audio, subtitles, renders] = await Promise.all([
                api.projects.getScript(projectId).catch(() => null),
                api.projects.getAssets(projectId).catch(() => []),
                api.projects.getAudio(projectId).catch(() => []),
                api.projects.getSubtitles(projectId).catch(() => []),
                api.projects.getRender(projectId).catch(() => []),
            ])

            setOutputs({ script, assets, audio, subtitles, renders })
            setError(null)
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to load project outputs')
        } finally {
            setLoading(false)
        }
    }, [enabled, projectId])

    useEffect(() => {
        if (!projectId || !enabled) {
            setOutputs(EMPTY_OUTPUTS)
            setError(null)
            setLoading(false)
            if (intervalRef.current) {
                clearInterval(intervalRef.current)
                intervalRef.current = null
            }
            return
        }

        void refresh()

        if (live) {
            intervalRef.current = setInterval(() => {
                void refresh()
            }, POLL_INTERVAL)
        }

        return () => {
            if (intervalRef.current) {
                clearInterval(intervalRef.current)
                intervalRef.current = null
            }
        }
    }, [projectId, enabled, live, refresh])

    return {
        ...outputs,
        loading,
        error,
        refresh,
    }
}
