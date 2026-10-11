import { memo, useEffect, useState } from 'react'
import { StyleSheet, Text, View } from 'react-native'
import { absoluteUrl, activeCues, getBaseUrl, parseVtt, type Cue } from 'sceniter-shared'
import type { SubtitleSize } from '@/lib/storage'
import { fonts, px } from '@/theme'

const SIZES: Record<SubtitleSize, number> = { small: 40, medium: 50, large: 62 }

/** Stáhne a rozparsuje WebVTT titulky vybrané stopy. */
export function useCues(url: string | undefined): Cue[] {
  const [cues, setCues] = useState<Cue[]>([])
  useEffect(() => {
    setCues([])
    if (!url) return
    let cancelled = false
    fetch(absoluteUrl(getBaseUrl(), url))
      .then((r) => (r.ok ? r.text() : ''))
      .then((text) => !cancelled && setCues(parseVtt(text)))
      .catch(() => undefined)
    return () => {
      cancelled = true
    }
  }, [url])
  return cues
}

/**
 * Titulky vykreslené nad videem. Vlastní vykreslení místo nativního: AVPlayer
 * neumí externí titulky k souboru přehrávanému napřímo a takhle vypadají
 * titulky stejně na Apple TV i Google TV.
 */
export const SubtitleOverlay = memo(function SubtitleOverlay({
  cues,
  time,
  size,
  raised,
}: {
  cues: Cue[]
  time: number
  size: SubtitleSize
  /** Při zobrazeném ovládání titulky uhnou nahoru. */
  raised: boolean
}) {
  const active = cues.length ? activeCues(cues, time) : []
  if (active.length === 0) return null
  const fontSize = px(SIZES[size])
  return (
    <View pointerEvents="none" style={[styles.wrap, { bottom: raised ? px(300) : px(90) }]}>
      {active.map((cue) => (
        <Text key={cue.start} style={[styles.text, { fontSize, lineHeight: fontSize * 1.25 }]}>
          {cue.lines.join('\n')}
        </Text>
      ))}
    </View>
  )
})

const styles = StyleSheet.create({
  wrap: { position: 'absolute', left: px(160), right: px(160), alignItems: 'center' },
  text: {
    fontFamily: fonts.bodyBold,
    color: '#FFFFFF',
    textAlign: 'center',
    textShadowColor: 'rgba(0,0,0,0.95)',
    textShadowOffset: { width: 0, height: px(2) },
    textShadowRadius: px(8),
    backgroundColor: 'rgba(0,0,0,0.35)',
    paddingHorizontal: px(18),
    borderRadius: px(8),
    overflow: 'hidden',
  },
})
