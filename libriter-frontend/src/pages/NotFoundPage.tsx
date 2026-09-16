import { Link } from 'react-router'
import { Button } from '@/components/ui/button'

export function NotFoundPage() {
  return (
    <div className="py-24 text-center">
      <p className="bg-brand-gradient font-heading bg-clip-text text-7xl font-bold text-transparent">
        404
      </p>
      <p className="font-heading mt-4 text-xl font-semibold">Tato stránka neexistuje.</p>
      <p className="mt-1 text-muted-foreground">Možná se přesunula, nebo je odkaz překlepnutý.</p>
      <Button asChild size="lg" className="mt-8">
        <Link to="/books">Zpět na knihy</Link>
      </Button>
    </div>
  )
}
