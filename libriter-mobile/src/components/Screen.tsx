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

  // Bezpečnou zónu drží rám, ne obsah – iOS rolovacím pohledům dopočítává
  // vlastní odsazení a obojí by se sečetlo (viz ListScreen).
  return (
    <View style={{ flex: 1, paddingTop: top ? insets.top : 0, backgroundColor: colors.background }}>
      <ScrollView
        style={styles.scroll}
        contentInsetAdjustmentBehavior="never"
        automaticallyAdjustContentInsets={false}
        contentContainerStyle={[
          styles.content,
          { paddingTop: spacing.md, paddingBottom: BOTTOM_SPACE + insets.bottom },
          contentStyle,
        ]}
        keyboardShouldPersistTaps="handled"
        refreshControl={
          onRefresh ? <RefreshControl refreshing={Boolean(refreshing)} onRefresh={onRefresh} tintColor={colors.primary} /> : undefined
        }
      >
        {children}
      </ScrollView>
    </View>
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
  scroll: { flex: 1 },
  content: { paddingHorizontal: spacing.md },
  back: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm, marginBottom: spacing.md, alignSelf: 'flex-start' },
  actions: { flexDirection: 'row', flexWrap: 'wrap', gap: spacing.sm },
})
