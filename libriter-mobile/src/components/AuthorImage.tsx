import { useState } from 'react'
import { StyleSheet, View } from 'react-native'
import { Image } from 'expo-image'
import { UserRound } from 'lucide-react-native'
import { apiUrl, type Author } from 'libriter-shared'

import { useTheme } from '@/theme'

/** Adresa fotky; `?v=` podle image_path obchází cache po výměně fotky. */
function imageUrl(author: Author): string {
  return `${apiUrl(`/authors/${author.id}/image`)}?v=${encodeURIComponent(author.image_path ?? '')}`
}

/** Kulatý portrét autora; bez fotky nebo při chybě zástupná ikona. */
export function AuthorImage({ author, size = 36 }: { author: Author; size?: number }) {
  const { colors } = useTheme()
  const [failed, setFailed] = useState(false)

  return (
    <View
      style={[
        styles.box,
        { width: size, height: size, borderRadius: size / 2, backgroundColor: colors.secondary, borderColor: colors.background },
      ]}
    >
      {!author.image_path || failed ? (
        <UserRound color={colors.primary} size={size / 2} strokeWidth={1.75} style={{ opacity: 0.6 }} />
      ) : (
        <Image
          source={{ uri: imageUrl(author) }}
          style={StyleSheet.absoluteFill}
          contentFit="cover"
          cachePolicy="disk"
          onError={() => setFailed(true)}
          accessibilityLabel={`Fotografie autora ${author.name}`}
        />
      )}
    </View>
  )
}

const styles = StyleSheet.create({
  box: { overflow: 'hidden', alignItems: 'center', justifyContent: 'center', borderWidth: 2 },
})
