import { ScrollView, StyleSheet, TVFocusGuideView, View } from 'react-native'
import { LinearGradient } from 'expo-linear-gradient'
import { useTranslation } from 'react-i18next'
import { PauseIcon, PlayIcon } from 'lucide-react-native'
import { formatClock, languageName, trackLabel, type Playback } from 'sceniter-shared'
import { Button } from '@/components/Button'
import { T } from '@/components/T'
import { colors, px, radius, safe } from '@/theme'

function endsAt(remainingSec: number, locale: string): string {
  const d = new Date(Date.now() + remainingSec * 1000)
  try {
    return d.toLocaleTimeString(locale, { hour: '2-digit', minute: '2-digit' })
  } catch {
    return `${d.getHours()}:${String(d.getMinutes()).padStart(2, '0')}`
  }
}

/** Horní a spodní lišta: název, čas, průběh (a cíl přetáčení). */
export function Chrome({
  title,
  subtitle,
  time,
  duration,
  scrubTo,
  paused,
}: {
  title: string
  subtitle?: string
  time: number
  duration: number
  /** Cíl přetáčení, dokud se nepotvrdí; undefined = nepřetáčí se. */
  scrubTo?: number
  paused: boolean
}) {
  const { t, i18n } = useTranslation()
  const shown = scrubTo ?? time
  const ratio = duration > 0 ? Math.min(1, shown / duration) : 0
  const played = duration > 0 ? Math.min(1, time / duration) : 0
  return (
    <View style={StyleSheet.absoluteFill} pointerEvents="none">
      <LinearGradient colors={['rgba(0,0,0,0.75)', 'transparent']} style={styles.top}>
        <T variant="title" numberOfLines={1}>
          {title}
        </T>
        {subtitle ? (
          <T variant="section" dim numberOfLines={1}>
            {subtitle}
          </T>
        ) : null}
      </LinearGradient>

      {paused && scrubTo === undefined ? (
        <View style={styles.center}>
          <View style={styles.bigIcon}>
            <PauseIcon size={px(72)} color={colors.text} fill={colors.text} />
          </View>
        </View>
      ) : null}

      <LinearGradient colors={['transparent', 'rgba(0,0,0,0.85)']} style={styles.bottom}>
        <View style={styles.bar}>
          <View style={[styles.played, { width: `${played * 100}%` }]} />
          {scrubTo !== undefined ? <View style={[styles.scrubFill, { width: `${ratio * 100}%` }]} /> : null}
          <View style={[styles.knob, { left: `${ratio * 100}%` }]} />
        </View>
        <View style={styles.times}>
          <View style={styles.timeLeft}>
            {paused ? <PlayIcon size={px(30)} color={colors.text} fill={colors.text} /> : null}
            <T variant="bodyMedium">{formatClock(shown)}</T>
          </View>
          <T variant="caption" dim>
            {duration > 0 ? t('video.tv.player.endsAt', { time: endsAt(duration - shown, i18n.language) }) : ''}
          </T>
          <T variant="bodyMedium" dim>
            {duration > 0 ? `−${formatClock(Math.max(0, duration - shown))}` : ''}
          </T>
        </View>
        <T variant="small" faint style={{ alignSelf: 'center', marginTop: px(10) }}>
          ↓ {t('video.tv.player.tracks')}
        </T>
      </LinearGradient>
    </View>
  )
}

