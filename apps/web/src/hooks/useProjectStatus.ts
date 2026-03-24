import { useState, useEffect, useRef } from 'react'
import { api } from '../lib/api'

const POLL_INTERVAL = 3000

const TERMINAL_STAGES = new Set(['finalize', 'done'])
const TERMINAL_STATUSES = new Set(['done', 'failed'])

export function useProjectStatus(projectId: string | null) {
  const [status, setStatus] = useState<string>('')
  const [stage, setStage] = useState<string>('')
  const [error, setError] = useState<string | null>(null)
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null)

  useEffect(() => {
    if (!projectId) return

    const poll = async () => {
      try {
        const result = await api.projects.status(projectId)
        setStatus(result.status)
        setStage(result.current_stage)
        setError(result.error_message ?? null)

        if (TERMINAL_STATUSES.has(result.status) || TERMINAL_STAGES.has(result.current_stage)) {
          if (intervalRef.current) {
            clearInterval(intervalRef.current)
            intervalRef.current = null
          }
        }
      } catch {
        // silently retry
      }
    }

    poll()
    intervalRef.current = setInterval(poll, POLL_INTERVAL)

    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current)
        intervalRef.current = null
      }
    }
  }, [projectId])

  return { status, stage, error }
}
