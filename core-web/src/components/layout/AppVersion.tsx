import { useTranslation } from 'react-i18next'
import { useServerVersion } from '../../api/hooks'
import { cn } from '../../lib/utils'

/** Drobná verze serveru (web je jeho součástí, má tedy stejnou). */
export function AppVersion({ className }: { className?: string }) {
  const { t } = useTranslation()
  const version = useServerVersion()
  if (!version.data) return null
  return (
    <p className={cn('truncate text-[0.6875rem] text-muted-foreground/80 tabular-nums', className)} title={version.data}>
      {t('layout.version', { version: version.data })}
    </p>
  )
}
