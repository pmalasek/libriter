import { PluginsSettings } from 'core-web/src/components/PluginsSettings'
import { useMetadataSettings } from '@/api/adminHooks'
import { MetadataSourcesEditor } from '@/components/admin/MetadataSourcesEditor'
import { ErrorState } from '@/components/ErrorState'
import { LoadingList } from '@/components/LoadingGrid'

export function AdminMetadataPage() {
  const settings = useMetadataSettings()

  if (settings.isPending) return <LoadingList count={3} />
  if (settings.error) {
    return <ErrorState error={settings.error} onRetry={() => void settings.refetch()} />
  }

  return (
    <div className="grid gap-6">
      <MetadataSourcesEditor settings={settings.data} />
      {/* Zdroje jsou pluginy – tady se instalují (načtou) a nastavují. */}
      <PluginsSettings kinds={['book', 'author']} />
    </div>
  )
}
