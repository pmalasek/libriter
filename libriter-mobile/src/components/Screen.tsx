import type { ReactNode } from 'react'
import { Pressable, RefreshControl, ScrollView, StyleSheet, View, type StyleProp, type ViewStyle } from 'react-native'
import { useRouter } from 'expo-router'
import { ArrowLeft } from 'lucide-react-native'
import { useSafeAreaInsets } from 'react-native-safe-area-context'

import { fonts, spacing, useTheme } from '@/theme'
import { Body } from './ui/Text'

/** Kolik místa dole zabere lišta tabů s kapslí přehrávače. */
const BOTTOM_SPACE = 160

/**
 * Rolovací plátno obrazovky: pozadí z palety, okraje jako `main` na webu,
 * bezpečné zóny a místo dole pro kapsli přehrávače a taby.
 */
export function Screen({
  children,
  refreshing,
  onRefresh,
  contentStyle,
  top = true,
}: {
  children: ReactNode
  refreshing?: boolean
  onRefresh?: () => void
  contentStyle?: StyleProp<ViewStyle>
  /** Odsadit od horní bezpečné zóny (taby bez nativní hlavičky). */
  top?: boolean
}) {
  const { colors } = useTheme()
  const insets = useSafeAreaInsets()

  return (
    <ScrollView
      style={{ flex: 1, backgroundColor: colors.background }}
      // iOS jinak dopočítává odsazení podle navigační lišty nad obrazovkou;
      // hlavičky si tu kreslíme sami, takže by jen odsunulo obsah dolů.
      contentInsetAdjustmentBehavior="never"
      automaticallyAdjustContentInsets={false}
      contentContainerStyle={[
        styles.content,
        { paddingTop: (top ? insets.top : 0) + spacing.md, paddingBottom: BOTTOM_SPACE + insets.bottom },
        contentStyle,
      ]}
      keyboardShouldPersistTaps="handled"
      refreshControl={
        onRefresh ? <RefreshControl refreshing={Boolean(refreshing)} onRefresh={onRefresh} tintColor={colors.primary} /> : undefined
      }
    >
      {children}
    </ScrollView>
  )
}

/** „Zpět na knihy“ – tichý odkaz nad detailem, jako na webu. */
export function BackButton({ label }: { label: string }) {
  const { colors } = useTheme()
  const router = useRouter()
  return (
    <Pressable onPress={() => router.back()} style={styles.back} hitSlop={8} accessibilityRole="button">
      <ArrowLeft color={colors.foreground} size={18} />
      <Body size={14} style={{ fontFamily: fonts.sansMedium }}>
        {label}
      </Body>
    </Pressable>
  )
}

/** Vodorovná řada akcí, která se zalamuje jako `flex-wrap gap-2` na webu. */
export function ActionRow({ children, style }: { children: ReactNode; style?: StyleProp<ViewStyle> }) {
  return <View style={[styles.actions, style]}>{children}</View>
}

const styles = StyleSheet.create({
  content: { paddingHorizontal: spacing.md },
  back: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm, marginBottom: spacing.md, alignSelf: 'flex-start' },
  actions: { flexDirection: 'row', flexWrap: 'wrap', gap: spacing.sm },
})
