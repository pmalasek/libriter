import { Image } from 'expo-image'
import { StyleSheet, Text, View } from 'react-native'
import { apiUrl } from 'libriter-shared'

import { bookDirectory } from '@/downloads/downloadManager'
import { colors, radius } from '@/theme'

/**
 * Obálka knihy. Nejdřív se zkouší stažený soubor, pak server – u knihy
 * v telefonu se tak obálka ukáže i bez signálu. Kniha bez obálky dostane
 * první písmeno názvu, ne prázdný rámeček.
 */
export function BookCover({
  bookId,
  title,
  size = 64,
  downloaded = false,
}: {
  bookId: string
  title: string
  size?: number
  downloaded?: boolean
}) {
  // bookDirectory už vrací `file://` URI, další prefix by adresu rozbil.
  const source = downloaded
    ? [{ uri: `${bookDirectory(bookId)}cover.jpg` }, { uri: apiUrl(`/books/${bookId}/cover`) }]
    : [{ uri: apiUrl(`/books/${bookId}/cover`) }]

  return (
    <View style={[styles.frame, { width: size, height: size * 1.4, borderRadius: radius.sm }]}>
      <Text style={[styles.letter, { fontSize: size * 0.45 }]}>
        {title.slice(0, 1).toUpperCase()}
      </Text>
      <Image
        source={source}
        style={StyleSheet.absoluteFill}
        contentFit="cover"
        transition={150}
        cachePolicy="disk"
      />
    </View>
  )
}

const styles = StyleSheet.create({
  frame: {
    backgroundColor: colors.surfaceAlt,
    overflow: 'hidden',
    alignItems: 'center',
    justifyContent: 'center',
  },
  letter: { color: colors.textMuted, fontWeight: '700' },
})
