import { useEffect } from 'react'
import { BackHandler, Dimensions, ScrollView, StyleSheet, TVFocusGuideView, View } from 'react-native'
import { useTranslation } from 'react-i18next'
import { colors, px, radius } from '@/theme'
import { Button } from './Button'
import { T } from './T'

const COLUMNS = 3
const PANEL_W = px(1240)
const PAD = px(44)
const GAP = px(14)
const OPTION_W = Math.floor((PANEL_W - PAD * 2 - GAP * (COLUMNS - 1)) / COLUMNS)

/**
 * Výběr žánru přes obrazovku. Žánrů bývají desítky, proto mřížka po třech
 * a posun, když se nevejdou. Prázdný žánr = všechny. Zpět zavře bez změny.
 */
export function GenrePicker({
  genres,
  value,
  onSelect,
  onClose,
}: {
  genres: string[]
  value: string
  onSelect: (genre: string) => void
  onClose: () => void
}) {
  const { t } = useTranslation()

  useEffect(() => {
    const sub = BackHandler.addEventListener('hardwareBackPress', () => {
      onClose()
      return true
    })
    return () => sub.remove()
  }, [onClose])

  const options = ['', ...genres]
  return (
    <View style={styles.scrim}>
      <View style={styles.panel}>
        <T variant="section" style={styles.title}>
          {t('video.tv.genre.title')}
        </T>
        <TVFocusGuideView autoFocus trapFocusUp trapFocusDown trapFocusLeft trapFocusRight>
          <ScrollView style={styles.scroll} contentContainerStyle={styles.options} showsVerticalScrollIndicator={false}>
            {options.map((g) => (
              <Button
                key={g || '*'}
                label={g || t('video.list.allGenres')}
                selected={g === value}
                hasTVPreferredFocus={g === value}
                scale={false}
                style={styles.option}
                onPress={() => (g === value ? onClose() : onSelect(g))}
              />
            ))}
          </ScrollView>
        </TVFocusGuideView>
      </View>
    </View>
  )
}

const styles = StyleSheet.create({
  scrim: { ...StyleSheet.absoluteFill, zIndex: 20, backgroundColor: colors.scrim, alignItems: 'center', justifyContent: 'center' },
  panel: { width: PANEL_W, padding: PAD, borderRadius: radius.card, backgroundColor: colors.surface, borderWidth: px(2), borderColor: colors.edge },
  title: { marginBottom: px(28) },
  scroll: { maxHeight: Dimensions.get('window').height * 0.7 },
  options: { flexDirection: 'row', flexWrap: 'wrap', gap: GAP },
  option: { width: OPTION_W },
})
