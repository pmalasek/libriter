import type { ComponentType } from 'react'
import { View, type ColorValue } from 'react-native'
import { Tabs } from 'expo-router'
import { Ellipsis, Home, Layers, Library, Users, type LucideProps } from 'lucide-react-native'
import { useSafeAreaInsets } from 'react-native-safe-area-context'

import { MiniPlayer } from '@/components/MiniPlayer'
import { GlassBackground } from '@/components/ui/Blur'
import { fonts, useTheme } from '@/theme'

/** Výška vlastní lišty tabů bez spodního bezpečného okraje. */
const TAB_BAR_HEIGHT = 58

/**
 * Spodní navigace jako `TabBar` na webu: čtyři hlavní záložky (Domů, Knihy,
 * Autoři, Série) a „Více“ se zbytkem. Nad lištou sedí kapsle přehrávače.
 */
export default function TabsLayout() {
  const { colors } = useTheme()
  const insets = useSafeAreaInsets()
  const barHeight = TAB_BAR_HEIGHT + insets.bottom

  const icon = (Icon: ComponentType<LucideProps>) =>
    function TabIcon({ color, focused }: { color: ColorValue; focused: boolean }) {
      return (
        <View
          style={{
            width: 34,
            height: 28,
            borderRadius: 10,
            alignItems: 'center',
            justifyContent: 'center',
            backgroundColor: focused ? `${colors.primary}1F` : 'transparent',
          }}
        >
          <Icon color={String(color)} size={20} strokeWidth={2} />
        </View>
      )
    }

  return (
    <View style={{ flex: 1, backgroundColor: colors.background }}>
      <Tabs
        screenOptions={{
          headerShown: false,
          sceneStyle: { backgroundColor: colors.background },
          // Lišta leží nad obsahem a prosvítá skrz ni rozostřený seznam –
          // stejně jako plovoucí rail a kapsle přehrávače na webu.
          tabBarBackground: () => <GlassBackground intensity={80} />,
          tabBarStyle: {
            position: 'absolute',
            backgroundColor: 'transparent',
            borderTopColor: colors.glassEdge,
            height: barHeight,
            paddingBottom: insets.bottom,
            paddingTop: 6,
          },
          tabBarLabelStyle: { fontFamily: fonts.sansMedium, fontSize: 11 },
          tabBarActiveTintColor: colors.primary,
          tabBarInactiveTintColor: colors.mutedForeground,
        }}
      >
        <Tabs.Screen name="index" options={{ title: 'Domů', tabBarIcon: icon(Home) }} />
        <Tabs.Screen name="books" options={{ title: 'Knihy', tabBarIcon: icon(Library) }} />
        <Tabs.Screen name="authors" options={{ title: 'Autoři', tabBarIcon: icon(Users) }} />
        <Tabs.Screen name="series" options={{ title: 'Série', tabBarIcon: icon(Layers) }} />
        <Tabs.Screen name="more" options={{ title: 'Více', tabBarIcon: icon(Ellipsis) }} />
      </Tabs>

      <View style={{ position: 'absolute', left: 0, right: 0, bottom: barHeight }} pointerEvents="box-none">
        <MiniPlayer />
      </View>
    </View>
  )
}
