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

function getStepState(index: number, currentIdx: number, status: string) {
  if (status === 'done') return 'complete'
  if (index < currentIdx) return 'complete'
  if (index === currentIdx && status === 'failed') return 'failed'
  if (index === currentIdx) return 'current'
  return 'upcoming'
}

export function PipelineProgress({ currentStage, status }: PipelineProgressProps) {
  const currentIdx = STAGES.indexOf(currentStage as (typeof STAGES)[number])

  return (
    <div className="w-full">
      <div className="mb-1 flex items-center justify-between text-sm text-gray-500">
        <span>{STAGE_LABELS[currentStage] ?? currentStage}</span>
        <span className="text-xs">
          {status === 'done' ? `${STAGES.length} / ${STAGES.length}` : currentIdx >= 0 ? `${currentIdx + 1} / ${STAGES.length}` : '0 / 12'}
        </span>
      </div>

      <div className="h-2 w-full overflow-hidden rounded-full bg-gray-200">
        <div
          className={`h-full rounded-full transition-all duration-500 ${status === 'failed' ? 'bg-red-500' : status === 'done' ? 'bg-green-500' : 'bg-purple-600'
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

      <div className="mt-3 grid grid-cols-2 gap-1.5 sm:grid-cols-3">
        {STAGES.map((stage, index) => {
          const stepState = getStepState(index, currentIdx, status)
          const styles =
            stepState === 'complete'
              ? 'border-green-200 bg-green-50 text-green-700'
              : stepState === 'current'
                ? 'border-purple-200 bg-purple-50 text-purple-700'
                : stepState === 'failed'
                  ? 'border-red-200 bg-red-50 text-red-700'
                  : 'border-gray-200 bg-gray-50 text-gray-500'

          return (
            <div key={stage} className={`rounded-md border px-2 py-1 text-[11px] font-medium ${styles}`}>
              <span className="mr-1 opacity-70">{index + 1}.</span>
              {STAGE_LABELS[stage]}
            </div>
          )
        })}
      </div>
    </div>
  )
}
