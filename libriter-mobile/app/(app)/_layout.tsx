import { Stack } from 'expo-router'

import { colors } from '@/theme'

/**
 * Přihlášená část aplikace. Taby jsou uvnitř (skupina `(tabs)` se v adrese
 * neprojeví), aby se nad ně dal vysunout detail knihy a přes všechno
 * celoobrazovkový přehrávač.
 */
export default function AppLayout() {
  return (
    <Stack
      screenOptions={{
        headerStyle: { backgroundColor: colors.background },
        headerTintColor: colors.text,
        headerShadowVisible: false,
        contentStyle: { backgroundColor: colors.background },
      }}
    >
      <Stack.Screen name="(tabs)" options={{ headerShown: false }} />
      <Stack.Screen name="book/[id]" options={{ title: '' }} />
      <Stack.Screen
        name="player"
        options={{ presentation: 'modal', title: 'Přehrávač', headerShown: false }}
      />
    </Stack>
  )
}
