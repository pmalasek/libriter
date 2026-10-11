import { useEffect } from 'react'
import { BackHandler, Platform, TVEventControl } from 'react-native'
import { Stack, router, usePathname } from 'expo-router'
import * as SplashScreen from 'expo-splash-screen'
import { StatusBar } from 'expo-status-bar'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { I18nextProvider } from 'react-i18next'
import { useFonts } from 'expo-font'
import { BricolageGrotesque_600SemiBold, BricolageGrotesque_700Bold } from '@expo-google-fonts/bricolage-grotesque'
import { Geist_400Regular, Geist_500Medium, Geist_600SemiBold } from '@expo-google-fonts/geist'
import { i18n } from 'sceniter-shared'
import { AuthProvider, useAuth } from '@/auth/AuthProvider'
import { PrefsProvider } from '@/lib/prefs'
import { Loading } from '@/components/States'
import { PairScreen } from '@/screens/PairScreen'
import { colors } from '@/theme'

void SplashScreen.preventAutoHideAsync()

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      gcTime: 10 * 60_000,
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
})

/**
 * Zpět (Menu na Siri Remote, Back na Android TV): o krok zpět v zásobníku,
 * z jiné sekce na Domů, z Domů pryč z aplikace. Na tvOS musí Menu na domovské
 * obrazovce patřit systému (požadavek App Store) – tam se klávesa uvolní.
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

  if (status === 'loading') return <Loading />
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
