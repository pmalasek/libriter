import { TriangleAlertIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'

export function ErrorState({ error, onRetry }: { error: unknown; onRetry?: () => void }) {
  const { t } = useTranslation()
  const message = error instanceof Error ? error.message : t('layout.errorState.unknown')

  return (
    <div className="rounded-3xl border border-destructive/25 bg-destructive/5 p-10 text-center">
      <span className="mx-auto mb-4 flex size-12 items-center justify-center rounded-2xl bg-destructive/10 text-destructive">
        <TriangleAlertIcon className="size-6" />
      </span>
      <p className="font-heading text-lg font-semibold text-destructive">
        {t('layout.errorState.title')}
      </p>
      <p className="mx-auto mt-1.5 max-w-md text-sm text-muted-foreground">{message}</p>
      {onRetry ? (
        <Button variant="outline" size="sm" className="mt-5" onClick={onRetry}>
          {t('common.retry')}
        </Button>
      ) : null}
    </div>
  )
}
