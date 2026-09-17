import { useEffect, useState } from 'react'
import { Alert, Pressable, ScrollView, StyleSheet, Switch, Text, View } from 'react-native'

import { useAuth } from '@/auth/AuthProvider'
import { SyncBadge } from '@/components/SyncBadge'
import { countPending } from '@/db/events'
import { getSetting, setWifiOnly, wifiOnly } from '@/db/settings'
import { downloadedBytes } from '@/downloads/downloadManager'
import { syncEngine } from '@/sync/syncEngine'
import { colors, formatBytes, radius, spacing } from '@/theme'

export default function SettingsScreen() {
  const { user, serverUrl, signOut } = useAuth()
  const [onlyWifi, setOnlyWifi] = useState(true)
  const [used, setUsed] = useState(0)
  const [pending, setPending] = useState(0)
  const [lastSync, setLastSync] = useState<string | null>(null)

  useEffect(() => {
    void (async () => {
      setOnlyWifi(await wifiOnly())
      setUsed(await downloadedBytes())
      setPending(await countPending())
      setLastSync(await getSetting('last_library_sync'))
    })()
  }, [])

  const confirmSignOut = () => {
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
    <ScrollView contentContainerStyle={styles.content}>
      <SyncBadge />

      <Section title="Účet">
        <Row label="Přihlášen jako" value={user?.display_name ?? '—'} />
        <Row label="E-mail" value={user?.email ?? '—'} />
        <Row label="Server" value={serverUrl || '—'} />
      </Section>

      <Section title="Stahování">
        <View style={styles.switchRow}>
          <View style={styles.switchTexts}>
            <Text style={styles.label}>Stahovat jen na Wi-Fi</Text>
            <Text style={styles.hint}>Kniha zabere stovky megabajtů; na datech se to pozná.</Text>
          </View>
          <Switch
            value={onlyWifi}
            onValueChange={(value) => {
              setOnlyWifi(value)
              void setWifiOnly(value)
            }}
            trackColor={{ true: colors.accent, false: colors.border }}
          />
        </View>
        <Row label="Zabráno v telefonu" value={formatBytes(used)} />
      </Section>

      <Section title="Synchronizace">
        <Row label="Čeká na odeslání" value={pending === 0 ? 'nic' : `${pending} záznamů`} />
        <Row label="Naposledy" value={lastSync ? new Date(lastSync).toLocaleString('cs-CZ') : 'zatím nikdy'} />
        <Pressable
          style={styles.button}
          onPress={() => {
            void syncEngine.syncNow().then(async () => {
              setPending(await countPending())
              setLastSync(await getSetting('last_library_sync'))
            })
          }}
        >
          <Text style={styles.buttonText}>Synchronizovat teď</Text>
        </Pressable>
      </Section>

      <Pressable style={styles.signOut} onPress={confirmSignOut}>
        <Text style={styles.signOutText}>Odhlásit se</Text>
      </Pressable>

      <Text style={styles.note}>
        Libriter v telefonu je jen přehrávač. Knihovnu, metadata i pořadí kapitol spravujte
        ve webovém rozhraní.
      </Text>
    </ScrollView>
  )
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <View style={styles.section}>
      <Text style={styles.sectionTitle}>{title}</Text>
      <View style={styles.card}>{children}</View>
    </View>
  )
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <View style={styles.row}>
      <Text style={styles.label}>{label}</Text>
      <Text style={styles.value} numberOfLines={1}>
        {value}
      </Text>
    </View>
  )
}

const styles = StyleSheet.create({
  content: { padding: spacing.md, gap: spacing.md, paddingBottom: spacing.xl },
  section: { gap: spacing.xs },
  sectionTitle: {
    color: colors.textMuted,
    fontSize: 13,
    textTransform: 'uppercase',
    letterSpacing: 1,
  },
  card: {
    backgroundColor: colors.surface,
    borderRadius: radius.md,
    padding: spacing.md,
    gap: spacing.sm,
  },
  row: { flexDirection: 'row', justifyContent: 'space-between', gap: spacing.md },
  switchRow: { flexDirection: 'row', alignItems: 'center', gap: spacing.md },
  switchTexts: { flex: 1 },
  label: { color: colors.textMuted, fontSize: 14 },
  hint: { color: colors.textMuted, fontSize: 12, opacity: 0.8, marginTop: 2 },
  value: { color: colors.text, fontSize: 14, flexShrink: 1, textAlign: 'right' },
  button: {
    backgroundColor: colors.surfaceAlt,
    borderRadius: radius.sm,
    paddingVertical: spacing.sm + 2,
    alignItems: 'center',
  },
  buttonText: { color: colors.text, fontSize: 15 },
  signOut: {
    borderColor: colors.danger,
    borderWidth: 1,
    borderRadius: radius.sm,
    paddingVertical: spacing.sm + 2,
    alignItems: 'center',
  },
  signOutText: { color: colors.danger, fontSize: 15, fontWeight: '600' },
  note: { color: colors.textMuted, fontSize: 12, textAlign: 'center' },
})
