import { StyleSheet, View } from 'react-native'
import type { HeroInfo } from '@/lib/titles'
import { px } from '@/theme'
import { T } from './T'

/** Informace o zaměřeném titulu v horní části domovské obrazovky. */
export function Hero({ info, inset }: { info?: HeroInfo; inset: number }) {
  return (
    <View style={[styles.hero, { paddingLeft: inset }]}>
      {info ? (
        <>
          {info.eyebrow ? (
            <T variant="section" dim numberOfLines={1}>
              {info.eyebrow}
            </T>
          ) : null}
          <T variant="hero" numberOfLines={2}>
            {info.title}
          </T>
          {info.meta ? (
            <T variant="caption" dim style={styles.meta} numberOfLines={1}>
              {info.meta}
            </T>
          ) : null}
          {info.overview ? (
            <T dim numberOfLines={3} style={styles.overview}>
              {info.overview}
            </T>
          ) : null}
        </>
      ) : null}
    </View>
  )
}

const styles = StyleSheet.create({
  hero: { height: px(520), justifyContent: 'flex-end', paddingRight: px(700), paddingBottom: px(30) },
  meta: { marginTop: px(14) },
  overview: { marginTop: px(16), maxWidth: px(1050) },
})