/** Panel zvukových stop a titulků (šipka dolů). */
export function TracksPanel({
  pb,
  subtitleKey,
  onAudio,
  onSubtitle,
}: {
  pb: Playback
  subtitleKey: string
  onAudio: (index: number) => void
  onSubtitle: (key: string) => void
}) {
  const { t } = useTranslation()
  const subLabel = (s: Playback['subtitles'][number]) =>
    [languageName(s.language) ?? t('video.detail.unknownTrack', { index: s.key }), s.title, s.forced ? t('video.detail.forced') : null]
      .filter(Boolean)
      .join(' · ')
  return (
    <View style={styles.panelWrap}>
      <LinearGradient colors={['transparent', 'rgba(0,0,0,0.92)']} style={StyleSheet.absoluteFill} />
      <TVFocusGuideView autoFocus trapFocusDown trapFocusLeft trapFocusRight style={styles.panel}>
        {pb.audio.length > 0 ? (
          <View style={styles.column}>
            <T variant="section" style={styles.columnTitle}>
              {t('video.player.audio')}
            </T>
            <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={styles.options}>
              {pb.audio.map((a) => (
                <Button
                  key={a.index}
                  label={trackLabel(a)}
                  selected={a.index === pb.audio_index}
                  hasTVPreferredFocus={a.index === pb.audio_index}
                  onPress={() => a.index !== pb.audio_index && onAudio(a.index)}
                />
              ))}
            </ScrollView>
          </View>
        ) : null}
        <View style={styles.column}>
          <T variant="section" style={styles.columnTitle}>
            {t('video.player.subtitles')}
          </T>
          <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={styles.options}>
            <Button label={t('video.player.subtitlesOff')} selected={subtitleKey === ''} onPress={() => onSubtitle('')} hasTVPreferredFocus={pb.audio.length === 0} />
            {pb.subtitles.map((s) => (
              <Button key={s.key} label={subLabel(s)} selected={s.key === subtitleKey} onPress={() => onSubtitle(s.key)} />
            ))}
          </ScrollView>
          {pb.unsupported_subtitles.length > 0 ? (
            <T variant="small" faint style={{ marginTop: px(12) }}>
              {t('video.player.unsupportedSubtitles')}
            </T>
          ) : null}
        </View>
        <T variant="small" faint>
          {t(`video.player.mode.${pb.mode}`)}
        </T>
      </TVFocusGuideView>
    </View>
  )
}

/** Konec dílu: odpočet do dalšího. */
export function NextUp({ label, seconds, onPlay, onCancel }: { label: string; seconds: number; onPlay: () => void; onCancel: () => void }) {
  const { t } = useTranslation()
  return (
    <View style={styles.next}>
      <T variant="caption" dim>
        {t('video.player.nextIn', { seconds })}
      </T>
      <T variant="section" numberOfLines={1} style={{ marginBottom: px(24) }}>
        {label}
      </T>
      <View style={{ flexDirection: 'row', gap: px(20) }}>
        <Button primary icon={PlayIcon} label={t('video.player.nextEpisode')} onPress={onPlay} hasTVPreferredFocus />
        <Button label={t('video.player.cancel')} onPress={onCancel} />
      </View>
    </View>
  )
}

const styles = StyleSheet.create({
  top: { position: 'absolute', left: 0, right: 0, top: 0, paddingHorizontal: safe.x, paddingTop: safe.y, paddingBottom: px(120) },
  center: { ...StyleSheet.absoluteFill, alignItems: 'center', justifyContent: 'center' },
  bigIcon: { width: px(160), height: px(160), borderRadius: px(80), backgroundColor: 'rgba(0,0,0,0.5)', alignItems: 'center', justifyContent: 'center' },
  bottom: { position: 'absolute', left: 0, right: 0, bottom: 0, paddingHorizontal: safe.x, paddingTop: px(160), paddingBottom: safe.y },
  bar: { height: px(10), borderRadius: px(5), backgroundColor: 'rgba(255,255,255,0.25)', justifyContent: 'center' },
  played: { position: 'absolute', left: 0, top: 0, bottom: 0, borderRadius: px(5), backgroundColor: colors.text },
  scrubFill: { position: 'absolute', left: 0, top: 0, bottom: 0, borderRadius: px(5), backgroundColor: colors.accent, opacity: 0.85 },
  knob: { position: 'absolute', width: px(26), height: px(26), marginLeft: -px(13), borderRadius: px(13), backgroundColor: colors.text },
  times: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', marginTop: px(18) },
  timeLeft: { flexDirection: 'row', alignItems: 'center', gap: px(12) },
  panelWrap: { position: 'absolute', left: 0, right: 0, bottom: 0, paddingTop: px(200) },
  panel: { paddingHorizontal: safe.x, paddingBottom: safe.y, gap: px(28) },
  column: {},
  columnTitle: { marginBottom: px(12) },
  options: { gap: px(16), paddingVertical: px(10) },
  next: {
    position: 'absolute',
    right: safe.x,
    bottom: px(200),
    padding: px(36),
    borderRadius: radius.card,
    backgroundColor: 'rgba(22,23,29,0.94)',
    maxWidth: px(900),
  },
})
