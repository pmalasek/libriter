import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useSearchParams } from 'react-router'
import { useApproveDevice, useDeviceInfo } from '../api/hooks'
import { useAuth } from '../auth/AuthContext'
import { AuthShell } from '../components/layout/AuthShell'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'

/** Kanonický tvar kódu z televize: ABCD-EFGH, bez mezer a malých písmen. */
function normalizeCode(raw: string): string {
  const s = raw.toUpperCase().replace(/[^A-Z0-9]/g, '').slice(0, 8)
  return s.length > 4 ? `${s.slice(0, 4)}-${s.slice(4)}` : s
}

/**
 * Schválení párování televize. Televize ukáže kód (a QR s odkazem sem
 * i s kódem); přihlášený uživatel ho potvrdí a televize se přihlásí jeho
 * účtem. Na stránku se chodí i z telefonu, proto rámec přihlašovacích stránek.
 */
export function LinkDevicePage() {
  const { t } = useTranslation()
  const { user } = useAuth()
  const [params] = useSearchParams()
  const [input, setInput] = useState(() => normalizeCode(params.get('code') ?? ''))
  // Kód se ověřuje až po potvrzení (nebo rovnou, když přišel z QR).
  const [code, setCode] = useState(() => (input.length === 9 ? input : ''))
  const info = useDeviceInfo(code)
  const approve = useApproveDevice()

  function reset() {
    approve.reset()
    setInput('')
    setCode('')
  }

  let body: React.ReactNode
  if (approve.isSuccess) {
    body = (
      <div className="space-y-4">
        <p className="text-sm">{t('auth.link.done')}</p>
        <div className="flex gap-2">
          <Button variant="outline" className="flex-1" onClick={reset}>
            {t('auth.link.again')}
          </Button>
          <Button asChild className="flex-1">
            <Link to="/">OK</Link>
          </Button>
        </div>
      </div>
    )
  } else if (code && info.data) {
    body = (
      <div className="space-y-4">
        <p className="text-center font-mono text-3xl font-semibold tracking-widest">{info.data.user_code}</p>
        <p className="text-sm">
          {t('auth.link.confirm', {
            name: info.data.device_name || t('auth.link.unnamed'),
            user: user?.display_name ?? '',
          })}
        </p>
        {approve.error ? <p className="text-sm text-destructive">{approve.error.message}</p> : null}
        <div className="flex gap-2">
          <Button variant="outline" className="flex-1" onClick={reset} disabled={approve.isPending}>
            {t('auth.link.cancel')}
          </Button>
          <Button className="flex-1" onClick={() => approve.mutate(code)} disabled={approve.isPending}>
            {approve.isPending ? t('auth.link.approving') : t('auth.link.approve')}
          </Button>
        </div>
      </div>
    )
  } else {
    body = (
      <form
        className="space-y-4"
        onSubmit={(e) => {
          e.preventDefault()
          setCode(input)
        }}
      >
        <div className="space-y-2">
          <Label htmlFor="device-code">{t('auth.link.code')}</Label>
          <Input
            id="device-code"
            autoComplete="off"
            autoCapitalize="characters"
            spellCheck={false}
            placeholder="ABCD-EFGH"
            className="text-center font-mono text-2xl tracking-widest"
            value={input}
            onChange={(e) => {
              setInput(normalizeCode(e.target.value))
              setCode('')
            }}
          />
        </div>
        {code && info.error ? <p className="text-sm text-destructive">{info.error.message}</p> : null}
        <Button type="submit" size="lg" className="w-full" disabled={input.length !== 9 || info.isFetching}>
          {t('auth.link.check')}
        </Button>
      </form>
    )
  }

  return (
    <AuthShell>
      <Card surface="strong" className="rounded-3xl shadow-glass-lg [--card-spacing:--spacing(6)]">
        <CardHeader>
          <CardTitle className="text-2xl">{t('auth.link.title')}</CardTitle>
          <CardDescription>{t('auth.link.description')}</CardDescription>
        </CardHeader>
        <CardContent>{body}</CardContent>
      </Card>
    </AuthShell>
  )
}
