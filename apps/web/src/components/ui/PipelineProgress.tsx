const STAGES = [
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
] as const

const STAGE_LABELS: Record<string, string> = {
  created: 'Created',
  script_generation: 'Generating Script',
  script_validation: 'Validating Script',
  media_search: 'Searching Media',
  media_prepare: 'Preparing Media',
  voice_generation: 'Generating Voice',
  subtitle_generation: 'Creating Subtitles',
  music_selection: 'Selecting Music',
  timeline_build: 'Building Timeline',
  render_preview: 'Rendering Preview',
  render_final: 'Rendering Final Video',
  render_thumbnail: 'Extracting Thumbnail',
  finalize: 'Finalizing',
}

interface PipelineProgressProps {
  currentStage: string
  status: string
}

export function PipelineProgress({ currentStage, status }: PipelineProgressProps) {
  const currentIdx = STAGES.indexOf(currentStage as (typeof STAGES)[number])

  return (
    <div className="w-full">
      <div className="flex items-center justify-between mb-1 text-sm text-gray-500">
        <span>{STAGE_LABELS[currentStage] ?? currentStage}</span>
        <span className="text-xs">
          {currentIdx >= 0 ? `${currentIdx + 1} / ${STAGES.length}` : ''}
        </span>
      </div>
      <div className="h-2 w-full rounded-full bg-gray-200 overflow-hidden">
        <div
          className={`h-full rounded-full transition-all duration-500 ${
            status === 'failed'
              ? 'bg-red-500'
              : status === 'done'
              ? 'bg-green-500'
              : 'bg-purple-600'
          }`}
          style={{
            width:
              status === 'done'
                ? '100%'
                : currentIdx >= 0
                ? `${((currentIdx + 1) / STAGES.length) * 100}%`
                : '0%',
          }}
        />
      </div>
    </div>
  )
}
