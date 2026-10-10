import { PlugIcon, RefreshCwIcon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import type { PluginInfo, PluginKind } from 'core-shared'
import { usePlugins, useReloadPlugins, useSetPluginConfig } from '../api/hooks'
import { ErrorState } from './ErrorState'
import { Badge } from './ui/badge'
import { Button } from './ui/button'
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from './ui/card'
import { Input } from './ui/input'
import { Label } from './ui/label'

/** Konfigurace jednoho pluginu – typicky API klíč. */
function PluginRow({ plugin }: { plugin: PluginInfo }) {
  const { t } = useTranslation()
  const save = useSetPluginConfig(plugin.name)
  // Tajné hodnoty server nevrací; pole je prázdné a vyplní se jen při změně.
  const [values, setValues] = useState<Record<string, string>>(() =>
    Object.fromEntries(plugin.config.map((f) => [f.key, f.secret ? '' : (f.value ?? '')])),
  )
  const changed = plugin.config.filter((f) => (f.secret ? values[f.key] !== '' : values[f.key] !== (f.value ?? '')))

  const submit = (patch: Record<string, string | null>) =>
    save.mutate(patch, {
      onSuccess: () => {
        toast.success(t('admin.plugins.saved', { name: plugin.title }))
        setValues((v) => Object.fromEntries(Object.entries(v).map(([k, val]) => [k, plugin.config.find((f) => f.key === k)?.secret ? '' : val])))
      },
      onError: (err) => toast.error(err.message),
    })

  return (
    <li className="rounded-2xl bg-foreground/4 p-3">
      <div className="flex flex-wrap items-center gap-2">
        <p className="font-medium">{plugin.title}</p>
        <span className="font-mono text-xs text-muted-foreground">{plugin.name}</span>
        <span className="text-xs text-muted-foreground">{t('admin.plugins.version', { version: plugin.version })}</span>
        <div className="ml-auto flex flex-wrap gap-1">
          {plugin.kinds.map((k: PluginKind) => (
            <Badge key={k} variant="brand">
              {t(`admin.plugins.kinds.${k}`)}
            </Badge>
          ))}
          {plugin.languages.filter((l) => l !== '*').map((l) => (
            <Badge key={l} variant="outline">
              {l}
            </Badge>
          ))}
        </div>
      </div>

      {plugin.config.length === 0 ? (
        <p className="mt-1 text-xs text-muted-foreground">{t('admin.plugins.noConfig')}</p>
      ) : (
        <form
          className="mt-3 grid gap-3"
          onSubmit={(e) => {
            e.preventDefault()
            submit(Object.fromEntries(changed.map((f) => [f.key, values[f.key].trim()])))
          }}
        >
          {plugin.config.map((f) => {
            const id = `plugin-${plugin.name}-${f.key}`
            return (
              <div key={f.key} className="grid gap-1.5">
                <Label htmlFor={id}>
                  {f.label}
                  {f.required ? <span className="text-xs font-normal text-muted-foreground">({t('admin.plugins.required')})</span> : null}
                </Label>
                <div className="flex flex-wrap gap-2">
                  <Input
                    id={id}
                    type={f.secret ? 'password' : 'text'}
                    autoComplete="off"
                    value={values[f.key] ?? ''}
                    placeholder={f.secret ? t('admin.plugins.secretPlaceholder') : undefined}
                    onChange={(e) => setValues((v) => ({ ...v, [f.key]: e.target.value }))}
                    className="min-w-48 flex-1"
                  />
                  {f.secret && f.is_set && !f.from_env ? (
                    <Button type="button" variant="ghost" disabled={save.isPending} onClick={() => submit({ [f.key]: '' })}>
                      {t('admin.plugins.clear')}
                    </Button>
                  ) : null}
                </div>
                {f.secret || f.from_env ? (
                  <p className="text-xs text-muted-foreground">
                    {f.is_set ? t('admin.plugins.secretSet') : t('admin.plugins.secretNotSet')}
                    {f.from_env && f.env ? ` · ${t('admin.plugins.fromEnv', { env: f.env })}` : ''}
                  </p>
                ) : null}
              </div>
            )
          })}
          <Button type="submit" size="sm" className="w-fit" disabled={changed.length === 0 || save.isPending}>
            {t('admin.plugins.save')}
          </Button>
        </form>
      )}
    </li>
  )
}

/**
 * Pluginy metadat nainstalované na serveru a jejich nastavení. Pluginy
 * i konfigurace jsou společné všem produktům, filtr `kinds` jen zúží výpis
 * na zdroje, které daný produkt používá.
 */
export function PluginsSettings({ kinds }: { kinds?: PluginKind[] }) {
  const { t } = useTranslation()
  const plugins = usePlugins()
  const reload = useReloadPlugins()

  if (plugins.isError) return <ErrorState error={plugins.error} onRetry={() => void plugins.refetch()} />
  const list = (plugins.data?.plugins ?? []).filter((p) => !kinds || p.kinds.some((k) => kinds.includes(k)))

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('admin.plugins.title')}</CardTitle>
        <CardDescription>{t('admin.plugins.description', { dir: plugins.data?.dir ?? '…' })}</CardDescription>
        <CardAction>
          <Button
            variant="outline"
            size="sm"
            disabled={reload.isPending}
            onClick={() =>
              reload.mutate(undefined, {
                onSuccess: (d) => toast.success(t('admin.plugins.reloaded', { count: d.plugins.length })),
                onError: (err) => toast.error(err.message),
              })
            }
          >
            <RefreshCwIcon className={reload.isPending ? 'animate-spin' : undefined} />
            {t('admin.plugins.reload')}
          </Button>
        </CardAction>
      </CardHeader>
      <CardContent>
        {plugins.data && list.length === 0 ? (
          <p className="flex items-center gap-2 text-sm text-muted-foreground">
            <PlugIcon className="size-4" />
            {t('admin.plugins.empty')}
          </p>
        ) : null}
        <ul className="grid gap-2">
          {list.map((p) => (
            // Klíč s verzí: po načtení nové verze se formulář vyprázdní.
            <PluginRow key={`${p.name}@${p.version}`} plugin={p} />
          ))}
        </ul>
      </CardContent>
    </Card>
  )
}
