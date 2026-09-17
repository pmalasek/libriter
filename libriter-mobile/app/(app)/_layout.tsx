import { Stack } from 'expo-router'

import { useTheme } from '@/theme'

/**
 * Přihlášená část aplikace. Taby jsou uvnitř (skupina `(tabs)` se v adrese
 * neprojeví), nad ně se vysouvají detaily a seznamy z nabídky Více; přes
 * všechno celoobrazovkový přehrávač. Hlavičky si kreslí obrazovky samy –
 * stejně jako na webu je to tlačítko „Zpět na …“ nad obsahem.
 */
export default function AppLayout() {
  const { colors } = useTheme()
  return (
    <Stack screenOptions={{ headerShown: false, contentStyle: { backgroundColor: colors.background } }}>
      <Stack.Screen name="(tabs)" />
      <Stack.Screen name="book/[id]" />
      <Stack.Screen name="author/[id]" />
      <Stack.Screen name="series/[id]" />
      <Stack.Screen name="sessions" />
      <Stack.Screen name="downloads" />
      <Stack.Screen name="settings" />
      <Stack.Screen name="player" options={{ presentation: 'modal' }} />
    </Stack>
  )
}
