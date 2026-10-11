import { forwardRef, useEffect, useRef, useState } from 'react'
import { Animated, Pressable, ScrollView, StyleSheet, TVFocusGuideView, View } from 'react-native'
import { LinearGradient } from 'expo-linear-gradient'
import { useTranslation } from 'react-i18next'
import { PauseIcon, PlayIcon, RotateCcwIcon, SkipForwardIcon, SubtitlesIcon, TvIcon } from 'lucide-react-native'
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

/** Horní a spodní lišta: název, posuvník (drží fokus), čas a řada tlačítek. */
export const Chrome = forwardRef<View, {
  title: string
  subtitle?: string
  time: number
  duration: number
  /** Kam už je rostoucí playlist hotový (přebalení); undefined = celé. */
  available?: number
  /** Cíl přetáčení, dokud se nepotvrdí; undefined = nepřetáčí se. */
  scrubTo?: number
  paused: boolean
  /** Lišta je vidět; skrytá zůstává namontovaná, aby posuvník držel fokus. */
  visible: boolean
  nextLabel?: string
  /** Po ukázání lišty šipkou dolů rovnou zaměřit první tlačítko. */
  focusButtons?: boolean
  onSeekFocus: () => void
  onButtonFocus: () => void
  onTracks: () => void
  onRestart: () => void
  onNext: () => void
  /** Hraje díl seriálu: skok na detail seriálu se zaměřeným dílem. */
  onShow?: () => void
}>(function Chrome(
  { title, subtitle, time, duration, available, scrubTo, paused, visible, nextLabel, focusButtons, onSeekFocus, onButtonFocus, onTracks, onRestart, onNext, onShow },
  seekRef,
) {
  const { t, i18n } = useTranslation()
  const opacity = useRef(new Animated.Value(visible ? 1 : 0)).current
  useEffect(() => {
    Animated.timing(opacity, { toValue: visible ? 1 : 0, duration: 180, useNativeDriver: true }).start()
  }, [opacity, visible])
  const [seekFocused, setSeekFocused] = useState(false)

  const shown = scrubTo ?? time
  const ratio = duration > 0 ? Math.min(1, shown / duration) : 0
  const played = duration > 0 ? Math.min(1, time / duration) : 0
  const ready = duration > 0 && available !== undefined ? Math.min(1, available / duration) : 1
  const active = seekFocused && visible

  return (
    <Animated.View style={[StyleSheet.absoluteFill, { opacity }]}>
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
        <View style={styles.center} pointerEvents="none">
          <View style={styles.bigIcon}>
            <PauseIcon size={px(72)} color={colors.text} fill={colors.text} />
          </View>
        </View>
      ) : null}

      <LinearGradient colors={['transparent', 'rgba(0,0,0,0.85)']} style={styles.bottom}>
        {/* Posuvník: jediný prvek, který má fokus i při skryté liště – jinak by
            Android nedoručil šipky do aplikace. Šipky přetáčí, OK pozastaví. */}
        <Pressable
          ref={seekRef}
          focusable
          onFocus={() => {
            setSeekFocused(true)
            onSeekFocus()
          }}
          onBlur={() => setSeekFocused(false)}
          style={styles.seek}
        >
          <View style={[styles.bar, active && styles.barActive]}>
            {ready < 1 ? <View style={[styles.ready, { width: `${ready * 100}%` }]} /> : null}
            <View style={[styles.played, { width: `${played * 100}%` }]} />
            {scrubTo !== undefined ? <View style={[styles.scrubFill, { width: `${ratio * 100}%` }]} /> : null}
            <View style={[styles.knob, active && styles.knobActive, { left: `${ratio * 100}%` }]} />
          </View>
        </Pressable>
        <View style={styles.times}>
          <View style={styles.timeLeft}>
            {paused ? <PlayIcon size={px(30)} color={colors.text} fill={colors.text} /> : null}
            <T variant="bodyMedium">{formatClock(shown)}</T>
          </View>
          <T variant="caption" dim>
            {active ? t('video.tv.player.seekHint') : duration > 0 ? t('video.tv.player.endsAt', { time: endsAt(duration - shown, i18n.language) }) : ''}
          </T>
          <T variant="bodyMedium" dim>
            {duration > 0 ? `−${formatClock(Math.max(0, duration - shown))}` : ''}
          </T>
        </View>
        {visible ? (
          <View style={styles.actions}>
            <Button
              icon={SubtitlesIcon}
              label={t('video.tv.player.tracks')}
              scale={false}
              hasTVPreferredFocus={focusButtons}
              onFocus={onButtonFocus}
              onPress={onTracks}
            />
            <Button icon={RotateCcwIcon} label={t('video.tv.player.restart')} scale={false} onFocus={onButtonFocus} onPress={onRestart} />
            {onShow ? <Button icon={TvIcon} label={t('video.tv.player.showDetail')} scale={false} onFocus={onButtonFocus} onPress={onShow} /> : null}
            {nextLabel ? (
              <Button
                icon={SkipForwardIcon}
                label={t('video.tv.player.nextEpisode', { episode: nextLabel })}
                scale={false}
                onFocus={onButtonFocus}
                onPress={onNext}
              />
            ) : null}
          </View>
        ) : null}
      </LinearGradient>
    </Animated.View>
  )
})

/** Poslední minuta dílu: nabídka dalšího dílu (OK přepne hned). */
export function UpNextPill({ label, onPlay, onFocus }: { label: string; onPlay: () => void; onFocus: () => void }) {
  const { t } = useTranslation()
  return (
    <View style={styles.pill}>
      <Button primary hasTVPreferredFocus icon={SkipForwardIcon} label={t('video.player.nextEpisode')} onFocus={onFocus} onPress={onPlay} />
      <T variant="small" dim numberOfLines={1} style={{ marginTop: px(10), maxWidth: px(520) }}>
        {label}
      </T>
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
  seek: { paddingVertical: px(14) },
  bar: { height: px(10), borderRadius: px(5), backgroundColor: 'rgba(255,255,255,0.25)', justifyContent: 'center' },
  barActive: { height: px(16), borderRadius: px(8) },
  ready: { position: 'absolute', left: 0, top: 0, bottom: 0, borderRadius: px(5), backgroundColor: 'rgba(255,255,255,0.45)' },
  played: { position: 'absolute', left: 0, top: 0, bottom: 0, borderRadius: px(5), backgroundColor: colors.text },
  scrubFill: { position: 'absolute', left: 0, top: 0, bottom: 0, borderRadius: px(5), backgroundColor: colors.accent, opacity: 0.85 },
  knob: { position: 'absolute', width: px(26), height: px(26), marginLeft: -px(13), borderRadius: px(13), backgroundColor: colors.text },
  knobActive: { width: px(38), height: px(38), marginLeft: -px(19), borderRadius: px(19), borderWidth: px(5), borderColor: colors.accent },
  actions: { flexDirection: 'row', gap: px(20), marginTop: px(24) },
  pill: { position: 'absolute', right: safe.x, bottom: px(120), alignItems: 'flex-end' },
  times: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', marginTop: px(4) },
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
    backgroundColor: 'rgba(23,30,35,0.94)',
    borderWidth: px(2),
    borderColor: colors.edge,
    maxWidth: px(900),
  },
})
