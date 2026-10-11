import { memo, useCallback, useEffect, useRef, type ReactElement } from 'react'
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
 *
 * Android posouvá řadu za fokusem jen tak, aby karta byla celá uvnitř řady –
 * neví, že levý okraj překrývá boční menu. Řada proto posun srovná sama:
 * zaměřená karta nikdy nezačíná vlevo od `inset` (při pohybu doleva ani po
 * přeskládání dat, kdy by posun zůstal podle starého pořadí).
 */
function RowInner<T>({ title, items, keyOf, render, itemWidth, inset = safe.x }: RowProps<T>) {
  const listRef = useRef<FlatList<T>>(null)
  const scrollX = useRef(0)
  const focusedKey = useRef<string | null>(null)
  const step = itemWidth + card.gap

  // Karta i začíná v obsahu na inset + step·i; vidět od insetu = posun ≤ step·i.
  const align = useCallback(
    (index: number, animated: boolean) => {
      const max = step * index
      if (scrollX.current > max + 1) listRef.current?.scrollToOffset({ offset: max, animated })
    },
    [step],
  )

  // Data se přeskládala (obnova ze serveru) – zaměřená karta mohla změnit pozici.
  useEffect(() => {
    if (focusedKey.current === null) return
    const index = items.findIndex((x) => keyOf(x) === focusedKey.current)
    if (index >= 0) align(index, false)
    // keyOf bývá inline funkce – stačí reagovat na změnu dat.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [items, align])

  if (items.length === 0) return null
  return (
    <View style={styles.row}>
      {title ? (
        <T variant="section" style={[styles.title, { marginLeft: inset }]}>
          {title}
        </T>
      ) : null}
      <TVFocusGuideView autoFocus>
        <FlatList
          ref={listRef}
          horizontal
          data={items}
          keyExtractor={keyOf}
          renderItem={({ item, index }) => (
            // onFocus na TV probublává z karty – řada ví, co je zaměřené.
            <View
              onFocus={() => {
                focusedKey.current = keyOf(item)
                align(index, true)
              }}
            >
              {render(item, index)}
            </View>
          )}
          onScroll={(e) => (scrollX.current = e.nativeEvent.contentOffset.x)}
          scrollEventThrottle={32}
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
