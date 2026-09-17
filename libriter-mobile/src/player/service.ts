import TrackPlayer, { Event } from 'react-native-track-player'

import { SKIP_BACK, SKIP_FORWARD } from 'libriter-shared'

/**
 * Obsluha ovládacích prvků mimo aplikaci: zamčená obrazovka, ovládací centrum,
 * sluchátka, Android Auto. Běží i když je aplikace na pozadí, a proto tady
 * není nic z Reactu – jen příkazy přehrávači.
 *
 * Registruje se v index.js dřív, než se vykreslí první obrazovka.
 */
export async function PlaybackService(): Promise<void> {
  TrackPlayer.addEventListener(Event.RemotePlay, () => TrackPlayer.play())
  TrackPlayer.addEventListener(Event.RemotePause, () => TrackPlayer.pause())
  TrackPlayer.addEventListener(Event.RemoteStop, () => TrackPlayer.pause())
  TrackPlayer.addEventListener(Event.RemoteNext, () => TrackPlayer.skipToNext())
  TrackPlayer.addEventListener(Event.RemotePrevious, () => TrackPlayer.skipToPrevious())

  TrackPlayer.addEventListener(Event.RemoteSeek, ({ position }) => TrackPlayer.seekTo(position))
  TrackPlayer.addEventListener(Event.RemoteJumpForward, ({ interval }) =>
    TrackPlayer.seekBy(interval ?? SKIP_FORWARD),
  )
  TrackPlayer.addEventListener(Event.RemoteJumpBackward, ({ interval }) =>
    TrackPlayer.seekBy(-(interval ?? SKIP_BACK)),
  )

  // Odpojená sluchátka mají pauzovat, ne hrát dál z reproduktoru.
  TrackPlayer.addEventListener(Event.RemoteDuck, async ({ permanent, paused }) => {
    if (permanent || paused) await TrackPlayer.pause()
  })
}
