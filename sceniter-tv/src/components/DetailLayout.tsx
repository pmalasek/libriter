import type { ReactNode } from 'react'
import { ScrollView, StyleSheet, View } from 'react-native'
import { px, safe } from '@/theme'
import { Backdrop } from './Backdrop'
import { T } from './T'

/**
 * Kostra detailu titulu: backdrop přes celou obrazovku, vlevo název,
 * metadata a popis, pod nimi akce a řady (díly, obsazení, podobné).
 */
export function DetailLayout({
  backdrop,
  eyebrow,
  title,
  meta,
  overview,
  actions,
  children,
}: {
  backdrop?: string
  eyebrow?: string
  title: string
  meta?: string
  overview?: string
  actions: ReactNode
  children?: ReactNode
}) {
  return (
    <View style={styles.root}>
      <Backdrop uri={backdrop} strength="strong" />
      <ScrollView contentContainerStyle={styles.content} showsVerticalScrollIndicator={false}>
        <View style={styles.head}>
          {eyebrow ? (
            <T variant="section" dim>
              {eyebrow}
            </T>
          ) : null}
          <T variant="hero" numberOfLines={2}>
            {title}
          </T>
          {meta ? (
            <T variant="caption" dim style={{ marginTop: px(16) }}>
              {meta}
            </T>
          ) : null}
          {overview ? (
            <T dim numberOfLines={5} style={styles.overview}>
              {overview}
            </T>
          ) : null}
          <View style={styles.actions}>{actions}</View>
        </View>
        {children}
      </ScrollView>
    </View>
  )
}

const styles = StyleSheet.create({
  root: { flex: 1 },
  content: { paddingTop: px(260), paddingBottom: px(120) },
  head: { paddingHorizontal: safe.x, marginBottom: px(56), maxWidth: px(1300) },
  overview: { marginTop: px(22), maxWidth: px(1150) },
  actions: { flexDirection: 'row', gap: px(24), marginTop: px(44) },
})
