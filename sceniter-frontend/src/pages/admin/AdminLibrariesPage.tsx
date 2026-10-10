import { ArrowDownIcon, ArrowUpIcon, FolderPlusIcon, PencilIcon, Trash2Icon, TriangleAlertIcon } from 'lucide-react'
import { createElement, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import type { Library, LibraryRequest, MetadataProvider, TitleKind } from 'sceniter-shared'
import { EmptyState } from 'core-web/src/components/EmptyState'
import { ErrorState } from 'core-web/src/components/ErrorState'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from 'core-web/src/components/ui/alert-dialog'
import { Badge } from 'core-web/src/components/ui/badge'
import { Button } from 'core-web/src/components/ui/button'
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from 'core-web/src/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from 'core-web/src/components/ui/dialog'
import { Input } from 'core-web/src/components/ui/input'
import { Label } from 'core-web/src/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from 'core-web/src/components/ui/select'
import { Switch } from 'core-web/src/components/ui/switch'
import { cn } from 'core-web/src/lib/utils'
import {
  useAdminLibraries,
  useDeleteLibrary,
  useLibraryCandidates,
  useMetadataProviders,
  useReorderLibraries,
  useSaveLibrary,
} from '@/api/hooks'
import { defaultLibraryIcon, LIBRARY_ICONS, libraryIcon } from '@/lib/libraryIcons'

/** Jazyky metadat v nabídce; názvy dodá prohlížeč v jazyce rozhraní. */
const LANGUAGES = ['cs', 'sk', 'en', 'de', 'fr', 'es', 'it', 'pl', 'hu', 'ru', 'uk', 'ja']
const NONE = '__none__'

/** Výběr ikony do menu; první volba je výchozí ikona podle typu. */
function IconPicker({ kind, value, onChange }: { kind: TitleKind; value: string; onChange: (icon: string) => void }) {
  const { t } = useTranslation()
  const Default = defaultLibraryIcon(kind)
  const options: { key: string; label: string; Icon: typeof Default }[] = [
    { key: '', label: t('video.admin.libraries.iconDefault'), Icon: Default },
    ...Object.entries(LIBRARY_ICONS).map(([key, Icon]) => ({ key, label: key, Icon })),
  ]
  return (
    <div role="radiogroup" aria-label={t('video.admin.libraries.icon')} className="flex flex-wrap gap-1.5">
      {options.map(({ key, label, Icon }) => {
        const active = value === key
        return (
          <button
            key={key || 'default'}
            type="button"
            role="radio"
            aria-checked={active}
            aria-label={label}
            title={label}
            onClick={() => onChange(key)}
            className={cn(
              'flex size-9 items-center justify-center rounded-xl outline-none transition-colors focus-visible:ring-3 focus-visible:ring-ring/50',
              active ? 'bg-primary/12 text-primary ring-1 ring-primary/30' : 'text-muted-foreground hover:bg-foreground/6 hover:text-foreground',
              key === '' && 'border border-dashed border-glass-edge',
            )}
          >
            <Icon className="size-4.5" />
          </button>
        )
      })}
    </div>
  )
}

function useLanguageName() {
  const { i18n } = useTranslation()
  let names: Intl.DisplayNames | undefined
  try {
    names = new Intl.DisplayNames([i18n.language], { type: 'language' })
  } catch {
    names = undefined
  }
  return (code: string) => names?.of(code) ?? code
}

/** Zdroje, které umí daný typ, a přednostně ty pro zvolený jazyk. */
function providersFor(all: MetadataProvider[], kind: TitleKind, language: string) {
  const usable = all.filter((p) => p.kinds.includes(kind))
  const fits = (p: MetadataProvider) => p.languages.length === 0 || p.languages.includes('*') || p.languages.includes(language)
  return [...usable.filter(fits), ...usable.filter((p) => !fits(p))]
}

function LibraryDialog({
  library,
  open,
  onOpenChange,
}: {
  /** undefined = nová knihovna. */
  library?: Library
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const languageName = useLanguageName()
  const candidates = useLibraryCandidates(open && !library)
  const providers = useMetadataProviders()
  const save = useSaveLibrary()

  const [dir, setDir] = useState(library?.dir ?? '')
  const [name, setName] = useState(library?.name ?? '')
  const [kind, setKind] = useState<TitleKind>(library?.kind ?? 'movie')
  const [language, setLanguage] = useState(library?.language ?? 'cs')
  const [provider, setProvider] = useState(library?.provider ?? '')
  const [icon, setIcon] = useState(library?.icon ?? '')

  const options = providersFor(providers.data ?? [], kind, language)
  // Zdroj, který neumí zvolený typ (změna typu u nové knihovny), se zahodí.
  const providerValue = provider && options.some((p) => p.name === provider) ? provider : NONE
  const languages = LANGUAGES.includes(language) ? LANGUAGES : [language, ...LANGUAGES]

  const submit = () => {
    const body: LibraryRequest = {
      name: name.trim() || dir,
      language,
      provider: providerValue === NONE ? '' : providerValue,
      icon,
    }
    if (!library) Object.assign(body, { dir, kind, enabled: true })
    save.mutate(
      { id: library?.id, body },
      {
        onSuccess: (lib) => {
          toast.success(library ? t('video.admin.libraries.saved') : t('video.admin.libraries.created', { name: lib.name }))
          onOpenChange(false)
        },
        onError: (err) => toast.error(err.message),
      },
    )
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{library ? t('video.admin.libraries.editTitle') : t('video.admin.libraries.addTitle')}</DialogTitle>
          {library ? <DialogDescription>{t('video.admin.libraries.kindLocked')}</DialogDescription> : null}
        </DialogHeader>

        <form
          id="library-form"
          className="grid gap-4"
          onSubmit={(e) => {
            e.preventDefault()
            submit()
          }}
        >
          <div className="grid gap-1.5">
            <Label htmlFor="library-dir">{t('video.admin.libraries.dir')}</Label>
            {library ? (
              <Input id="library-dir" value={library.dir} disabled />
            ) : candidates.data && candidates.data.length === 0 ? (
              <p className="text-sm text-muted-foreground">{t('video.admin.libraries.noCandidates')}</p>
            ) : (
              <Select
                value={dir}
                onValueChange={(v) => {
                  if (!name || name === dir) setName(v)
                  setDir(v)
                }}
              >
                <SelectTrigger id="library-dir" className="w-full">
                  <SelectValue placeholder={t('video.admin.libraries.dir')} />
                </SelectTrigger>
                <SelectContent>
                  {(candidates.data ?? []).map((d) => (
                    <SelectItem key={d} value={d}>
                      {d}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          </div>

          <div className="grid gap-1.5">
            <Label htmlFor="library-name">{t('video.admin.libraries.name')}</Label>
            <Input id="library-name" value={name} maxLength={100} onChange={(e) => setName(e.target.value)} />
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="grid gap-1.5">
              <Label htmlFor="library-kind">{t('video.admin.libraries.kind')}</Label>
              <Select value={kind} onValueChange={(v) => setKind(v as TitleKind)} disabled={Boolean(library)}>
                <SelectTrigger id="library-kind" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="movie">{t('video.admin.libraries.kindMovie')}</SelectItem>
                  <SelectItem value="show">{t('video.admin.libraries.kindShow')}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="library-language">{t('video.admin.libraries.language')}</Label>
              <Select value={language} onValueChange={setLanguage}>
                <SelectTrigger id="library-language" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {languages.map((code) => (
                    <SelectItem key={code} value={code}>
                      {languageName(code)} ({code})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>

          <div className="grid gap-1.5">
            <Label>{t('video.admin.libraries.icon')}</Label>
            <IconPicker kind={kind} value={icon} onChange={setIcon} />
          </div>

          <div className="grid gap-1.5">
            <Label htmlFor="library-provider">{t('video.admin.libraries.provider')}</Label>
            <Select value={providerValue} onValueChange={(v) => setProvider(v === NONE ? '' : v)}>
              <SelectTrigger id="library-provider" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={NONE}>{t('video.admin.libraries.noProvider')}</SelectItem>
                {options.map((p) => (
                  <SelectItem key={p.name} value={p.name}>
                    {p.title}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </form>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t('common.cancel')}
          </Button>
          <Button type="submit" form="library-form" disabled={save.isPending || (!library && !dir)}>
            {library ? t('video.admin.libraries.save') : t('video.admin.libraries.create')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function LibraryRow({
  library,
  first,
  last,
  onMove,
  providers,
}: {
  library: Library
  first: boolean
  last: boolean
  onMove: (dir: -1 | 1) => void
  providers: MetadataProvider[]
}) {
  const { t } = useTranslation()
  const languageName = useLanguageName()
  const save = useSaveLibrary()
  const remove = useDeleteLibrary()
  const [editing, setEditing] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const provider = library.provider ? providers.find((p) => p.name === library.provider) : undefined

  return (
    <li className="glass inset-shadow-glass flex flex-wrap items-center gap-3 rounded-2xl p-3 ring-1 ring-glass-edge">
      <span className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
        {createElement(libraryIcon(library), { className: 'size-5' })}
      </span>
      <div className="min-w-0 flex-1">
        <p className="flex flex-wrap items-center gap-2 font-medium">
          {library.name}
          {!library.enabled ? <Badge variant="outline">{t('video.admin.libraries.disabled')}</Badge> : null}
        </p>
        <p className="truncate text-xs text-muted-foreground">
          <span className="font-mono">{library.dir}</span>
          {' · '}
          {[
            library.kind === 'show' ? t('video.admin.libraries.kindShow') : t('video.admin.libraries.kindMovie'),
            languageName(library.language),
            provider ? provider.title : library.provider ? null : t('video.admin.libraries.noProvider'),
            t('video.admin.libraries.items', { count: library.item_count }),
          ]
            .filter(Boolean)
            .join(' · ')}
        </p>
        {library.provider && !provider ? (
          <p className="mt-0.5 flex items-center gap-1 text-xs text-destructive">
            <TriangleAlertIcon className="size-3.5" />
            {t('video.admin.libraries.providerMissing', { name: library.provider })}
          </p>
        ) : null}
      </div>

      <div className="flex items-center gap-1">
        <Switch
          checked={library.enabled}
          aria-label={t('video.admin.libraries.enabled')}
          disabled={save.isPending}
          onCheckedChange={(enabled) =>
            save.mutate({ id: library.id, body: { enabled } }, { onError: (err) => toast.error(err.message) })
          }
        />
        <Button variant="ghost" size="icon" disabled={first} aria-label={t('video.admin.libraries.moveUp', { name: library.name })} onClick={() => onMove(-1)}>
          <ArrowUpIcon />
        </Button>
        <Button variant="ghost" size="icon" disabled={last} aria-label={t('video.admin.libraries.moveDown', { name: library.name })} onClick={() => onMove(1)}>
          <ArrowDownIcon />
        </Button>
        <Button variant="ghost" size="icon" aria-label={t('video.admin.libraries.editTitle')} onClick={() => setEditing(true)}>
          <PencilIcon />
        </Button>
        <Button variant="ghost" size="icon" aria-label={t('video.admin.libraries.delete')} onClick={() => setDeleting(true)}>
          <Trash2Icon />
        </Button>
      </div>

      {editing ? <LibraryDialog library={library} open={editing} onOpenChange={setEditing} /> : null}

      <AlertDialog open={deleting} onOpenChange={setDeleting}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('video.admin.libraries.deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              <strong>{library.name}</strong> ({t('video.admin.libraries.items', { count: library.item_count })}).{' '}
              {t('video.admin.libraries.deleteDescription')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={remove.isPending}
              onClick={() =>
                remove.mutate(library.id, {
                  onSuccess: () => toast.success(t('video.admin.libraries.deleted', { name: library.name })),
                  onError: (err) => toast.error(err.message),
                })
              }
            >
              {t('video.admin.libraries.delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </li>
  )
}

/** Správa knihoven: které podsložky VIDEO_ROOT se procházejí a jako co. */
export function AdminLibrariesPage() {
  const { t } = useTranslation()
  const libraries = useAdminLibraries()
  const providers = useMetadataProviders()
  const reorder = useReorderLibraries()
  const [adding, setAdding] = useState(false)

  if (libraries.isError) return <ErrorState error={libraries.error} onRetry={() => void libraries.refetch()} />
  const list = libraries.data ?? []

  const move = (index: number, dir: -1 | 1) => {
    const ids = list.map((l) => l.id)
    const [id] = ids.splice(index, 1)
    ids.splice(index + dir, 0, id)
    reorder.mutate(ids, { onError: (err) => toast.error(err.message) })
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('video.admin.libraries.title')}</CardTitle>
        <CardDescription>{t('video.admin.libraries.description')}</CardDescription>
        <CardAction>
          <Button size="sm" onClick={() => setAdding(true)}>
            <FolderPlusIcon />
            {t('video.admin.libraries.add')}
          </Button>
        </CardAction>
      </CardHeader>
      <CardContent>
        {libraries.data && list.length === 0 ? <EmptyState icon={FolderPlusIcon} title={t('video.admin.libraries.empty')} /> : null}
        <ul className="grid gap-2">
          {list.map((lib, i) => (
            <LibraryRow
              key={lib.id}
              library={lib}
              first={i === 0}
              last={i === list.length - 1}
              onMove={(dir) => move(i, dir)}
              providers={providers.data ?? []}
            />
          ))}
        </ul>
      </CardContent>
      {adding ? <LibraryDialog open={adding} onOpenChange={setAdding} /> : null}
    </Card>
  )
}
