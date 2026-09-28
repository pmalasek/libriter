import { useEffect, useState } from 'react'
import { Platform, StyleSheet, Switch, View } from 'react-native'
import { useTranslation } from 'react-i18next'
import { currentLanguage, formatBytes } from 'libriter-shared'

import { useAuth } from '@/auth/AuthProvider'
import { PageHeader } from '@/components/PageHeader'
import { BackButton, Screen } from '@/components/Screen'
import { Button } from '@/components/ui/Button'
import { Body, Muted, SectionTitle } from '@/components/ui/Text'
import { useMode } from '@/data/ModeProvider'
import { countPending } from '@/db/events'
import { compactDownloads, getSetting, setCompactDownloads, setWifiOnly, wifiOnly } from '@/db/settings'
import { downloadedBytes } from '@/downloads/downloadManager'
import { syncEngine } from '@/sync/syncEngine'
import { radius, spacing, useTheme } from '@/theme'

export default function SettingsScreen() {
  const { t } = useTranslation()
  const { colors } = useTheme()
  const { user, serverUrl } = useAuth()
  const { mode, setMode } = useMode()
  const [onlyWifi, setOnlyWifi] = useState(true)
  const [compact, setCompact] = useState(false)
  const [used, setUsed] = useState(0)
  const [pending, setPending] = useState(0)
  const [lastSync, setLastSync] = useState<string | null>(null)

  const reload = async () => {
    setOnlyWifi(await wifiOnly())
    setCompact(await compactDownloads())
    setUsed(await downloadedBytes())
    setPending(await countPending())
    setLastSync(await getSetting('last_library_sync'))
  }

  // Úsporné stahování se bez vlastní volby řídí offline režimem, proto se
  // po jeho přepnutí načítá znovu.
  useEffect(() => {
    void reload()
  }, [mode])

  return (
    <Screen>
      <BackButton label={t('common.back')} />
      <PageHeader title={t('mobile.settings.title')} />

      <Section title={t('mobile.settings.mode')}>
        <SwitchRow
          label={t('mobile.settings.offlineMode')}
          hint={
            mode === 'offline'
              ? t('mobile.settings.offlineHint')
              : t('mobile.settings.onlineHint')
          }
          value={mode === 'offline'}
          onChange={(value) => void setMode(value ? 'offline' : 'online')}
        />
      </Section>

      <Section title={t('mobile.settings.account')}>
        <Row label={t('mobile.settings.signedInAs')} value={user?.display_name ?? '—'} />
        <Row label={t('mobile.settings.email')} value={user?.email ?? '—'} />
        {user?.login ? <Row label={t('mobile.settings.login')} value={user.login} /> : null}
        <Row label={t('mobile.settings.server')} value={serverUrl || '—'} />
      </Section>

      <Section title={t('mobile.settings.downloads')}>
        <SwitchRow
          label={t('mobile.settings.wifiOnly')}
          hint={t('mobile.settings.wifiOnlyHint')}
          value={onlyWifi}
          onChange={(value) => {
            setOnlyWifi(value)
            void setWifiOnly(value)
          }}
        />
        <SwitchRow
          label={t('mobile.settings.compactDownloads')}
          // Formát se liší: iOS neumí Ogg, dostává AAC (viz downloadManager).
          hint={t(Platform.OS === 'ios' ? 'mobile.settings.compactDownloadsHintIos' : 'mobile.settings.compactDownloadsHint')}
          value={compact}
          onChange={(value) => {
            setCompact(value)
            void setCompactDownloads(value)
          }}
        />
        <Row label={t('mobile.settings.storageUsed')} value={formatBytes(used)} />
      </Section>

      <Section title={t('mobile.settings.sync')}>
        <Row label={t('mobile.settings.pending')} value={pending === 0 ? t('mobile.settings.pendingNone') : t('mobile.settings.pendingCount', { count: pending })} />
        <Row label={t('mobile.settings.lastSync')} value={lastSync ? new Date(lastSync).toLocaleString(currentLanguage()) : t('mobile.settings.never')} />
        <Button
          variant="secondary"
          label={t('mobile.settings.syncNow')}
          onPress={() => void syncEngine.syncNow().then(reload)}
          style={{ marginTop: spacing.xs }}
        />
      </Section>

      <Muted size={12} style={{ textAlign: 'center', marginTop: spacing.md }}>
        {t('mobile.settings.footer')}
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
