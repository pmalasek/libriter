import { useState } from 'react'
import { StyleSheet, View, type StyleProp, type ViewStyle } from 'react-native'
import { Image } from 'expo-image'
import { BookHeadphones } from 'lucide-react-native'
import { apiUrl, type Book } from 'libriter-shared'

import { bookDirectory } from '@/downloads/downloadManager'
import { radius, useTheme } from '@/theme'

/**
 * URL obálky – veřejný endpoint, `?v=` podle `updated_at` obchází cache po
 * výměně obálky (stejně jako `coverUrl` na webu).
 */
export function coverUrl(book: Pick<Book, 'id' | 'updated_at'>): string {
  return `${apiUrl(`/books/${book.id}/cover`)}?v=${encodeURIComponent(book.updated_at)}`
}

/**
 * Obálka knihy ve čtvercovém rámu jako na webu: obrázek se vejde celý
 * (`contain`) nad svou rozmazanou kopií, která vyplní zbytek. Stažená kniha
 * bere obálku z disku, aby se ukázala i bez signálu; bez obálky se zobrazí
 * ikona knihy se sluchátky.
 */
export function BookCover({
  book,
  size,
  downloaded = false,
  rounded = radius['2xl'],
  style,
}: {
  book: Pick<Book, 'id' | 'title' | 'updated_at' | 'cover_path'>
  /** Hrana čtverce; bez ní vyplní šířku rodiče (aspect 1:1). */
  size?: number
  downloaded?: boolean
  rounded?: number
  style?: StyleProp<ViewStyle>
}) {
  const { colors } = useTheme()
  const [failed, setFailed] = useState(false)

  const remote = coverUrl(book)
  const source = downloaded ? [{ uri: `${bookDirectory(book.id)}cover.jpg` }, { uri: remote }] : { uri: remote }
  const hasCover = Boolean(book.cover_path) && !failed

  return (
    <View
      style={[
        styles.frame,
        { backgroundColor: colors.muted, borderRadius: rounded },
        size ? { width: size, height: size } : { width: '100%', aspectRatio: 1 },
        style,
      ]}
    >
      {hasCover ? (
        <>
          <Image
            source={source}
            style={StyleSheet.absoluteFill}
            contentFit="cover"
            blurRadius={24}
            cachePolicy="disk"
          />
          <Image
            source={source}
            style={StyleSheet.absoluteFill}
            contentFit="contain"
            transition={150}
            cachePolicy="disk"
            onError={() => setFailed(true)}
            accessibilityLabel={`Obálka knihy ${book.title}`}
          />
        </>
      ) : (
        <View style={styles.fallback}>
          <BookHeadphones color={colors.primary} size={size ? Math.max(18, size * 0.4) : 40} strokeWidth={1.5} style={{ opacity: 0.6 }} />
        </View>
      )}
    </View>
  )
}

const styles = StyleSheet.create({
  frame: { overflow: 'hidden' },
  fallback: { flex: 1, alignItems: 'center', justifyContent: 'center' },
})
