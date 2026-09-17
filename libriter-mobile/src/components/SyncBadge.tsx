import { useEffect, useState } from 'react'
import { StyleSheet, View } from 'react-native'

import { syncEngine, type SyncState } from '@/sync/syncEngine'
import { radius, spacing, useTheme } from '@/theme'
import { Muted } from './ui/Text'

/**
 * Stav synchronizace. Mlčí, když je všechno odeslané – zajímavý je jen
 * okamžik, kdy poslech čeká ve frontě nebo se plní zrcadlo knihovny.
 */
export function SyncBadge() {
  const { colors } = useTheme()
  const [state, setState] = useState<SyncState>(syncEngine.current())

  useEffect(() => syncEngine.subscribe(setState), [])

  const label = describe(state)
  if (!label) return null

  return (
    <View style={[styles.badge, { backgroundColor: colors.secondary }]}>
      <Muted size={13} style={{ color: colors.secondaryForeground }}>
        {label}
      </Muted>
    </View>
  )
}

function describe(state: SyncState): string | null {
  switch (state.kind) {
    case 'idle':
      return null
    case 'syncing':
      return state.progress ? `Stahuji knihovnu ${state.progress.done}/${state.progress.total}…` : 'Synchronizuji…'
    case 'offline':
      return state.pending > 0 ? `Offline – ${state.pending} ${plural(state.pending)} čeká na odeslání` : null
    case 'backoff':
      return state.pending > 0
        ? `Server neodpovídá – ${state.pending} ${plural(state.pending)} čeká`
        : 'Server neodpovídá, zkusím to znovu'
  }
}

function plural(count: number): string {
  if (count === 1) return 'záznam'
  if (count < 5) return 'záznamy'
  return 'záznamů'
}

const styles = StyleSheet.create({
  badge: { borderRadius: radius.lg, paddingHorizontal: spacing.md, paddingVertical: spacing.sm, marginBottom: spacing.md },
})
