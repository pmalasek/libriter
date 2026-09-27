import {
  ClockIcon,
  CopyIcon,
  FileTextIcon,
  ImageOffIcon,
  LayersIcon,
  LibraryIcon,
  ListMusicIcon,
  TimerOffIcon,
  UserRoundIcon,
  UsersIcon,
  type LucideIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { currentLanguage, type LibraryStats, type SystemInfo } from '@/api/types'
import { useLibraryStats, useSystemInfo } from '@/api/adminHooks'
import { ErrorState } from '@/components/ErrorState'
import { LoadingList } from '@/components/LoadingGrid'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { formatBytes, formatDateTime, formatDuration, formatUptime } from '@/lib/format'
import { cn } from '@/lib/utils'

export function AdminOverviewPage() {
  const stats = useLibraryStats()
  const system = useSystemInfo()

  if (stats.isPending || system.isPending) return <LoadingList count={4} />
  if (stats.error) return <ErrorState error={stats.error} onRetry={() => void stats.refetch()} />
  if (system.error) return <ErrorState error={system.error} onRetry={() => void system.refetch()} />

  return (
    <div className="space-y-6">
      <StatsGrid stats={stats.data} />
      <SystemInfoCard info={system.data} placeholderChapters={stats.data.placeholder_chapters} />
    </div>
  )
}

function StatsGrid({ stats }: { stats: LibraryStats }) {
  const { t } = useTranslation()
  const locale = currentLanguage()
  const tiles = [
    {
      label: t('admin.overview.stats.books'),
      value: stats.books.toLocaleString(locale),
      icon: LibraryIcon,
    },
    {
      label: t('admin.overview.stats.authors'),
      value: stats.authors.toLocaleString(locale),
      icon: UsersIcon,
    },
    {
      label: t('admin.overview.stats.series'),
      value: stats.series.toLocaleString(locale),
      icon: LayersIcon,
    },
    {
      label: t('admin.overview.stats.users'),
      value: stats.users.toLocaleString(locale),
      icon: UserRoundIcon,
    },
    {
      label: t('admin.overview.stats.chapters'),
      value: stats.chapters.toLocaleString(locale),
      icon: ListMusicIcon,
    },
    {
      label: t('admin.overview.stats.totalDuration'),
      value: formatDuration(stats.total_duration_seconds),
      icon: ClockIcon,
    },
  ]

  // Co čeká na doplnění – kvůli tomu se na přehled chodí nejčastěji.
  // Nenulová hodnota se zvýrazní oranžově, ať je hned vidět, co řešit.
  const todo = [
    {
      label: t('admin.overview.todo.withoutCover'),
      value: stats.books_without_cover,
      icon: ImageOffIcon,
    },
    {
      label: t('admin.overview.todo.withoutDescription'),
      value: stats.books_without_description,
      icon: FileTextIcon,
    },
    {
      label: t('admin.overview.todo.placeholderChapters'),
      value: stats.placeholder_chapters,
      icon: TimerOffIcon,
      hint: t('admin.overview.todo.inBooks', { count: stats.books_with_placeholder_chapters }),
    },
    {
      label: t('admin.overview.todo.duplicates'),
      value: stats.duplicate_album_books,
      icon: CopyIcon,
      hint: t('admin.overview.todo.duplicatesHint'),
    },
  ]

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
        {tiles.map((tile) => (
          <StatCard key={tile.label} label={tile.label} value={tile.value} icon={tile.icon} />
        ))}
      </div>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
        {todo.map((tile) => (
          <StatCard
            key={tile.label}
            label={tile.label}
            value={tile.value.toLocaleString(locale)}
            hint={tile.hint}
            icon={tile.icon}
            tone={tile.value === 0 ? 'muted' : 'highlight'}
          />
        ))}
      </div>
    </div>
  )
}

const TONES = {
  brand: 'bg-primary/12 text-primary',
  highlight: 'bg-highlight/15 text-highlight-deep',
  muted: 'bg-muted text-muted-foreground',
} as const

function StatCard({
  label,
  value,
  hint,
  icon: Icon,
  tone = 'brand',
}: {
  label: string
  value: string
  hint?: string
  icon: LucideIcon
  tone?: keyof typeof TONES
}) {
  return (
    <Card>
      <CardContent className="flex items-start gap-3">
        <span
          className={cn(
            'flex size-10 shrink-0 items-center justify-center rounded-xl',
            TONES[tone],
          )}
        >
          <Icon className="size-5" />
        </span>
        <div className="min-w-0">
          <p className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
            {label}
          </p>
          <p
            className={cn(
              'font-heading text-2xl font-bold tabular-nums',
              tone === 'muted' && 'text-muted-foreground',
            )}
          >
            {value}
          </p>
          {hint ? <p className="mt-0.5 text-xs text-muted-foreground">{hint}</p> : null}
        </div>
      </CardContent>
    </Card>
  )
}

function SystemInfoCard({
  info,
  placeholderChapters,
}: {
  info: SystemInfo
  placeholderChapters: number
}) {
  const { t } = useTranslation()
  const rows: { label: string; value: React.ReactNode }[] = [
    { label: t('admin.overview.system.version'), value: info.version },
    { label: 'Go', value: `${info.go_version} (${info.os})` },
    { label: t('admin.overview.system.environment'), value: info.env },
    {
      label: t('admin.overview.system.runningSince'),
      value: `${formatDateTime(info.started_at)} (${formatUptime(info.uptime_seconds)})`,
    },
    { label: t('admin.overview.system.database'), value: <Path value={info.db_path} /> },
    { label: t('admin.overview.system.audio'), value: <Path value={info.audio_root} /> },
    { label: t('admin.overview.system.covers'), value: <Path value={info.cover_root} /> },
    {
      label: t('admin.overview.system.authorPhotos'),
      value: <Path value={info.author_image_root} />,
    },
    {
      label: 'ffprobe',
      value: info.ffprobe_available ? (
        <span className="flex flex-wrap items-center gap-2">
          <Badge variant="secondary">{t('admin.overview.system.ffprobeAvailable')}</Badge>
          <Path value={info.ffprobe_path} />
        </span>
      ) : (
        <span className="flex flex-wrap items-center gap-2">
          <Badge variant="destructive">{t('admin.overview.system.ffprobeMissing')}</Badge>
          <span className="text-xs text-muted-foreground">
            {t('admin.overview.system.ffprobeMissingHint')}
            {placeholderChapters > 0
              ? t('admin.overview.system.ffprobeMissingCount', { n: placeholderChapters })
              : ''}
          </span>
        </span>
      ),
    },
  ]

  if (info.disk) {
    rows.push({
      label: t('admin.overview.system.freeSpace'),
      value: t('admin.overview.system.freeSpaceValue', {
        free: formatBytes(info.disk.free_bytes),
        total: formatBytes(info.disk.total_bytes),
      }),
    })
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('admin.overview.system.title')}</CardTitle>
        <CardDescription>{t('admin.overview.system.description')}</CardDescription>
      </CardHeader>
      <CardContent>
        <dl className="grid grid-cols-1 gap-x-6 gap-y-3 sm:grid-cols-[10rem_1fr]">
          {rows.map((row) => (
            <div key={row.label} className="sm:contents">
              <dt className="text-sm text-muted-foreground">{row.label}</dt>
              <dd className="min-w-0 text-sm">{row.value}</dd>
            </div>
          ))}
        </dl>
      </CardContent>
    </Card>
  )
}

function Path({ value }: { value: string }) {
  return <span className="break-all font-mono text-xs">{value}</span>
}
