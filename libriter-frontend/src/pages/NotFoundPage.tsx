import { Link } from 'react-router'
import { Button } from '@/components/ui/button'

export function NotFoundPage() {
  return (
    <div className="py-20 text-center">
      <p className="font-heading text-3xl font-semibold">404</p>
      <p className="mt-2 text-muted-foreground">Tato stránka neexistuje.</p>
      <Button asChild className="mt-6">
        <Link to="/">Zpět na knihy</Link>
      </Button>
    </div>
  )
}
