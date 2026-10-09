/**
 * Profil přehrávání pro server: Safari (macOS, iOS) má nativní HLS a umí
 * HEVC i AC3, ostatní prohlížeče přehrávají HLS přes hls.js (MSE).
 */
export type PlaybackProfile = 'web' | 'safari'

export function detectProfile(): PlaybackProfile {
  const ua = navigator.userAgent
  const safari = /^((?!chrome|chromium|android|crios|fxios|edg).)*safari/i.test(ua)
  return safari ? 'safari' : 'web'
}

/** Přehraje prohlížeč HLS sám (bez hls.js)? */
export function hasNativeHls(video: HTMLVideoElement): boolean {
  return video.canPlayType('application/vnd.apple.mpegurl') !== ''
}

const DEVICE_KEY = 'sceniter.device'

/** Trvalé ID prohlížeče pro zápis pozice (rozlišení zařízení na serveru). */
export function deviceId(): string {
  try {
    let id = localStorage.getItem(DEVICE_KEY)
    if (!id) {
      id = crypto.randomUUID()
      localStorage.setItem(DEVICE_KEY, id)
    }
    return id
  } catch {
    return 'web'
  }
}
