import { cn } from '../../lib/utils'

interface BadgeProps {
  status: string
  className?: string
}

const statusColors: Record<string, string> = {
  draft: 'bg-gray-100 text-gray-700',
  queued: 'bg-blue-100 text-blue-700',
  processing: 'bg-yellow-100 text-yellow-700',
  done: 'bg-green-100 text-green-700',
  failed: 'bg-red-100 text-red-700',
}

export function StatusBadge({ status, className }: BadgeProps) {
  const color = statusColors[status] ?? 'bg-gray-100 text-gray-600'
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium capitalize',
        color,
        className
      )}
    >
      {status === 'processing' && (
        <span className="mr-1.5 inline-block h-2 w-2 animate-pulse rounded-full bg-yellow-500" />
      )}
      {status}
    </span>
  )
}
