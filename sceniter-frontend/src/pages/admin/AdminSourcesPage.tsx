import { PluginsSettings } from 'core-web/src/components/PluginsSettings'

/** Pluginy metadat pro filmy a seriály a jejich nastavení (API klíče). */
export function AdminSourcesPage() {
  return <PluginsSettings kinds={['movie', 'show']} />
}
