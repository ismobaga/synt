import { StatusBadge } from '../../components/ui/StatusBadge'
import { PipelineProgress } from '../../components/ui/PipelineProgress'
import { useProjectStatus } from '../../hooks/useProjectStatus'
import type { Project } from '../../lib/api'

interface ProjectCardProps {
  project: Project
  onGenerate: (id: string) => void
  onDelete: (id: string) => void
  isGenerating: boolean
}

export function ProjectCard({ project, onGenerate, onDelete, isGenerating }: ProjectCardProps) {
  const { status, stage } = useProjectStatus(
    project.status === 'processing' || project.status === 'queued' ? project.id : null
  )

  const currentStatus = status || project.status
  const currentStage = stage || project.current_stage

  const isActive = currentStatus === 'queued' || currentStatus === 'processing'
  const isDone = currentStatus === 'done'
  const isFailed = currentStatus === 'failed'

  const platformLabels: Record<string, string> = {
    youtube_shorts: '▶ YouTube Shorts',
    tiktok: '♪ TikTok',
    instagram_reels: '◉ Instagram Reels',
  }

  return (
    <div className="rounded-xl border border-gray-200 bg-white p-5 shadow-sm hover:shadow-md transition-shadow">
      <div className="flex items-start justify-between gap-2 mb-3">
        <div>
          <h3 className="font-semibold text-gray-900 text-sm leading-snug">{project.topic}</h3>
          <p className="text-xs text-gray-500 mt-0.5">
            {platformLabels[project.platform] ?? project.platform} · {project.duration_sec}s
          </p>
        </div>
        <StatusBadge status={currentStatus} />
      </div>

      {isActive && (
        <div className="mb-3">
          <PipelineProgress currentStage={currentStage} status={currentStatus} />
        </div>
      )}

      {isFailed && project.error_message && (
        <p className="text-xs text-red-600 mb-3 bg-red-50 rounded p-2">{project.error_message}</p>
      )}

      <div className="flex gap-2 mt-3">
        {currentStatus === 'draft' && (
          <button
            onClick={() => onGenerate(project.id)}
            disabled={isGenerating}
            className="flex-1 rounded-lg bg-purple-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-purple-700 disabled:opacity-50 transition-colors"
          >
            {isGenerating ? 'Starting…' : '▶ Generate'}
          </button>
        )}
        {isFailed && (
          <button
            onClick={() => onGenerate(project.id)}
            disabled={isGenerating}
            className="flex-1 rounded-lg bg-orange-500 px-3 py-1.5 text-xs font-semibold text-white hover:bg-orange-600 disabled:opacity-50 transition-colors"
          >
            ↺ Retry
          </button>
        )}
        {isDone && (
          <button className="flex-1 rounded-lg bg-green-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-green-700 transition-colors">
            ↓ Download
          </button>
        )}
        <button
          onClick={() => onDelete(project.id)}
          className="rounded-lg border border-gray-200 px-3 py-1.5 text-xs font-medium text-gray-600 hover:bg-gray-50 transition-colors"
        >
          Delete
        </button>
      </div>
    </div>
  )
}
