import { useEffect, useState } from 'react'
import { ActivityIndicator, Animated, StyleSheet, View } from 'react-native'
import { useSafeAreaInsets } from 'react-native-safe-area-context'

import { syncEngine, type SyncState } from '@/sync/syncEngine'
import { radius, spacing, useTheme } from '@/theme'
import { Muted } from './ui/Text'

/**
 * Stav synchronizace. Mlčí, když je všechno odeslané – zajímavý je jen
 * okamžik, kdy poslech čeká ve frontě nebo se plní zrcadlo knihovny.
 *
 * Plove nad obsahem, ne v něm: dřív to byl řádek v toku stránky a při každém
 * kolečku synchronizace (start aplikace, návrat z pozadí, uložení pozice)
 * odskočila celá obrazovka o kus dolů a po chvíli zase nahoru. Proto je
 * komponenta absolutně pozicovaná a vykresluje se jednou v kořeni aplikace,
 * stejně jako `Toaster`.
 */
export function SyncBadge() {
  const { colors } = useTheme()
  const insets = useSafeAreaInsets()
  const [state, setState] = useState<SyncState>(syncEngine.current())
  // Text přežije zmizení důvodu, jinak by nebylo co vykreslit při odchodu.
  const [shown, setShown] = useState<string | null>(null)
  const [opacity] = useState(() => new Animated.Value(0))
  const [offset] = useState(() => new Animated.Value(-ENTER_PX))

  useEffect(() => syncEngine.subscribe(setState), [])

  const label = describe(state)
  const busy = state.kind === 'syncing'

  useEffect(() => {
    if (label) setShown(label)
    const duration = label ? 160 : 220
    Animated.parallel([
      Animated.timing(opacity, { toValue: label ? 1 : 0, duration, useNativeDriver: true }),
      Animated.timing(offset, { toValue: label ? 0 : -ENTER_PX, duration, useNativeDriver: true }),
    ]).start(() => {
      if (!label) setShown(null)
    })
  }, [label, opacity, offset])

  if (!shown) return null

  return (
    <Animated.View
      // Nechytá dotyky: pod pilulkou je obsah, který má jít dál ovládat.
      pointerEvents="none"
      style={[styles.wrap, { top: insets.top + spacing.sm, opacity, transform: [{ translateY: offset }] }]}
    >
      <View style={[styles.pill, { backgroundColor: colors.card, borderColor: colors.glassEdge }]}>
        {busy ? <ActivityIndicator size="small" color={colors.mutedForeground} /> : null}
        <Muted size={13}>{shown}</Muted>
      </View>
    </Animated.View>
  )
}

/** O kolik pilulka přiletí shora. */
const ENTER_PX = 8

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
  wrap: { position: 'absolute', left: spacing.md, right: spacing.md, alignItems: 'center' },
  pill: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    borderRadius: radius.xl,
    borderWidth: 1,
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.sm,
    shadowColor: '#000',
    shadowOpacity: 0.15,
    shadowRadius: 12,
    shadowOffset: { width: 0, height: 6 },
    elevation: 6,
  },
})
