import { useEffect, useState } from 'react'
import { StyleSheet, Text, View } from 'react-native'

import { syncEngine, type SyncState } from '@/sync/syncEngine'
import { colors, radius, spacing } from '@/theme'

/**
 * Stav synchronizace. Mlčí, když je všechno odeslané – zajímavý je jen
 * okamžik, kdy poslech čeká ve frontě a uživatel se ptá, jestli o něj přijde.
 */
export function SyncBadge() {
  const [state, setState] = useState<SyncState>(syncEngine.current())

  useEffect(() => syncEngine.subscribe(setState), [])

  const label = describe(state)
  if (!label) return null

  return (
    <View style={styles.badge}>
      <Text style={styles.text}>{label}</Text>
    </View>
  )
}

function describe(state: SyncState): string | null {
  switch (state.kind) {
    case 'idle':
      return null
    case 'syncing':
      return 'Synchronizuji…'
    case 'offline':
      return state.pending > 0
        ? `Offline – ${state.pending} ${plural(state.pending)} čeká na odeslání`
        : 'Offline'
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
  badge: {
    backgroundColor: colors.surfaceAlt,
    borderRadius: radius.sm,
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.sm,
    marginBottom: spacing.sm,
  },
  text: { color: colors.textMuted, fontSize: 13 },
})
