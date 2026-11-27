import { StyleSheet, View, type StyleProp, type ViewStyle } from 'react-native'
import { useTranslation } from 'react-i18next'
import { ShieldAlertIcon } from 'lucide-react-native'
import { colors, px } from '@/theme'
import { T } from './T'

/**
 * Varování u serveru na http:// – spojení (párování, token, sledování) jde
 * po síti nešifrovaně. V domácí síti to obvykle nevadí, přes internet ano.
 */
export function InsecureNotice({ text, style }: { text?: string; style?: StyleProp<ViewStyle> }) {
  const { t } = useTranslation()
  return (
    <View style={[styles.row, style]}>
      <ShieldAlertIcon size={px(32)} color={colors.accent} strokeWidth={2.2} />
      <T style={styles.text}>{text ?? t('video.tv.pair.insecure')}</T>
    </View>
  )
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'flex-start', gap: px(14), maxWidth: px(1000) },
  text: { color: colors.accent, flexShrink: 1 },
})
