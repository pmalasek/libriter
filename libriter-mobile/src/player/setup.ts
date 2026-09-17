import TrackPlayer, {
  AndroidAudioContentType,
  AppKilledPlaybackBehavior,
  Capability,
  IOSCategory,
  IOSCategoryMode,
} from 'react-native-track-player'

import { SKIP_BACK, SKIP_FORWARD } from 'libriter-shared'

let ready: Promise<void> | null = null

/**
 * Připraví přehrávač. Volá se z několika míst (start aplikace, první
 * přehrání), ale proběhnout smí jen jednou – opakované setupPlayer skončí
 * chybou, tak se drží rozpracovaný slib.
 */
export function ensurePlayer(): Promise<void> {
  ready ??= setup().catch((error: unknown) => {
    // Neúspěch se nesmí zapamatovat, jinak by se přehrávač už nikdy
    // nerozběhl.
    ready = null
    throw error
  })
  return ready
}

async function setup(): Promise<void> {
  await TrackPlayer.setupPlayer({
    autoHandleInterruptions: true,
    // Mluvené slovo, ne hudba: systém podle toho volí míru zpracování zvuku
    // a na Androidu i to, jak se chová při hlášení navigace.
    androidAudioContentType: AndroidAudioContentType.Speech,
    iosCategory: IOSCategory.Playback,
    iosCategoryMode: IOSCategoryMode.SpokenAudio,
  })

  await TrackPlayer.updateOptions({
    android: {
      // Ukončení aplikace ze seznamu úloh má zastavit i přehrávání; jinak
      // zůstane viset notifikace, kterou nemá co ovládat.
      appKilledPlaybackBehavior: AppKilledPlaybackBehavior.StopPlaybackAndRemoveNotification,
    },
    capabilities: [
      Capability.Play,
      Capability.Pause,
      Capability.SkipToNext,
      Capability.SkipToPrevious,
      Capability.SeekTo,
      Capability.JumpForward,
      Capability.JumpBackward,
    ],
    // Na zamčené obrazovce se vejdou jen tři; u audioknihy jsou užitečnější
    // skoky o pár vteřin než přeskakování celých kapitol.
    compactCapabilities: [Capability.JumpBackward, Capability.Play, Capability.JumpForward],
    forwardJumpInterval: SKIP_FORWARD,
    backwardJumpInterval: SKIP_BACK,
    // Průběh se hlásí po sekundě: z něj se počítají odposlouchané sekundy
    // do deníku a podle něj se ukládá pozice.
    progressUpdateEventInterval: 1,
  })
}
