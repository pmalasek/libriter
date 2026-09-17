import { Alert, Pressable, StyleSheet, View } from 'react-native'
import { useRouter } from 'expo-router'
import { Download, Headphones, LogOut, Settings, type LucideProps } from 'lucide-react-native'
import type { ComponentType } from 'react'

import { useAuth } from '@/auth/AuthProvider'
import { PageHeader } from '@/components/PageHeader'
import { Screen } from '@/components/Screen'
import { SyncBadge } from '@/components/SyncBadge'
import { ThemeToggle } from '@/components/ThemeToggle'
import { Body, Muted } from '@/components/ui/Text'
import { countPending } from '@/db/events'
import { useSessions } from '@/data/hooks'
import { radius, spacing, useTheme } from '@/theme'

/**
 * Obsah panelu „Více“ z webové spodní lišty: co se nevešlo mezi čtyři
 * hlavní taby – Právě posloucháno, Stažené, Nastavení, vzhled a odhlášení.
 */
export default function MoreScreen() {
  const { colors } = useTheme()
  const { user, signOut } = useAuth()
  const sessions = useSessions()
  const router = useRouter()
  const hasSessions = (sessions.data ?? []).length > 0

  const confirmSignOut = async () => {
    const pending = await countPending()
    Alert.alert(
      'Odhlásit se?',
      pending > 0
        ? `${pending} záznamů poslechu ještě čeká na odeslání. Odhlášením o ně přijdete.`
        : 'Stažené knihy zůstanou v telefonu.',
      [
        { text: 'Zpět', style: 'cancel' },
        { text: 'Odhlásit', style: 'destructive', onPress: () => void signOut() },
      ],
    )
  }

  return (
    <Screen>
      <PageHeader title={user?.display_name ?? 'Účet'} description={user?.email} />
      <SyncBadge />

      <View style={[styles.group, { backgroundColor: colors.card, borderColor: colors.border }]}>
        {hasSessions ? <Row icon={Headphones} label="Právě posloucháno" onPress={() => router.push('/sessions')} /> : null}
        <Row icon={Download} label="Stažené" onPress={() => router.push('/downloads')} />
        <Row icon={Settings} label="Nastavení" onPress={() => router.push('/settings')} />
        <Row icon={LogOut} label="Odhlásit se" onPress={() => void confirmSignOut()} last />
      </View>

      <View style={[styles.group, { backgroundColor: colors.card, borderColor: colors.border, padding: spacing.md, marginTop: spacing.md }]}>
        <Muted size={13} style={{ marginBottom: spacing.sm + 4 }}>
          Vzhled
        </Muted>
        <ThemeToggle />
      </View>
    </Screen>
  )
}

function Row({
  icon: Icon,
  label,
  onPress,
  last = false,
}: {
  icon: ComponentType<LucideProps>
  label: string
  onPress: () => void
  last?: boolean
}) {
  const { colors } = useTheme()
  return (
    <Pressable
      onPress={onPress}
      style={({ pressed }) => [
        styles.row,
        !last && { borderBottomWidth: StyleSheet.hairlineWidth, borderBottomColor: colors.border },
        pressed && { backgroundColor: colors.muted },
      ]}
    >
      <Icon color={colors.foreground} size={20} />
      <Body medium>{label}</Body>
    </Pressable>
  )
}

const styles = StyleSheet.create({
  group: { borderWidth: 1, borderRadius: radius['2xl'], overflow: 'hidden' },
  row: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm + 4, paddingHorizontal: spacing.md, paddingVertical: spacing.sm + 6 },
})
