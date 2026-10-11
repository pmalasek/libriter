import { useEffect } from 'react'
import { BackHandler, StyleSheet, TVFocusGuideView, View } from 'react-native'
import { useTranslation } from 'react-i18next'
import { SORTS, SORT_LABEL, type Sort } from '@/lib/sort'
import { colors, px, radius } from '@/theme'
import { Button } from './Button'
import { T } from './T'

/**
 * Výběr řazení přes obrazovku. Fokus je uvnitř uzavřený a začíná na
 * aktuální volbě; Zpět zavře bez změny.
 */
export function SortPicker({ value, onSelect, onClose }: { value: Sort; onSelect: (s: Sort) => void; onClose: () => void }) {
  const { t } = useTranslation()

  useEffect(() => {
    const sub = BackHandler.addEventListener('hardwareBackPress', () => {
      onClose()
      return true
    })
    return () => sub.remove()
  }, [onClose])

  return (
    <View style={styles.scrim}>
      <View style={styles.panel}>
        <T variant="section" style={styles.title}>
          {t('video.tv.sort.title')}
        </T>
        <TVFocusGuideView autoFocus trapFocusUp trapFocusDown trapFocusLeft trapFocusRight style={styles.options}>
          {SORTS.map((s) => (
            <Button
              key={s}
              label={t(SORT_LABEL[s])}
              selected={s === value}
              hasTVPreferredFocus={s === value}
              scale={false}
              onPress={() => (s === value ? onClose() : onSelect(s))}
            />
          ))}
        </TVFocusGuideView>
      </View>
    </View>
  )
}

const styles = StyleSheet.create({
  scrim: { ...StyleSheet.absoluteFill, zIndex: 20, backgroundColor: colors.scrim, alignItems: 'center', justifyContent: 'center' },
  panel: { width: px(620), padding: px(44), borderRadius: radius.card, backgroundColor: colors.surface, borderWidth: px(2), borderColor: colors.edge },
  title: { marginBottom: px(28) },
  options: { gap: px(14) },
})
