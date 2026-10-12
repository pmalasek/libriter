import { useEffect } from 'react'
import { AppState, BackHandler, Platform, TVEventControl } from 'react-native'
import { Stack, router, usePathname } from 'expo-router'
import * as SplashScreen from 'expo-splash-screen'
import { StatusBar } from 'expo-status-bar'
import { QueryClient, QueryClientProvider, focusManager } from '@tanstack/react-query'
import { I18nextProvider } from 'react-i18next'
import { useFonts } from 'expo-font'
import { BricolageGrotesque_600SemiBold, BricolageGrotesque_700Bold } from '@expo-google-fonts/bricolage-grotesque'
import { Geist_400Regular, Geist_500Medium, Geist_600SemiBold } from '@expo-google-fonts/geist'
import { i18n } from 'sceniter-shared'
import { AuthProvider, useAuth } from '@/auth/AuthProvider'
import { handleSectionBack } from '@/lib/nav'
import { PrefsProvider } from '@/lib/prefs'
import { StartupLoading } from '@/components/States'
import { PairScreen } from '@/screens/PairScreen'
import { colors } from '@/theme'

void SplashScreen.preventAutoHideAsync()
SplashScreen.setOptions({ fade: true, duration: 300 })

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      gcTime: 10 * 60_000,
      retry: 1,
    },
  },
})

// Návrat aplikace do popředí = „focus okna“: zastaralé dotazy se obnoví, ať
// Domů a detail ukážou pozici rozkoukanou mezitím na jiném zařízení.
focusManager.setEventListener((setFocused) => {
  const sub = AppState.addEventListener('change', (state) => setFocused(state === 'active'))
  return () => sub.remove()
})

/**
 * Zpět (Menu na Siri Remote, Back na Android TV): o krok zpět v zásobníku,
 * v sekci nejdřív do bočního menu, z menu na Domů, z Domů pryč z aplikace.
 * Na tvOS musí Menu na domovské obrazovce patřit systému (požadavek App
 * Store) – tam se klávesa uvolní.
 */
function useBackBehaviour(ready: boolean) {
  const pathname = usePathname()
  const atRoot = pathname === '/' || !ready
  useEffect(() => {
    if (Platform.OS === 'ios') {
      if (atRoot) TVEventControl.disableTVMenuKey()
      else TVEventControl.enableTVMenuKey()
    }
  }, [atRoot])
  useEffect(() => {
    const sub = BackHandler.addEventListener('hardwareBackPress', () => {
      if (router.canGoBack()) {
        router.back()
        return true
      }
      if (handleSectionBack()) return true
      if (!atRoot) {
        router.replace('/')
        return true
      }
      return false
    })
    return () => sub.remove()
  }, [atRoot])
}

function Root() {
  const { status } = useAuth()
  useBackBehaviour(status === 'ready')

  if (status === 'loading') return <StartupLoading />
  if (status !== 'ready') return <PairScreen />
  return (
    <Stack screenOptions={{ headerShown: false, contentStyle: { backgroundColor: colors.bg }, animation: 'fade' }}>
      <Stack.Screen name="(main)" />
      <Stack.Screen name="player" options={{ animation: 'fade', contentStyle: { backgroundColor: '#000' } }} />
    </Stack>
  )
}

export default function RootLayout() {
  const [fontsLoaded] = useFonts({
    BricolageGrotesque_600SemiBold,
    BricolageGrotesque_700Bold,
    Geist_400Regular,
    Geist_500Medium,
    Geist_600SemiBold,
  })

  useEffect(() => {
    if (fontsLoaded) void SplashScreen.hideAsync()
  }, [fontsLoaded])

  if (!fontsLoaded) return null

  return (
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <PrefsProvider>
            <StatusBar hidden />
            <Root />
          </PrefsProvider>
        </AuthProvider>
      </QueryClientProvider>
    </I18nextProvider>
  )
}
