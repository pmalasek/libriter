import { useEffect, useState } from 'react'
import { StyleSheet, Switch, View } from 'react-native'
import { formatBytes } from 'libriter-shared'

import { useAuth } from '@/auth/AuthProvider'
import { PageHeader } from '@/components/PageHeader'
import { BackButton, Screen } from '@/components/Screen'
import { Button } from '@/components/ui/Button'
import { Body, Muted, SectionTitle } from '@/components/ui/Text'
import { useMode } from '@/data/ModeProvider'
import { countPending } from '@/db/events'
import { getSetting, setWifiOnly, wifiOnly } from '@/db/settings'
import { downloadedBytes } from '@/downloads/downloadManager'
import { syncEngine } from '@/sync/syncEngine'
import { radius, spacing, useTheme } from '@/theme'

export default function SettingsScreen() {
  const { colors } = useTheme()
  const { user, serverUrl } = useAuth()
  const { mode, setMode } = useMode()
  const [onlyWifi, setOnlyWifi] = useState(true)
  const [used, setUsed] = useState(0)
  const [pending, setPending] = useState(0)
  const [lastSync, setLastSync] = useState<string | null>(null)

  const reload = async () => {
    setOnlyWifi(await wifiOnly())
    setUsed(await downloadedBytes())
    setPending(await countPending())
    setLastSync(await getSetting('last_library_sync'))
  }

  useEffect(() => {
    void reload()
  }, [])

  return (
    <Screen>
      <BackButton label="Zpět" />
      <PageHeader title="Nastavení" />

      <Section title="Režim">
        <SwitchRow
          label="Offline režim"
          hint={
            mode === 'offline'
              ? 'Knihovna, autoři, série a poslechy se zrcadlí do telefonu. Stažené knihy hrají i bez signálu.'
              : 'Data se čtou živě ze serveru jako na webu. Zapněte před cestou – aplikace si stáhne celou knihovnu.'
          }
          value={mode === 'offline'}
          onChange={(value) => void setMode(value ? 'offline' : 'online')}
        />
      </Section>

      <Section title="Účet">
        <Row label="Přihlášen jako" value={user?.display_name ?? '—'} />
        <Row label="E-mail" value={user?.email ?? '—'} />
        <Row label="Server" value={serverUrl || '—'} />
      </Section>

      <Section title="Stahování">
        <SwitchRow
          label="Stahovat jen na Wi-Fi"
          hint="Kniha zabere stovky megabajtů; na datech se to pozná."
          value={onlyWifi}
          onChange={(value) => {
            setOnlyWifi(value)
            void setWifiOnly(value)
          }}
        />
        <Row label="Zabráno v telefonu" value={formatBytes(used)} />
      </Section>

      <Section title="Synchronizace">
        <Row label="Čeká na odeslání" value={pending === 0 ? 'nic' : `${pending} záznamů`} />
        <Row label="Naposledy" value={lastSync ? new Date(lastSync).toLocaleString('cs-CZ') : 'zatím nikdy'} />
        <Button
          variant="secondary"
          label="Synchronizovat teď"
          onPress={() => void syncEngine.syncNow().then(reload)}
          style={{ marginTop: spacing.xs }}
        />
      </Section>

      <Muted size={12} style={{ textAlign: 'center', marginTop: spacing.md }}>
        Libriter v telefonu je jen přehrávač. Knihovnu, metadata i pořadí kapitol spravujte ve webovém rozhraní.
      </Muted>
    </Screen>
  )

  function Section({ title, children }: { title: string; children: React.ReactNode }) {
    return (
      <View style={{ marginBottom: spacing.md, gap: spacing.xs }}>
        <SectionTitle style={{ fontSize: 15 }}>{title}</SectionTitle>
        <View style={[styles.card, { backgroundColor: colors.card, borderColor: colors.border }]}>{children}</View>
      </View>
    )
  }

  function Row({ label, value }: { label: string; value: string }) {
    return (
      <View style={styles.row}>
        <Muted size={14}>{label}</Muted>
        <Body size={14} numberOfLines={1} style={{ flexShrink: 1, textAlign: 'right' }}>
          {value}
        </Body>
      </View>
    )
  }

  function SwitchRow({ label, hint, value, onChange }: { label: string; hint: string; value: boolean; onChange: (value: boolean) => void }) {
    return (
      <View style={[styles.row, { alignItems: 'center' }]}>
        <View style={{ flex: 1 }}>
          <Body size={14} medium>
            {label}
          </Body>
          <Muted size={12}>{hint}</Muted>
        </View>
        <Switch value={value} onValueChange={onChange} trackColor={{ true: colors.primary, false: colors.border }} />
      </View>
    )
  }
}

const styles = StyleSheet.create({
  card: { borderWidth: 1, borderRadius: radius['2xl'], padding: spacing.md, gap: spacing.sm + 2 },
  row: { flexDirection: 'row', justifyContent: 'space-between', gap: spacing.md },
})
