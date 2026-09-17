import type { ReactNode } from 'react'
import { Pressable, ScrollView, StyleSheet, View } from 'react-native'
import { useRouter, type Href } from 'expo-router'
import { ChevronRight } from 'lucide-react-native'

import { fonts, spacing, useTheme } from '@/theme'
import { Body, SectionTitle } from './ui/Text'

/** Šířka položky police – jako `w-32` na webu. */
export const SHELF_ITEM_WIDTH = 128

/**
 * Vodorovná police na domovské stránce. Položky se posouvají do strany a
 * zaskakují na začátek, aby police nikdy nekončila rozpůlenou obálkou.
 */
export function Shelf({ title, to, children }: { title: string; to?: Href; children: ReactNode }) {
  const { colors } = useTheme()
  const router = useRouter()

  return (
    <View style={styles.section}>
      <View style={styles.head}>
        <SectionTitle>{title}</SectionTitle>
        {to ? (
          <Pressable onPress={() => router.push(to)} style={styles.link} hitSlop={8}>
            <Body size={13} style={{ color: colors.primary, fontFamily: fonts.sansMedium }}>
              Zobrazit vše
            </Body>
            <ChevronRight color={colors.primary} size={16} />
          </Pressable>
        ) : null}
      </View>
      <ScrollView
        horizontal
        showsHorizontalScrollIndicator={false}
        snapToInterval={SHELF_ITEM_WIDTH + spacing.md}
        decelerationRate="fast"
        contentContainerStyle={styles.items}
        style={styles.scroll}
      >
        {children}
      </ScrollView>
    </View>
  )
}

/** Jedna položka police – pevná šířka, ať mřížka drží rytmus. */
export function ShelfItem({ children, width = SHELF_ITEM_WIDTH }: { children: ReactNode; width?: number }) {
  return <View style={{ width }}>{children}</View>
}

const styles = StyleSheet.create({
  section: { marginTop: spacing.xl },
  head: { flexDirection: 'row', alignItems: 'flex-end', justifyContent: 'space-between', marginBottom: spacing.sm + 4 },
  link: { flexDirection: 'row', alignItems: 'center', gap: 2 },
  // Záporný okraj nechá polici začínat i končit u hrany obsahu.
  scroll: { marginHorizontal: -spacing.md },
  items: { paddingHorizontal: spacing.md, gap: spacing.md },
})
