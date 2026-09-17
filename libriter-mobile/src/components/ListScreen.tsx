import type { ReactElement } from 'react'
import { FlatList, RefreshControl, StyleSheet, type ListRenderItem } from 'react-native'
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
    <FlatList
      key={listKey}
      style={{ flex: 1, backgroundColor: colors.background }}
      data={data}
      renderItem={renderItem}
      keyExtractor={keyExtractor}
      numColumns={columns > 1 ? columns : undefined}
      columnWrapperStyle={columns > 1 ? styles.column : undefined}
      ListHeaderComponent={header}
      ListEmptyComponent={empty}
      contentContainerStyle={[
        styles.content,
        { paddingTop: insets.top + spacing.md, paddingBottom: BOTTOM_SPACE + insets.bottom },
      ]}
      // iOS jinak dopočítává odsazení podle navigační lišty nad obrazovkou;
      // hlavičky si tu kreslíme sami, takže by jen odsunulo obsah dolů.
      contentInsetAdjustmentBehavior="never"
      automaticallyAdjustContentInsets={false}
      keyboardShouldPersistTaps="handled"
      keyboardDismissMode="on-drag"
      removeClippedSubviews
      initialNumToRender={12}
      maxToRenderPerBatch={12}
      windowSize={7}
      refreshControl={
        onRefresh ? <RefreshControl refreshing={Boolean(refreshing)} onRefresh={onRefresh} tintColor={colors.primary} /> : undefined
      }
    />
  )
}

const styles = StyleSheet.create({
  content: { paddingHorizontal: spacing.md },
  column: { gap: spacing.md },
})
