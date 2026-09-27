import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { Button } from '@/components/ui/button'

export function NotFoundPage() {
  const { t } = useTranslation()

  return (
    <div className="py-24 text-center">
      <p className="bg-brand-gradient font-heading bg-clip-text text-7xl font-bold text-transparent">
        404
      </p>
      <p className="font-heading mt-4 text-xl font-semibold">{t('layout.notFound.title')}</p>
      <p className="mt-1 text-muted-foreground">{t('layout.notFound.description')}</p>
      <Button asChild size="lg" className="mt-8">
        <Link to="/books">{t('layout.notFound.back')}</Link>
      </Button>
    </div>
  )
}
