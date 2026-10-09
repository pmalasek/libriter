import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { Button } from 'core-web/src/components/ui/button'

export function NotFoundPage() {
  const { t } = useTranslation()
  return (
    <div className="py-24 text-center">
      <h1 className="font-heading text-2xl font-semibold">{t('layout.notFound.title')}</h1>
      <p className="mt-2 text-muted-foreground">{t('layout.notFound.description')}</p>
      <Button asChild className="mt-6">
        <Link to="/movies">{t('layout.notFound.back')}</Link>
      </Button>
    </div>
  )
}
