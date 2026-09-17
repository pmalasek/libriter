import type { ReactNode } from 'react'
import { StyleSheet, View } from 'react-native'

import { radius, spacing, useTheme } from '@/theme'
import { Eyebrow, Heading, Muted } from './ui/Text'

/**
 * Hlavička stránky jako na webu: štítek, titulek, popis, akce (hledání,
 * řazení) a volitelný další řádek (lišta výběru). Varianta `panel` je
 * skleněný plovoucí blok – používá se u výpisů s hledáním.
 */
export function PageHeader({
  eyebrow,
  title,
  description,
  actions,
  panel = false,
  children,
}: {
  eyebrow?: string
  title: string
  description?: string
  actions?: ReactNode
  panel?: boolean
  children?: ReactNode
}) {
  const { colors } = useTheme()
  return (
    <View
      style={[
        styles.wrap,
        panel && { backgroundColor: colors.glassStrong, borderColor: colors.glassEdge, borderWidth: 1, borderRadius: radius['3xl'], padding: spacing.md },
      ]}
    >
      {eyebrow ? <Eyebrow style={{ marginBottom: 6 }}>{eyebrow}</Eyebrow> : null}
      <Heading>{title}</Heading>
      {description ? <Muted size={14} style={{ marginTop: 6 }}>{description}</Muted> : null}
      {actions ? <View style={styles.actions}>{actions}</View> : null}
      {children}
    </View>
  )
}

const styles = StyleSheet.create({
  wrap: { marginBottom: spacing.lg },
  actions: { marginTop: spacing.md, gap: spacing.sm },
})
