import { Pressable, StyleSheet, View } from 'react-native'
import { UI_LANGUAGE_NAMES, UI_LANGUAGES } from 'libriter-shared'

import { useLanguage } from '@/i18n/LanguageProvider'
import { radius, spacing, useTheme } from '@/theme'
import { Body } from './ui/Text'

/** Výběr jazyka rozhraní; názvy jazyků jsou v nich samotných. */
export function LanguagePicker() {
  const { colors } = useTheme()
  const { language, setLanguage, saving } = useLanguage()

  return (
    <View style={[styles.list, { opacity: saving ? 0.7 : 1 }]}>
      {UI_LANGUAGES.map((code) => {
        const active = code === language
        return (
          <Pressable
            key={code}
            onPress={() => setLanguage(code)}
            style={[styles.item, { borderColor: active ? colors.primary : colors.border, backgroundColor: colors.card }]}
            accessibilityRole="radio"
            accessibilityState={{ checked: active }}
            accessibilityLanguage={code}
          >
            <Body size={13} medium style={active ? { color: colors.primary } : undefined}>
              {UI_LANGUAGE_NAMES[code]}
            </Body>
          </Pressable>
        )
      })}
    </View>
  )
}

const styles = StyleSheet.create({
  list: { flexDirection: 'row', flexWrap: 'wrap', gap: spacing.sm },
  item: {
    justifyContent: 'center',
    borderWidth: 1.5,
    borderRadius: radius.lg,
    paddingHorizontal: spacing.md - 4,
    height: 38,
  },
})
