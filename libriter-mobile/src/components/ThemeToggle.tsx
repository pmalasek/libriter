import { Pressable, StyleSheet, View } from 'react-native'
import { Monitor, Moon, Sun } from 'lucide-react-native'
import { COLOR_SCHEMES, THEME_MODES, type ColorScheme, type ThemeMode } from 'libriter-shared'

import { palette, radius, spacing, useTheme } from '@/theme'
import { Body, Muted } from './ui/Text'

const MODE_ICONS = { light: Sun, dark: Moon, system: Monitor } as const

/**
 * Režim zobrazení a barevné schéma – totéž, co nabízí `ThemeToggle` na webu,
 * jen rozložené do řádků místo rozbalovací nabídky.
 */
export function ThemeToggle() {
  const { colors, scheme, mode, setScheme, setMode, saving } = useTheme()

  return (
    <View style={{ gap: spacing.md, opacity: saving ? 0.7 : 1 }}>
      <View style={{ gap: spacing.sm }}>
        <Muted size={12}>Režim zobrazení</Muted>
        <View style={[styles.pill, { backgroundColor: colors.muted }]}>
          {THEME_MODES.map((option) => {
            const Icon = MODE_ICONS[option.value]
            const active = option.value === mode
            return (
              <Pressable
                key={option.value}
                onPress={() => setMode(option.value as ThemeMode)}
                style={[styles.pillItem, active && { backgroundColor: colors.card }]}
                accessibilityRole="radio"
                accessibilityState={{ checked: active }}
              >
                <Icon color={active ? colors.primary : colors.mutedForeground} size={16} />
                <Body size={13} medium style={{ color: active ? colors.primary : colors.mutedForeground }}>
                  {option.label}
                </Body>
              </Pressable>
            )
          })}
        </View>
      </View>

      <View style={{ gap: spacing.sm }}>
        <Muted size={12}>Barevné schéma</Muted>
        <View style={styles.schemes}>
          {COLOR_SCHEMES.map((option) => {
            const active = option.value === scheme
            // Ukázková barva je „primary“ daného schématu ve světlém režimu.
            const swatch = palette(option.value as ColorScheme, 'light').primary
            return (
              <Pressable
                key={option.value}
                onPress={() => setScheme(option.value as ColorScheme)}
                style={[styles.scheme, { borderColor: active ? colors.primary : colors.border, backgroundColor: colors.card }]}
                accessibilityRole="radio"
                accessibilityState={{ checked: active }}
              >
                <View style={[styles.dot, { backgroundColor: swatch, borderColor: colors.glassEdge }]} />
                <Body size={13} medium style={active ? { color: colors.primary } : undefined}>
                  {option.label}
                </Body>
              </Pressable>
            )
          })}
        </View>
      </View>
    </View>
  )
}

const styles = StyleSheet.create({
  pill: { flexDirection: 'row', borderRadius: 999, padding: 3 },
  pillItem: {
    flex: 1,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    gap: 6,
    height: 36,
    borderRadius: 999,
  },
  schemes: { flexDirection: 'row', flexWrap: 'wrap', gap: spacing.sm },
  scheme: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    borderWidth: 1.5,
    borderRadius: radius.lg,
    paddingHorizontal: spacing.md - 4,
    height: 38,
  },
  dot: { width: 16, height: 16, borderRadius: 8, borderWidth: 1 },
})
