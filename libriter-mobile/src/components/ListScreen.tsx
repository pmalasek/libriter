import type { ReactElement } from 'react'
import { FlatList, Platform, RefreshControl, StyleSheet, View, type ListRenderItem } from 'react-native'
import { useSafeAreaInsets } from 'react-native-safe-area-context'

import { spacing, useTheme } from '@/theme'

/** Kolik místa dole zabere lišta tabů s kapslí přehrávače. */
const BOTTOM_SPACE = 160

/**
 * Rolovací výpis s hlavičkou.
 *
 * Proti obyčejnému `Screen` je to `FlatList`: knihovna má stovky položek a
 * každá je nativní pohled s obálkou. Bez virtualizace se při psaní do hledání
 * překresluje celý seznam a klávesnice nestíhá.
 *
 * Bezpečnou zónu nahoře drží **rám** seznamu, ne jeho obsah. iOS totiž
 * rolovacím pohledům dopočítává vlastní odsazení a to se s odsazením v obsahu
 * sčítalo – výpis pak začínal až pod první třetinou obrazovky.
 */
export function ListScreen<T>({
  data,
  renderItem,
  keyExtractor,
  header,
  empty,
  columns = 1,
  refreshing,
  onRefresh,
  /** Mění se s režimem zobrazení – FlatList neumí přepnout numColumns za běhu. */
  listKey,
}: {
  data: T[]
  renderItem: ListRenderItem<T>
  keyExtractor: (item: T) => string
  header?: ReactElement
  empty?: ReactElement
  columns?: number
  refreshing?: boolean
  onRefresh?: () => void
  listKey?: string
}) {
  const { colors } = useTheme()
  const insets = useSafeAreaInsets()

  return (
    <View style={{ flex: 1, paddingTop: insets.top, backgroundColor: colors.background }}>
      <FlatList
        key={listKey}
        style={styles.list}
        data={data}
        renderItem={renderItem}
        keyExtractor={keyExtractor}
        numColumns={columns > 1 ? columns : undefined}
        columnWrapperStyle={columns > 1 ? styles.column : undefined}
        ListHeaderComponent={header}
        ListEmptyComponent={empty}
        contentContainerStyle={[styles.content, { paddingBottom: BOTTOM_SPACE + insets.bottom }]}
        // Rám už pod stavovým řádkem je; další dopočítané odsazení by obsah
        // jen posunulo níž.
        contentInsetAdjustmentBehavior="never"
        automaticallyAdjustContentInsets={false}
        keyboardShouldPersistTaps="handled"
        keyboardDismissMode="on-drag"
        // Jen na Androidu: na iOSu tahle optimalizace nechává v seznamu
        // prázdná místa.
        removeClippedSubviews={Platform.OS === 'android'}
        initialNumToRender={12}
        maxToRenderPerBatch={12}
        windowSize={7}
        refreshControl={
          onRefresh ? <RefreshControl refreshing={Boolean(refreshing)} onRefresh={onRefresh} tintColor={colors.primary} /> : undefined
        }
      />
    </View>
  )
}

const styles = StyleSheet.create({
  list: { flex: 1 },
  content: { paddingHorizontal: spacing.md, paddingTop: spacing.md },
  column: { gap: spacing.md },
})
