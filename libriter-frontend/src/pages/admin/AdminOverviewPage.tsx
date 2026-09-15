import {
  ClockIcon,
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
import type { LibraryStats, SystemInfo } from '@/api/types'
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
  const tiles = [
    { label: 'Knihy', value: stats.books.toLocaleString('cs-CZ'), icon: LibraryIcon },
    { label: 'Autoři', value: stats.authors.toLocaleString('cs-CZ'), icon: UsersIcon },
    { label: 'Série', value: stats.series.toLocaleString('cs-CZ'), icon: LayersIcon },
    { label: 'Uživatelé', value: stats.users.toLocaleString('cs-CZ'), icon: UserRoundIcon },
    { label: 'Kapitoly', value: stats.chapters.toLocaleString('cs-CZ'), icon: ListMusicIcon },
    {
      label: 'Celková délka',
      value: formatDuration(stats.total_duration_seconds),
      icon: ClockIcon,
    },
  ]

  // Co čeká na doplnění – kvůli tomu se na přehled chodí nejčastěji.
  // Nenulová hodnota se zvýrazní oranžově, ať je hned vidět, co řešit.
  const todo = [
    { label: 'Knihy bez obálky', value: stats.books_without_cover, icon: ImageOffIcon },
    { label: 'Knihy bez popisu', value: stats.books_without_description, icon: FileTextIcon },
    {
      label: 'Kapitoly s délkou 1 s',
      value: stats.placeholder_chapters,
      icon: TimerOffIcon,
      hint: `v ${stats.books_with_placeholder_chapters} knihách`,
    },
  ]

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
        {tiles.map((tile) => (
          <StatCard key={tile.label} label={tile.label} value={tile.value} icon={tile.icon} />
        ))}
      </div>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
        {todo.map((tile) => (
          <StatCard
            key={tile.label}
            label={tile.label}
            value={tile.value.toLocaleString('cs-CZ')}
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
  const rows: { label: string; value: React.ReactNode }[] = [
    { label: 'Verze', value: info.version },
    { label: 'Go', value: `${info.go_version} (${info.os})` },
    { label: 'Prostředí', value: info.env },
    {
      label: 'Běží od',
      value: `${formatDateTime(info.started_at)} (${formatUptime(info.uptime_seconds)})`,
    },
    { label: 'Databáze', value: <Path value={info.db_path} /> },
    { label: 'Audio', value: <Path value={info.audio_root} /> },
    { label: 'Obálky', value: <Path value={info.cover_root} /> },
    { label: 'Fotky autorů', value: <Path value={info.author_image_root} /> },
    {
      label: 'ffprobe',
      value: info.ffprobe_available ? (
        <span className="flex flex-wrap items-center gap-2">
          <Badge variant="secondary">dostupný</Badge>
          <Path value={info.ffprobe_path} />
        </span>
      ) : (
        <span className="flex flex-wrap items-center gap-2">
          <Badge variant="destructive">chybí</Badge>
          <span className="text-xs text-muted-foreground">
            bez něj se délka kapitol ukládá jako 1 s
            {placeholderChapters > 0 ? ` – takových kapitol je ${placeholderChapters}` : ''}
          </span>
        </span>
      ),
    },
  ]

  if (info.disk) {
    rows.push({
      label: 'Volné místo',
      value: `${formatBytes(info.disk.free_bytes)} z ${formatBytes(info.disk.total_bytes)}`,
    })
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Server</CardTitle>
        <CardDescription>Kde server bere data a čím disponuje.</CardDescription>
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
