import type { ReactNode } from 'react'
import { ScrollView, StyleSheet, View } from 'react-native'
import Constants from 'expo-constants'
import { useTranslation } from 'react-i18next'
import { LogOutIcon, ServerIcon } from 'lucide-react-native'
import { useAuth } from '@/auth/AuthProvider'
import { Button } from '@/components/Button'
import { T } from '@/components/T'
import { usePrefs } from '@/lib/prefs'
import type { SubtitleMode, SubtitleSize } from '@/lib/storage'
import { colors, px, radius, safe } from '@/theme'

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <View style={styles.section}>
      <T variant="section" style={{ marginBottom: px(20) }}>
        {title}
      </T>
      {children}
    </View>
  )
}

export default function SettingsScreen() {
  const { t } = useTranslation()
  const { user, server, signOut, changeServer } = useAuth()
  const { prefs, update } = usePrefs()

  const modes: { key: SubtitleMode; label: string }[] = [
    { key: 'off', label: t('video.tv.settings.subtitlesNever') },
    { key: 'language', label: t('video.tv.settings.subtitlesLanguage') },
  ]
  const sizes: { key: SubtitleSize; label: string }[] = [
    { key: 'small', label: t('video.tv.settings.small') },
    { key: 'medium', label: t('video.tv.settings.medium') },
    { key: 'large', label: t('video.tv.settings.large') },
  ]

  return (
    <ScrollView contentContainerStyle={styles.root}>
      <T variant="title" style={{ marginBottom: px(48) }}>
        {t('video.tv.settings.title')}
      </T>

      <Section title={t('video.tv.settings.subtitles')}>
        <T dim style={styles.label}>
          {t('video.tv.settings.subtitlesDefault')}
        </T>
        <View style={styles.options}>
          {modes.map((m, i) => (
            <Button key={m.key} label={m.label} selected={prefs.subtitles === m.key} hasTVPreferredFocus={i === 0} onPress={() => update({ subtitles: m.key })} />
          ))}
        </View>
        <T dim style={[styles.label, { marginTop: px(32) }]}>
          {t('video.tv.settings.subtitleSize')}
        </T>
        <View style={styles.options}>
          {sizes.map((s) => (
            <Button key={s.key} label={s.label} selected={prefs.subtitleSize === s.key} onPress={() => update({ subtitleSize: s.key })} />
          ))}
        </View>
      </Section>

      <Section title={t('video.tv.settings.account')}>
        <View style={styles.card}>
          <T variant="bodyMedium">{t('video.tv.settings.signedInAs', { name: user?.display_name ?? '' })}</T>
          <T dim>{user?.email}</T>
          <T dim style={{ marginTop: px(16) }}>
            {t('video.tv.settings.server')}: {server}
          </T>
        </View>
        <View style={[styles.options, { marginTop: px(24) }]}>
          <Button icon={LogOutIcon} label={t('video.tv.settings.signOut')} onPress={() => void signOut()} />
          <Button
            icon={ServerIcon}
            label={t('video.tv.pair.changeServer')}
            onPress={() => {
              void signOut().then(changeServer)
            }}
          />
        </View>
      </Section>

      <Section title={t('video.tv.settings.about')}>
        <T dim>Sceniter TV · {t('video.tv.settings.version', { version: Constants.expoConfig?.version ?? '?' })}</T>
      </Section>
    </ScrollView>
  )
}

const styles = StyleSheet.create({
  root: { paddingHorizontal: safe.x, paddingTop: safe.y, paddingBottom: px(120), maxWidth: px(1500) },
  section: { marginBottom: px(64) },
  label: { marginBottom: px(16) },
  options: { flexDirection: 'row', flexWrap: 'wrap', gap: px(16) },
  card: { padding: px(32), borderRadius: radius.card, backgroundColor: colors.surface, alignSelf: 'flex-start', minWidth: px(800) },
})
