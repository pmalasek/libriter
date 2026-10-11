import { memo, type ReactElement } from 'react'
import { FlatList, StyleSheet, TVFocusGuideView, View } from 'react-native'
import { card, px, safe } from '@/theme'
import { T } from './T'

interface RowProps<T> {
  title?: string
  items: T[]
  keyOf: (item: T) => string
  render: (item: T, index: number) => ReactElement
  /** Šířka jedné položky (bez mezery) – pro getItemLayout. */
  itemWidth: number
  /** Levý okraj řady (domovská obrazovka nechává místo bočnímu menu). */
  inset?: number
}

/**
 * Vodorovná řada karet. TVFocusGuideView s autoFocus vrací fokus na kartu,
 * ze které uživatel z řady odešel – pohyb nahoru/dolů nepřeskakuje sloupce.
 */
function RowInner<T>({ title, items, keyOf, render, itemWidth, inset = safe.x }: RowProps<T>) {
  if (items.length === 0) return null
  const step = itemWidth + card.gap
  return (
    <View style={styles.row}>
      {title ? (
        <T variant="section" style={[styles.title, { marginLeft: inset }]}>
          {title}
        </T>
      ) : null}
      <TVFocusGuideView autoFocus>
        <FlatList
          horizontal
          data={items}
          keyExtractor={keyOf}
          renderItem={({ item, index }) => render(item, index)}
          showsHorizontalScrollIndicator={false}
          contentContainerStyle={[styles.content, { paddingLeft: inset }]}
          ItemSeparatorComponent={Separator}
          getItemLayout={(_, index) => ({ length: step, offset: inset + step * index, index })}
          initialNumToRender={8}
          windowSize={5}
          removeClippedSubviews
        />
      </TVFocusGuideView>
    </View>
  )
}

function Separator() {
  return <View style={{ width: card.gap }} />
}

export const Row = memo(RowInner) as typeof RowInner

const styles = StyleSheet.create({
  row: { marginBottom: px(44) },
  title: { marginBottom: px(18) },
  // Svislý okraj nechává místo pro zvětšenou kartu, ať ji FlatList neořízne.
  content: { paddingHorizontal: safe.x, paddingVertical: px(20) },
})
