import { Text, View } from 'react-native'
import { Tabs } from 'expo-router'
import { useSafeAreaInsets } from 'react-native-safe-area-context'

import { MiniPlayer } from '@/components/MiniPlayer'
import { colors } from '@/theme'

/** Výška vlastní lišty tabů bez spodního bezpečného okraje. */
const TAB_BAR_HEIGHT = 56

/**
 * Knihovna, stažené knihy a nastavení.
 *
 * Nad lištou tabů sedí přehrávaná kniha – z každé obrazovky je tak přehrávač
 * na jedno klepnutí. Výška lišty se nastavuje ručně právě proto, aby se dalo
 * spolehlivě spočítat, kam lištu poslechu posadit.
 */
export default function TabsLayout() {
  const insets = useSafeAreaInsets()
  const barHeight = TAB_BAR_HEIGHT + insets.bottom

  return (
    <View style={{ flex: 1, backgroundColor: colors.background }}>
      <Tabs
        screenOptions={{
          headerStyle: { backgroundColor: colors.background },
          headerTitleStyle: { color: colors.text },
          headerShadowVisible: false,
          sceneStyle: { backgroundColor: colors.background },
          tabBarStyle: {
            backgroundColor: colors.surface,
            borderTopColor: colors.border,
            height: barHeight,
            paddingBottom: insets.bottom,
          },
          tabBarActiveTintColor: colors.accent,
          tabBarInactiveTintColor: colors.textMuted,
        }}
      >
        <Tabs.Screen
          name="index"
          options={{ title: 'Knihovna', tabBarIcon: () => <TabIcon glyph="📚" /> }}
        />
        <Tabs.Screen
          name="downloads"
          options={{ title: 'Stažené', tabBarIcon: () => <TabIcon glyph="⬇️" /> }}
        />
        <Tabs.Screen
          name="settings"
          options={{ title: 'Nastavení', tabBarIcon: () => <TabIcon glyph="⚙️" /> }}
        />
      </Tabs>

      <View style={{ position: 'absolute', left: 0, right: 0, bottom: barHeight }} pointerEvents="box-none">
        <MiniPlayer />
      </View>
    </View>
  )
}

function TabIcon({ glyph }: { glyph: string }) {
  return <Text style={{ fontSize: 20 }}>{glyph}</Text>
}
