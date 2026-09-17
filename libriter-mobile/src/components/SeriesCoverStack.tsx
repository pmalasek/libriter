import { StyleSheet, View } from 'react-native'
import { Layers } from 'lucide-react-native'
import type { Book } from 'libriter-shared'

import { radius, useTheme } from '@/theme'
import { BookCover } from './BookCover'

/**
 * Geometrie stohu jako na webu: rám má pevnou šířku bez ohledu na počet
 * obálek, aby text na všech kartách sérií začínal ve stejném místě.
 */
const VARIANTS = {
  sm: { cover: 56, step: 24, tilts: [-6, 0, 6], icon: 28 },
  lg: { cover: 96, step: 40, tilts: [-6, 0, 6], icon: 48 },
} as const

/** Stoh prvních dílů série – až tři překrývající se obálky. */
export function SeriesCoverStack({ books, variant = 'sm' }: { books: Book[]; variant?: keyof typeof VARIANTS }) {
  const { colors } = useTheme()
  const style = VARIANTS[variant]
  const shown = books.slice(0, 3)
  const width = style.cover + style.step * 2

  return (
    <View style={{ width, height: style.cover }} accessibilityElementsHidden>
      {shown.length === 0 ? (
        <View
          style={[
            styles.placeholder,
            { width: style.cover, height: style.cover, backgroundColor: colors.secondary, borderRadius: radius['2xl'] },
          ]}
        >
          <Layers color={colors.primary} size={style.icon} strokeWidth={1.5} style={{ opacity: 0.6 }} />
        </View>
      ) : (
        shown.map((book, index) => (
          <View
            key={book.id}
            style={[
              styles.cover,
              { left: index * style.step, transform: [{ rotate: `${style.tilts[index]}deg` }], zIndex: index },
            ]}
          >
            <BookCover book={book} size={style.cover} rounded={radius.xl} style={styles.shadow} />
          </View>
        ))
      )}
    </View>
  )
}

const styles = StyleSheet.create({
  placeholder: { alignItems: 'center', justifyContent: 'center' },
  cover: { position: 'absolute', top: 0 },
  shadow: {
    shadowColor: '#000',
    shadowOpacity: 0.18,
    shadowRadius: 8,
    shadowOffset: { width: 0, height: 4 },
    elevation: 3,
  },
})
